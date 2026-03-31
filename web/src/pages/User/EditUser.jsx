import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Dropdown, Form, Label } from 'semantic-ui-react';
import { useNavigate, useParams } from 'react-router-dom';
import { API, isRoot, showError, showSuccess } from '../../helpers';

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

const createEmptyDailyQuota = () => ({
  group_id: '',
  group_name: '',
  user_id: '',
  biz_date: '',
  limit: 0,
  consumed_quota: 0,
  reserved_quota: 0,
  remaining_quota: 0,
  unlimited: true,
  timezone: '',
  updated_at: 0,
});

const createEmptyUserQuotaSummary = () => ({
  user_id: '',
  daily: {
    user_id: '',
    biz_date: '',
    limit: 0,
    consumed_quota: 0,
    reserved_quota: 0,
    remaining_quota: 0,
    unlimited: true,
    timezone: '',
    updated_at: 0,
  },
  monthly_emergency: {
    user_id: '',
    biz_month: '',
    limit: 0,
    consumed_quota: 0,
    reserved_quota: 0,
    remaining_quota: 0,
    enabled: false,
    timezone: '',
    updated_at: 0,
  },
});

const parseFirstGroupRef = (raw) => {
  const normalized = (raw || '').toString().trim();
  if (normalized === '') {
    return '';
  }
  const parts = normalized
    .split(',')
    .map((item) => item.trim())
    .filter((item) => item !== '');
  if (parts.length === 0) {
    return '';
  }
  return parts[0];
};

const UserDetail = () => {
  const { t } = useTranslation();
  const { id: userId } = useParams();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [isEditing, setIsEditing] = useState(false);
  const [actionLoading, setActionLoading] = useState('');
  const [persistedUsername, setPersistedUsername] = useState('');
  const [groupMap, setGroupMap] = useState({});
  const [dailyQuota, setDailyQuota] = useState(createEmptyDailyQuota());
  const [dailyQuotaLoading, setDailyQuotaLoading] = useState(false);
  const [userQuotaSummary, setUserQuotaSummary] = useState(createEmptyUserQuotaSummary());
  const [userQuotaSummaryLoading, setUserQuotaSummaryLoading] = useState(false);
  const [inputs, setInputs] = useState({
    username: '',
    email: '',
    quota: 0,
    group: '',
    daily_quota_limit: 0,
    monthly_emergency_quota_limit: 0,
    quota_reset_timezone: 'Asia/Shanghai',
    role: 1,
    status: 1,
    wallet_address: '',
    used_quota: 0,
    request_count: 0,
    can_manage_users: false,
  });
  const [editInputs, setEditInputs] = useState({
    username: '',
    email: '',
    quota: 0,
    group: '',
    daily_quota_limit: 0,
    monthly_emergency_quota_limit: 0,
    quota_reset_timezone: 'Asia/Shanghai',
  });

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
        quota: data?.quota ?? 0,
        group: data?.group || '',
        daily_quota_limit: Number(data?.daily_quota_limit || 0),
        monthly_emergency_quota_limit: Number(data?.monthly_emergency_quota_limit || 0),
        quota_reset_timezone: data?.quota_reset_timezone || 'Asia/Shanghai',
        role: Number(data?.role || 1),
        status: Number(data?.status || 1),
        wallet_address: walletAddress,
        used_quota: data?.used_quota ?? 0,
        request_count: data?.request_count ?? 0,
        can_manage_users: data?.can_manage_users === true,
      };
      setInputs(nextInputs);
      setEditInputs({
        username: nextInputs.username,
        email: nextInputs.email,
        quota: nextInputs.quota,
        group: nextInputs.group,
        daily_quota_limit: nextInputs.daily_quota_limit,
        monthly_emergency_quota_limit: nextInputs.monthly_emergency_quota_limit,
        quota_reset_timezone: nextInputs.quota_reset_timezone,
      });
      setPersistedUsername((data?.username || '').toString().trim());
      setIsEditing(false);
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setLoading(false);
    }
  }, [navigate, userId]);

  const loadUserQuotaSummary = useCallback(async () => {
    const normalizedUserId = (userId || '').toString().trim();
    if (normalizedUserId === '') {
      setUserQuotaSummary(createEmptyUserQuotaSummary());
      return;
    }
    setUserQuotaSummaryLoading(true);
    try {
      const res = await API.get(`/api/v1/admin/user/${encodeURIComponent(normalizedUserId)}/quota/summary`);
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('user.messages.user_quota_load_failed'));
        return;
      }
      setUserQuotaSummary({
        ...createEmptyUserQuotaSummary(),
        ...(data || {}),
        daily: {
          ...createEmptyUserQuotaSummary().daily,
          ...(data?.daily || {}),
        },
        monthly_emergency: {
          ...createEmptyUserQuotaSummary().monthly_emergency,
          ...(data?.monthly_emergency || {}),
        },
      });
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setUserQuotaSummaryLoading(false);
    }
  }, [t, userId]);

  useEffect(() => {
    const init = async () => {
      await loadGroups();
      await loadUser();
    };
    init().then();
  }, [loadGroups, loadUser]);

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

  const currentGroupId = useMemo(() => {
    const groupRef = parseFirstGroupRef(inputs.group);
    if (groupRef === '') {
      return '';
    }
    if (groupMap[groupRef]) {
      return groupRef;
    }
    const matched = Object.entries(groupMap).find(([, name]) => name === groupRef);
    if (matched) {
      return matched[0];
    }
    return groupRef;
  }, [groupMap, inputs.group]);

  const isProtectedUser = inputs.can_manage_users === true;
  const canManageRole = isRoot() && !isProtectedUser;

  const loadDailyQuota = useCallback(async () => {
    const normalizedUserId = (userId || '').toString().trim();
    const normalizedGroupId = (currentGroupId || '').toString().trim();
    if (normalizedUserId === '' || normalizedGroupId === '') {
      setDailyQuota(createEmptyDailyQuota());
      return;
    }
    setDailyQuotaLoading(true);
    try {
      const encodedGroupID = encodeURIComponent(normalizedGroupId);
      const res = await API.get(`/api/v1/admin/group/${encodedGroupID}/quota/daily`, {
        params: {
          user_id: normalizedUserId,
        },
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('user.messages.daily_quota_load_failed'));
        return;
      }
      setDailyQuota({
        ...createEmptyDailyQuota(),
        ...(data || {}),
        group_id: normalizedGroupId,
        group_name: groupMap[normalizedGroupId] || normalizedGroupId,
      });
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setDailyQuotaLoading(false);
    }
  }, [currentGroupId, groupMap, t, userId]);

  useEffect(() => {
    loadDailyQuota().then();
  }, [loadDailyQuota]);

  useEffect(() => {
    loadUserQuotaSummary().then();
  }, [loadUserQuotaSummary]);

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
        disabled={loading || actionLoading !== '' || isEditing}
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
    inputs.role,
    isEditing,
    loadUser,
    loading,
    persistedUsername,
    t,
  ]);

  const handleEditInputChange = useCallback((e, { name, value }) => {
    setEditInputs((prev) => ({
      ...prev,
      [name]: value,
    }));
  }, []);

  const resetEditInputs = useCallback(() => {
    setEditInputs({
      username: inputs.username || '',
      email: inputs.email || '',
      quota: inputs.quota ?? 0,
      group: inputs.group || '',
      daily_quota_limit: inputs.daily_quota_limit ?? 0,
      monthly_emergency_quota_limit: inputs.monthly_emergency_quota_limit ?? 0,
      quota_reset_timezone: inputs.quota_reset_timezone || 'Asia/Shanghai',
    });
  }, [
    inputs.daily_quota_limit,
    inputs.email,
    inputs.group,
    inputs.monthly_emergency_quota_limit,
    inputs.quota,
    inputs.quota_reset_timezone,
    inputs.username,
  ]);

  const startEditing = useCallback(() => {
    resetEditInputs();
    setIsEditing(true);
  }, [resetEditInputs]);

  const cancelEditing = useCallback(() => {
    resetEditInputs();
    setIsEditing(false);
  }, [resetEditInputs]);

  const submit = useCallback(async () => {
    const username = (editInputs.username || '').toString().trim();
    const email = (editInputs.email || '').toString().trim();
    const group = (editInputs.group || '').toString().trim();
    const quota = Number(editInputs.quota);
    const dailyQuotaLimit = Number(editInputs.daily_quota_limit);
    const monthlyEmergencyQuotaLimit = Number(editInputs.monthly_emergency_quota_limit);
    const quotaResetTimezone = (editInputs.quota_reset_timezone || '').toString().trim();
    if (username === '') {
      showError(t('user.edit.username_placeholder'));
      return;
    }
    if (
      !Number.isFinite(quota) ||
      quota < 0 ||
      !Number.isFinite(dailyQuotaLimit) ||
      dailyQuotaLimit < 0 ||
      !Number.isFinite(monthlyEmergencyQuotaLimit) ||
      monthlyEmergencyQuotaLimit < 0
    ) {
      showError(t('user.messages.operation_failed'));
      return;
    }
    setActionLoading('save');
    try {
      const res = await API.put('/api/v1/admin/user/', {
        id: userId,
        username,
        email,
        group,
        quota: Math.trunc(quota),
        daily_quota_limit: Math.trunc(dailyQuotaLimit),
        monthly_emergency_quota_limit: Math.trunc(monthlyEmergencyQuotaLimit),
        quota_reset_timezone: quotaResetTimezone,
        role: Number(inputs.role || 1),
        status: Number(inputs.status || 1),
        display_name: username,
        password: '',
      });
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('user.messages.operation_failed'));
        return;
      }
      showSuccess(t('user.messages.update_success'));
      await loadUser();
      await loadUserQuotaSummary();
      setIsEditing(false);
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setActionLoading('');
    }
  }, [
    editInputs.daily_quota_limit,
    editInputs.email,
    editInputs.group,
    editInputs.monthly_emergency_quota_limit,
    editInputs.quota,
    editInputs.quota_reset_timezone,
    editInputs.username,
    inputs.role,
    inputs.status,
    loadUser,
    loadUserQuotaSummary,
    t,
    userId,
  ]);

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <Card.Header className='header router-page-title'>
            {t('user.detail.title')}
          </Card.Header>
          <Form loading={loading || actionLoading === 'save'} autoComplete='new-password'>
            <div className='router-toolbar router-block-gap-sm'>
              <div className='router-toolbar-start'>
                {isEditing ? (
                  <>
                    <Button
                      type='button'
                      className='router-page-button'
                      onClick={cancelEditing}
                      disabled={actionLoading !== ''}
                    >
                      {t('user.edit.buttons.cancel')}
                    </Button>
                    <Button
                      type='button'
                      positive
                      className='router-page-button'
                      onClick={submit}
                      loading={actionLoading === 'save'}
                      disabled={actionLoading !== ''}
                    >
                      {t('user.edit.buttons.submit')}
                    </Button>
                  </>
                ) : (
                  <>
                    <Button
                      type='button'
                      className='router-page-button'
                      onClick={() => navigate('/user')}
                    >
                      {t('user.detail.buttons.back')}
                    </Button>
                    <Button
                      type='button'
                      className='router-page-button'
                      onClick={startEditing}
                      disabled={loading || actionLoading !== ''}
                    >
                      {t('user.detail.buttons.edit')}
                    </Button>
                  </>
                )}
              </div>
              <div className='router-toolbar-end'>
                {renderStatusLabel(inputs.status, t)}
              </div>
            </div>

            <Form.Group widths='equal'>
              {isEditing ? (
                <Form.Input
                  className='router-section-input'
                  label={t('user.edit.username')}
                  name='username'
                  value={editInputs.username}
                  placeholder={t('user.edit.username_placeholder')}
                  onChange={handleEditInputChange}
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
              {isEditing ? (
                <Form.Dropdown
                  className='router-section-input'
                  label={t('user.edit.group')}
                  name='group'
                  selection
                  clearable
                  search
                  options={groupOptions}
                  value={editInputs.group || ''}
                  placeholder={t('user.edit.group_placeholder')}
                  onChange={handleEditInputChange}
                />
              ) : (
                <Form.Input
                  className='router-section-input'
                  label={t('user.edit.group')}
                  value={groupDisplayValue}
                  readOnly
                />
              )}
              {isEditing ? (
                <Form.Input
                  className='router-section-input'
                  type='number'
                  min='0'
                  step='1'
                  label={t('user.edit.daily_quota_limit')}
                  name='daily_quota_limit'
                  value={editInputs.daily_quota_limit}
                  placeholder={t('user.edit.daily_quota_limit_placeholder')}
                  onChange={handleEditInputChange}
                />
              ) : (
                <Form.Input
                  className='router-section-input'
                  label={t('user.edit.daily_quota_limit')}
                  value={Number(inputs.daily_quota_limit || 0)}
                  readOnly
                />
              )}
            </Form.Group>

            <Form.Group widths='equal'>
              {isEditing ? (
                <Form.Input
                  className='router-section-input'
                  type='number'
                  min='0'
                  step='1'
                  label={t('user.edit.quota')}
                  name='quota'
                  value={editInputs.quota}
                  placeholder={t('user.edit.quota_placeholder')}
                  onChange={handleEditInputChange}
                />
              ) : (
                <Form.Input
                  className='router-section-input'
                  label={t('user.edit.quota')}
                  value={inputs.quota}
                  readOnly
                />
              )}
              {isEditing ? (
                <Form.Input
                  className='router-section-input'
                  type='number'
                  min='0'
                  step='1'
                  label={t('user.edit.monthly_emergency_quota_limit')}
                  name='monthly_emergency_quota_limit'
                  value={editInputs.monthly_emergency_quota_limit}
                  placeholder={t('user.edit.monthly_emergency_quota_limit_placeholder')}
                  onChange={handleEditInputChange}
                />
              ) : (
                <Form.Input
                  className='router-section-input'
                  label={t('user.edit.monthly_emergency_quota_limit')}
                  value={Number(inputs.monthly_emergency_quota_limit || 0)}
                  readOnly
                />
              )}
              {isEditing ? (
                <Form.Input
                  className='router-section-input'
                  label={t('user.edit.quota_reset_timezone')}
                  name='quota_reset_timezone'
                  value={editInputs.quota_reset_timezone}
                  placeholder={t('user.edit.quota_reset_timezone_placeholder')}
                  onChange={handleEditInputChange}
                />
              ) : (
                <Form.Input
                  className='router-section-input'
                  label={t('user.edit.quota_reset_timezone')}
                  value={readOnlyValue(inputs.quota_reset_timezone)}
                  readOnly
                />
              )}
            </Form.Group>

            <Form.Group widths='equal'>
              <Form.Input
                className='router-section-input'
                label={t('user.table.wallet')}
                value={readOnlyValue(inputs.wallet_address)}
                readOnly
              />
              <Form.Input
                className='router-section-input'
                label={t('user.table.used_quota')}
                value={inputs.used_quota}
                readOnly
              />
              <Form.Input
                className='router-section-input'
                label={t('user.table.request_count')}
                value={inputs.request_count}
                readOnly
              />
            </Form.Group>

            <div className='router-block-top-sm'>
              <div className='router-toolbar router-block-gap-xs'>
                <div className='router-toolbar-title'>
                  {t('user.detail.user_quota_title')}
                </div>
                <Button
                  type='button'
                  className='router-inline-button'
                  loading={userQuotaSummaryLoading}
                  disabled={userQuotaSummaryLoading || loading || actionLoading !== ''}
                  onClick={() => loadUserQuotaSummary()}
                >
                  {t('user.buttons.refresh')}
                </Button>
              </div>
              <Form.Group widths='equal'>
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.user_quota_daily_limit')}
                  value={
                    userQuotaSummary.daily.unlimited
                      ? t('common.unlimited')
                      : Number(userQuotaSummary.daily.limit || 0)
                  }
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.user_quota_daily_consumed')}
                  value={Number(userQuotaSummary.daily.consumed_quota || 0)}
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.user_quota_daily_reserved')}
                  value={Number(userQuotaSummary.daily.reserved_quota || 0)}
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.user_quota_daily_remaining')}
                  value={
                    userQuotaSummary.daily.unlimited
                      ? t('common.unlimited')
                      : Number(userQuotaSummary.daily.remaining_quota || 0)
                  }
                  readOnly
                />
              </Form.Group>
              <Form.Group widths='equal'>
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.user_quota_biz_date')}
                  value={readOnlyValue(userQuotaSummary.daily.biz_date)}
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.user_quota_monthly_emergency_limit')}
                  value={
                    userQuotaSummary.monthly_emergency.enabled
                      ? Number(userQuotaSummary.monthly_emergency.limit || 0)
                      : '-'
                  }
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.user_quota_monthly_emergency_consumed')}
                  value={Number(userQuotaSummary.monthly_emergency.consumed_quota || 0)}
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.user_quota_monthly_emergency_remaining')}
                  value={
                    userQuotaSummary.monthly_emergency.enabled
                      ? Number(userQuotaSummary.monthly_emergency.remaining_quota || 0)
                      : '-'
                  }
                  readOnly
                />
              </Form.Group>
              <Form.Group widths='equal'>
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.user_quota_monthly_emergency_reserved')}
                  value={Number(userQuotaSummary.monthly_emergency.reserved_quota || 0)}
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.user_quota_biz_month')}
                  value={readOnlyValue(userQuotaSummary.monthly_emergency.biz_month)}
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.user_quota_timezone')}
                  value={readOnlyValue(userQuotaSummary.daily.timezone)}
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.user_quota_updated_at')}
                  value={
                    userQuotaSummary.daily.updated_at
                      ? new Date(Number(userQuotaSummary.daily.updated_at) * 1000).toLocaleString('zh-CN', { hour12: false })
                      : userQuotaSummary.monthly_emergency.updated_at
                        ? new Date(Number(userQuotaSummary.monthly_emergency.updated_at) * 1000).toLocaleString('zh-CN', { hour12: false })
                        : '-'
                  }
                  readOnly
                />
              </Form.Group>
            </div>

            <div className='router-block-top-sm'>
              <div className='router-toolbar router-block-gap-xs'>
                <div className='router-toolbar-title'>
                  {t('user.detail.daily_quota_title')}
                </div>
                <Button
                  type='button'
                  className='router-inline-button'
                  loading={dailyQuotaLoading}
                  disabled={dailyQuotaLoading || loading || actionLoading !== ''}
                  onClick={() => loadDailyQuota()}
                >
                  {t('user.buttons.refresh')}
                </Button>
              </div>
              <Form.Group widths='equal'>
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.daily_quota_group')}
                  value={readOnlyValue(dailyQuota.group_name || groupDisplayValue)}
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.daily_quota_limit')}
                  value={dailyQuota.unlimited ? t('common.unlimited') : Number(dailyQuota.limit || 0)}
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.daily_quota_consumed')}
                  value={Number(dailyQuota.consumed_quota || 0)}
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.daily_quota_remaining')}
                  value={
                    dailyQuota.unlimited ? t('common.unlimited') : Number(dailyQuota.remaining_quota || 0)
                  }
                  readOnly
                />
              </Form.Group>
              <Form.Group widths='equal'>
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.daily_quota_reserved')}
                  value={Number(dailyQuota.reserved_quota || 0)}
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.daily_quota_biz_date')}
                  value={readOnlyValue(dailyQuota.biz_date)}
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.daily_quota_timezone')}
                  value={readOnlyValue(dailyQuota.timezone)}
                  readOnly
                />
                <Form.Input
                  className='router-section-input'
                  label={t('user.detail.daily_quota_updated_at')}
                  value={dailyQuota.updated_at ? new Date(Number(dailyQuota.updated_at) * 1000).toLocaleString('zh-CN', { hour12: false }) : '-'}
                  readOnly
                />
              </Form.Group>
            </div>
            {isEditing ? (
              <Form.Input
                className='router-section-input'
                label={t('user.edit.email')}
                name='email'
                value={editInputs.email}
                placeholder={t('user.edit.email_placeholder')}
                onChange={handleEditInputChange}
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
          </Form>
        </Card.Content>
      </Card>
    </div>
  );
};

export default UserDetail;
