import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Breadcrumb, Button, Card, Dropdown, Form, Header, Icon, Label, Modal, Table } from 'semantic-ui-react';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { API, copy, isRoot, showError, showInfo, showSuccess } from '../../helpers';
import {
  buildBillingCurrencyIndex,
  buildBillingUnitOptions,
  yycToBillingInputValue,
  resolveDefaultBillingUnit,
  resolveBillingInputStep,
} from '../../helpers/billing';
import UnitDropdown from '../../components/UnitDropdown';
import {
  formatAmountWithUnit,
} from '../../helpers/render';

const ROLE_OPTIONS = (t) => [
  { key: 1, value: 1, text: t('user.table.role_types.normal') },
  { key: 10, value: 10, text: t('user.table.role_types.admin') },
];

const renderRoleLabel = (role, t) => {
  switch (Number(role)) {
    case 1:
      return <Label className='router-tag'>{t('user.table.role_types.normal')}</Label>;
    case 10:
      return (
        <Label color='yellow' className='router-tag'>
          {t('user.table.role_types.admin')}
        </Label>
      );
    default:
      return (
        <Label color='red' className='router-tag'>
          {t('user.table.role_types.unknown')}
        </Label>
      );
  }
};

const renderStatusLabel = (status, t) => {
  switch (Number(status)) {
    case 1:
      return (
        <Label basic className='router-tag'>
          {t('user.table.status_types.activated')}
        </Label>
      );
    case 2:
      return (
        <Label basic color='red' className='router-tag'>
          {t('user.table.status_types.banned')}
        </Label>
      );
    default:
      return (
        <Label basic color='grey' className='router-tag'>
          {t('user.table.status_types.unknown')}
        </Label>
      );
  }
};

const readOnlyValue = (value) => {
  const normalized = (value || '').toString().trim();
  return normalized || '-';
};

const formatDateTime = (timestamp) => {
  const value = Number(timestamp || 0);
  if (!Number.isFinite(value) || value <= 0) {
    return '-';
  }
  return new Date(value * 1000).toLocaleString('zh-CN', { hour12: false });
};

const formatCountValue = (value) => {
  const normalized = Number(value || 0);
  if (!Number.isFinite(normalized)) {
    return '0';
  }
  return normalized.toLocaleString();
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

const renderPackageStatusLabel = (status, t) => {
  switch (Number(status)) {
    case 1:
      return (
        <Label basic color='green' className='router-tag'>
          {t('user.detail.package_status_types.active')}
        </Label>
      );
    case 2:
      return (
        <Label basic color='grey' className='router-tag'>
          {t('user.detail.package_status_types.expired')}
        </Label>
      );
    case 3:
      return (
        <Label basic color='blue' className='router-tag'>
          {t('user.detail.package_status_types.replaced')}
        </Label>
      );
    case 4:
      return (
        <Label basic color='red' className='router-tag'>
          {t('user.detail.package_status_types.canceled')}
        </Label>
      );
    case 5:
      return (
        <Label basic color='teal' className='router-tag'>
          {t('user.detail.package_status_types.pending')}
        </Label>
      );
    default:
      return (
        <Label basic color='grey' className='router-tag'>
          {t('user.detail.package_status_types.unknown')}
        </Label>
      );
  }
};

const renderBalanceLotStatusLabel = (status, t) => {
  switch ((status || '').toString().trim()) {
    case 'active':
      return (
        <Label basic color='green' className='router-tag'>
          {t('topup.balance_lots.status.active')}
        </Label>
      );
    case 'exhausted':
      return (
        <Label basic color='grey' className='router-tag'>
          {t('topup.balance_lots.status.exhausted')}
        </Label>
      );
    case 'expired':
      return (
        <Label basic color='orange' className='router-tag'>
          {t('topup.balance_lots.status.expired')}
        </Label>
      );
    default:
      return (
        <Label basic color='grey' className='router-tag'>
          {readOnlyValue(status)}
        </Label>
      );
  }
};

const formatBalanceLotSource = (sourceType, t) => {
  switch ((sourceType || '').toString().trim()) {
    case 'topup_order':
      return t('topup.balance_lots.source.topup_order');
    case 'redemption':
      return t('topup.balance_lots.source.redemption');
    case 'legacy_migration':
      return t('topup.balance_lots.source.legacy_migration');
    default:
      return readOnlyValue(sourceType);
  }
};

const createEmptyActivePackage = () => ({
  has_active_subscription: false,
  subscription: null,
});

const createEmptyDailySnapshot = () => ({
  biz_date: '',
  timezone: '',
  limit: 0,
  consumed_quota: 0,
  reserved_quota: 0,
  remaining_quota: 0,
  unlimited: false,
});

const createEmptyQuotaSummary = () => ({
  package_emergency: {
    biz_month: '',
    timezone: '',
    limit: 0,
    consumed_quota: 0,
    reserved_quota: 0,
    remaining_quota: 0,
    enabled: false,
  },
});

const normalizeDailySnapshot = (raw) => ({
  biz_date: (raw?.biz_date || '').toString().trim(),
  timezone: (raw?.timezone || '').toString().trim(),
  limit: Number(raw?.yyc_limit ?? raw?.limit ?? 0) || 0,
  consumed_quota: Number(raw?.yyc_consumed ?? raw?.consumed_quota ?? 0) || 0,
  reserved_quota: Number(raw?.yyc_reserved ?? raw?.reserved_quota ?? 0) || 0,
  remaining_quota: Number(raw?.yyc_remaining ?? raw?.remaining_quota ?? 0) || 0,
  unlimited: raw?.unlimited === true,
});

const normalizeQuotaSummary = (raw) => ({
  package_emergency: {
    biz_month: (raw?.package_emergency?.biz_month || '').toString().trim(),
    timezone: (raw?.package_emergency?.timezone || '').toString().trim(),
    limit: Number(raw?.package_emergency?.yyc_limit ?? raw?.package_emergency?.limit ?? 0) || 0,
    consumed_quota:
      Number(
        raw?.package_emergency?.yyc_consumed ??
          raw?.package_emergency?.consumed_quota ??
          0,
      ) || 0,
    reserved_quota:
      Number(
        raw?.package_emergency?.yyc_reserved ??
          raw?.package_emergency?.reserved_quota ??
          0,
      ) || 0,
    remaining_quota:
      Number(
        raw?.package_emergency?.yyc_remaining ??
          raw?.package_emergency?.remaining_quota ??
          0,
      ) || 0,
    enabled: raw?.package_emergency?.enabled === true,
  },
});

const normalizeActivePackage = (raw) => {
  if (!raw || typeof raw !== 'object') {
    return createEmptyActivePackage();
  }
  const subscription =
    raw.subscription && typeof raw.subscription === 'object'
      ? {
          id: (raw.subscription.id || '').toString().trim(),
          user_id: (raw.subscription.user_id || '').toString().trim(),
          package_id: (raw.subscription.package_id || '').toString().trim(),
          package_name: (raw.subscription.package_name || '').toString().trim(),
          group_id: (raw.subscription.group_id || '').toString().trim(),
          group_name: (raw.subscription.group_name || '').toString().trim(),
          daily_amount: Number(raw.subscription.daily_quota_limit || 0),
          emergency_amount: Number(raw.subscription.package_emergency_quota_limit ?? 0),
          reset_timezone: (raw.subscription.quota_reset_timezone || '').toString().trim(),
          started_at: Number(raw.subscription.started_at || 0),
          expires_at: Number(raw.subscription.expires_at || 0),
          status: Number(raw.subscription.status || 0),
          source: (raw.subscription.source || '').toString().trim(),
        }
      : null;
  const hasActive = raw.has_active_subscription === true && subscription !== null;
  return {
    has_active_subscription: hasActive,
    subscription: hasActive ? subscription : null,
  };
};

const toPackageOptions = (rows) =>
  (Array.isArray(rows) ? rows : []).map((item) => {
    const id = (item?.id || '').toString().trim();
    const name = (item?.name || '').toString().trim() || id;
    const groupName =
      (item?.group_name || '').toString().trim() ||
      (item?.group_id || '').toString().trim();
    return {
      key: id,
      value: id,
      text: groupName ? `${name} (${groupName})` : name,
    };
  });

const toTopupPlanOptions = (rows, t) =>
  (Array.isArray(rows) ? rows : [])
    .filter((item) => Boolean(item?.enabled))
    .map((item) => {
      const id = (item?.id || '').toString().trim();
      const amount = formatPlanNumber(item?.amount || 0);
      const amountCurrency = (item?.amount_currency || '').toString().trim().toUpperCase();
      const quotaAmount = formatPlanNumber(item?.quota_amount || 0);
      const quotaCurrency = (item?.quota_currency || '').toString().trim().toUpperCase();
      const validityDays = Number(item?.validity_days || 0);
      const labelParts = [`${amount} ${amountCurrency}`, `${quotaAmount} ${quotaCurrency}`];
      if (validityDays > 0) {
        labelParts.push(`${validityDays}${t('common.day')}`);
      } else {
        labelParts.push(t('common.never'));
      }
      return {
        key: id,
        value: id,
        text: labelParts.join(' / '),
      };
    })
    .filter((option) => option.value);

const parseDatetimeLocalValue = (value) => {
  if (typeof value !== 'string' || value.trim() === '') {
    return 0;
  }
  const ts = Date.parse(value.trim());
  if (!Number.isFinite(ts)) {
    return NaN;
  }
  return Math.floor(ts / 1000);
};

const UserDetail = () => {
  const { t } = useTranslation();
  const { id: userId } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [editSection, setEditSection] = useState('');
  const [actionLoading, setActionLoading] = useState('');
  const [persistedUsername, setPersistedUsername] = useState('');
  const [billingCurrencyIndex, setBillingCurrencyIndex] = useState(
    buildBillingCurrencyIndex([], { activeOnly: true })
  );
  const [balanceUnit, setBalanceUnit] = useState('USD');
  const [activePackage, setActivePackage] = useState(createEmptyActivePackage());
  const [activePackageLoading, setActivePackageLoading] = useState(false);
  const [packageDailySnapshot, setPackageDailySnapshot] = useState(createEmptyDailySnapshot());
  const [packageQuotaSummary, setPackageQuotaSummary] = useState(createEmptyQuotaSummary());
  const [balanceLots, setBalanceLots] = useState([]);
  const [balanceLotsLoading, setBalanceLotsLoading] = useState(false);
  const [balanceLotFilters, setBalanceLotFilters] = useState({
    source_type: '',
    status: 'active',
    positive_only: true,
  });
  const [packageOptions, setPackageOptions] = useState([]);
  const [packageOptionsLoading, setPackageOptionsLoading] = useState(false);
  const [assignPackageOpen, setAssignPackageOpen] = useState(false);
  const [assignPackageForm, setAssignPackageForm] = useState({
    package_id: '',
    start_at: '',
  });
  const [topupPlanOptions, setTopupPlanOptions] = useState([]);
  const [topupPlanOptionsLoading, setTopupPlanOptionsLoading] = useState(false);
  const [assignTopupOpen, setAssignTopupOpen] = useState(false);
  const [assignTopupForm, setAssignTopupForm] = useState({
    plan_id: '',
  });
  const [inputs, setInputs] = useState({
    username: '',
    email: '',
    yyc_balance: 0,
    group: '',
    reset_timezone: 'Asia/Shanghai',
    role: 1,
    status: 1,
    wallet_address: '',
    yyc_used: 0,
    request_count: 0,
    can_manage_users: false,
    created_at: 0,
    updated_at: 0,
  });
  const [basicEditInputs, setBasicEditInputs] = useState({
    username: '',
    email: '',
  });
  const returnPath = useMemo(() => {
    const from = location.state?.from;
    if (typeof from !== 'string') {
      return '';
    }
    const normalized = from.trim();
    return normalized.startsWith('/') ? normalized : '';
  }, [location.state]);

  const loadUser = useCallback(async () => {
    if (!userId) {
      navigate('/user', { replace: true });
      return;
    }
    setLoading(true);
    try {
      const res = await API.get(`/api/v1/admin/user/${userId}`);
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message);
        return;
      }
      const walletAddress =
        typeof data?.wallet_address === 'string'
          ? data.wallet_address
          : data?.wallet_address || '';
      const nextInputs = {
        username: data?.username || '',
        email: data?.email || '',
        yyc_balance: Number(data?.yyc_balance ?? data?.quota ?? 0),
        group: data?.group || '',
        reset_timezone: data?.quota_reset_timezone || 'Asia/Shanghai',
        role: Number(data?.role || 1),
        status: Number(data?.status || 1),
        wallet_address: walletAddress,
        yyc_used: Number(data?.yyc_used ?? data?.used_quota ?? 0),
        request_count: data?.request_count ?? 0,
        can_manage_users: data?.can_manage_users === true,
        created_at: Number(data?.created_at || 0),
        updated_at: Number(data?.updated_at || 0),
      };
      setInputs(nextInputs);
      setBasicEditInputs({
        username: nextInputs.username,
        email: nextInputs.email,
      });
      setPersistedUsername((data?.username || '').toString().trim());
      setEditSection('');
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setLoading(false);
    }
  }, [navigate, userId]);

  const loadActivePackage = useCallback(async () => {
    const normalizedUserId = (userId || '').toString().trim();
    if (normalizedUserId === '') {
      setActivePackage(createEmptyActivePackage());
      return;
    }
    setActivePackageLoading(true);
    try {
      const res = await API.get(
        `/api/v1/admin/user/${encodeURIComponent(normalizedUserId)}/package/subscription`,
      );
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('user.messages.active_package_load_failed'));
        return;
      }
      const normalizedPackage = normalizeActivePackage(data);
      setActivePackage(normalizedPackage);
      if (!normalizedPackage.has_active_subscription) {
        setPackageDailySnapshot(createEmptyDailySnapshot());
        setPackageQuotaSummary(createEmptyQuotaSummary());
        return;
      }
      const normalizedGroupId = (
        normalizedPackage.subscription?.group_id || ''
      ).toString().trim();
      const [dailyRes, summaryRes] = await Promise.all([
        API.get(`/api/v1/admin/group/${encodeURIComponent(normalizedGroupId)}/quota/daily`, {
          params: {
            user_id: normalizedUserId,
          },
        }),
        API.get(`/api/v1/admin/user/${encodeURIComponent(normalizedUserId)}/quota/summary`),
      ]);
      const dailyPayload = dailyRes?.data || {};
      if (dailyPayload.success) {
        setPackageDailySnapshot(normalizeDailySnapshot(dailyPayload.data));
      } else {
        setPackageDailySnapshot(createEmptyDailySnapshot());
      }
      const summaryPayload = summaryRes?.data || {};
      if (summaryPayload.success) {
        setPackageQuotaSummary(normalizeQuotaSummary(summaryPayload.data));
      } else {
        setPackageQuotaSummary(createEmptyQuotaSummary());
      }
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setActivePackageLoading(false);
    }
  }, [t, userId]);

  const loadBalanceLots = useCallback(
    async ({ silent = false } = {}) => {
      const normalizedUserId = (userId || '').toString().trim();
      if (normalizedUserId === '') {
        setBalanceLots([]);
        return;
      }
      if (!silent) {
        setBalanceLotsLoading(true);
      }
      try {
        const res = await API.get(
          `/api/v1/admin/user/${encodeURIComponent(normalizedUserId)}/topup/balance/lots`,
          {
            params: {
              page: 1,
              page_size: 20,
              source_type: (balanceLotFilters.source_type || '').toString().trim() || undefined,
              status: (balanceLotFilters.status || '').toString().trim() || undefined,
              positive_only: balanceLotFilters.positive_only !== false,
            },
          },
        );
        const { success, message, data } = res.data || {};
        if (!success) {
          if (!silent) {
            showError(message || t('user.messages.operation_failed'));
          }
          return;
        }
        setBalanceLots(Array.isArray(data?.items) ? data.items : []);
      } catch (error) {
        if (!silent) {
          showError(error?.message || error);
        }
      } finally {
        if (!silent) {
          setBalanceLotsLoading(false);
        }
      }
    },
    [
      balanceLotFilters.positive_only,
      balanceLotFilters.source_type,
      balanceLotFilters.status,
      t,
      userId,
    ],
  );

  const loadPackageOptions = useCallback(async () => {
    if (packageOptions.length > 0) {
      return;
    }
    setPackageOptionsLoading(true);
    try {
      const items = [];
      let page = 1;
      while (page <= 50) {
        const res = await API.get('/api/v1/admin/packages', {
          params: {
            page,
            page_size: 100,
          },
        });
        const { success, message, data } = res.data || {};
        if (!success) {
          showError(message || t('package_manage.messages.load_failed'));
          return;
        }
        const pageItems = Array.isArray(data?.items) ? data.items : [];
        items.push(...pageItems);
        const total = Number(data?.total || pageItems.length || 0);
        if (
          pageItems.length === 0 ||
          items.length >= total ||
          pageItems.length < 100
        ) {
          break;
        }
        page += 1;
      }
      setPackageOptions(toPackageOptions(items));
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setPackageOptionsLoading(false);
    }
  }, [packageOptions.length, t]);

  const loadTopupPlanOptions = useCallback(async () => {
    if (topupPlanOptions.length > 0) {
      return;
    }
    setTopupPlanOptionsLoading(true);
    try {
      const res = await API.get('/api/v1/admin/topup/plans');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('topup.manage.load_failed'));
        return;
      }
      setTopupPlanOptions(toTopupPlanOptions(data, t));
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setTopupPlanOptionsLoading(false);
    }
  }, [topupPlanOptions.length, t]);

  const loadBillingCurrencies = useCallback(async () => {
    try {
      const res = await API.get('/api/v1/admin/billing/currencies');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('user.messages.operation_failed'));
        return;
      }
      const next = buildBillingCurrencyIndex(Array.isArray(data) ? data : [], {
        activeOnly: true,
      });
      setBillingCurrencyIndex(next);
      setBalanceUnit((current) => {
        const normalizedCurrent = (current || '').toString().trim().toUpperCase();
        if (normalizedCurrent && next[normalizedCurrent]) {
          return normalizedCurrent;
        }
        return resolveDefaultBillingUnit(next);
      });
    } catch (error) {
      showError(error?.message || error);
    }
  }, [t]);

  useEffect(() => {
    const init = async () => {
      await loadBillingCurrencies();
      await loadUser();
    };
    init().then();
  }, [loadBillingCurrencies, loadUser]);
  const billingUnitOptions = useMemo(
    () => buildBillingUnitOptions(billingCurrencyIndex),
    [billingCurrencyIndex],
  );
  const balanceInputStep = useMemo(
    () => resolveBillingInputStep(balanceUnit, billingCurrencyIndex),
    [balanceUnit, billingCurrencyIndex],
  );
  const balanceDisplayValue = useMemo(
    () => yycToBillingInputValue(inputs.yyc_balance, balanceUnit, billingCurrencyIndex),
    [balanceUnit, billingCurrencyIndex, inputs.yyc_balance],
  );
  const usedDisplayValue = useMemo(
    () => yycToBillingInputValue(inputs.yyc_used, balanceUnit, billingCurrencyIndex),
    [balanceUnit, billingCurrencyIndex, inputs.yyc_used],
  );

  const isProtectedUser = inputs.can_manage_users === true;
  const canManageRole = isRoot() && !isProtectedUser;
  const hasActivePackage = activePackage.has_active_subscription === true && activePackage.subscription;
  const activePackageSubscription = hasActivePackage ? activePackage.subscription : null;
  const packageEmergencySnapshot = packageQuotaSummary.package_emergency;
  const balanceLotSourceOptions = useMemo(
    () => [
      {
        key: 'all',
        value: '',
        text: t('user.detail.balance_lots.filters.all_source'),
      },
      {
        key: 'topup_order',
        value: 'topup_order',
        text: t('topup.balance_lots.source.topup_order'),
      },
      {
        key: 'redemption',
        value: 'redemption',
        text: t('topup.balance_lots.source.redemption'),
      },
      {
        key: 'legacy_migration',
        value: 'legacy_migration',
        text: t('topup.balance_lots.source.legacy_migration'),
      },
    ],
    [t],
  );
  const balanceLotStatusOptions = useMemo(
    () => [
      {
        key: 'all',
        value: '',
        text: t('user.detail.balance_lots.filters.all_status'),
      },
      {
        key: 'active',
        value: 'active',
        text: t('topup.balance_lots.status.active'),
      },
      {
        key: 'exhausted',
        value: 'exhausted',
        text: t('topup.balance_lots.status.exhausted'),
      },
      {
        key: 'expired',
        value: 'expired',
        text: t('topup.balance_lots.status.expired'),
      },
    ],
    [t],
  );
  const balanceLotPositiveOnlyOptions = useMemo(
    () => [
      {
        key: 'positive',
        value: '1',
        text: t('user.detail.balance_lots.filters.positive_only_yes'),
      },
      {
        key: 'all',
        value: '0',
        text: t('user.detail.balance_lots.filters.positive_only_no'),
      },
    ],
    [t],
  );

  useEffect(() => {
    loadActivePackage().then();
  }, [loadActivePackage]);

  useEffect(() => {
    loadBalanceLots({ silent: true }).then();
  }, [loadBalanceLots]);

  const roleControl = useMemo(() => {
    return (
      <Dropdown
        className='router-section-input'
        selection
        options={ROLE_OPTIONS(t)}
        value={Number(inputs.role || 1)}
        disabled={!canManageRole || loading || actionLoading !== '' || editSection !== ''}
        onChange={(e, { value }) => {
          const nextRole = Number(value);
          if (!Number.isFinite(nextRole) || nextRole === Number(inputs.role)) {
            return;
          }
          const action = nextRole === 10 ? 'promote' : 'demote';
          if (!persistedUsername || actionLoading !== '') {
            return;
          }
          setActionLoading(action);
          API.post('/api/v1/admin/user/manage', {
            username: persistedUsername,
            action,
          })
            .then((res) => {
              const { success, message } = res.data || {};
              if (!success) {
                showError(message);
                return;
              }
              showSuccess(t('user.messages.operation_success'));
              return loadUser();
            })
            .catch((error) => {
              showError(error?.message || error);
            })
            .finally(() => {
              setActionLoading('');
            });
        }}
      />
    );
  }, [
    actionLoading,
    canManageRole,
    editSection,
    inputs.role,
    loadUser,
    loading,
    persistedUsername,
    t,
  ]);

  const handleBasicEditInputChange = useCallback((e, { name, value }) => {
    setBasicEditInputs((prev) => ({
      ...prev,
      [name]: value,
    }));
  }, []);

  const handleBalanceUnitChange = useCallback(
    (nextUnit) => {
      const normalizedNextUnit = (nextUnit || '').toString().trim().toUpperCase();
      if (!normalizedNextUnit || normalizedNextUnit === balanceUnit) {
        return;
      }
      setBalanceUnit(normalizedNextUnit);
    },
    [balanceUnit],
  );

  const resetBasicEditInputs = useCallback(() => {
    setBasicEditInputs({
      username: inputs.username || '',
      email: inputs.email || '',
    });
  }, [
    inputs.email,
    inputs.username,
  ]);

  const startBasicEditing = useCallback(() => {
    resetBasicEditInputs();
    setEditSection('basic');
  }, [resetBasicEditInputs]);

  const cancelBasicEditing = useCallback(() => {
    resetBasicEditInputs();
    setEditSection('');
  }, [resetBasicEditInputs]);

  const updateUser = useCallback(async ({ username, email, group, yycBalance, actionKey }) => {
    if (username === '') {
      showError(t('user.edit.username_placeholder'));
      return false;
    }
    if (!Number.isFinite(yycBalance) || yycBalance < 0) {
      showError(t('user.messages.operation_failed'));
      return false;
    }
    setActionLoading(actionKey);
    try {
      const res = await API.put('/api/v1/admin/user/', {
        id: userId,
        username,
        email,
        group,
        quota: Math.trunc(yycBalance),
        quota_reset_timezone: inputs.reset_timezone || 'Asia/Shanghai',
        role: Number(inputs.role || 1),
        status: Number(inputs.status || 1),
        display_name: username,
        password: '',
      });
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('user.messages.operation_failed'));
        return false;
      }
      showSuccess(t('user.messages.update_success'));
      await loadUser();
      await loadActivePackage();
      setEditSection('');
      return true;
    } catch (error) {
      showError(error?.message || error);
      return false;
    } finally {
      setActionLoading('');
    }
  }, [
    inputs.role,
    inputs.status,
    inputs.reset_timezone,
    loadUser,
    loadActivePackage,
    t,
    userId,
  ]);

  const submitBasic = useCallback(async () => {
    const username = (basicEditInputs.username || '').toString().trim();
    const email = (basicEditInputs.email || '').toString().trim();
    await updateUser({
      username,
      email,
      group: (inputs.group || '').toString().trim(),
      yycBalance: Number(inputs.yyc_balance || 0),
      actionKey: 'save-basic',
    });
  }, [basicEditInputs.email, basicEditInputs.username, inputs.group, inputs.yyc_balance, updateUser]);

  const backToList = useCallback(() => {
    if (returnPath !== '') {
      navigate(-1);
      return;
    }
    navigate('/admin/user');
  }, [navigate, returnPath]);

  const openPackageManagement = useCallback(() => {
    const keyword = hasActivePackage
      ? (activePackageSubscription?.package_name || activePackageSubscription?.package_id || '')
          .toString()
          .trim()
      : '';
    const target = keyword !== '' ? `/admin/package?keyword=${encodeURIComponent(keyword)}` : '/admin/package';
    navigate(target);
  }, [activePackageSubscription?.package_id, activePackageSubscription?.package_name, hasActivePackage, navigate]);

  const copyWalletAddress = useCallback(async () => {
    const value = (inputs.wallet_address || '').toString().trim();
    if (value === '') {
      return;
    }
    if (await copy(value)) {
      showSuccess(t('user.messages.wallet_copy_success'));
      return;
    }
    showError(t('user.messages.wallet_copy_failed'));
  }, [inputs.wallet_address, t]);

  const refreshBalanceSection = useCallback(async () => {
    await Promise.all([loadUser(), loadBalanceLots()]);
  }, [loadBalanceLots, loadUser]);

  const openAssignPackageModal = useCallback(() => {
    setAssignPackageForm({
      package_id: '',
      start_at: '',
    });
    setAssignPackageOpen(true);
    loadPackageOptions().then();
  }, [loadPackageOptions]);

  const closeAssignPackageModal = useCallback(() => {
    if (actionLoading === 'assign-package') {
      return;
    }
    setAssignPackageOpen(false);
    setAssignPackageForm({
      package_id: '',
      start_at: '',
    });
  }, [actionLoading]);

  const submitAssignPackage = useCallback(async () => {
    const normalizedPackageID = (assignPackageForm.package_id || '').toString().trim();
    const normalizedUserID = (userId || '').toString().trim();
    if (normalizedPackageID === '') {
      showInfo(t('user.detail.assign.package_required'));
      return;
    }
    if (normalizedUserID === '') {
      return;
    }
    const startAt = parseDatetimeLocalValue(assignPackageForm.start_at);
    if (!Number.isFinite(startAt)) {
      showInfo(t('package_manage.messages.start_at_invalid'));
      return;
    }
    setActionLoading('assign-package');
    try {
      const res = await API.post(
        `/api/v1/admin/package/${encodeURIComponent(normalizedPackageID)}/assign`,
        {
          user_id: normalizedUserID,
          start_at: startAt > 0 ? startAt : 0,
        }
      );
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('package_manage.messages.assign_failed'));
        return;
      }
      showSuccess(t('package_manage.messages.assign_success'));
      setAssignPackageOpen(false);
      setAssignPackageForm({
        package_id: '',
        start_at: '',
      });
      await loadActivePackage();
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setActionLoading('');
    }
  }, [
    assignPackageForm.package_id,
    assignPackageForm.start_at,
    closeAssignPackageModal,
    loadActivePackage,
    t,
    userId,
  ]);

  const openAssignTopupModal = useCallback(() => {
    setAssignTopupForm({
      plan_id: '',
    });
    setAssignTopupOpen(true);
    loadTopupPlanOptions().then();
  }, [loadTopupPlanOptions]);

  const closeAssignTopupModal = useCallback(() => {
    if (actionLoading === 'assign-topup') {
      return;
    }
    setAssignTopupOpen(false);
    setAssignTopupForm({
      plan_id: '',
    });
  }, [actionLoading]);

  const submitAssignTopup = useCallback(async () => {
    const normalizedUserId = (userId || '').toString().trim();
    if (normalizedUserId === '') {
      return;
    }
    const normalizedPlanID = (assignTopupForm.plan_id || '').toString().trim();
    if (normalizedPlanID === '') {
      showInfo(t('user.detail.assign.topup_plan_required'));
      return;
    }
    setActionLoading('assign-topup');
    try {
      const res = await API.post(
        `/api/v1/admin/user/${encodeURIComponent(normalizedUserId)}/topup/grant`,
        {
          plan_id: normalizedPlanID,
        },
      );
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('user.messages.operation_failed'));
        return;
      }
      showSuccess(t('user.detail.messages.gift_topup_success'));
      setAssignTopupOpen(false);
      setAssignTopupForm({
        plan_id: '',
      });
      await Promise.all([loadUser(), loadBalanceLots(), loadActivePackage()]);
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setActionLoading('');
    }
  }, [assignTopupForm.plan_id, loadActivePackage, loadBalanceLots, loadUser, t, userId]);

  const formatAmountBySelectedUnit = useCallback(
    (yycAmount, { unlimited = false } = {}) => {
      if (unlimited) {
        return t('common.unlimited');
      }
      const convertedAmount = yycToBillingInputValue(
        yycAmount,
        balanceUnit,
        billingCurrencyIndex,
      );
      return formatAmountWithUnit(convertedAmount, balanceUnit);
    },
    [balanceUnit, billingCurrencyIndex, t],
  );

  const renderBalanceAmountField = useCallback(
    ({ label, name, value }) => (
      <Form.Field className='router-section-input'>
        <label>{label}</label>
        <div className='router-section-input-with-unit'>
          <Form.Input
            className='router-section-input router-section-input-with-unit-field'
            type='number'
            min='0'
            step={balanceInputStep}
            name={name}
            value={value}
            readOnly
          />
          <UnitDropdown
            variant='inputUnit'
            options={billingUnitOptions}
            value={balanceUnit}
            onChange={(_, { value }) => handleBalanceUnitChange(value)}
            disabled={loading || actionLoading !== '' || billingUnitOptions.length === 0}
          />
        </div>
      </Form.Field>
    ),
    [
      actionLoading,
      balanceInputStep,
      balanceUnit,
      handleBalanceUnitChange,
      loading,
      billingUnitOptions,
    ],
  );

  const renderReadonlyMetaField = useCallback(
    ({ label, value, action = null }) => (
      <Form.Field className='router-section-input'>
        <label>{label}</label>
        <div className='router-inline-meta-card'>
          <div className='router-inline-meta-value'>{value}</div>
          {action ? <div className='router-inline-meta-action'>{action}</div> : null}
        </div>
      </Form.Field>
    ),
    [],
  );

  const renderReadonlyAmountField = useCallback(
    ({ label, value }) => (
      <Form.Field className='router-section-input'>
        <label>{label}</label>
        <div className='router-inline-amount-card'>
          <div className='router-inline-meta-value'>{value}</div>
        </div>
      </Form.Field>
    ),
    [],
  );

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <div className='router-entity-detail-page'>
            <div className='router-entity-detail-breadcrumb'>
              <Breadcrumb size='small'>
                <Breadcrumb.Section link onClick={backToList}>
                  {t('header.user')}
                </Breadcrumb.Section>
                <Breadcrumb.Divider icon='right chevron' />
                <Breadcrumb.Section active>
                  {readOnlyValue(inputs.username || userId)}
                </Breadcrumb.Section>
              </Breadcrumb>
            </div>
            <Form
              loading={loading || actionLoading === 'save-basic'}
              autoComplete='new-password'
            >
              <section className='router-entity-detail-section'>
                <div className='router-entity-detail-section-header'>
                  <Header as='h3' className='router-entity-detail-section-title'>
                    {t('common.basic_info')}
                  </Header>
                  <div className='router-toolbar-start'>
                    {renderStatusLabel(inputs.status, t)}
                    {editSection === 'basic' ? (
                      <>
                        <Button
                          type='button'
                          className='router-page-button'
                          onClick={cancelBasicEditing}
                          disabled={actionLoading !== ''}
                        >
                          {t('user.edit.buttons.cancel')}
                        </Button>
                        <Button
                          type='button'
                          positive
                          className='router-page-button'
                          onClick={submitBasic}
                          loading={actionLoading === 'save-basic'}
                          disabled={actionLoading !== ''}
                        >
                          {t('user.edit.buttons.submit')}
                        </Button>
                      </>
                    ) : (
                      <Button
                        type='button'
                        className='router-page-button'
                        onClick={startBasicEditing}
                        disabled={loading || actionLoading !== '' || editSection !== ''}
                      >
                        {t('user.detail.buttons.edit')}
                      </Button>
                    )}
                  </div>
                </div>

                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.user_id')}
                  value={readOnlyValue(userId)}
                  readOnly
                />

                <Form.Group widths='equal'>
                  {editSection === 'basic' ? (
                    <Form.Input
                      className='router-section-input'
                      label={t('user.edit.username')}
                      name='username'
                      value={basicEditInputs.username}
                      placeholder={t('user.edit.username_placeholder')}
                      onChange={handleBasicEditInputChange}
                      autoComplete='off'
                    />
                  ) : (
                    <Form.Input
                      className='router-section-input'
                      label={t('user.edit.username')}
                      value={readOnlyValue(inputs.username)}
                      readOnly
                    />
                  )}
                  <Form.Field className='router-section-input'>
                    <label>{t('user.table.role_text')}</label>
                    <div>{roleControl}</div>
                  </Form.Field>
                </Form.Group>

                <Form.Group widths='equal'>
                  {editSection === 'basic' ? (
                    <Form.Input
                      className='router-section-input'
                      label={t('user.edit.email')}
                      name='email'
                      value={basicEditInputs.email}
                      placeholder={t('user.edit.email_placeholder')}
                      onChange={handleBasicEditInputChange}
                      autoComplete='off'
                    />
                  ) : (
                    <Form.Input
                      className='router-section-input'
                      label={t('user.edit.email')}
                      name='email'
                      value={readOnlyValue(inputs.email)}
                      autoComplete='new-password'
                      readOnly
                    />
                  )}
                  {renderReadonlyMetaField({
                    label: t('user.table.wallet'),
                    value: readOnlyValue(inputs.wallet_address),
                    action:
                      inputs.wallet_address && inputs.wallet_address.toString().trim() !== '' ? (
                        <Button
                          type='button'
                          basic
                          compact
                          size='mini'
                          className='router-inline-meta-copy'
                          onClick={copyWalletAddress}
                        >
                          <Icon name='copy outline' />
                        </Button>
                      ) : null,
                  })}
                </Form.Group>

                <Form.Group widths='equal'>
                  {renderReadonlyMetaField({
                    label: t('user.table.created_at'),
                    value: formatDateTime(inputs.created_at),
                  })}
                  {renderReadonlyMetaField({
                    label: t('user.table.updated_at'),
                    value: formatDateTime(inputs.updated_at),
                  })}
                </Form.Group>
              </section>

              <section className='router-entity-detail-section'>
                <div className='router-entity-detail-section-header'>
                  <Header as='h3' className='router-entity-detail-section-title'>
                    {t('user.detail.package_mode_title')}
                  </Header>
                  <div className='router-toolbar-start'>
                    <Button
                      type='button'
                      className='router-inline-button'
                      loading={activePackageLoading}
                      disabled={activePackageLoading || loading || actionLoading !== '' || editSection !== ''}
                      onClick={() => loadActivePackage()}
                    >
                      {t('user.buttons.refresh')}
                    </Button>
                    <Button
                      type='button'
                      className='router-page-button'
                      disabled={loading || actionLoading !== '' || editSection !== ''}
                      onClick={openAssignPackageModal}
                    >
                      {t('user.detail.buttons.gift_package')}
                    </Button>
                    <Button
                      type='button'
                      className='router-page-button'
                      disabled={loading || actionLoading !== '' || editSection !== ''}
                      onClick={openPackageManagement}
                    >
                      {t('package_manage.title')}
                    </Button>
                  </div>
                </div>
              <Form.Group widths='equal'>
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.package_name')}
                  value={
                    hasActivePackage
                      ? readOnlyValue(activePackageSubscription?.package_name)
                      : t('user.detail.package_none')
                  }
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.package_group')}
                  value={
                    hasActivePackage
                      ? readOnlyValue(
                          activePackageSubscription?.group_name ||
                            activePackageSubscription?.group_id,
                        )
                      : '-'
                  }
                  readOnly
                />
                <Form.Field className='router-section-input'>
                  <label>{t('user.detail.package_status')}</label>
                  <div className='router-inline-status-card'>
                    {hasActivePackage
                      ? renderPackageStatusLabel(activePackageSubscription?.status, t)
                      : '-'}
                  </div>
                </Form.Field>
              </Form.Group>
              <Form.Group widths='equal'>
                {renderReadonlyAmountField({
                  label: t('user.detail.package_daily_limit'),
                  value:
                    hasActivePackage
                      ? Number(activePackageSubscription?.daily_amount || 0) > 0
                        ? formatAmountBySelectedUnit(
                            activePackageSubscription?.daily_amount || 0,
                          )
                        : formatAmountBySelectedUnit(0, { unlimited: true })
                      : '-'
                })}
                {renderReadonlyAmountField({
                  label: t('user.detail.package_emergency_limit'),
                  value:
                    hasActivePackage
                      ? formatAmountBySelectedUnit(
                          activePackageSubscription?.emergency_amount || 0,
                        )
                      : '-'
                })}
              </Form.Group>
              <Form.Group widths='equal'>
                {renderReadonlyAmountField({
                  label: t('user.detail.package_daily_used'),
                  value:
                    hasActivePackage
                      ? formatAmountBySelectedUnit(packageDailySnapshot.consumed_quota || 0)
                      : '-'
                })}
                {renderReadonlyAmountField({
                  label: t('user.detail.package_daily_remaining'),
                  value:
                    hasActivePackage
                      ? packageDailySnapshot.unlimited
                        ? t('common.unlimited')
                        : formatAmountBySelectedUnit(packageDailySnapshot.remaining_quota || 0)
                      : '-'
                })}
                {renderReadonlyAmountField({
                  label: t('user.detail.package_emergency_used'),
                  value:
                    hasActivePackage
                      ? packageEmergencySnapshot.enabled
                        ? formatAmountBySelectedUnit(packageEmergencySnapshot.consumed_quota || 0)
                        : '-'
                      : '-'
                })}
                {renderReadonlyAmountField({
                  label: t('user.detail.package_emergency_remaining'),
                  value:
                    hasActivePackage
                      ? packageEmergencySnapshot.enabled
                        ? formatAmountBySelectedUnit(packageEmergencySnapshot.remaining_quota || 0)
                        : '-'
                      : '-'
                })}
              </Form.Group>
              <Form.Group widths='equal'>
                {renderReadonlyMetaField({
                  label: t('user.detail.package_source'),
                  value:
                    hasActivePackage
                      ? readOnlyValue(activePackageSubscription?.source)
                      : '-'
                })}
                {renderReadonlyMetaField({
                  label: t('user.detail.package_timezone'),
                  value:
                    hasActivePackage
                      ? readOnlyValue(activePackageSubscription?.reset_timezone)
                      : '-'
                })}
                {renderReadonlyMetaField({
                  label: t('user.detail.package_started_at'),
                  value:
                    hasActivePackage
                      ? formatDateTime(activePackageSubscription?.started_at)
                      : '-'
                })}
                {renderReadonlyMetaField({
                  label: t('user.detail.package_expires_at'),
                  value:
                    hasActivePackage
                      ? Number(activePackageSubscription?.expires_at || 0) > 0
                        ? formatDateTime(activePackageSubscription?.expires_at)
                        : t('common.unlimited')
                      : '-'
                })}
              </Form.Group>
              </section>

              <section className='router-entity-detail-section'>
                <div className='router-entity-detail-section-header'>
                  <Header as='h3' className='router-entity-detail-section-title'>
                    {t('user.detail.balance_mode_title')}
                  </Header>
                  <div className='router-toolbar-start'>
                    <Button
                      type='button'
                      className='router-inline-button'
                      loading={loading}
                      disabled={loading || actionLoading !== '' || editSection !== ''}
                      onClick={refreshBalanceSection}
                    >
                      {t('user.buttons.refresh')}
                    </Button>
                    <Button
                      type='button'
                      className='router-page-button'
                      disabled={loading || actionLoading !== '' || editSection !== ''}
                      onClick={openAssignTopupModal}
                    >
                      {t('user.detail.buttons.gift_topup')}
                    </Button>
                  </div>
                </div>
                <Form.Group widths='equal'>
                  {renderBalanceAmountField({
                    label: t('user.detail.remaining_amount'),
                    name: 'amount',
                    value: balanceDisplayValue,
                  })}
                  {renderBalanceAmountField({
                    label: t('user.detail.used_amount'),
                    name: 'yyc_used',
                    value: usedDisplayValue,
                  })}
                  <Form.Field className='router-section-input'>
                    <label>{t('user.table.request_count')}</label>
                    <div className='router-inline-stat-card'>
                      <div className='router-inline-meta-value'>
                        {formatCountValue(inputs.request_count)}
                      </div>
                    </div>
                  </Form.Field>
                </Form.Group>

                <div className='router-toolbar router-block-gap-sm' style={{ marginTop: '0.5rem' }}>
                  <div className='router-toolbar-start'>
                    <Header as='h4' className='router-entity-detail-section-title' style={{ margin: 0 }}>
                      {t('user.detail.balance_lots.title')}
                    </Header>
                  </div>
                  <div className='router-toolbar-end'>
                    <Dropdown
                      className='router-mini-dropdown'
                      selection
                      options={balanceLotSourceOptions}
                      value={balanceLotFilters.source_type}
                      disabled={loading || actionLoading !== '' || editSection !== '' || balanceLotsLoading}
                      onChange={(e, { value }) =>
                        setBalanceLotFilters((prev) => ({
                          ...prev,
                          source_type: (value || '').toString(),
                        }))
                      }
                    />
                    <Dropdown
                      className='router-mini-dropdown'
                      selection
                      options={balanceLotStatusOptions}
                      value={balanceLotFilters.status}
                      disabled={loading || actionLoading !== '' || editSection !== '' || balanceLotsLoading}
                      onChange={(e, { value }) =>
                        setBalanceLotFilters((prev) => ({
                          ...prev,
                          status: (value || '').toString(),
                        }))
                      }
                    />
                    <Dropdown
                      className='router-mini-dropdown'
                      selection
                      options={balanceLotPositiveOnlyOptions}
                      value={balanceLotFilters.positive_only ? '1' : '0'}
                      disabled={loading || actionLoading !== '' || editSection !== '' || balanceLotsLoading}
                      onChange={(e, { value }) =>
                        setBalanceLotFilters((prev) => ({
                          ...prev,
                          positive_only: (value || '1').toString() !== '0',
                        }))
                      }
                    />
                    <Button
                      type='button'
                      className='router-inline-button'
                      loading={balanceLotsLoading}
                      disabled={loading || actionLoading !== '' || editSection !== ''}
                      onClick={() => loadBalanceLots()}
                    >
                      {t('user.buttons.refresh')}
                    </Button>
                  </div>
                </div>

                {balanceLots.length === 0 ? (
                  <div className='router-empty'>{t('user.detail.balance_lots.empty')}</div>
                ) : (
                  <div className='router-table-scroll-x'>
                    <Table celled className='router-table router-list-table'>
                      <Table.Header>
                        <Table.Row>
                          <Table.HeaderCell>{t('user.detail.balance_lots.columns.source')}</Table.HeaderCell>
                          <Table.HeaderCell>{t('user.detail.balance_lots.columns.source_id')}</Table.HeaderCell>
                          <Table.HeaderCell>{t('user.detail.balance_lots.columns.remaining')}</Table.HeaderCell>
                          <Table.HeaderCell>{t('user.detail.balance_lots.columns.total')}</Table.HeaderCell>
                          <Table.HeaderCell>{t('user.detail.balance_lots.columns.status')}</Table.HeaderCell>
                          <Table.HeaderCell>{t('user.detail.balance_lots.columns.granted_at')}</Table.HeaderCell>
                          <Table.HeaderCell>{t('user.detail.balance_lots.columns.expires_at')}</Table.HeaderCell>
                        </Table.Row>
                      </Table.Header>
                      <Table.Body>
                        {balanceLots.map((lot) => (
                          <Table.Row key={lot.id || `${lot.source_type}-${lot.source_id}`}>
                            <Table.Cell>{formatBalanceLotSource(lot.source_type, t)}</Table.Cell>
                            <Table.Cell>{readOnlyValue(lot.source_id)}</Table.Cell>
                            <Table.Cell>{formatAmountBySelectedUnit(lot.remaining_yyc || 0)}</Table.Cell>
                            <Table.Cell>{formatAmountBySelectedUnit(lot.total_yyc || 0)}</Table.Cell>
                            <Table.Cell>{renderBalanceLotStatusLabel(lot.status, t)}</Table.Cell>
                            <Table.Cell>{formatDateTime(lot.granted_at)}</Table.Cell>
                            <Table.Cell>
                              {Number(lot.expires_at || 0) > 0
                                ? formatDateTime(lot.expires_at)
                                : t('common.never')}
                            </Table.Cell>
                          </Table.Row>
                        ))}
                      </Table.Body>
                    </Table>
                  </div>
                )}
              </section>
            </Form>
          </div>
        </Card.Content>
      </Card>

      <Modal open={assignPackageOpen} onClose={closeAssignPackageModal} size='small'>
        <Modal.Header>{t('user.detail.buttons.gift_package')}</Modal.Header>
        <Modal.Content>
          <Form>
            <Form.Select
              className='router-section-input'
              search
              selection
              clearable
              loading={packageOptionsLoading}
              label={t('user.detail.assign.package')}
              placeholder={t('user.detail.assign.package_placeholder')}
              options={packageOptions}
              value={assignPackageForm.package_id}
              onChange={(e, { value }) =>
                setAssignPackageForm((prev) => ({
                  ...prev,
                  package_id: (value || '').toString(),
                }))
              }
            />
            <Form.Input
              className='router-section-input'
              type='datetime-local'
              label={t('package_manage.assign.start_at')}
              placeholder={t('package_manage.assign.start_at_placeholder')}
              value={assignPackageForm.start_at}
              onChange={(e, { value }) =>
                setAssignPackageForm((prev) => ({
                  ...prev,
                  start_at: value || '',
                }))
              }
            />
          </Form>
        </Modal.Content>
        <Modal.Actions>
          <Button type='button' onClick={closeAssignPackageModal} disabled={actionLoading === 'assign-package'}>
            {t('common.cancel')}
          </Button>
          <Button
            type='button'
            color='blue'
            loading={actionLoading === 'assign-package'}
            onClick={submitAssignPackage}
          >
            {t('common.confirm')}
          </Button>
        </Modal.Actions>
      </Modal>

      <Modal open={assignTopupOpen} onClose={closeAssignTopupModal} size='small'>
        <Modal.Header>{t('user.detail.buttons.gift_topup')}</Modal.Header>
        <Modal.Content>
          <Form>
            <Form.Select
              className='router-section-input'
              search
              selection
              clearable
              loading={topupPlanOptionsLoading}
              label={t('user.detail.assign.topup_plan')}
              placeholder={t('user.detail.assign.topup_plan_placeholder')}
              options={topupPlanOptions}
              value={assignTopupForm.plan_id}
              onChange={(e, { value }) =>
                setAssignTopupForm((prev) => ({
                  ...prev,
                  plan_id: (value || '').toString(),
                }))
              }
            />
          </Form>
        </Modal.Content>
        <Modal.Actions>
          <Button type='button' onClick={closeAssignTopupModal} disabled={actionLoading === 'assign-topup'}>
            {t('common.cancel')}
          </Button>
          <Button
            type='button'
            color='blue'
            loading={actionLoading === 'assign-topup'}
            onClick={submitAssignTopup}
          >
            {t('common.confirm')}
          </Button>
        </Modal.Actions>
      </Modal>
    </div>
  );
};

export default UserDetail;
