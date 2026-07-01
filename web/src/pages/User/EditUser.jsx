import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { API, isRoot, showError, showInfo, showSuccess } from '../../helpers';
import {
  BALANCE_LOT_COLUMN_WIDTHS,
  BALANCE_LOT_DETAIL_TABLE_MIN_WIDTH,
} from '../../constants/tableWidthPresets';
import {
  buildBillingCurrencyIndex,
  buildBillingUnitOptions,
  chargeAmountToBillingInputValue,
  resolveDefaultBillingUnit,
  resolveBillingInputStep,
} from '../../helpers/billing';
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
  AppModal,
  AppPagination,
  AppSelect,
  AppTable,
  AppTabs,
  AppTag,
  AppToolbar,
} from '../../router-ui';
import {
  formatAmountWithUnit,
} from '../../helpers/render';
import {
  formatRequestCount,
  formatUserFacingPackageConcurrency,
  getServicePackagePeriodLabel,
  getServicePackageTypeLabel,
  isRequestQuotaPackage,
} from '../../helpers/package';

const BALANCE_LOT_PAGE_SIZE = 20;

const ROLE_OPTIONS = (t) => [
  { key: 1, value: 1, text: t('user.table.role_types.normal') },
  { key: 10, value: 10, text: t('user.table.role_types.admin') },
];

const renderRoleLabel = (role, t) => {
  switch (Number(role)) {
    case 1:
      return <AppTag className='router-tag'>{t('user.table.role_types.normal')}</AppTag>;
    case 10:
      return (
        <AppTag color='yellow' className='router-tag'>
          {t('user.table.role_types.admin')}
        </AppTag>
      );
    default:
      return (
        <AppTag color='red' className='router-tag'>
          {t('user.table.role_types.unknown')}
        </AppTag>
      );
  }
};

const renderStatusLabel = (status, t) => {
  switch (Number(status)) {
    case 1:
      return (
        <AppTag className='router-tag'>
          {t('user.table.status_types.activated')}
        </AppTag>
      );
    case 2:
      return (
        <AppTag color='red' className='router-tag'>
          {t('user.table.status_types.banned')}
        </AppTag>
      );
    default:
      return (
        <AppTag color='grey' className='router-tag'>
          {t('user.table.status_types.unknown')}
        </AppTag>
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
        <AppTag color='green' className='router-tag'>
          {t('user.detail.package_status_types.active')}
        </AppTag>
      );
    case 2:
      return (
        <AppTag color='grey' className='router-tag'>
          {t('user.detail.package_status_types.expired')}
        </AppTag>
      );
    case 3:
      return (
        <AppTag color='blue' className='router-tag'>
          {t('user.detail.package_status_types.replaced')}
        </AppTag>
      );
    case 4:
      return (
        <AppTag color='red' className='router-tag'>
          {t('user.detail.package_status_types.canceled')}
        </AppTag>
      );
    default:
      return (
        <AppTag color='grey' className='router-tag'>
          {t('user.detail.package_status_types.unknown')}
        </AppTag>
      );
  }
};

const renderBalanceLotStatusLabel = (status, t) => {
  switch ((status || '').toString().trim()) {
    case 'active':
      return (
        <AppTag color='green' className='router-tag'>
          {t('topup.balance_lots.status.active')}
        </AppTag>
      );
    case 'exhausted':
      return (
        <AppTag color='grey' className='router-tag'>
          {t('topup.balance_lots.status.exhausted')}
        </AppTag>
      );
    case 'expired':
      return (
        <AppTag color='orange' className='router-tag'>
          {t('topup.balance_lots.status.expired')}
        </AppTag>
      );
    default:
      return (
        <AppTag color='grey' className='router-tag'>
          {readOnlyValue(status)}
        </AppTag>
      );
  }
};

const formatBalanceLotSource = (sourceType, t) => {
  switch ((sourceType || '').toString().trim()) {
    case 'topup_order':
      return t('topup.balance_lots.source.topup_order');
    case 'redemption':
      return t('topup.balance_lots.source.redemption');
    default:
      return readOnlyValue(sourceType);
  }
};

const createEmptyActivePackage = () => ({
  has_active_packages: false,
  active_packages: [],
});

const normalizePackageView = (raw) => {
  if (!raw || typeof raw !== 'object') {
    return null;
  }
  return {
    id: (raw.id || '').toString().trim(),
    user_id: (raw.user_id || '').toString().trim(),
    package_id: (raw.package_id || '').toString().trim(),
    package_name: (raw.package_name || '').toString().trim(),
    group_id: (raw.group_id || '').toString().trim(),
    group_name: (raw.group_name || '').toString().trim(),
    source: (raw.source || '').toString().trim(),
    status: Number(raw.status || 0),
    package_type: (raw.package_type || '').toString().trim(),
    quota_metric: (raw.quota_metric || '').toString().trim(),
    period_type: (raw.period_type || '').toString().trim(),
    period_limit: Number(raw.period_limit || 0),
    max_concurrency_per_user: Number(raw.max_concurrency_per_user || 0),
    max_concurrency_per_package: Number(raw.max_concurrency_per_package || 0),
    allow_balance_fallback: raw.allow_balance_fallback === true,
    daily_quota_limit: Number(raw.daily_quota_limit || 0),
    package_emergency_quota_limit: Number(raw.package_emergency_quota_limit || 0),
    quota_reset_timezone: (raw.quota_reset_timezone || '').toString().trim(),
    started_at: Number(raw.started_at || 0),
    expires_at: Number(raw.expires_at || 0),
    usage: raw.usage || null,
  };
};

const normalizeActivePackage = (raw) => {
  if (!raw || typeof raw !== 'object') {
    return createEmptyActivePackage();
  }
  const activePackages = Array.isArray(raw.active_packages)
    ? raw.active_packages.map(normalizePackageView).filter(Boolean)
    : [];
  return {
    has_active_packages: activePackages.length > 0,
    active_packages: activePackages,
  };
};

const renderPackageEntitlementValue = (item, t, renderAmount) => {
  if (isRequestQuotaPackage(item)) {
    return `${formatRequestCount(item?.period_limit || 0)} ${t(
      'package_manage.request_unit',
    )} / ${getServicePackagePeriodLabel(item?.period_type, t)}`;
  }
  return renderAmount(item?.daily_quota_limit || 0);
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

const UserDetail = () => {
  const { t } = useTranslation();
  const { id: userId } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [activeDetailTab, setActiveDetailTab] = useState('basic');
  const [editSection, setEditSection] = useState('');
  const [actionLoading, setActionLoading] = useState('');
  const [persistedUsername, setPersistedUsername] = useState('');
  const [billingCurrencyIndex, setBillingCurrencyIndex] = useState(
    buildBillingCurrencyIndex([], { activeOnly: true })
  );
  const [balanceUnit, setBalanceUnit] = useState('USD');
  const [activePackage, setActivePackage] = useState(createEmptyActivePackage());
  const [activePackageLoading, setActivePackageLoading] = useState(false);
  const [balanceLots, setBalanceLots] = useState([]);
  const [balanceLotsLoading, setBalanceLotsLoading] = useState(false);
  const [balanceLotsPage, setBalanceLotsPage] = useState(1);
  const [balanceLotsPageSize, setBalanceLotsPageSize] = useState(BALANCE_LOT_PAGE_SIZE);
  const [balanceLotsTotal, setBalanceLotsTotal] = useState(0);
  const [balanceLotFilters, setBalanceLotFilters] = useState({
    source_type: '',
    status: '',
    positive_only: false,
  });
  const [packageOptions, setPackageOptions] = useState([]);
  const [packageOptionsLoading, setPackageOptionsLoading] = useState(false);
  const [assignPackageOpen, setAssignPackageOpen] = useState(false);
  const [assignPackageForm, setAssignPackageForm] = useState({
    package_id: '',
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
    balance_amount: 0,
    group: '',
    reset_timezone: 'Asia/Shanghai',
    role: 1,
    status: 1,
    wallet_address: '',
    used_amount: 0,
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
      navigate('/admin/user', { replace: true });
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
        balance_amount: Number(data?.balance_amount ?? data?.quota ?? 0),
        group: data?.group || '',
        reset_timezone: data?.quota_reset_timezone || 'Asia/Shanghai',
        role: Number(data?.role || 1),
        status: Number(data?.status || 1),
        wallet_address: walletAddress,
        used_amount: Number(data?.used_amount ?? data?.used_quota ?? 0),
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
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setActivePackageLoading(false);
    }
  }, [t, userId]);

  const loadBalanceLots = useCallback(
    async ({ silent = false, page = balanceLotsPage } = {}) => {
      const normalizedUserId = (userId || '').toString().trim();
      if (normalizedUserId === '') {
        setBalanceLots([]);
        setBalanceLotsTotal(0);
        return;
      }
      const nextPage = Math.max(1, Number(page || 1) || 1);
      if (!silent) {
        setBalanceLotsLoading(true);
      }
      try {
        const res = await API.get(
          `/api/v1/admin/user/${encodeURIComponent(normalizedUserId)}/topup/balance/lots`,
          {
            params: {
              page: nextPage,
              page_size: BALANCE_LOT_PAGE_SIZE,
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
        const items = Array.isArray(data?.items) ? data.items : [];
        const responsePage = Math.max(1, Number(data?.page || nextPage) || nextPage);
        const responsePageSize = Math.max(1, Number(data?.page_size || BALANCE_LOT_PAGE_SIZE) || BALANCE_LOT_PAGE_SIZE);
        const responseTotal = Math.max(0, Number(data?.total ?? items.length ?? 0) || 0);
        setBalanceLots(items);
        setBalanceLotsPage(responsePage);
        setBalanceLotsPageSize(responsePageSize);
        setBalanceLotsTotal(responseTotal);
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
      balanceLotsPage,
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
    () => chargeAmountToBillingInputValue(inputs.balance_amount, balanceUnit, billingCurrencyIndex),
    [balanceUnit, billingCurrencyIndex, inputs.balance_amount],
  );
  const usedDisplayValue = useMemo(
    () => chargeAmountToBillingInputValue(inputs.used_amount, balanceUnit, billingCurrencyIndex),
    [balanceUnit, billingCurrencyIndex, inputs.used_amount],
  );

  const isProtectedUser = inputs.can_manage_users === true;
  const canManageRole = isRoot() && !isProtectedUser;
  const activePackages = activePackage.active_packages || [];
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
  const balanceLotTotalPages = useMemo(
    () => Math.max(1, Math.ceil(balanceLotsTotal / Math.max(1, balanceLotsPageSize))),
    [balanceLotsPageSize, balanceLotsTotal],
  );

  const currentDetailPath = useMemo(
    () => `${location.pathname}${location.search}${location.hash}`,
    [location.hash, location.pathname, location.search],
  );

  const resolveBalanceLotSourcePath = useCallback((lot) => {
    const detailPath = (lot?.source_detail?.detail_path || '').toString().trim();
    if (detailPath !== '') {
      return detailPath;
    }
    const sourceID = (lot?.source_id || '').toString().trim();
    if (sourceID === '') {
      return '';
    }
    switch ((lot?.source_type || '').toString().trim()) {
      case 'topup_order':
        return `/admin/flow/topup/${encodeURIComponent(sourceID)}`;
      case 'redemption':
        return `/admin/flow/redemption/${encodeURIComponent(sourceID)}`;
      default:
        return '';
    }
  }, []);

  const goToBalanceLotSource = useCallback(
    (lot) => {
      const path = resolveBalanceLotSourcePath(lot);
      if (path === '') {
        return;
      }
      navigate(path, {
        state: { from: currentDetailPath },
      });
    },
    [currentDetailPath, navigate, resolveBalanceLotSourcePath],
  );

  const renderBalanceLotSourceLink = useCallback(
    (lot, content, className = '') => {
      if (resolveBalanceLotSourcePath(lot) === '') {
        return content;
      }
      return (
        <button
          type='button'
          className={`router-link-button router-link-inline ${className}`.trim()}
          onClick={(event) => {
            event.stopPropagation();
            goToBalanceLotSource(lot);
          }}
        >
          {content}
        </button>
      );
    },
    [goToBalanceLotSource, resolveBalanceLotSourcePath],
  );

  const renderBalanceLotSourceCell = useCallback(
    (lot) => {
      const sourceLabel = formatBalanceLotSource(lot?.source_type, t);
      const title = (lot?.source_detail?.title || '').toString().trim();
      const content = (
        <span className='router-balance-lot-source-cell'>
          <span>{title || sourceLabel}</span>
          {title && title !== sourceLabel ? (
            <span className='router-balance-lot-source-type'>{sourceLabel}</span>
          ) : null}
        </span>
      );
      return renderBalanceLotSourceLink(lot, content);
    },
    [renderBalanceLotSourceLink, t],
  );

  const renderBalanceLotSourceIDCell = useCallback(
    (lot) => {
      const value = (lot?.source_id || '').toString();
      if (value === '') {
        return readOnlyValue(value);
      }
      const content = (
        <span
          className='router-monospace-value router-monospace-truncate'
          title={value}
        >
          {value}
        </span>
      );
      return renderBalanceLotSourceLink(lot, content, 'router-balance-lot-source-id-link');
    },
    [renderBalanceLotSourceLink],
  );

  useEffect(() => {
    loadActivePackage().then();
  }, [loadActivePackage]);

  useEffect(() => {
    loadBalanceLots({ silent: true }).then();
  }, [loadBalanceLots]);

  useEffect(() => {
    if (balanceLotsPage > balanceLotTotalPages) {
      setBalanceLotsPage(balanceLotTotalPages);
    }
  }, [balanceLotTotalPages, balanceLotsPage]);

  const roleControl = useMemo(() => {
    return (
      <AppSelect
        className='router-section-input'
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

  const updateUser = useCallback(async ({ username, email, group, balanceAmount, actionKey }) => {
    if (username === '') {
      showError(t('user.edit.username_placeholder'));
      return false;
    }
    if (!Number.isFinite(balanceAmount) || balanceAmount < 0) {
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
        quota: Math.trunc(balanceAmount),
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
      balanceAmount: Number(inputs.balance_amount || 0),
      actionKey: 'save-basic',
    });
  }, [basicEditInputs.email, basicEditInputs.username, inputs.group, inputs.balance_amount, updateUser]);

  const backToList = useCallback(() => {
    if (returnPath !== '') {
      navigate(-1);
      return;
    }
    navigate('/admin/user');
  }, [navigate, returnPath]);

  const refreshBalanceSection = useCallback(async () => {
    await Promise.all([loadUser(), loadBalanceLots()]);
  }, [loadBalanceLots, loadUser]);

  const openAssignPackageModal = useCallback(() => {
    setAssignPackageForm({
      package_id: '',
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
    setActionLoading('assign-package');
    try {
      const res = await API.post(
        `/api/v1/admin/package/${encodeURIComponent(normalizedPackageID)}/assign`,
        {
          user_id: normalizedUserID,
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
      });
      await loadActivePackage();
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setActionLoading('');
    }
  }, [
    assignPackageForm.package_id,
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
    (chargeAmount, { unlimited = false } = {}) => {
      if (unlimited) {
        return t('common.unlimited');
      }
      const convertedAmount = chargeAmountToBillingInputValue(
        chargeAmount,
        balanceUnit,
        billingCurrencyIndex,
      );
      return formatAmountWithUnit(convertedAmount, balanceUnit);
    },
    [balanceUnit, billingCurrencyIndex, t],
  );

  const renderBalanceAmountField = useCallback(
    ({ label, name, value }) => (
      <AppField label={label} readOnly>
        <AppCompact className='router-section-input-with-unit' block>
          <AppInputNumber
            className='router-section-input router-section-input-with-unit-field'
            fluid
            min={0}
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
        </AppCompact>
      </AppField>
    ),
    [
      actionLoading,
      balanceInputStep,
      balanceUnit,
      billingCurrencyIndex,
      billingUnitOptions,
      handleBalanceUnitChange,
      loading,
    ],
  );

  const renderReadonlyMetaField = useCallback(
    ({ label, value, action = null }) => (
      <AppField className='router-section-input' label={label} readOnly>
        <div className='router-inline-meta-card'>
          <div className='router-inline-meta-value'>{value}</div>
          {action ? <div className='router-inline-meta-action'>{action}</div> : null}
        </div>
      </AppField>
    ),
    [],
  );

  const renderReadonlyAmount = useCallback(
    (chargeAmount) =>
      formatAmountWithUnit(
        chargeAmountToBillingInputValue(chargeAmount, balanceUnit, billingCurrencyIndex),
        balanceUnit,
      ),
    [balanceUnit, billingCurrencyIndex],
  );

  const renderReadonlyAmountField = useCallback(
    ({ label, chargeAmount, fallback = '-' }) => {
      if (fallback !== null) {
        return (
          <AppField className='router-section-input' label={label} readOnly>
            <AppInput
              className='router-section-input'
              value={fallback}
              readOnly
            />
          </AppField>
        );
      }
      return (
        <AppField className='router-section-input' label={label} readOnly>
          <AppCompact className='router-section-input-with-unit' block>
            <AppInputNumber
              className='router-section-input router-section-input-with-unit-field'
              fluid
              min={0}
              step={balanceInputStep}
              value={chargeAmountToBillingInputValue(chargeAmount, balanceUnit, billingCurrencyIndex)}
              readOnly
            />
            <UnitDropdown
              variant='inputUnit'
              options={billingUnitOptions}
              value={balanceUnit}
              onChange={(_, { value }) => handleBalanceUnitChange(value)}
              disabled={loading || actionLoading !== '' || billingUnitOptions.length === 0}
            />
          </AppCompact>
        </AppField>
      );
    },
    [
      actionLoading,
      balanceInputStep,
      balanceUnit,
      billingCurrencyIndex,
      billingUnitOptions,
      handleBalanceUnitChange,
      loading,
    ],
  );

  const detailTabItems = [
    {
      key: 'basic',
      label: t('common.basic_info'),
      disabled: editSection !== '' && activeDetailTab !== 'basic',
    },
    {
      key: 'package',
      label: t('user.detail.package_mode_title'),
      disabled: editSection !== '' && activeDetailTab !== 'package',
    },
    {
      key: 'balance',
      label: t('user.detail.balance_mode_title'),
      disabled: editSection !== '' && activeDetailTab !== 'balance',
    },
  ];

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'admin', label: t('header.admin_workspace') },
          { key: 'business', label: t('header.business_operation') },
          {
            key: 'user-list',
            label: t('header.user'),
            onClick: backToList,
          },
          {
            key: 'user-current',
            label: readOnlyValue(inputs.username || userId),
            active: true,
          },
        ]}
        title={t('user.detail.title')}
      />
      <div className='router-tab-detail-page router-entity-detail-page'>
        <div className='router-entity-detail-tabs router-block-gap-sm'>
          <AppTabs
            className='router-detail-tab-menu'
            activeKey={activeDetailTab}
            items={detailTabItems}
            onChange={setActiveDetailTab}
          />
        </div>
        <div className='router-page-stack'>
              {activeDetailTab === 'basic' ? (
              <AppDetailSection
                title={t('common.basic_info')}
                headerStart={renderStatusLabel(inputs.status, t)}
                headerEnd={
                  editSection === 'basic' ? (
                    <>
                      <AppButton
                        type='button'
                        className='router-page-button'
                        onClick={cancelBasicEditing}
                        disabled={actionLoading !== ''}
                      >
                        {t('user.edit.buttons.cancel')}
                      </AppButton>
                      <AppButton
                        type='button'
                        color='blue'
                        className='router-page-button'
                        onClick={submitBasic}
                        loading={actionLoading === 'save-basic'}
                        disabled={actionLoading !== ''}
                      >
                        {t('user.edit.buttons.submit')}
                      </AppButton>
                    </>
                  ) : (
                    <AppButton
                      type='button'
                      className='router-page-button'
                      onClick={startBasicEditing}
                      disabled={loading || actionLoading !== '' || editSection !== ''}
                    >
                      {t('user.detail.buttons.edit')}
                    </AppButton>
                  )
                }
              >
                <AppFormRow>
                  <AppField label={t('user.detail.user_id')} readOnly>
                    <AppInput
                      className='router-section-input router-machine-input'
                      value={readOnlyValue(userId)}
                      readOnly
                    />
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  {editSection === 'basic' ? (
                    <AppField label={t('user.edit.username')} required>
                      <AppInput
                        className='router-section-input'
                        name='username'
                        value={basicEditInputs.username}
                        placeholder={t('user.edit.username_placeholder')}
                        onChange={handleBasicEditInputChange}
                        autoComplete='off'
                      />
                    </AppField>
                  ) : (
                    <AppField label={t('user.edit.username')} readOnly>
                      <AppInput
                        className='router-section-input'
                        value={readOnlyValue(inputs.username)}
                        readOnly
                      />
                    </AppField>
                  )}
                  <AppField label={t('user.table.role_text')}>
                    {roleControl}
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  {editSection === 'basic' ? (
                    <AppField label={t('user.edit.email')}>
                      <AppInput
                        className='router-section-input'
                        name='email'
                        value={basicEditInputs.email}
                        placeholder={t('user.edit.email_placeholder')}
                        onChange={handleBasicEditInputChange}
                        autoComplete='off'
                      />
                    </AppField>
                  ) : (
                    <AppField label={t('user.edit.email')} readOnly>
                      <AppInput
                        className='router-section-input'
                        name='email'
                        value={readOnlyValue(inputs.email)}
                        autoComplete='new-password'
                        readOnly
                      />
                    </AppField>
                  )}
                  {renderReadonlyMetaField({
                    label: t('user.table.wallet'),
                    value: readOnlyValue(inputs.wallet_address),
                  })}
                </AppFormRow>

                <AppFormRow>
                  {renderReadonlyMetaField({
                    label: t('user.table.created_at'),
                    value: formatDateTime(inputs.created_at),
                  })}
                  {renderReadonlyMetaField({
                    label: t('user.table.updated_at'),
                    value: formatDateTime(inputs.updated_at),
                  })}
                </AppFormRow>
              </AppDetailSection>
              ) : null}

              {activeDetailTab === 'package' ? (
              <AppDetailSection
                title={t('user.detail.package_mode_title')}
                headerEnd={
                  <>
                    <AppButton
                      type='button'
                      className='router-inline-button'
                      loading={activePackageLoading}
                      disabled={activePackageLoading || loading || actionLoading !== '' || editSection !== ''}
                      onClick={() => loadActivePackage()}
                    >
                      {t('user.buttons.refresh')}
                    </AppButton>
                    <AppButton
                      type='button'
                      className='router-page-button'
                      disabled={loading || actionLoading !== '' || editSection !== ''}
                      onClick={openAssignPackageModal}
                    >
                      {t('user.detail.buttons.gift_package')}
                    </AppButton>
                  </>
                }
              >
                {activePackageLoading ? (
                  <div className='router-text-muted'>{t('common.loading')}</div>
                ) : activePackages.length === 0 ? (
                  <div className='router-text-muted'>{t('user.detail.package_none')}</div>
                ) : (
                  <div className='router-package-purchase-list'>
                    {activePackages.map((item) => {
                      const requestQuotaPackage = isRequestQuotaPackage(item);
                      const usage = item?.usage || null;
                      const groupLabel =
                        readOnlyValue(item?.group_name || item?.group_id);
                      return (
                        <div
                          key={item.id || item.package_id}
                          className='router-package-purchase-card'
                        >
                          <div className='router-package-purchase-card-header'>
                            <div>
                              <div className='router-package-purchase-card-title'>
                                {readOnlyValue(item?.package_name || item?.package_id)}
                              </div>
                              <div className='router-text-muted router-package-purchase-description'>
                                {groupLabel}
                              </div>
                            </div>
                            {renderPackageStatusLabel(item?.status, t)}
                          </div>
                          <div className='router-current-package-info-grid'>
                            <div className='router-current-package-info-card'>
                              <div className='router-current-package-info-label'>
                                {t('user.detail.package_group')}
                              </div>
                              <div className='router-current-package-info-value'>
                                {groupLabel}
                              </div>
                            </div>
                            <div className='router-current-package-info-card'>
                              <div className='router-current-package-info-label'>
                                {t('package_manage.table.package_type')}
                              </div>
                              <div className='router-current-package-info-value'>
                                {getServicePackageTypeLabel(item, t)}
                              </div>
                            </div>
                            <div className='router-current-package-info-card'>
                              <div className='router-current-package-info-label'>
                                {requestQuotaPackage
                                  ? t('package_manage.table.period_entitlement')
                                  : t('user.detail.package_daily_limit')}
                              </div>
                              <div className='router-current-package-info-value'>
                                {renderPackageEntitlementValue(item, t, renderReadonlyAmount)}
                              </div>
                            </div>
                            {!requestQuotaPackage ? (
                              <div className='router-current-package-info-card'>
                                <div className='router-current-package-info-label'>
                                  {t('user.detail.package_emergency_limit')}
                                </div>
                                <div className='router-current-package-info-value'>
                                  {renderReadonlyAmount(item?.package_emergency_quota_limit || 0)}
                                </div>
                              </div>
                            ) : null}
                            <div className='router-current-package-info-card'>
                              <div className='router-current-package-info-label'>
                                {t('package_manage.table.concurrency_limit')}
                              </div>
                              <div className='router-current-package-info-value'>
                                {formatUserFacingPackageConcurrency(
                                  item,
                                  t,
                                  t('common.unlimited'),
                                )}
                              </div>
                            </div>
                            <div className='router-current-package-info-card'>
                              <div className='router-current-package-info-label'>
                                {t('user.detail.package_source')}
                              </div>
                              <div className='router-current-package-info-value'>
                                {readOnlyValue(item?.source)}
                              </div>
                            </div>
                            <div className='router-current-package-info-card'>
                              <div className='router-current-package-info-label'>
                                {t('user.detail.package_timezone')}
                              </div>
                              <div className='router-current-package-info-value'>
                                {readOnlyValue(item?.quota_reset_timezone)}
                              </div>
                            </div>
                            <div className='router-current-package-info-card'>
                              <div className='router-current-package-info-label'>
                                {t('user.detail.package_started_at')}
                              </div>
                              <div className='router-current-package-info-value'>
                                {formatDateTime(item?.started_at)}
                              </div>
                            </div>
                            <div className='router-current-package-info-card'>
                              <div className='router-current-package-info-label'>
                                {t('user.detail.package_expires_at')}
                              </div>
                              <div className='router-current-package-info-value'>
                                {Number(item?.expires_at || 0) > 0
                                  ? formatDateTime(item.expires_at)
                                  : t('common.unlimited')}
                              </div>
                            </div>
                            {requestQuotaPackage && usage ? (
                              <>
                                <div className='router-current-package-info-card'>
                                  <div className='router-current-package-info-label'>
                                    {t('topup.package_status.period')}
                                  </div>
                                  <div className='router-current-package-info-value'>
                                    {usage.period_key || '-'}
                                  </div>
                                </div>
                                <div className='router-current-package-info-card'>
                                  <div className='router-current-package-info-label'>
                                    {t('user.detail.used_amount')}
                                  </div>
                                  <div className='router-current-package-info-value'>
                                    {formatRequestCount(usage.consumed_amount || 0)}
                                  </div>
                                </div>
                                <div className='router-current-package-info-card'>
                                  <div className='router-current-package-info-label'>
                                    {t('user.detail.remaining_amount')}
                                  </div>
                                  <div className='router-current-package-info-value'>
                                    {usage.unlimited
                                      ? t('common.unlimited')
                                      : formatRequestCount(usage.remaining_amount || 0)}
                                  </div>
                                </div>
                              </>
                            ) : null}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </AppDetailSection>
              ) : null}

              {activeDetailTab === 'balance' ? (
              <AppDetailSection
                title={t('user.detail.balance_mode_title')}
                headerEnd={
                  <>
                    <AppButton
                      type='button'
                      className='router-inline-button'
                      loading={loading}
                      disabled={loading || actionLoading !== '' || editSection !== ''}
                      onClick={refreshBalanceSection}
                    >
                      {t('user.buttons.refresh')}
                    </AppButton>
                    <AppButton
                      type='button'
                      className='router-page-button'
                      disabled={loading || actionLoading !== '' || editSection !== ''}
                      onClick={openAssignTopupModal}
                    >
                      {t('user.detail.buttons.gift_topup')}
                    </AppButton>
                  </>
                }
              >
                <AppFormRow>
                  {renderBalanceAmountField({
                    label: t('user.detail.remaining_amount'),
                    name: 'amount',
                    value: balanceDisplayValue,
                  })}
                  {renderBalanceAmountField({
                    label: t('user.detail.used_amount'),
                    name: 'used_amount',
                    value: usedDisplayValue,
                  })}
                  <AppField
                    className='router-section-input'
                    label={t('user.table.request_count')}
                    readOnly
                  >
                    <div className='router-inline-stat-card'>
                      <div className='router-inline-meta-value'>
                        {formatCountValue(inputs.request_count)}
                      </div>
                    </div>
                  </AppField>
                </AppFormRow>

                <AppFilterHeader
                  className='router-block-gap-sm'
                  title={t('user.detail.balance_lots.title')}
                  titleClassName='router-entity-detail-section-title'
                  end={
                    <>
                    <AppSelect
                      className='router-mini-dropdown'
                      options={balanceLotSourceOptions}
                      value={balanceLotFilters.source_type}
                      disabled={loading || actionLoading !== '' || editSection !== '' || balanceLotsLoading}
                      onChange={(e, { value }) =>
                        {
                          setBalanceLotsPage(1);
                          setBalanceLotFilters((prev) => ({
                            ...prev,
                            source_type: (value || '').toString(),
                          }));
                        }
                      }
                    />
                    <AppSelect
                      className='router-mini-dropdown'
                      options={balanceLotStatusOptions}
                      value={balanceLotFilters.status}
                      disabled={loading || actionLoading !== '' || editSection !== '' || balanceLotsLoading}
                      onChange={(e, { value }) =>
                        {
                          setBalanceLotsPage(1);
                          setBalanceLotFilters((prev) => {
                            const nextStatus = (value || '').toString();
                            return {
                              ...prev,
                              status: nextStatus,
                              positive_only: nextStatus === 'active',
                            };
                          });
                        }
                      }
                    />
                    <AppSelect
                      className='router-mini-dropdown'
                      options={balanceLotPositiveOnlyOptions}
                      value={balanceLotFilters.positive_only ? '1' : '0'}
                      disabled={loading || actionLoading !== '' || editSection !== '' || balanceLotsLoading}
                      onChange={(e, { value }) =>
                        {
                          setBalanceLotsPage(1);
                          setBalanceLotFilters((prev) => ({
                            ...prev,
                            positive_only: (value || '1').toString() !== '0',
                          }));
                        }
                      }
                    />
                    <AppButton
                      type='button'
                      className='router-inline-button'
                      loading={balanceLotsLoading}
                      disabled={loading || actionLoading !== '' || editSection !== ''}
                      onClick={() => loadBalanceLots()}
                    >
                      {t('user.buttons.refresh')}
                    </AppButton>
                    </>
                  }
                />

                <div className='router-balance-lot-summary'>
                  {t('user.detail.balance_lots.summary', {
                    page_count: balanceLots.length,
                    total_count: balanceLotsTotal,
                  })}
                </div>

                {balanceLots.length === 0 ? (
                  <div className='router-empty'>{t('user.detail.balance_lots.empty')}</div>
                ) : (
                  <>
                    <div className='router-table-scroll-x'>
                      <AppTable
                        className='router-table router-list-table router-table-fit-page'
                        pagination={false}
                        scroll={{ x: BALANCE_LOT_DETAIL_TABLE_MIN_WIDTH }}
                        rowKey={(lot) => lot.id || `${lot.source_type}-${lot.source_id}`}
                        dataSource={balanceLots}
                        columns={[
                          {
                            title: t('user.detail.balance_lots.columns.source'),
                            key: 'source',
                            width: BALANCE_LOT_COLUMN_WIDTHS.source,
                            render: (_, lot) => renderBalanceLotSourceCell(lot),
                          },
                          {
                            title: t('user.detail.balance_lots.columns.source_id'),
                            dataIndex: 'source_id',
                            key: 'source_id',
                            width: BALANCE_LOT_COLUMN_WIDTHS.sourceId,
                            render: (_, lot) => renderBalanceLotSourceIDCell(lot),
                          },
                          {
                            title: t('user.detail.balance_lots.columns.remaining'),
                            key: 'remaining_amount',
                            width: BALANCE_LOT_COLUMN_WIDTHS.remaining,
                            render: (_, lot) =>
                              formatAmountBySelectedUnit(lot.remaining_amount || 0),
                          },
                          {
                            title: t('user.detail.balance_lots.columns.total'),
                            key: 'total_amount',
                            width: BALANCE_LOT_COLUMN_WIDTHS.total,
                            render: (_, lot) =>
                              formatAmountBySelectedUnit(lot.total_amount || 0),
                          },
                          {
                            title: t('user.detail.balance_lots.columns.status'),
                            key: 'status',
                            className: 'router-table-col-status-compact',
                            width: BALANCE_LOT_COLUMN_WIDTHS.status,
                            render: (_, lot) => renderBalanceLotStatusLabel(lot.status, t),
                          },
                          {
                            title: t('user.detail.balance_lots.columns.granted_at'),
                            dataIndex: 'granted_at',
                            key: 'granted_at',
                            className: 'router-table-col-datetime',
                            width: BALANCE_LOT_COLUMN_WIDTHS.grantedAt,
                            render: (value) => formatDateTime(value),
                          },
                          {
                            title: t('user.detail.balance_lots.columns.expires_at'),
                            dataIndex: 'expires_at',
                            key: 'expires_at',
                            className: 'router-table-col-datetime',
                            width: BALANCE_LOT_COLUMN_WIDTHS.expiresAt,
                            render: (value) =>
                              Number(value || 0) > 0
                                ? formatDateTime(value)
                                : t('common.never'),
                          },
                        ]}
                      />
                    </div>
                    {balanceLotTotalPages > 1 ? (
                      <div className='router-pagination-wrap'>
                        <AppPagination
                          activePage={balanceLotsPage}
                          totalPages={balanceLotTotalPages}
                          onPageChange={(event, { activePage }) => setBalanceLotsPage(activePage)}
                        />
                      </div>
                    ) : null}
                  </>
                )}
              </AppDetailSection>
              ) : null}
        </div>
      </div>

      <AppModal
        open={assignPackageOpen}
        onClose={closeAssignPackageModal}
        size='small'
        title={t('user.detail.buttons.gift_package')}
        footer={
          <AppFormActions>
            <AppButton
              type='button'
              onClick={closeAssignPackageModal}
              disabled={actionLoading === 'assign-package'}
            >
              {t('common.cancel')}
            </AppButton>
            <AppButton
              type='button'
              color='blue'
              loading={actionLoading === 'assign-package'}
              onClick={submitAssignPackage}
            >
              {t('common.confirm')}
            </AppButton>
          </AppFormActions>
        }
      >
          <div className='router-page-stack'>
            <AppFormRow className='router-user-detail-modal-form-row'>
              <AppField label={t('user.detail.assign.package')}>
                <AppSelect
                  className='router-section-input'
                  fluid
                  search
                  clearable
                  loading={packageOptionsLoading}
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
              </AppField>
            </AppFormRow>
          </div>
      </AppModal>

      <AppModal
        open={assignTopupOpen}
        onClose={closeAssignTopupModal}
        size='small'
        title={t('user.detail.buttons.gift_topup')}
        footer={
          <AppFormActions>
            <AppButton
              type='button'
              onClick={closeAssignTopupModal}
              disabled={actionLoading === 'assign-topup'}
            >
              {t('common.cancel')}
            </AppButton>
            <AppButton
              type='button'
              color='blue'
              loading={actionLoading === 'assign-topup'}
              onClick={submitAssignTopup}
            >
              {t('common.confirm')}
            </AppButton>
          </AppFormActions>
        }
      >
          <div className='router-page-stack'>
            <AppFormRow className='router-user-detail-modal-form-row'>
              <AppField label={t('user.detail.assign.topup_plan')}>
                <AppSelect
                  className='router-section-input'
                  fluid
                  search
                  clearable
                  loading={topupPlanOptionsLoading}
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
              </AppField>
            </AppFormRow>
          </div>
      </AppModal>
    </div>
  );
};

export default UserDetail;
