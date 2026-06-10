import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  useLocation,
  useNavigate,
  useParams,
} from 'react-router-dom';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import UnitDropdown from '../../components/UnitDropdown';
import {
  billingInputValueToYYC,
  buildBillingUnitOptions,
  buildPublicDisplayCurrencyIndex,
  convertBillingInputValueUnit,
  loadPublicDisplayCurrencyCatalog,
  resolveBillingInputStep,
  resolveDefaultBillingUnit,
  yycToBillingInputValue,
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
    unlimited_quota: hasUnlimitedYYCLimit,
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
  const basicReadonly = isDetailMode && detailEditingSection !== 'basic';
  const modelsReadonly = isDetailMode && detailEditingSection !== 'models';
  const isEveryModelSelected = modelOptions.length > 0 && (
    allModelsSelected || inputs.models.length === modelOptions.length
  );
  const selectedModels = isEveryModelSelected ? allModelValues : inputs.models;

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
      data?.yyc_remain ?? normalizedData.remain_quota ?? 0
    ) || 0;
    normalizedData.used_quota = Number(
      data?.yyc_used ?? normalizedData.used_quota ?? 0
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
    setQuotaInputValue(
      yycToBillingInputValue(
        normalizedData.remain_quota,
        quotaDisplayUnit,
        billingCurrencyIndex
      )
    );
  }, [billingCurrencyIndex, quotaDisplayUnit]);

  const handleInputChange = (e, { name, value }) => {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  };

  const startDetailSectionEdit = useCallback((section) => {
    if (!isDetailMode) {
      return;
    }
    setDetailEditingSection(section);
  }, [isDetailMode]);

  const cancelDetailSectionEdit = useCallback(() => {
    setInputs(persistedInputs);
    setQuotaInputValue(
      yycToBillingInputValue(
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
      setInputs({ ...inputs, expired_time: timestamp2string(timestamp) });
    } else {
      setInputs({ ...inputs, expired_time: '' });
    }
  };

  const toggleUnlimitedYYCLimit = () => {
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
        let options = data.map((model) => {
          return {
            key: model,
            text: model,
            value: model,
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
        yycToBillingInputValue(inputs.remain_quota, nextUnit, nextIndex)
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
    const quotaYYC = billingInputValueToYYC(
      quotaInputValue,
      quotaDisplayUnit,
      billingCurrencyIndex
    );
    if (!Number.isFinite(quotaYYC) || quotaYYC < 0) {
      showError(t('token.edit.messages.quota_invalid'));
      return;
    }
    localInputs.remain_quota = quotaYYC;
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
      res = await API.put(`/api/v1/public/token/`, {
        ...localInputs,
        id: parseInt(tokenId),
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
        setQuotaInputValue(
          yycToBillingInputValue(
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
                <AppFormRow>
                  <AppField label={t('token.edit.expire_time')}>
                    <AppInput
                      className='router-section-input'
                      name='expired_time'
                      placeholder={t('token.edit.expire_time_placeholder')}
                      onChange={handleInputChange}
                      value={expired_time}
                      autoComplete='new-password'
                      type='datetime-local'
                      readOnly={false}
                    />
                  </AppField>
                </AppFormRow>
                <div className='router-token-expire-actions'>
                  <AppButton
                    className='router-inline-button'
                    type='button'
                    onClick={() => {
                      setExpiredTime(0, 0, 0, 0);
                    }}
                  >
                    {t('token.edit.buttons.never_expire')}
                  </AppButton>
                  <AppButton
                    className='router-inline-button'
                    type='button'
                    onClick={() => {
                      setExpiredTime(1, 0, 0, 0);
                    }}
                  >
                    {t('token.edit.buttons.expire_1_month')}
                  </AppButton>
                  <AppButton
                    className='router-inline-button'
                    type='button'
                    onClick={() => {
                      setExpiredTime(0, 1, 0, 0);
                    }}
                  >
                    {t('token.edit.buttons.expire_1_day')}
                  </AppButton>
                  <AppButton
                    className='router-inline-button'
                    type='button'
                    onClick={() => {
                      setExpiredTime(0, 0, 1, 0);
                    }}
                  >
                    {t('token.edit.buttons.expire_1_hour')}
                  </AppButton>
                  <AppButton
                    className='router-inline-button'
                    type='button'
                    onClick={() => {
                      setExpiredTime(0, 0, 0, 1);
                    }}
                  >
                    {t('token.edit.buttons.expire_1_minute')}
                  </AppButton>
                </div>
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
                        disabled={hasUnlimitedYYCLimit}
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
                      checked={hasUnlimitedYYCLimit}
                      onChange={() => {
                        toggleUnlimitedYYCLimit();
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
                  <AppFormRow>
                    <AppField label={t('token.edit.expire_time')}>
                      <AppInput
                        className='router-section-input'
                        name='expired_time'
                        placeholder={t('token.edit.expire_time_placeholder')}
                        onChange={handleInputChange}
                        value={expired_time}
                        autoComplete='new-password'
                        type='datetime-local'
                        readOnly={basicReadonly}
                      />
                    </AppField>
                  </AppFormRow>
                  {detailEditingSection === 'basic' ? (
                    <div className='router-token-expire-actions'>
                      <AppButton
                        className='router-inline-button'
                        type='button'
                        onClick={() => {
                          setExpiredTime(0, 0, 0, 0);
                        }}
                      >
                        {t('token.edit.buttons.never_expire')}
                      </AppButton>
                      <AppButton
                        className='router-inline-button'
                        type='button'
                        onClick={() => {
                          setExpiredTime(1, 0, 0, 0);
                        }}
                      >
                        {t('token.edit.buttons.expire_1_month')}
                      </AppButton>
                      <AppButton
                        className='router-inline-button'
                        type='button'
                        onClick={() => {
                          setExpiredTime(0, 1, 0, 0);
                        }}
                      >
                        {t('token.edit.buttons.expire_1_day')}
                      </AppButton>
                      <AppButton
                        className='router-inline-button'
                        type='button'
                        onClick={() => {
                          setExpiredTime(0, 0, 1, 0);
                        }}
                      >
                        {t('token.edit.buttons.expire_1_hour')}
                      </AppButton>
                      <AppButton
                        className='router-inline-button'
                        type='button'
                        onClick={() => {
                          setExpiredTime(0, 0, 0, 1);
                        }}
                      >
                        {t('token.edit.buttons.expire_1_minute')}
                      </AppButton>
                    </div>
                  ) : null}
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
                          disabled={hasUnlimitedYYCLimit || basicReadonly}
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
                        checked={hasUnlimitedYYCLimit}
                        disabled={basicReadonly}
                        onChange={() => {
                          toggleUnlimitedYYCLimit();
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
