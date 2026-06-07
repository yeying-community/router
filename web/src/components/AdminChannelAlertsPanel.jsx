import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { API } from '../helpers/api';
import {
  AppButton,
  AppInput,
  AppModal,
  AppPagination,
  AppSection,
  AppSegmented,
  AppSelect,
  AppTag,
  AppTable,
  AppTextarea,
} from '../router-ui';

const ALERT_LEVEL_COLORS = {
  critical: 'red',
  warning: 'orange',
  info: 'blue',
};

const normalizeAlertLevel = (value) => {
  const normalized = String(value || '').trim().toLowerCase();
  if (normalized === 'critical' || normalized === 'error') return 'critical';
  if (normalized === 'warning' || normalized === 'warn') return 'warning';
  return 'info';
};

const normalizeAlertItems = (items) =>
  (Array.isArray(items) ? items : []).map((item) => ({
    ...item,
    channelId: item?.channel_id || '',
    channelName: item?.channel_name || '',
    createdAt: Number(item?.created_at || 0),
    acknowledgedAt: Number(item?.acknowledged_at || 0),
    acknowledgedBy: item?.acknowledged_by || '',
    resolvedAt: Number(item?.resolved_at || 0),
    resolvedBy: item?.resolved_by || '',
    operatorNote: item?.operator_note || '',
  }));

const formatActorTimestampLabel = (timestamp) =>
  timestamp > 0
    ? new Date(timestamp * 1000).toLocaleString('zh-CN', { hour12: false })
    : '-';

function AdminChannelAlertsPanel({ embedded = false }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [alertItems, setAlertItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [acknowledgingAlertID, setAcknowledgingAlertID] = useState('');
  const [resolvingAlertID, setResolvingAlertID] = useState('');
  const [noteModal, setNoteModal] = useState({
    open: false,
    action: '',
    alert: null,
    note: '',
  });
  const [statusFilter, setStatusFilter] = useState(embedded ? 'active' : 'all');
  const [typeFilter, setTypeFilter] = useState('all');
  const [levelFilter, setLevelFilter] = useState('all');
  const [timeFilter, setTimeFilter] = useState('all');
  const [keywordInput, setKeywordInput] = useState('');
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);
  const pageSize = embedded ? 20 : 20;

  const loadAlertItems = useCallback(async () => {
    setLoading(true);
    try {
      const response = await API.get('/api/v1/admin/channel/alerts', {
        params: {
          limit: embedded ? 20 : Math.max(page * pageSize, 100),
          page: embedded ? undefined : page,
          page_size: embedded ? undefined : pageSize,
          status: statusFilter,
          type: embedded || typeFilter === 'all' ? undefined : typeFilter,
          level: embedded || levelFilter === 'all' ? undefined : levelFilter,
          keyword: embedded || keyword === '' ? undefined : keyword,
          time: embedded || timeFilter === 'all' ? undefined : timeFilter,
        },
      });
      const nextItems =
        response?.data?.success === true
          ? normalizeAlertItems(response?.data?.data?.items || [])
          : [];
      setAlertItems(nextItems);
      setTotal(Number(response?.data?.data?.total || 0));
    } catch (error) {
      console.error('Failed to load channel alerts:', error);
      setAlertItems([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [embedded, keyword, levelFilter, page, pageSize, statusFilter, timeFilter, typeFilter]);

  useEffect(() => {
    loadAlertItems();
  }, [loadAlertItems]);

  useEffect(() => {
    if (embedded) {
      setStatusFilter('active');
      setTypeFilter('all');
      setLevelFilter('all');
      setTimeFilter('all');
      setKeywordInput('');
      setKeyword('');
      setPage(1);
    }
  }, [embedded]);

  useEffect(() => {
    if (!embedded) {
      setPage(1);
    }
  }, [embedded, keyword, levelFilter, statusFilter, timeFilter, typeFilter]);

  const openNoteModal = useCallback((action, alert) => {
    setNoteModal({
      open: true,
      action,
      alert,
      note: '',
    });
  }, []);

  const closeNoteModal = useCallback(() => {
    if (acknowledgingAlertID || resolvingAlertID) {
      return;
    }
    setNoteModal({
      open: false,
      action: '',
      alert: null,
      note: '',
    });
  }, [acknowledgingAlertID, resolvingAlertID]);

  const submitNoteAction = useCallback(async () => {
    const action = String(noteModal?.action || '').trim();
    const alert = noteModal?.alert;
    const note = String(noteModal?.note || '').trim();
    if (!alert || (action !== 'acknowledge' && action !== 'resolve')) {
      return;
    }
    if (action === 'acknowledge') {
      const alertID = String(alert?.id || '').trim();
      const alertType = String(alert?.type || '').trim();
      const channelID = String(alert?.channel_id || alert?.channelId || '').trim();
      if (alertID === '' || alertType === '' || channelID === '') {
        return;
      }
      setAcknowledgingAlertID(alertID);
      try {
        const response = await API.post('/api/v1/admin/channel/alerts/acknowledge', {
          alert_type: alertType,
          alert_key: alertID,
          channel_id: channelID,
          note,
        });
        if (response?.data?.success === true) {
          setAlertItems((current) =>
            current.map((item) =>
              item.id === alertID
                ? {
                    ...item,
                    status: 'acknowledged',
                    acknowledged_at: Number(response?.data?.data?.acknowledged_at || 0),
                    acknowledged_by: String(response?.data?.data?.acknowledged_by || ''),
                    acknowledgedAt: Number(response?.data?.data?.acknowledged_at || 0),
                    acknowledgedBy: String(response?.data?.data?.acknowledged_by || ''),
                    operatorNote: String(response?.data?.data?.last_operator_note || note),
                  }
                : item,
            ),
          );
          setNoteModal({ open: false, action: '', alert: null, note: '' });
          loadAlertItems();
        }
      } catch (error) {
        console.error('Failed to acknowledge channel alert:', error);
      } finally {
        setAcknowledgingAlertID('');
      }
      return;
    }
    const alertID = String(alert?.id || '').trim();
    const alertType = String(alert?.type || '').trim();
    const channelID = String(alert?.channel_id || alert?.channelId || '').trim();
    if (alertID === '' || alertType === '' || channelID === '') {
      return;
    }
    setResolvingAlertID(alertID);
    try {
      const response = await API.post('/api/v1/admin/channel/alerts/resolve', {
        alert_type: alertType,
        alert_key: alertID,
        channel_id: channelID,
        note,
      });
      if (response?.data?.success === true) {
        setAlertItems((current) => current.filter((item) => item.id !== alertID));
        setNoteModal({ open: false, action: '', alert: null, note: '' });
        loadAlertItems();
      }
    } catch (error) {
      console.error('Failed to resolve channel alert:', error);
    } finally {
      setResolvingAlertID('');
    }
  }, [loadAlertItems, noteModal]);

  const formatUpdatedAt = useCallback((value) => {
    if (!value) return '-';
    return new Date(Number(value) * 1000).toLocaleString('zh-CN', {
      hour12: false,
    });
  }, []);

  const renderAlertLevelTag = useCallback(
    (level) => {
      const normalized = normalizeAlertLevel(level);
      return (
        <AppTag color={ALERT_LEVEL_COLORS[normalized] || 'grey'} className='router-tag'>
          {t(`dashboard.admin.alerts.level.${normalized}`)}
        </AppTag>
      );
    },
    [t],
  );

  const renderAlertMeta = useCallback(
    (record) => {
      if (String(record?.status || '').trim() === 'resolved') {
        return (
          <div className='admin-dashboard-alert-meta'>
            {t('dashboard.admin.alerts.meta.resolved', {
              actor: String(record?.resolvedBy || record?.resolved_by || '').trim() || '-',
              time: formatActorTimestampLabel(
                Number(record?.resolvedAt || record?.resolved_at || 0),
              ),
            })}
          </div>
        );
      }
      if (String(record?.status || '').trim() === 'acknowledged') {
        return (
          <div className='admin-dashboard-alert-meta'>
            {t('dashboard.admin.alerts.meta.acknowledged', {
              actor: String(record?.acknowledgedBy || record?.acknowledged_by || '').trim() || '-',
              time: formatActorTimestampLabel(
                Number(record?.acknowledgedAt || record?.acknowledged_at || 0),
              ),
            })}
          </div>
        );
      }
      return null;
    },
    [t],
  );

  const displayAlertItems = alertItems;

  const totalPages = useMemo(() => {
    if (embedded) return 1;
    const normalizedTotal = Number(total || 0);
    return Math.max(1, Math.ceil(normalizedTotal / pageSize));
  }, [embedded, pageSize, total]);

  useEffect(() => {
    if (embedded) {
      return;
    }
    if (page > totalPages) {
      setPage(totalPages);
    }
  }, [embedded, page, totalPages]);

  const statusOptions = useMemo(
    () => [
      {
        value: 'all',
        label: t('dashboard.admin.alerts.filters.status.all'),
      },
      {
        value: 'active',
        label: t('dashboard.admin.alerts.filters.status.active'),
      },
      {
        value: 'unacknowledged',
        label: t('dashboard.admin.alerts.filters.status.unacknowledged'),
      },
      {
        value: 'acknowledged',
        label: t('dashboard.admin.alerts.filters.status.acknowledged'),
      },
      {
        value: 'resolved',
        label: t('dashboard.admin.alerts.filters.status.resolved'),
      },
    ],
    [t],
  );

  const typeOptions = useMemo(
    () => [
      { key: 'all', value: 'all', text: t('dashboard.admin.alerts.filters.type.all') },
      { key: 'billing', value: 'billing', text: t('dashboard.admin.alerts.type_labels.billing') },
      { key: 'circuit', value: 'circuit', text: t('dashboard.admin.alerts.type_labels.circuit') },
      { key: 'model_disabled', value: 'model_disabled', text: t('dashboard.admin.alerts.type_labels.model_disabled') },
      { key: 'endpoint_disabled', value: 'endpoint_disabled', text: t('dashboard.admin.alerts.type_labels.endpoint_disabled') },
    ],
    [t],
  );

  const levelOptions = useMemo(
    () => [
      { key: 'all', value: 'all', text: t('dashboard.admin.alerts.filters.level.all') },
      { key: 'critical', value: 'critical', text: t('dashboard.admin.alerts.level.critical') },
      { key: 'warning', value: 'warning', text: t('dashboard.admin.alerts.level.warning') },
      { key: 'info', value: 'info', text: t('dashboard.admin.alerts.level.info') },
    ],
    [t],
  );

  const timeOptions = useMemo(
    () => [
      { key: 'all', value: 'all', text: t('dashboard.admin.alerts.filters.time.all') },
      { key: '24h', value: '24h', text: t('dashboard.admin.alerts.filters.time.last_24h') },
      { key: '7d', value: '7d', text: t('dashboard.admin.alerts.filters.time.last_7d') },
      { key: '30d', value: '30d', text: t('dashboard.admin.alerts.filters.time.last_30d') },
    ],
    [t],
  );

  const alertColumns = useMemo(
    () => [
      {
        title: t('dashboard.admin.alerts.columns.level'),
        dataIndex: 'level',
        key: 'level',
        width: 116,
        render: (_, record) => renderAlertLevelTag(record.level),
      },
      {
        title: t('dashboard.admin.alerts.columns.event'),
        dataIndex: 'title',
        key: 'title',
        width: 180,
        render: (_, record) => (
          <div className='admin-dashboard-alert-title-cell'>
            <div className='admin-dashboard-alert-title'>{record.title || '-'}</div>
            <div className='admin-dashboard-alert-type'>
              {t(`dashboard.admin.alerts.type_labels.${record.type}`, {
                defaultValue: record.type || '-',
              })}
            </div>
          </div>
        ),
      },
      {
        title: t('dashboard.admin.alerts.columns.channel'),
        dataIndex: 'channelName',
        key: 'channelName',
        width: 180,
        ellipsis: true,
        render: (_, record) =>
          record.channelId ? (
            <button
              type='button'
              className='admin-dashboard-user-link'
              title={record.channelName || record.channelId || '-'}
              onClick={() =>
                navigate(`/admin/channel/detail/${encodeURIComponent(record.channelId)}`)
              }
            >
              {record.channelName || record.channelId || '-'}
            </button>
          ) : (
            <span>{record.channelName || '-'}</span>
          ),
      },
      {
        title: t('dashboard.admin.alerts.columns.summary'),
        dataIndex: 'summary',
        key: 'summary',
        ellipsis: true,
        render: (_, record) => (
          <div className='admin-dashboard-alert-summary-cell'>
            <div className='admin-dashboard-alert-summary'>{record.summary || '-'}</div>
            <div className='admin-dashboard-alert-detail'>{record.detail || '-'}</div>
            <div className='admin-dashboard-alert-state'>
              {record.status === 'acknowledged'
                ? t('dashboard.admin.alerts.status.acknowledged')
                : record.status === 'resolved'
                  ? t('dashboard.admin.alerts.status.resolved')
                  : t('dashboard.admin.alerts.status.active')}
            </div>
            {record.operatorNote || record.operator_note ? (
              <div className='admin-dashboard-alert-note'>
                {t('dashboard.admin.alerts.note_prefix')}
                {record.operatorNote || record.operator_note}
              </div>
            ) : null}
            {renderAlertMeta(record)}
          </div>
        ),
      },
      {
        title: t('dashboard.admin.alerts.columns.time'),
        dataIndex: 'createdAt',
        key: 'createdAt',
        width: 180,
        render: (value) => formatUpdatedAt(value),
      },
      {
        title: t('dashboard.admin.alerts.columns.actions'),
        key: 'actions',
        width: 280,
        render: (_, record) => (
          <div className='admin-dashboard-alert-actions'>
            <AppButton
              type='button'
              className='router-inline-button'
              onClick={() =>
                navigate(`/admin/channel/detail/${encodeURIComponent(record.channelId)}`)
              }
            >
              {t('dashboard.admin.alerts.actions.view_channel')}
            </AppButton>
            <AppButton
              color='blue'
              type='button'
              loading={acknowledgingAlertID === record.id}
              disabled={record.status === 'acknowledged' || resolvingAlertID === record.id}
              onClick={() => openNoteModal('acknowledge', record)}
            >
              {record.status === 'acknowledged'
                ? t('dashboard.admin.alerts.actions.acknowledged')
                : t('dashboard.admin.alerts.actions.acknowledge')}
            </AppButton>
            <AppButton
              type='button'
              className='router-inline-button'
              loading={resolvingAlertID === record.id}
              disabled={record.status !== 'acknowledged' || acknowledgingAlertID === record.id}
              onClick={() => openNoteModal('resolve', record)}
            >
              {t('dashboard.admin.alerts.actions.resolve')}
            </AppButton>
          </div>
        ),
      },
    ],
    [
      acknowledgingAlertID,
      formatUpdatedAt,
      navigate,
      openNoteModal,
      renderAlertLevelTag,
      renderAlertMeta,
      resolvingAlertID,
      t,
    ],
  );

  const content = (
    <div className={embedded ? 'admin-dashboard-alerts-block' : ''}>
      <div className='admin-dashboard-subsection-header admin-dashboard-alerts-header'>
        <div className='admin-dashboard-subsection-header-main'>
          <div className='admin-dashboard-subsection-title admin-dashboard-subsection-title-strong'>
            {t('dashboard.admin.alerts.title')}
          </div>
          <div className='admin-dashboard-subsection-description'>
            {t('dashboard.admin.alerts.description')}
          </div>
        </div>
        <div className='admin-dashboard-alerts-count'>
          {embedded
            ? t('dashboard.admin.alerts.active_count', { count: displayAlertItems.length })
            : t('dashboard.admin.alerts.page_count', {
                page_count: displayAlertItems.length,
                total_count: total,
              })}
        </div>
      </div>
      {!embedded ? (
        <div className='admin-dashboard-alert-filters'>
          <AppSegmented
            className='admin-dashboard-segmented'
            options={statusOptions}
            value={statusFilter}
            onChange={(_, event) => {
              setStatusFilter(event?.value || 'all');
            }}
          />
          <div className='admin-dashboard-alert-filter-selects'>
            <div className='router-list-toolbar-query router-list-toolbar-query-compact'>
              <AppInput
                className='admin-dashboard-alert-search'
                value={keywordInput}
                placeholder={t('dashboard.admin.alerts.filters.search.placeholder')}
                onChange={(e, { value }) => setKeywordInput(value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    setKeyword(String(keywordInput || '').trim());
                  }
                }}
              />
              <AppButton color='blue' type='button' onClick={() => setKeyword(String(keywordInput || '').trim())}>
                {t('dashboard.admin.alerts.filters.search.submit')}
              </AppButton>
              {keyword ? (
                <AppButton
                  type='button'
                  className='router-inline-button'
                  onClick={() => {
                    setKeywordInput('');
                    setKeyword('');
                  }}
                >
                  {t('dashboard.admin.alerts.filters.search.reset')}
                </AppButton>
              ) : null}
            </div>
            <AppSelect
              className='router-section-dropdown'
              options={timeOptions}
              value={timeFilter}
              onChange={(e, { value }) => setTimeFilter(value)}
            />
            <AppSelect
              className='router-section-dropdown'
              options={typeOptions}
              value={typeFilter}
              onChange={(e, { value }) => setTypeFilter(value)}
            />
            <AppSelect
              className='router-section-dropdown'
              options={levelOptions}
              value={levelFilter}
              onChange={(e, { value }) => setLevelFilter(value)}
            />
          </div>
        </div>
      ) : null}
      {loading ? (
        <div className='admin-dashboard-empty'>{t('common.loading')}</div>
      ) : displayAlertItems.length === 0 ? (
        <div className='admin-dashboard-empty'>
          {t('dashboard.admin.alerts.empty')}
        </div>
      ) : (
        <AppTable
          className='admin-dashboard-alert-table'
          columns={alertColumns}
          dataSource={displayAlertItems}
          pagination={false}
          rowKey='id'
          scroll={{ x: 1160 }}
        />
      )}
      {!embedded && totalPages > 1 ? (
        <div className='router-pagination-wrap'>
          <AppPagination
            className='router-page-pagination'
            activePage={page}
            totalPages={totalPages}
            siblingRange={1}
            boundaryRange={0}
            onPageChange={(e, { activePage }) => {
              setPage(Number(activePage || 1));
            }}
          />
        </div>
      ) : null}
    </div>
  );

  if (embedded) {
    return (
      <>
        {content}
        <AppModal
          open={noteModal.open}
          onClose={closeNoteModal}
          size='small'
          title={
            noteModal.action === 'resolve'
              ? t('dashboard.admin.alerts.dialog.resolve_title')
              : t('dashboard.admin.alerts.dialog.acknowledge_title')
          }
          footer={null}
        >
          <div className='router-page-stack'>
            <div className='admin-dashboard-alert-dialog-hint'>
              {noteModal?.alert?.title || '-'}
            </div>
            <AppTextarea
              className='router-section-input'
              rows={4}
              value={noteModal.note}
              placeholder={t('dashboard.admin.alerts.dialog.note_placeholder')}
              onChange={(e) =>
                setNoteModal((current) => ({
                  ...current,
                  note: e?.target?.value || '',
                }))
              }
            />
            <div className='admin-dashboard-alert-dialog-actions'>
              <AppButton type='button' onClick={closeNoteModal}>
                {t('common.cancel')}
              </AppButton>
              <AppButton
                color='blue'
                type='button'
                loading={
                  (noteModal.action === 'acknowledge' && acknowledgingAlertID !== '') ||
                  (noteModal.action === 'resolve' && resolvingAlertID !== '')
                }
                onClick={submitNoteAction}
              >
                {t('common.confirm')}
              </AppButton>
            </div>
          </div>
        </AppModal>
      </>
    );
  }

  return (
    <>
      <AppSection className='admin-dashboard-section'>{content}</AppSection>
      <AppModal
        open={noteModal.open}
        onClose={closeNoteModal}
        size='small'
        title={
          noteModal.action === 'resolve'
            ? t('dashboard.admin.alerts.dialog.resolve_title')
            : t('dashboard.admin.alerts.dialog.acknowledge_title')
        }
        footer={null}
      >
        <div className='router-page-stack'>
          <div className='admin-dashboard-alert-dialog-hint'>
            {noteModal?.alert?.title || '-'}
          </div>
          <AppTextarea
            className='router-section-input'
            rows={4}
            value={noteModal.note}
            placeholder={t('dashboard.admin.alerts.dialog.note_placeholder')}
            onChange={(e) =>
              setNoteModal((current) => ({
                ...current,
                note: e?.target?.value || '',
              }))
            }
          />
          <div className='admin-dashboard-alert-dialog-actions'>
            <AppButton type='button' onClick={closeNoteModal}>
              {t('common.cancel')}
            </AppButton>
            <AppButton
              color='blue'
              type='button'
              loading={
                (noteModal.action === 'acknowledge' && acknowledgingAlertID !== '') ||
                (noteModal.action === 'resolve' && resolvingAlertID !== '')
              }
              onClick={submitNoteAction}
            >
              {t('common.confirm')}
            </AppButton>
          </div>
        </div>
      </AppModal>
    </>
  );
}

export default AdminChannelAlertsPanel;
