import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { API } from '../helpers/api';
import {
  AppButton,
  AppDetailSection,
  AppTable,
  AppTag,
} from '../router-ui';

const CIRCUIT_BREAKER_STATE_COLORS = {
  open: 'red',
  half_open: 'orange',
  recovered: 'green',
  canceled: 'grey',
};

const normalizeEvents = (items) =>
  (Array.isArray(items) ? items : []).map((item) => ({
    ...item,
    id: Number(item?.id || 0),
    channelId: String(item?.channel_id || item?.channelId || '').trim(),
    channelName: String(item?.channel_name || item?.channelName || '').trim(),
    state: String(item?.state || item?.event || '').trim().toLowerCase(),
    reason: String(item?.reason || '').trim(),
    successRate: Number(item?.success_rate || item?.successRate || 0),
    recoverAfter: Number(item?.recover_after || item?.recoverAfter || 0),
    createdAt: Number(item?.created_at || item?.createdAt || 0),
  }));

function AdminCircuitBreakerEventsPanel({ embedded = false, limit = 10 }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [events, setEvents] = useState([]);
  const [loading, setLoading] = useState(false);

  const loadEvents = useCallback(async () => {
    setLoading(true);
    try {
      const response = await API.get('/api/v1/admin/channel/circuit-breaker/events', {
        params: { limit },
      });
      if (response?.data?.success === true) {
        setEvents(normalizeEvents(response?.data?.data?.items || []));
        return;
      }
      setEvents([]);
    } catch (error) {
      console.error('Failed to load circuit breaker events:', error);
      setEvents([]);
    } finally {
      setLoading(false);
    }
  }, [limit]);

  useEffect(() => {
    loadEvents();
  }, [loadEvents]);

  const formatTimestamp = useCallback((value) => {
    if (!value) return '-';
    return new Date(Number(value) * 1000).toLocaleString('zh-CN', {
      hour12: false,
    });
  }, []);

  const renderStateTag = useCallback(
    (state) => {
      const normalized = String(state || '').trim().toLowerCase();
      if (normalized === '') {
        return '-';
      }
      return (
        <AppTag
          color={CIRCUIT_BREAKER_STATE_COLORS[normalized] || 'grey'}
          className='router-tag'
        >
          {t(`channel.table.circuit_state_${normalized}`, {
            defaultValue: normalized,
          })}
        </AppTag>
      );
    },
    [t],
  );

  const columns = useMemo(
    () => [
      {
        title: t('dashboard.admin.channels.events.columns.time'),
        dataIndex: 'createdAt',
        key: 'createdAt',
        width: 176,
        render: (value) => formatTimestamp(value),
      },
      {
        title: t('dashboard.admin.channels.events.columns.channel'),
        dataIndex: 'channelName',
        key: 'channelName',
        render: (_, row) => {
          const label = row.channelName || row.channelId || '-';
          if (!row.channelId) {
            return label;
          }
          return (
            <button
              type='button'
              className='admin-dashboard-channel-health-name'
              onClick={() =>
                navigate(`/admin/channel/detail/${encodeURIComponent(row.channelId)}`)
              }
            >
              {label}
            </button>
          );
        },
      },
      {
        title: t('dashboard.admin.channels.events.columns.state'),
        dataIndex: 'state',
        key: 'state',
        width: 132,
        render: (value) => renderStateTag(value),
      },
      {
        title: t('dashboard.admin.channels.events.columns.reason'),
        dataIndex: 'reason',
        key: 'reason',
        render: (value) => value || '-',
      },
      {
        title: t('dashboard.admin.channels.events.columns.success_rate'),
        dataIndex: 'successRate',
        key: 'successRate',
        width: 128,
        render: (value, row) =>
          row.state === 'open' && value > 0
            ? `${(Number(value) * 100).toFixed(2)}%`
            : '-',
      },
      {
        title: t('dashboard.admin.channels.events.columns.recover_after'),
        dataIndex: 'recoverAfter',
        key: 'recoverAfter',
        width: 176,
        render: (value) => formatTimestamp(value),
      },
    ],
    [formatTimestamp, navigate, renderStateTag, t],
  );

  const headerAction = embedded ? null : (
    <AppButton
      size='small'
      onClick={() => loadEvents()}
    >
      {t('common.refresh')}
    </AppButton>
  );

  return (
    <AppDetailSection
      title={t('dashboard.admin.channels.events.title')}
      titleTag='div'
      headerEnd={headerAction}
    >
      <div className='admin-dashboard-circuit-breaker-events'>
        <div className='admin-dashboard-channel-health-hint'>
          {t('dashboard.admin.channels.events.hint')}
        </div>
        <AppTable
          className='router-detail-table'
          pagination={false}
          rowKey={(row) => row.id || `${row.channelId}-${row.createdAt}-${row.state}`}
          loading={loading}
          dataSource={events}
          locale={{
            emptyText: loading
              ? t('common.loading')
              : t('dashboard.admin.channels.events.empty'),
          }}
          columns={columns}
        />
      </div>
    </AppDetailSection>
  );
}

export default AdminCircuitBreakerEventsPanel;
