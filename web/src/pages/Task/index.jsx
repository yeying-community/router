import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Dropdown,
  Form,
  Header,
  Label,
  Pagination,
  Popup,
  Table,
} from 'semantic-ui-react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';

const PAGE_SIZE = 20;
const TASK_PAGE_KIND_WORKSPACE_USER = 'workspace_user';
const TASK_PAGE_KIND_ADMIN_USER = 'admin_user';
const TASK_PAGE_KIND_ADMIN_SYSTEM = 'admin_system';

const normalizeTaskStatus = (value) => {
  const normalized = (value || '').toString().trim().toLowerCase();
  switch (normalized) {
    case 'pending':
    case 'queued':
      return 'pending';
    case 'running':
    case 'processing':
    case 'in_progress':
      return 'running';
    case 'succeeded':
    case 'success':
    case 'completed':
      return 'succeeded';
    case 'failed':
    case 'error':
      return 'failed';
    case 'canceled':
    case 'cancelled':
      return 'canceled';
    default:
      return normalized || 'pending';
  }
};

const taskStatusColor = (status) => {
  switch (normalizeTaskStatus(status)) {
    case 'running':
      return 'blue';
    case 'succeeded':
      return 'green';
    case 'failed':
      return 'red';
    case 'canceled':
      return 'grey';
    default:
      return 'orange';
  }
};

const resolveTaskPageKind = (pathname = '') => {
  const normalizedPath = (pathname || '').toString().trim().toLowerCase();
  if (normalizedPath.startsWith('/admin/channel/tasks')) {
    return TASK_PAGE_KIND_ADMIN_SYSTEM;
  }
  if (normalizedPath.startsWith('/admin/task')) {
    return TASK_PAGE_KIND_ADMIN_USER;
  }
  return TASK_PAGE_KIND_WORKSPACE_USER;
};

const getTaskEndpoint = (pageKind) => {
  switch (pageKind) {
    case TASK_PAGE_KIND_ADMIN_SYSTEM:
      return '/api/v1/admin/tasks';
    case TASK_PAGE_KIND_ADMIN_USER:
      return '/api/v1/admin/user/tasks';
    default:
      return '/api/v1/public/user/tasks';
  }
};

const getTaskOptionsEndpoint = (pageKind) => {
  switch (pageKind) {
    case TASK_PAGE_KIND_ADMIN_SYSTEM:
      return '/api/v1/admin/tasks/options';
    case TASK_PAGE_KIND_ADMIN_USER:
      return '/api/v1/admin/user/tasks/options';
    default:
      return '/api/v1/public/user/tasks/options';
  }
};

const getTaskId = (item) => item?.id || item?.task_id || '';

const getTaskTypeOptions = (t, scope) => {
  const common = [{ key: 'all', value: '', text: t('task.filters.type_all') }];
  if (scope === 'user') {
    common.push({
      key: 'video',
      value: 'video',
      text: t('task.types.video'),
    });
    return common;
  }
  common.push(
    {
      key: 'channel_model_test',
      value: 'channel_model_test',
      text: t('task.types.channel_model_test'),
    },
    {
      key: 'channel_refresh_models',
      value: 'channel_refresh_models',
      text: t('task.types.channel_refresh_models'),
    },
    {
      key: 'channel_refresh_balance',
      value: 'channel_refresh_balance',
      text: t('task.types.channel_refresh_balance'),
    },
  );
  return common;
};

const getTaskStatusOptions = (t) => [
  { key: 'all', value: '', text: t('task.filters.status_all') },
  { key: 'pending', value: 'pending', text: t('task.status.pending') },
  { key: 'running', value: 'running', text: t('task.status.running') },
  { key: 'succeeded', value: 'succeeded', text: t('task.status.succeeded') },
  { key: 'failed', value: 'failed', text: t('task.status.failed') },
  { key: 'canceled', value: 'canceled', text: t('task.status.canceled') },
];

const renderTaskFilterSummary = (filterKey, filters, t, optionResolvers = {}) => {
  const value = (filters?.[filterKey] || '').toString().trim();
  if (value === '') {
    return t('task.filters.not_set');
  }
  if (typeof optionResolvers[filterKey] === 'function') {
    return optionResolvers[filterKey](value) || value;
  }
  return value;
};

const Task = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const pageKind = useMemo(
    () => resolveTaskPageKind(location.pathname),
    [location.pathname],
  );
  const isAdminPage = pageKind !== TASK_PAGE_KIND_WORKSPACE_USER;
  const isSystemTaskPage = pageKind === TASK_PAGE_KIND_ADMIN_SYSTEM;
  const isAdminUserTaskPage = pageKind === TASK_PAGE_KIND_ADMIN_USER;
  const isUserTaskPage = !isSystemTaskPage;
  const initialQuery = useMemo(
    () => new URLSearchParams(location.search),
    [location.search],
  );
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(() => {
    const parsed = Number(initialQuery.get('page') || 1);
    return Number.isInteger(parsed) && parsed > 0 ? parsed : 1;
  });
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState(() => ({
    type: (initialQuery.get('type') || '').trim(),
    status: (initialQuery.get('status') || '').trim(),
    channel_id: (initialQuery.get('channel_id') || '').trim(),
    model: (initialQuery.get('model') || '').trim(),
    user_keyword: (initialQuery.get('user_keyword') || '').trim(),
  }));
  const [activeFilterKeys, setActiveFilterKeys] = useState(() => {
    const keys = [];
    if ((initialQuery.get('type') || '').trim() !== '') {
      keys.push('type');
    }
    if ((initialQuery.get('status') || '').trim() !== '') {
      keys.push('status');
    }
    if ((initialQuery.get('channel_id') || '').trim() !== '') {
      keys.push('channel_id');
    }
    if ((initialQuery.get('model') || '').trim() !== '') {
      keys.push('model');
    }
    if ((initialQuery.get('user_keyword') || '').trim() !== '') {
      keys.push('user_keyword');
    }
    return keys;
  });
  const [addFilterPopupOpen, setAddFilterPopupOpen] = useState(false);
  const [draftFilterKey, setDraftFilterKey] = useState('');
  const [draftFilterValue, setDraftFilterValue] = useState('');
  const [filterOptions, setFilterOptions] = useState({
    models: [],
    channels: [],
    users: [],
  });

  const totalPages = useMemo(
    () => Math.max(1, Math.ceil(total / PAGE_SIZE)),
    [total],
  );

  const taskTypeOptions = useMemo(
    () => getTaskTypeOptions(t, isUserTaskPage ? 'user' : 'admin'),
    [isUserTaskPage, t],
  );
  const taskStatusOptions = useMemo(() => getTaskStatusOptions(t), [t]);
  const endpoint = useMemo(
    () => getTaskEndpoint(pageKind),
    [pageKind],
  );
  const optionsEndpoint = useMemo(
    () => getTaskOptionsEndpoint(pageKind),
    [pageKind],
  );
  const conditionalFilterConfig = useMemo(() => {
    const items = [
      {
        key: 'type',
        label: t('task.table.type'),
        type: 'select',
        options: taskTypeOptions.filter((item) => (item.value || '') !== ''),
      },
      {
        key: 'status',
        label: t('task.table.status'),
        type: 'select',
        options: taskStatusOptions.filter((item) => (item.value || '') !== ''),
      },
      {
        key: 'model',
        label: t('task.table.model'),
        type: filterOptions.models.length > 0 ? 'select' : 'text',
        options: filterOptions.models.map((item) => ({
          key: item.value,
          text: item.label,
          value: item.value,
        })),
        placeholder: t('task.filters.model'),
      },
    ];
    if (isSystemTaskPage) {
      items.push({
        key: 'channel_id',
        label: t('task.table.channel'),
        type: filterOptions.channels.length > 0 ? 'select' : 'text',
        options: filterOptions.channels.map((item) => ({
          key: item.value,
          text: item.label,
          value: item.value,
        })),
        placeholder: t('task.filters.channel_id'),
      });
    }
    if (isAdminUserTaskPage) {
      items.push({
        key: 'user_keyword',
        label: t('task.table.user'),
        type: filterOptions.users.length > 0 ? 'select' : 'text',
        options: filterOptions.users.map((item) => ({
          key: item.value,
          text: item.label,
          value: item.value,
        })),
        placeholder: t('task.filters.user_keyword'),
      });
    }
    return items;
  }, [
    filterOptions.channels,
    filterOptions.models,
    filterOptions.users,
    isAdminUserTaskPage,
    isSystemTaskPage,
    t,
    taskStatusOptions,
    taskTypeOptions,
  ]);
  const conditionalFilterOptions = useMemo(
    () =>
      conditionalFilterConfig.map((item) => ({
        key: item.key,
        text: item.label,
        value: item.key,
      })),
    [conditionalFilterConfig],
  );
  const visibleFilterConfig = useMemo(
    () =>
      conditionalFilterConfig.filter((item) =>
        activeFilterKeys.includes(item.key),
      ),
    [activeFilterKeys, conditionalFilterConfig],
  );
  const availableConditionalFilterOptions = useMemo(
    () =>
      conditionalFilterOptions.filter(
        (item) => !activeFilterKeys.includes(item.value),
      ),
    [activeFilterKeys, conditionalFilterOptions],
  );

  const closeFilterDraft = useCallback(() => {
    setAddFilterPopupOpen(false);
    setDraftFilterKey('');
    setDraftFilterValue('');
  }, []);

  const openFilterDraft = useCallback(
    (filterKey) => {
      const config = conditionalFilterConfig.find((item) => item.key === filterKey);
      if (!config) {
        return;
      }
      setDraftFilterKey(filterKey);
      setDraftFilterValue((filters?.[filterKey] || '').toString());
      setAddFilterPopupOpen(true);
    },
    [conditionalFilterConfig, filters],
  );

  const applyFilterDraft = useCallback(() => {
    const nextFilterKey = (draftFilterKey || '').trim();
    if (nextFilterKey === '') {
      return;
    }
    const nextValue = (draftFilterValue || '').toString().trim();
    if (nextValue === '') {
      showError(t('task.filters.value_required'));
      return;
    }
    setFilters((prev) => ({
      ...prev,
      [nextFilterKey]: nextValue,
    }));
    setActiveFilterKeys((prev) =>
      prev.includes(nextFilterKey) ? prev : [...prev, nextFilterKey],
    );
    setPage(1);
    closeFilterDraft();
  }, [closeFilterDraft, draftFilterKey, draftFilterValue, t]);

  const removeConditionalFilter = useCallback((filterKey) => {
    setActiveFilterKeys((prev) => prev.filter((item) => item !== filterKey));
    setFilters((prev) => ({
      ...prev,
      [filterKey]: '',
    }));
    setPage(1);
  }, []);

  const resolveTypeLabel = useCallback(
    (value) =>
      taskTypeOptions.find((item) => item.value === value)?.text || value,
    [taskTypeOptions],
  );

  const resolveStatusLabel = useCallback(
    (value) =>
      taskStatusOptions.find((item) => item.value === value)?.text || value,
    [taskStatusOptions],
  );

  const loadFilterOptions = useCallback(async () => {
    setFilterOptions({
      models: [],
      channels: [],
      users: [],
    });
    try {
      const res = await API.get(optionsEndpoint);
      const { success, message, data } = res.data || {};
      if (!success) {
        setFilterOptions({
          models: [],
          channels: [],
          users: [],
        });
        showError(message || t('task.messages.load_failed'));
        return;
      }
      setFilterOptions({
        models: Array.isArray(data?.models) ? data.models : [],
        channels: Array.isArray(data?.channels) ? data.channels : [],
        users: Array.isArray(data?.users) ? data.users : [],
      });
    } catch (error) {
      setFilterOptions({
        models: [],
        channels: [],
        users: [],
      });
      showError(error?.message || t('task.messages.load_failed'));
    }
  }, [optionsEndpoint, t]);

  const loadTasks = useCallback(
    async (targetPage = 1) => {
      setLoading(true);
      try {
        const enabledFilters = new Set(activeFilterKeys);
        const res = await API.get(endpoint, {
          params: {
            page: targetPage,
            page_size: PAGE_SIZE,
            type: enabledFilters.has('type') ? filters.type : '',
            status: enabledFilters.has('status') ? filters.status : '',
            channel_id: enabledFilters.has('channel_id')
              ? filters.channel_id.trim()
              : '',
            model: enabledFilters.has('model') ? filters.model.trim() : '',
            user_keyword:
              isAdminUserTaskPage && enabledFilters.has('user_keyword')
                ? filters.user_keyword.trim()
                : '',
          },
        });
        const { success, message, data } = res.data || {};
        if (!success) {
          showError(message || t('task.messages.load_failed'));
          return;
        }
        setItems(Array.isArray(data?.items) ? data.items : []);
        setTotal(Number(data?.total || 0));
        setPage(Number(data?.page || targetPage || 1));
      } catch (error) {
        showError(error?.message || t('task.messages.load_failed'));
      } finally {
        setLoading(false);
      }
    },
    [
      activeFilterKeys,
      endpoint,
      filters.channel_id,
      filters.model,
      filters.status,
      filters.type,
      filters.user_keyword,
      isAdminUserTaskPage,
      t,
    ],
  );

  useEffect(() => {
    loadTasks(1).then();
  }, [loadTasks]);

  useEffect(() => {
    loadFilterOptions().then();
  }, [loadFilterOptions]);

  useEffect(() => {
    const query = new URLSearchParams();
    if (page > 1) {
      query.set('page', String(page));
    }
    if (activeFilterKeys.includes('type') && filters.type) {
      query.set('type', filters.type);
    }
    if (activeFilterKeys.includes('status') && filters.status) {
      query.set('status', filters.status);
    }
    if (activeFilterKeys.includes('model') && filters.model.trim()) {
      query.set('model', filters.model.trim());
    }
    if (
      isAdminUserTaskPage &&
      activeFilterKeys.includes('user_keyword') &&
      filters.user_keyword.trim()
    ) {
      query.set('user_keyword', filters.user_keyword.trim());
    }
    if (
      isSystemTaskPage &&
      activeFilterKeys.includes('channel_id') &&
      filters.channel_id.trim()
    ) {
      query.set('channel_id', filters.channel_id.trim());
    }
    const nextSearch = query.toString();
    navigate(
      {
        pathname: location.pathname,
        search: nextSearch ? `?${nextSearch}` : '',
      },
      { replace: true },
    );
  }, [
    activeFilterKeys,
    filters.channel_id,
    filters.model,
    filters.status,
    filters.type,
    filters.user_keyword,
    isAdminUserTaskPage,
    isSystemTaskPage,
    location.pathname,
    navigate,
    page,
  ]);

  useEffect(() => {
    setActiveFilterKeys((prev) =>
      prev.filter((item) => (filters?.[item] || '').toString().trim() !== ''),
    );
  }, [
    filters.channel_id,
    filters.model,
    filters.status,
    filters.type,
    filters.user_keyword,
  ]);

  useEffect(() => {
    const allowedFilterKeys = new Set(
      conditionalFilterConfig.map((item) => item.key),
    );
    setActiveFilterKeys((prev) =>
      prev.filter((item) => allowedFilterKeys.has(item)),
    );
    if (!allowedFilterKeys.has('user_keyword') && filters.user_keyword !== '') {
      setFilters((prev) => ({
        ...prev,
        user_keyword: '',
      }));
    }
    if (!allowedFilterKeys.has('channel_id') && filters.channel_id !== '') {
      setFilters((prev) => ({
        ...prev,
        channel_id: '',
      }));
    }
    if (
      filters.type !== '' &&
      !taskTypeOptions.some((item) => item.value === filters.type)
    ) {
      setFilters((prev) => ({
        ...prev,
        type: '',
      }));
      setActiveFilterKeys((prev) => prev.filter((item) => item !== 'type'));
    }
    if (draftFilterKey !== '' && !allowedFilterKeys.has(draftFilterKey)) {
      closeFilterDraft();
    }
  }, [
    closeFilterDraft,
    conditionalFilterConfig,
    draftFilterKey,
    filters.channel_id,
    filters.type,
    filters.user_keyword,
    taskTypeOptions,
  ]);

  useEffect(() => {
    const hasActive = items.some((item) => {
      const status = normalizeTaskStatus(item?.status);
      return status === 'pending' || status === 'running';
    });
    if (!hasActive) {
      return undefined;
    }
    const timer = window.setInterval(() => {
      loadTasks(page).then();
    }, 2500);
    return () => window.clearInterval(timer);
  }, [items, loadTasks, page]);

  const handleRetryTask = async (taskId) => {
    try {
      const res = await API.post(`/api/v1/admin/tasks/${taskId}/retry`);
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('task.messages.retry_failed'));
        return;
      }
      showSuccess(t('task.messages.retry_success'));
      loadTasks(page).then();
    } catch (error) {
      showError(error?.message || t('task.messages.retry_failed'));
    }
  };

  const handleCancelTask = async (taskId) => {
    try {
      const res = await API.post(`/api/v1/admin/tasks/${taskId}/cancel`);
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('task.messages.cancel_failed'));
        return;
      }
      showSuccess(t('task.messages.cancel_success'));
      loadTasks(page).then();
    } catch (error) {
      showError(error?.message || t('task.messages.cancel_failed'));
    }
  };

  const detailBasePath = isSystemTaskPage
    ? '/admin/channel/tasks'
    : isAdminPage
      ? '/admin/task'
      : '/workspace/task';
  const pageTitle = isSystemTaskPage
    ? t('task.scopes.admin')
    : isAdminUserTaskPage
      ? t('task.scopes.user')
      : t('task.title');
  const resolveFilterOptionLabel = useCallback(
    (filterKey, value) => {
      const normalizedValue = (value || '').toString().trim();
      if (normalizedValue === '') {
        return '';
      }
      if (filterKey === 'model') {
        return (
          filterOptions.models.find((item) => item.value === normalizedValue)
            ?.label || normalizedValue
        );
      }
      if (filterKey === 'channel_id') {
        return (
          filterOptions.channels.find((item) => item.value === normalizedValue)
            ?.label || normalizedValue
        );
      }
      if (filterKey === 'user_keyword') {
        return (
          filterOptions.users.find((item) => item.value === normalizedValue)
            ?.label || normalizedValue
        );
      }
      return normalizedValue;
    },
    [filterOptions.channels, filterOptions.models, filterOptions.users],
  );

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <Header as='h3' className='router-section-title'>
            {pageTitle}
          </Header>
          <Form className='router-toolbar router-log-toolbar router-block-gap-sm'>
            <div className='router-toolbar-start router-log-toolbar-start'>
              <Popup
                open={addFilterPopupOpen}
                on='click'
                position='bottom left'
                onClose={closeFilterDraft}
                trigger={
                  <Button
                    type='button'
                    className='router-page-button'
                    disabled={availableConditionalFilterOptions.length === 0}
                    onClick={() => setAddFilterPopupOpen(true)}
                  >
                    {t('task.filters.add')}
                  </Button>
                }
                content={
                  <div className='router-log-filter-picker'>
                    <div className='router-log-filter-picker-options'>
                      {availableConditionalFilterOptions.map((item) => (
                        <Button
                          key={item.value}
                          type='button'
                          size='mini'
                          className='router-inline-button'
                          primary={draftFilterKey === item.value}
                          onClick={() => openFilterDraft(item.value)}
                        >
                          {item.text}
                        </Button>
                      ))}
                    </div>
                    {draftFilterKey !== '' && (
                      <div className='router-log-filter-editor'>
                        <div className='router-log-filter-editor-title'>
                          {
                            conditionalFilterConfig.find(
                              (item) => item.key === draftFilterKey,
                            )?.label
                          }
                        </div>
                        {conditionalFilterConfig.find(
                          (item) => item.key === draftFilterKey,
                        )?.type === 'select' ? (
                          <Dropdown
                            className='router-section-dropdown router-log-filter-select'
                            fluid
                            search
                            selection
                            clearable
                            options={
                              conditionalFilterConfig.find(
                                (item) => item.key === draftFilterKey,
                              )?.options || []
                            }
                            value={draftFilterValue}
                            onChange={(e, { value }) =>
                              setDraftFilterValue(value ? String(value) : '')
                            }
                          />
                        ) : (
                          <input
                            className='router-log-filter-editor-input'
                            type='text'
                            value={draftFilterValue}
                            placeholder={
                              conditionalFilterConfig.find(
                                (item) => item.key === draftFilterKey,
                              )?.placeholder || ''
                            }
                            onChange={(e) =>
                              setDraftFilterValue(e.target.value)
                            }
                          />
                        )}
                        <div className='router-log-filter-editor-actions'>
                          <Button
                            type='button'
                            size='mini'
                            className='router-inline-button'
                            onClick={closeFilterDraft}
                          >
                            {t('common.cancel')}
                          </Button>
                          <Button
                            type='button'
                            size='mini'
                            className='router-inline-button'
                            primary
                            onClick={applyFilterDraft}
                          >
                            {t('common.confirm')}
                          </Button>
                        </div>
                      </div>
                    )}
                  </div>
                }
              />
            </div>
            <div className='router-toolbar-end router-log-query-wrap'>
              <div className='router-log-query-box router-log-query-box-inline'>
                <div className='router-log-query-fields'>
                  {visibleFilterConfig.length === 0 ? (
                    <div className='router-log-filter-chip router-log-filter-chip-static'>
                      <span className='router-log-filter-chip-label'>
                        {t('task.filters.none')}
                      </span>
                    </div>
                  ) : (
                    visibleFilterConfig.map((item) => (
                      <div
                        key={item.key}
                        className='router-log-filter-chip router-log-filter-chip-static'
                      >
                        <span className='router-log-filter-chip-label'>
                          {item.label}
                        </span>
                        <span className='router-log-filter-chip-value'>
                          {renderTaskFilterSummary(item.key, filters, t, {
                            type: resolveTypeLabel,
                            status: resolveStatusLabel,
                            model: (value) =>
                              resolveFilterOptionLabel('model', value),
                            channel_id: (value) =>
                              resolveFilterOptionLabel('channel_id', value),
                            user_keyword: (value) =>
                              resolveFilterOptionLabel('user_keyword', value),
                          })}
                        </span>
                        <button
                          type='button'
                          className='router-log-filter-chip-remove'
                          onClick={() => removeConditionalFilter(item.key)}
                        >
                          ×
                        </button>
                      </div>
                    ))
                  )}
                </div>
              </div>
              <Button
                type='button'
                className='router-page-button router-log-query-button'
                onClick={() => loadTasks(page)}
                loading={loading}
              >
                {t('task.buttons.query')}
              </Button>
            </div>
          </Form>

          <Table basic='very' compact className='router-list-table'>
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell>{t('task.table.type')}</Table.HeaderCell>
                {isAdminUserTaskPage ? (
                  <Table.HeaderCell>{t('task.table.user')}</Table.HeaderCell>
                ) : null}
                <Table.HeaderCell>{t('task.table.channel')}</Table.HeaderCell>
                <Table.HeaderCell>{t('task.table.model')}</Table.HeaderCell>
                <Table.HeaderCell>{t('task.table.status')}</Table.HeaderCell>
                <Table.HeaderCell>
                  {t('task.table.created_at')}
                </Table.HeaderCell>
                <Table.HeaderCell>
                  {isUserTaskPage
                    ? t('task.table.updated_at')
                    : t('task.table.finished_at')}
                </Table.HeaderCell>
                <Table.HeaderCell>{t('task.table.actions')}</Table.HeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {items.length === 0 ? (
                <Table.Row>
                  <Table.Cell
                    colSpan={isAdminUserTaskPage ? '8' : '7'}
                    className='router-empty-cell'
                  >
                    {loading ? t('common.loading') : t('task.empty')}
                  </Table.Cell>
                </Table.Row>
              ) : (
                items.map((item) => {
                  const taskId = getTaskId(item);
                  const rawStatus = (item?.status || '')
                    .toString()
                    .trim()
                    .toLowerCase();
                  const status = normalizeTaskStatus(rawStatus);
                  const canCancel =
                    isSystemTaskPage &&
                    (status === 'pending' || status === 'running');
                  const canRetry =
                    isSystemTaskPage &&
                    (status === 'failed' || status === 'canceled');
                  return (
                    <Table.Row
                      key={taskId}
                      className='router-row-clickable'
                      onClick={() => navigate(`${detailBasePath}/${taskId}`)}
                    >
                      <Table.Cell>
                        {t(`task.types.${item.type || 'video'}`)}
                      </Table.Cell>
                      {isAdminUserTaskPage ? (
                        <Table.Cell>
                          {item.user_name || item.user_id || '-'}
                        </Table.Cell>
                      ) : null}
                      <Table.Cell>
                        {item.channel_name || item.channel_id || '-'}
                      </Table.Cell>
                      <Table.Cell>{item.model || '-'}</Table.Cell>
                      <Table.Cell>
                        <Label
                          basic
                          color={taskStatusColor(rawStatus)}
                          className='router-tag'
                        >
                          {t(`task.status.${status}`)}
                        </Label>
                      </Table.Cell>
                      <Table.Cell>
                        {item.created_at
                          ? timestamp2string(item.created_at)
                          : '-'}
                      </Table.Cell>
                      <Table.Cell>
                        {isUserTaskPage
                          ? item.updated_at
                            ? timestamp2string(item.updated_at)
                            : '-'
                          : item.finished_at
                            ? timestamp2string(item.finished_at)
                            : '-'}
                      </Table.Cell>
                      <Table.Cell collapsing>
                        {isUserTaskPage ? (
                          <Button
                            type='button'
                            className='router-inline-button'
                            basic
                            onClick={(e) => {
                              e.stopPropagation();
                              navigate(`${detailBasePath}/${taskId}`);
                            }}
                          >
                            {t('task.buttons.view')}
                          </Button>
                        ) : (
                          <div className='router-inline-actions'>
                            <Button
                              type='button'
                              className='router-inline-button'
                              basic
                              disabled={!canRetry}
                              onClick={(e) => {
                                e.stopPropagation();
                                handleRetryTask(taskId);
                              }}
                            >
                              {t('task.buttons.retry')}
                            </Button>
                            <Button
                              type='button'
                              className='router-inline-button'
                              basic
                              disabled={!canCancel}
                              onClick={(e) => {
                                e.stopPropagation();
                                handleCancelTask(taskId);
                              }}
                            >
                              {t('task.buttons.cancel')}
                            </Button>
                          </div>
                        )}
                      </Table.Cell>
                    </Table.Row>
                  );
                })
              )}
            </Table.Body>
            <Table.Footer>
              <Table.Row>
                <Table.HeaderCell
                  colSpan={isAdminUserTaskPage ? '8' : '7'}
                >
                  <div className='router-toolbar router-task-footer-toolbar'>
                    <div className='router-toolbar-start'>
                      <span className='router-toolbar-meta'>
                        {t('task.summary', { total })}
                      </span>
                    </div>
                    <div className='router-toolbar-end'>
                      <Pagination
                        className='router-page-pagination'
                        activePage={page}
                        totalPages={totalPages}
                        siblingRange={1}
                        boundaryRange={0}
                        onPageChange={(e, { activePage }) => {
                          const nextPage = Number(activePage || 1);
                          setPage(nextPage);
                          loadTasks(nextPage).then();
                        }}
                      />
                    </div>
                  </div>
                </Table.HeaderCell>
              </Table.Row>
            </Table.Footer>
          </Table>
        </Card.Content>
      </Card>
    </div>
  );
};

export default Task;
