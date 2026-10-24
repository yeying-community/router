import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
import { API, showError, showInfo, showSuccess, timestamp2string } from '../../helpers';
import useBatchRowActions from '../../hooks/useBatchRowActions';
import useList, {
  adaptListResponse,
  sorterToSort,
  sortOrderForColumn,
} from '../../hooks/useList';
import { parseListPageSize } from '../../hooks/useUrlState';
import {
  TASK_LIST_COLUMN_WIDTHS,
  TASK_LIST_TABLE_MIN_WIDTH,
} from '../../constants/tableWidthPresets';
import {
  AppButton,
  AppEmpty,
  AppErrorState,
  AppFilterHeader,
  AppPagination,
  AppPopconfirm,
  AppTable,
  AppTableActionButton,
  AppTag,
  AppToolbar,
} from '../../router-ui';
import ListFilterBar from '../../components/ListFilterBar';
import ChannelSectionTabs from '../../components/ChannelSectionTabs';
import UserSectionTabs from '../../components/UserSectionTabs';

const PAGE_SIZE = 20;
const TASK_QUERY_KEYS = [
  'page',
  'page_size',
  'order_by',
  'order',
  'type',
  'status',
  'channel_id',
  'model',
  'user_keyword',
];
export const TASK_PAGE_KIND_WORKSPACE_USER = 'workspace_user';
export const TASK_PAGE_KIND_ADMIN_USER = 'admin_user';
export const TASK_PAGE_KIND_ADMIN_SYSTEM = 'admin_system';

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

const parseTaskJSONField = (value) => {
  const text = (value || '').toString().trim();
  if (text === '') {
    return null;
  }
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
};

const parseDownloadFilename = (contentDisposition, fallbackName) => {
  const fallback =
    (fallbackName || 'download.bin').toString().trim() || 'download.bin';
  const headerValue = (contentDisposition || '').toString();
  const utf8Match = headerValue.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1]);
    } catch {
      return utf8Match[1];
    }
  }
  const plainMatch = headerValue.match(/filename=\"?([^\";]+)\"?/i);
  if (plainMatch?.[1]) {
    return plainMatch[1];
  }
  return fallback;
};

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
      key: 'channel_refresh_billing',
      value: 'channel_refresh_billing',
      text: t('task.types.channel_refresh_billing'),
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

const Task = ({ pageKind: pageKindOverride = '', embedded = false }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const currentPagePath = `${location.pathname}${location.search}${location.hash}`;
  const returnPath = useMemo(() => {
    const from = location.state?.from;
    if (typeof from !== 'string') {
      return '';
    }
    const normalized = from.trim();
    return normalized.startsWith('/') ? normalized : '';
  }, [location.state]);
  const returnLabel = useMemo(() => {
    const raw = location.state?.fromLabel;
    if (typeof raw !== 'string') {
      return '';
    }
    return raw.trim();
  }, [location.state]);
  const contextType = useMemo(() => {
    const raw = location.state?.contextType;
    if (typeof raw !== 'string') {
      return '';
    }
    return raw.trim();
  }, [location.state]);
  const contextLabel = useMemo(() => {
    const raw = location.state?.contextLabel;
    if (typeof raw !== 'string') {
      return '';
    }
    return raw.trim();
  }, [location.state]);
  const taskPageNavState = useMemo(() => {
    if (
      returnPath === '' &&
      returnLabel === '' &&
      contextType === '' &&
      contextLabel === ''
    ) {
      return undefined;
    }
    return {
      from: returnPath,
      fromLabel: returnLabel,
      contextType,
      contextLabel,
    };
  }, [contextLabel, contextType, returnLabel, returnPath]);
  const pageKind = useMemo(() => {
    if (pageKindOverride) {
      return pageKindOverride;
    }
    return resolveTaskPageKind(location.pathname);
  }, [location.pathname, pageKindOverride]);
  const isAdminPage = pageKind !== TASK_PAGE_KIND_WORKSPACE_USER;
  const isSystemTaskPage = pageKind === TASK_PAGE_KIND_ADMIN_SYSTEM;
  const isAdminUserTaskPage = pageKind === TASK_PAGE_KIND_ADMIN_USER;
  const isUserTaskPage = !isSystemTaskPage;
  const initialQuery = useMemo(
    () => new URLSearchParams(location.search),
    [location.search],
  );
  const [filters, setFilters] = useState(() => ({
    type: (initialQuery.get('type') || '').trim(),
    status: (initialQuery.get('status') || '').trim(),
    channel_id: (initialQuery.get('channel_id') || '').trim(),
    model: (initialQuery.get('model') || '').trim(),
    user_keyword: (initialQuery.get('user_keyword') || '').trim(),
  }));
  const [batchRunning, setBatchRunning] = useState(false);
  const batchActions = useBatchRowActions();
  const { isSelecting: isBatchSelecting, selectedCount: batchSelectedCount } = batchActions;
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
  const [filterOptions, setFilterOptions] = useState({
    models: [],
    channels: [],
    users: [],
  });

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

  const fetchTasks = useCallback(
    async ({ page: reqPage, pageSize, orderBy, order }) => {
      const enabledFilters = new Set(activeFilterKeys);
      const res = await API.get(endpoint, {
        params: {
          page: reqPage,
          page_size: pageSize,
          order_by: orderBy || '',
          order: order || '',
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
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('task.messages.load_failed'));
        throw new Error(message || 'load failed');
      }
      return adaptListResponse(res.data);
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

  const {
    rows: items,
    total,
    loading,
    loadError,
    page,
    pageSize,
    sort,
    setPage,
    load: loadTasks,
    setPageSize,
    setSort,
  } = useList({
    fetcher: fetchTasks,
    pageSize: (() => {
      const raw = initialQuery.get('page_size');
      return raw ? parseListPageSize(raw) : PAGE_SIZE;
    })(),
    initialPage: (() => {
      const parsed = Number(initialQuery.get('page') || 1);
      return Number.isInteger(parsed) && parsed > 0 ? parsed : 1;
    })(),
    initialSort: (() => {
      const field = (initialQuery.get('order_by') || '').trim();
      if (field === '') {
        return null;
      }
      return { field, order: (initialQuery.get('order') || '').trim() === 'asc' ? 'asc' : 'desc' };
    })(),
  });

  const handleTableChange = useCallback(
    (_pagination, _filters, sorter) => {
      setSort(sorterToSort(sorter));
    },
    [setSort],
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

  const getTaskFilterConfig = useCallback(
    (filterKey) =>
      conditionalFilterConfig.find((item) => item.key === filterKey) || null,
    [conditionalFilterConfig],
  );

  const getTaskInitialDraft = useCallback(
    (filterKey) => ({ value: (filters?.[filterKey] || '').toString() }),
    [filters],
  );

  const applyTaskFilterDraft = useCallback(
    (filterKey, draft) => {
      const nextFilterKey = (filterKey || '').trim();
      if (nextFilterKey === '') {
        return false;
      }
      const nextValue = (draft?.value || '').toString().trim();
      if (nextValue === '') {
        showError(t('task.filters.value_required'));
        return false;
      }
      setFilters((prev) => ({
        ...prev,
        [nextFilterKey]: nextValue,
      }));
      setActiveFilterKeys((prev) =>
        prev.includes(nextFilterKey) ? prev : [...prev, nextFilterKey],
      );
      setPage(1);
      return true;
    },
    [setPage, t],
  );

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

  // Reload page 1 whenever the request shape (filters / endpoint) changes.
  // `loadTasks` (useList.load) is stable, so `fetchTasks` is the real trigger.
  useEffect(() => {
    loadTasks(1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fetchTasks]);

  useEffect(() => {
    loadFilterOptions().then();
  }, [loadFilterOptions]);

  useEffect(() => {
    // Task can be embedded in a domain shell such as
    // /admin/user?tab=tasks. Only replace task-owned query keys; rebuilding
    // the entire query would erase the host tab and immediately return users
    // to the shell default view.
    const query = new URLSearchParams(location.search);
    TASK_QUERY_KEYS.forEach((key) => query.delete(key));
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
    if (sort?.field) {
      query.set('order_by', sort.field);
      query.set('order', sort.order === 'asc' ? 'asc' : 'desc');
    }
    if (Number(pageSize) !== PAGE_SIZE) {
      query.set('page_size', String(pageSize));
    }
    const nextSearch = query.toString();
    const currentSearch = location.search.startsWith('?')
      ? location.search.slice(1)
      : location.search;
    if (nextSearch === currentSearch) {
      return;
    }
    navigate(
      {
        pathname: location.pathname,
        search: nextSearch ? `?${nextSearch}` : '',
      },
      { replace: true, state: taskPageNavState },
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
    location.search,
    location.pathname,
    navigate,
    page,
    pageSize,
    sort,
    taskPageNavState,
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
  }, [
    conditionalFilterConfig,
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
  }, [batchRunning, isBatchSelecting, items, loadTasks, page]);

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

  // 批量重试/取消:后端无批量接口,沿用 ChannelsTable/告警面板的「串行循环+聚合
  // toast」模式;仅后台执行兼容视图启用,用户任务不可触发(per-row 已被隐藏)。
  const runBatchTaskAction = useCallback(
    async (action) => {
      if (batchRunning) return;
      if (action !== 'retry' && action !== 'cancel') return;
      if (!isSystemTaskPage) return;
      const selected = new Set(batchActions.selectedRowKeys);
      const targets = items.filter((item) => {
        const id = getTaskId(item);
        if (!selected.has(id)) return false;
        const status = normalizeTaskStatus(item?.status);
        return action === 'retry'
          ? status === 'failed' || status === 'canceled'
          : status === 'pending' || status === 'running';
      });
      if (targets.length === 0) {
        showInfo(t(`task.batch.no_${action}_target`));
        return;
      }
      const endpoint =
        action === 'retry'
          ? (id) => `/api/v1/admin/tasks/${id}/retry`
          : (id) => `/api/v1/admin/tasks/${id}/cancel`;
      setBatchRunning(true);
      let successCount = 0;
      const failures = [];
      for (const item of targets) {
        const id = getTaskId(item);
        try {
          const res = await API.post(endpoint(id));
          if (res?.data?.success) {
            successCount += 1;
          } else {
            failures.push(id);
          }
        } catch (error) {
          console.error(`Failed to batch ${action} task:`, error);
          failures.push(id);
        }
      }
      setBatchRunning(false);
      const failedCount = failures.length;
      if (failedCount === 0) {
        showSuccess(
          t(`task.batch.all_success_${action}`, { count: successCount }),
        );
      } else if (successCount === 0) {
        showError(
          t(`task.batch.all_failed_${action}`, { count: failedCount }),
        );
      } else {
        showError(
          t('task.batch.partial', {
            action: t(`task.batch.action_${action}`),
            success: successCount,
            failed: failedCount,
          }),
        );
      }
      batchActions.exit();
      loadTasks(page).then();
    },
    [batchActions, batchRunning, isSystemTaskPage, items, loadTasks, page, t],
  );

  const handleDownloadTaskArtifact = useCallback(
    async (item) => {
      const payload = parseTaskJSONField(item?.payload);
      const result = parseTaskJSONField(item?.result);
      const channelId = (
        payload?.channel_id ||
        item?.channel_id ||
        ''
      ).toString().trim();
      const modelName = (
        payload?.model ||
        result?.model ||
        item?.model ||
        ''
      ).toString().trim();
      const endpoint = (
        result?.endpoint ||
        payload?.endpoint ||
        item?.endpoint ||
        ''
      ).toString().trim();
      const fallbackArtifactName =
        (result?.artifact_name || `${modelName || 'task'}.bin`).toString();
      if (
        channelId === '' ||
        modelName === '' ||
        endpoint === '' ||
        !(result?.artifact_name || result?.artifact_path)
      ) {
        showError(t('task.messages.download_unavailable'));
        return;
      }
      try {
        const response = await API.get(
          `/api/v1/admin/channel/${channelId}/tests/artifact`,
          {
            params: {
              model: modelName,
              endpoint,
            },
            responseType: 'blob',
          },
        );
        const responseContentType = (
          response.headers?.['content-type'] || ''
        ).toString();
        if (responseContentType.includes('application/json')) {
          const text = await response.data.text();
          let parsed = null;
          try {
            parsed = JSON.parse(text);
          } catch {
            parsed = null;
          }
          showError(
            parsed?.message ||
              parsed?.error?.message ||
              t('task.messages.download_failed'),
          );
          return;
        }
        const blob = new Blob([response.data], {
          type:
            responseContentType ||
            result?.artifact_content_type ||
            'application/octet-stream',
        });
        const downloadUrl = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = downloadUrl;
        link.download = parseDownloadFilename(
          response.headers?.['content-disposition'],
          fallbackArtifactName,
        );
        document.body.appendChild(link);
        link.click();
        link.remove();
        window.URL.revokeObjectURL(downloadUrl);
      } catch (error) {
        showError(error?.message || t('task.messages.download_failed'));
      }
    },
    [t],
  );

  const detailBasePath = isSystemTaskPage
    ? '/admin/channel/tasks'
    : isAdminPage
      ? '/admin/task'
      : '/workspace/task';
  const isChannelTestHistoryContext =
    contextType === 'channel_test_history' && returnPath !== '';
  const pageTitle = isSystemTaskPage
    ? isChannelTestHistoryContext
      ? t('channel.edit.model_tester.history_tasks')
      : t('task.scopes.admin')
    : isAdminUserTaskPage
      ? t('task.scopes.user')
      : t('task.title');
  const goToChannelList = useCallback(() => {
    navigate('/admin/channel');
  }, [navigate]);
  const goBackToOrigin = useCallback(() => {
    if (returnPath !== '') {
      navigate(returnPath, {
        state: {
          channelLabel: contextLabel || returnLabel,
        },
      });
    }
  }, [contextLabel, navigate, returnLabel, returnPath]);
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

  const rootBreadcrumbs = isSystemTaskPage
    ? [
        { key: 'workspace', label: t('header.admin_workspace') },
        { key: 'channel', label: t('header.channel') },
        { key: 'task', label: pageTitle, active: true },
      ]
    : isAdminPage
      ? [
          { key: 'workspace', label: t('header.admin_workspace') },
          { key: 'task', label: pageTitle, active: true },
        ]
      : [
          { key: 'workspace', label: t('header.user_workspace') },
          { key: 'service', label: t('header.service') },
          { key: 'task', label: pageTitle, active: true },
        ];

  return (
    <div className={embedded ? '' : 'dashboard-container'}>
      {returnPath !== '' ? (
        <AppFilterHeader
          breadcrumbs={
            isChannelTestHistoryContext
              ? [
                  {
                    key: 'channel-list',
                    label: t('header.channel'),
                    onClick: (e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      goToChannelList();
                    },
                  },
                  {
                    key: 'task-origin',
                    label: contextLabel || returnLabel || '-',
                    onClick: (e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      goBackToOrigin();
                    },
                  },
                  {
                    key: 'task-current',
                    label: pageTitle,
                    active: true,
                  },
                ]
              : [
                  {
                    key: 'task-origin',
                    label: returnLabel || t('header.channel'),
                    onClick: (e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      goBackToOrigin();
                    },
                  },
                  {
                    key: 'task-current',
                    label: pageTitle,
                    active: true,
                  },
                ]
          }
        />
      ) : null}
      <AppFilterHeader
            breadcrumbs={returnPath === '' && !embedded ? rootBreadcrumbs : undefined}
            title={returnPath === '' && !embedded ? pageTitle : undefined}
            titleClassName='router-ui-section-title'
            query={
              <ListFilterBar
                availableOptions={availableConditionalFilterOptions}
                visibleFilters={visibleFilterConfig}
                getFilterConfig={getTaskFilterConfig}
                getInitialDraft={getTaskInitialDraft}
                onApplyDraft={applyTaskFilterDraft}
                onRemoveFilter={removeConditionalFilter}
                renderSummary={(key) =>
                  renderTaskFilterSummary(key, filters, t, {
                    type: resolveTypeLabel,
                    status: resolveStatusLabel,
                    model: (value) => resolveFilterOptionLabel('model', value),
                    channel_id: (value) =>
                      resolveFilterOptionLabel('channel_id', value),
                    user_keyword: (value) =>
                      resolveFilterOptionLabel('user_keyword', value),
                  })
                }
                onQuery={() => loadTasks(page)}
                queryLoading={loading}
                addButtonText={t('task.filters.add')}
                addButtonClassName='router-page-button'
                queryButtonText={t('task.buttons.query')}
                queryButtonClassName='router-page-button router-log-query-button'
                emptyChipText={t('task.filters.none')}
              />
            }
            endClassName='router-log-query-wrap'
      />

      {isSystemTaskPage && returnPath === '' && !embedded ? (
        <ChannelSectionTabs active='tasks' />
      ) : null}
      {isAdminUserTaskPage && returnPath === '' && !embedded ? (
        <UserSectionTabs active='tasks' />
      ) : null}

      <div className='router-table-scroll-x'>
        <AppTable
              className='router-list-table router-table-fit-page'
              pagination={false}
              scroll={{ x: TASK_LIST_TABLE_MIN_WIDTH }}
              rowKey={(item) => getTaskId(item)}
              dataSource={items}
              onChange={handleTableChange}
              rowSelection={
                isSystemTaskPage && isBatchSelecting
                  ? {
                      ...batchActions.tableSelection,
                      renderCell: (_, __, ___, originNode) => (
                        <span
                          onClick={(event) => event.stopPropagation()}
                        >
                          {originNode}
                        </span>
                      ),
                    }
                  : undefined
              }
              locale={{
                emptyText: loading ? (
                  t('common.loading')
                ) : loadError ? (
                  <AppErrorState
                    message={t('common.load_failed')}
                    onRetry={() => loadTasks(page)}
                    retryText={t('common.retry')}
                  />
                ) : (
                  <AppEmpty>{t('task.empty')}</AppEmpty>
                ),
              }}
              onRow={(item) => {
                const taskId = getTaskId(item);
                return {
                  className: isBatchSelecting
                    ? undefined
                    : 'router-row-clickable',
                  onClick: isBatchSelecting
                    ? undefined
                    : () =>
                        navigate(`${detailBasePath}/${taskId}`, {
                          state: {
                            from: currentPagePath,
                            fromLabel: pageTitle,
                            contextType,
                            contextLabel,
                            originPath: returnPath,
                            originLabel: contextLabel || returnLabel,
                          },
                        }),
                };
              }}
              columns={[
              {
                title: t('task.table.type'),
                dataIndex: 'type',
                key: 'type',
                className: 'router-table-col-status-compact',
                width: TASK_LIST_COLUMN_WIDTHS.type,
                render: (value) => t(`task.types.${value || 'video'}`),
              },
              ...(isAdminUserTaskPage
                ? [{
                    title: t('task.table.user'),
                    dataIndex: 'user_name',
                    key: 'user_name',
                    width: TASK_LIST_COLUMN_WIDTHS.user,
                    ellipsis: true,
                    render: (_, item) => item.user_name || item.user_id || '-',
                  }]
                : []),
              {
                title: t('task.table.channel'),
                dataIndex: 'channel_name',
                key: 'channel_name',
                width: TASK_LIST_COLUMN_WIDTHS.channel,
                ellipsis: true,
                render: (_, item) => item.channel_name || item.channel_id || '-',
              },
              {
                title: t('task.table.model'),
                dataIndex: 'model',
                key: 'model',
                width: TASK_LIST_COLUMN_WIDTHS.model,
                ellipsis: true,
                render: (value) => value || '-',
              },
              {
                title: t('task.table.status'),
                dataIndex: 'status',
                key: 'status',
                className: 'router-table-col-status-compact',
                width: TASK_LIST_COLUMN_WIDTHS.status,
                render: (value) => {
                  const rawStatus = (value || '').toString().trim().toLowerCase();
                  const status = normalizeTaskStatus(rawStatus);
                  return (
                    <AppTag color={taskStatusColor(rawStatus)} className='router-tag'>
                      {t(`task.status.${status}`)}
                    </AppTag>
                  );
                },
              },
              {
                title: t('task.table.created_at'),
                dataIndex: 'created_at',
                key: 'created_at',
                className: 'router-table-col-datetime',
                width: TASK_LIST_COLUMN_WIDTHS.createdAt,
                sorter: true,
                sortOrder: sortOrderForColumn(sort, 'created_at'),
                render: (value) => (value ? timestamp2string(value) : '-'),
              },
              {
                title: isUserTaskPage
                  ? t('task.table.updated_at')
                  : t('task.table.finished_at'),
                key: 'updated_or_finished_at',
                className: 'router-table-col-datetime',
                width: TASK_LIST_COLUMN_WIDTHS.updatedAt,
                render: (_, item) =>
                  isUserTaskPage
                    ? item.updated_at
                      ? timestamp2string(item.updated_at)
                      : '-'
                    : item.finished_at
                      ? timestamp2string(item.finished_at)
                      : '-',
              },
              {
                title: t('task.table.actions'),
                key: 'actions',
                className: isUserTaskPage
                  ? 'router-table-col-actions-icon'
                  : 'router-table-col-actions-icon',
                width: isUserTaskPage ? 52 : 120,
                render: (_, item) => {
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
                  const taskResult = parseTaskJSONField(item?.result);
                  const canDownloadArtifact =
                    isSystemTaskPage &&
                    item?.type === 'channel_model_test' &&
                    !!(taskResult?.artifact_name || taskResult?.artifact_path);
                  return isUserTaskPage ? (
                    <AppTableActionButton
                      icon='eye'
                      title={t('task.buttons.view')}
                      onClick={(e) => {
                        e.stopPropagation();
                        navigate(`${detailBasePath}/${taskId}`, {
                          state: {
                            from: currentPagePath,
                            fromLabel: pageTitle,
                            contextType,
                            contextLabel,
                            originPath: returnPath,
                            originLabel: contextLabel || returnLabel,
                          },
                        });
                      }}
                    />
                  ) : (
                    <div
                      className='router-inline-actions router-table-actions-icon-compact'
                      onClick={(e) => {
                        e.stopPropagation();
                      }}
                    >
                      <AppTableActionButton
                        icon='download'
                        title={t('common.download')}
                        disabled={!canDownloadArtifact}
                        onClick={() => {
                          handleDownloadTaskArtifact(item);
                        }}
                      />
                      <AppTableActionButton
                        icon='exchange'
                        title={t('task.buttons.retry')}
                        disabled={!canRetry}
                        onClick={() => {
                          handleRetryTask(taskId);
                        }}
                      />
                      <AppTableActionButton
                        icon='close'
                        title={t('task.buttons.cancel')}
                        disabled={!canCancel}
                        onClick={() => {
                          handleCancelTask(taskId);
                        }}
                      />
                    </div>
                  );
                },
              },
            ]}
              footer={() => (
                <AppToolbar
                  className='router-task-footer-toolbar'
                  start={
                    <span className='router-toolbar-meta'>
                      {t('task.summary', { total })}
                    </span>
                  }
                  end={
                    <div className='router-task-footer-actions'>
                      {isSystemTaskPage ? (
                        isBatchSelecting ? (
                          <>
                            <AppPopconfirm
                              title={t('task.batch.confirm_retry', {
                                count: batchSelectedCount,
                              })}
                              okText={t('common.confirm')}
                              cancelText={t('common.cancel')}
                              disabled={
                                batchSelectedCount === 0 || batchRunning
                              }
                              onConfirm={() => runBatchTaskAction('retry')}
                            >
                              <AppButton
                                className='router-page-button'
                                color='blue'
                                disabled={
                                  batchSelectedCount === 0 || batchRunning
                                }
                                loading={batchRunning}
                              >
                                {t('task.batch.retry_selected', {
                                  count: batchSelectedCount,
                                })}
                              </AppButton>
                            </AppPopconfirm>
                            <AppPopconfirm
                              title={t('task.batch.confirm_cancel', {
                                count: batchSelectedCount,
                              })}
                              okText={t('common.confirm')}
                              cancelText={t('common.cancel')}
                              disabled={
                                batchSelectedCount === 0 || batchRunning
                              }
                              onConfirm={() => runBatchTaskAction('cancel')}
                            >
                              <AppButton
                                className='router-page-button'
                                color='red'
                                disabled={
                                  batchSelectedCount === 0 || batchRunning
                                }
                                loading={batchRunning}
                              >
                                {t('task.batch.cancel_selected', {
                                  count: batchSelectedCount,
                                })}
                              </AppButton>
                            </AppPopconfirm>
                            <AppButton
                              className='router-page-button'
                              disabled={batchRunning}
                              onClick={batchActions.exit}
                            >
                              {t('task.batch.cancel_selection')}
                            </AppButton>
                          </>
                        ) : (
                          <AppButton
                            className='router-page-button'
                            disabled={batchRunning || loading}
                            onClick={batchActions.enter}
                          >
                            {t('task.batch.enter_selection')}
                          </AppButton>
                        )
                      ) : null}
                      <AppPagination
                      className='router-page-pagination'
                      activePage={page}
                      total={total}
                      pageSize={pageSize}
                      siblingRange={1}
                      boundaryRange={0}
                      onPageChange={(e, { activePage, pageSize: nextSize }) => {
                        const size = Number(nextSize) > 0 ? Number(nextSize) : pageSize;
                        if (size !== pageSize) {
                          // Page-size change reloads page 1; URL effect mirrors it.
                          setPageSize(size);
                          return;
                        }
                        const nextPage = Number(activePage || 1);
                        setPage(nextPage);
                        loadTasks(nextPage).then();
                      }}
                    />
                    </div>
                  }
                />
              )}
            />
      </div>
    </div>
  );
};

export default Task;
