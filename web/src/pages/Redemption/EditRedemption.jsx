import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { API, downloadTextAsFile, showError, showSuccess } from '../../helpers';
import {
  AppButton,
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

const originInputs = {
  name: '',
  entitlement_product_id: '',
  count: 1,
};

const EditRedemption = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [inputs, setInputs] = useState(originInputs);
  const [productOptions, setProductOptions] = useState([]);
  const [products, setProducts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  const {
    name,
    entitlement_product_id,
    count,
  } = inputs;
  const selectedProduct = useMemo(
    () => products.find((item) => item.id === entitlement_product_id) || null,
    [entitlement_product_id, products],
  );

  useEffect(() => {
    const loadOptions = async () => {
      setLoading(true);
      try {
        const [groupsRes, currenciesRes, productsRes] = await Promise.all([
          API.get('/api/v1/admin/groups', {
            params: {
              page: 1,
              page_size: 200,
            },
          }),
          API.get('/api/v1/admin/billing/currencies'),
          API.get('/api/v1/admin/entitlement/products', {
            params: { kind: 'balance', page: 1, page_size: 100 },
          }),
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
        const nextProducts = productsRes?.data?.data?.items || [];
        setProducts(nextProducts);
        setProductOptions(nextProducts.map((item) => ({
          key: item.id,
          value: item.id,
          text: item.name || item.id,
        })));
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
    if ((inputs.entitlement_product_id || '').trim() === '') {
      showError('请选择充值权益');
      return;
    }
    const localInputs = { ...inputs };
    localInputs.count = Number.parseInt(`${localInputs.count ?? ''}`, 10);
    if (!Number.isFinite(localInputs.count) || localInputs.count <= 0) {
      showError(t('redemption.messages.count_invalid'));
      return;
    }

    setSubmitting(true);
    try {
      const res = await API.post('/api/v1/admin/redemption/', {
        name: (localInputs.name || '').toString().trim(),
        entitlement_product_id: localInputs.entitlement_product_id,
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
          { key: 'business', label: t('header.operation') },
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
              <AppField label='绑定充值权益' required>
                <AppSelect
                  className='router-section-input'
                  name='entitlement_product_id'
                  placeholder='请选择充值权益'
                  options={productOptions}
                  value={entitlement_product_id}
                  onChange={handleInputChange}
                  search
                  required
                />
              </AppField>
            </AppFormRow>
            {selectedProduct ? (
              <AppFormRow>
                <AppField label='权益配置' readOnly>
                  <AppInput
                    className='router-section-input'
                    value={`${selectedProduct.name || '-'} / ${selectedProduct.quota_amount || 0} ${selectedProduct.quota_currency || 'YYC'} / ${selectedProduct.validity_days || 0} 天`}
                    readOnly
                  />
                </AppField>
              </AppFormRow>
            ) : null}
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
