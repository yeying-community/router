import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from 'react-router-dom';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import {
  AppButton,
  AppDetailSection,
  AppField,
  AppFilterHeader,
  AppFormRow,
  AppInput,
  AppSection,
  AppTag,
} from '../../router-ui';

const TASK_DETAIL_KIND_WORKSPACE_USER = 'workspace_user';
const TASK_DETAIL_KIND_ADMIN_USER = 'admin_user';
const TASK_DETAIL_KIND_ADMIN_SYSTEM = 'admin_system';

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

const renderTaskStatus = (status, t) => {
  const normalized = normalizeTaskStatus(status);
  const colorMap = {
    pending: 'orange',
    running: 'blue',
    succeeded: 'green',
    failed: 'red',
    canceled: 'grey',
  };
  return (
    <AppTag color={colorMap[normalized] || 'grey'} className='router-tag'>
      {t(`task.status.${normalized}`)}
    </AppTag>
  );
};

const parseJSONValue = (value) => {
  if (typeof value !== 'string') {
    return { parsed: value, isJSON: value !== null && value !== undefined };
  }
  const trimmed = value.trim();
  if (!trimmed) {
    return { parsed: '', isJSON: false };
  }
  try {
    return { parsed: JSON.parse(trimmed), isJSON: true };
  } catch (error) {
    return { parsed: value, isJSON: false };
  }
};

const formatDetailValue = (value) => {
  if (value === null || value === undefined || value === '') {
    return '-';
  }
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }
  if (typeof value === 'object') {
    return JSON.stringify(value, null, 2);
  }
  return String(value);
};

const renderDetailFields = (data, fields) => {
  const rows = fields
    .map(({ key, label, formatter }) => {
      const rawValue = data?.[key];
      if (rawValue === undefined || rawValue === null || rawValue === '') {
        return null;
      }
      const value = formatter
        ? formatter(rawValue, data)
        : formatDetailValue(rawValue);
      return { key, label, value };
    })
    .filter(Boolean);
  if (rows.length === 0) {
    return null;
  }
  return (
    <div className='router-detail-grid'>
      {rows.map((item) => (
        <div key={item.key} className='router-detail-item'>
          <div className='router-detail-label'>{item.label}</div>
          <pre className='router-detail-value'>{item.value}</pre>
        </div>
      ))}
    </div>
  );
};

const renderStructuredContent = (title, value, fields) => {
  const { parsed, isJSON } = parseJSONValue(value);
  const hasObjectContent =
    isJSON && parsed && typeof parsed === 'object' && !Array.isArray(parsed);
  return (
    <div className='router-detail-section'>
      <div className='router-detail-section-title'>{title}</div>
      {hasObjectContent ? renderDetailFields(parsed, fields) : null}
      <pre className='router-detail-pre'>
        {value && value.toString().trim()
          ? isJSON
            ? JSON.stringify(parsed, null, 2)
            : value
          : '-'}
      </pre>
    </div>
  );
};

const resolveTaskDetailKind = (pathname = '') => {
  const normalizedPath = (pathname || '').toString().trim().toLowerCase();
  if (normalizedPath.startsWith('/admin/channel/tasks/')) {
    return TASK_DETAIL_KIND_ADMIN_SYSTEM;
  }
  if (normalizedPath.startsWith('/admin/task/')) {
    return TASK_DETAIL_KIND_ADMIN_USER;
  }
  return TASK_DETAIL_KIND_WORKSPACE_USER;
};

const TaskDetail = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const { id } = useParams();
  const detailKind = useMemo(
    () => resolveTaskDetailKind(location.pathname),
    [location.pathname],
  );
  const isAdminPage = detailKind !== TASK_DETAIL_KIND_WORKSPACE_USER;
  const isSystemTaskPage = detailKind === TASK_DETAIL_KIND_ADMIN_SYSTEM;
  const isAdminUserTaskPage = detailKind === TASK_DETAIL_KIND_ADMIN_USER;
  const isUserTaskPage = !isSystemTaskPage;
  const currentPagePath = `${location.pathname}${location.search}${location.hash}`;
  const returnPath = useMemo(() => {
    const raw = location.state?.from;
    if (typeof raw !== 'string') {
      return '';
    }
    const normalized = raw.trim();
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
  const originPath = useMemo(() => {
    const raw = location.state?.originPath;
    if (typeof raw !== 'string') {
      return '';
    }
    const normalized = raw.trim();
    return normalized.startsWith('/') ? normalized : '';
  }, [location.state]);
  const originLabel = useMemo(() => {
    const raw = location.state?.originLabel;
    if (typeof raw !== 'string') {
      return '';
    }
    return raw.trim();
  }, [location.state]);
  const taskListNavState = useMemo(() => {
    if (
      originPath === '' &&
      originLabel === '' &&
      contextType === '' &&
      contextLabel === ''
    ) {
      return undefined;
    }
    return {
      from: originPath,
      fromLabel: originLabel,
      contextType,
      contextLabel,
    };
  }, [contextLabel, contextType, originLabel, originPath]);
  const [loading, setLoading] = useState(true);
  const [task, setTask] = useState(null);

  const buildTaskListPath = useCallback(
    (extraParams = {}) => {
      const nextSearchParams = new URLSearchParams(searchParams.toString());
      Object.entries(extraParams || {}).forEach(([key, value]) => {
        const normalizedValue = (value || '').toString().trim();
        if (normalizedValue === '') {
          nextSearchParams.delete(key);
          return;
        }
        nextSearchParams.set(key, normalizedValue);
      });
      const search = nextSearchParams.toString();
      const basePath = isSystemTaskPage
        ? '/admin/channel/tasks'
        : isAdminPage
          ? '/admin/task'
          : '/workspace/task';
      return `${basePath}${search ? `?${search}` : ''}`;
    },
    [isAdminPage, isSystemTaskPage, searchParams],
  );

  const payloadFields = useMemo(
    () => [
      { key: 'channel_id', label: 'channel_id' },
      { key: 'model', label: 'model' },
      { key: 'endpoint', label: 'endpoint' },
      { key: 'type', label: 'type' },
      { key: 'refresh_type', label: 'refresh_type' },
      { key: 'round', label: 'round' },
    ],
    [],
  );

  const resultFields = useMemo(
    () => [
      { key: 'status', label: 'status' },
      { key: 'supported', label: 'supported' },
      { key: 'latency_ms', label: 'latency_ms' },
      { key: 'message', label: 'message' },
      { key: 'endpoint', label: 'endpoint' },
      { key: 'model', label: 'model' },
      { key: 'base_url', label: 'base_url' },
      { key: 'request_url', label: 'request_url' },
      { key: 'api_base_url', label: 'api_base_url' },
      { key: 'account_base_url', label: 'account_base_url' },
      { key: 'models_url', label: 'models_url' },
      { key: 'balance_urls', label: 'balance_urls' },
      { key: 'count', label: 'count' },
      { key: 'balance', label: 'balance' },
      { key: 'round', label: 'round' },
    ],
    [],
  );

  const errorFields = useMemo(
    () => [
      { key: 'message', label: 'message' },
      { key: 'error', label: 'error' },
      { key: 'detail', label: 'detail' },
    ],
    [],
  );

  const backToList = useCallback(() => {
    navigate(returnPath || buildTaskListPath(), {
      state: taskListNavState,
    });
  }, [buildTaskListPath, navigate, returnPath, taskListNavState]);
  const goToChannelList = useCallback(() => {
    navigate('/admin/channel');
  }, [navigate]);
  const goBackToOrigin = useCallback(() => {
    if (originPath !== '') {
      navigate(originPath, {
        state: {
          channelLabel: originLabel || contextLabel,
        },
      });
    }
  }, [contextLabel, navigate, originLabel, originPath]);

  const loadTask = useCallback(async () => {
    setLoading(true);
    try {
      const endpoint = isSystemTaskPage
        ? `/api/v1/admin/tasks/${id}`
        : isAdminPage
          ? `/api/v1/admin/user/tasks/${id}`
          : `/api/v1/public/user/tasks/${id}`;
      const res = await API.get(endpoint);
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('task.messages.load_failed'));
        return;
      }
      setTask(data || null);
    } catch (error) {
      showError(error?.message || t('task.messages.load_failed'));
    } finally {
      setLoading(false);
    }
  }, [id, isAdminPage, isSystemTaskPage, t]);

  useEffect(() => {
    loadTask().then();
  }, [loadTask]);

  const handleRetry = useCallback(async () => {
    try {
      const res = await API.post(`/api/v1/admin/tasks/${id}/retry`);
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('task.messages.retry_failed'));
        return;
      }
      showSuccess(t('task.messages.retry_success'));
      navigate(buildTaskListPath({ refresh_at: String(Date.now()) }));
    } catch (error) {
      showError(error?.message || t('task.messages.retry_failed'));
    }
  }, [buildTaskListPath, id, navigate, t]);

  const handleCancel = useCallback(async () => {
    try {
      const res = await API.post(`/api/v1/admin/tasks/${id}/cancel`);
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('task.messages.cancel_failed'));
        return;
      }
      showSuccess(t('task.messages.cancel_success'));
      navigate(buildTaskListPath({ refresh_at: String(Date.now()) }));
    } catch (error) {
      showError(error?.message || t('task.messages.cancel_failed'));
    }
  }, [buildTaskListPath, id, navigate, t]);

  const canRetry =
    isSystemTaskPage &&
    ['failed', 'canceled'].includes(normalizeTaskStatus(task?.status));
  const canCancel =
    isSystemTaskPage &&
    ['pending', 'running'].includes(normalizeTaskStatus(task?.status));
  const channelDetailPath =
    isAdminPage && task?.channel_id
      ? `/admin/channel/detail/${task.channel_id}`
      : '';
  const isChannelTestHistoryContext =
    contextType === 'channel_test_history' && originPath !== '';
  const breadcrumbItems = isChannelTestHistoryContext
    ? [
        {
          key: 'workspace',
          label: isAdminPage ? t('header.admin_workspace') : t('header.user_workspace'),
        },
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
          key: 'channel-origin',
          label: originLabel || contextLabel || '-',
          onClick: (e) => {
            e.preventDefault();
            e.stopPropagation();
            goBackToOrigin();
          },
        },
        {
          key: 'task-list',
          label: returnLabel || t('channel.edit.model_tester.history_tasks'),
          onClick: backToList,
        },
        {
          key: 'task-current',
          label: task?.id || id,
          active: true,
        },
      ]
    : [
        {
          key: 'workspace',
          label: isAdminPage ? t('header.admin_workspace') : t('header.user_workspace'),
        },
        {
          key: 'task-list',
          label: returnLabel || t('header.task'),
          onClick: backToList,
        },
        {
          key: 'task-current',
          label: task?.id || id,
          active: true,
        },
      ];

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={breadcrumbItems}
        title={returnLabel || t('header.task')}
      />
      <AppSection>
        <div className='router-entity-detail-page'>
            <AppDetailSection
              className='router-detail-section'
              title={t('common.basic_info')}
              titleTag='div'
              titleClassName='router-detail-section-title'
              headerEnd={
                <>
                  <AppButton
                    className='router-page-button'
                    onClick={loadTask}
                    loading={loading}
                  >
                    {t('task.buttons.refresh')}
                  </AppButton>
                  {isSystemTaskPage ? (
                    <>
                      <AppButton
                        className='router-page-button'
                        disabled={!canRetry}
                        onClick={handleRetry}
                      >
                        {t('task.buttons.retry')}
                      </AppButton>
                      <AppButton
                        className='router-page-button'
                        disabled={!canCancel}
                        onClick={handleCancel}
                      >
                        {t('task.buttons.cancel')}
                      </AppButton>
                    </>
                  ) : null}
                  <AppButton
                    className='router-page-button'
                    disabled={!channelDetailPath}
                    onClick={() =>
                      navigate(channelDetailPath, {
                        state: {
                          from: currentPagePath,
                        },
                      })
                    }
                  >
                    {t('task.detail.buttons.channel')}
                  </AppButton>
                </>
              }
              bodyClassName='router-page-stack'
            >
                <AppFormRow>
                  <AppField label={t('task.table.type')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={task ? t(`task.types.${task.type || 'video'}`) : ''}
                      readOnly
                    />
                  </AppField>
                  <AppField label={t('task.table.status')} readOnly>
                    <div className='router-field-display'>
                      {task ? renderTaskStatus(task.status, t) : null}
                    </div>
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  {isAdminUserTaskPage ? (
                    <AppField label={t('task.table.user')} readOnly>
                      <AppInput
                        className='router-section-input'
                        value={task?.user_name || task?.user_id || '-'}
                        readOnly
                      />
                    </AppField>
                  ) : null}
                  <AppField label={t('task.table.channel')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={task?.channel_name || task?.channel_id || '-'}
                      readOnly
                    />
                  </AppField>
                  <AppField label={t('task.table.model')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={task?.model || '-'}
                      readOnly
                    />
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  <AppField label={t('task.table.created_at')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={
                        task?.created_at ? timestamp2string(task.created_at) : '-'
                      }
                      readOnly
                    />
                  </AppField>
                  <AppField
                    label={
                      isUserTaskPage
                        ? t('task.table.updated_at')
                        : t('task.table.finished_at')
                    }
                    readOnly
                  >
                    <AppInput
                      className='router-section-input'
                      value={
                        isUserTaskPage
                          ? task?.updated_at
                            ? timestamp2string(task.updated_at)
                            : '-'
                          : task?.finished_at
                            ? timestamp2string(task.finished_at)
                            : '-'
                      }
                      readOnly
                    />
                  </AppField>
                </AppFormRow>

                {isUserTaskPage ? (
                  <>
                    <AppFormRow>
                      <AppField label={t('task.detail.provider')} readOnly>
                        <AppInput
                          className='router-section-input'
                          value={task?.provider || '-'}
                          readOnly
                        />
                      </AppField>
                      <AppField label={t('task.detail.request_id')} readOnly>
                        <AppInput
                          className='router-section-input'
                          value={task?.request_id || '-'}
                          readOnly
                        />
                      </AppField>
                    </AppFormRow>
                    <AppFormRow>
                      <AppField label={t('task.detail.result_url')} readOnly>
                        <AppInput
                          className='router-section-input'
                          value={task?.result_url || '-'}
                          readOnly
                        />
                      </AppField>
                    </AppFormRow>
                    <AppFormRow>
                      <AppField label={t('task.detail.source')} readOnly>
                        <AppInput
                          className='router-section-input'
                          value={task?.source || '-'}
                          readOnly
                        />
                      </AppField>
                    </AppFormRow>
                  </>
                ) : (
                  <AppFormRow>
                    <AppField label={t('task.detail.endpoint')} readOnly>
                      <AppInput
                        className='router-section-input'
                        value={task?.endpoint || '-'}
                        readOnly
                      />
                    </AppField>
                  </AppFormRow>
                )}
              </AppDetailSection>

            {isSystemTaskPage
              ? (
                <>
                  {renderStructuredContent(
                    t('task.detail.payload'),
                    task?.payload || '',
                    payloadFields,
                  )}
                  {renderStructuredContent(
                    t('task.detail.result'),
                    task?.result || '',
                    resultFields,
                  )}
                  {renderStructuredContent(
                    t('task.detail.error'),
                    task?.error_message || '',
                    errorFields,
                  )}
                </>
              )
              : null}
        </div>
      </AppSection>
    </div>
  );
};

export default TaskDetail;
