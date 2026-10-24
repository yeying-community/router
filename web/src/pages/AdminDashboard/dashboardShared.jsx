import { useCallback, useEffect, useMemo, useState } from 'react';
import { API } from '../../helpers/api';
import { showError } from '../../helpers/utils';
import {
  buildPublicDisplayCurrencyIndex,
  convertChargeAmountToDisplayAmount,
  formatCompactDisplayAmount,
} from '../../helpers/billing';
import {
  formatBillingPercent,
  BILLING_PERCENT_DECIMALS,
  chartCategoricalPalette,
  chartNeutralColor,
  chartStatusPalette,
} from '../../router-ui';

// Shared, closure-free building blocks for the admin dashboard surfaces.
//
// These constants and pure helpers were previously inlined in AdminDashboard's
// index.jsx. They are extracted here so the per-section shells (channel health /
// user analytics / model operations) can reuse them without dragging in the
// whole dashboard component. Anything that closes over component state stays in
// the components; only pure values/functions and the data-fetch hook live here.

export const PERIOD_OPTIONS = [
  'today',
  'last_7_days',
  'last_30_days',
  'this_month',
  'last_month',
  'this_year',
  'all_time',
];

export const TREND_METRIC_OPTIONS = [
  'spend_amount',
  'topup_amount',
  'request_count',
  'active_user_count',
];

export const USER_GROWTH_GRANULARITY_OPTIONS = ['week', 'month'];
export const USER_GROWTH_LINE_KEYS = ['new_user_count', 'active_user_count', 'topup_user_count'];
export const USER_SEGMENT_FOCUS_LIMIT = 100;

export const DASHBOARD_SECTIONS = ['spending', 'channels', 'users', 'models'];
export const DASHBOARD_SECTION_TITLES = {
  spending: 'dashboard.admin.nav.spending',
  channels: 'dashboard.admin.nav.channels',
  users: 'dashboard.admin.nav.users',
  models: 'dashboard.admin.nav.models',
};

export const MODEL_SORT_OPTIONS = [
  'spend',
  'requests',
  'health',
  'latency',
];

export const EMPTY_SUMMARY = {
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

export const EMPTY_CHANNEL_HEALTH_SUMMARY = {
  with_tests: 0,
  without_tests: 0,
  avg_pass_rate: 0,
  avg_coverage_rate: 0,
  avg_latency_ms: 0,
  needs_retest: 0,
  risk_count: 0,
  active_circuit_breaker_count: 0,
  high_latency_count: 0,
  low_balance_channel_count: 0,
};

export const EMPTY_USER_GROWTH_COMPARISON = {
  current: 0,
  previous: 0,
  delta: 0,
  growth_rate: 0,
  has_baseline: false,
};

export const EMPTY_USER_GROWTH_SUMMARY = {
  granularity: 'week',
  current: {
    bucket: '',
    start_timestamp: 0,
    end_timestamp: 0,
    new_user_count: 0,
    active_user_count: 0,
    topup_user_count: 0,
    request_count: 0,
  },
  previous: {
    bucket: '',
    start_timestamp: 0,
    end_timestamp: 0,
    new_user_count: 0,
    active_user_count: 0,
    topup_user_count: 0,
    request_count: 0,
  },
  new_users: EMPTY_USER_GROWTH_COMPARISON,
  active_users: EMPTY_USER_GROWTH_COMPARISON,
  topup_users: EMPTY_USER_GROWTH_COMPARISON,
};

export const EMPTY_DASHBOARD = {
  period: 'last_7_days',
  granularity: 'day',
  start_timestamp: 0,
  end_timestamp: 0,
  summary: EMPTY_SUMMARY,
  trend: [],
  top_channels: [],
  usage_summary: {
    user_count: 0,
    request_count: 0,
    total_tokens: 0,
    spend_amount: 0,
    top_username: '',
    top_user_share: 0,
  },
  usage_totals: {
    user_count: 0,
    request_count: 0,
    total_tokens: 0,
    spend_amount: 0,
  },
  usage_rank: [],
  user_growth_summary: EMPTY_USER_GROWTH_SUMMARY,
  user_growth_trend: [],
  model_summary: {
    selected_model_count: 0,
    tested_model_count: 0,
    healthy_model_count: 0,
    warning_model_count: 0,
    critical_model_count: 0,
    request_count: 0,
    total_tokens: 0,
    spend_amount: 0,
    avg_pass_rate: 0,
    avg_latency_ms: 0,
  },
  channel_health_summary: EMPTY_CHANNEL_HEALTH_SUMMARY,
  top_models: [],
  generated_at: 0,
};

export const HEALTH_LEVEL_COLORS = {
  healthy: chartStatusPalette.success,
  warning: chartStatusPalette.warning,
  critical: chartStatusPalette.danger,
  unknown: chartNeutralColor(),
};

export const CHANNEL_HEALTH_HISTORY_SIZE = 60;

export const CHANNEL_HEALTH_POINT_COLORS = {
  success: chartStatusPalette.success,
  warning: chartStatusPalette.warning,
  failure: chartStatusPalette.danger,
  unknown: chartNeutralColor(),
};

export const USER_GROWTH_LINE_COLORS = {
  new_user_count: chartCategoricalPalette[0],
  active_user_count: chartCategoricalPalette[2],
  topup_user_count: chartCategoricalPalette[1],
};

export const ACTIVE_CIRCUIT_BREAKER_STATES = new Set(['open', 'half_open']);

export const formatCount = (value) => {
  const num = Number(value || 0);
  if (!Number.isFinite(num)) return '0';
  return num.toLocaleString('zh-CN');
};

export const toPercent = (raw) => {
  const value = Number(raw || 0);
  if (!Number.isFinite(value)) return 0;
  if (value <= 1) return value * 100;
  return value;
};

export const formatPercent = (raw) =>
  formatBillingPercent(toPercent(raw), BILLING_PERCENT_DECIMALS);

export const formatUpdatedAt = (value) => {
  if (!value) return '-';
  return new Date(Number(value) * 1000).toLocaleString('zh-CN', {
    hour12: false,
  });
};

export const formatTimeRange = (start, end) => {
  const startTs = Number(start || 0);
  const endTs = Number(end || 0);
  if (!startTs || !endTs) return '-';
  return `${formatUpdatedAt(startTs)} - ${formatUpdatedAt(endTs)}`;
};

export const formatPeriodRange = (startTimestamp, endTimestamp) => {
  const start = Number(startTimestamp || 0);
  const end = Number(endTimestamp || 0);
  if (!start || !end) return '-';
  const formatDate = (timestamp) =>
    new Date(timestamp * 1000).toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    });
  return `${formatDate(start)} - ${formatDate(end)}`;
};

export const formatSignedCount = (value) => {
  const num = Number(value || 0);
  if (!Number.isFinite(num) || num === 0) return '0';
  return `${num > 0 ? '+' : '-'}${formatCount(Math.abs(num))}`;
};

export const formatSignedPercent = (value) => {
  const num = Number(value || 0);
  if (!Number.isFinite(num) || num === 0) return '0.00%';
  return `${num > 0 ? '+' : '-'}${(Math.abs(num) * 100).toFixed(2)}%`;
};

export const deltaTone = (value) => {
  const num = Number(value || 0);
  if (!Number.isFinite(num) || num === 0) return 'neutral';
  return num > 0 ? 'positive' : 'negative';
};

export const normalizeChannelHealthPointState = (point) => {
  const raw = typeof point === 'string' ? point : point?.state;
  const normalized = String(raw || '').trim().toLowerCase();
  if (normalized === 'success' || normalized === 'ok') return 'success';
  if (normalized === 'warning') return 'warning';
  if (normalized === 'failure' || normalized === 'failed' || normalized === 'error') {
    return 'failure';
  }
  return 'unknown';
};

export const normalizeAdminDashboardPayload = (payload) => {
  const summary = payload?.summary || {};
  const trend = Array.isArray(payload?.trend) ? payload.trend : [];
  const topChannels = Array.isArray(payload?.top_channels)
    ? payload.top_channels
    : [];
  const usageSummary = payload?.usage_summary || {};
  const usageTotals = payload?.usage_totals || {};
  const usageRank = Array.isArray(payload?.usage_rank) ? payload.usage_rank : [];
  const userGrowthSummary = payload?.user_growth_summary || {};
  const userGrowthTrend = Array.isArray(payload?.user_growth_trend)
    ? payload.user_growth_trend
    : [];
  const modelSummary = payload?.model_summary || {};
  const channelHealthSummary = payload?.channel_health_summary || {};
  const topModels = Array.isArray(payload?.top_models) ? payload.top_models : [];
  return {
    ...EMPTY_DASHBOARD,
    ...(payload || {}),
    summary: {
      ...EMPTY_SUMMARY,
      ...summary,
      spend_amount: Number(summary?.consume_amount ?? summary?.consume_quota ?? 0),
      topup_amount: Number(summary?.topup_amount ?? summary?.topup_quota ?? 0),
      net_amount: Number(summary?.net_amount ?? summary?.net_quota ?? 0),
    },
    trend: trend.map((item) => ({
      ...item,
      spend_amount: Number(item?.consume_amount ?? item?.consume_quota ?? 0),
      topup_amount: Number(item?.topup_amount ?? item?.topup_quota ?? 0),
    })),
    top_channels: topChannels.map((item) => ({
      ...item,
      usedYyc: Number(item?.used_amount ?? item?.used_quota ?? 0),
      circuit_breaker:
        item?.circuit_breaker && typeof item.circuit_breaker === 'object'
          ? item.circuit_breaker
          : null,
      health_points: Array.isArray(item?.health_points)
        ? item.health_points.map((point) => ({
            ...point,
            state: normalizeChannelHealthPointState(point),
            bucket_start: Number(point?.bucket_start || 0),
            bucket_end: Number(point?.bucket_end || 0),
            success_count: Number(point?.success_count || 0),
            failure_count: Number(point?.failure_count || 0),
            total_count: Number(point?.total_count || 0),
            avg_latency_ms: Number(point?.avg_latency_ms || 0),
            pass_rate: Number(point?.pass_rate || 0),
          }))
        : [],
    })),
    channel_health_summary: {
      with_tests: Number(channelHealthSummary?.with_tests || 0),
      without_tests: Number(channelHealthSummary?.without_tests || 0),
      avg_pass_rate: Number(channelHealthSummary?.avg_pass_rate || 0),
      avg_coverage_rate: Number(channelHealthSummary?.avg_coverage_rate || 0),
      avg_latency_ms: Number(channelHealthSummary?.avg_latency_ms || 0),
      needs_retest: Number(channelHealthSummary?.needs_retest || 0),
      risk_count: Number(channelHealthSummary?.risk_count || 0),
      active_circuit_breaker_count: Number(
        channelHealthSummary?.active_circuit_breaker_count || 0,
      ),
      high_latency_count: Number(channelHealthSummary?.high_latency_count || 0),
      low_balance_channel_count: Number(
        channelHealthSummary?.low_balance_channel_count || 0,
      ),
    },
    usage_summary: {
      user_count: Number(usageSummary?.user_count || 0),
      request_count: Number(usageSummary?.request_count || 0),
      total_tokens: Number(usageSummary?.total_tokens || 0),
      spend_amount: Number(usageSummary?.spend_amount ?? usageSummary?.spend_quota ?? 0),
      top_username: String(usageSummary?.top_username || ''),
      top_user_share: Number(usageSummary?.top_user_share || 0),
    },
    usage_totals: {
      user_count: Number(usageTotals?.user_count || 0),
      request_count: Number(usageTotals?.request_count || 0),
      total_tokens: Number(usageTotals?.total_tokens || 0),
      spend_amount: Number(usageTotals?.spend_amount ?? usageTotals?.spend_quota ?? 0),
    },
    usage_rank: usageRank.map((item) => ({
      ...item,
      request_count: Number(item?.request_count || 0),
      total_tokens: Number(item?.total_tokens || 0),
      spend_amount: Number(item?.spend_amount ?? item?.spend_quota ?? 0),
      balance_amount: Number(item?.balance_amount || 0),
      share_rate: Number(item?.share_rate || 0),
      last_used_at: Number(item?.last_used_at || 0),
    })),
    user_growth_summary: {
      ...EMPTY_USER_GROWTH_SUMMARY,
      ...(userGrowthSummary || {}),
      current: {
        ...EMPTY_USER_GROWTH_SUMMARY.current,
        ...(userGrowthSummary?.current || {}),
        new_user_count: Number(userGrowthSummary?.current?.new_user_count || 0),
        active_user_count: Number(userGrowthSummary?.current?.active_user_count || 0),
        topup_user_count: Number(userGrowthSummary?.current?.topup_user_count || 0),
        request_count: Number(userGrowthSummary?.current?.request_count || 0),
        start_timestamp: Number(userGrowthSummary?.current?.start_timestamp || 0),
        end_timestamp: Number(userGrowthSummary?.current?.end_timestamp || 0),
      },
      previous: {
        ...EMPTY_USER_GROWTH_SUMMARY.previous,
        ...(userGrowthSummary?.previous || {}),
        new_user_count: Number(userGrowthSummary?.previous?.new_user_count || 0),
        active_user_count: Number(userGrowthSummary?.previous?.active_user_count || 0),
        topup_user_count: Number(userGrowthSummary?.previous?.topup_user_count || 0),
        request_count: Number(userGrowthSummary?.previous?.request_count || 0),
        start_timestamp: Number(userGrowthSummary?.previous?.start_timestamp || 0),
        end_timestamp: Number(userGrowthSummary?.previous?.end_timestamp || 0),
      },
      new_users: {
        ...EMPTY_USER_GROWTH_COMPARISON,
        ...(userGrowthSummary?.new_users || {}),
        current: Number(userGrowthSummary?.new_users?.current || 0),
        previous: Number(userGrowthSummary?.new_users?.previous || 0),
        delta: Number(userGrowthSummary?.new_users?.delta || 0),
        growth_rate: Number(userGrowthSummary?.new_users?.growth_rate || 0),
        has_baseline: Boolean(userGrowthSummary?.new_users?.has_baseline),
      },
      active_users: {
        ...EMPTY_USER_GROWTH_COMPARISON,
        ...(userGrowthSummary?.active_users || {}),
        current: Number(userGrowthSummary?.active_users?.current || 0),
        previous: Number(userGrowthSummary?.active_users?.previous || 0),
        delta: Number(userGrowthSummary?.active_users?.delta || 0),
        growth_rate: Number(userGrowthSummary?.active_users?.growth_rate || 0),
        has_baseline: Boolean(userGrowthSummary?.active_users?.has_baseline),
      },
      topup_users: {
        ...EMPTY_USER_GROWTH_COMPARISON,
        ...(userGrowthSummary?.topup_users || {}),
        current: Number(userGrowthSummary?.topup_users?.current || 0),
        previous: Number(userGrowthSummary?.topup_users?.previous || 0),
        delta: Number(userGrowthSummary?.topup_users?.delta || 0),
        growth_rate: Number(userGrowthSummary?.topup_users?.growth_rate || 0),
        has_baseline: Boolean(userGrowthSummary?.topup_users?.has_baseline),
      },
    },
    user_growth_trend: userGrowthTrend.map((item) => ({
      ...item,
      start_timestamp: Number(item?.start_timestamp || 0),
      end_timestamp: Number(item?.end_timestamp || 0),
      new_user_count: Number(item?.new_user_count || 0),
      active_user_count: Number(item?.active_user_count || 0),
      topup_user_count: Number(item?.topup_user_count || 0),
      request_count: Number(item?.request_count || 0),
    })),
    model_summary: {
      selected_model_count: Number(modelSummary?.selected_model_count || 0),
      tested_model_count: Number(modelSummary?.tested_model_count || 0),
      healthy_model_count: Number(modelSummary?.healthy_model_count || 0),
      warning_model_count: Number(modelSummary?.warning_model_count || 0),
      critical_model_count: Number(modelSummary?.critical_model_count || 0),
      request_count: Number(modelSummary?.request_count || 0),
      total_tokens: Number(modelSummary?.total_tokens || 0),
      spend_amount: Number(modelSummary?.spend_amount ?? modelSummary?.spend_quota ?? 0),
      avg_pass_rate: Number(modelSummary?.avg_pass_rate || 0),
      avg_latency_ms: Number(modelSummary?.avg_latency_ms || 0),
    },
    top_models: topModels.map((item) => ({
      ...item,
      request_count: Number(item?.request_count || 0),
      total_tokens: Number(item?.total_tokens || 0),
      spend_amount: Number(item?.spend_amount ?? item?.spend_quota ?? 0),
      channel_count: Number(item?.channel_count || 0),
      tested_channel_count: Number(item?.tested_channel_count || 0),
      tested_endpoint_count: Number(item?.tested_endpoint_count || 0),
      supported_count: Number(item?.supported_count || 0),
      unsupported_count: Number(item?.unsupported_count || 0),
      supported_endpoint_count: Number(item?.supported_endpoint_count || 0),
      pass_rate: Number(item?.pass_rate || 0),
      avg_latency_ms: Number(item?.avg_latency_ms || 0),
      health_score: Number(item?.health_score || 0),
      last_tested_at: Number(item?.last_tested_at || 0),
      tags: Array.isArray(item?.tags) ? item.tags : [],
    })),
  };
};

export const buildChannelHealthHistory = (points) => {
  const normalized = Array.isArray(points)
    ? points.slice(-CHANNEL_HEALTH_HISTORY_SIZE).map((point) => ({
        ...point,
        state: normalizeChannelHealthPointState(point),
        observed: Number(point?.total_count || 0) > 0,
      }))
    : [];
  const paddingCount = Math.max(
    0,
    CHANNEL_HEALTH_HISTORY_SIZE - normalized.length,
  );
  const history = [
    ...Array.from({ length: paddingCount }, () => ({
      state: 'unknown',
      observed: false,
    })),
    ...normalized,
  ];
  return history.map((point, index) => ({
    ...point,
    key: `${index}-${point.state}-${point.bucket_start || 0}`,
  }));
};

export const normalizeCircuitBreakerState = (raw) =>
  String(raw || '').trim().toLowerCase();

export const isActiveCircuitBreaker = (circuitBreaker) =>
  ACTIVE_CIRCUIT_BREAKER_STATES.has(
    normalizeCircuitBreakerState(circuitBreaker?.state),
  );

// Dashboard figures are shown in USD via a fixed fallback charge rate rather
// than the runtime currency table: passing [] makes buildPublicDisplayCurrencyIndex
// fall back to getFallbackUSDChargeRate(), so this is intentional (no runtime
// currency load needed for the operator's USD-only overview).
export const useUsdFormatter = () => {
  const displayCurrencyIndex = useMemo(
    () => buildPublicDisplayCurrencyIndex([]),
    [],
  );
  const toUsd = useCallback(
    (chargeAmount) => {
      const amount = convertChargeAmountToDisplayAmount(
        chargeAmount,
        'USD',
        displayCurrencyIndex,
      );
      if (!Number.isFinite(amount)) return 0;
      return amount;
    },
    [displayCurrencyIndex],
  );
  const formatUsd = useCallback(
    (chargeAmount) => formatCompactDisplayAmount(toUsd(chargeAmount)),
    [toUsd],
  );
  return { toUsd, formatUsd };
};

// Fetch + normalize one dashboard section. Mirrors the original loadData:
// GET /api/v1/admin/dashboard/ with { period, section, ...extraParams }.
// `extraParams` carries the users section's user_keyword / user_growth_granularity.
// `enabled: false` turns the hook into a no-op (no fetch, no effect) so a caller
// can satisfy the rules-of-hooks while sourcing the data from elsewhere (e.g. a
// parent that already loaded this section and injects it — avoids a duplicate
// request).
export const useAdminDashboardData = (
  section,
  { period, extraParams } = {},
  { enabled = true } = {},
) => {
  const [dashboard, setDashboard] = useState(EMPTY_DASHBOARD);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const extraParamsKey = JSON.stringify(extraParams || {});

  const reload = useCallback(async () => {
    if (!enabled) {
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const params = { period, section, ...(extraParams || {}) };
      const res = await API.get('/api/v1/admin/dashboard/', { params });
      if (res.data?.success) {
        setDashboard(normalizeAdminDashboardPayload(res.data.data || {}));
      } else {
        setDashboard(EMPTY_DASHBOARD);
        setError(new Error(res.data?.message || 'Failed to load admin dashboard'));
      }
    } catch (error) {
      console.error('Failed to load admin dashboard:', error);
      showError(error);
      setDashboard(EMPTY_DASHBOARD);
      setError(error);
    } finally {
      setLoading(false);
    }
    // extraParamsKey stands in for the extraParams object identity.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [section, period, extraParamsKey, enabled]);

  useEffect(() => {
    reload();
  }, [reload]);

  return { dashboard, loading, error, reload };
};
