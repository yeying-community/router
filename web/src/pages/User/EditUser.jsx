import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Breadcrumb, Button, Card, Dropdown, Form, Header, Icon, Label, Modal } from 'semantic-ui-react';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { API, copy, isRoot, showError, showSuccess } from '../../helpers';
import {
  buildBillingCurrencyIndex,
  buildBillingUnitOptions,
  convertBillingInputValueUnit,
  billingInputValueToYYC,
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
    default:
      return (
        <Label basic color='grey' className='router-tag'>
          {t('user.detail.package_status_types.unknown')}
        </Label>
      );
  }
};

const createEmptyActivePackage = () => ({
  has_active_subscription: false,
  subscription: null,
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
  const [groupMap, setGroupMap] = useState({});
  const [billingCurrencyIndex, setBillingCurrencyIndex] = useState(
    buildBillingCurrencyIndex([], { activeOnly: true })
  );
  const [balanceUnit, setBalanceUnit] = useState('USD');
  const [activePackage, setActivePackage] = useState(createEmptyActivePackage());
  const [activePackageLoading, setActivePackageLoading] = useState(false);
  const [packageOptions, setPackageOptions] = useState([]);
  const [packageOptionsLoading, setPackageOptionsLoading] = useState(false);
  const [assignPackageOpen, setAssignPackageOpen] = useState(false);
  const [assignPackageForm, setAssignPackageForm] = useState({
    package_id: '',
    start_at: '',
  });
  const [inputs, setInputs] = useState({
    username: '',
    email: '',
    yyc_balance: 0,
    group: '',
    daily_amount: 0,
    emergency_amount: 0,
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
    group: '',
  });
  const [balanceEditInputs, setBalanceEditInputs] = useState({
    amount: 0,
  });
  const returnPath = useMemo(() => {
    const from = location.state?.from;
    if (typeof from !== 'string') {
      return '';
    }
    const normalized = from.trim();
    return normalized.startsWith('/') ? normalized : '';
  }, [location.state]);

  const loadGroups = useCallback(async () => {
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
          showError(message || t('user.messages.operation_failed'));
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
      const nextMap = {};
      rows.forEach((group) => {
        const id = (group?.id || '').toString().trim();
        if (id === '') {
          return;
        }
        nextMap[id] = (group?.name || '').toString().trim() || id;
      });
      setGroupMap(nextMap);
    } catch (error) {
      showError(error?.message || error);
    }
  }, [t]);

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
        daily_amount: Number(data?.yyc_daily_limit ?? data?.daily_quota_limit ?? 0),
        emergency_amount: Number(data?.yyc_package_emergency_limit ?? data?.package_emergency_quota_limit ?? 0),
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
        group: nextInputs.group,
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
      setActivePackage(normalizeActivePackage(data));
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setActivePackageLoading(false);
    }
  }, [t, userId]);

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
      await loadGroups();
      await loadBillingCurrencies();
      await loadUser();
    };
    init().then();
  }, [loadBillingCurrencies, loadGroups, loadUser]);

  const groupDisplayValue = useMemo(() => {
    const raw = (inputs.group || '').toString().trim();
    if (raw === '') {
      return '-';
    }
    return raw
      .split(',')
      .map((item) => item.trim())
      .filter((item) => item !== '')
      .map((item) => groupMap[item] || item)
      .join(', ') || '-';
  }, [groupMap, inputs.group]);

  const groupOptions = useMemo(
    () =>
      Object.entries(groupMap).map(([value, text]) => ({
        key: value,
        value,
        text,
      })),
    [groupMap],
  );
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

  useEffect(() => {
    loadActivePackage().then();
  }, [loadActivePackage]);

  useEffect(() => {
    if (editSection === 'balance') {
      return;
    }
    setBalanceEditInputs({
      amount: yycToBillingInputValue(inputs.yyc_balance, balanceUnit, billingCurrencyIndex),
    });
  }, [balanceUnit, billingCurrencyIndex, editSection, inputs.yyc_balance]);

  const roleControl = useMemo(() => {
    if (!canManageRole) {
      return renderRoleLabel(inputs.role, t);
    }
    return (
      <Dropdown
        className='router-inline-dropdown router-role-dropdown'
        selection
        compact
        options={ROLE_OPTIONS(t)}
        value={Number(inputs.role || 1)}
        disabled={loading || actionLoading !== '' || editSection !== ''}
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

  const handleBalanceEditInputChange = useCallback((e, { name, value }) => {
    setBalanceEditInputs((prev) => ({
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
      if (editSection === 'balance') {
        setBalanceEditInputs((prev) => ({
          ...prev,
          amount: convertBillingInputValueUnit(
            prev.amount,
            balanceUnit,
            normalizedNextUnit,
            billingCurrencyIndex,
          ),
        }));
      }
      setBalanceUnit(normalizedNextUnit);
    },
    [balanceUnit, billingCurrencyIndex, editSection],
  );

  const resetBasicEditInputs = useCallback(() => {
    setBasicEditInputs({
      username: inputs.username || '',
      email: inputs.email || '',
      group: inputs.group || '',
    });
  }, [
    inputs.email,
    inputs.group,
    inputs.username,
  ]);

  const resetBalanceEditInputs = useCallback(() => {
    setBalanceEditInputs({
      amount: yycToBillingInputValue(inputs.yyc_balance ?? 0, balanceUnit, billingCurrencyIndex),
    });
  }, [balanceUnit, billingCurrencyIndex, inputs.yyc_balance]);

  const startBasicEditing = useCallback(() => {
    resetBasicEditInputs();
    setEditSection('basic');
  }, [resetBasicEditInputs]);

  const startBalanceEditing = useCallback(() => {
    resetBalanceEditInputs();
    setEditSection('balance');
  }, [resetBalanceEditInputs]);

  const cancelBasicEditing = useCallback(() => {
    resetBasicEditInputs();
    setEditSection('');
  }, [resetBasicEditInputs]);

  const cancelBalanceEditing = useCallback(() => {
    resetBalanceEditInputs();
    setEditSection('');
  }, [resetBalanceEditInputs]);

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
        daily_quota_limit: Math.trunc(Number(inputs.daily_amount || 0)),
        package_emergency_quota_limit: Math.trunc(Number(inputs.emergency_amount || 0)),
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
    inputs.daily_amount,
    inputs.emergency_amount,
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
    const group = (basicEditInputs.group || '').toString().trim();
    await updateUser({
      username,
      email,
      group,
      yycBalance: Number(inputs.yyc_balance || 0),
      actionKey: 'save-basic',
    });
  }, [basicEditInputs.email, basicEditInputs.group, basicEditInputs.username, inputs.yyc_balance, updateUser]);

  const submitBalance = useCallback(async () => {
    const yycBalance = billingInputValueToYYC(
      balanceEditInputs.amount,
      balanceUnit,
      billingCurrencyIndex,
    );
    await updateUser({
      username: (inputs.username || '').toString().trim(),
      email: (inputs.email || '').toString().trim(),
      group: (inputs.group || '').toString().trim(),
      yycBalance,
      actionKey: 'save-balance',
    });
  }, [
    balanceEditInputs.amount,
    balanceUnit,
    billingCurrencyIndex,
    inputs.email,
    inputs.group,
    inputs.username,
    updateUser,
  ]);

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
    await loadUser();
  }, [loadUser]);

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
    ({ label, name, value, placeholder = '', editable = false }) => (
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
            placeholder={placeholder}
            onChange={editable ? handleBalanceEditInputChange : undefined}
            readOnly={!editable}
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
      handleBalanceEditInputChange,
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
          <div className='router-inline-amount-value'>{value}</div>
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
              loading={loading || actionLoading === 'save-basic' || actionLoading === 'save-balance'}
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
                  {editSection === 'basic' ? (
                    <Form.Dropdown
                      className='router-section-input'
                      label={t('user.edit.group')}
                      name='group'
                      selection
                      clearable
                      search
                      options={groupOptions}
                      value={basicEditInputs.group || ''}
                      placeholder={t('user.edit.group_placeholder')}
                      onChange={handleBasicEditInputChange}
                    />
                  ) : (
                    <Form.Input
                      className='router-section-input'
                      label={t('user.edit.group')}
                      value={groupDisplayValue}
                      readOnly
                    />
                  )}
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
                    {editSection === 'balance' ? (
                      <>
                        <Button
                          type='button'
                          className='router-page-button'
                          onClick={cancelBalanceEditing}
                          disabled={actionLoading !== ''}
                        >
                          {t('user.edit.buttons.cancel')}
                        </Button>
                        <Button
                          type='button'
                          positive
                          className='router-page-button'
                          onClick={submitBalance}
                          loading={actionLoading === 'save-balance'}
                          disabled={actionLoading !== ''}
                        >
                          {t('user.edit.buttons.submit')}
                        </Button>
                      </>
                    ) : (
                      <>
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
                          onClick={startBalanceEditing}
                          disabled={loading || actionLoading !== '' || editSection !== ''}
                        >
                          {t('user.detail.buttons.edit')}
                        </Button>
                      </>
                    )}
                  </div>
                </div>
                <Form.Group widths='equal'>
                  {editSection === 'balance' ? (
                    renderBalanceAmountField({
                      label: t('user.detail.remaining_amount'),
                      name: 'amount',
                      value: balanceEditInputs.amount,
                      placeholder: t('user.edit.quota_placeholder'),
                      editable: true,
                    })
                  ) : (
                    renderBalanceAmountField({
                      label: t('user.detail.remaining_amount'),
                      name: 'amount',
                      value: balanceDisplayValue,
                    })
                  )}
                  {renderBalanceAmountField({
                    label: t('user.detail.used_amount'),
                    name: 'yyc_used',
                    value: usedDisplayValue,
                  })}
                  <Form.Field className='router-section-input'>
                    <label>{t('user.table.request_count')}</label>
                    <div className='router-inline-stat-card'>
                      <div className='router-inline-stat-value'>
                        {formatCountValue(inputs.request_count)}
                      </div>
                      <div className='router-inline-stat-hint'>
                        {t('user.detail.request_count_hint')}
                      </div>
                    </div>
                  </Form.Field>
                </Form.Group>
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
    </div>
  );
};

export default UserDetail;
