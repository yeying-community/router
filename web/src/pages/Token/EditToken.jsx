import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  useLocation,
  useNavigate,
  useParams,
} from 'react-router-dom';
import {
  API,
  copy,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import UnitDropdown from '../../components/UnitDropdown';
import {
  billingInputValueToChargeAmount,
  buildBillingUnitOptions,
  buildPublicDisplayCurrencyIndex,
  convertBillingInputValueUnit,
  loadPublicDisplayCurrencyCatalog,
  resolveBillingInputStep,
  resolveDefaultBillingUnit,
  chargeAmountToBillingInputValue,
} from '../../helpers/billing';
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
  AppSwitch,
  AppTable,
  AppTabs,
  AppTag,
  AppTextarea,
} from '../../router-ui';

const EditToken = () => {
  const { t } = useTranslation();
  const location = useLocation();
  const params = useParams();
  const tokenId = params.id;
  const isCreateMode = tokenId === undefined;
  const isDetailMode = !isCreateMode;
  const returnPath = (() => {
    const from = location.state?.from;
    if (typeof from !== 'string') {
      return '';
    }
    const normalized = from.trim();
    return normalized.startsWith('/') ? normalized : '';
  })();
  const [loading, setLoading] = useState(isDetailMode);
  const [modelOptions, setModelOptions] = useState([]);
  const [allModelsSelected, setAllModelsSelected] = useState(isCreateMode);
  const [modelKeyword, setModelKeyword] = useState('');
  const [detailEditingSection, setDetailEditingSection] = useState('');
  const [activeDetailTab, setActiveDetailTab] = useState('basic');
  const [createdToken, setCreatedToken] = useState(null);
  const [expireTimeMode, setExpireTimeMode] = useState('custom');
  const [billingCurrencyIndex, setBillingCurrencyIndex] = useState(
    buildPublicDisplayCurrencyIndex([])
  );
  const [quotaDisplayUnit, setQuotaDisplayUnit] = useState('USD');
  const originInputs = {
    name: '',
    remain_quota: isDetailMode ? 0 : 500000,
    expired_time: '',
    unlimited_quota: false,
    models: [],
    subnet: '',
    status: 1,
    created_time: 0,
    updated_time: 0,
    key: '',
    used_quota: 0,
  };
  const [inputs, setInputs] = useState(originInputs);
  const [persistedInputs, setPersistedInputs] = useState(originInputs);
  const [quotaInputValue, setQuotaInputValue] = useState(`${originInputs.remain_quota}`);
  const {
    name,
    expired_time,
    unlimited_quota: hasUnlimitedLimitAmount,
  } = inputs;
  const navigate = useNavigate();
  const allModelValues = modelOptions.map((option) => option.value);
  const filteredModelOptions = modelOptions.filter((option) =>
    option.value.toLowerCase().includes(modelKeyword.trim().toLowerCase())
  );
  const quotaUnitOptions = useMemo(
    () => buildBillingUnitOptions(billingCurrencyIndex),
    [billingCurrencyIndex]
  );
  const expireTimeModeOptions = useMemo(
    () => [
      {
        key: 'custom',
        value: 'custom',
        text: t('token.edit.expire_time_options.custom'),
      },
      {
        key: 'never',
        value: 'never',
        text: t('token.edit.buttons.never_expire'),
      },
      {
        key: '1_month',
        value: '1_month',
        text: t('token.edit.buttons.expire_1_month'),
      },
      {
        key: '1_day',
        value: '1_day',
        text: t('token.edit.buttons.expire_1_day'),
      },
      {
        key: '1_hour',
        value: '1_hour',
        text: t('token.edit.buttons.expire_1_hour'),
      },
      {
        key: '1_minute',
        value: '1_minute',
        text: t('token.edit.buttons.expire_1_minute'),
      },
    ],
    [t]
  );
  const basicReadonly = isDetailMode && detailEditingSection !== 'basic';
  const modelsReadonly = isDetailMode && detailEditingSection !== 'models';
  const isEveryModelSelected = modelOptions.length > 0 && (
    allModelsSelected || inputs.models.length === modelOptions.length
  );
  const selectedModels = isEveryModelSelected ? allModelValues : inputs.models;

  const formatEntitlementSourceLabel = useCallback(
    (source) => {
      const sourceName = (source?.source_name || '').toString().trim();
      const sourceType = (source?.source_type || '').toString().trim();
      let typeLabel = '';
      if (sourceType === 'package') {
        typeLabel = t('token.edit.model_source_package');
      } else if (sourceType === 'topup') {
        typeLabel = t('token.edit.model_source_topup');
      } else if (sourceType === 'redemption') {
        typeLabel = t('token.edit.model_source_redemption');
      } else {
        typeLabel = t('token.edit.model_source_legacy');
      }
      return sourceName ? `${typeLabel}: ${sourceName}` : typeLabel;
    },
    [t]
  );

  const renderStatus = (status) => {
    switch (status) {
      case 1:
        return (
          <AppTag color='green' className='router-tag'>
            {t('token.table.status_enabled')}
          </AppTag>
        );
      case 2:
        return (
          <AppTag color='red' className='router-tag'>
            {t('token.table.status_disabled')}
          </AppTag>
        );
      case 3:
        return (
          <AppTag color='yellow' className='router-tag'>
            {t('token.table.status_expired')}
          </AppTag>
        );
      case 4:
        return (
          <AppTag color='grey' className='router-tag'>
            {t('token.table.status_depleted')}
          </AppTag>
        );
      default:
        return (
          <AppTag color='black' className='router-tag'>
            {t('token.table.status_unknown')}
          </AppTag>
        );
    }
  };

  const renderFullToken = (key) => {
    const raw = typeof key === 'string' ? key.trim() : '';
    if (raw === '') {
      return '-';
    }
    return raw.startsWith('sk-') ? raw : `sk-${raw}`;
  };

  const syncTokenState = useCallback((data) => {
    const normalizedData = {
      ...originInputs,
      ...data,
    };
    normalizedData.remain_quota = Number(
      data?.remaining_amount ?? normalizedData.remain_quota ?? 0
    ) || 0;
    normalizedData.used_quota = Number(
      data?.used_amount ?? normalizedData.used_quota ?? 0
    ) || 0;
    if (normalizedData.expired_time !== -1) {
      normalizedData.expired_time = timestamp2string(normalizedData.expired_time);
    } else {
      normalizedData.expired_time = '';
    }
    if (
      normalizedData.models === '' ||
      normalizedData.models === null ||
      normalizedData.models === undefined
    ) {
      normalizedData.models = [];
      setAllModelsSelected(true);
    } else {
      normalizedData.models = normalizedData.models.split(',');
      setAllModelsSelected(false);
    }
    setInputs(normalizedData);
    setPersistedInputs(normalizedData);
    setExpireTimeMode('custom');
    setQuotaInputValue(
      chargeAmountToBillingInputValue(
        normalizedData.remain_quota,
        quotaDisplayUnit,
        billingCurrencyIndex
      )
    );
  }, [billingCurrencyIndex, quotaDisplayUnit]);

  const handleInputChange = (e, { name, value }) => {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  };

  const formatDateTimeLocalInputValue = (value) => {
    const normalizedValue = (value || '').toString().trim();
    if (normalizedValue === '') {
      return '';
    }
    return normalizedValue.replace(' ', 'T').slice(0, 16);
  };

  const startDetailSectionEdit = useCallback((section) => {
    if (!isDetailMode) {
      return;
    }
    setDetailEditingSection(section);
  }, [isDetailMode]);

  const cancelDetailSectionEdit = useCallback(() => {
    setInputs(persistedInputs);
    setExpireTimeMode('custom');
    setQuotaInputValue(
      chargeAmountToBillingInputValue(
        persistedInputs.remain_quota,
        quotaDisplayUnit,
        billingCurrencyIndex
      )
    );
    setAllModelsSelected(
      Array.isArray(persistedInputs.models)
        ? persistedInputs.models.length === 0
        : true
    );
    setDetailEditingSection('');
  }, [billingCurrencyIndex, persistedInputs, quotaDisplayUnit]);

  const handleCancel = () => {
    if (isCreateMode) {
      navigate('/token');
      return;
    }
    if (detailEditingSection !== '') {
      cancelDetailSectionEdit();
      return;
    }
    if (returnPath !== '') {
      navigate(-1);
      return;
    }
    navigate('/token');
  };

  const handleBack = () => {
    if (returnPath !== '') {
      navigate(-1);
      return;
    }
    navigate('/token');
  };

  const setExpiredTime = (month, day, hour, minute) => {
    let now = new Date();
    let timestamp = now.getTime() / 1000;
    let seconds = month * 30 * 24 * 60 * 60;
    seconds += day * 24 * 60 * 60;
    seconds += hour * 60 * 60;
    seconds += minute * 60;
    if (seconds !== 0) {
      timestamp += seconds;
      setInputs((prev) => ({ ...prev, expired_time: timestamp2string(timestamp) }));
    } else {
      setInputs((prev) => ({ ...prev, expired_time: '' }));
    }
  };

  const handleExpiredTimeChange = (event, data) => {
    handleInputChange(event, data);
    setExpireTimeMode('custom');
  };

  const handleExpireTimeModeChange = (_, { value }) => {
    const nextMode = (value || 'custom').toString();
    setExpireTimeMode(nextMode);
    switch (nextMode) {
      case 'never':
        setExpiredTime(0, 0, 0, 0);
        break;
      case '1_month':
        setExpiredTime(1, 0, 0, 0);
        break;
      case '1_day':
        setExpiredTime(0, 1, 0, 0);
        break;
      case '1_hour':
        setExpiredTime(0, 0, 1, 0);
        break;
      case '1_minute':
        setExpiredTime(0, 0, 0, 1);
        break;
      default:
        break;
    }
  };

  const toggleUnlimitedLimitAmount = () => {
    setInputs((prev) => ({
      ...prev,
      unlimited_quota: !prev.unlimited_quota,
    }));
  };

  const handleQuotaInputChange = (_, { value }) => {
    setQuotaInputValue(value ?? '0');
  };

  const handleQuotaUnitChange = (_, { value }) => {
    const nextUnit = (value || 'YYC').toString().trim().toUpperCase();
    setQuotaInputValue((currentValue) =>
      convertBillingInputValueUnit(
        currentValue,
        quotaDisplayUnit,
        nextUnit,
        billingCurrencyIndex
      )
    );
    setQuotaDisplayUnit(nextUnit);
  };

  const loadToken = useCallback(async () => {
    try {
      let res = await API.get(`/api/v1/public/token/${tokenId}`);
      const { success, message, data } = res.data || {};
      if (success && data) {
        syncTokenState(data);
      } else {
        showError(message || 'Failed to load token');
      }
    } catch (error) {
      showError(error.message || 'Network error');
    }
    setLoading(false);
  }, [syncTokenState, tokenId]);

  const loadAvailableModels = useCallback(async () => {
    try {
      let res = await API.get(`/api/v1/public/user/available_models`);
      const { success, message, data } = res.data || {};
      if (success && data) {
        const sourceItemsByModel = new Map(
          (Array.isArray(res.data?.items) ? res.data.items : []).map((item) => [
            (item?.model || '').toString().trim(),
            Array.isArray(item?.sources) ? item.sources : [],
          ])
        );
        let options = data.map((model) => {
          const normalizedModel = (model || '').toString().trim();
          return {
            key: normalizedModel,
            text: normalizedModel,
            value: normalizedModel,
            sources: sourceItemsByModel.get(normalizedModel) || [],
          };
        });
        setModelOptions(options);
        if (isCreateMode) {
          setAllModelsSelected(true);
          setInputs((prev) => ({
            ...prev,
            models: options.map((option) => option.value),
          }));
        }
      } else {
        showError(message || 'Failed to load models');
      }
    } catch (error) {
      showError(error.message || 'Network error');
    }
  }, [isCreateMode]);

  useEffect(() => {
    if (isDetailMode) {
      loadToken().catch((error) => {
        showError(error.message || 'Failed to load token');
        setLoading(false);
      });
    }
    loadAvailableModels().catch((error) => {
      showError(error.message || 'Failed to load models');
    });
  }, [isDetailMode, loadAvailableModels, loadToken]);

  useEffect(() => {
    let disposed = false;
    loadPublicDisplayCurrencyCatalog().then(({ currencyIndex: nextIndex }) => {
      if (disposed) {
        return;
      }
      const nextUnit = resolveDefaultBillingUnit(nextIndex);
      setBillingCurrencyIndex(nextIndex);
      setQuotaDisplayUnit(nextUnit);
      setQuotaInputValue(
        chargeAmountToBillingInputValue(inputs.remain_quota, nextUnit, nextIndex)
      );
    });
    return () => {
      disposed = true;
    };
  }, []);

  const submit = async () => {
    if (isCreateMode && inputs.name.trim() === '') {
      showError(t('token.edit.messages.name_required'));
      return;
    }
    const localInputs = { ...inputs };
    const quotaChargeAmount = billingInputValueToChargeAmount(
      quotaInputValue,
      quotaDisplayUnit,
      billingCurrencyIndex
    );
    if (!Number.isFinite(quotaChargeAmount) || quotaChargeAmount < 0) {
      showError(t('token.edit.messages.quota_invalid'));
      return;
    }
    localInputs.remain_quota = quotaChargeAmount;
    if (localInputs.expired_time) {
      let time = Date.parse(localInputs.expired_time);
      if (isNaN(time)) {
        showError(t('token.edit.messages.expire_time_invalid'));
        return;
      }
      localInputs.expired_time = Math.ceil(time / 1000);
    } else {
      localInputs.expired_time = -1;
    }
    if (!isEveryModelSelected && localInputs.models.length === 0 && modelOptions.length > 0) {
      showError(t('token.edit.messages.models_required'));
      return;
    }
    localInputs.models = isEveryModelSelected ? '' : localInputs.models.join(',');
    let res;
    if (isDetailMode) {
      const normalizedTokenId = (tokenId || '').toString().trim();
      if (normalizedTokenId === '') {
        showError(t('token.edit.messages.id_required'));
        return;
      }
      res = await API.put(`/api/v1/public/token/`, {
        ...localInputs,
        id: normalizedTokenId,
      });
    } else {
      res = await API.post(`/api/v1/public/token/`, localInputs);
    }
    const { success, message, data } = res.data;
    if (success) {
      if (isDetailMode) {
        showSuccess(t('token.edit.messages.update_success'));
        syncTokenState(data || {
          ...inputs,
          id: tokenId,
          models: localInputs.models,
          expired_time: localInputs.expired_time,
        });
        setDetailEditingSection('');
      } else {
        showSuccess(t('token.edit.messages.create_success'));
        setCreatedToken(data || null);
        setInputs(originInputs);
        setExpireTimeMode('custom');
        setQuotaInputValue(
          chargeAmountToBillingInputValue(
            originInputs.remain_quota,
            quotaDisplayUnit,
            billingCurrencyIndex
          )
        );
      }
    } else {
      showError(message);
    }
  };

  const toggleAllModels = (_, { checked }) => {
    const normalizedChecked = !!checked;
    setAllModelsSelected(normalizedChecked);
    setInputs((prev) => ({
      ...prev,
      models: normalizedChecked ? allModelValues : [],
    }));
  };

  const toggleModel = (modelName, checked) => {
    const selected = new Set(inputs.models);
    if (checked) {
      selected.add(modelName);
    } else {
      selected.delete(modelName);
    }
    const nextModels = allModelValues.filter((value) => selected.has(value));
    setAllModelsSelected(nextModels.length === allModelValues.length && allModelValues.length > 0);
    setInputs((prev) => ({
      ...prev,
      models: nextModels,
    }));
  };

  const handleModelKeywordChange = (_, { value }) => {
    setModelKeyword(value || '');
  };
  const modelTableRowSelection = {
    selectedRowKeys: selectedModels,
    columnWidth: 72,
    columnTitle: t('token.edit.models_select_all'),
    getTitleCheckboxProps: () => ({
      checked: isEveryModelSelected,
      disabled: false,
    }),
    getCheckboxProps: () => ({
      disabled: false,
    }),
    onSelect: (record, selected) => {
      toggleModel(record.value, selected);
    },
    onSelectAll: (selected) => {
      toggleAllModels(null, { checked: selected });
    },
  };

  const renderModelTable = (readonly = false) => (
    <div className='router-token-model-table-wrap'>
      <AppTable
        className='router-list-table router-table-fit-page router-token-model-table'
        pagination={false}
        rowKey='value'
        dataSource={filteredModelOptions}
        rowSelection={
          readonly
            ? {
                ...modelTableRowSelection,
                getTitleCheckboxProps: () => ({
                  checked: isEveryModelSelected,
                  disabled: true,
                }),
                getCheckboxProps: () => ({
                  disabled: true,
                }),
              }
            : modelTableRowSelection
        }
        locale={{
          emptyText:
            modelOptions.length === 0
              ? t('token.edit.models_table_empty')
              : t('token.edit.models_search_empty'),
        }}
        columns={[
          {
            title: t('token.edit.models_table_name'),
            dataIndex: 'text',
            key: 'text',
            render: (value) => (
              <span className='router-monospace-value'>{value}</span>
            ),
          },
          {
            title: t('token.edit.models_table_sources'),
            dataIndex: 'sources',
            key: 'sources',
            render: (sources) => {
              const normalizedSources = Array.isArray(sources) ? sources : [];
              if (normalizedSources.length === 0) {
                return <span className='router-text-muted'>-</span>;
              }
              return (
                <div className='router-token-model-source-list'>
                  {normalizedSources.slice(0, 3).map((source, index) => (
                    <AppTag
                      key={[
                        source?.source_type || 'source',
                        source?.source_id || source?.group_id || index,
                      ].join('-')}
                      className='router-tag'
                    >
                      {formatEntitlementSourceLabel(source)}
                    </AppTag>
                  ))}
                  {normalizedSources.length > 3 ? (
                    <AppTag className='router-tag'>
                      +{normalizedSources.length - 3}
                    </AppTag>
                  ) : null}
                </div>
              );
            },
          },
        ]}
      />
    </div>
  );

  return (
    <div className='dashboard-container'>
      {isDetailMode ? (
        <AppFilterHeader
          breadcrumbs={[
            { key: 'workspace', label: t('header.user_workspace') },
            { key: 'mine', label: t('header.mine') },
            {
              key: 'token-list',
              label: t('header.token'),
              onClick: handleBack,
            },
            {
              key: 'token-current',
              label: inputs.name || tokenId,
              active: true,
            },
          ]}
          title={t('token.detail.title')}
        />
      ) : (
        <AppFilterHeader
          breadcrumbs={[
            { key: 'workspace', label: t('header.user_workspace') },
            { key: 'mine', label: t('header.mine') },
            {
              key: 'token-list',
              label: t('header.token'),
              onClick: handleCancel,
            },
            {
              key: 'token-create',
              label: t('common.add'),
              active: true,
            },
          ]}
          title={t('token.edit.title_create')}
          className='router-block-gap-sm'
          actions={
            <AppButton
              className='router-page-button'
              onClick={handleCancel}
            >
              {t('token.edit.buttons.cancel')}
            </AppButton>
          }
          end={
            createdToken ? null :
            <AppButton className='router-page-button' color='blue' onClick={submit}>
              {t('token.edit.buttons.submit')}
            </AppButton>
          }
        />
      )}
      {isCreateMode && createdToken ? (
            <div className='router-page-stack'>
              <AppDetailSection title='令牌已创建'>
                <div className='router-section-message'>
                  令牌只会在创建成功后显示一次，请现在保存到你的客户端或密钥管理工具中。离开当前页面后，系统不会再次展示完整令牌。
                </div>
                <AppFormRow className='router-token-basic-info-row'>
                  <AppField label={t('token.table.token')} readOnly>
                    <AppTextarea
                      className='router-section-input'
                      value={renderFullToken(createdToken.key)}
                      readOnly
                      autoSize={{ minRows: 2, maxRows: 5 }}
                    />
                  </AppField>
                </AppFormRow>
                <AppFormActions>
                  <AppButton
                    className='router-page-button'
                    onClick={async () => {
                      const rawToken = renderFullToken(createdToken.key);
                      if (await copy(rawToken)) {
                        showSuccess(t('token.messages.copy_success'));
                        return;
                      }
                      showError(t('token.messages.copy_failed'));
                    }}
                  >
                    {t('token.copy_options.raw')}
                  </AppButton>
                  <AppButton
                    className='router-page-button'
                    color='blue'
                    onClick={() => navigate('/token')}
                  >
                    {t('common.back')}
                  </AppButton>
                </AppFormActions>
              </AppDetailSection>
            </div>
      ) : isCreateMode ? (
            <div className='router-page-stack'>
                <AppFormRow>
                  <AppField label={t('token.edit.name')} required={isCreateMode}>
                    <AppInput
                      className='router-section-input'
                      name='name'
                      placeholder={t('token.edit.name_placeholder')}
                      onChange={handleInputChange}
                      value={name}
                      autoComplete='new-password'
                      required={isCreateMode}
                      readOnly={false}
                    />
                  </AppField>
                </AppFormRow>
                <div>
                  <label>{t('token.edit.models')}</label>
                  <div className='router-section-message'>
                    {t('token.edit.models_table_notice')}
                  </div>
                  <AppInput
                    className='router-section-input router-token-model-search'
                    placeholder={t('token.edit.models_search_placeholder')}
                    value={modelKeyword}
                    onChange={handleModelKeywordChange}
                  />
                  {renderModelTable(false)}
                </div>
                <AppFormRow>
                  <AppField label={t('token.edit.ip_limit')}>
                    <AppInput
                      className='router-section-input'
                      name='subnet'
                      placeholder={t('token.edit.ip_limit_placeholder')}
                      onChange={handleInputChange}
                      value={inputs.subnet}
                      autoComplete='new-password'
                      readOnly={false}
                    />
                  </AppField>
                </AppFormRow>
                <AppFormRow className='router-token-expire-row'>
                  <AppField label={t('token.edit.expire_time')}>
                    <AppSelect
                      className='router-section-dropdown router-token-expire-mode-select'
                      options={expireTimeModeOptions}
                      value={expireTimeMode}
                      onChange={handleExpireTimeModeChange}
                    />
                  </AppField>
                  <AppField label={t('token.edit.expire_time_options.custom')}>
                    <AppInput
                      className='router-section-input'
                      name='expired_time'
                      placeholder={t('token.edit.expire_time_placeholder')}
                      onChange={handleExpiredTimeChange}
                      value={formatDateTimeLocalInputValue(expired_time)}
                      autoComplete='new-password'
                      type='datetime-local'
                      disabled={expireTimeMode !== 'custom'}
                      readOnly={false}
                    />
                  </AppField>
                </AppFormRow>
                <AppFormRow className='router-token-quota-row'>
                  <AppField
                    className='router-token-quota-field'
                    label={t('token.edit.quota')}
                    hint={t('token.edit.quota_notice')}
                  >
                    <AppCompact className='router-section-input-with-unit' block>
                      <AppInputNumber
                        className='router-section-input router-section-input-with-unit-field'
                        name='remain_quota'
                        placeholder={t('token.edit.quota_placeholder')}
                        onChange={handleQuotaInputChange}
                        value={quotaInputValue}
                        min={0}
                        step={resolveBillingInputStep(quotaDisplayUnit, billingCurrencyIndex)}
                        precision={6}
                        fluid
                        disabled={hasUnlimitedLimitAmount}
                      />
                      <UnitDropdown
                        variant='inputUnit'
                        options={quotaUnitOptions}
                        value={quotaDisplayUnit}
                        onChange={handleQuotaUnitChange}
                        aria-label={t('token.edit.quota')}
                      />
                    </AppCompact>
                  </AppField>
                  <AppField className='router-token-unlimited-field' label={t('token.edit.buttons.unlimited_quota')}>
                    <AppSwitch
                      checked={hasUnlimitedLimitAmount}
                      onChange={() => {
                        toggleUnlimitedLimitAmount();
                      }}
                    />
                  </AppField>
                </AppFormRow>
              </div>
      ) : (
        <div className='router-tab-detail-page router-entity-detail-page'>
          <div className='router-entity-detail-tabs router-block-gap-sm'>
            <AppTabs
              className='router-detail-tab-menu'
              activeKey={activeDetailTab}
              onChange={setActiveDetailTab}
              items={[
                { key: 'basic', label: t('common.basic_info') },
                { key: 'models', label: t('token.detail.sections.models') },
              ]}
            />
          </div>
          <div className='router-page-stack'>
            {activeDetailTab === 'basic' ? (
              <AppDetailSection
                title={t('common.basic_info')}
                headerStart={renderStatus(Number(inputs.status || 0))}
                headerEnd={
                  detailEditingSection === 'basic' ? (
                    <>
                      <AppButton className='router-page-button' onClick={cancelDetailSectionEdit}>
                        {t('token.edit.buttons.cancel')}
                      </AppButton>
                      <AppButton className='router-page-button' color='blue' onClick={submit}>
                        {t('token.edit.buttons.submit')}
                      </AppButton>
                    </>
                  ) : (
                    <AppButton
                      className='router-page-button'
                      color='blue'
                      onClick={() => startDetailSectionEdit('basic')}
                      disabled={detailEditingSection !== ''}
                    >
                      {t('token.buttons.edit')}
                    </AppButton>
                  )
                }
                bodyClassName='router-page-stack'
              >
                  <AppFormRow className='router-token-basic-info-row'>
                    <AppField label={t('token.edit.name')}>
                      <AppInput
                        className='router-section-input'
                        name='name'
                        placeholder={t('token.edit.name_placeholder')}
                        onChange={handleInputChange}
                        value={name}
                        autoComplete='new-password'
                        readOnly={basicReadonly}
                      />
                    </AppField>
                  </AppFormRow>
                  <AppFormRow className='router-token-basic-info-row'>
                    <AppField label={t('token.table.created_time')} readOnly>
                      <AppInput
                        className='router-section-input'
                        value={
                          inputs.created_time
                            ? timestamp2string(inputs.created_time)
                            : ''
                        }
                        readOnly
                      />
                    </AppField>
                  </AppFormRow>
                  <AppFormRow className='router-token-basic-info-row'>
                    <AppField label={t('token.table.updated_time')} readOnly>
                      <AppInput
                        className='router-section-input'
                        value={
                          inputs.updated_time || inputs.created_time
                            ? timestamp2string(inputs.updated_time || inputs.created_time)
                            : ''
                        }
                        readOnly
                      />
                    </AppField>
                  </AppFormRow>
                  <AppFormRow>
                    <AppField label={t('token.edit.ip_limit')}>
                      <AppInput
                        className='router-section-input'
                        name='subnet'
                        placeholder={t('token.edit.ip_limit_placeholder')}
                        onChange={handleInputChange}
                        value={inputs.subnet}
                        autoComplete='new-password'
                        readOnly={basicReadonly}
                      />
                    </AppField>
                  </AppFormRow>
                  <AppFormRow className='router-token-expire-row'>
                    <AppField label={t('token.edit.expire_time')}>
                      <AppSelect
                        className='router-section-dropdown router-token-expire-mode-select'
                        options={expireTimeModeOptions}
                        value={expireTimeMode}
                        onChange={handleExpireTimeModeChange}
                        disabled={basicReadonly}
                      />
                    </AppField>
                    <AppField label={t('token.edit.expire_time_options.custom')}>
                      <AppInput
                        className='router-section-input'
                        name='expired_time'
                        placeholder={t('token.edit.expire_time_placeholder')}
                        onChange={handleExpiredTimeChange}
                        value={formatDateTimeLocalInputValue(expired_time)}
                        autoComplete='new-password'
                        type='datetime-local'
                        disabled={basicReadonly || expireTimeMode !== 'custom'}
                        readOnly={basicReadonly}
                      />
                    </AppField>
                  </AppFormRow>
                  <AppFormRow className='router-token-quota-row'>
                    <AppField
                      className='router-token-quota-field'
                      label={t('token.edit.quota')}
                      hint={t('token.edit.quota_notice')}
                    >
                      <AppCompact className='router-section-input-with-unit' block>
                        <AppInputNumber
                          className='router-section-input router-section-input-with-unit-field'
                          name='remain_quota'
                          placeholder={t('token.edit.quota_placeholder')}
                          onChange={handleQuotaInputChange}
                          value={quotaInputValue}
                          min={0}
                          step={resolveBillingInputStep(quotaDisplayUnit, billingCurrencyIndex)}
                          precision={6}
                          fluid
                          disabled={hasUnlimitedLimitAmount || basicReadonly}
                        />
                        <UnitDropdown
                          variant='inputUnit'
                          options={quotaUnitOptions}
                          value={quotaDisplayUnit}
                          onChange={handleQuotaUnitChange}
                          disabled={basicReadonly}
                          aria-label={t('token.edit.quota')}
                        />
                      </AppCompact>
                    </AppField>
                    <AppField className='router-token-unlimited-field' label={t('token.edit.buttons.unlimited_quota')}>
                      <AppSwitch
                        checked={hasUnlimitedLimitAmount}
                        disabled={basicReadonly}
                        onChange={() => {
                          toggleUnlimitedLimitAmount();
                        }}
                      />
                    </AppField>
                  </AppFormRow>
              </AppDetailSection>
            ) : null}
            {activeDetailTab === 'models' ? (
            <AppDetailSection
                title={t('token.detail.sections.models')}
                headerEnd={
                  detailEditingSection === 'models' ? (
                    <>
                      <AppButton className='router-page-button' onClick={cancelDetailSectionEdit}>
                        {t('token.edit.buttons.cancel')}
                      </AppButton>
                      <AppButton className='router-page-button' color='blue' onClick={submit}>
                        {t('token.edit.buttons.submit')}
                      </AppButton>
                    </>
                  ) : (
                    <AppButton
                      className='router-page-button'
                      color='blue'
                      onClick={() => startDetailSectionEdit('models')}
                      disabled={detailEditingSection !== ''}
                    >
                      {t('token.buttons.edit')}
                    </AppButton>
                  )
                }
                bodyClassName='router-page-stack'
              >
                  <div className='router-section-message'>
                    {t('token.edit.models_table_notice')}
                  </div>
                  <AppInput
                    className='router-section-input router-token-model-search'
                    placeholder={t('token.edit.models_search_placeholder')}
                    value={modelKeyword}
                    onChange={handleModelKeywordChange}
                  />
                  {renderModelTable(modelsReadonly)}
            </AppDetailSection>
            ) : null}
          </div>
        </div>
      )}
    </div>
  );
};

export default EditToken;
