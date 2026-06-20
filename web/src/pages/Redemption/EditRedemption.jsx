import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { API, downloadTextAsFile, showError, showSuccess } from '../../helpers';
import {
  buildBillingCurrencyIndex,
  buildFaceValueUnitOptions,
} from '../../helpers/billing';
import { formatCreditAmount } from '../../helpers/render';
import UnitDropdown from '../../components/UnitDropdown';
import {
  AppButton,
  AppCompact,
  AppDetailSection,
  AppField,
  AppFilterHeader,
  AppFormActions,
  AppFormRow,
  AppInput,
  AppInputNumber,
  AppSelect,
  AppSpin,
} from '../../router-ui';

const YYC_UNIT = 'YYC';

const originInputs = {
  name: '',
  group_id: '',
  face_value_amount: '100000',
  face_value_unit: YYC_UNIT,
  code_validity_days: 0,
  credit_validity_days: 0,
  count: 1,
};

const toGroupOptions = (rows) =>
  (Array.isArray(rows) ? rows : []).map((item) => ({
    key: item.id,
    value: item.id,
    text: item.name || item.id,
  }));

const computeChargePreview = (amountValue, unitValue, currencyIndex) => {
  const amount = Number.parseFloat(`${amountValue ?? ''}`);
  if (!Number.isFinite(amount) || amount <= 0) {
    return 0;
  }
  const normalizedUnit = (unitValue || YYC_UNIT).toString().trim().toUpperCase();
  if (normalizedUnit === YYC_UNIT) {
    return Math.round(amount);
  }
  const currency = currencyIndex[normalizedUnit];
  const rate = Number(currency?.charge_rate || 0);
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
  const [unitOptions, setUnitOptions] = useState(buildFaceValueUnitOptions([]));
  const [currencyIndex, setCurrencyIndex] = useState(buildBillingCurrencyIndex([]));
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  const {
    name,
    group_id,
    face_value_amount,
    face_value_unit,
    code_validity_days,
    credit_validity_days,
    count,
  } = inputs;

  const chargePreview = useMemo(
    () => computeChargePreview(face_value_amount, face_value_unit, currencyIndex),
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
        setUnitOptions(buildFaceValueUnitOptions(nextCurrencies));
        setCurrencyIndex(buildBillingCurrencyIndex(nextCurrencies));
      } catch (error) {
        showError(error?.message || error);
      } finally {
        setLoading(false);
      }
    };
    loadOptions().then();
  }, [t]);

  const handleCancel = () => {
    navigate('/admin/redemption');
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
    localInputs.code_validity_days = Number.parseInt(`${localInputs.code_validity_days ?? ''}`, 10);
    localInputs.credit_validity_days = Number.parseInt(`${localInputs.credit_validity_days ?? ''}`, 10);
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
    if (!Number.isFinite(localInputs.code_validity_days) || localInputs.code_validity_days < 0) {
      showError(t('redemption.messages.code_validity_invalid'));
      return;
    }
    if (!Number.isFinite(localInputs.credit_validity_days) || localInputs.credit_validity_days < 0) {
      showError(t('redemption.messages.credit_validity_invalid'));
      return;
    }

    setSubmitting(true);
    try {
      const res = await API.post('/api/v1/admin/redemption/', {
        name: (localInputs.name || '').toString().trim(),
        group_id: localInputs.group_id,
        face_value_amount: localInputs.face_value_amount,
        face_value_unit: localInputs.face_value_unit,
        code_validity_days: localInputs.code_validity_days,
        credit_validity_days: localInputs.credit_validity_days,
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
        navigate('/admin/redemption');
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
      <AppFilterHeader
        breadcrumbs={[
          { key: 'workspace', label: t('header.admin_workspace') },
          { key: 'business', label: t('header.business_operation') },
          {
            key: 'redemption-list',
            label: t('header.redemption'),
            onClick: handleCancel,
          },
          {
            key: 'redemption-create',
            label: t('redemption.edit.title_create'),
            active: true,
          },
        ]}
        title={t('redemption.edit.title_create')}
        className='router-block-gap-sm'
        actions={
          <>
            <AppButton className='router-page-button' onClick={handleCancel} disabled={submitting}>
              {t('redemption.edit.buttons.cancel')}
            </AppButton>
            <AppButton
              className='router-page-button'
              color='blue'
              onClick={submit}
              loading={submitting}
              disabled={loading || submitting}
            >
              {t('redemption.edit.buttons.submit')}
            </AppButton>
          </>
        }
      />
      <div className='router-entity-detail-page'>
        <AppSpin spinning={loading}>
          <AppDetailSection title={t('common.basic_info')} bodyClassName='router-page-stack'>
            <AppFormRow>
              <AppField label={t('redemption.edit.name')} required>
                <AppInput
                  className='router-section-input'
                  name='name'
                  placeholder={t('redemption.edit.name_placeholder')}
                  onChange={handleInputChange}
                  value={name}
                  autoComplete='off'
                  required
                />
              </AppField>
            </AppFormRow>
            <AppFormRow>
              <AppField label={t('redemption.edit.group')} required>
                <AppSelect
                  className='router-section-input'
                  name='group_id'
                  placeholder={t('redemption.edit.group_placeholder')}
                  options={groupOptions}
                  value={group_id}
                  onChange={handleInputChange}
                  search
                  required
                />
              </AppField>
            </AppFormRow>
            <AppFormRow>
              <AppField label={t('redemption.edit.face_value_amount')}>
                <AppCompact className='router-section-input-with-unit' block>
                  <AppInputNumber
                    className='router-section-input router-section-input-with-unit-field'
                    name='face_value_amount'
                    placeholder={t('redemption.edit.face_value_amount_placeholder')}
                    onChange={handleInputChange}
                    value={face_value_amount}
                    step={face_value_unit === YYC_UNIT ? 1 : 0.01}
                    precision={face_value_unit === YYC_UNIT ? 0 : 2}
                    min={0}
                    fluid
                  />
                  <UnitDropdown
                    variant='inputUnit'
                    name='face_value_unit'
                    placeholder={t('redemption.edit.face_value_unit_placeholder')}
                    options={unitOptions}
                    value={face_value_unit}
                    onChange={handleInputChange}
                  />
                </AppCompact>
              </AppField>
            </AppFormRow>
            <AppFormRow>
              <AppField label={t('redemption.edit.credit_yyc')} readOnly>
                <AppInput
                  className='router-section-input'
                  value={chargePreview > 0 ? formatCreditAmount(chargePreview) : '-'}
                  readOnly
                />
              </AppField>
            </AppFormRow>
            <AppFormRow>
              <AppField label={t('redemption.edit.count')}>
                <AppInputNumber
                  className='router-section-input'
                  name='count'
                  placeholder={t('redemption.edit.count_placeholder')}
                  onChange={handleInputChange}
                  value={count}
                  min={1}
                  precision={0}
                  fluid
                />
              </AppField>
            </AppFormRow>
            <AppFormRow>
              <AppField label={t('redemption.edit.code_validity_days')}>
                <AppInputNumber
                  className='router-section-input'
                  name='code_validity_days'
                  placeholder={t('redemption.edit.code_validity_days_placeholder')}
                  onChange={handleInputChange}
                  value={code_validity_days}
                  min={0}
                  precision={0}
                  fluid
                />
              </AppField>
              <AppField label={t('redemption.edit.credit_validity_days')}>
                <AppInputNumber
                  className='router-section-input'
                  name='credit_validity_days'
                  placeholder={t('redemption.edit.credit_validity_days_placeholder')}
                  onChange={handleInputChange}
                  value={credit_validity_days}
                  min={0}
                  precision={0}
                  fluid
                />
              </AppField>
            </AppFormRow>
            <AppFormActions align='start'>
              <AppButton className='router-page-button' onClick={handleCancel} disabled={submitting}>
                {t('redemption.edit.buttons.cancel')}
              </AppButton>
              <AppButton
                className='router-page-button'
                color='blue'
                onClick={submit}
                loading={submitting}
                disabled={loading || submitting}
              >
                {t('redemption.edit.buttons.submit')}
              </AppButton>
            </AppFormActions>
          </AppDetailSection>
        </AppSpin>
      </div>
    </div>
  );
};

export default EditRedemption;
