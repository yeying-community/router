import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { API, showError } from '../../helpers';
import { formatDecimalNumber } from '../../helpers/render';
import {
  AppButton,
  AppFilterHeader,
  AppSection,
  AppSelect,
  AppSpin,
  AppTag,
} from '../../router-ui';
import './BillingOverview.css';

const formatCNY = (value) => `¥${formatDecimalNumber(value || 0, 4)}`;
const formatCount = (value) => formatDecimalNumber(value || 0, 0);
const formatPercent = (value) => `${(Number(value || 0) * 100).toFixed(1)}%`;

const recentRange = () => {
  const end = Math.floor(Date.now() / 1000);
  return { start_at: end - 7 * 24 * 60 * 60, end_at: end };
};

const normalize = (payload) => ({
  request_count: Number(payload?.request_count || 0),
  router_consumed_yyc: Number(payload?.router_consumed_yyc || 0),
  sell_base_amount: Number(payload?.sell_base_amount || 0),
  procurement_cost_base_amount: Number(payload?.procurement_cost_base_amount || 0),
  gross_profit_base_amount: Number(payload?.gross_profit_base_amount || 0),
  gross_margin: Number(payload?.gross_margin || 0),
  configured_cost_request_count: Number(payload?.configured_cost_request_count || 0),
  estimated_cost_request_count: Number(payload?.estimated_cost_request_count || 0),
  pending_cost_request_count: Number(payload?.pending_cost_request_count || 0),
  unconfigured_cost_request_count: Number(payload?.unconfigured_cost_request_count || 0),
  cost_floor_triggered_count: Number(payload?.cost_floor_triggered_count || 0),
  cost_floor_triggered_amount: Number(payload?.cost_floor_triggered_amount || 0),
  items: Array.isArray(payload?.items) ? payload.items : [],
});

function BillingOverview() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [report, setReport] = useState(() => normalize({}));
  const [health, setHealth] = useState({ status: 'ok', issues: [], critical_count: 0, warning_count: 0 });
  const [trend, setTrend] = useState([]);
  const [channelID, setChannelID] = useState('');
  const [modelName, setModelName] = useState('');
  const [modelOptions, setModelOptions] = useState([]);

  const load = useCallback(async () => {
    setLoading(true);
    const range = recentRange();
    try {
      const [reportResponse, modelReportResponse, healthResponse, trendResponse] = await Promise.all([
        API.get('/api/v1/admin/billing/procurement-report', {
          params: { ...range, group_by: 'channel', cost_scope: 'all' },
        }),
        API.get('/api/v1/admin/billing/procurement-report', {
          params: { ...range, group_by: 'model', cost_scope: 'all' },
        }),
        API.get('/api/v1/admin/billing/health'),
        API.get('/api/v1/admin/billing/procurement-trend', { params: { ...range, channel_id: channelID, model: modelName } }),
      ]);
      if (reportResponse.data?.success) {
        setReport(normalize(reportResponse.data.data));
      } else {
        showError(reportResponse.data?.message || t('billing.overview.load_failed'));
      }
      if (modelReportResponse.data?.success) {
        setModelOptions((Array.isArray(modelReportResponse.data?.data?.items) ? modelReportResponse.data.data.items : []).map((item) => ({ key: item.dimension_key, value: item.dimension_key, text: item.dimension_key })));
      }
      if (healthResponse.data?.success) {
        setHealth(healthResponse.data.data || {});
      }
      if (trendResponse.data?.success) setTrend(Array.isArray(trendResponse.data?.data?.items) ? trendResponse.data.data.items : []);
    } catch (error) {
      showError(error?.message || t('billing.overview.load_failed'));
    } finally {
      setLoading(false);
    }
  }, [channelID, modelName, t]);

  useEffect(() => {
    load().then();
  }, [load]);

  const metricCards = [
    [t('billing.overview.metrics.revenue'), formatCNY(report.sell_base_amount)],
    [t('billing.overview.metrics.cost'), formatCNY(report.procurement_cost_base_amount)],
    [t('billing.overview.metrics.profit'), formatCNY(report.gross_profit_base_amount)],
    [t('billing.overview.metrics.margin'), formatPercent(report.gross_margin)],
    [t('billing.overview.metrics.yyc'), formatCount(report.router_consumed_yyc)],
    [t('billing.overview.metrics.requests'), formatCount(report.request_count)],
    [t('billing.overview.metrics.floor_triggered'), formatCount(report.cost_floor_triggered_count)],
    [t('billing.overview.metrics.floor_amount'), formatCNY(report.cost_floor_triggered_amount)],
  ];
  const knownCostCount = report.configured_cost_request_count;
  const totalCostStateCount = knownCostCount + report.estimated_cost_request_count + report.pending_cost_request_count + report.unconfigured_cost_request_count;
  const knownRatio = totalCostStateCount > 0 ? knownCostCount / totalCostStateCount : 0;
  const topRisks = (Array.isArray(health.issues) ? health.issues : []).slice(0, 5);
  const topChannels = report.items.slice(0, 5);

  return (
    <div className='dashboard-container billing-overview-page'>
      <AppFilterHeader
        breadcrumbs={[{ key: 'billing', label: t('header.billing') }, { key: 'billing-overview', label: t('billing.overview.title'), active: true }]}
        actions={<><AppSelect clearable options={report.items.map((item) => ({ key: item.dimension_key, value: item.dimension_key, text: item.dimension_key }))} value={channelID} placeholder={t('billing.overview.channel_placeholder')} onChange={(e, { value }) => setChannelID((value || '').toString())} /><AppSelect clearable options={modelOptions} value={modelName} placeholder={t('billing.overview.model_placeholder')} onChange={(e, { value }) => setModelName((value || '').toString())} /><AppButton className='router-page-button' color='blue' loading={loading} onClick={() => load().then()}>{t('common.refresh')}</AppButton></>}
      />
      <AppSpin spinning={loading}>
        <AppSection className='billing-overview-section'>
          <div className='billing-overview-metric-grid'>
            {metricCards.map(([label, value]) => <div className='billing-overview-metric' key={label}><div className='billing-overview-label'>{label}</div><div className='billing-overview-value'>{value}</div></div>)}
          </div>
          <div className={`billing-overview-confidence is-${knownRatio >= 0.8 ? 'good' : 'warning'}`}>
            <div><strong>{t('billing.overview.confidence.title')}</strong><span>{t('billing.overview.confidence.coverage', { value: formatPercent(knownRatio) })}</span></div>
            <div className='billing-overview-confidence-bar'><span style={{ width: `${Math.min(100, knownRatio * 100)}%` }} /></div>
            <div className='billing-overview-confidence-detail'>{t('billing.overview.confidence.detail', { estimated: formatCount(report.estimated_cost_request_count), pending: formatCount(report.pending_cost_request_count), unconfigured: formatCount(report.unconfigured_cost_request_count) })}</div>
          </div>
        </AppSection>
        <div className='billing-overview-columns'>
          <AppSection className='billing-overview-section'>
            <div className='billing-overview-section-heading'><h2>{t('billing.overview.risks.title')}</h2><Link to='/admin/billing/procurement-report'>{t('billing.overview.risks.view_details')}</Link></div>
            {topRisks.length === 0 ? <div className='billing-overview-empty'>{t('billing.overview.risks.empty')}</div> : topRisks.map((issue) => <div className='billing-overview-risk' key={issue.key}><AppTag color={issue.level === 'critical' ? 'red' : 'orange'}>{t(`billing.procurement_report.health.level.${issue.level || 'warning'}`)}</AppTag><span title={issue.message}>{issue.title}{issue.count ? ` (${formatCount(issue.count)})` : ''}</span></div>)}
          </AppSection>
          <AppSection className='billing-overview-section'>
            <div className='billing-overview-section-heading'><h2>{t('billing.overview.channels.title')}</h2><Link to='/admin/billing/procurement-report'>{t('billing.overview.channels.view_details')}</Link></div>
            {topChannels.length === 0 ? <div className='billing-overview-empty'>{t('billing.overview.channels.empty')}</div> : topChannels.map((item) => <div className='billing-overview-channel' key={item.dimension_key}><span>{item.dimension_name || item.dimension_key}</span><strong>{item.cost_floor_triggered_count > 0 ? `${formatCount(item.cost_floor_triggered_count)} / ${formatCNY(item.cost_floor_triggered_amount)}` : formatCNY(item.gross_profit_base_amount)}</strong></div>)}
          </AppSection>
        </div>
        <AppSection className='billing-overview-section'>
          <div className='billing-overview-section-heading'><h2>{t('billing.overview.trend.title')}</h2></div>
          <div className='billing-overview-trend'>{trend.map((item) => <div className='billing-overview-channel' key={item.day}><span>{item.day}</span><strong>{`${formatCount(item.cost_floor_triggered_count)} / ${formatCNY(item.cost_floor_triggered_amount)}`}</strong></div>)}</div>
        </AppSection>
      </AppSpin>
    </div>
  );
}

export default BillingOverview;
