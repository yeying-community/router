import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Breadcrumb, Button, Card, Form, Header, Label } from 'semantic-ui-react';
import {
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from 'react-router-dom';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import {
  formatAmountWithUnit,
  formatYYCValue,
} from '../../helpers/render';

const YYC_UNIT = 'YYC';

function renderStatus(status, t) {
  switch (status) {
    case 1:
      return (
        <Label basic color='green' className='router-tag'>
          {t('redemption.status.unused')}
        </Label>
      );
    case 2:
      return (
        <Label basic color='red' className='router-tag'>
          {t('redemption.status.disabled')}
        </Label>
      );
    case 3:
      return (
        <Label basic color='grey' className='router-tag'>
          {t('redemption.status.used')}
        </Label>
      );
    default:
      return (
        <Label basic color='black' className='router-tag'>
          {t('redemption.status.unknown')}
        </Label>
      );
  }
}

const toGroupOptions = (rows) =>
  (Array.isArray(rows) ? rows : []).map((item) => ({
    key: item.id,
    value: item.id,
    text: item.name || item.id,
  }));

const toFaceValueUnitOptions = (rows, currentUnit = '') => {
  const options = [
    {
      key: YYC_UNIT,
      value: YYC_UNIT,
      text: YYC_UNIT,
    },
  ];
  const seen = new Set([YYC_UNIT]);
  (Array.isArray(rows) ? rows : [])
    .filter((item) => Number(item?.status || 0) === 1)
    .forEach((item) => {
      const code = (item?.code || '').toString().trim().toUpperCase();
      if (!code || seen.has(code)) {
        return;
      }
      seen.add(code);
      options.push({
        key: code,
        value: code,
        text: `${code}${item?.name ? ` (${item.name})` : ''}`,
      });
    });
  const normalizedCurrentUnit = (currentUnit || '').toString().trim().toUpperCase();
  if (normalizedCurrentUnit && !seen.has(normalizedCurrentUnit)) {
    options.push({
      key: normalizedCurrentUnit,
      value: normalizedCurrentUnit,
      text: normalizedCurrentUnit,
    });
  }
  return options;
};

const buildCurrencyIndex = (rows) => {
  const next = {
    [YYC_UNIT]: {
      code: YYC_UNIT,
      yyc_per_unit: 1,
      minor_unit: 0,
    },
  };
  (Array.isArray(rows) ? rows : []).forEach((item) => {
    const code = (item?.code || '').toString().trim().toUpperCase();
    if (!code) {
      return;
    }
    next[code] = item;
  });
  return next;
};

const computeYYCPreview = (amountValue, unitValue, currencyIndex) => {
  const amount = Number.parseFloat(`${amountValue ?? ''}`);
  if (!Number.isFinite(amount) || amount <= 0) {
    return 0;
  }
  const normalizedUnit = (unitValue || YYC_UNIT).toString().trim().toUpperCase();
  if (normalizedUnit === YYC_UNIT) {
    return Math.round(amount);
  }
  const currency = currencyIndex[normalizedUnit];
  const rate = Number(currency?.yyc_per_unit || 0);
  if (!Number.isFinite(rate) || rate <= 0) {
    return 0;
  }
  return Math.round(amount * rate);
};

const normalizeFaceValueAmount = (data) => {
  const rawAmount = Number(data?.face_value_amount ?? 0);
  if (Number.isFinite(rawAmount) && rawAmount > 0) {
    return `${rawAmount}`;
  }
  const yycValue = Number(data?.yyc_value ?? data?.quota ?? 0);
  if (Number.isFinite(yycValue) && yycValue > 0) {
    return `${yycValue}`;
  }
  return '0';
};

const normalizeFaceValueUnit = (data) => {
  const unit = (data?.face_value_unit || '').toString().trim().toUpperCase();
  return unit || YYC_UNIT;
};

const formatGroupLabel = (data) => {
  const name = (data?.group_name || '').toString().trim();
  if (name) {
    return name;
  }
  const id = (data?.group_id || '').toString().trim();
  return id || '-';
};

const RedemptionDetail = () => {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const { id } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [optionsLoading, setOptionsLoading] = useState(false);
  const [redemption, setRedemption] = useState(null);
  const [groupOptions, setGroupOptions] = useState([]);
  const [unitOptions, setUnitOptions] = useState([
    {
      key: YYC_UNIT,
      value: YYC_UNIT,
      text: YYC_UNIT,
    },
  ]);
  const [currencyIndex, setCurrencyIndex] = useState(buildCurrencyIndex([]));
  const [inputs, setInputs] = useState({
    name: '',
    group_id: '',
    face_value_amount: '0',
    face_value_unit: YYC_UNIT,
  });
  const isEditing = searchParams.get('edit') === '1';
  const returnPath = (() => {
    const from = location.state?.from;
    if (typeof from !== 'string') {
      return '';
    }
    const normalized = from.trim();
    return normalized.startsWith('/') ? normalized : '';
  })();

  const yycPreview = useMemo(
    () => computeYYCPreview(inputs.face_value_amount, inputs.face_value_unit, currencyIndex),
    [currencyIndex, inputs.face_value_amount, inputs.face_value_unit]
  );

  const syncInputs = useCallback((data) => {
    setInputs({
      name: (data?.name || '').toString(),
      group_id: (data?.group_id || '').toString().trim(),
      face_value_amount: normalizeFaceValueAmount(data),
      face_value_unit: normalizeFaceValueUnit(data),
    });
  }, []);

  const setEditMode = useCallback(
    (nextEditing) => {
      const nextSearchParams = new URLSearchParams(searchParams.toString());
      if (nextEditing) {
        nextSearchParams.set('edit', '1');
      } else {
        nextSearchParams.delete('edit');
      }
      setSearchParams(nextSearchParams, { replace: true });
    },
    [searchParams, setSearchParams]
  );

  const handleInputChange = useCallback((e, { name, value }) => {
    setInputs((prev) => ({
      ...prev,
      [name]: value,
    }));
  }, []);

  const loadOptions = useCallback(async (currentUnit = '') => {
    setOptionsLoading(true);
    try {
      const [groupsRes, currenciesRes] = await Promise.all([
        API.get('/api/v1/admin/groups', {
          params: {
            page: 1,
            page_size: 200,
          },
        }),
        API.get('/api/v1/admin/billing/currencies'),
      ]);
      const groupsPayload = groupsRes?.data || {};
      if (!groupsPayload.success) {
        throw new Error(groupsPayload.message || t('redemption.messages.load_groups_failed'));
      }
      const currenciesPayload = currenciesRes?.data || {};
      if (!currenciesPayload.success) {
        throw new Error(
          currenciesPayload.message || t('redemption.messages.load_units_failed')
        );
      }
      const nextGroups = groupsPayload?.data?.items || [];
      const nextCurrencies = Array.isArray(currenciesPayload?.data)
        ? currenciesPayload.data
        : [];
      setGroupOptions(toGroupOptions(nextGroups));
      setUnitOptions(toFaceValueUnitOptions(nextCurrencies, currentUnit));
      setCurrencyIndex(buildCurrencyIndex(nextCurrencies));
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setOptionsLoading(false);
    }
  }, [t]);

  const loadRedemption = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get(`/api/v1/admin/redemption/${id}`);
      const { success, message, data } = res.data;
      if (success) {
        setRedemption(data);
        syncInputs(data);
        await loadOptions(normalizeFaceValueUnit(data));
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  }, [id, loadOptions, syncInputs]);

  useEffect(() => {
    loadRedemption().then();
  }, [loadRedemption]);

  const handleCancelEdit = () => {
    syncInputs(redemption);
    setEditMode(false);
  };

  const submitEdit = async () => {
    if ((inputs.name || '').trim() === '') {
      showError(t('redemption.messages.name_required'));
      return;
    }
    if ((inputs.group_id || '').trim() === '') {
      showError(t('redemption.messages.group_required'));
      return;
    }
    const faceValueAmount = Number.parseFloat(`${inputs.face_value_amount ?? ''}`);
    if (!Number.isFinite(faceValueAmount) || faceValueAmount <= 0) {
      showError(t('redemption.messages.face_value_invalid'));
      return;
    }

    setSaving(true);
    try {
      const res = await API.put('/api/v1/admin/redemption/', {
        id,
        name: (inputs.name || '').toString().trim(),
        group_id: inputs.group_id,
        face_value_amount: faceValueAmount,
        face_value_unit: inputs.face_value_unit,
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message);
        return;
      }
      setRedemption(data);
      syncInputs(data);
      setUnitOptions(toFaceValueUnitOptions(
        Object.values(currencyIndex).filter(Boolean),
        normalizeFaceValueUnit(data)
      ));
      setEditMode(false);
      showSuccess(t('redemption.messages.update_success'));
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setSaving(false);
    }
  };

  const redeemedByValue =
    redemption?.redeemed_by_username ||
    redemption?.redeemed_by_user_id ||
    t('redemption.table.not_redeemed');

  const handleBack = () => {
    if (returnPath !== '') {
      navigate(-1);
      return;
    }
    navigate('/redemption');
  };

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <div className='router-entity-detail-page'>
            <div className='router-entity-detail-breadcrumb'>
              <Breadcrumb size='small'>
                <Breadcrumb.Section link onClick={handleBack}>
                  {t('header.redemption')}
                </Breadcrumb.Section>
                <Breadcrumb.Divider icon='right chevron' />
                <Breadcrumb.Section active>
                  {redemption?.name || redemption?.code || id}
                </Breadcrumb.Section>
              </Breadcrumb>
            </div>

            <section className='router-entity-detail-section'>
              <div className='router-entity-detail-section-header'>
                <div className='router-toolbar-start'>
                  <Header as='h3' className='router-entity-detail-section-title'>
                    {t('common.basic_info')}
                  </Header>
                  {redemption ? renderStatus(redemption.status, t) : null}
                </div>
                <div className='router-toolbar-start'>
                  {isEditing ? (
                    <>
                      <Button
                        className='router-page-button'
                        onClick={handleCancelEdit}
                        disabled={saving}
                      >
                        {t('redemption.edit.buttons.cancel')}
                      </Button>
                      <Button
                        className='router-page-button'
                        primary
                        loading={saving}
                        disabled={saving}
                        onClick={submitEdit}
                      >
                        {t('redemption.edit.buttons.submit')}
                      </Button>
                    </>
                  ) : (
                    <Button
                      className='router-page-button'
                      primary
                      onClick={() => setEditMode(true)}
                    >
                      {t('redemption.buttons.edit')}
                    </Button>
                  )}
                </div>
              </div>

              <Form loading={loading || optionsLoading}>
                <Form.Group widths='equal'>
                  {isEditing ? (
                    <Form.Input
                      className='router-section-input'
                      label={t('redemption.edit.name')}
                      name='name'
                      value={inputs.name}
                      placeholder={t('redemption.edit.name_placeholder')}
                      onChange={handleInputChange}
                    />
                  ) : (
                    <Form.Input
                      className='router-section-input'
                      label={t('redemption.table.name')}
                      value={redemption?.name || t('redemption.table.no_name')}
                      readOnly
                    />
                  )}
                  <Form.Input
                    className='router-section-input'
                    label={t('redemption.detail.code')}
                    value={redemption?.code || ''}
                    readOnly
                  />
                </Form.Group>
                <Form.Group widths='equal'>
                  {isEditing ? (
                    <Form.Select
                      className='router-section-input'
                      label={t('redemption.edit.group')}
                      name='group_id'
                      placeholder={t('redemption.edit.group_placeholder')}
                      options={groupOptions}
                      value={inputs.group_id}
                      onChange={handleInputChange}
                      search
                      selection
                    />
                  ) : (
                    <Form.Input
                      className='router-section-input'
                      label={t('redemption.table.group')}
                      value={formatGroupLabel(redemption)}
                      readOnly
                    />
                  )}
                  <Form.Input
                    className='router-section-input'
                    label={t('redemption.detail.redeemed_by')}
                    value={redeemedByValue}
                    readOnly
                  />
                </Form.Group>
                <Form.Group widths='equal'>
                  {isEditing ? (
                    <Form.Input
                      className='router-section-input'
                      label={t('redemption.edit.face_value_amount')}
                      name='face_value_amount'
                      type='number'
                      value={inputs.face_value_amount}
                      placeholder={t('redemption.edit.face_value_amount_placeholder')}
                      onChange={handleInputChange}
                      step={inputs.face_value_unit === YYC_UNIT ? '1' : '0.01'}
                      min='0'
                    />
                  ) : (
                    <Form.Input
                      className='router-section-input'
                      label={t('redemption.table.face_value')}
                      value={formatAmountWithUnit(
                        redemption?.face_value_amount ?? redemption?.yyc_value ?? redemption?.quota ?? 0,
                        normalizeFaceValueUnit(redemption)
                      )}
                      readOnly
                    />
                  )}
                  {isEditing ? (
                    <Form.Select
                      className='router-section-input'
                      label={t('redemption.edit.face_value_unit')}
                      name='face_value_unit'
                      placeholder={t('redemption.edit.face_value_unit_placeholder')}
                      options={unitOptions}
                      value={inputs.face_value_unit}
                      onChange={handleInputChange}
                      selection
                    />
                  ) : (
                    <Form.Input
                      className='router-section-input'
                      label={t('redemption.table.quota')}
                      value={redemption ? formatYYCValue(redemption.yyc_value ?? redemption.quota) : ''}
                      readOnly
                    />
                  )}
                </Form.Group>
                {isEditing ? (
                  <Form.Field>
                    <Form.Input
                      className='router-section-input'
                      label={t('redemption.edit.credit_yyc')}
                      value={yycPreview > 0 ? formatYYCValue(yycPreview) : '-'}
                      readOnly
                    />
                  </Form.Field>
                ) : null}
                <Form.Group widths='equal'>
                  <Form.Input
                    className='router-section-input'
                    label={t('redemption.table.created_time')}
                    value={
                      redemption?.created_time
                        ? timestamp2string(redemption.created_time)
                        : ''
                    }
                    readOnly
                  />
                  <Form.Input
                    className='router-section-input'
                    label={t('redemption.table.redeemed_time')}
                    value={
                      redemption?.redeemed_time
                        ? timestamp2string(redemption.redeemed_time)
                        : t('redemption.table.not_redeemed')
                    }
                    readOnly
                  />
                </Form.Group>
              </Form>
            </section>
          </div>
        </Card.Content>
      </Card>
    </div>
  );
};

export default RedemptionDetail;
