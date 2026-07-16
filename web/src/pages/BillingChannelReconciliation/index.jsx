import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { API, showError } from '../../helpers';
import { formatDecimalNumber } from '../../helpers/render';
import { AppButton, AppFilterHeader, AppSection, AppSpin, AppTable, AppTag } from '../../router-ui';
import './BillingChannelReconciliation.css';

const formatCNY = (value) => `¥${formatDecimalNumber(value || 0, 4)}`;
const formatCount = (value) => formatDecimalNumber(value || 0, 0);
const formatPercent = (value) => `${(Number(value || 0) * 100).toFixed(1)}%`;

const recentRange = () => {
  const end = Math.floor(Date.now() / 1000);
  return { start_at: end - 7 * 24 * 60 * 60, end_at: end };
};

const normalizeBatch = (row) => ({
  ...row,
  capacity_total: Number(row?.capacity_total || 0),
  capacity_effective: Number(row?.capacity_effective || 0),
  capacity_remaining: Number(row?.capacity_remaining || 0),
  purchase_cost_amount: Number(row?.purchase_cost_amount || 0),
  cost_per_unit_amount: Number(row?.cost_per_unit_amount || 0),
});

function BillingChannelReconciliation() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [reportResponse, batchesResponse] = await Promise.all([
        API.get('/api/v1/admin/billing/procurement-report', {
          params: { ...recentRange(), group_by: 'channel', cost_scope: 'all' },
        }),
        API.get('/api/v1/admin/billing/procurement-batches'),
      ]);
      if (!reportResponse.data?.success) {
        showError(reportResponse.data?.message || t('billing.channel_reconciliation.load_failed'));
        return;
      }
      const items = Array.isArray(reportResponse.data?.data?.items) ? reportResponse.data.data.items : [];
      const batchesByChannel = {};
      const allBatches = Array.isArray(batchesResponse.data?.data?.items) ? batchesResponse.data.data.items.map(normalizeBatch) : [];
      allBatches.forEach((batch) => {
        const channelID = String(batch?.channel_id || '').trim();
        if (!channelID) return;
        if (!batchesByChannel[channelID]) batchesByChannel[channelID] = [];
        batchesByChannel[channelID].push(batch);
      });
      const enriched = items.map((item) => {
        const channelID = String(item?.dimension_key || '').trim();
        return { ...item, batches: !channelID || channelID === '-' ? [] : (batchesByChannel[channelID] || []) };
      });
      setRows(enriched);
    } catch (error) {
      showError(error?.message || t('billing.channel_reconciliation.load_failed'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { load().then(); }, [load]);

  const columns = [
    {
      title: t('billing.channel_reconciliation.columns.channel'),
      dataIndex: 'dimension_key',
      width: 210,
      render: (value, row) => <Link to={`/admin/channel/detail/${encodeURIComponent(value || '')}?tab=billing`}>{row.dimension_name || value || '-'}</Link>,
    },
    { title: t('billing.channel_reconciliation.columns.requests'), dataIndex: 'request_count', width: 90, align: 'right', render: formatCount },
    { title: t('billing.channel_reconciliation.columns.yyc'), dataIndex: 'router_consumed_yyc', width: 120, align: 'right', render: formatCount },
    { title: t('billing.channel_reconciliation.columns.revenue'), dataIndex: 'sell_base_amount', width: 120, align: 'right', render: formatCNY },
    { title: t('billing.channel_reconciliation.columns.cost'), dataIndex: 'procurement_cost_base_amount', width: 120, align: 'right', render: formatCNY },
    { title: t('billing.channel_reconciliation.columns.profit'), dataIndex: 'gross_profit_base_amount', width: 120, align: 'right', render: formatCNY },
    { title: t('billing.channel_reconciliation.columns.margin'), dataIndex: 'gross_margin', width: 100, align: 'right', render: formatPercent },
    {
      title: t('billing.channel_reconciliation.columns.batches'),
      key: 'batches',
      width: 260,
      render: (_, row) => {
        const batches = Array.isArray(row.batches) ? row.batches : [];
        const active = batches.filter((batch) => batch.cost_status === 'active').length;
        const incomplete = batches.filter((batch) => !['actual', 'zero_cost'].includes(batch.cost_source)).length;
        return <div className='billing-channel-reconciliation-batches'><span>{t('billing.channel_reconciliation.batch_summary', { count: batches.length, active })}</span>{incomplete > 0 ? <AppTag color='orange'>{t('billing.channel_reconciliation.incomplete', { count: incomplete })}</AppTag> : null}{row.batch_load_failed ? <AppTag color='red'>{t('billing.channel_reconciliation.load_error')}</AppTag> : null}</div>;
      },
    },
  ];

  return (
    <div className='dashboard-container billing-channel-reconciliation-page'>
      <AppFilterHeader
        breadcrumbs={[{ key: 'billing', label: t('header.billing') }, { key: 'channel-reconciliation', label: t('billing.channel_reconciliation.title'), active: true }]}
        title={t('billing.channel_reconciliation.title')}
        actions={<AppButton className='router-page-button' color='blue' loading={loading} onClick={() => load().then()}>{t('common.refresh')}</AppButton>}
      />
      <AppSpin spinning={loading}>
        <AppSection className='billing-channel-reconciliation-section'>
          <div className='billing-channel-reconciliation-note'>{t('billing.channel_reconciliation.note')}</div>
          <AppTable className='router-detail-table billing-channel-reconciliation-table' size='small' pagination={false} rowKey={(row) => row.dimension_key} dataSource={rows} columns={columns} locale={{ emptyText: t('billing.channel_reconciliation.empty') }} />
        </AppSection>
      </AppSpin>
    </div>
  );
}

export default BillingChannelReconciliation;
