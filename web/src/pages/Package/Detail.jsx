import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useParams } from 'react-router-dom';
import { API, showError, showInfo, showSuccess, timestamp2string } from '../../helpers';
import {
  buildBillingCurrencyIndex,
  buildBillingUnitOptions,
  buildDisplayUnitOptions,
  billingInputValueToYYC,
  convertBillingInputValueUnit,
  resolveBillingInputStep,
  resolveDefaultBillingUnit,
  yycToBillingInputValue,
} from '../../helpers/billing';
import { formatDecimalNumber } from '../../helpers/render';
import UnitDropdown from '../../components/UnitDropdown';
import {
  AppButton,
  AppDetailSection,
  AppField,
  AppFilterHeader,
  AppFormActions,
  AppFormRow,
  AppInput,
  AppInputNumber,
  AppModal,
  AppSelect,
  AppSwitch,
  AppTextarea,
} from '../../router-ui';

const createEmptyForm = (defaultBillingUnit = 'USD') => ({
  id: '',
  name: '',
  description: '',
  group_id: '',
  visibility_scope: 'all',
  visible_user_ids: [],
  sale_price: '0',
  sale_currency: 'CNY',
  daily_amount: '0',
  daily_amount_unit: defaultBillingUnit,
  emergency_amount: '0',
  emergency_amount_unit: defaultBillingUnit,
  duration_days: 30,
  reset_timezone: 'Asia/Shanghai',
  enabled: true,
  source: 'manual',
});

const toGroupOptions = (rows) =>
  (Array.isArray(rows) ? rows : []).map((item) => ({
    key: item.id,
    value: item.id,
    text: item.name || item.id,
  }));

const toUserOption = (item) => {
  const id = (item?.id || '').toString().trim();
  const username = (item?.username || '').toString().trim();
  const displayName = (item?.display_name || '').toString().trim();
  return {
    key: id,
    value: id,
    text: [username, displayName].filter(Boolean).join(' / ') || id,
  };
};

const appendGroupOptionIfMissing = (options, groupID, groupName) => {
  const normalizedGroupID = (groupID || '').toString().trim();
  if (!normalizedGroupID) {
    return options;
  }
  const currentOptions = Array.isArray(options) ? options : [];
  if (currentOptions.some((item) => (item?.value || '').toString().trim() === normalizedGroupID)) {
    return currentOptions;
  }
  return [
    ...currentOptions,
    {
      key: normalizedGroupID,
      value: normalizedGroupID,
      text: (groupName || '').toString().trim() || normalizedGroupID,
    },
  ];
};

const appendUserOptionsIfMissing = (options, users) => {
  const currentOptions = Array.isArray(options) ? options : [];
  const nextOptions = [...currentOptions];
  const seen = new Set(
    currentOptions.map((item) => (item?.value || '').toString().trim()).filter(Boolean),
  );
  (Array.isArray(users) ? users : []).forEach((item) => {
    const option = toUserOption(item);
    const normalizedID = (option?.value || '').toString().trim();
    if (!normalizedID || seen.has(normalizedID)) {
      return;
    }
    seen.add(normalizedID);
    nextOptions.push(option);
  });
  return nextOptions;
};

const formatByCurrencyMinorUnit = (amount, currency) => {
  const normalizedAmount = Number(amount || 0);
  if (!Number.isFinite(normalizedAmount)) {
    return '-';
  }
  const minorUnit = Number(currency?.minor_unit);
  const maximumFractionDigits =
    Number.isInteger(minorUnit) && minorUnit >= 0 ? minorUnit : 8;
  const unit = (currency?.code || '').toString().trim().toUpperCase();
  if (unit === 'YYC') {
    return formatDecimalNumber(Math.round(normalizedAmount), 0);
  }
  return formatDecimalNumber(normalizedAmount, maximumFractionDigits);
};

const renderPackageAmountValue = (yycAmount, displayUnit, currencyIndex) => {
  const normalizedYYCAmount = Number(yycAmount || 0);
  if (!Number.isFinite(normalizedYYCAmount)) {
    return '-';
  }
  const targetCurrency = currencyIndex[displayUnit] || currencyIndex.YYC;
  const rate = Number(targetCurrency?.yyc_per_unit || 0);
  if (!Number.isFinite(rate) || rate <= 0) {
    return '-';
  }
  return formatByCurrencyMinorUnit(normalizedYYCAmount / rate, targetCurrency);
};

const resolvePackageYYCAmount = (row, type) => {
  if (type === 'daily') {
    return Number(row?.daily_quota_limit ?? 0);
  }
  return Number(row?.package_emergency_quota_limit ?? 0);
};

const PackageDetail = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { id } = useParams();
  const [loading, setLoading] = useState(false);
  const [detail, setDetail] = useState(null);
  const [dailyDisplayUnit, setDailyDisplayUnit] = useState('USD');
  const [emergencyDisplayUnit, setEmergencyDisplayUnit] = useState('USD');
  const [currencyIndex, setCurrencyIndex] = useState(
    buildBillingCurrencyIndex([], { activeOnly: true }),
  );
  const [editOpen, setEditOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [form, setForm] = useState(createEmptyForm('USD'));
  const [groupOptions, setGroupOptions] = useState([]);
  const [groupLoading, setGroupLoading] = useState(false);
  const [userOptions, setUserOptions] = useState([]);
  const [userLoading, setUserLoading] = useState(false);

  const normalizedId = useMemo(() => (id || '').toString().trim(), [id]);

  const displayUnitOptions = useMemo(
    () => buildDisplayUnitOptions(currencyIndex, { order: 'yyc-first' }),
    [currencyIndex],
  );

  const billingUnitOptions = useMemo(
    () => buildBillingUnitOptions(currencyIndex),
    [currencyIndex],
  );

  const loadGroups = useCallback(async () => {
    setGroupLoading(true);
    try {
      const items = [];
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
          showError(message || t('package_manage.messages.group_load_failed'));
          return;
        }
        const pageItems = Array.isArray(data?.items) ? data.items : [];
        items.push(...pageItems);
        const total = Number(data?.total || pageItems.length || 0);
        if (pageItems.length === 0 || items.length >= total || pageItems.length < 100) {
          break;
        }
        page += 1;
      }
      setGroupOptions(toGroupOptions(items));
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setGroupLoading(false);
    }
  }, [t]);

  const searchUsers = useCallback(
    async (keyword) => {
      const normalizedValue = (keyword || '').toString().trim();
      if (normalizedValue === '') {
        return;
      }
      setUserLoading(true);
      try {
        const res = await API.get('/api/v1/admin/user/search', {
          params: {
            keyword: normalizedValue,
          },
        });
        const { success, message, data } = res?.data || {};
        if (!success) {
          showError(message || t('package_manage.messages.user_load_failed'));
          return;
        }
        setUserOptions((current) => appendUserOptionsIfMissing(current, data));
      } catch (error) {
        showError(error?.message || t('package_manage.messages.user_load_failed'));
      } finally {
        setUserLoading(false);
      }
    },
    [t],
  );

  const loadInitialUsers = useCallback(async () => {
    setUserLoading(true);
    try {
      const res = await API.get('/api/v1/admin/user', {
        params: {
          page: 1,
        },
      });
      const { success, message, data } = res?.data || {};
      if (!success) {
        showError(message || t('package_manage.messages.user_load_failed'));
        return;
      }
      setUserOptions((current) => appendUserOptionsIfMissing(current, data));
    } catch (error) {
      showError(error?.message || t('package_manage.messages.user_load_failed'));
    } finally {
      setUserLoading(false);
    }
  }, [t]);

  const loadDisplayUnits = useCallback(async () => {
    try {
      const res = await API.get('/api/v1/admin/billing/currencies');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message);
        return;
      }
      const next = buildBillingCurrencyIndex(Array.isArray(data) ? data : [], {
        activeOnly: true,
      });
      setCurrencyIndex(next);
      const resolveNextUnit = (current) => {
        const normalizedCurrent = (current || '').toString().trim().toUpperCase();
        if (normalizedCurrent && next[normalizedCurrent]) {
          return normalizedCurrent;
        }
        if (next.USD) {
          return 'USD';
        }
        const fallbackUnit = Object.keys(next)
          .filter((code) => code)
          .sort((a, b) => a.localeCompare(b))[0];
        return fallbackUnit || 'YYC';
      };
      setDailyDisplayUnit(resolveNextUnit);
      setEmergencyDisplayUnit(resolveNextUnit);
    } catch (error) {
      showError(error?.message || error);
    }
  }, []);

  const loadDetail = useCallback(async () => {
    if (normalizedId === '') {
      return;
    }
    setLoading(true);
    try {
      const res = await API.get(`/api/v1/admin/package/${encodeURIComponent(normalizedId)}`);
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('package_manage.messages.load_failed'));
        return;
      }
      setDetail(data || null);
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setLoading(false);
    }
  }, [normalizedId, t]);

  useEffect(() => {
    loadDisplayUnits().then();
  }, [loadDisplayUnits]);

  useEffect(() => {
    loadGroups().then();
  }, [loadGroups]);

  useEffect(() => {
    loadDetail().then();
  }, [loadDetail]);

  const closeEditModal = () => {
    if (submitting) return;
    setEditOpen(false);
  };

  const openEditModal = () => {
    if (!detail || submitting) return;
    const defaultBillingUnit = resolveDefaultBillingUnit(currencyIndex);
    const resolvedGroupID = (detail?.group_id || '').toString().trim();
    const resolvedGroupName = (detail?.group_name || '').toString().trim();
    setGroupOptions((current) =>
      appendGroupOptionIfMissing(current, resolvedGroupID, resolvedGroupName),
    );
    setForm({
      id: detail.id || '',
      name: detail.name || '',
      description: detail.description || '',
      group_id: resolvedGroupID,
      visibility_scope: detail?.visibility_scope || 'all',
      visible_user_ids: Array.isArray(detail?.visible_user_ids)
        ? detail.visible_user_ids.map((item) => (item || '').toString()).filter(Boolean)
        : [],
      sale_price: detail?.sale_price ?? '0',
      sale_currency: detail?.sale_currency || 'CNY',
      daily_amount: yycToBillingInputValue(
        Number(detail?.daily_quota_limit ?? 0),
        defaultBillingUnit,
        currencyIndex,
      ),
      daily_amount_unit: defaultBillingUnit,
      emergency_amount: yycToBillingInputValue(
        Number(detail?.package_emergency_quota_limit ?? 0),
        defaultBillingUnit,
        currencyIndex,
      ),
      emergency_amount_unit: defaultBillingUnit,
      duration_days: Number(detail?.duration_days || 30),
      reset_timezone: detail?.quota_reset_timezone || 'Asia/Shanghai',
      enabled: Boolean(detail?.enabled),
      source: detail?.source || 'manual',
    });
    setUserOptions((current) =>
      appendUserOptionsIfMissing(current, detail?.visible_users),
    );
    loadInitialUsers().then();
    setEditOpen(true);
  };

  const buildPayloadFromForm = () => {
    const name = (form.name || '').trim();
    if (name === '') {
      showInfo(t('package_manage.messages.name_required'));
      return null;
    }
    const groupID = (form.group_id || '').trim();
    if (groupID === '') {
      showInfo(t('package_manage.messages.group_required'));
      return null;
    }
    const visibilityScope = (form.visibility_scope || 'all').toString().trim() || 'all';
    const visibleUserIDs = Array.isArray(form.visible_user_ids)
      ? [...new Set(form.visible_user_ids.map((item) => (item || '').toString().trim()).filter(Boolean))]
      : [];
    if (visibilityScope === 'partial_users' && visibleUserIDs.length === 0) {
      showInfo(t('package_manage.messages.visible_users_required'));
      return null;
    }
    const dailyStored = billingInputValueToYYC(
      form.daily_amount ?? 0,
      form.daily_amount_unit,
      currencyIndex,
    );
    const emergencyStored = billingInputValueToYYC(
      form.emergency_amount ?? 0,
      form.emergency_amount_unit,
      currencyIndex,
    );
    if (
      !Number.isFinite(Number(form.sale_price || 0)) ||
      Number(form.sale_price || 0) < 0 ||
      !Number.isFinite(dailyStored) ||
      dailyStored < 0 ||
      !Number.isFinite(emergencyStored) ||
      emergencyStored < 0
    ) {
      showInfo(t('package_manage.messages.quota_invalid'));
      return null;
    }
    const durationDays = Number(form.duration_days || 0);
    if (!Number.isFinite(durationDays) || durationDays <= 0) {
      showInfo(t('package_manage.messages.duration_invalid'));
      return null;
    }
    return {
      id: (form.id || '').trim(),
      name,
      description: (form.description || '').trim(),
      group_id: groupID,
      visibility_scope: visibilityScope,
      visible_user_ids: visibilityScope === 'partial_users' ? visibleUserIDs : [],
      sale_price: Number(form.sale_price || 0),
      sale_currency: (form.sale_currency || 'CNY').trim().toUpperCase() || 'CNY',
      daily_quota_limit: Math.trunc(dailyStored),
      package_emergency_quota_limit: Math.trunc(emergencyStored),
      duration_days: Math.trunc(durationDays),
      quota_reset_timezone:
        (form.reset_timezone || '').trim() || 'Asia/Shanghai',
      enabled: Boolean(form.enabled),
      sort_order: Math.trunc(Number(detail?.sort_order || 0)),
      source: (form.source || '').trim() || 'manual',
    };
  };

  const submitEdit = async () => {
    const payload = buildPayloadFromForm();
    if (!payload) return;
    if (!payload.id) {
      showInfo(t('package_manage.messages.id_required'));
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.put('/api/v1/admin/package/', payload);
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('package_manage.messages.update_failed'));
        return;
      }
      showSuccess(t('package_manage.messages.update_success'));
      setDetail(data || null);
      setEditOpen(false);
      loadDetail().then();
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setSubmitting(false);
    }
  };

  const renderEditForm = () => (
    <div>
      <AppFormRow>
        <AppField label={t('package_manage.form.name')} required>
          <AppInput
            className='router-section-input'
            value={form.name}
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, name: value || '' }))
            }
          />
        </AppField>
        <AppField label={t('package_manage.form.group')} required>
          <AppSelect
            className='router-section-dropdown'
            options={groupOptions}
            value={form.group_id}
            loading={groupLoading}
            search
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, group_id: (value || '').toString() }))
            }
          />
        </AppField>
      </AppFormRow>

      <AppFormRow>
        <AppField label={t('package_manage.form.visibility_scope')}>
          <AppSelect
            className='router-section-dropdown'
            options={[
              {
                key: 'all',
                value: 'all',
                text: t('package_manage.form.visibility_scope_all'),
              },
              {
                key: 'partial_users',
                value: 'partial_users',
                text: t('package_manage.form.visibility_scope_partial_users'),
              },
            ]}
            value={form.visibility_scope}
            onChange={(e, { value }) =>
              setForm((prev) => ({
                ...prev,
                visibility_scope: (value || 'all').toString(),
                visible_user_ids:
                  (value || 'all').toString() === 'partial_users'
                    ? prev.visible_user_ids
                    : [],
              }))
            }
            onClick={() => {
              if (userOptions.length === 0) {
                loadInitialUsers().then();
              }
            }}
          />
        </AppField>
        <AppField label={t('package_manage.form.visible_users')}>
          <AppSelect
            className='router-section-input'
            placeholder={t('package_manage.form.visible_users_placeholder')}
            options={userOptions}
            value={form.visible_user_ids}
            loading={userLoading}
            multiple
            search
            clearable
            disabled={form.visibility_scope !== 'partial_users'}
            onClick={() => {
              if (userOptions.length === 0) {
                loadInitialUsers().then();
              }
            }}
            onSearch={searchUsers}
            onChange={(e, { value }) =>
              setForm((prev) => ({
                ...prev,
                visible_user_ids: Array.isArray(value) ? value : [],
              }))
            }
          />
        </AppField>
      </AppFormRow>

      <AppFormRow>
        <AppField label={t('package_manage.form.description')}>
          <AppTextarea
            className='router-section-input'
            value={form.description}
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, description: (value || '').toString() }))
            }
          />
        </AppField>
      </AppFormRow>

      <AppFormRow>
        <AppField label={t('package_manage.form.sale_price')}>
          <AppInputNumber
            className='router-section-input'
            min={0}
            step={0.01}
            precision={2}
            fluid
            value={form.sale_price}
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, sale_price: value ?? '0' }))
            }
          />
        </AppField>
        <AppField label={t('package_manage.form.sale_currency')}>
          <AppInput
            className='router-section-input'
            value={form.sale_currency}
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, sale_currency: (value || 'CNY').toUpperCase() }))
            }
          />
        </AppField>
      </AppFormRow>

      <AppFormRow>
        <AppField label={t('package_manage.form.daily_quota_limit')}>
          <AppInputNumber
            className='router-section-input'
            value={form.daily_amount}
            step={resolveBillingInputStep(form.daily_amount_unit, currencyIndex)}
            min={0}
            precision={6}
            fluid
            addonAfter={
              <UnitDropdown
                variant='inputUnit'
                bordered={false}
                options={billingUnitOptions}
                value={form.daily_amount_unit}
                onChange={(_, { value }) => {
                  const nextUnit = (value || 'YYC').toString().trim().toUpperCase();
                  setForm((prev) => ({
                    ...prev,
                    daily_amount: convertBillingInputValueUnit(
                      prev.daily_amount,
                      prev.daily_amount_unit,
                      nextUnit,
                      currencyIndex,
                    ),
                    daily_amount_unit: nextUnit,
                  }));
                }}
                aria-label={t('package_manage.form.daily_quota_limit')}
              />
            }
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, daily_amount: value ?? '0' }))
            }
          />
        </AppField>
        <AppField label={t('package_manage.form.package_emergency_quota_limit')}>
          <AppInputNumber
            className='router-section-input'
            value={form.emergency_amount}
            step={resolveBillingInputStep(form.emergency_amount_unit, currencyIndex)}
            min={0}
            precision={6}
            fluid
            addonAfter={
              <UnitDropdown
                variant='inputUnit'
                bordered={false}
                options={billingUnitOptions}
                value={form.emergency_amount_unit}
                onChange={(_, { value }) => {
                  const nextUnit = (value || 'YYC').toString().trim().toUpperCase();
                  setForm((prev) => ({
                    ...prev,
                    emergency_amount: convertBillingInputValueUnit(
                      prev.emergency_amount,
                      prev.emergency_amount_unit,
                      nextUnit,
                      currencyIndex,
                    ),
                    emergency_amount_unit: nextUnit,
                  }));
                }}
                aria-label={t('package_manage.form.package_emergency_quota_limit')}
              />
            }
            onChange={(e, { value }) =>
              setForm((prev) => ({
                ...prev,
                emergency_amount: value ?? '0',
              }))
            }
          />
        </AppField>
      </AppFormRow>

      <AppFormRow>
        <AppField label={t('package_manage.form.duration_days')}>
          <AppInputNumber
            className='router-section-input'
            min={1}
            step={1}
            precision={0}
            fluid
            value={form.duration_days}
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, duration_days: value || 0 }))
            }
          />
        </AppField>
        <AppField label={t('package_manage.form.quota_reset_timezone')}>
          <AppInput
            className='router-section-input'
            value={form.reset_timezone}
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, reset_timezone: value || '' }))
            }
          />
        </AppField>
      </AppFormRow>

      <AppFormRow>
        <AppField label={t('package_manage.form.enabled')}>
          <AppSwitch
            checked={Boolean(form.enabled)}
            onChange={(e, { checked }) =>
              setForm((prev) => ({ ...prev, enabled: Boolean(checked) }))
            }
          />
        </AppField>
        <AppField label={t('package_manage.form.source')}>
          <AppInput
            className='router-section-input'
            value={form.source}
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, source: value || '' }))
            }
          />
        </AppField>
      </AppFormRow>
    </div>
  );

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'admin', label: t('header.admin_workspace') },
          { key: 'business', label: t('header.business_operation') },
          {
            key: 'package-list',
            label: t('header.package'),
            onClick: () => navigate('/admin/package'),
          },
          { key: 'package-current', label: normalizedId || '-', active: true },
        ]}
        title={t('package_manage.dialog.detail_title')}
        actions={
          <AppButton
            type='button'
            className='router-page-button'
            color='blue'
            disabled={loading || !detail}
            onClick={openEditModal}
          >
            {t('package_manage.buttons.edit')}
          </AppButton>
        }
      />
      <div className='router-entity-detail-page'>
        <AppDetailSection title={t('common.basic_info')} bodyClassName='router-page-stack'>
            {loading ? (
              <div className='router-empty-cell'>{t('common.loading')}</div>
            ) : (
              <>
                <AppFormRow>
                  <AppField label={t('package_manage.form.id')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={detail?.id || '-'}
                      readOnly
                    />
                  </AppField>
                  <AppField label={t('package_manage.table.name')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={detail?.name || '-'}
                      readOnly
                    />
                  </AppField>
                  <AppField label={t('package_manage.table.group')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={detail?.group_name || detail?.group_id || '-'}
                      readOnly
                    />
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  <AppField label={t('package_manage.form.description')} readOnly>
                    <AppTextarea
                      className='router-section-input'
                      value={detail?.description || '-'}
                      readOnly
                      rows={3}
                    />
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  <AppField label={t('package_manage.form.sale_price')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={`${detail?.sale_currency || 'CNY'} ${detail?.sale_price ?? 0}`}
                      readOnly
                    />
                  </AppField>
                  <AppField label={t('package_manage.form.sale_currency')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={detail?.sale_currency || 'CNY'}
                      readOnly
                    />
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  <AppField label={t('package_manage.table.daily_quota_limit')} readOnly>
                    <div className='router-section-input-with-unit'>
                      <AppInput
                        className='router-section-input router-section-input-with-unit-field'
                        value={renderPackageAmountValue(
                          resolvePackageYYCAmount(detail, 'daily'),
                          dailyDisplayUnit,
                          currencyIndex,
                        )}
                        readOnly
                      />
                      <UnitDropdown
                        variant='inputUnit'
                        options={displayUnitOptions}
                        value={dailyDisplayUnit}
                        onChange={(_, { value }) =>
                          setDailyDisplayUnit((value || '').toString().trim().toUpperCase())
                        }
                        aria-label={t('package_manage.table.daily_quota_limit')}
                      />
                    </div>
                  </AppField>
                  <AppField
                    label={t('package_manage.table.package_emergency_quota_limit')}
                    readOnly
                  >
                    <div className='router-section-input-with-unit'>
                      <AppInput
                        className='router-section-input router-section-input-with-unit-field'
                        value={renderPackageAmountValue(
                          resolvePackageYYCAmount(detail, 'emergency'),
                          emergencyDisplayUnit,
                          currencyIndex,
                        )}
                        readOnly
                      />
                      <UnitDropdown
                        variant='inputUnit'
                        options={displayUnitOptions}
                        value={emergencyDisplayUnit}
                        onChange={(_, { value }) =>
                          setEmergencyDisplayUnit((value || '').toString().trim().toUpperCase())
                        }
                        aria-label={t('package_manage.table.package_emergency_quota_limit')}
                      />
                    </div>
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  <AppField label={t('package_manage.table.duration_days')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={Number(detail?.duration_days || 0) || '-'}
                      readOnly
                    />
                  </AppField>
                  <AppField label={t('package_manage.table.status')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={
                        detail?.enabled
                          ? t('package_manage.status.enabled')
                          : t('package_manage.status.disabled')
                      }
                      readOnly
                    />
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  <AppField label={t('package_manage.form.quota_reset_timezone')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={detail?.quota_reset_timezone || '-'}
                      readOnly
                    />
                  </AppField>
                  <AppField label={t('package_manage.table.created_at')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={detail?.created_at ? timestamp2string(detail.created_at) : '-'}
                      readOnly
                    />
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  <AppField label={t('package_manage.table.updated_at')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={detail?.updated_at ? timestamp2string(detail.updated_at) : '-'}
                      readOnly
                    />
                  </AppField>
                </AppFormRow>
              </>
            )}
        </AppDetailSection>
      </div>
      <AppModal
        open={editOpen}
        onClose={closeEditModal}
        size='small'
        title={t('package_manage.dialog.edit_title')}
        footer={null}
      >
        <div className='router-page-stack'>
          {renderEditForm()}
          <AppFormActions>
            <AppButton type='button' onClick={closeEditModal} disabled={submitting}>
              {t('common.cancel')}
            </AppButton>
            <AppButton type='button' color='blue' loading={submitting} onClick={submitEdit}>
              {t('common.confirm')}
            </AppButton>
          </AppFormActions>
        </div>
      </AppModal>
    </div>
  );
};

export default PackageDetail;
