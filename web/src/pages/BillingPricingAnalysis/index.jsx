import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { API, showError } from '../../helpers';
import { formatDecimalNumber } from '../../helpers/render';
import {
  AppButton,
  AppFilterHeader,
  AppSection,
  AppSpin,
  AppTable,
  AppTag,
} from '../../router-ui';
import './BillingPricingAnalysis.css';

const formatCNY = (value) => `¥${formatDecimalNumber(value || 0, 4)}`;
const formatCount = (value) => formatDecimalNumber(value || 0, 0);
const formatPercent = (value) => `${(Number(value || 0) * 100).toFixed(1)}%`;

const normalize = (payload) => ({
  items: (Array.isArray(payload?.items) ? payload.items : []).map((item) => ({
    ...item,
    request_count: Number(item?.request_count || 0),
    configured_cost_request_count: Number(item?.configured_cost_request_count || 0),
    estimated_cost_request_count: Number(item?.estimated_cost_request_count || 0),
    pending_cost_request_count: Number(item?.pending_cost_request_count || 0),
    unconfigured_cost_request_count: Number(item?.unconfigured_cost_request_count || 0),
    router_consumed_yyc: Number(item?.router_consumed_yyc || 0),
    sell_base_amount: Number(item?.sell_base_amount || 0),
    procurement_cost_base_amount: Number(item?.procurement_cost_base_amount || 0),
    gross_profit_base_amount: Number(item?.gross_profit_base_amount || 0),
    gross_margin: Number(item?.gross_margin || 0),
  })),
});

const recentRange = () => {
  const end = Math.floor(Date.now() / 1000);
  return { start_at: end - 7 * 24 * 60 * 60, end_at: end };
};

const pricingState = (row) => {
  if (row.unconfigured_cost_request_count > 0 || row.pending_cost_request_count > 0) return 'unknown';
  if (row.configured_cost_request_count <= 0) return 'unknown';
  if (row.gross_margin < 0) return 'loss';
  if (row.gross_margin < 0.1) return 'low_margin';
  return 'healthy';
};

function BillingPricingAnalysis() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await API.get('/api/v1/admin/billing/procurement-report', {
        params: { ...recentRange(), group_by: 'model', cost_scope: 'all' },
      });
      if (!response.data?.success) {
        showError(response.data?.message || t('billing.pricing_analysis.load_failed'));
        return;
      }
      setRows(normalize(response.data.data).items);
    } catch (error) {
      showError(error?.message || t('billing.pricing_analysis.load_failed'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { load().then(); }, [load]);

  const columns = [
    {
      title: t('billing.pricing_analysis.columns.model'),
      dataIndex: 'dimension_key',
      width: 220,
      render: (value) => <span className='billing-pricing-analysis-model'>{value || '-'}</span>,
    },
    {
      title: t('billing.pricing_analysis.columns.state'),
      key: 'state',
      width: 120,
      render: (_, row) => {
        const state = pricingState(row);
        return <AppTag color={state === 'loss' ? 'red' : state === 'healthy' ? 'green' : 'orange'}>{t(`billing.pricing_analysis.states.${state}`)}</AppTag>;
      },
    },
    {
      title: t('billing.pricing_analysis.columns.requests'),
      dataIndex: 'request_count',
      width: 100,
      align: 'right',
      render: formatCount,
    },
    {
      title: t('billing.pricing_analysis.columns.router_yyc'),
      dataIndex: 'router_consumed_yyc',
      width: 130,
      align: 'right',
      render: formatCount,
    },
    {
      title: t('billing.pricing_analysis.columns.sell'),
      dataIndex: 'sell_base_amount',
      width: 130,
      align: 'right',
      render: formatCNY,
    },
    {
      title: t('billing.pricing_analysis.columns.cost'),
      dataIndex: 'procurement_cost_base_amount',
      width: 130,
      align: 'right',
      render: formatCNY,
    },
    {
      title: t('billing.pricing_analysis.columns.profit'),
      dataIndex: 'gross_profit_base_amount',
      width: 130,
      align: 'right',
      render: formatCNY,
    },
    {
      title: t('billing.pricing_analysis.columns.margin'),
      dataIndex: 'gross_margin',
      width: 110,
      align: 'right',
      render: formatPercent,
    },
    {
      title: t('billing.pricing_analysis.columns.cost_coverage'),
      key: 'coverage',
      width: 150,
      align: 'right',
      render: (_, row) => `${formatCount(row.configured_cost_request_count)} / ${formatCount(row.request_count)}`,
    },
    {
      title: t('billing.pricing_analysis.columns.details'),
      key: 'details',
      width: 100,
      render: () => <Link to='/admin/billing/procurement-report'>{t('billing.pricing_analysis.columns.view')}</Link>,
    },
  ];

  return (
    <div className='dashboard-container billing-pricing-analysis-page'>
      <AppFilterHeader
        breadcrumbs={[{ key: 'billing', label: t('header.billing') }, { key: 'pricing-analysis', label: t('billing.pricing_analysis.title'), active: true }]}
        title={t('billing.pricing_analysis.title')}
        actions={<AppButton className='router-page-button' color='blue' loading={loading} onClick={() => load().then()}>{t('common.refresh')}</AppButton>}
      />
      <AppSpin spinning={loading}>
        <AppSection className='billing-pricing-analysis-section'>
          <div className='billing-pricing-analysis-note'>{t('billing.pricing_analysis.note')}</div>
          <AppTable className='router-detail-table billing-pricing-analysis-table' size='small' pagination={false} rowKey={(row) => row.dimension_key} dataSource={rows} columns={columns} locale={{ emptyText: t('billing.pricing_analysis.empty') }} />
        </AppSection>
      </AppSpin>
    </div>
  );
}

export default BillingPricingAnalysis;
