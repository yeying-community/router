import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { API, showError, showInfo, showSuccess, timestamp2string } from '../../helpers';
import { formatAmountWithUnit } from '../../helpers/render';
import { formatPackageConcurrencyLimit } from '../../helpers/package';
import {
  AppButton,
  AppDetailSection,
  AppField,
  AppFormActions,
  AppFormRow,
  AppFilterHeader,
  AppInput,
  AppInputNumber,
  AppModal,
  AppSelect,
  AppSwitch,
  AppTable,
  AppTableActionButton,
  AppTabs,
  AppTextarea,
} from '../../router-ui';

const readOnlyText = (value) => {
  const normalized = (value || '').toString().trim();
  return normalized || '-';
};

const formatDateTime = (value) => {
  const normalized = Number(value || 0);
  if (!Number.isFinite(normalized) || normalized <= 0) {
    return '-';
  }
  return timestamp2string(normalized);
};

const normalizeModels = (models) =>
  Array.isArray(models)
    ? models
      .map((item) => (item?.model || item?.name || item || '').toString().trim())
      .filter(Boolean)
    : [];

const normalizeVisibleUsers = (users) =>
  (Array.isArray(users) ? users : [])
    .map((item) => {
      const id = (item?.id || item?.user_id || '').toString().trim();
      if (!id) {
        return null;
      }
      const username = (item?.username || '').toString().trim();
      const displayName = (item?.display_name || '').toString().trim();
      const walletAddress = (item?.wallet_address || '').toString().trim();
      return {
        id,
        name: displayName || username || id,
        walletAddress,
      };
    })
    .filter(Boolean);

const formatVisibilityScope = (scope, t) =>
  scope === 'partial_users'
    ? t('package_manage.form.visibility_scope_partial_users')
    : t('package_manage.form.visibility_scope_all');

const VISIBILITY_OPTIONS = [
  { key: 'all', value: 'all', textKey: 'package_manage.form.visibility_scope_all' },
  {
    key: 'partial_users',
    value: 'partial_users',
    textKey: 'package_manage.form.visibility_scope_partial_users',
  },
];

const toVisibilityOptions = (t) =>
  VISIBILITY_OPTIONS.map((item) => ({
    key: item.key,
    value: item.value,
    text: t(item.textKey),
  }));

const createTopupEditForm = () => ({
  id: '',
  kind: 'balance',
  name: '',
  description: '',
  group_id: '',
  sale_price: 0,
  sale_currency: 'CNY',
  quota_amount: 0,
  quota_currency: 'YYC',
  validity_days: 0,
  max_concurrency_per_user: 0,
  max_concurrency_per_package: 0,
  visibility_scope: 'all',
  visible_user_ids: [],
  enabled: true,
  sort_order: 0,
  source: 'manual',
});

const toGroupOptions = (rows) =>
  (Array.isArray(rows) ? rows : [])
    .map((item) => {
      const value = (item?.id || '').toString().trim();
      if (!value) {
        return null;
      }
      return {
        key: value,
        value,
        text: (item?.name || value).toString(),
      };
    })
    .filter(Boolean);

const appendGroupOptionIfMissing = (options, groupID, groupName) => {
  const normalizedGroupID = (groupID || '').toString().trim();
  if (!normalizedGroupID) {
    return Array.isArray(options) ? options : [];
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

const toUserOption = (item) => {
  const id = (item?.id || item?.user_id || '').toString().trim();
  if (!id) {
    return null;
  }
  const username = (item?.username || '').toString().trim();
  const displayName = (item?.display_name || '').toString().trim();
  const walletAddress = (item?.wallet_address || '').toString().trim();
  const primaryName = displayName || username;
  return {
    key: id,
    value: id,
    text: [primaryName, walletAddress].filter(Boolean).join(' / ') || id,
    label: primaryName || id,
    walletAddress,
  };
};

const appendUserOptionsIfMissing = (options, users) => {
  const currentOptions = Array.isArray(options) ? options : [];
  const nextOptions = [...currentOptions];
  const seen = new Set(
    currentOptions.map((item) => (item?.value || '').toString().trim()).filter(Boolean),
  );
  (Array.isArray(users) ? users : []).forEach((item) => {
    const option = toUserOption(item);
    const value = (option?.value || '').toString().trim();
    if (!value || seen.has(value)) {
      return;
    }
    seen.add(value);
    nextOptions.push(option);
  });
  return nextOptions;
};

const productToTopupPlan = (item) => ({
  ...item,
  id: (item?.id || '').toString().trim(),
  amount: Number(item?.amount ?? item?.sale_price ?? 0) || 0,
  amount_currency: item?.amount_currency || item?.sale_currency || 'CNY',
  quota_amount: Number(item?.quota_amount || 0) || 0,
  quota_currency: item?.quota_currency || 'YYC',
  validity_days: Number(item?.validity_days || item?.duration_days || 0) || 0,
});

const TopupPlanDetail = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { id } = useParams();
  const redemptionSourcePath = typeof location.state?.from === 'string' &&
    location.state.from.startsWith('/admin/redemption/')
    ? location.state.from
    : '';
  const [activeTabKey, setActiveTabKey] = useState('basic');
  const [loading, setLoading] = useState(true);
  const [plan, setPlan] = useState(null);
  const [visibilityScope, setVisibilityScope] = useState('all');
  const [visibilityUserIDs, setVisibilityUserIDs] = useState([]);
  const [visibilitySubmitting, setVisibilitySubmitting] = useState(false);
  const [userOptions, setUserOptions] = useState([]);
  const [userLoading, setUserLoading] = useState(false);
  const [visibilityPickerOpen, setVisibilityPickerOpen] = useState(false);
  const [visibilityPickerValue, setVisibilityPickerValue] = useState([]);
  const [editing, setEditing] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [form, setForm] = useState(createTopupEditForm);
  const [groupOptions, setGroupOptions] = useState([]);
  const [groupLoading, setGroupLoading] = useState(false);

  const visibilityOptions = useMemo(() => toVisibilityOptions(t), [t]);

  const loadDetail = useCallback(async () => {
    setLoading(true);
    try {
      const normalizedID = (id || '').toString().trim();
      const res = await API.get(
        `/api/v1/admin/entitlement/products/${encodeURIComponent(normalizedID)}`,
      );
      const { success, message, data } = res?.data || {};
      if (!success || data?.kind !== 'balance') {
        showError(message || t('topup.manage.load_failed'));
        return;
      }
      const nextPlan = productToTopupPlan(data);
      setPlan(nextPlan);
      setVisibilityScope(nextPlan.visibility_scope === 'partial_users' ? 'partial_users' : 'all');
      setVisibilityUserIDs(
        Array.isArray(nextPlan.visible_user_ids)
          ? nextPlan.visible_user_ids.map((item) => (item || '').toString().trim()).filter(Boolean)
          : [],
      );
      setUserOptions((current) => appendUserOptionsIfMissing(current, nextPlan.visible_users));
    } catch (error) {
      showError(error?.message || t('topup.manage.load_failed'));
    } finally {
      setLoading(false);
    }
  }, [id, t]);

  useEffect(() => {
    loadDetail().then();
  }, [loadDetail]);

  const supportedModels = useMemo(
    () => normalizeModels(plan?.supported_models),
    [plan?.supported_models],
  );
  const visibleUsers = useMemo(
    () => normalizeVisibleUsers(plan?.visible_users),
    [plan?.visible_users],
  );
  const visibilityTableRows = useMemo(() => {
    const usersByID = new Map(visibleUsers.map((item) => [item.id, item]));
    return (Array.isArray(visibilityUserIDs) ? visibilityUserIDs : [])
      .map((userID) => {
        const normalizedUserID = (userID || '').toString().trim();
        if (!normalizedUserID) {
          return null;
        }
        return usersByID.get(normalizedUserID) || {
          id: normalizedUserID,
          name: normalizedUserID,
          walletAddress: normalizedUserID,
        };
      })
      .filter(Boolean);
  }, [visibilityUserIDs, visibleUsers]);

  const loadGroups = useCallback(async () => {
    setGroupLoading(true);
    try {
      const items = [];
      let page = 1;
      while (page <= 50) {
        const res = await API.get('/api/v1/admin/groups', {
          params: { page, page_size: 100 },
        });
        const { success, message, data } = res?.data || {};
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
      showError(error?.message || t('package_manage.messages.group_load_failed'));
    } finally {
      setGroupLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadGroups().then();
  }, [loadGroups]);

  const loadInitialUsers = useCallback(async () => {
    setUserLoading(true);
    try {
      const res = await API.get('/api/v1/admin/user', {
        params: { page: 1 },
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

  const searchUsers = useCallback(
    async (keyword) => {
      const normalizedKeyword = (keyword || '').toString().trim();
      if (!normalizedKeyword) {
        return;
      }
      setUserLoading(true);
      try {
        const res = await API.get('/api/v1/admin/user/search', {
          params: { keyword: normalizedKeyword },
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

  const persistVisibility = useCallback(
    async (nextScope, nextUserIDs) => {
      if (!plan?.id) {
        return false;
      }
      const normalizedScope = (nextScope || 'all').toString().trim() === 'partial_users'
        ? 'partial_users'
        : 'all';
      const normalizedUserIDs = normalizedScope === 'partial_users'
        ? [...new Set(
            (Array.isArray(nextUserIDs) ? nextUserIDs : [])
              .map((item) => (item || '').toString().trim())
              .filter(Boolean),
          )]
        : [];
      setVisibilitySubmitting(true);
      try {
        const payload = {
          id: plan.id,
          kind: plan.kind,
          name: plan.name || '',
          description: plan.description || '',
          group_id: plan.group_id || '',
          sale_price: Number(plan.sale_price || 0),
          sale_currency: plan.sale_currency || 'CNY',
          quota_metric: plan.quota_metric || 'yyc',
          quota_amount: Number(plan.quota_amount || 0),
          quota_currency: plan.quota_currency || 'YYC',
          period_type: plan.period_type || 'none',
          period_limit: Number(plan.period_limit || 0),
          duration_days: Number(plan.duration_days || plan.validity_days || 0),
          validity_days: Number(plan.validity_days || plan.duration_days || 0),
          max_concurrency_per_user: Number(plan.max_concurrency_per_user || 0),
          max_concurrency_per_package: Number(plan.max_concurrency_per_package || 0),
          allow_balance_fallback: Boolean(plan.allow_balance_fallback),
          visibility_scope: normalizedScope,
          visible_user_ids: normalizedUserIDs,
          enabled: Boolean(plan.enabled),
          sort_order: Number(plan.sort_order || 0),
          source: plan.source || 'manual',
        };
        const res = await API.put(
          `/api/v1/admin/entitlement/products/${encodeURIComponent(plan.id)}`,
          payload,
        );
        const { success, message, data } = res?.data || {};
        if (!success) {
          showError(message || t('package_manage.messages.update_failed'));
          return false;
        }
        const nextPlan = productToTopupPlan(data);
        setPlan(nextPlan);
        setVisibilityScope(normalizedScope);
        setVisibilityUserIDs(normalizedUserIDs);
        setUserOptions((current) => appendUserOptionsIfMissing(current, nextPlan.visible_users));
        showSuccess(t('package_manage.messages.update_success'));
        return true;
      } catch (error) {
        showError(error?.message || t('package_manage.messages.update_failed'));
        return false;
      } finally {
        setVisibilitySubmitting(false);
      }
    },
    [plan, t],
  );

  const openVisibilityPicker = async () => {
    if (userOptions.length === 0) {
      await loadInitialUsers();
    }
    setVisibilityPickerValue([]);
    setVisibilityPickerOpen(true);
  };

  const closeVisibilityPicker = () => {
    if (userLoading) {
      return;
    }
    setVisibilityPickerOpen(false);
    setVisibilityPickerValue([]);
  };

  const confirmVisibilityPicker = () => {
    const nextIDs = Array.isArray(visibilityPickerValue)
      ? visibilityPickerValue.map((item) => (item || '').toString().trim()).filter(Boolean)
      : [];
    if (nextIDs.length === 0) {
      closeVisibilityPicker();
      return;
    }
    const mergedUserIDs = [...new Set([...(visibilityUserIDs || []), ...nextIDs])];
    persistVisibility(visibilityScope, mergedUserIDs).then((saved) => {
      if (saved) {
        closeVisibilityPicker();
      }
    });
  };

  const removeVisibleUser = (userID) => {
    const normalizedUserID = (userID || '').toString().trim();
    if (!normalizedUserID) {
      return;
    }
    const nextUserIDs = (Array.isArray(visibilityUserIDs) ? visibilityUserIDs : []).filter(
      (item) => (item || '').toString().trim() !== normalizedUserID,
    );
    persistVisibility(visibilityScope, nextUserIDs).then();
  };

  const startEditing = () => {
    if (!plan || submitting) {
      return;
    }
    const groupID = (plan.group_id || '').toString().trim();
    setGroupOptions((current) =>
      appendGroupOptionIfMissing(current, groupID, plan.group_name || ''),
    );
    setForm({
      ...createTopupEditForm(),
      id: plan.id || '',
      kind: plan.kind || 'balance',
      name: plan.name || '',
      description: plan.description || '',
      group_id: groupID,
      sale_price: Number(plan.sale_price ?? plan.amount ?? 0) || 0,
      sale_currency: plan.sale_currency || plan.amount_currency || 'CNY',
      quota_amount: Number(plan.quota_amount || 0) || 0,
      quota_currency: plan.quota_currency || 'YYC',
      validity_days: Number(plan.validity_days || plan.duration_days || 0) || 0,
      max_concurrency_per_user: Number(plan.max_concurrency_per_user || 0) || 0,
      max_concurrency_per_package: Number(plan.max_concurrency_per_package || 0) || 0,
      visibility_scope: visibilityScope,
      visible_user_ids: visibilityUserIDs,
      enabled: plan.enabled !== false,
      sort_order: Number(plan.sort_order || 0) || 0,
      source: plan.source || 'manual',
    });
    setEditing(true);
  };

  const cancelEditing = () => {
    if (submitting) {
      return;
    }
    setEditing(false);
  };

  const submitEdit = async () => {
    const name = (form.name || '').toString().trim();
    if (!name) {
      showInfo(t('package_manage.messages.name_required'));
      return;
    }
    const groupID = (form.group_id || '').toString().trim();
    if (!groupID) {
      showInfo(t('package_manage.messages.group_required'));
      return;
    }
    const salePrice = Number(form.sale_price || 0);
    const quotaAmount = Number(form.quota_amount || 0);
    const validityDays = Math.trunc(Number(form.validity_days || 0));
    const maxConcurrencyPerUser = Math.trunc(Number(form.max_concurrency_per_user || 0));
    const maxConcurrencyPerPackage = Math.trunc(Number(form.max_concurrency_per_package || 0));
    if (
      !Number.isFinite(salePrice) ||
      salePrice < 0 ||
      !Number.isFinite(quotaAmount) ||
      quotaAmount < 0 ||
      !Number.isFinite(validityDays) ||
      validityDays < 0 ||
      !Number.isFinite(maxConcurrencyPerUser) ||
      maxConcurrencyPerUser < 0 ||
      !Number.isFinite(maxConcurrencyPerPackage) ||
      maxConcurrencyPerPackage < 0
    ) {
      showInfo(t('package_manage.messages.quota_invalid'));
      return;
    }
    setSubmitting(true);
    try {
      const payload = {
        id: form.id || plan?.id || '',
        kind: 'balance',
        name,
        description: (form.description || '').toString().trim(),
        group_id: groupID,
        sale_price: salePrice,
        sale_currency: (form.sale_currency || 'CNY').toString().trim().toUpperCase() || 'CNY',
        quota_metric: 'yyc',
        quota_amount: quotaAmount,
        quota_currency: (form.quota_currency || 'YYC').toString().trim().toUpperCase() || 'YYC',
        period_type: 'none',
        period_limit: 0,
        duration_days: validityDays,
        validity_days: validityDays,
        max_concurrency_per_user: maxConcurrencyPerUser,
        max_concurrency_per_package: maxConcurrencyPerPackage,
        allow_balance_fallback: false,
        visibility_scope: visibilityScope,
        visible_user_ids: visibilityUserIDs,
        enabled: form.enabled !== false,
        sort_order: Math.trunc(Number(form.sort_order || 0)),
        source: form.source || 'manual',
      };
      const res = await API.put(
        `/api/v1/admin/entitlement/products/${encodeURIComponent(payload.id)}`,
        payload,
      );
      const { success, message, data } = res?.data || {};
      if (!success) {
        showError(message || t('package_manage.messages.update_failed'));
        return;
      }
      const nextPlan = productToTopupPlan(data);
      setPlan(nextPlan);
      setVisibilityScope(nextPlan.visibility_scope === 'partial_users' ? 'partial_users' : 'all');
      setVisibilityUserIDs(
        Array.isArray(nextPlan.visible_user_ids)
          ? nextPlan.visible_user_ids.map((item) => (item || '').toString().trim()).filter(Boolean)
          : [],
      );
      setUserOptions((current) => appendUserOptionsIfMissing(current, nextPlan.visible_users));
      setEditing(false);
      showSuccess(t('package_manage.messages.update_success'));
    } catch (error) {
      showError(error?.message || t('package_manage.messages.update_failed'));
    } finally {
      setSubmitting(false);
    }
  };

  const renderBasicInfo = () => (
    <div className='router-page-stack'>
      <AppFormRow>
        <AppField label={t('redemption.table.id')} readOnly>
          <AppInput
            className='router-section-input router-monospace-value'
            value={readOnlyText(plan?.id || id)}
            readOnly
          />
        </AppField>
        <AppField label={t('topup.manage.columns.name')} readOnly>
          <AppInput className='router-section-input' value={readOnlyText(plan?.name)} readOnly />
        </AppField>
        <AppField label={t('topup.manage.columns.group')} readOnly>
          <AppInput
            className='router-section-input'
            value={readOnlyText(plan?.group_name || plan?.group_id)}
            readOnly
          />
        </AppField>
      </AppFormRow>
      <AppFormRow>
        <AppField label={t('package_manage.form.description')} readOnly>
          <AppTextarea
            className='router-section-input'
            value={readOnlyText(plan?.description)}
            rows={3}
            readOnly
          />
        </AppField>
      </AppFormRow>
      <AppFormRow>
        <AppField label={t('topup.manage.columns.pay_amount')} readOnly>
          <AppInput
            className='router-section-input'
            value={formatAmountWithUnit(plan?.amount || 0, plan?.amount_currency || '', 6)}
            readOnly
          />
        </AppField>
        <AppField label={t('topup.manage.columns.credited_amount')} readOnly>
          <AppInput
            className='router-section-input'
            value={formatAmountWithUnit(plan?.quota_amount || 0, plan?.quota_currency || 'YYC', 6)}
            readOnly
          />
        </AppField>
      </AppFormRow>
      <AppFormRow>
        <AppField label={t('topup.manage.columns.concurrency_limit')} readOnly>
          <AppInput
            className='router-section-input'
            value={formatPackageConcurrencyLimit(plan, t, t('common.unlimited'))}
            readOnly
          />
        </AppField>
        <AppField label={t('topup.manage.columns.validity_days')} readOnly>
          <AppInput
            className='router-section-input'
            value={
              Number(plan?.validity_days || 0) > 0
                ? `${Number(plan?.validity_days || 0)} ${t('common.day')}`
                : t('common.never')
            }
            readOnly
          />
        </AppField>
        <AppField label={t('topup.manage.columns.enabled')} readOnly>
          <AppInput
            className='router-section-input'
            value={
              Boolean(plan?.enabled)
                ? t('package_manage.status.enabled')
                : t('package_manage.status.disabled')
            }
            readOnly
          />
        </AppField>
      </AppFormRow>
      <AppFormRow>
        <AppField label={t('topup.manage.columns.created_at')} readOnly>
          <AppInput className='router-section-input' value={formatDateTime(plan?.created_at)} readOnly />
        </AppField>
        <AppField label={t('topup.manage.columns.updated_at')} readOnly>
          <AppInput className='router-section-input' value={formatDateTime(plan?.updated_at)} readOnly />
        </AppField>
      </AppFormRow>
    </div>
  );

  const renderEditForm = () => (
    <div className='router-page-stack'>
      <AppFormRow className='router-modal-form-row'>
        <AppField label={t('redemption.table.id')} readOnly>
          <AppInput
            className='router-section-input router-monospace-value'
            value={readOnlyText(plan?.id || id)}
            readOnly
            disabled
          />
        </AppField>
        <AppField label={t('topup.manage.columns.name')} required>
          <AppInput
            className='router-section-input'
            value={form.name}
            onChange={(_, { value }) =>
              setForm((current) => ({ ...current, name: value || '' }))
            }
          />
        </AppField>
        <AppField label={t('topup.manage.columns.group')} required>
          <AppSelect
            className='router-section-dropdown'
            options={groupOptions}
            value={form.group_id}
            loading={groupLoading}
            search
            onChange={(_, { value }) =>
              setForm((current) => ({ ...current, group_id: (value || '').toString() }))
            }
          />
        </AppField>
      </AppFormRow>
      <AppFormRow className='router-modal-form-row'>
        <AppField label={t('package_manage.form.description')}>
          <AppTextarea
            className='router-section-input'
            value={form.description}
            rows={3}
            onChange={(_, { value }) =>
              setForm((current) => ({ ...current, description: value || '' }))
            }
          />
        </AppField>
      </AppFormRow>
      <AppFormRow className='router-modal-form-row'>
        <AppField label={t('topup.manage.columns.pay_amount')} required>
          <AppInputNumber
            className='router-section-input'
            min={0}
            precision={2}
            step={0.01}
            fluid
            value={form.sale_price}
            onChange={(_, { value }) =>
              setForm((current) => ({ ...current, sale_price: Number(value || 0) }))
            }
          />
        </AppField>
        <AppField label={t('package_manage.form.sale_currency')} required>
          <AppInput
            className='router-section-input'
            value={form.sale_currency}
            onChange={(_, { value }) =>
              setForm((current) => ({
                ...current,
                sale_currency: (value || '').toString().toUpperCase(),
              }))
            }
          />
        </AppField>
      </AppFormRow>
      <AppFormRow className='router-modal-form-row'>
        <AppField label={t('topup.manage.columns.credited_amount')} required>
          <AppInputNumber
            className='router-section-input'
            min={0}
            precision={6}
            step={0.01}
            fluid
            value={form.quota_amount}
            onChange={(_, { value }) =>
              setForm((current) => ({ ...current, quota_amount: Number(value || 0) }))
            }
          />
        </AppField>
        <AppField label={t('topup.manage.columns.credited_unit')}>
          <AppInput
            className='router-section-input'
            value={form.quota_currency}
            onChange={(_, { value }) =>
              setForm((current) => ({
                ...current,
                quota_currency: (value || '').toString().toUpperCase(),
              }))
            }
          />
        </AppField>
      </AppFormRow>
      <AppFormRow className='router-modal-form-row'>
        <AppField label={t('topup.manage.columns.validity_days')}>
          <AppInputNumber
            className='router-section-input'
            min={0}
            precision={0}
            step={1}
            fluid
            value={form.validity_days}
            onChange={(_, { value }) =>
              setForm((current) => ({ ...current, validity_days: Number(value || 0) }))
            }
          />
        </AppField>
        <AppField label={t('package_manage.form.max_concurrency_per_user')}>
          <AppInputNumber
            className='router-section-input'
            min={0}
            precision={0}
            step={1}
            fluid
            value={form.max_concurrency_per_user}
            onChange={(_, { value }) =>
              setForm((current) => ({
                ...current,
                max_concurrency_per_user: Number(value || 0),
              }))
            }
          />
        </AppField>
        <AppField label={t('package_manage.form.max_concurrency_per_package')}>
          <AppInputNumber
            className='router-section-input'
            min={0}
            precision={0}
            step={1}
            fluid
            value={form.max_concurrency_per_package}
            onChange={(_, { value }) =>
              setForm((current) => ({
                ...current,
                max_concurrency_per_package: Number(value || 0),
              }))
            }
          />
        </AppField>
      </AppFormRow>
      <AppFormRow className='router-modal-form-row'>
        <AppField label={t('topup.manage.columns.enabled')}>
          <AppSwitch
            checked={form.enabled !== false}
            onChange={(_, { checked }) =>
              setForm((current) => ({ ...current, enabled: Boolean(checked) }))
            }
          />
        </AppField>
        <AppField label={t('package_manage.form.sort_order')}>
          <AppInputNumber
            className='router-section-input'
            min={0}
            precision={0}
            step={1}
            fluid
            value={form.sort_order}
            onChange={(_, { value }) =>
              setForm((current) => ({ ...current, sort_order: Number(value || 0) }))
            }
          />
        </AppField>
      </AppFormRow>
      <AppFormRow className='router-modal-form-row'>
        <AppField label={t('topup.manage.columns.created_at')} readOnly>
          <AppInput
            className='router-section-input'
            value={formatDateTime(plan?.created_at)}
            readOnly
            disabled
          />
        </AppField>
        <AppField label={t('topup.manage.columns.updated_at')} readOnly>
          <AppInput
            className='router-section-input'
            value={formatDateTime(plan?.updated_at)}
            readOnly
            disabled
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
          ...(redemptionSourcePath
            ? [
                { key: 'operation', label: t('header.operation') },
                {
                  key: 'redemption-source',
                  label: t('header.redemption'),
                  onClick: () => navigate('/admin/redemption'),
                },
                ...(redemptionSourcePath !== '/admin/redemption'
                  ? [{
                      key: 'redemption-detail-source',
                      label: redemptionSourcePath.split('/').filter(Boolean).pop(),
                      onClick: () => navigate(redemptionSourcePath),
                    }]
                  : []),
              ]
            : []),
          ...(redemptionSourcePath ? [] : [
            { key: 'model', label: t('header.model') },
            {
              key: 'entitlement',
              label: t('header.entitlement'),
              onClick: () => navigate('/admin/entitlement'),
            },
          ]),
          {
            key: 'topup-current',
            label: readOnlyText(plan?.id || id),
            active: true,
          },
        ]}
        title={t('topup.manage.detail_title')}
      />
      <div className='router-entity-detail-page router-tab-detail-page'>
        <div className='router-entity-detail-tabs router-block-gap-sm'>
          <AppTabs
            activeKey={activeTabKey}
            onChange={(key) => setActiveTabKey((key || 'basic').toString())}
            items={[
              {
                key: 'basic',
                label: t('common.basic_info'),
                children: (
                  <AppDetailSection
                    title={t('common.basic_info')}
                    titleTag='div'
                    headerEnd={
                      editing ? (
                        <>
                          <AppButton
                            type='button'
                            className='router-page-button'
                            onClick={cancelEditing}
                            disabled={submitting}
                          >
                            {t('common.cancel')}
                          </AppButton>
                          <AppButton
                            type='button'
                            className='router-page-button'
                            color='blue'
                            loading={submitting}
                            disabled={submitting}
                            onClick={submitEdit}
                          >
                            {t('common.confirm')}
                          </AppButton>
                        </>
                      ) : (
                        <AppButton
                          type='button'
                          className='router-page-button'
                          color='blue'
                          disabled={loading || !plan}
                          onClick={startEditing}
                        >
                          {t('package_manage.buttons.edit')}
                        </AppButton>
                      )
                    }
                  >
                    {loading ? (
                      <div className='router-empty-cell'>{t('common.loading')}</div>
                    ) : editing ? (
                      renderEditForm()
                    ) : (
                      renderBasicInfo()
                    )}
                  </AppDetailSection>
                ),
              },
              {
                key: 'models',
                label: t('topup.manage.columns.applicable_models'),
                children: (
                  <AppDetailSection title={t('topup.manage.columns.applicable_models')} titleTag='div'>
                    {loading ? (
                      <div className='router-empty-cell'>{t('common.loading')}</div>
                    ) : supportedModels.length > 0 ? (
                      <div className='router-entity-list'>
                        {supportedModels.map((model, index) => (
                          <div key={`${model}-${index}`} className='router-entity-list-item'>
                            <div className='router-entity-list-title router-monospace-value'>
                              {model}
                            </div>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className='router-text-muted'>-</div>
                    )}
                  </AppDetailSection>
                ),
              },
              {
                key: 'visibility',
                label: t('package_manage.form.visibility_scope'),
                children: (
                  <AppDetailSection title={t('package_manage.form.visibility_scope')} titleTag='div'>
                    {loading ? (
                      <div className='router-empty-cell'>{t('common.loading')}</div>
                    ) : (
                      <div className='router-page-stack'>
                        <div className='router-detail-grid'>
                          <div className='router-detail-item'>
                            <div className='router-detail-label'>
                              {t('package_manage.form.visibility_scope')}
                            </div>
                            <div className='router-detail-value'>
                              <AppSelect
                                className='router-section-dropdown'
                                options={visibilityOptions}
                                value={visibilityScope}
                                disabled={visibilitySubmitting}
                                onClick={() => {
                                  if (userOptions.length === 0) {
                                    loadInitialUsers().then();
                                  }
                                }}
                                onChange={(_, { value }) => {
                                  const nextScope = (value || 'all').toString();
                                  persistVisibility(nextScope, visibilityUserIDs).then();
                                }}
                              />
                            </div>
                          </div>
                        </div>
                        {visibilityScope === 'partial_users' ? (
                          <>
                            <div className='router-inline-actions'>
                              <AppButton
                                type='button'
                                className='router-page-button'
                                onClick={openVisibilityPicker}
                                disabled={userLoading || visibilitySubmitting}
                              >
                                {t('common.add')}
                              </AppButton>
                            </div>
                            {visibilityTableRows.length > 0 ? (
                              <AppTable
                                className='router-list-table router-table-fit-page'
                                pagination={false}
                                rowKey='id'
                                dataSource={visibilityTableRows}
                                columns={[
                                  {
                                    title: t('user.table.username'),
                                    dataIndex: 'name',
                                    key: 'name',
                                    render: (value) => value || '-',
                                  },
                                  {
                                    title: t('user.table.wallet'),
                                    dataIndex: 'walletAddress',
                                    key: 'walletAddress',
                                    render: (value) => (
                                      <span className='router-monospace-value'>
                                        {value || '-'}
                                      </span>
                                    ),
                                  },
                                  {
                                    title: t('redemption.table.actions'),
                                    key: 'actions',
                                    className: 'router-table-col-actions-icon',
                                    width: 52,
                                    render: (_, row) => (
                                      <AppTableActionButton
                                        icon='trash'
                                        title={t('common.delete')}
                                        color='red'
                                        disabled={visibilitySubmitting}
                                        onClick={() => removeVisibleUser(row.id)}
                                      />
                                    ),
                                  },
                                ]}
                              />
                            ) : (
                              <div className='router-text-muted'>-</div>
                            )}
                          </>
                        ) : (
                          <div className='router-text-muted'>
                            {formatVisibilityScope(visibilityScope, t)}
                          </div>
                        )}
                      </div>
                    )}
                  </AppDetailSection>
                ),
              },
            ]}
          />
        </div>
      </div>
      <AppModal
        open={visibilityPickerOpen}
        onClose={closeVisibilityPicker}
        size='small'
        title={t('common.add')}
        footer={null}
      >
        <div className='router-page-stack'>
          <AppSelect
            className='router-section-input'
            placeholder={t('package_manage.form.visible_users_placeholder')}
            options={userOptions}
            value={visibilityPickerValue}
            loading={userLoading}
            multiple
            search
            clearable
            onClick={() => {
              if (userOptions.length === 0) {
                loadInitialUsers().then();
              }
            }}
            onSearch={searchUsers}
            onChange={(_, { value }) =>
              setVisibilityPickerValue(Array.isArray(value) ? value : [])
            }
          />
          <AppFormActions>
            <AppButton type='button' onClick={closeVisibilityPicker}>
              {t('common.cancel')}
            </AppButton>
            <AppButton type='button' color='blue' onClick={confirmVisibilityPicker}>
              {t('common.confirm')}
            </AppButton>
          </AppFormActions>
        </div>
      </AppModal>
    </div>
  );
};

export default TopupPlanDetail;
