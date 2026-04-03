import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Breadcrumb, Button, Card, Form, Label } from 'semantic-ui-react';
import { useTranslation } from 'react-i18next';
import {
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from 'react-router-dom';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';

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
    <Label basic color={colorMap[normalized] || 'grey'} className='router-tag'>
      {t(`task.status.${normalized}`)}
    </Label>
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
    navigate(buildTaskListPath());
  }, [buildTaskListPath, navigate]);

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

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <div className='router-entity-detail-page'>
            <div className='router-entity-detail-breadcrumb'>
              <Breadcrumb size='small'>
                <Breadcrumb.Section link onClick={backToList}>
                  {t('header.task')}
                </Breadcrumb.Section>
                <Breadcrumb.Divider icon='right chevron' />
                <Breadcrumb.Section active>
                  {task?.id || id}
                </Breadcrumb.Section>
              </Breadcrumb>
            </div>

            <div className='router-detail-section'>
              <div className='router-entity-detail-section-header'>
                <div className='router-detail-section-title'>
                  {t('common.basic_info')}
                </div>
                <div className='router-toolbar-start'>
                  <Button
                    className='router-page-button'
                    onClick={loadTask}
                    loading={loading}
                  >
                    {t('task.buttons.refresh')}
                  </Button>
                  {isSystemTaskPage ? (
                    <>
                      <Button
                        className='router-page-button'
                        disabled={!canRetry}
                        onClick={handleRetry}
                      >
                        {t('task.buttons.retry')}
                      </Button>
                      <Button
                        className='router-page-button'
                        disabled={!canCancel}
                        onClick={handleCancel}
                      >
                        {t('task.buttons.cancel')}
                      </Button>
                    </>
                  ) : null}
                  <Button
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
                  </Button>
                </div>
              </div>
              <Form loading={loading}>
                <Form.Group widths='equal'>
                  <Form.Input
                    className='router-section-input'
                    label={t('task.table.type')}
                    value={task ? t(`task.types.${task.type || 'video'}`) : ''}
                    readOnly
                  />
                  <Form.Field>
                    <label>{t('task.table.status')}</label>
                    <div className='router-field-display'>
                      {task ? renderTaskStatus(task.status, t) : null}
                    </div>
                  </Form.Field>
                </Form.Group>

                <Form.Group widths='equal'>
                  {isAdminUserTaskPage ? (
                    <Form.Input
                      className='router-section-input'
                      label={t('task.table.user')}
                      value={task?.user_name || task?.user_id || '-'}
                      readOnly
                    />
                  ) : null}
                  <Form.Input
                    className='router-section-input'
                    label={t('task.table.channel')}
                    value={task?.channel_name || task?.channel_id || '-'}
                    readOnly
                  />
                  <Form.Input
                    className='router-section-input'
                    label={t('task.table.model')}
                    value={task?.model || '-'}
                    readOnly
                  />
                </Form.Group>

                <Form.Group widths='equal'>
                  <Form.Input
                    className='router-section-input'
                    label={t('task.table.created_at')}
                    value={
                      task?.created_at ? timestamp2string(task.created_at) : '-'
                    }
                    readOnly
                  />
                  <Form.Input
                    className='router-section-input'
                    label={
                      isUserTaskPage
                        ? t('task.table.updated_at')
                        : t('task.table.finished_at')
                    }
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
                </Form.Group>

                {isUserTaskPage ? (
                  <>
                    <Form.Group widths='equal'>
                      <Form.Input
                        className='router-section-input'
                        label={t('task.detail.provider')}
                        value={task?.provider || '-'}
                        readOnly
                      />
                      <Form.Input
                        className='router-section-input'
                        label={t('task.detail.request_id')}
                        value={task?.request_id || '-'}
                        readOnly
                      />
                    </Form.Group>
                    <Form.Input
                      className='router-section-input'
                      label={t('task.detail.result_url')}
                      value={task?.result_url || '-'}
                      readOnly
                    />
                    <Form.Input
                      className='router-section-input'
                      label={t('task.detail.source')}
                      value={task?.source || '-'}
                      readOnly
                    />
                  </>
                ) : (
                  <Form.Input
                    className='router-section-input'
                    label={t('task.detail.endpoint')}
                    value={task?.endpoint || '-'}
                    readOnly
                  />
                )}
              </Form>
            </div>

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
        </Card.Content>
      </Card>
    </div>
  );
};

export default TaskDetail;
