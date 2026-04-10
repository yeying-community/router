import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Divider, Form, Grid, Header, Message } from 'semantic-ui-react';
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
    <Form.Field>
      <label>{t(labelKey)}</label>
      <div className='router-section-input-with-unit'>
        <Form.Input
          className='router-section-input router-section-input-with-unit-field'
          autoComplete='new-password'
          value={inputs[optionKey] ?? '0'}
          type='number'
          min='0'
          step={resolveBillingInputStep(billingUnits[optionKey], billingCurrencyIndex)}
          placeholder={t(placeholderKey)}
          onChange={(e) => {
            setInputs((prev) => ({
              ...prev,
              [optionKey]: e.target.value || '0',
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
      {descriptionKey ? (
        <div className='router-section-field-description'>{t(descriptionKey)}</div>
      ) : null}
    </Form.Field>
  );

  const renderTopupPlanField = (
    labelKey,
    optionKey,
    placeholderKey,
    descriptionKey,
  ) => (
    <Form.Field>
      <label>{t(labelKey)}</label>
      <Form.Dropdown
        className='router-section-input'
        selection
        clearable
        search
        options={topupPlanOptions}
        name={optionKey}
        value={inputs[optionKey] || ''}
        onChange={handleInputChange}
        placeholder={t(placeholderKey)}
      />
      <div className='router-section-field-description'>{t(descriptionKey)}</div>
    </Form.Field>
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
    <Grid columns={1}>
      <Grid.Column>
        <Form loading={loading}>
          {sectionVisible.balance ? (
            <>
              <Header as='h3' className='router-section-title'>{t('setting.operation.quota.title')}</Header>
              <Form.Group widths='equal'>
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
              </Form.Group>
              <Form.Group widths='equal'>
                <Form.Dropdown
                  className='router-section-input'
                  label={t('setting.operation.quota.default_group')}
                  name={BALANCE_OPTION_KEYS.defaultGroup}
                  selection
                  clearable
                  search
                  options={groupOptions}
                  onChange={handleInputChange}
                  value={inputs[BALANCE_OPTION_KEYS.defaultGroup] || ''}
                  placeholder={t('setting.operation.quota.default_group_placeholder')}
                />
              </Form.Group>
              <Form.Button
                className='router-section-button'
                onClick={() => {
                  saveSectionConfig('balance').then();
                }}
              >
                {t('setting.operation.quota.buttons.save')}
              </Form.Button>
              {shouldRenderDividerAfter('balance') ? <Divider /> : null}
            </>
          ) : null}

          {sectionVisible.monitor ? (
            <>
              <Header as='h3' className='router-section-title'>{t('setting.operation.monitor.title')}</Header>
              <Form.Group widths={3}>
                <Form.Input
                  className='router-section-input'
                  label={t('setting.operation.monitor.max_response_time')}
                  name='ChannelDisableThreshold'
                  onChange={handleInputChange}
                  autoComplete='new-password'
                  value={inputs.ChannelDisableThreshold}
                  type='number'
                  min='0'
                  placeholder={t(
                    'setting.operation.monitor.max_response_time_placeholder'
                  )}
                />
                <Form.Input
                  className='router-section-input'
                  label={t('setting.operation.monitor.quota_reminder')}
                  name={BALANCE_OPTION_KEYS.balanceReminderThreshold}
                  onChange={handleInputChange}
                  autoComplete='new-password'
                  value={inputs[BALANCE_OPTION_KEYS.balanceReminderThreshold]}
                  type='number'
                  min='0'
                  placeholder={t(
                    'setting.operation.monitor.quota_reminder_placeholder'
                  )}
                />
              </Form.Group>
              <Form.Group inline>
                <Form.Checkbox
                  className='router-section-checkbox'
                  checked={inputs.AutomaticDisableChannelEnabled === 'true'}
                  label={t('setting.operation.monitor.auto_disable')}
                  name='AutomaticDisableChannelEnabled'
                  onChange={handleInputChange}
                />
                <Form.Checkbox
                  className='router-section-checkbox'
                  checked={inputs.AutomaticEnableChannelEnabled === 'true'}
                  label={t('setting.operation.monitor.auto_enable')}
                  name='AutomaticEnableChannelEnabled'
                  onChange={handleInputChange}
                />
              </Form.Group>
              <Form.Button
                className='router-section-button'
                onClick={() => {
                  saveSectionConfig('monitor').then();
                }}
              >
                {t('setting.operation.monitor.buttons.save')}
              </Form.Button>
              {shouldRenderDividerAfter('monitor') ? <Divider /> : null}
            </>
          ) : null}

          {sectionVisible.retry ? (
            <>
              <Header as='h3' className='router-section-title'>
                {t('setting.operation.retry.title')}
              </Header>
              <Message info className='router-section-message'>
                <Message.Header>
                  {t('setting.operation.retry.description_title')}
                </Message.Header>
                <p>{t('setting.operation.retry.description')}</p>
                <p>{t('setting.operation.retry.description_effective')}</p>
                <p>{t('setting.operation.retry.description_disabled')}</p>
              </Message>
              <Form.Group widths={2}>
                <Form.Dropdown
                  className='router-section-input'
                  selection
                  name='RetryTimes'
                  label={t('setting.operation.retry.limit')}
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
              </Form.Group>
              <Form.Button
                className='router-section-button'
                onClick={() => {
                  saveSectionConfig('retry').then();
                }}
              >
                {t('setting.operation.retry.buttons.save')}
              </Form.Button>
              {shouldRenderDividerAfter('retry') ? <Divider /> : null}
            </>
          ) : null}

          {sectionVisible.log ? (
            <>
              <Header as='h3' className='router-section-title'>{t('setting.operation.log.title')}</Header>
              <Form.Group inline>
                <Form.Checkbox
                  className='router-section-checkbox'
                  checked={inputs.LogConsumeEnabled === 'true'}
                  label={t('setting.operation.log.enable_consume')}
                  name='LogConsumeEnabled'
                  onChange={handleInputChange}
                />
              </Form.Group>
              <Form.Group widths={4}>
                <Form.Input
                  className='router-section-input'
                  label={t('setting.operation.log.target_time')}
                  value={logCleanupTimestamp}
                  type='datetime-local'
                  name='history_timestamp'
                  onChange={(e, { value }) => {
                    setLogCleanupTimestamp(value);
                  }}
                />
              </Form.Group>
              <Form.Button
                className='router-section-button'
                onClick={() => {
                  deleteHistoryLogs().then();
                }}
              >
                {t('setting.operation.log.buttons.clean')}
              </Form.Button>
              {shouldRenderDividerAfter('log') ? <Divider /> : null}
            </>
          ) : null}

        </Form>
      </Grid.Column>
    </Grid>
  );
};

export default OperationSetting;
