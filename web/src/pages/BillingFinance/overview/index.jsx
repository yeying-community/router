import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { API, showError, showSuccess, timestamp2string, withCardLabels } from '../../../helpers';
import { formatDecimalNumber, formatCreditAmount } from '../../../helpers/render';
import {
  BILLING_DECIMALS,
  BILLING_PERCENT_DECIMALS,
  chartAxisStyle,
  chartCategoricalPalette,
  chartGridStyle,
  chartStatusPalette,
  chartTooltipStyle,
  formatBillingPercent,
  formatCnyFixed,
  formatCsvCurrency,
  formatCsvPercent,
} from '../../../router-ui/theme/charts';
import {
  AppButton,
  AppErrorState,
  AppFilterHeader,
  AppInput,
  AppSelect,
  AppSegmented,
  AppSpin,
  AppTable,
  AppTag,
} from '../../../router-ui';
import { exportCSV } from '../../../helpers/csv';
import './BillingOverview.css';

const formatCNY = (value) => formatCnyFixed(value, BILLING_DECIMALS);
const formatYYC = (value) => formatCreditAmount(value || 0, true);
const formatCount = (value) => formatDecimalNumber(value || 0, 0);
const formatPercent = (value) => formatBillingPercent(value, BILLING_PERCENT_DECIMALS);

const riskLevel = (critical, warning) => {
  if (Number(critical || 0) > 0) return 'critical';
  if (Number(warning || 0) > 0) return 'warning';
  return 'ok';
};

const recentRange = () => {
  const end = Math.floor(Date.now() / 1000);
  return { start_at: end - 7 * 24 * 60 * 60, end_at: end };
};

const positiveTimestamp = (value, fallback) => {
  const normalized = Number(value || 0);
  return Number.isFinite(normalized) && normalized > 0 ? normalized : fallback;
};

const toDateTimeLocalValue = (timestamp) => {
  const date = new Date(Number(timestamp || 0) * 1000);
  if (Number.isNaN(date.getTime())) return '';
  const pad = (value) => String(value).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
};

const timestampFromDateTimeLocal = (value, fallback) => {
  const timestamp = Math.floor(new Date(value || '').getTime() / 1000);
  return Number.isFinite(timestamp) && timestamp > 0 ? timestamp : fallback;
};

const normalize = (payload) => ({
  request_count: Number(payload?.request_count || 0),
  router_consumed_yyc: Number(payload?.router_consumed_yyc || 0),
  sell_base_amount: Number(payload?.sell_base_amount || 0),
  procurement_cost_base_amount: Number(payload?.procurement_cost_base_amount || 0),
  gross_profit_base_amount: Number(payload?.gross_profit_base_amount || 0),
  gross_profit_yyc: Number(payload?.gross_profit_yyc || 0),
  gross_margin: Number(payload?.gross_margin || 0),
  configured_cost_request_count: Number(payload?.configured_cost_request_count || 0),
  estimated_cost_request_count: Number(payload?.estimated_cost_request_count || 0),
  pending_cost_request_count: Number(payload?.pending_cost_request_count || 0),
  retry_cost_request_count: Number(payload?.retry_cost_request_count || 0),
  unconfigured_cost_request_count: Number(payload?.unconfigured_cost_request_count || 0),
  cost_floor_triggered_count: Number(payload?.cost_floor_triggered_count || 0),
  cost_floor_triggered_amount: Number(payload?.cost_floor_triggered_amount || 0),
  items: Array.isArray(payload?.items) ? payload.items : [],
});

const buildOperatingRiskItems = (items, t) => {
  const risks = [];
  (Array.isArray(items) ? items : []).forEach((item) => {
    const modelKey = item.dimension_key || '';
    const model = item.dimension_name || modelKey || '-';
    const requestCount = Number(item.request_count || 0);
    const unconfigured = Number(item.unconfigured_cost_request_count || 0);
    const estimated = Number(item.estimated_cost_request_count || 0);
    const pending = Number(item.pending_cost_request_count || 0);
    const retry = Number(item.retry_cost_request_count || 0);
    const configured = Number(item.configured_cost_request_count || 0);
    const profit = Number(item.gross_profit_base_amount || 0);
    const margin = Number(item.gross_margin || 0);
    const floorCount = Number(item.cost_floor_triggered_count || 0);
    const push = (type, level, count, weight, text, target) => risks.push({
      key: `${model}-${type}`,
      type,
      level,
      model: modelKey,
      count,
      weight,
      text,
      target,
    });
    if (unconfigured > 0) push('unconfigured', 'critical', unconfigured, unconfigured, t('billing.overview.operating_risks.unconfigured', { model, count: formatCount(unconfigured) }), 'procurement');
    if (configured > 0 && profit < 0) push('loss', 'critical', configured, Math.abs(profit), t('billing.overview.operating_risks.loss', { model, amount: formatCNY(profit) }), 'profit');
    else if (configured > 0 && margin < 0.1) push('low_margin', 'warning', configured, requestCount, t('billing.overview.operating_risks.low_margin', { model, margin: formatPercent(margin) }), 'profit');
    if (floorCount > 0) push('floor', 'warning', floorCount, floorCount, t('billing.overview.operating_risks.floor', { model, count: formatCount(floorCount) }), 'profit');
    if (estimated > 0) push('estimated', 'warning', estimated, estimated, t('billing.overview.operating_risks.estimated', { model, count: formatCount(estimated) }), 'procurement');
    if (pending > 0) push('pending', 'warning', pending, pending, t('billing.overview.operating_risks.pending', { model, count: formatCount(pending) }), 'procurement');
    if (retry > 0) push('retry', 'critical', retry, retry, t('billing.overview.operating_risks.retry', { model, count: formatCount(retry) }), 'procurement');
  });
  return risks.sort((left, right) => (
    left.level === right.level
      ? right.weight - left.weight
      : left.level === 'critical' ? -1 : 1
  ));
};

function BillingOverview({ embedded = false }) {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const initialContext = useMemo(() => {
    const defaults = recentRange();
    const params = new URLSearchParams(location.search);
    return {
      startAt: positiveTimestamp(params.get('start_at'), defaults.start_at),
      endAt: positiveTimestamp(params.get('end_at'), defaults.end_at),
      channelID: params.get('channel_id') || '',
      modelName: params.get('model') || '',
      dimension: params.get('dimension') === 'model' ? 'model' : 'channel',
    };
  }, []);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const [report, setReport] = useState(() => normalize({}));
  const [modelReport, setModelReport] = useState(() => normalize({}));
  const [health, setHealth] = useState({ status: 'ok', issues: [], critical_count: 0, warning_count: 0 });
  const [trend, setTrend] = useState([]);
  const [consistencyIssues, setConsistencyIssues] = useState([]);
  const [startAt, setStartAt] = useState(initialContext.startAt);
  const [endAt, setEndAt] = useState(initialContext.endAt);
  const [channelID, setChannelID] = useState(initialContext.channelID);
  const [modelName, setModelName] = useState(initialContext.modelName);
  const [channelOptions, setChannelOptions] = useState([]);
  const [modelOptions, setModelOptions] = useState([]);
  const [dimension, setDimension] = useState(initialContext.dimension);

  const financeContext = useMemo(() => ({
    start_at: startAt,
    end_at: endAt,
    channel_id: channelID,
    model: modelName,
  }), [channelID, endAt, modelName, startAt]);

  const buildTarget = useCallback((tab, overrides = {}) => {
    const params = new URLSearchParams();
    params.set('tab', tab);
    Object.entries({ ...financeContext, ...overrides }).forEach(([key, value]) => {
      if (value !== '' && value !== null && value !== undefined) params.set(key, String(value));
    });
    // Drill-downs return to the overview tab with its filters intact; the source
    // tab must be encoded explicitly since every finance page now shares the
    // `/admin/finance` pathname.
    const currentParams = new URLSearchParams();
    currentParams.set('tab', 'overview');
    Object.entries(financeContext).forEach(([key, value]) => {
      if (value !== '') currentParams.set(key, String(value));
    });
    params.set('return_to', `/admin/finance?${currentParams.toString()}`);
    return `/admin/finance?${params.toString()}`;
  }, [financeContext]);

  useEffect(() => {
    const params = new URLSearchParams();
    Object.entries(financeContext).forEach(([key, value]) => {
      if (value !== '') params.set(key, String(value));
    });
    // Persist the active dimension (channel/model) so the view is bookmarkable;
    // the default (channel) is stripped to keep URLs clean.
    if (dimension === 'model') params.set('dimension', dimension);
    // Preserve the shell tab so the finance layout doesn't fall back to overview
    // when this page rewrites its own filter params.
    const currentTab = new URLSearchParams(location.search).get('tab');
    if (currentTab) params.set('tab', currentTab);
    navigate({ pathname: location.pathname, search: `?${params.toString()}` }, { replace: true });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [financeContext, dimension, location.pathname, navigate]);

  const load = useCallback(async () => {
    if (!startAt || !endAt || endAt < startAt) {
      showError(t('billing.overview.invalid_time'));
      return;
    }
    setLoading(true);
    setLoadError(false);
    try {
      const filters = { start_at: startAt, end_at: endAt, channel_id: channelID, model: modelName };
      const optionRange = recentRange();
      const [reportResponse, filteredModelResponse, modelOptionsResponse, channelOptionsResponse, healthResponse, trendResponse, consistencyResponse] = await Promise.all([
        API.get('/api/v1/admin/billing/procurement-report', { params: { ...filters, group_by: 'channel', cost_scope: 'all' } }),
        API.get('/api/v1/admin/billing/procurement-report', { params: { ...filters, group_by: 'model', cost_scope: 'all' } }),
        API.get('/api/v1/admin/billing/procurement-report', { params: { ...optionRange, group_by: 'model', cost_scope: 'all' } }),
        API.get('/api/v1/admin/billing/procurement-report', { params: { ...optionRange, group_by: 'channel', cost_scope: 'all' } }),
        API.get('/api/v1/admin/billing/health'),
        API.get('/api/v1/admin/billing/procurement-trend', { params: filters }),
        API.get('/api/v1/admin/billing/finance/consistency/issues', { params: { start_at: startAt, end_at: endAt, limit: 200 } }),
      ]);
      if (!reportResponse.data?.success) throw new Error(reportResponse.data?.message);
      setReport(normalize(reportResponse.data.data));
      if (filteredModelResponse.data?.success) setModelReport(normalize(filteredModelResponse.data.data));
      if (modelOptionsResponse.data?.success) {
        const items = Array.isArray(modelOptionsResponse.data?.data?.items) ? modelOptionsResponse.data.data.items : [];
        setModelOptions(items.map((item) => ({ key: item.dimension_key, value: item.dimension_key, text: item.dimension_key })));
      }
      if (channelOptionsResponse.data?.success) {
        const items = Array.isArray(channelOptionsResponse.data?.data?.items) ? channelOptionsResponse.data.data.items : [];
        setChannelOptions(items.map((item) => ({ key: item.dimension_key, value: item.dimension_key, text: item.dimension_name || item.dimension_key })));
      }
      if (healthResponse.data?.success) setHealth(healthResponse.data.data || {});
      if (trendResponse.data?.success) setTrend(Array.isArray(trendResponse.data?.data?.items) ? trendResponse.data.data.items : []);
      if (consistencyResponse.data?.success) setConsistencyIssues(Array.isArray(consistencyResponse.data?.data?.items) ? consistencyResponse.data.data.items : []);
    } catch (error) {
      setLoadError(true);
      showError(error?.message || t('billing.overview.load_failed'));
    } finally {
      setLoading(false);
    }
  }, [channelID, endAt, modelName, startAt, t]);

  useEffect(() => { load().then(); }, [load]);

  const hasRequests = report.request_count > 0;
  const knownRatio = hasRequests ? report.configured_cost_request_count / report.request_count : 0;
  const operatingRiskItems = buildOperatingRiskItems(modelReport.items, t);
  const configurationRisks = (Array.isArray(health.issues) ? health.issues : []).map((issue) => ({
    ...issue,
    source: 'configuration',
    level: issue.level || 'warning',
    count: Number(issue.count || 0),
    target: buildTarget('procurement'),
  }));
  const configurationRiskCount = Number(health.critical_count || 0) + Number(health.warning_count || 0);
  const operatingCriticalCount = operatingRiskItems.filter((item) => item.level === 'critical').length;
  const operatingWarningCount = operatingRiskItems.length - operatingCriticalCount;
  const negativeChannelCount = report.items.filter((item) => Number(item.gross_profit_base_amount || 0) < 0).length;
  const lowMarginModelCount = modelReport.items.filter((item) => Number(item.configured_cost_request_count || 0) > 0 && Number(item.gross_margin || 0) >= 0 && Number(item.gross_margin || 0) < 0.1).length;
  const currentScopeRiskCount = operatingRiskItems.length;
  const priorityRisks = [
    ...operatingRiskItems.map((issue) => ({
      ...issue,
      source: 'operating',
      target: buildTarget(issue.target === 'profit' ? 'profit' : 'procurement', {
        model: issue.model,
        cost_scope: issue.target === 'procurement' && issue.type === 'unconfigured' ? 'unconfigured' : undefined,
      }),
    })),
    ...configurationRisks,
  ].sort((left, right) => (
    left.level === right.level ? Number(right.weight || right.count || 0) - Number(left.weight || left.count || 0) : left.level === 'critical' ? -1 : 1
  ));
  const overviewRows = [
    {
      key: 'profitability',
      dimension: t('billing.overview.dimensions.profitability.title'),
      primary: t('billing.overview.dimensions.profitability.primary', { revenue: formatCNY(report.sell_base_amount), profit: formatCNY(report.gross_profit_base_amount) }),
      secondary: t('billing.overview.dimensions.profitability.secondary', { margin: formatPercent(report.gross_margin), requests: formatCount(report.request_count) }),
      level: !hasRequests ? 'empty' : report.gross_profit_base_amount < 0 ? 'critical' : report.gross_margin < 0.1 ? 'warning' : 'ok',
      target: buildTarget('profit'),
    },
    {
      key: 'cost_coverage',
      dimension: t('billing.overview.dimensions.cost_coverage.title'),
      primary: t('billing.overview.dimensions.cost_coverage.primary', { coverage: formatPercent(knownRatio) }),
      secondary: t('billing.overview.dimensions.cost_coverage.secondary', { configured: formatCount(report.configured_cost_request_count), unconfigured: formatCount(report.unconfigured_cost_request_count), pending: formatCount(report.pending_cost_request_count) }),
      level: !hasRequests ? 'empty' : report.unconfigured_cost_request_count > 0 || report.retry_cost_request_count > 0 ? 'critical' : knownRatio < 1 ? 'warning' : 'ok',
      target: buildTarget('procurement', { cost_scope: report.unconfigured_cost_request_count > 0 ? 'unconfigured' : undefined }),
    },
    {
      key: 'channel',
      dimension: t('billing.overview.dimensions.channel.title'),
      primary: t('billing.overview.dimensions.channel.primary', { count: formatCount(report.items.length), loss: formatCount(negativeChannelCount) }),
      secondary: t('billing.overview.dimensions.channel.secondary'),
      level: !hasRequests ? 'empty' : negativeChannelCount > 0 ? 'critical' : 'ok',
      target: buildTarget('procurement'),
    },
    {
      key: 'model',
      dimension: t('billing.overview.dimensions.model.title'),
      primary: t('billing.overview.dimensions.model.primary', { count: formatCount(modelReport.items.length), low: formatCount(lowMarginModelCount) }),
      secondary: t('billing.overview.dimensions.model.secondary'),
      level: !hasRequests ? 'empty' : lowMarginModelCount > 0 ? 'warning' : 'ok',
      target: buildTarget('profit'),
    },
    {
      key: 'operating',
      dimension: t('billing.overview.dimensions.operating.title'),
      primary: t('billing.overview.dimensions.operating.primary', { count: formatCount(currentScopeRiskCount) }),
      secondary: t('billing.overview.dimensions.operating.secondary', { critical: formatCount(operatingCriticalCount), warning: formatCount(operatingWarningCount) }),
      level: riskLevel(operatingCriticalCount, operatingWarningCount),
      target: operatingRiskItems[0]?.target === 'procurement' ? buildTarget('procurement', { model: operatingRiskItems[0]?.model, cost_scope: 'unconfigured' }) : buildTarget('profit', { model: operatingRiskItems[0]?.model }),
    },
    {
      key: 'configuration',
      dimension: t('billing.overview.dimensions.configuration.title'),
      primary: t('billing.overview.dimensions.configuration.primary', { count: formatCount(configurationRiskCount) }),
      secondary: t('billing.overview.dimensions.configuration.secondary', { critical: formatCount(health.critical_count || 0), warning: formatCount(health.warning_count || 0) }),
      level: riskLevel(health.critical_count, health.warning_count),
      target: buildTarget('procurement'),
    },
  ];

  const statusColor = (level) => (level === 'critical' ? 'red' : level === 'warning' ? 'orange' : level === 'empty' ? 'grey' : 'green');
  const statusLabel = (level) => t(`billing.overview.status.${level || 'ok'}`);

  // The six KPIs the previous version rendered are condensed into one headline:
  // gross profit + gross margin are the action-relevant numbers; revenue/cost live
  // in the trend chart and the drill-down pages.
  const headlineMarginTone =
    report.gross_margin < 0 ? 'negative' : report.gross_margin < 0.1 ? 'warning' : 'positive';
  const headlineProfit = formatCNY(report.gross_profit_base_amount);
  const headlineMargin = formatPercent(report.gross_margin);
  const headlineUnconfigured = Number(report.unconfigured_cost_request_count || 0);
  const headlineUnconfiguredAmount = Number(report.unconfigured_sell_base_amount || 0);
  const headlineIssuesCount = currentScopeRiskCount;

  // Surface the most urgent risk per source as inline chips; the full list lives
  // in the priority-risk table below.
  const headlineChips = overviewRows
    .filter((row) => row.level === 'critical' || row.level === 'warning')
    .slice(0, 4);

  const trendChartData = trend.map((item) => ({
    day: item.day,
    revenue: Number(item.sell_base_amount || 0),
    cost: Number(item.procurement_cost_base_amount || 0),
    profit: Number(item.gross_profit_base_amount || 0),
  }));
  const trendSeries = [
    { key: 'revenue', color: chartCategoricalPalette[0], label: t('billing.procurement_report.summary.sell_amount') },
    { key: 'cost', color: chartStatusPalette.warning, label: t('billing.procurement_report.summary.procurement_cost') },
    { key: 'profit', color: chartStatusPalette.success, label: t('billing.procurement_report.summary.gross_profit') },
  ];

  const priorityColumns = [
    { title: t('billing.overview.priority.columns.source'), dataIndex: 'source', width: 96, render: (value) => t(`billing.overview.priority.sources.${value}`) },
    { title: t('billing.overview.priority.columns.level'), dataIndex: 'level', width: 88, render: (value) => <AppTag color={statusColor(value)}>{statusLabel(value)}</AppTag> },
    { title: t('billing.overview.priority.columns.issue'), key: 'issue', render: (_, row) => row.text || row.title || '-' },
    { title: t('billing.overview.priority.columns.impact'), key: 'impact', width: 120, align: 'right', render: (_, row) => formatCount(row.count) || '-' },
    { title: t('billing.overview.priority.columns.action'), key: 'action', width: 110, render: (_, row) => <Link to={row.target}>{row.source === 'configuration' || row.target.includes('/procurement') ? t('billing.overview.actions.procurement') : t('billing.overview.actions.profit')}</Link> },
  ];

  const channelColumns = [
    { title: t('billing.overview.channels.columns.channel'), key: 'channel', width: 200, render: (_, row) => row.dimension_name || row.dimension_key || '-' },
    { title: t('billing.overview.channels.columns.requests'), dataIndex: 'request_count', width: 100, align: 'right', render: formatCount },
    { title: t('billing.overview.channels.columns.revenue'), dataIndex: 'sell_base_amount', width: 130, align: 'right', render: formatCNY },
    { title: t('billing.overview.channels.columns.cost'), dataIndex: 'procurement_cost_base_amount', width: 130, align: 'right', render: formatCNY },
    { title: t('billing.overview.channels.columns.profit'), dataIndex: 'gross_profit_base_amount', width: 130, align: 'right', render: formatCNY },
    { title: t('billing.overview.channels.columns.margin'), dataIndex: 'gross_margin', width: 110, align: 'right', render: formatPercent },
    { title: t('billing.overview.channels.columns.coverage'), key: 'coverage', width: 130, align: 'right', render: (_, row) => formatPercent(Number(row.configured_cost_request_count || 0) / Math.max(Number(row.request_count || 0), 1)) },
    { title: t('billing.overview.channels.columns.actions'), key: 'actions', width: 170, render: (_, row) => <div className='billing-overview-actions'><Link to={buildTarget('profit', { channel_id: row.dimension_key })}>{t('billing.overview.actions.profit')}</Link><Link to={`/admin/channel/detail/${encodeURIComponent(String(row.dimension_key || ''))}?tab=procurement`}>{t('billing.overview.actions.procurement')}</Link></div> },
  ];

  const modelColumns = [
    { title: t('billing.overview.models.columns.model'), key: 'model', width: 200, render: (_, row) => row.dimension_name || row.dimension_key || '-' },
    { title: t('billing.overview.models.columns.requests'), dataIndex: 'request_count', width: 100, align: 'right', render: formatCount },
    { title: t('billing.overview.models.columns.profit'), dataIndex: 'gross_profit_base_amount', width: 130, align: 'right', render: formatCNY },
    { title: t('billing.overview.models.columns.margin'), dataIndex: 'gross_margin', width: 110, align: 'right', render: formatPercent },
    { title: t('billing.overview.models.columns.coverage'), key: 'coverage', width: 130, align: 'right', render: (_, row) => formatPercent(Number(row.configured_cost_request_count || 0) / Math.max(Number(row.request_count || 0), 1)) },
    { title: t('billing.overview.models.columns.actions'), key: 'actions', width: 170, render: (_, row) => <div className='billing-overview-actions'><Link to={buildTarget('profit', { model: row.dimension_key })}>{t('billing.overview.actions.profit')}</Link><Link to={buildTarget('procurement', { model: row.dimension_key })}>{t('billing.overview.actions.procurement')}</Link></div> },
  ];

  const activeDimensionRows = dimension === 'channel' ? report.items.slice(0, 10) : modelReport.items.slice(0, 10);
  const activeDimensionColumns = dimension === 'channel' ? channelColumns : modelColumns;
  const consistencyIssueColumns = [
    { title: t('billing.overview.consistency.columns.type'), dataIndex: 'issue_type', width: 180, render: (value) => t(`billing.overview.consistency.types.${value}`, { defaultValue: value || '-' }) },
    { title: t('billing.overview.consistency.columns.request'), dataIndex: 'request_log_id', width: 280, render: (value) => <Link to={`/admin/log/${encodeURIComponent(value || '')}`}>{value || '-'}</Link> },
    { title: t('billing.overview.consistency.columns.created_at'), dataIndex: 'created_at', width: 180, render: (value) => (value ? timestamp2string(value) : '-') },
    { title: t('billing.overview.consistency.columns.prompt_tokens'), dataIndex: 'prompt_tokens', width: 120, align: 'right', render: formatCount },
    { title: t('billing.overview.consistency.columns.completion_tokens'), dataIndex: 'completion_tokens', width: 120, align: 'right', render: formatCount },
  ];

  return (
    <div className={`${embedded ? '' : 'dashboard-container '}billing-overview-page`}>
      <AppFilterHeader
        breadcrumbs={embedded ? undefined : [{ key: 'finance', label: t('header.finance') }, { key: 'billing-overview', label: t('billing.overview.title'), active: true }]}
        actions={
          <>
            <AppButton
              className='router-page-button'
              loading={loading}
              onClick={() => {
                exportCSV(
                  `billing-overview-${toDateTimeLocalValue(startAt).replace(/[:T]/g, '-')}_${toDateTimeLocalValue(endAt).replace(/[:T]/g, '-')}.csv`,
                  [
                    { key: 'dimension_key', label: t('billing.overview.columns.dimension') },
                    { key: 'request_count', label: t('billing.overview.columns.request_count') },
                    { key: 'sell_base_amount', label: t('billing.procurement_report.summary.sell_amount'), format: formatCsvCurrency },
                    { key: 'procurement_cost_base_amount', label: t('billing.procurement_report.summary.procurement_cost'), format: formatCsvCurrency },
                    { key: 'gross_profit_base_amount', label: t('billing.procurement_report.summary.gross_profit'), format: formatCsvCurrency },
                    { key: 'gross_margin', label: t('billing.procurement_report.summary.gross_margin'), format: formatCsvPercent },
                    { key: 'configured_cost_request_count', label: t('billing.procurement_report.columns.configured_count') },
                    { key: 'unconfigured_cost_request_count', label: t('billing.procurement_report.columns.unconfigured_count') },
                  ],
                  report.items,
                );
                showSuccess(t('billing.export.success', { count: report.items.length }));
              }}
            >
              {t('common.export_csv')}
            </AppButton>
            <AppButton className='router-page-button' color='blue' loading={loading} onClick={() => load().then()}>{t('common.refresh')}</AppButton>
          </>
        }
        query={<div className='billing-overview-filters'><AppInput className='billing-overview-time-input' type='datetime-local' value={toDateTimeLocalValue(startAt)} onChange={(e, { value }) => setStartAt(timestampFromDateTimeLocal(value, startAt))} /><AppInput className='billing-overview-time-input' type='datetime-local' value={toDateTimeLocalValue(endAt)} onChange={(e, { value }) => setEndAt(timestampFromDateTimeLocal(value, endAt))} /><AppSelect className='billing-overview-channel-select' clearable search options={channelOptions} value={channelID} placeholder={t('billing.overview.channel_placeholder')} onChange={(e, { value }) => setChannelID((value || '').toString())} /><AppSelect className='billing-overview-model-select' clearable search options={modelOptions} value={modelName} placeholder={t('billing.overview.model_placeholder')} onChange={(e, { value }) => setModelName((value || '').toString())} /></div>}
      />
      <div className='billing-overview-context'><span>{t('billing.overview.context.range', { start: toDateTimeLocalValue(startAt).replace('T', ' '), end: toDateTimeLocalValue(endAt).replace('T', ' ') })}</span><span>{t('billing.overview.context.currency')}</span></div>
      <AppSpin spinning={loading}>
        {loadError ? (
          <AppErrorState
            message={t('billing.overview.load_failed')}
            onRetry={() => load().then()}
            retryText={t('common.retry')}
          />
        ) : (
        <>
        <div className='billing-overview-headline'>
          <div className={`billing-overview-headline-main is-${headlineMarginTone}`}>
            <span className='billing-overview-headline-label'>{t('billing.overview.headline.profit')}</span>
            <span className='billing-overview-headline-value'>{headlineProfit}</span>
            <span className='billing-overview-headline-sub'>{t('billing.overview.headline.margin', { margin: headlineMargin })}</span>
          </div>
          <div className='billing-overview-headline-side'>
            <div className='billing-overview-headline-stat'>
              <span className='billing-overview-headline-stat-label'>{t('billing.overview.headline.profit_yyc')}</span>
              <span className='billing-overview-headline-stat-value'>{formatYYC(report.gross_profit_yyc)}</span>
            </div>
            <div className='billing-overview-headline-stat'>
              <span className='billing-overview-headline-stat-label'>{t('billing.overview.headline.revenue')}</span>
              <span className='billing-overview-headline-stat-value'>{formatCNY(report.sell_base_amount)}</span>
            </div>
            <div className='billing-overview-headline-stat'>
              <span className='billing-overview-headline-stat-label'>{t('billing.overview.headline.cost')}</span>
              <span className='billing-overview-headline-stat-value'>{formatCNY(report.procurement_cost_base_amount)}</span>
            </div>
            <div className={`billing-overview-headline-stat${headlineUnconfigured > 0 ? ' is-warning' : ''}`}>
              <span className='billing-overview-headline-stat-label'>{t('billing.overview.headline.unconfigured')}</span>
              <span className='billing-overview-headline-stat-value'>{formatCount(headlineUnconfigured)}</span>
            </div>
            <div className='billing-overview-headline-stat'>
              <span className='billing-overview-headline-stat-label'>{t('billing.overview.headline.requests')}</span>
              <span className='billing-overview-headline-stat-value'>{formatCount(report.request_count)}</span>
            </div>
          </div>
        </div>
        {headlineChips.length > 0 ? (
          <div className='billing-overview-headline-chips'>
            {headlineChips.map((chip) => (
              <Link key={chip.key} to={chip.target} className={`billing-overview-headline-chip is-${chip.level}`}>
                <AppTag color={statusColor(chip.level)}>{statusLabel(chip.level)}</AppTag>
                <span className='billing-overview-headline-chip-text'>{chip.dimension}</span>
                <span className='billing-overview-headline-chip-arrow'>›</span>
              </Link>
            ))}
          </div>
        ) : null}
        <section className='billing-overview-section'>
          <div className='billing-overview-section-heading'><h2>{t('billing.overview.trend.title')}</h2></div>
          {trendChartData.length > 0 ? (
            <div className='billing-overview-trend-chart'>
              <ResponsiveContainer width='100%' height={280}>
                <LineChart data={trendChartData} margin={{ top: 8, right: 16, left: 0, bottom: 0 }}>
                  <CartesianGrid {...chartGridStyle()} />
                  <XAxis dataKey='day' {...chartAxisStyle()} />
                  <YAxis {...chartAxisStyle()} width={72} tickFormatter={(value) => formatCNY(value)} />
                  <Tooltip contentStyle={chartTooltipStyle()} formatter={(value, name) => [formatCNY(value), trendSeries.find((s) => s.key === name)?.label || name]} />
                  <Legend formatter={(value) => trendSeries.find((s) => s.key === value)?.label || value} />
                  {trendSeries.map((series) => (
                    <Line key={series.key} type='monotone' dataKey={series.key} name={series.key} stroke={series.color} strokeWidth={2} dot={false} />
                  ))}
                </LineChart>
              </ResponsiveContainer>
            </div>
          ) : (
            <div className='billing-overview-empty'>{t('common.no_data')}</div>
          )}
        </section>
        <section className='billing-overview-section'>
          <div className='billing-overview-section-heading'><h2>{t('billing.overview.priority.title')}</h2><span>{t('billing.overview.priority.scope_note')}</span></div>
          {priorityRisks.length > 0 ? (
            <AppTable className='router-detail-table' size='small' pagination={false} rowKey={(row) => `${row.source}-${row.key}`} dataSource={priorityRisks.slice(0, 8)} columns={priorityColumns} scroll={{ x: 760 }} />
          ) : (
            <div className='billing-overview-empty'>{t('billing.overview.priority.empty')}</div>
          )}
        </section>
        <section className='billing-overview-section'>
          <div className='billing-overview-section-heading'><h2>{t(`billing.overview.${dimension === 'channel' ? 'channels' : 'models'}.title`)}</h2><div className='billing-overview-section-controls'><AppSegmented options={[{ value: 'channel', label: t('billing.overview.channels.title') }, { value: 'model', label: t('billing.overview.models.title') }]} value={dimension} onChange={(e, { value }) => setDimension(value)} /><Link to={buildTarget(dimension === 'channel' ? 'procurement' : 'profit')}>{t(`billing.overview.${dimension === 'channel' ? 'channels' : 'models'}.view_details`)}</Link></div></div>
          <div className='billing-overview-table-note'>{t(`billing.overview.${dimension === 'channel' ? 'channels' : 'models'}.sorted_note`)}</div>
          <AppTable className='router-detail-table router-table-cardify' size='small' pagination={false} rowKey={(row) => row.dimension_key} dataSource={activeDimensionRows} columns={withCardLabels(activeDimensionColumns)} scroll={{ x: dimension === 'channel' ? 1000 : 840 }} locale={{ emptyText: t(`billing.overview.${dimension === 'channel' ? 'channels' : 'models'}.empty`) }} />
        </section>
        <section className='billing-overview-section'>
          <div className='billing-overview-section-heading'><h2>{t('billing.overview.consistency.title')}</h2><span>{t('billing.overview.consistency.summary', { count: formatCount(consistencyIssues.length) })}</span></div>
          {consistencyIssues.length > 0 ? (
            <AppTable className='router-detail-table' size='small' pagination={false} rowKey={(row) => `${row.issue_type}-${row.request_log_id}`} dataSource={consistencyIssues} columns={consistencyIssueColumns} scroll={{ x: 900 }} locale={{ emptyText: t('billing.overview.consistency.empty') }} />
          ) : (
            <div className='billing-overview-empty'>{t('billing.overview.consistency.empty')}</div>
          )}
        </section>
        </>
        )}
      </AppSpin>
    </div>
  );
}

export default BillingOverview;
