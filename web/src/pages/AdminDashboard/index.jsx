import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation } from 'react-router-dom';
import { Button, Card, Dropdown } from 'semantic-ui-react';
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { API } from '../../helpers/api';
import {
  buildPublicDisplayCurrencyIndex,
  convertYYCToDisplayAmount,
  formatCompactDisplayAmount,
} from '../../helpers/billing';
import '../Dashboard/Dashboard.css';
import './AdminDashboard.css';

const PERIOD_OPTIONS = [
  'today',
  'last_7_days',
  'last_30_days',
  'this_month',
  'last_month',
  'this_year',
  'all_time',
];

const TREND_METRIC_OPTIONS = [
  'spend_amount',
  'topup_amount',
  'request_count',
  'active_user_count',
];

const DASHBOARD_SECTIONS = ['overview', 'trend', 'health'];
const DASHBOARD_SECTION_TITLES = {
  overview: 'dashboard.admin.nav.overview',
  trend: 'dashboard.admin.sections.trend',
  health: 'dashboard.admin.sections.channels',
};

const EMPTY_SUMMARY = {
  spend_amount: 0,
  topup_amount: 0,
  net_amount: 0,
  request_count: 0,
  active_user_count: 0,
  channel_total: 0,
  channel_enabled: 0,
  channel_disabled: 0,
  group_total: 0,
  provider_total: 0,
  task_active_total: 0,
  task_failed_total: 0,
};

const EMPTY_DASHBOARD = {
  period: 'last_7_days',
  granularity: 'day',
  start_timestamp: 0,
  end_timestamp: 0,
  summary: EMPTY_SUMMARY,
  trend: [],
  top_channels: [],
  generated_at: 0,
};

const HEALTH_LEVEL_COLORS = {
  healthy: '#16a34a',
  warning: '#f59e0b',
  critical: '#ef4444',
  unknown: '#94a3b8',
};

const formatCount = (value) => {
  const num = Number(value || 0);
  if (!Number.isFinite(num)) return '0';
  return num.toLocaleString('zh-CN');
};

const normalizeAdminDashboardPayload = (payload) => {
  const summary = payload?.summary || {};
  const trend = Array.isArray(payload?.trend) ? payload.trend : [];
  const topChannels = Array.isArray(payload?.top_channels)
    ? payload.top_channels
    : [];
  return {
    ...EMPTY_DASHBOARD,
    ...(payload || {}),
    summary: {
      ...EMPTY_SUMMARY,
      ...summary,
      spend_amount: Number(summary?.consume_yyc ?? summary?.consume_quota ?? 0),
      topup_amount: Number(summary?.topup_yyc ?? summary?.topup_quota ?? 0),
      net_amount: Number(summary?.net_yyc ?? summary?.net_quota ?? 0),
    },
    trend: trend.map((item) => ({
      ...item,
      spend_amount: Number(item?.consume_yyc ?? item?.consume_quota ?? 0),
      topup_amount: Number(item?.topup_yyc ?? item?.topup_quota ?? 0),
    })),
    top_channels: topChannels.map((item) => ({
      ...item,
      usedYyc: Number(item?.yyc_used ?? item?.used_quota ?? 0),
    })),
  };
};

const toPercent = (raw) => {
  const value = Number(raw || 0);
  if (!Number.isFinite(value)) return 0;
  if (value <= 1) return value * 100;
  return value;
};

const formatPercent = (raw) => `${toPercent(raw).toFixed(1)}%`;

const AdminDashboard = () => {
  const { t } = useTranslation();
  const location = useLocation();
  const displayCurrencyIndex = useMemo(
    () => buildPublicDisplayCurrencyIndex([]),
    [],
  );
  const [period, setPeriod] = useState('last_7_days');
  const [loading, setLoading] = useState(false);
  const [trendMetric, setTrendMetric] = useState('spend_amount');
  const [dashboard, setDashboard] = useState(EMPTY_DASHBOARD);

  const activeSection = useMemo(() => {
    const params = new URLSearchParams(location.search || '');
    const rawSection = (params.get('section') || '').trim().toLowerCase();
    return DASHBOARD_SECTIONS.includes(rawSection) ? rawSection : 'overview';
  }, [location.search]);

  const activeSectionTitle = t(DASHBOARD_SECTION_TITLES[activeSection]);

  const toUsd = useCallback(
    (yycAmount) => {
      const amount = convertYYCToDisplayAmount(
        yycAmount,
        'USD',
        displayCurrencyIndex,
      );
      if (!Number.isFinite(amount)) return 0;
      return amount;
    },
    [displayCurrencyIndex],
  );

  const formatUsd = useCallback(
    (yycAmount) => formatCompactDisplayAmount(toUsd(yycAmount)),
    [toUsd],
  );

  const periodOptions = useMemo(
    () =>
      PERIOD_OPTIONS.map((value) => ({
        key: value,
        value,
        text: t(`dashboard.spending.period.${value}`),
      })),
    [t],
  );

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/v1/admin/dashboard/', {
        params: { period, section: activeSection },
      });
      if (res.data?.success) {
        setDashboard(normalizeAdminDashboardPayload(res.data.data || {}));
      } else {
        setDashboard(EMPTY_DASHBOARD);
      }
    } catch (error) {
      console.error('Failed to load admin dashboard:', error);
      setDashboard(EMPTY_DASHBOARD);
    } finally {
      setLoading(false);
    }
  }, [activeSection, period]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const formatUpdatedAt = (value) => {
    if (!value) return '-';
    return new Date(Number(value) * 1000).toLocaleString('zh-CN', {
      hour12: false,
    });
  };

  const renderCapabilities = useCallback(
    (raw) => {
      if (!Array.isArray(raw) || raw.length === 0) return '-';
      return raw
        .map((item) =>
          t(`dashboard.admin.capabilities.${item}`, { defaultValue: item }),
        )
        .join(' / ');
    },
    [t],
  );

  const channelHealthData = useMemo(
    () =>
      (dashboard.top_channels || []).map((row, index) => ({
        id: row.id || `channel-${index}`,
        name: row.name || row.id || '-',
        status: Number(row.status || 0),
        capabilities: renderCapabilities(row.capabilities),
        health_score: Number(row.health_score || 0),
        health_level: row.health_level || 'unknown',
        pass_rate_percent: toPercent(row.pass_rate),
        coverage_rate_percent: toPercent(row.coverage_rate),
        avg_latency_ms: Number(row.avg_latency_ms || 0),
        selected_model_count: Number(row.selected_model_count || 0),
        tested_model_count: Number(row.tested_model_count || 0),
        tested_endpoint_count: Number(row.tested_endpoint_count || 0),
        has_test_data: Boolean(row.has_test_data),
        supported_count: Number(row.supported_count || 0),
        unsupported_count: Number(row.unsupported_count || 0),
        last_tested_at: Number(row.last_tested_at || 0),
      })),
    [dashboard.top_channels, renderCapabilities],
  );

  const channelHealthSummary = useMemo(() => {
    const rows = channelHealthData;
    if (rows.length === 0) {
      return {
        with_tests: 0,
        without_tests: 0,
        avg_pass_rate: 0,
        avg_coverage_rate: 0,
        avg_latency_ms: 0,
        needs_retest: 0,
      };
    }
    const withTestsRows = rows.filter((item) => item.has_test_data);
    const withoutTests = rows.length - withTestsRows.length;
    const avgPassRate =
      withTestsRows.length > 0
        ? withTestsRows.reduce(
            (sum, item) => sum + item.pass_rate_percent,
            0,
          ) / withTestsRows.length
        : 0;
    const selectedTotal = rows.reduce(
      (sum, item) => sum + item.selected_model_count,
      0,
    );
    const testedTotal = rows.reduce(
      (sum, item) => sum + item.tested_model_count,
      0,
    );
    const avgCoverageRate = selectedTotal > 0 ? (testedTotal / selectedTotal) * 100 : 0;
    const latencyRows = rows.filter((item) => item.avg_latency_ms > 0);
    const avgLatencyMs =
      latencyRows.length > 0
        ? Math.round(
            latencyRows.reduce((sum, item) => sum + item.avg_latency_ms, 0) /
              latencyRows.length,
          )
        : 0;
    const needsRetest = rows.filter(
      (item) =>
        item.selected_model_count > 0 &&
        (!item.has_test_data ||
          item.coverage_rate_percent < 100 ||
          item.pass_rate_percent < 100),
    ).length;
    return {
      with_tests: withTestsRows.length,
      without_tests: withoutTests,
      avg_pass_rate: avgPassRate,
      avg_coverage_rate: avgCoverageRate,
      avg_latency_ms: avgLatencyMs,
      needs_retest: needsRetest,
    };
  }, [channelHealthData]);

  const trendLineColor = useMemo(() => {
    switch (trendMetric) {
      case 'topup_amount':
        return '#16a34a';
      case 'request_count':
        return '#2563eb';
      case 'active_user_count':
        return '#9333ea';
      default:
        return '#ea580c';
    }
  }, [trendMetric]);

  const trendFormatter = (value) => {
    if (trendMetric === 'spend_amount' || trendMetric === 'topup_amount') {
      return formatUsd(value);
    }
    return formatCount(value);
  };

  const renderToolbar = () => (
    <div className='admin-dashboard-toolbar'>
      <div className='admin-dashboard-period'>
        <span className='admin-dashboard-period-label'>
          {t('dashboard.admin.period.label')}
        </span>
        <Dropdown
          className='router-section-dropdown'
          selection
          options={periodOptions}
          value={period}
          onChange={(e, { value }) => setPeriod(value)}
        />
      </div>
      <div className='admin-dashboard-toolbar-right'>
        <span className='admin-dashboard-updated'>
          {t('dashboard.admin.updated_at', {
            time: formatUpdatedAt(dashboard.generated_at),
          })}
        </span>
        <Button
          className='router-inline-button'
          type='button'
          loading={loading}
          onClick={loadData}
        >
          {t('dashboard.admin.buttons.refresh')}
        </Button>
      </div>
    </div>
  );

  const renderOverviewSection = () => (
    <Card fluid className='chart-card'>
      <Card.Content>
        <Card.Header className='router-card-header router-section-title'>
          {activeSectionTitle}
        </Card.Header>
        {renderToolbar()}
        <div className='admin-dashboard-kpi-grid'>
          <div className='admin-dashboard-kpi-item'>
            <div className='admin-dashboard-kpi-label'>
              {t('dashboard.admin.metrics.consume')}
            </div>
            <div className='admin-dashboard-kpi-value'>
              {formatUsd(dashboard.summary.spend_amount)}
            </div>
          </div>
          <div className='admin-dashboard-kpi-item'>
            <div className='admin-dashboard-kpi-label'>
              {t('dashboard.admin.metrics.topup')}
            </div>
            <div className='admin-dashboard-kpi-value'>
              {formatUsd(dashboard.summary.topup_amount)}
            </div>
          </div>
          <div className='admin-dashboard-kpi-item'>
            <div className='admin-dashboard-kpi-label'>
              {t('dashboard.admin.metrics.net')}
            </div>
            <div className='admin-dashboard-kpi-value'>
              {formatUsd(dashboard.summary.net_amount)}
            </div>
          </div>
          <div className='admin-dashboard-kpi-item'>
            <div className='admin-dashboard-kpi-label'>
              {t('dashboard.admin.metrics.request_count')}
            </div>
            <div className='admin-dashboard-kpi-value'>
              {formatCount(dashboard.summary.request_count)}
            </div>
          </div>
          <div className='admin-dashboard-kpi-item'>
            <div className='admin-dashboard-kpi-label'>
              {t('dashboard.admin.metrics.active_user_count')}
            </div>
            <div className='admin-dashboard-kpi-value'>
              {formatCount(dashboard.summary.active_user_count)}
            </div>
          </div>
          <div className='admin-dashboard-kpi-item'>
            <div className='admin-dashboard-kpi-label'>
              {t('dashboard.admin.metrics.channels')}
            </div>
            <div className='admin-dashboard-kpi-value'>
              {dashboard.summary.channel_enabled} / {dashboard.summary.channel_total}
            </div>
          </div>
          <div className='admin-dashboard-kpi-item'>
            <div className='admin-dashboard-kpi-label'>
              {t('dashboard.admin.metrics.channel_disabled')}
            </div>
            <div className='admin-dashboard-kpi-value'>
              {formatCount(dashboard.summary.channel_disabled)}
            </div>
          </div>
          <div className='admin-dashboard-kpi-item'>
            <div className='admin-dashboard-kpi-label'>
              {t('dashboard.admin.metrics.groups')}
            </div>
            <div className='admin-dashboard-kpi-value'>
              {formatCount(dashboard.summary.group_total)}
            </div>
          </div>
          <div className='admin-dashboard-kpi-item'>
            <div className='admin-dashboard-kpi-label'>
              {t('dashboard.admin.metrics.providers')}
            </div>
            <div className='admin-dashboard-kpi-value'>
              {formatCount(dashboard.summary.provider_total)}
            </div>
          </div>
          <div className='admin-dashboard-kpi-item'>
            <div className='admin-dashboard-kpi-label'>
              {t('dashboard.admin.metrics.tasks_active')}
            </div>
            <div className='admin-dashboard-kpi-value'>
              {formatCount(dashboard.summary.task_active_total)}
            </div>
          </div>
          <div className='admin-dashboard-kpi-item'>
            <div className='admin-dashboard-kpi-label'>
              {t('dashboard.admin.metrics.tasks_failed')}
            </div>
            <div className='admin-dashboard-kpi-value'>
              {formatCount(dashboard.summary.task_failed_total)}
            </div>
          </div>
        </div>
      </Card.Content>
    </Card>
  );

  const renderTrendSection = () => (
    <Card fluid className='chart-card admin-dashboard-section'>
      <Card.Content>
        <Card.Header className='router-card-header router-section-title'>
          {activeSectionTitle}
        </Card.Header>
        {renderToolbar()}
        <div className='admin-dashboard-trend-toolbar'>
          <Button.Group>
            {TREND_METRIC_OPTIONS.map((metric) => (
              <Button
                key={metric}
                className='router-inline-button'
                active={trendMetric === metric}
                onClick={() => setTrendMetric(metric)}
              >
                {t(`dashboard.admin.trend.metrics.${metric}`)}
              </Button>
            ))}
          </Button.Group>
        </div>
        {dashboard.trend.length === 0 ? (
          <div className='admin-dashboard-empty'>
            {t('dashboard.admin.empty.trend')}
          </div>
        ) : (
          <div className='chart-container'>
            <ResponsiveContainer width='100%' height={240}>
              <LineChart data={dashboard.trend}>
                <CartesianGrid
                  strokeDasharray='3 3'
                  vertical={false}
                  opacity={0.1}
                />
                <XAxis
                  dataKey='bucket'
                  axisLine={false}
                  tickLine={false}
                  tick={{ fontSize: 12, fill: '#A3AED0' }}
                  minTickGap={8}
                />
                <YAxis
                  axisLine={false}
                  tickLine={false}
                  tick={{ fontSize: 12, fill: '#A3AED0' }}
                />
                <Tooltip
                  contentStyle={{
                    background: '#fff',
                    border: 'none',
                    borderRadius: '4px',
                    boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
                  }}
                  formatter={(value) => [
                    trendFormatter(value),
                    t(`dashboard.admin.trend.metrics.${trendMetric}`),
                  ]}
                  labelFormatter={(label) =>
                    `${t('dashboard.statistics.tooltip.date')}: ${label}`
                  }
                />
                <Line
                  type='monotone'
                  dataKey={trendMetric}
                  stroke={trendLineColor}
                  strokeWidth={2}
                  dot={false}
                  activeDot={{ r: 4 }}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        )}
      </Card.Content>
    </Card>
  );

  const renderHealthSection = () => (
    <Card fluid className='chart-card admin-dashboard-section'>
      <Card.Content>
        <Card.Header className='router-card-header router-section-title'>
          {activeSectionTitle}
        </Card.Header>
        {renderToolbar()}
        {channelHealthData.length === 0 ? (
          <div className='admin-dashboard-empty'>
            {t('dashboard.admin.empty.channels')}
          </div>
        ) : (
          <>
            <div className='admin-dashboard-health-summary-grid'>
              <div className='admin-dashboard-kpi-item'>
                <div className='admin-dashboard-kpi-label'>
                  {t('dashboard.admin.health.summary.with_tests')}
                </div>
                <div className='admin-dashboard-kpi-value'>
                  {formatCount(channelHealthSummary.with_tests)}
                </div>
              </div>
              <div className='admin-dashboard-kpi-item'>
                <div className='admin-dashboard-kpi-label'>
                  {t('dashboard.admin.health.summary.without_tests')}
                </div>
                <div className='admin-dashboard-kpi-value'>
                  {formatCount(channelHealthSummary.without_tests)}
                </div>
              </div>
              <div className='admin-dashboard-kpi-item'>
                <div className='admin-dashboard-kpi-label'>
                  {t('dashboard.admin.health.summary.avg_pass_rate')}
                </div>
                <div className='admin-dashboard-kpi-value'>
                  {formatPercent(channelHealthSummary.avg_pass_rate)}
                </div>
              </div>
              <div className='admin-dashboard-kpi-item'>
                <div className='admin-dashboard-kpi-label'>
                  {t('dashboard.admin.health.summary.avg_coverage_rate')}
                </div>
                <div className='admin-dashboard-kpi-value'>
                  {formatPercent(channelHealthSummary.avg_coverage_rate)}
                </div>
              </div>
              <div className='admin-dashboard-kpi-item'>
                <div className='admin-dashboard-kpi-label'>
                  {t('dashboard.admin.health.summary.avg_latency')}
                </div>
                <div className='admin-dashboard-kpi-value'>
                  {channelHealthSummary.avg_latency_ms > 0
                    ? `${channelHealthSummary.avg_latency_ms} ms`
                    : '-'}
                </div>
              </div>
              <div className='admin-dashboard-kpi-item'>
                <div className='admin-dashboard-kpi-label'>
                  {t('dashboard.admin.health.summary.needs_retest')}
                </div>
                <div className='admin-dashboard-kpi-value'>
                  {formatCount(channelHealthSummary.needs_retest)}
                </div>
              </div>
            </div>
            <div className='chart-container admin-dashboard-health-chart'>
              <ResponsiveContainer width='100%' height={300}>
                <BarChart
                  data={channelHealthData}
                  margin={{ top: 8, right: 20, left: 0, bottom: 0 }}
                >
                  <CartesianGrid
                    strokeDasharray='3 3'
                    vertical={false}
                    opacity={0.1}
                  />
                  <XAxis
                    dataKey='name'
                    axisLine={false}
                    tickLine={false}
                    tick={{ fontSize: 12, fill: '#A3AED0' }}
                    minTickGap={8}
                  />
                  <YAxis
                    yAxisId='score'
                    domain={[0, 100]}
                    axisLine={false}
                    tickLine={false}
                    tick={{ fontSize: 12, fill: '#A3AED0' }}
                  />
                  <YAxis
                    yAxisId='latency'
                    orientation='right'
                    axisLine={false}
                    tickLine={false}
                    tick={{ fontSize: 12, fill: '#A3AED0' }}
                  />
                  <Tooltip
                    contentStyle={{
                      background: '#fff',
                      border: 'none',
                      borderRadius: '4px',
                      boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
                    }}
                    formatter={(value, name) => {
                      if (name === 'health_score') {
                        return [
                          `${Number(value).toFixed(0)}`,
                          t('dashboard.admin.health.chart.health_score'),
                        ];
                      }
                      if (name === 'pass_rate_percent') {
                        return [
                          `${Number(value).toFixed(1)}%`,
                          t('dashboard.admin.health.chart.pass_rate'),
                        ];
                      }
                      if (name === 'coverage_rate_percent') {
                        return [
                          `${Number(value).toFixed(1)}%`,
                          t('dashboard.admin.health.chart.coverage_rate'),
                        ];
                      }
                      if (name === 'avg_latency_ms') {
                        return [
                          `${Number(value).toFixed(0)} ms`,
                          t('dashboard.admin.health.chart.avg_latency'),
                        ];
                      }
                      return [String(value ?? '-'), String(name)];
                    }}
                    labelFormatter={(label, payload) => {
                      if (!Array.isArray(payload) || payload.length === 0) {
                        return label;
                      }
                      const entry = payload[0]?.payload || {};
                      const statusText = t(
                        `dashboard.admin.channel_status.${Number(entry.status)}`,
                        {
                          defaultValue: t(
                            'dashboard.admin.channel_status.default',
                          ),
                        },
                      );
                      const healthLevelText = t(
                        `dashboard.admin.health.level.${
                          entry.health_level || 'unknown'
                        }`,
                        {
                          defaultValue: t(
                            'dashboard.admin.health.level.unknown',
                          ),
                        },
                      );
                      const lastTested = entry.last_tested_at
                        ? formatUpdatedAt(entry.last_tested_at)
                        : '-';
                      const capabilitiesText = entry.capabilities || '-';
                      return `${label} | ${statusText} | ${healthLevelText} | ${t('dashboard.admin.table.capabilities')}: ${capabilitiesText} | ${t('dashboard.admin.health.chart.last_tested')}: ${lastTested}`;
                    }}
                  />
                  <Bar
                    yAxisId='score'
                    dataKey='health_score'
                    name='health_score'
                    radius={[4, 4, 0, 0]}
                    fill='#60a5fa'
                  >
                    {channelHealthData.map((item) => (
                      <Cell
                        key={item.id}
                        fill={
                          HEALTH_LEVEL_COLORS[item.health_level] ||
                          HEALTH_LEVEL_COLORS.unknown
                        }
                      />
                    ))}
                  </Bar>
                  <Line
                    yAxisId='score'
                    type='monotone'
                    dataKey='pass_rate_percent'
                    name='pass_rate_percent'
                    stroke='#16a34a'
                    strokeWidth={2}
                    dot={false}
                  />
                  <Line
                    yAxisId='score'
                    type='monotone'
                    dataKey='coverage_rate_percent'
                    name='coverage_rate_percent'
                    stroke='#2563eb'
                    strokeWidth={2}
                    dot={false}
                  />
                  <Line
                    yAxisId='latency'
                    type='monotone'
                    dataKey='avg_latency_ms'
                    name='avg_latency_ms'
                    stroke='#ef4444'
                    strokeWidth={2}
                    dot={false}
                  />
                </BarChart>
              </ResponsiveContainer>
            </div>
            <div className='admin-dashboard-health-legend'>
              <span>{t('dashboard.admin.health.chart.legend_health')}</span>
              <span>{t('dashboard.admin.health.chart.legend_pass')}</span>
              <span>{t('dashboard.admin.health.chart.legend_coverage')}</span>
              <span>{t('dashboard.admin.health.chart.legend_latency')}</span>
            </div>
          </>
        )}
      </Card.Content>
    </Card>
  );

  return (
    <div className='dashboard-container admin-dashboard-container'>
      {activeSection === 'overview' ? renderOverviewSection() : null}
      {activeSection === 'trend' ? renderTrendSection() : null}
      {activeSection === 'health' ? renderHealthSection() : null}
    </div>
  );
};

export default AdminDashboard;
