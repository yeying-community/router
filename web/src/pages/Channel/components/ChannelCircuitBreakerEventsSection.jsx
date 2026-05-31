import React from 'react';
import {
  AppAlert,
  AppDetailSection,
  AppTable,
  AppTag,
} from '../../../router-ui';

const renderState = (value, t) => {
  const state = (value || '').toString().trim();
  if (!state) {
    return '-';
  }
  const color = state === 'open' ? 'red' : state === 'half_open' ? 'orange' : 'green';
  return (
    <AppTag color={color}>
      {t(`channel.table.circuit_state_${state}`, { defaultValue: state })}
    </AppTag>
  );
};

const ChannelCircuitBreakerEventsSection = ({
  t,
  events,
  loading,
  error,
  timestamp2string,
}) => (
  <AppDetailSection
    title={t('channel.edit.circuit_breaker.title')}
    titleTag='span'
  >
    <div>
      <AppAlert
        type='info'
        showIcon
        className='router-section-message'
        title={t('channel.edit.circuit_breaker.hint')}
      />
      <AppTable
        className='router-detail-table'
        pagination={false}
        dataSource={events}
        rowKey={(row) => row.id || `${row.channel_id}-${row.event}-${row.created_at}`}
        locale={{
          emptyText: loading
            ? t('common.loading')
            : t('channel.edit.circuit_breaker.empty'),
        }}
        columns={[
          {
            title: t('channel.edit.circuit_breaker.columns.event'),
            dataIndex: 'event',
            key: 'event',
            width: 120,
            render: (value, row) => renderState(row.state || value, t),
          },
          {
            title: t('channel.edit.circuit_breaker.columns.reason'),
            dataIndex: 'reason',
            key: 'reason',
            render: (value) => value || '-',
          },
          {
            title: t('channel.edit.circuit_breaker.columns.success_rate'),
            dataIndex: 'success_rate',
            key: 'success_rate',
            width: 120,
            render: (value) =>
              Number(value || 0) > 0
                ? `${(Number(value || 0) * 100).toFixed(2)}%`
                : '-',
          },
          {
            title: t('channel.edit.circuit_breaker.columns.recover_after'),
            dataIndex: 'recover_after',
            key: 'recover_after',
            width: 168,
            render: (value) => (value ? timestamp2string(value) : '-'),
          },
          {
            title: t('channel.edit.circuit_breaker.columns.created_at'),
            dataIndex: 'created_at',
            key: 'created_at',
            width: 168,
            render: (value) => (value ? timestamp2string(value) : '-'),
          },
        ]}
      />
      {error && (
        <div className='router-error-text router-error-text-top'>
          {error}
        </div>
      )}
    </div>
  </AppDetailSection>
);

export default ChannelCircuitBreakerEventsSection;
