import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { API, showError } from '../../helpers';
import { formatDecimalNumber } from '../../helpers/render';
import {
  AppButton,
  AppFilterHeader,
  AppSelect,
  AppSection,
  AppSpin,
  AppTable,
  AppTag,
} from '../../router-ui';
import './BillingPricingAnalysis.css';

const formatCNY = (value) => `¥${formatDecimalNumber(value || 0, 4)}`;
const formatCount = (value) => formatDecimalNumber(value || 0, 0);
const formatPercent = (value) => `${(Number(value || 0) * 100).toFixed(1)}%`;
const formatUnitCost = (value, unit) => value > 0 ? `${formatCNY(value)} / ${unit || '-'}` : '-';

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
    cost_floor_triggered_count: Number(item?.cost_floor_triggered_count || 0),
    cost_floor_triggered_amount: Number(item?.cost_floor_triggered_amount || 0),
    procurement_cost_base_per_unit: Number(item?.procurement_cost_base_per_unit || 0),
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

const summarizeMatrix = (items) => {
  const grouped = {};
  (Array.isArray(items) ? items : []).forEach((item) => {
    const key = String(item?.model || '').trim();
    if (!key) return;
    if (!grouped[key]) grouped[key] = [];
    grouped[key].push(item);
  });
  return Object.fromEntries(Object.entries(grouped).map(([model, values]) => {
    const inputs = new Set(values.map((item) => Number(item?.current_input_sell || 0)));
    const official = new Set(values.map((item) => Number(item?.official_input_price || 0)));
    const ratios = new Set(values.map((item) => Number(item?.group_channel_ratio || 0)));
    return [model, {
      channel_count: values.length,
      mixed: inputs.size > 1 || official.size > 1 || ratios.size > 1,
      official_input_price: values[0]?.official_input_price || 0,
      current_input_sell: values[0]?.current_input_sell || 0,
      group_channel_ratio: values[0]?.group_channel_ratio || 0,
      pricing_source: values[0]?.pricing_source || '',
      procurement_cost_state: values[0]?.procurement_cost_state || 'unit_mismatch',
      procurement_cost_base_per_unit: values[0]?.procurement_cost_base_per_unit || 0,
      procurement_cost_unit: values[0]?.procurement_cost_unit || '',
      procurement_cost_in_price_unit: values[0]?.procurement_cost_in_price_unit || 0,
      selected_input_sell: values[0]?.selected_input_sell || 0,
      final_pricing_state: values[0]?.final_pricing_state || 'cost_unavailable',
    }];
  }));
};

function BillingPricingAnalysis() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState([]);
  const [groupID, setGroupID] = useState('');
  const [groupOptions, setGroupOptions] = useState([]);

  useEffect(() => {
    API.get('/api/v1/admin/groups', { params: { page: 1, page_size: 200 } })
      .then((response) => {
        if (!response.data?.success) return;
        const items = Array.isArray(response.data?.data?.items) ? response.data.data.items : [];
        setGroupOptions(items.map((group) => ({ key: group.id, value: group.id, text: group.name || group.id })));
      })
      .catch(() => {});
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [response, matrixResponse] = await Promise.all([
        API.get('/api/v1/admin/billing/procurement-report', {
          params: { ...recentRange(), group_by: 'model', cost_scope: 'all', group_id: groupID },
        }),
        API.get('/api/v1/admin/billing/pricing-matrix', { params: { group_id: groupID } }),
      ]);
      if (!response.data?.success) {
        showError(response.data?.message || t('billing.pricing_analysis.load_failed'));
        return;
      }
      const matrix = summarizeMatrix(matrixResponse.data?.success ? matrixResponse.data?.data?.items : []);
      setRows(normalize(response.data.data).items.map((item) => ({
        ...item,
        pricing_matrix: matrix[item.dimension_key] || null,
      })));
    } catch (error) {
      showError(error?.message || t('billing.pricing_analysis.load_failed'));
    } finally {
      setLoading(false);
    }
  }, [groupID, t]);

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
      title: t('billing.pricing_analysis.columns.official_price'),
      key: 'official_price',
      width: 130,
      align: 'right',
      render: (_, row) => row.pricing_matrix?.mixed ? t('billing.pricing_analysis.mixed') : formatCNY(row.pricing_matrix?.official_input_price),
    },
    {
      title: t('billing.pricing_analysis.columns.current_price'),
      key: 'current_price',
      width: 130,
      align: 'right',
      render: (_, row) => row.pricing_matrix?.mixed ? t('billing.pricing_analysis.mixed') : formatCNY(row.pricing_matrix?.current_input_sell),
    },
    {
      title: t('billing.pricing_analysis.columns.ratio'),
      key: 'ratio',
      width: 100,
      align: 'right',
      render: (_, row) => row.pricing_matrix?.mixed ? t('billing.pricing_analysis.mixed') : Number(row.pricing_matrix?.group_channel_ratio || 0).toFixed(2),
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
      title: t('billing.pricing_analysis.columns.cost_state'),
      key: 'cost_state',
      width: 140,
      render: (_, row) => <AppTag color={row.pricing_matrix?.procurement_cost_state === 'actual_available' ? 'green' : 'orange'}>{t(`billing.pricing_analysis.cost_states.${row.pricing_matrix?.procurement_cost_state || 'unit_mismatch'}`)}</AppTag>,
    },
    {
      title: t('billing.pricing_analysis.columns.cost_unit'),
      key: 'cost_unit',
      width: 150,
      align: 'right',
      render: (_, row) => formatUnitCost(row.pricing_matrix?.procurement_cost_base_per_unit, row.pricing_matrix?.procurement_cost_unit),
    },
    {
      title: t('billing.pricing_analysis.columns.selected_price'),
      key: 'selected_price',
      width: 140,
      align: 'right',
      render: (_, row) => row.pricing_matrix?.final_pricing_state === 'cost_currency_mismatch' ? '-' : (row.pricing_matrix?.mixed ? t('billing.pricing_analysis.mixed') : formatCNY(row.pricing_matrix?.selected_input_sell)),
    },
    {
      title: t('billing.pricing_analysis.columns.decision'),
      key: 'decision',
      width: 130,
      render: (_, row) => row.pricing_matrix?.cost_floor_triggered ? <AppTag color='red'>{t('billing.pricing_analysis.decision.cost_floor')}</AppTag> : <AppTag color='green'>{t('billing.pricing_analysis.decision.current_price')}</AppTag>,
    },
    {
      title: t('billing.pricing_analysis.columns.floor_impact'),
      key: 'floor_impact',
      width: 130,
      align: 'right',
      render: (_, row) => row.cost_floor_triggered_count > 0 ? `${formatCount(row.cost_floor_triggered_count)} / ${formatCNY(row.cost_floor_triggered_amount)}` : '-',
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
        actions={<AppButton className='router-page-button' color='blue' loading={loading} onClick={() => load().then()}>{t('common.refresh')}</AppButton>}
        query={<AppSelect className='billing-pricing-analysis-group-select' clearable search options={groupOptions} value={groupID} placeholder={t('billing.pricing_analysis.group_placeholder')} onChange={(e, { value }) => setGroupID((value || '').toString())} />}
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
