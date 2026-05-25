import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
} from '../helpers';
import {
  applyBillingInputValues,
  buildBillingCurrencyIndex,
  buildBillingUnitOptions,
  convertBillingInputValueUnit,
  createBillingUnitState,
  BILLING_OPTION_SETTING_KEYS,
  billingInputValueToYYC,
  resolveDefaultBillingUnit,
  resolveBillingInputStep,
} from '../helpers/billing';
import UnitDropdown from './UnitDropdown';
import {
  AppAlert,
  AppButton,
  AppDivider,
  AppField,
  AppFilterHeader,
  AppFormActions,
  AppFormRow,
  AppInput,
  AppInputNumber,
  AppSelect,
  AppSpin,
  AppSwitch,
} from '../router-ui';

const normalizeOptionValue = (value, fallback = '') => {
  if (value === null || value === undefined) {
    return fallback;
  }
  return `${value}`;
};

const formatPlanNumber = (value) => {
  const numeric = Number(value || 0);
  if (!Number.isFinite(numeric)) {
    return '0';
  }
  if (Math.abs(numeric - Math.round(numeric)) < 0.000001) {
    return `${Math.round(numeric)}`;
  }
  return numeric.toFixed(6).replace(/\.?0+$/, '');
};

const BALANCE_OPTION_KEYS = {
  newUserRewardPlan: 'NewUserRewardTopupPlanID',
  defaultGroup: 'DefaultUserGroup',
  inviterRewardPlan: 'InviterRewardTopupPlanID',
  balanceReminderThreshold: 'QuotaRemindThreshold',
  preConsumedAmount: 'PreConsumedQuota',
};

const OperationSetting = ({ section = '' }) => {
  const { t } = useTranslation();
  const now = new Date();
  const [inputs, setInputs] = useState({
    [BALANCE_OPTION_KEYS.newUserRewardPlan]: '',
    [BALANCE_OPTION_KEYS.defaultGroup]: '',
    [BALANCE_OPTION_KEYS.inviterRewardPlan]: '',
    [BALANCE_OPTION_KEYS.balanceReminderThreshold]: 0,
    [BALANCE_OPTION_KEYS.preConsumedAmount]: 0,
    AutomaticDisableChannelEnabled: '',
    AutomaticEnableChannelEnabled: '',
    ChannelDisableThreshold: 0,
    LogConsumeEnabled: '',
    RetryTimes: 0,
    ChannelBillingAutoRefreshEnabled: 'true',
    ChannelBillingAutoRefreshIntervalSeconds: 1800,
    ChannelBillingAutoRefreshLastRunAt: 0,
  });
  const [originInputs, setOriginInputs] = useState({});
  const [groupOptions, setGroupOptions] = useState([]);
  const [topupPlanOptions, setTopupPlanOptions] = useState([]);
  const [topupPlanById, setTopupPlanById] = useState({});
  const [billingCurrencyIndex, setBillingCurrencyIndex] = useState(
    buildBillingCurrencyIndex([], { activeOnly: true })
  );
  const [billingCurrenciesReady, setBillingCurrenciesReady] = useState(false);
  const [billingUnits, setBillingUnits] = useState(createBillingUnitState('USD'));
  const [billingDisplayInitialized, setBillingDisplayInitialized] = useState(false);
  const [loading, setLoading] = useState(false);
  const [logCleanupTimestamp, setLogCleanupTimestamp] = useState(
    timestamp2string(now.getTime() / 1000 - 30 * 24 * 3600)
  ); // a month ago
  const normalizedSection = (section || '').trim().toLowerCase();
  const showAllSections =
    normalizedSection === '' || normalizedSection === 'all';
  const showConfigSection = normalizedSection === 'config';
  const showBalanceSection =
    showAllSections ||
    showConfigSection ||
    normalizedSection === 'quota' ||
    normalizedSection === 'balance';
  const sectionVisible = {
    balance: showBalanceSection,
    monitor: showAllSections || normalizedSection === 'monitor',
    retry: showAllSections || normalizedSection === 'retry',
    log: showAllSections || normalizedSection === 'log',
    general: showAllSections || showConfigSection || normalizedSection === 'general',
  };
  const sectionOrder = ['balance', 'monitor', 'retry', 'log', 'general'];
  const shouldRenderDividerAfter = (key) => {
    if (!showAllSections) {
      return false;
    }
    const index = sectionOrder.indexOf(key);
    if (index < 0) {
      return false;
    }
    return sectionOrder
      .slice(index + 1)
      .some((nextKey) => Boolean(sectionVisible[nextKey]));
  };

  const getOptions = async () => {
    const res = await API.get('/api/v1/admin/option/');
    const { success, message, data } = res.data;
    if (success) {
      let newInputs = {};
      data.forEach((item) => {
        if (item.value === '{}') {
          item.value = '';
        }
        newInputs[item.key] = item.value;
      });
      setBillingDisplayInitialized(false);
      setInputs(newInputs);
      setOriginInputs(newInputs);
    } else {
      showError(message);
    }
  };

  useEffect(() => {
    getOptions().then();
    loadGroups().then();
    loadTopupPlans().then();
    loadBillingCurrencies().then();
  }, []);

  const billingUnitOptions = useMemo(
    () => buildBillingUnitOptions(billingCurrencyIndex),
    [billingCurrencyIndex]
  );

  const loadGroups = async () => {
    try {
      const rows = [];
      let page = 1;
      while (page <= 50) {
        const res = await API.get('/api/v1/admin/groups', {
          params: {
            page,
            page_size: 100,
          },
        });
        const { success, message, data } = res.data || {};
        if (!success) {
          showError(message);
          return;
        }
        const pageItems = Array.isArray(data?.items) ? data.items : [];
        rows.push(...pageItems);
        const total = Number(data?.total || pageItems.length || 0);
        if (
          pageItems.length === 0 ||
          rows.length >= total ||
          pageItems.length < 100
        ) {
          break;
        }
        page += 1;
      }
      setGroupOptions(
        rows.map((group) => ({
          key: group.id,
          value: group.id,
          text: group.name || group.id,
        })),
      );
    } catch (error) {
      showError(error?.message || error);
    }
  };

  const loadTopupPlans = async () => {
    try {
      const res = await API.get('/api/v1/admin/topup/plans');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message);
        return;
      }
      const rows = (Array.isArray(data) ? data : [])
        .filter((item) => Boolean(item?.enabled))
        .map((item) => ({
          id: (item?.id || '').toString().trim(),
          name: (item?.name || '').toString().trim(),
          amount: Number(item?.amount || 0),
          amount_currency: (item?.amount_currency || '').toString().trim().toUpperCase(),
          quota_amount: Number(item?.quota_amount || 0),
          quota_currency: (item?.quota_currency || '').toString().trim().toUpperCase(),
        }))
        .filter((item) => item.id);
      const indexed = rows.reduce((result, item) => {
        result[item.id] = item;
        return result;
      }, {});
      setTopupPlanById(indexed);
      setTopupPlanOptions(
        rows.map((item) => ({
          key: item.id,
          value: item.id,
          text: `${formatPlanNumber(item.amount)} ${item.amount_currency} / ${formatPlanNumber(item.quota_amount)} ${item.quota_currency}`,
        }))
      );
    } catch (error) {
      showError(error?.message || error);
    }
  };

  const loadBillingCurrencies = async () => {
    try {
      const res = await API.get('/api/v1/admin/billing/currencies');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('setting.currency.catalog.messages.load_failed'));
        return;
      }
      const rows = (Array.isArray(data) ? data : [])
        .map((item) => ({
          ...item,
          minor_unit: Number(item?.minor_unit ?? 6),
          yyc_per_unit:
            item?.yyc_per_unit === 0 || item?.yyc_per_unit
              ? `${item.yyc_per_unit}`
              : '',
          status: Number(item?.status || 1),
          _isNew: false,
        }))
        .sort((a, b) => (a.code || '').localeCompare(b.code || ''));
      const nextCurrencyIndex = buildBillingCurrencyIndex(rows, {
        activeOnly: true,
      });
      const defaultBillingUnit = resolveDefaultBillingUnit(nextCurrencyIndex);
      setBillingCurrencyIndex(nextCurrencyIndex);
      setBillingUnits((prev) =>
        BILLING_OPTION_SETTING_KEYS.reduce((result, key) => {
          const currentUnit = (prev?.[key] || '').toString().trim().toUpperCase();
          result[key] =
            currentUnit && nextCurrencyIndex[currentUnit]
              ? currentUnit
              : defaultBillingUnit;
          return result;
        }, {})
      );
    } catch (error) {
      showError(error?.message || t('setting.currency.catalog.messages.load_failed'));
    } finally {
      setBillingCurrenciesReady(true);
    }
  };

  const updateOption = async (key, value) => {
    setLoading(true);
    let nextValue = value;
    let syncInputState = false;
    if (key.endsWith('Enabled')) {
      nextValue = inputs[key] === 'true' ? 'false' : 'true';
      syncInputState = true;
    }
    const res = await API.put('/api/v1/admin/option/', {
      key,
      value: nextValue,
    });
    const { success, message } = res.data;
    if (success) {
      setOriginInputs((prev) => ({ ...prev, [key]: normalizeOptionValue(nextValue) }));
      if (syncInputState) {
        setInputs((prev) => ({ ...prev, [key]: nextValue }));
      }
    } else {
      showError(message);
    }
    setLoading(false);
  };

  const handleInputChange = async (e, { name, value }) => {
    const normalizedValue = value ?? '';
    if (name.endsWith('Enabled')) {
      await updateOption(name, normalizedValue);
    } else {
      setInputs((inputs) => ({ ...inputs, [name]: normalizedValue }));
    }
  };

  const saveSectionConfig = async (sectionKey) => {
    switch (sectionKey) {
      case 'monitor':
        if (
          originInputs['ChannelDisableThreshold'] !==
          inputs.ChannelDisableThreshold
        ) {
          await updateOption(
            'ChannelDisableThreshold',
            inputs.ChannelDisableThreshold
          );
        }
        if (
          originInputs[BALANCE_OPTION_KEYS.balanceReminderThreshold] !==
          inputs[BALANCE_OPTION_KEYS.balanceReminderThreshold]
        ) {
          await updateOption(
            BALANCE_OPTION_KEYS.balanceReminderThreshold,
            inputs[BALANCE_OPTION_KEYS.balanceReminderThreshold]
          );
        }
        break;
      case 'balance':
        {
          const newUserRewardPlanID = normalizeOptionValue(
            inputs[BALANCE_OPTION_KEYS.newUserRewardPlan],
            '',
          ).trim();
          const inviterRewardPlanID = normalizeOptionValue(
            inputs[BALANCE_OPTION_KEYS.inviterRewardPlan],
            '',
          ).trim();
          if (newUserRewardPlanID && !topupPlanById[newUserRewardPlanID]) {
            showError(t('setting.operation.quota.messages.plan_invalid'));
            break;
          }
          if (inviterRewardPlanID && !topupPlanById[inviterRewardPlanID]) {
            showError(t('setting.operation.quota.messages.plan_invalid'));
            break;
          }
          const preConsumedYYC = billingInputValueToYYC(
            inputs[BALANCE_OPTION_KEYS.preConsumedAmount],
            billingUnits[BALANCE_OPTION_KEYS.preConsumedAmount],
            billingCurrencyIndex
          );
          if (
            !Number.isFinite(preConsumedYYC) ||
            preConsumedYYC < 0
          ) {
            showError(t('setting.operation.quota.messages.amount_invalid'));
            break;
          }
          if (
            normalizeOptionValue(originInputs[BALANCE_OPTION_KEYS.preConsumedAmount], '0') !==
            `${Math.trunc(preConsumedYYC)}`
          ) {
            await updateOption(
              BALANCE_OPTION_KEYS.preConsumedAmount,
              `${Math.trunc(preConsumedYYC)}`
            );
          }
          if (
            normalizeOptionValue(originInputs[BALANCE_OPTION_KEYS.newUserRewardPlan], '') !==
            newUserRewardPlanID
          ) {
            await updateOption(
              BALANCE_OPTION_KEYS.newUserRewardPlan,
              newUserRewardPlanID
            );
          }
          if (
            normalizeOptionValue(originInputs[BALANCE_OPTION_KEYS.inviterRewardPlan], '') !==
            inviterRewardPlanID
          ) {
            await updateOption(
              BALANCE_OPTION_KEYS.inviterRewardPlan,
              inviterRewardPlanID
            );
          }
        }
        if (
          originInputs[BALANCE_OPTION_KEYS.defaultGroup] !==
          inputs[BALANCE_OPTION_KEYS.defaultGroup]
        ) {
          await updateOption(
            BALANCE_OPTION_KEYS.defaultGroup,
            inputs[BALANCE_OPTION_KEYS.defaultGroup]
          );
        }
        break;
      case 'retry':
        {
          const retryLimit = Number(inputs.RetryTimes || 0) > 0 ? 1 : 0;
          if (
            normalizeOptionValue(originInputs.RetryTimes, '0') !==
            `${retryLimit}`
          ) {
            await updateOption('RetryTimes', `${retryLimit}`);
          }
        }
        break;
      case 'general':
        {
          const chatLink = normalizeOptionValue(inputs.ChatLink, '').trim();
          if (
            normalizeOptionValue(originInputs.ChatLink, '').trim() !==
            chatLink
          ) {
            await updateOption('ChatLink', chatLink);
          }
          const billingRefreshInterval = Math.trunc(
            Number(inputs.ChannelBillingAutoRefreshIntervalSeconds || 0)
          );
          if (
            !Number.isFinite(billingRefreshInterval) ||
            billingRefreshInterval < 60
          ) {
            showError(
              t('setting.operation.general.channel_billing_auto_refresh.interval_invalid')
            );
            break;
          }
          if (
            normalizeOptionValue(
              originInputs.ChannelBillingAutoRefreshIntervalSeconds,
              '1800'
            ) !== `${billingRefreshInterval}`
          ) {
            await updateOption(
              'ChannelBillingAutoRefreshIntervalSeconds',
              `${billingRefreshInterval}`
            );
          }
        }
        break;
      default:
        break;
    }
  };

  useEffect(() => {
    if (billingDisplayInitialized) {
      return;
    }
    if (!billingCurrenciesReady) {
      return;
    }
    if (Object.keys(originInputs || {}).length === 0) {
      return;
    }
    const defaultBillingUnit = resolveDefaultBillingUnit(billingCurrencyIndex);
    const nextBillingUnits = BILLING_OPTION_SETTING_KEYS.reduce((result, key) => {
      const currentUnit = (billingUnits?.[key] || '').toString().trim().toUpperCase();
      result[key] =
        currentUnit && billingCurrencyIndex[currentUnit]
          ? currentUnit
          : defaultBillingUnit;
      return result;
    }, {});
    setBillingUnits(nextBillingUnits);
    setInputs((prev) => ({
      ...prev,
      ...applyBillingInputValues(originInputs, nextBillingUnits, billingCurrencyIndex),
    }));
    setBillingDisplayInitialized(true);
  }, [
    billingCurrenciesReady,
    billingCurrencyIndex,
    originInputs,
    billingDisplayInitialized,
    billingUnits,
  ]);

  const renderBalanceInputField = (
    labelKey,
    optionKey,
    placeholderKey,
    descriptionKey = '',
  ) => (
    <AppField
      label={t(labelKey)}
      hint={descriptionKey ? t(descriptionKey) : ''}
    >
      <div className='router-section-input-with-unit'>
        <AppInputNumber
          className='router-section-input router-section-input-with-unit-field'
          value={inputs[optionKey] ?? '0'}
          min={0}
          step={resolveBillingInputStep(billingUnits[optionKey], billingCurrencyIndex)}
          placeholder={t(placeholderKey)}
          precision={6}
          fluid
          onChange={(e, { value }) => {
            setInputs((prev) => ({
              ...prev,
              [optionKey]: value ?? '0',
            }));
          }}
        />
        <UnitDropdown
          variant='inputUnit'
          options={billingUnitOptions}
          value={billingUnits[optionKey] || resolveDefaultBillingUnit(billingCurrencyIndex)}
          onChange={(_, { value }) => {
            const nextUnit = (value || 'YYC').toString().trim().toUpperCase();
            setInputs((prev) => ({
              ...prev,
              [optionKey]: convertBillingInputValueUnit(
                prev[optionKey],
                billingUnits[optionKey],
                nextUnit,
                billingCurrencyIndex
              ),
            }));
            setBillingUnits((prev) => ({
              ...prev,
              [optionKey]: nextUnit,
            }));
          }}
          aria-label={t(labelKey)}
        />
      </div>
    </AppField>
  );

  const renderTopupPlanField = (
    labelKey,
    optionKey,
    placeholderKey,
    descriptionKey,
  ) => (
    <AppField label={t(labelKey)} hint={t(descriptionKey)}>
      <AppSelect
        className='router-section-input'
        clearable
        search
        options={topupPlanOptions}
        name={optionKey}
        value={inputs[optionKey] || ''}
        onChange={handleInputChange}
        placeholder={t(placeholderKey)}
      />
    </AppField>
  );

  const deleteHistoryLogs = async () => {
    const res = await API.delete(
      `/api/v1/admin/log/?target_timestamp=${Date.parse(logCleanupTimestamp) / 1000}`
    );
    const { success, message, data } = res.data;
    if (success) {
      showSuccess(`${data} 条日志已清理！`);
      return;
    }
    showError('日志清理失败：' + message);
  };

  return (
    <AppSpin spinning={loading}>
      <div>
          {sectionVisible.balance ? (
            <>
              <AppFilterHeader
                title={t('setting.operation.quota.title')}
                titleClassName='router-ui-section-title'
                className='router-toolbar-compact'
              />
              <AppFormRow>
                {renderBalanceInputField(
                  'setting.operation.quota.pre_consume',
                  BALANCE_OPTION_KEYS.preConsumedAmount,
                  'setting.operation.quota.pre_consume_placeholder',
                  'setting.operation.quota.pre_consume_description'
                )}
                {renderTopupPlanField(
                  'setting.operation.quota.new_user_reward',
                  BALANCE_OPTION_KEYS.newUserRewardPlan,
                  'setting.operation.quota.reward_plan_placeholder',
                  'setting.operation.quota.new_user_reward_description'
                )}
                {renderTopupPlanField(
                  'setting.operation.quota.inviter_reward',
                  BALANCE_OPTION_KEYS.inviterRewardPlan,
                  'setting.operation.quota.reward_plan_placeholder',
                  'setting.operation.quota.inviter_reward_description'
                )}
              </AppFormRow>
              <AppFormRow>
                <AppField label={t('setting.operation.quota.default_group')}>
                  <AppSelect
                    className='router-section-input'
                    name={BALANCE_OPTION_KEYS.defaultGroup}
                    clearable
                    search
                    options={groupOptions}
                    onChange={handleInputChange}
                    value={inputs[BALANCE_OPTION_KEYS.defaultGroup] || ''}
                    placeholder={t('setting.operation.quota.default_group_placeholder')}
                  />
                </AppField>
              </AppFormRow>
              <AppFormActions align='start'>
                <AppButton
                  className='router-section-button'
                  onClick={() => {
                    saveSectionConfig('balance').then();
                  }}
                >
                  {t('setting.operation.quota.buttons.save')}
                </AppButton>
              </AppFormActions>
              {shouldRenderDividerAfter('balance') ? <AppDivider /> : null}
            </>
          ) : null}

          {sectionVisible.monitor ? (
            <>
              <AppFilterHeader
                title={t('setting.operation.monitor.title')}
                titleClassName='router-ui-section-title'
                className='router-toolbar-compact'
              />
              <AppFormRow>
                <AppField label={t('setting.operation.monitor.max_response_time')}>
                  <AppInputNumber
                    className='router-section-input'
                    name='ChannelDisableThreshold'
                    onChange={handleInputChange}
                    value={inputs.ChannelDisableThreshold}
                    min={0}
                    precision={0}
                    fluid
                    placeholder={t(
                      'setting.operation.monitor.max_response_time_placeholder'
                    )}
                  />
                </AppField>
                <AppField label={t('setting.operation.monitor.quota_reminder')}>
                  <AppInputNumber
                    className='router-section-input'
                    name={BALANCE_OPTION_KEYS.balanceReminderThreshold}
                    onChange={handleInputChange}
                    value={inputs[BALANCE_OPTION_KEYS.balanceReminderThreshold]}
                    min={0}
                    precision={0}
                    fluid
                    placeholder={t(
                      'setting.operation.monitor.quota_reminder_placeholder'
                    )}
                  />
                </AppField>
              </AppFormRow>
              <AppFormRow>
                <AppField label={t('setting.operation.monitor.auto_disable')}>
                  <AppSwitch
                    checked={inputs.AutomaticDisableChannelEnabled === 'true'}
                    onChange={() =>
                      handleInputChange(null, {
                        name: 'AutomaticDisableChannelEnabled',
                        value: inputs.AutomaticDisableChannelEnabled === 'true'
                          ? 'false'
                          : 'true',
                      })
                    }
                  />
                </AppField>
                <AppField label={t('setting.operation.monitor.auto_enable')}>
                  <AppSwitch
                    checked={inputs.AutomaticEnableChannelEnabled === 'true'}
                    onChange={() =>
                      handleInputChange(null, {
                        name: 'AutomaticEnableChannelEnabled',
                        value: inputs.AutomaticEnableChannelEnabled === 'true'
                          ? 'false'
                          : 'true',
                      })
                    }
                  />
                </AppField>
              </AppFormRow>
              <AppFormActions align='start'>
                <AppButton
                  className='router-section-button'
                  onClick={() => {
                    saveSectionConfig('monitor').then();
                  }}
                >
                  {t('setting.operation.monitor.buttons.save')}
                </AppButton>
              </AppFormActions>
              {shouldRenderDividerAfter('monitor') ? <AppDivider /> : null}
            </>
          ) : null}

          {sectionVisible.retry ? (
            <>
              <AppFilterHeader
                title={t('setting.operation.retry.title')}
                titleClassName='router-ui-section-title'
                className='router-toolbar-compact'
              />
              <AppAlert
                className='router-section-message'
                type='info'
                showIcon
                title={t('setting.operation.retry.description_title')}
                description={
                  <>
                    <p>{t('setting.operation.retry.description')}</p>
                    <p>{t('setting.operation.retry.description_effective')}</p>
                    <p>{t('setting.operation.retry.description_disabled')}</p>
                  </>
                }
              />
              <AppFormRow>
                <AppField label={t('setting.operation.retry.limit')}>
                  <AppSelect
                    className='router-section-input'
                    name='RetryTimes'
                    onChange={handleInputChange}
                    value={inputs.RetryTimes}
                    placeholder={t('setting.operation.retry.limit_placeholder')}
                    options={[
                      {
                        key: 'disabled',
                        text: t('setting.operation.retry.options.disabled'),
                        value: '0',
                      },
                      {
                        key: 'all_candidates',
                        text: t('setting.operation.retry.options.all_candidates'),
                        value: '1',
                      },
                    ]}
                  />
                </AppField>
              </AppFormRow>
              <AppFormActions align='start'>
                <AppButton
                  className='router-section-button'
                  onClick={() => {
                    saveSectionConfig('retry').then();
                  }}
                >
                  {t('setting.operation.retry.buttons.save')}
                </AppButton>
              </AppFormActions>
              {shouldRenderDividerAfter('retry') ? <AppDivider /> : null}
            </>
          ) : null}

          {sectionVisible.log ? (
            <>
              <AppFilterHeader
                title={t('setting.operation.log.title')}
                titleClassName='router-ui-section-title'
                className='router-toolbar-compact'
              />
              <AppFormRow>
                <AppField label={t('setting.operation.log.enable_consume')}>
                  <AppSwitch
                    checked={inputs.LogConsumeEnabled === 'true'}
                    onChange={() =>
                      handleInputChange(null, {
                        name: 'LogConsumeEnabled',
                        value: inputs.LogConsumeEnabled === 'true'
                          ? 'false'
                          : 'true',
                      })
                    }
                  />
                </AppField>
              </AppFormRow>
              <AppFormRow>
                <AppField label={t('setting.operation.log.target_time')}>
                  <AppInput
                    className='router-section-input'
                    value={logCleanupTimestamp}
                    type='datetime-local'
                    name='history_timestamp'
                    onChange={(e, { value }) => {
                      setLogCleanupTimestamp(value);
                    }}
                  />
                </AppField>
              </AppFormRow>
              <AppFormActions align='start'>
                <AppButton
                  className='router-section-button'
                  onClick={() => {
                    deleteHistoryLogs().then();
                  }}
                >
                  {t('setting.operation.log.buttons.clean')}
                </AppButton>
              </AppFormActions>
              {shouldRenderDividerAfter('log') ? <AppDivider /> : null}
            </>
          ) : null}

          {sectionVisible.general ? (
            <>
              <AppFilterHeader
                title={t('setting.operation.general.title')}
                titleClassName='router-ui-section-title'
                className='router-toolbar-compact'
              />
              <AppFormRow>
                <AppField label={t('setting.operation.general.chat_link')}>
                  <AppInput
                    className='router-section-input'
                    name='ChatLink'
                    value={inputs.ChatLink || ''}
                    onChange={handleInputChange}
                    placeholder={t('setting.operation.general.chat_link_placeholder')}
                  />
                </AppField>
              </AppFormRow>
              <AppFormRow>
                <AppField
                  label={t(
                    'setting.operation.general.channel_billing_auto_refresh.enabled'
                  )}
                >
                  <AppSwitch
                    checked={inputs.ChannelBillingAutoRefreshEnabled === 'true'}
                    onChange={() =>
                      handleInputChange(null, {
                        name: 'ChannelBillingAutoRefreshEnabled',
                        value:
                          inputs.ChannelBillingAutoRefreshEnabled === 'true'
                            ? 'false'
                            : 'true',
                      })
                    }
                  />
                </AppField>
                <AppField
                  label={t(
                    'setting.operation.general.channel_billing_auto_refresh.interval_seconds'
                  )}
                >
                  <AppInputNumber
                    className='router-section-input'
                    name='ChannelBillingAutoRefreshIntervalSeconds'
                    onChange={handleInputChange}
                    value={inputs.ChannelBillingAutoRefreshIntervalSeconds}
                    min={60}
                    precision={0}
                    fluid
                  />
                </AppField>
              </AppFormRow>
              <AppFormRow>
                <AppField
                  label={t(
                    'setting.operation.general.channel_billing_auto_refresh.last_run'
                  )}
                >
                  <AppInput
                    className='router-section-input'
                    value={
                      Number(inputs.ChannelBillingAutoRefreshLastRunAt || 0) > 0
                        ? timestamp2string(
                            Number(inputs.ChannelBillingAutoRefreshLastRunAt || 0)
                          )
                        : '-'
                    }
                    readOnly
                  />
                </AppField>
              </AppFormRow>
              <AppFormActions align='start'>
                <AppButton
                  className='router-section-button'
                  onClick={() => {
                    saveSectionConfig('general').then();
                  }}
                >
                  {t('setting.operation.general.buttons.save')}
                </AppButton>
              </AppFormActions>
            </>
          ) : null}

      </div>
    </AppSpin>
  );
};

export default OperationSetting;
