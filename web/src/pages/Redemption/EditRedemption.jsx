import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Card } from 'semantic-ui-react';
import { useNavigate } from 'react-router-dom';
import { API, downloadTextAsFile, showError, showSuccess } from '../../helpers';
import { formatYYCValue } from '../../helpers/render';

const YYC_UNIT = 'YYC';

const originInputs = {
  name: '',
  group_id: '',
  face_value_amount: '100000',
  face_value_unit: YYC_UNIT,
  count: 1,
};

const toGroupOptions = (rows) =>
  (Array.isArray(rows) ? rows : []).map((item) => ({
    key: item.id,
    value: item.id,
    text: item.name || item.id,
  }));

const toFaceValueUnitOptions = (rows) => {
  const options = [
    {
      key: YYC_UNIT,
      value: YYC_UNIT,
      text: YYC_UNIT,
    },
  ];
  (Array.isArray(rows) ? rows : [])
    .filter((item) => Number(item?.status || 0) === 1)
    .forEach((item) => {
      const code = (item?.code || '').toString().trim().toUpperCase();
      if (!code || code === YYC_UNIT) {
        return;
      }
      options.push({
        key: code,
        value: code,
        text: `${code}${item?.name ? ` (${item.name})` : ''}`,
      });
    });
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

const EditRedemption = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [inputs, setInputs] = useState(originInputs);
  const [groupOptions, setGroupOptions] = useState([]);
  const [unitOptions, setUnitOptions] = useState([
    {
      key: YYC_UNIT,
      value: YYC_UNIT,
      text: YYC_UNIT,
    },
  ]);
  const [currencyIndex, setCurrencyIndex] = useState(buildCurrencyIndex([]));
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  const { name, group_id, face_value_amount, face_value_unit, count } = inputs;

  const yycPreview = useMemo(
    () => computeYYCPreview(face_value_amount, face_value_unit, currencyIndex),
    [currencyIndex, face_value_amount, face_value_unit]
  );

  useEffect(() => {
    const loadOptions = async () => {
      setLoading(true);
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
        setUnitOptions(toFaceValueUnitOptions(nextCurrencies));
        setCurrencyIndex(buildCurrencyIndex(nextCurrencies));
      } catch (error) {
        showError(error?.message || error);
      } finally {
        setLoading(false);
      }
    };
    loadOptions().then();
  }, [t]);

  const handleCancel = () => {
    navigate('/redemption');
  };

  const handleInputChange = (e, { name, value }) => {
    setInputs((current) => ({ ...current, [name]: value }));
  };

  const submit = async () => {
    if ((inputs.name || '').trim() === '') {
      showError(t('redemption.messages.name_required'));
      return;
    }
    if ((inputs.group_id || '').trim() === '') {
      showError(t('redemption.messages.group_required'));
      return;
    }
    const localInputs = { ...inputs };
    localInputs.count = Number.parseInt(`${localInputs.count ?? ''}`, 10);
    localInputs.face_value_amount = Number.parseFloat(`${localInputs.face_value_amount ?? ''}`);
    if (!Number.isFinite(localInputs.count) || localInputs.count <= 0) {
      showError(t('redemption.messages.count_invalid'));
      return;
    }
    if (
      !Number.isFinite(localInputs.face_value_amount) ||
      localInputs.face_value_amount <= 0
    ) {
      showError(t('redemption.messages.face_value_invalid'));
      return;
    }

    setSubmitting(true);
    try {
      const res = await API.post('/api/v1/admin/redemption/', {
        name: (localInputs.name || '').toString().trim(),
        group_id: localInputs.group_id,
        face_value_amount: localInputs.face_value_amount,
        face_value_unit: localInputs.face_value_unit,
        count: localInputs.count,
      });
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(t('redemption.messages.create_success'));
        if (data) {
          let text = '';
          for (let i = 0; i < data.length; i++) {
            text += `${data[i]}\n`;
          }
          downloadTextAsFile(text, `${inputs.name}.txt`);
        }
        setInputs(originInputs);
        navigate('/redemption');
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <Card.Header className='header router-page-title'>
            {t('redemption.edit.title_create')}
          </Card.Header>
          <div className='router-toolbar router-block-gap-sm'>
            <div className='router-toolbar-start'>
              <Button className='router-page-button' onClick={handleCancel} disabled={submitting}>
                {t('redemption.edit.buttons.cancel')}
              </Button>
              <Button
                className='router-page-button'
                positive
                onClick={submit}
                loading={submitting}
                disabled={loading || submitting}
              >
                {t('redemption.edit.buttons.submit')}
              </Button>
            </div>
          </div>
          <Form autoComplete='off' loading={loading}>
            <Form.Field>
              <Form.Input
                className='router-section-input'
                label={t('redemption.edit.name')}
                name='name'
                placeholder={t('redemption.edit.name_placeholder')}
                onChange={handleInputChange}
                value={name}
                autoComplete='off'
                required
              />
            </Form.Field>
            <Form.Field>
              <Form.Select
                className='router-section-input'
                label={t('redemption.edit.group')}
                name='group_id'
                placeholder={t('redemption.edit.group_placeholder')}
                options={groupOptions}
                value={group_id}
                onChange={handleInputChange}
                search
                selection
                required
              />
            </Form.Field>
            <Form.Group widths='equal'>
              <Form.Input
                className='router-section-input'
                label={t('redemption.edit.face_value_amount')}
                name='face_value_amount'
                placeholder={t('redemption.edit.face_value_amount_placeholder')}
                onChange={handleInputChange}
                value={face_value_amount}
                autoComplete='off'
                type='number'
                step={face_value_unit === YYC_UNIT ? '1' : '0.01'}
                min='0'
              />
              <Form.Select
                className='router-section-input'
                label={t('redemption.edit.face_value_unit')}
                name='face_value_unit'
                placeholder={t('redemption.edit.face_value_unit_placeholder')}
                options={unitOptions}
                value={face_value_unit}
                onChange={handleInputChange}
                selection
              />
            </Form.Group>
            <Form.Field>
              <Form.Input
                className='router-section-input'
                label={t('redemption.edit.credit_yyc')}
                value={yycPreview > 0 ? formatYYCValue(yycPreview) : '-'}
                readOnly
              />
            </Form.Field>
            <Form.Field>
              <Form.Input
                className='router-section-input'
                label={t('redemption.edit.count')}
                name='count'
                placeholder={t('redemption.edit.count_placeholder')}
                onChange={handleInputChange}
                value={count}
                autoComplete='off'
                type='number'
                min='1'
              />
            </Form.Field>
          </Form>
        </Card.Content>
      </Card>
    </div>
  );
};

export default EditRedemption;
