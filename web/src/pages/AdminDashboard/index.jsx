import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
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
import {
  AppButton,
  AppFilterHeader,
  AppInput,
  AppSegmented,
  AppSection,
  AppSelect,
  AppTag,
  AppTable,
  AppTooltip,
  AppToolbar,
} from '../../router-ui';
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

const USAGE_RANK_PERIOD_OPTIONS = ['today', 'last_7_days', 'this_month', 'all_time'];

const TREND_METRIC_OPTIONS = [
  'spend_amount',
  'topup_amount',
  'request_count',
  'active_user_count',
];

const DASHBOARD_SECTIONS = ['spending', 'channels', 'users', 'models'];
const DASHBOARD_SECTION_TITLES = {
  spending: 'dashboard.admin.nav.spending',
  channels: 'dashboard.admin.nav.channels',
  users: 'dashboard.admin.nav.users',
  models: 'dashboard.admin.nav.models',
};

const MODEL_SORT_OPTIONS = [
  'spend',
  'requests',
  'health',
  'latency',
];

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
  usage_summary: {
    user_count: 0,
    request_count: 0,
    total_tokens: 0,
    spend_yyc: 0,
    top_username: '',
    top_user_share: 0,
  },
  usage_totals: {
    user_count: 0,
    request_count: 0,
    total_tokens: 0,
    spend_yyc: 0,
  },
  usage_rank: [],
  model_summary: {
    selected_model_count: 0,
    tested_model_count: 0,
    healthy_model_count: 0,
    warning_model_count: 0,
    critical_model_count: 0,
    request_count: 0,
    total_tokens: 0,
    spend_yyc: 0,
    avg_pass_rate: 0,
    avg_latency_ms: 0,
  },
  top_models: [],
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
  const usageSummary = payload?.usage_summary || {};
  const usageTotals = payload?.usage_totals || {};
  const usageRank = Array.isArray(payload?.usage_rank) ? payload.usage_rank : [];
  const modelSummary = payload?.model_summary || {};
  const topModels = Array.isArray(payload?.top_models) ? payload.top_models : [];
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
    usage_summary: {
      user_count: Number(usageSummary?.user_count || 0),
      request_count: Number(usageSummary?.request_count || 0),
      total_tokens: Number(usageSummary?.total_tokens || 0),
      spend_yyc: Number(usageSummary?.spend_yyc ?? usageSummary?.spend_quota ?? 0),
      top_username: String(usageSummary?.top_username || ''),
      top_user_share: Number(usageSummary?.top_user_share || 0),
    },
    usage_totals: {
      user_count: Number(usageTotals?.user_count || 0),
      request_count: Number(usageTotals?.request_count || 0),
      total_tokens: Number(usageTotals?.total_tokens || 0),
      spend_yyc: Number(usageTotals?.spend_yyc ?? usageTotals?.spend_quota ?? 0),
    },
    usage_rank: usageRank.map((item) => ({
      ...item,
      request_count: Number(item?.request_count || 0),
      total_tokens: Number(item?.total_tokens || 0),
      spend_yyc: Number(item?.spend_yyc ?? item?.spend_quota ?? 0),
      share_rate: Number(item?.share_rate || 0),
      last_used_at: Number(item?.last_used_at || 0),
    })),
    model_summary: {
      selected_model_count: Number(modelSummary?.selected_model_count || 0),
      tested_model_count: Number(modelSummary?.tested_model_count || 0),
      healthy_model_count: Number(modelSummary?.healthy_model_count || 0),
      warning_model_count: Number(modelSummary?.warning_model_count || 0),
      critical_model_count: Number(modelSummary?.critical_model_count || 0),
      request_count: Number(modelSummary?.request_count || 0),
      total_tokens: Number(modelSummary?.total_tokens || 0),
      spend_yyc: Number(modelSummary?.spend_yyc ?? modelSummary?.spend_quota ?? 0),
      avg_pass_rate: Number(modelSummary?.avg_pass_rate || 0),
      avg_latency_ms: Number(modelSummary?.avg_latency_ms || 0),
    },
    top_models: topModels.map((item) => ({
      ...item,
      request_count: Number(item?.request_count || 0),
      total_tokens: Number(item?.total_tokens || 0),
      spend_yyc: Number(item?.spend_yyc ?? item?.spend_quota ?? 0),
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
  const navigate = useNavigate();
  const displayCurrencyIndex = useMemo(
    () => buildPublicDisplayCurrencyIndex([]),
    [],
  );
  const [period, setPeriod] = useState('last_7_days');
  const [loading, setLoading] = useState(false);
  const [trendMetric, setTrendMetric] = useState('spend_amount');
  const [modelSort, setModelSort] = useState('spend');
  const [dashboard, setDashboard] = useState(EMPTY_DASHBOARD);
  const [usageKeywordInput, setUsageKeywordInput] = useState('');
  const [usageKeyword, setUsageKeyword] = useState('');

  const activeSection = useMemo(() => {
    const params = new URLSearchParams(location.search || '');
    const rawSection = (params.get('section') || '').trim().toLowerCase();
    if (rawSection === 'overview' || rawSection === 'trend') {
      return 'spending';
    }
    if (rawSection === 'health') {
      return 'channels';
    }
    return DASHBOARD_SECTIONS.includes(rawSection) ? rawSection : 'spending';
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

  const usageRankPeriodOptions = useMemo(
    () =>
      USAGE_RANK_PERIOD_OPTIONS.map((value) => ({
        value,
        label: t(`dashboard.spending.period.${value}`),
      })),
    [t],
  );

  const trendMetricOptions = useMemo(
    () =>
      TREND_METRIC_OPTIONS.map((metric) => ({
        value: metric,
        label: t(`dashboard.admin.trend.metrics.${metric}`),
      })),
    [t],
  );

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const params = { period, section: activeSection };
      if (activeSection === 'users' && usageKeyword.trim() !== '') {
        params.user_keyword = usageKeyword.trim();
      }
      const res = await API.get('/api/v1/admin/dashboard/', {
        params,
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
  }, [activeSection, period, usageKeyword]);

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

  const modelSortOptions = useMemo(
    () =>
      MODEL_SORT_OPTIONS.map((value) => ({
        value,
        label: t(`dashboard.admin.models.sort.${value}`),
      })),
    [t],
  );

  const sortedModels = useMemo(() => {
    const items = Array.isArray(dashboard.top_models) ? [...dashboard.top_models] : [];
    items.sort((left, right) => {
      if (modelSort === 'requests') {
        if (left.request_count !== right.request_count) {
          return right.request_count - left.request_count;
        }
      } else if (modelSort === 'health') {
        if (left.health_score !== right.health_score) {
          return right.health_score - left.health_score;
        }
      } else if (modelSort === 'latency') {
        const leftLatency = Number(left.avg_latency_ms || 0);
        const rightLatency = Number(right.avg_latency_ms || 0);
        if (leftLatency !== rightLatency) {
          if (leftLatency <= 0) return 1;
          if (rightLatency <= 0) return -1;
          return leftLatency - rightLatency;
        }
      } else if (left.spend_yyc !== right.spend_yyc) {
        return right.spend_yyc - left.spend_yyc;
      }
      if (left.request_count !== right.request_count) {
        return right.request_count - left.request_count;
      }
      return String(left.model || '').localeCompare(String(right.model || ''));
    });
    return items;
  }, [dashboard.top_models, modelSort]);

  const renderHealthTag = useCallback(
    (level) => {
      const normalized = (level || 'unknown').toString().trim().toLowerCase();
      const color =
        normalized === 'healthy'
          ? 'green'
          : normalized === 'warning'
            ? 'orange'
            : normalized === 'critical'
              ? 'red'
              : 'grey';
      return (
        <AppTag color={color} className='router-tag'>
          {t(`dashboard.admin.health.level.${normalized}`, {
            defaultValue: t('dashboard.admin.health.level.unknown'),
          })}
        </AppTag>
      );
    },
    [t],
  );

  const usageRankColumns = useMemo(
    () => [
      {
        title: t('dashboard.admin.usage_rank.columns.rank'),
        key: 'rank',
        width: 72,
        render: (_, __, index) => (
          <span className='admin-dashboard-rank-index'>{index + 1}</span>
        ),
      },
      {
        title: t('dashboard.admin.usage_rank.columns.user'),
        dataIndex: 'username',
        key: 'user',
        width: 180,
        ellipsis: true,
        render: (_, record) => (
          record.user_id ? (
            <button
              type='button'
              className='admin-dashboard-user-link admin-dashboard-rank-user'
              title={record.username || record.user_id || '-'}
              onClick={() =>
                navigate(`/admin/user/detail/${encodeURIComponent(record.user_id)}`)
              }
            >
              {record.username || record.user_id || '-'}
            </button>
          ) : (
            <span
              className='admin-dashboard-rank-user'
              title={record.username || record.user_id || '-'}
            >
              {record.username || record.user_id || '-'}
            </span>
          )
        ),
      },
      {
        title: t('dashboard.admin.usage_rank.columns.requests'),
        dataIndex: 'request_count',
        key: 'request_count',
        width: 120,
        render: (value) => formatCount(value),
      },
      {
        title: t('dashboard.admin.usage_rank.columns.tokens'),
        dataIndex: 'total_tokens',
        key: 'total_tokens',
        width: 140,
        render: (value) => formatCount(value),
      },
      {
        title: t('dashboard.admin.usage_rank.columns.spend'),
        dataIndex: 'spend_yyc',
        key: 'spend_yyc',
        width: 120,
        render: (value) => formatUsd(value),
      },
      {
        title: t('dashboard.admin.usage_rank.columns.share'),
        dataIndex: 'share_rate',
        key: 'share_rate',
        width: 220,
        render: (value) => (
          <span className='admin-dashboard-rank-share-cell'>
            <span className='admin-dashboard-rank-share-text'>
              {formatPercent(value)}
            </span>
            <span className='admin-dashboard-rank-share-track'>
              <span
                className='admin-dashboard-rank-share-bar'
                style={{
                  '--admin-dashboard-share-width': `${Math.max(
                    4,
                    toPercent(value),
                  )}%`,
                }}
              />
            </span>
          </span>
        ),
      },
      {
        title: t('dashboard.admin.usage_rank.columns.last_used_at'),
        dataIndex: 'last_used_at',
        key: 'last_used_at',
        width: 180,
        render: (value) => formatUpdatedAt(value),
      },
    ],
    [formatUsd, navigate, t],
  );

  const primaryUsageUser = useMemo(() => {
    const rankedRows = Array.isArray(dashboard.usage_rank) ? dashboard.usage_rank : [];
    const topUsername = String(dashboard.usage_summary.top_username || '').trim();
    if (topUsername !== '') {
      const matched = rankedRows.find(
        (item) => String(item?.username || '').trim() === topUsername,
      );
      if (matched) {
        return matched;
      }
    }
    return rankedRows[0] || null;
  }, [dashboard.usage_rank, dashboard.usage_summary.top_username]);

  const modelHealthDistribution = useMemo(
    () => [
      {
        key: 'healthy',
        label: t('dashboard.admin.health.level.healthy'),
        count: Number(dashboard.model_summary.healthy_model_count || 0),
        color: HEALTH_LEVEL_COLORS.healthy,
      },
      {
        key: 'warning',
        label: t('dashboard.admin.health.level.warning'),
        count: Number(dashboard.model_summary.warning_model_count || 0),
        color: HEALTH_LEVEL_COLORS.warning,
      },
      {
        key: 'critical',
        label: t('dashboard.admin.health.level.critical'),
        count: Number(dashboard.model_summary.critical_model_count || 0),
        color: HEALTH_LEVEL_COLORS.critical,
      },
    ],
    [dashboard.model_summary, t],
  );

  const modelLeaderboardData = useMemo(() => {
    return sortedModels.slice(0, 8).map((item) => {
      let value = Number(item.spend_yyc || 0);
      let displayValue = formatUsd(item.spend_yyc);
      let metricLabel = t('dashboard.admin.models.sort.spend');
      if (modelSort === 'requests') {
        value = Number(item.request_count || 0);
        displayValue = formatCount(item.request_count);
        metricLabel = t('dashboard.admin.models.sort.requests');
      } else if (modelSort === 'health') {
        value = Number(item.health_score || 0);
        displayValue = `${Number(item.health_score || 0).toFixed(0)}`;
        metricLabel = t('dashboard.admin.models.sort.health');
      } else if (modelSort === 'latency') {
        const latency = Number(item.avg_latency_ms || 0);
        value = latency > 0 ? latency : 0;
        displayValue = latency > 0 ? `${latency} ms` : '-';
        metricLabel = t('dashboard.admin.models.sort.latency');
      }
      return {
        model: item.model || '-',
        short_model: String(item.model || '-').slice(0, 20),
        value,
        display_value: displayValue,
        metric_label: metricLabel,
        health_level: item.health_level || 'unknown',
      };
    });
  }, [formatCount, formatUsd, modelSort, sortedModels, t]);

  const usageInsightData = useMemo(() => {
    const rows = Array.isArray(dashboard.usage_rank) ? dashboard.usage_rank : [];
    if (rows.length === 0) {
      return {
        distribution: [],
        shareChart: [],
      };
    }
    const requestAvg =
      rows.reduce((sum, item) => sum + Number(item.request_count || 0), 0) /
      rows.length;
    const tokenAvg =
      rows.reduce((sum, item) => sum + Number(item.total_tokens || 0), 0) /
      rows.length;
    const highSpendUsers = rows.filter((item) => toPercent(item.share_rate) >= 10);
    const activeUsers = rows.filter(
      (item) => Number(item.request_count || 0) >= requestAvg,
    );
    const longTailUsers = rows.filter(
      (item) =>
        toPercent(item.share_rate) < 3 &&
        Number(item.total_tokens || 0) <= tokenAvg,
    );
    return {
      distribution: [
        {
          key: 'high_spend',
          label: t('dashboard.admin.users.insights.high_spend'),
          count: highSpendUsers.length,
          hint: t('dashboard.admin.users.insights.high_spend_hint'),
          color: '#dc2626',
          userIds: highSpendUsers
            .map((item) => (item?.user_id || '').toString().trim())
            .filter(Boolean),
        },
        {
          key: 'active',
          label: t('dashboard.admin.users.insights.active'),
          count: activeUsers.length,
          hint: t('dashboard.admin.users.insights.active_hint'),
          color: '#2563eb',
          userIds: activeUsers
            .map((item) => (item?.user_id || '').toString().trim())
            .filter(Boolean),
        },
        {
          key: 'long_tail',
          label: t('dashboard.admin.users.insights.long_tail'),
          count: longTailUsers.length,
          hint: t('dashboard.admin.users.insights.long_tail_hint'),
          color: '#64748b',
          userIds: longTailUsers
            .map((item) => (item?.user_id || '').toString().trim())
            .filter(Boolean),
        },
      ],
      shareChart: rows.slice(0, 8).map((item) => ({
        user: item.username || item.user_id || '-',
        short_user: String(item.username || item.user_id || '-').slice(0, 16),
        share_percent: Number(toPercent(item.share_rate).toFixed(1)),
      })),
    };
  }, [dashboard.usage_rank, t]);

  const openUserFocusList = useCallback((item) => {
    const userIDs = [...new Set(
      (Array.isArray(item?.userIds) ? item.userIds : [])
        .map((entry) => (entry || '').toString().trim())
        .filter(Boolean),
    )];
    if (userIDs.length === 0) {
      return;
    }
    const params = new URLSearchParams();
    params.set('focus_ids', userIDs.join(','));
    params.set('focus_name', item?.label || t('header.user'));
    navigate(`/admin/user?${params.toString()}`);
  }, [navigate, t]);

  const usageFocusData = useMemo(() => {
    const rankedRows = Array.isArray(dashboard.usage_rank) ? dashboard.usage_rank : [];
    const matchedUserCount = Number(dashboard.usage_totals.user_count || 0);
    const totalSpendYyc = Number(dashboard.usage_totals.spend_yyc || 0);
    const avgSpendPerUserYyc =
      matchedUserCount > 0 ? totalSpendYyc / matchedUserCount : 0;
    const topUserSharePercent = toPercent(dashboard.usage_summary.top_user_share);
    const top3SharePercent = rankedRows
      .slice(0, 3)
      .reduce((sum, item) => sum + toPercent(item.share_rate), 0);
    const highSpendBucket =
      usageInsightData.distribution.find((item) => item.key === 'high_spend') || null;
    const longTailBucket =
      usageInsightData.distribution.find((item) => item.key === 'long_tail') || null;

    let concentrationTone = 'stable';
    if (top3SharePercent >= 60) {
      concentrationTone = 'high';
    } else if (top3SharePercent >= 35) {
      concentrationTone = 'medium';
    }

    return [
      {
        key: 'top_user',
        label: t('dashboard.admin.users.focus.top_user'),
        value: dashboard.usage_summary.top_username || '-',
        hint: t('dashboard.admin.users.focus.top_user_hint', {
          share: formatPercent(dashboard.usage_summary.top_user_share),
        }),
        tone: 'neutral',
        compact: true,
        userId: primaryUsageUser?.user_id || '',
      },
      {
        key: 'concentration',
        label: t('dashboard.admin.users.focus.concentration'),
        value: `${top3SharePercent.toFixed(1)}%`,
        hint: t(`dashboard.admin.users.focus.concentration_${concentrationTone}`),
        tone: concentrationTone,
      },
      {
        key: 'avg_spend',
        label: t('dashboard.admin.users.focus.avg_spend'),
        value: formatUsd(avgSpendPerUserYyc),
        hint: t('dashboard.admin.users.focus.avg_spend_hint', {
          count: formatCount(matchedUserCount),
        }),
        tone: 'neutral',
      },
      {
        key: 'long_tail',
        label: t('dashboard.admin.users.focus.long_tail'),
        value: formatCount(longTailBucket?.count || 0),
        hint: t('dashboard.admin.users.focus.long_tail_hint', {
          count: formatCount(highSpendBucket?.count || 0),
          topShare: `${topUserSharePercent.toFixed(1)}%`,
        }),
        tone: longTailBucket?.count > (highSpendBucket?.count || 0) ? 'positive' : 'neutral',
      },
    ];
  }, [dashboard.usage_rank, dashboard.usage_summary, dashboard.usage_totals, formatUsd, primaryUsageUser?.user_id, t, usageInsightData]);

  const channelInsightData = useMemo(() => {
    const rows = Array.isArray(channelHealthData) ? channelHealthData : [];
    const retestCount = rows.filter(
      (item) =>
        item.selected_model_count > 0 &&
        (!item.has_test_data ||
          item.coverage_rate_percent < 100 ||
          item.pass_rate_percent < 100),
    ).length;
    const riskyCount = rows.filter(
      (item) =>
        item.health_level === 'critical' ||
        (item.has_test_data && item.pass_rate_percent < 80),
    ).length;
    const latencyCount = rows.filter(
      (item) => Number(item.avg_latency_ms || 0) >= 8000,
    ).length;
    return [
      {
        key: 'retest',
        label: t('dashboard.admin.channels.insights.retest'),
        hint: t('dashboard.admin.channels.insights.retest_hint'),
        count: retestCount,
        color: '#2563eb',
      },
      {
        key: 'risk',
        label: t('dashboard.admin.channels.insights.risk'),
        hint: t('dashboard.admin.channels.insights.risk_hint'),
        count: riskyCount,
        color: '#dc2626',
      },
      {
        key: 'latency',
        label: t('dashboard.admin.channels.insights.latency'),
        hint: t('dashboard.admin.channels.insights.latency_hint'),
        count: latencyCount,
        color: '#f59e0b',
      },
    ];
  }, [channelHealthData, t]);

  const spendingInsightData = useMemo(() => {
    const trendRows = Array.isArray(dashboard.trend) ? dashboard.trend : [];
    const peakSpend = trendRows.reduce(
      (best, item) =>
        Number(item.spend_amount || 0) > Number(best?.spend_amount || 0) ? item : best,
      null,
    );
    const peakTopup = trendRows.reduce(
      (best, item) =>
        Number(item.topup_amount || 0) > Number(best?.topup_amount || 0) ? item : best,
      null,
    );
    const peakActiveUsers = trendRows.reduce(
      (best, item) =>
        Number(item.active_user_count || 0) > Number(best?.active_user_count || 0)
          ? item
          : best,
      null,
    );
    const netAmount = Number(dashboard.summary.net_amount || 0);
    return {
      net: {
        label:
          netAmount >= 0
            ? t('dashboard.admin.spending.insights.net_positive')
            : t('dashboard.admin.spending.insights.net_negative'),
        value: formatUsd(Math.abs(netAmount)),
        hint:
          netAmount >= 0
            ? t('dashboard.admin.spending.insights.net_positive_hint')
            : t('dashboard.admin.spending.insights.net_negative_hint'),
        tone: netAmount >= 0 ? 'green' : 'red',
      },
      peakSpend: {
        label: t('dashboard.admin.spending.insights.peak_spend'),
        value: peakSpend ? formatUsd(peakSpend.spend_amount) : '-',
        hint: peakSpend?.bucket || '-',
      },
      peakTopup: {
        label: t('dashboard.admin.spending.insights.peak_topup'),
        value: peakTopup ? formatUsd(peakTopup.topup_amount) : '-',
        hint: peakTopup?.bucket || '-',
      },
      activeUsers: {
        label: t('dashboard.admin.spending.insights.peak_active_users'),
        value: peakActiveUsers ? formatCount(peakActiveUsers.active_user_count) : '-',
        hint: peakActiveUsers?.bucket || '-',
      },
    };
  }, [dashboard.summary.net_amount, dashboard.trend, formatUsd, t]);

  const renderPageHeader = () => (
    <AppFilterHeader
      className='admin-dashboard-toolbar'
      breadcrumbs={[
        { key: 'admin', label: t('header.admin_workspace') },
        { key: 'dashboard', label: t('header.system_overview') },
        { key: activeSection, label: activeSectionTitle, active: true },
      ]}
      title={activeSectionTitle}
      picker={
        <div className='admin-dashboard-period'>
          <AppSelect
            className='router-section-dropdown'
            options={periodOptions}
            value={period}
            onChange={(e, { value }) => setPeriod(value)}
          />
        </div>
      }
      actions={
        <>
          <span className='admin-dashboard-generated-at'>
            {formatUpdatedAt(dashboard.generated_at)}
          </span>
          <AppButton
            className='router-inline-button'
            type='button'
            loading={loading}
            onClick={loadData}
          >
            {t('dashboard.admin.buttons.refresh')}
          </AppButton>
        </>
      }
      endClassName='admin-dashboard-toolbar-end'
    />
  );

  const applyUsageKeyword = useCallback(() => {
    setUsageKeyword(usageKeywordInput.trim());
  }, [usageKeywordInput]);

  const clearUsageKeyword = useCallback(() => {
    setUsageKeywordInput('');
    setUsageKeyword('');
  }, []);

  const renderSpendingSection = () => (
    <AppSection>
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
      </div>
      <div className='admin-dashboard-usage-rank'>
        <div className='admin-dashboard-spending-overview-grid'>
          <div className='admin-dashboard-spending-panel admin-dashboard-spending-panel-emphasis'>
            <div className='admin-dashboard-spending-panel-label'>
              {spendingInsightData.net.label}
            </div>
            <div
              className={`admin-dashboard-spending-panel-value admin-dashboard-spending-panel-value-${spendingInsightData.net.tone}`}
            >
              {spendingInsightData.net.value}
            </div>
            <div className='admin-dashboard-spending-panel-hint'>
              {spendingInsightData.net.hint}
            </div>
          </div>
          <div className='admin-dashboard-spending-panel'>
            <div className='admin-dashboard-spending-panel-label'>
              {spendingInsightData.peakSpend.label}
            </div>
            <div className='admin-dashboard-spending-panel-value'>
              {spendingInsightData.peakSpend.value}
            </div>
            <div className='admin-dashboard-spending-panel-hint'>
              {spendingInsightData.peakSpend.hint}
            </div>
          </div>
          <div className='admin-dashboard-spending-panel'>
            <div className='admin-dashboard-spending-panel-label'>
              {spendingInsightData.peakTopup.label}
            </div>
            <div className='admin-dashboard-spending-panel-value'>
              {spendingInsightData.peakTopup.value}
            </div>
            <div className='admin-dashboard-spending-panel-hint'>
              {spendingInsightData.peakTopup.hint}
            </div>
          </div>
          <div className='admin-dashboard-spending-panel'>
            <div className='admin-dashboard-spending-panel-label'>
              {spendingInsightData.activeUsers.label}
            </div>
            <div className='admin-dashboard-spending-panel-value'>
              {spendingInsightData.activeUsers.value}
            </div>
            <div className='admin-dashboard-spending-panel-hint'>
              {spendingInsightData.activeUsers.hint}
            </div>
          </div>
        </div>
        <div className='admin-dashboard-subsection-header'>
          <div className='admin-dashboard-subsection-header-main'>
            <div className='admin-dashboard-subsection-title admin-dashboard-subsection-title-strong'>
              {t('dashboard.admin.sections.spending')}
            </div>
            <div className='admin-dashboard-subsection-description'>
              {t('dashboard.admin.spending.insights.trend_hint')}
            </div>
          </div>
          <AppToolbar
            className='admin-dashboard-trend-toolbar'
            end={
              <AppSegmented
                className='admin-dashboard-segmented'
                options={trendMetricOptions}
                value={trendMetric}
                onChange={(e, { value }) => setTrendMetric(value)}
              />
            }
          />
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
      </div>
    </AppSection>
  );

  const renderChannelsSection = () => (
    <AppSection className='admin-dashboard-section'>
      <div className='admin-dashboard-kpi-grid admin-dashboard-kpi-grid-compact'>
        <div className='admin-dashboard-kpi-item'>
          <div className='admin-dashboard-kpi-label'>
            {t('dashboard.admin.metrics.channels')}
          </div>
          <div className='admin-dashboard-kpi-value'>
            {formatCount(dashboard.summary.channel_enabled)} / {formatCount(dashboard.summary.channel_total)}
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
            {t('dashboard.admin.health.summary.needs_retest')}
          </div>
          <div className='admin-dashboard-kpi-value'>
            {formatCount(channelHealthSummary.needs_retest)}
          </div>
        </div>
      </div>
      <div className='admin-dashboard-usage-rank'>
        {channelHealthData.length === 0 ? (
          <div className='admin-dashboard-empty'>
            {t('dashboard.admin.empty.channels')}
          </div>
        ) : (
          <>
            <div className='admin-dashboard-channel-overview-grid'>
              {channelInsightData.map((item) => (
                <div
                  key={item.key}
                  className='admin-dashboard-channel-panel'
                >
                  <div className='admin-dashboard-channel-panel-main'>
                    <AppTooltip title={item.hint}>
                      <div className='admin-dashboard-channel-panel-label-row'>
                        <span
                          className='admin-dashboard-channel-panel-dot'
                          style={{ background: item.color }}
                        />
                        <span className='admin-dashboard-channel-panel-label'>
                          {item.label}
                        </span>
                      </div>
                    </AppTooltip>
                  </div>
                  <div className='admin-dashboard-channel-panel-value'>
                    {formatCount(item.count)}
                  </div>
                </div>
              ))}
            </div>
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
                      return `${label} | ${statusText} | ${healthLevelText} | ${t('dashboard.admin.health.chart.last_tested')}: ${lastTested}`;
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
      </div>
    </AppSection>
  );

  const renderUsersSection = () => (
    <AppSection className='admin-dashboard-section'>
      <div className='admin-dashboard-subsection-header'>
        <div className='admin-dashboard-subsection-header-main'>
          <div className='admin-dashboard-subsection-title admin-dashboard-subsection-title-strong'>
            {t('dashboard.admin.usage_rank.title')}
          </div>
          <div className='admin-dashboard-subsection-description'>
            {t('dashboard.admin.usage_rank.description')}
          </div>
        </div>
        <AppToolbar
          className='admin-dashboard-usage-rank-toolbar'
          end={
            <>
              <AppSegmented
                className='admin-dashboard-segmented'
                options={usageRankPeriodOptions}
                value={period}
                onChange={(e, { value }) => setPeriod(value)}
              />
              <div className='router-list-toolbar-query router-list-toolbar-query-compact'>
                <AppInput
                  className='admin-dashboard-usage-rank-search'
                  value={usageKeywordInput}
                  placeholder={t('dashboard.admin.usage_rank.search.placeholder')}
                  onChange={(e, { value }) => setUsageKeywordInput(value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      applyUsageKeyword();
                    }
                  }}
                />
                <AppButton color='blue' type='button' onClick={applyUsageKeyword}>
                  {t('dashboard.admin.usage_rank.search.submit')}
                </AppButton>
                {usageKeyword ? (
                  <AppButton
                    type='button'
                    className='router-inline-button'
                    onClick={clearUsageKeyword}
                  >
                    {t('dashboard.admin.usage_rank.search.reset')}
                  </AppButton>
                ) : null}
              </div>
            </>
          }
        />
      </div>
      {dashboard.usage_rank.length === 0 ? (
        <div className='admin-dashboard-empty'>
          {t('dashboard.admin.empty.usage_rank')}
        </div>
      ) : (
        <>
          <div className='admin-dashboard-kpi-grid admin-dashboard-kpi-grid-compact'>
            <div className='admin-dashboard-kpi-item'>
              <div className='admin-dashboard-kpi-label'>
                {t('dashboard.admin.users.summary.user_count')}
              </div>
              <div className='admin-dashboard-kpi-value'>
                {formatCount(dashboard.usage_totals.user_count)}
              </div>
            </div>
            <div className='admin-dashboard-kpi-item'>
              <div className='admin-dashboard-kpi-label'>
                {t('dashboard.admin.users.summary.request_count')}
              </div>
              <div className='admin-dashboard-kpi-value'>
                {formatCount(dashboard.usage_totals.request_count)}
              </div>
            </div>
            <div className='admin-dashboard-kpi-item'>
              <div className='admin-dashboard-kpi-label'>
                {t('dashboard.admin.users.summary.total_tokens')}
              </div>
              <div className='admin-dashboard-kpi-value'>
                {formatCount(dashboard.usage_totals.total_tokens)}
              </div>
            </div>
            <div className='admin-dashboard-kpi-item'>
              <div className='admin-dashboard-kpi-label'>
                {t('dashboard.admin.users.summary.total_spend')}
              </div>
              <div className='admin-dashboard-kpi-value'>
                {formatUsd(dashboard.usage_totals.spend_yyc)}
              </div>
            </div>
          </div>
          <div className='admin-dashboard-user-focus-grid'>
            {usageFocusData.map((item) => (
              <div
                key={item.key}
                className={`admin-dashboard-user-focus-card admin-dashboard-user-focus-card-${item.tone}`}
              >
                <div className='admin-dashboard-user-focus-label'>{item.label}</div>
                <div
                  className={`admin-dashboard-user-focus-value ${
                    item.compact ? 'admin-dashboard-user-focus-value-compact' : ''
                  }`.trim()}
                  title={item.value}
                >
                  {item.userId ? (
                    <button
                      type='button'
                      className='admin-dashboard-user-link admin-dashboard-user-focus-link'
                      onClick={() =>
                        navigate(`/admin/user/detail/${encodeURIComponent(item.userId)}`)
                      }
                    >
                      {item.value}
                    </button>
                  ) : (
                    item.value
                  )}
                </div>
                <div className='admin-dashboard-user-focus-hint'>{item.hint}</div>
              </div>
            ))}
          </div>
          <div className='admin-dashboard-user-overview-grid'>
            <div className='admin-dashboard-user-panel'>
              <div className='admin-dashboard-card-title'>
                {t('dashboard.admin.users.insights.title')}
              </div>
              <div className='admin-dashboard-user-distribution-list'>
                {usageInsightData.distribution.map((item) => (
                  <div
                    key={item.key}
                    className='admin-dashboard-user-distribution-item'
                  >
                    <div className='admin-dashboard-user-distribution-main'>
                      <span
                        className='admin-dashboard-user-distribution-dot'
                        style={{ background: item.color }}
                      />
                      <div className='admin-dashboard-user-distribution-copy'>
                        <div className='admin-dashboard-user-distribution-label'>
                          {item.label}
                        </div>
                        <div className='admin-dashboard-user-distribution-hint'>
                          {item.hint}
                        </div>
                      </div>
                    </div>
                    <div className='admin-dashboard-user-distribution-side'>
                      <div className='admin-dashboard-user-distribution-value'>
                        {formatCount(item.count)}
                      </div>
                      {item.userIds?.length > 0 ? (
                        <AppButton
                          type='button'
                          className='router-inline-button'
                          onClick={() => openUserFocusList(item)}
                        >
                          {t('common.view')}
                        </AppButton>
                      ) : null}
                    </div>
                  </div>
                ))}
                <div className='admin-dashboard-user-distribution-footnote'>
                  {t('dashboard.admin.users.insights.footnote')}
                </div>
              </div>
            </div>
            <div className='admin-dashboard-user-panel'>
              <div className='admin-dashboard-card-title'>
                {t('dashboard.admin.users.share_chart.title')}
              </div>
              <div className='admin-dashboard-user-chart'>
                <ResponsiveContainer width='100%' height={240}>
                  <BarChart
                    data={usageInsightData.shareChart}
                    layout='vertical'
                    margin={{ top: 0, right: 12, left: 12, bottom: 0 }}
                  >
                    <CartesianGrid
                      strokeDasharray='3 3'
                      horizontal={false}
                      opacity={0.08}
                    />
                    <XAxis
                      type='number'
                      axisLine={false}
                      tickLine={false}
                      tick={{ fontSize: 12, fill: '#94a3b8' }}
                    />
                    <YAxis
                      type='category'
                      dataKey='short_user'
                      width={110}
                      axisLine={false}
                      tickLine={false}
                      tick={{ fontSize: 12, fill: '#475569' }}
                    />
                    <Tooltip
                      contentStyle={{
                        background: '#fff',
                        border: 'none',
                        borderRadius: '4px',
                        boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
                      }}
                      formatter={(value) => [`${Number(value || 0).toFixed(1)}%`, t('dashboard.admin.usage_rank.columns.share')]}
                      labelFormatter={(_, payload) =>
                        payload?.[0]?.payload?.user || '-'
                      }
                    />
                    <Bar
                      dataKey='share_percent'
                      fill='#2563eb'
                      radius={[0, 4, 4, 0]}
                    />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            </div>
          </div>
          <div className='admin-dashboard-usage-rank-section-title'>
            {t('dashboard.admin.users.table_title')}
          </div>
          <AppTable
            className='admin-dashboard-rank-table'
            columns={usageRankColumns}
            dataSource={dashboard.usage_rank}
            pagination={false}
            rowKey={(record) =>
              record.user_id ||
              `${record.username || 'unknown'}-${record.last_used_at || 0}`
            }
            scroll={{ x: 980 }}
          />
        </>
      )}
    </AppSection>
  );

  const renderModelsSection = () => (
    <AppSection className='admin-dashboard-section'>
      <div className='admin-dashboard-subsection-header'>
        <div className='admin-dashboard-subsection-header-main'>
          <div className='admin-dashboard-subsection-title admin-dashboard-subsection-title-strong'>
            {t('dashboard.admin.models.title')}
          </div>
          <div className='admin-dashboard-subsection-description'>
            {t('dashboard.admin.models.description')}
          </div>
        </div>
        <AppToolbar
          className='admin-dashboard-trend-toolbar'
          end={
            <AppSegmented
              className='admin-dashboard-segmented'
              options={modelSortOptions}
              value={modelSort}
              onChange={(e, { value }) => setModelSort(value)}
            />
          }
        />
      </div>
      <div className='admin-dashboard-kpi-grid admin-dashboard-kpi-grid-compact'>
        <div className='admin-dashboard-kpi-item'>
          <div className='admin-dashboard-kpi-label'>
            {t('dashboard.admin.models.summary.selected_model_count')}
          </div>
          <div className='admin-dashboard-kpi-value'>
            {formatCount(dashboard.model_summary.selected_model_count)}
          </div>
        </div>
        <div className='admin-dashboard-kpi-item'>
          <div className='admin-dashboard-kpi-label'>
            {t('dashboard.admin.models.summary.tested_model_count')}
          </div>
          <div className='admin-dashboard-kpi-value'>
            {formatCount(dashboard.model_summary.tested_model_count)}
          </div>
        </div>
        <div className='admin-dashboard-kpi-item'>
          <div className='admin-dashboard-kpi-label'>
            {t('dashboard.admin.models.summary.healthy_model_count')}
          </div>
          <div className='admin-dashboard-kpi-value'>
            {formatCount(dashboard.model_summary.healthy_model_count)}
          </div>
        </div>
        <div className='admin-dashboard-kpi-item'>
          <div className='admin-dashboard-kpi-label'>
            {t('dashboard.admin.models.summary.warning_model_count')}
          </div>
          <div className='admin-dashboard-kpi-value'>
            {formatCount(dashboard.model_summary.warning_model_count)}
          </div>
        </div>
        <div className='admin-dashboard-kpi-item'>
          <div className='admin-dashboard-kpi-label'>
            {t('dashboard.admin.models.summary.critical_model_count')}
          </div>
          <div className='admin-dashboard-kpi-value'>
            {formatCount(dashboard.model_summary.critical_model_count)}
          </div>
        </div>
        <div className='admin-dashboard-kpi-item'>
          <div className='admin-dashboard-kpi-label'>
            {t('dashboard.admin.models.summary.request_count')}
          </div>
          <div className='admin-dashboard-kpi-value'>
            {formatCount(dashboard.model_summary.request_count)}
          </div>
        </div>
        <div className='admin-dashboard-kpi-item'>
          <div className='admin-dashboard-kpi-label'>
            {t('dashboard.admin.models.summary.total_spend')}
          </div>
          <div className='admin-dashboard-kpi-value'>
            {formatUsd(dashboard.model_summary.spend_yyc)}
          </div>
        </div>
        <div className='admin-dashboard-kpi-item'>
          <div className='admin-dashboard-kpi-label'>
            {t('dashboard.admin.models.summary.avg_pass_rate')}
          </div>
          <div className='admin-dashboard-kpi-value'>
            {formatPercent(dashboard.model_summary.avg_pass_rate)}
          </div>
        </div>
        <div className='admin-dashboard-kpi-item'>
          <div className='admin-dashboard-kpi-label'>
            {t('dashboard.admin.models.summary.avg_latency')}
          </div>
          <div className='admin-dashboard-kpi-value'>
            {dashboard.model_summary.avg_latency_ms > 0
              ? `${dashboard.model_summary.avg_latency_ms} ms`
              : '-'}
          </div>
        </div>
      </div>
      <div className='admin-dashboard-usage-rank'>
        {sortedModels.length === 0 ? (
          <div className='admin-dashboard-empty'>
            {t('dashboard.admin.empty.models')}
          </div>
        ) : (
          <>
            <div className='admin-dashboard-model-overview-grid'>
              <div className='admin-dashboard-model-panel'>
                <div className='admin-dashboard-card-title'>
                  {t('dashboard.admin.models.chart.leaderboard')}
                </div>
                <div className='admin-dashboard-model-chart'>
                  <ResponsiveContainer width='100%' height={260}>
                    <BarChart
                      data={modelLeaderboardData}
                      layout='vertical'
                      margin={{ top: 0, right: 12, left: 12, bottom: 0 }}
                    >
                      <CartesianGrid
                        strokeDasharray='3 3'
                        horizontal={false}
                        opacity={0.08}
                      />
                      <XAxis
                        type='number'
                        axisLine={false}
                        tickLine={false}
                        tick={{ fontSize: 12, fill: '#94a3b8' }}
                      />
                      <YAxis
                        type='category'
                        dataKey='short_model'
                        width={128}
                        axisLine={false}
                        tickLine={false}
                        tick={{ fontSize: 12, fill: '#475569' }}
                      />
                      <Tooltip
                        contentStyle={{
                          background: '#fff',
                          border: 'none',
                          borderRadius: '4px',
                          boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
                        }}
                        formatter={(value, _, payload) => [
                          payload?.payload?.display_value || value,
                          payload?.payload?.metric_label || '',
                        ]}
                        labelFormatter={(_, payload) =>
                          payload?.[0]?.payload?.model || '-'
                        }
                      />
                      <Bar dataKey='value' radius={[0, 4, 4, 0]}>
                        {modelLeaderboardData.map((item) => (
                          <Cell
                            key={`${item.model}-leaderboard`}
                            fill={
                              HEALTH_LEVEL_COLORS[item.health_level] ||
                              HEALTH_LEVEL_COLORS.unknown
                            }
                          />
                        ))}
                      </Bar>
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              </div>
              <div className='admin-dashboard-model-panel'>
                <div className='admin-dashboard-card-title'>
                  {t('dashboard.admin.models.chart.distribution')}
                </div>
                <div className='admin-dashboard-model-distribution-list'>
                  {modelHealthDistribution.map((item) => (
                    <div
                      key={item.key}
                      className='admin-dashboard-model-distribution-item'
                    >
                      <div className='admin-dashboard-model-distribution-main'>
                        <span
                          className='admin-dashboard-model-distribution-dot'
                          style={{ background: item.color }}
                        />
                        <span className='admin-dashboard-model-distribution-label'>
                          {item.label}
                        </span>
                      </div>
                      <div className='admin-dashboard-model-distribution-value'>
                        {formatCount(item.count)}
                      </div>
                    </div>
                  ))}
                  <div className='admin-dashboard-model-distribution-footnote'>
                    {t('dashboard.admin.models.chart.distribution_hint')}
                  </div>
                </div>
              </div>
            </div>
            <div className='admin-dashboard-model-grid'>
              {sortedModels.map((item) => (
                <div
                  key={`${item.provider || 'unknown'}-${item.model}`}
                  className='admin-dashboard-model-card'
                >
                  <div className='admin-dashboard-model-card-header'>
                    <div className='admin-dashboard-model-card-main'>
                      <div
                        className='admin-dashboard-model-card-title'
                        title={item.model || '-'}
                      >
                        {item.model || '-'}
                      </div>
                      <div className='admin-dashboard-model-card-subtitle'>
                        <span>{item.provider || '-'}</span>
                        {Array.isArray(item.tags) && item.tags.length > 0 ? (
                          <div className='router-tag-group'>
                            {item.tags.map((tag) => (
                              <AppTag
                                key={`${item.model}-${tag}`}
                                className='router-tag'
                              >
                                {tag}
                              </AppTag>
                            ))}
                          </div>
                        ) : null}
                      </div>
                    </div>
                    <div className='admin-dashboard-model-card-status'>
                      {renderHealthTag(item.health_level)}
                    </div>
                  </div>
                  <div className='admin-dashboard-model-card-metrics'>
                    <div className='admin-dashboard-model-metric'>
                      <div className='admin-dashboard-model-metric-label'>
                        {t('dashboard.admin.models.card.requests')}
                      </div>
                      <div className='admin-dashboard-model-metric-value'>
                        {formatCount(item.request_count)}
                      </div>
                    </div>
                    <div className='admin-dashboard-model-metric'>
                      <div className='admin-dashboard-model-metric-label'>
                        {t('dashboard.admin.models.card.tokens')}
                      </div>
                      <div className='admin-dashboard-model-metric-value'>
                        {formatCount(item.total_tokens)}
                      </div>
                    </div>
                    <div className='admin-dashboard-model-metric'>
                      <div className='admin-dashboard-model-metric-label'>
                        {t('dashboard.admin.models.card.spend')}
                      </div>
                      <div className='admin-dashboard-model-metric-value'>
                        {formatUsd(item.spend_yyc)}
                      </div>
                    </div>
                    <div className='admin-dashboard-model-metric'>
                      <div className='admin-dashboard-model-metric-label'>
                        {t('dashboard.admin.models.card.channels')}
                      </div>
                      <div className='admin-dashboard-model-metric-value'>
                        {formatCount(item.channel_count)}
                      </div>
                    </div>
                    <div className='admin-dashboard-model-metric'>
                      <div className='admin-dashboard-model-metric-label'>
                        {t('dashboard.admin.models.card.supported_endpoints')}
                      </div>
                      <div className='admin-dashboard-model-metric-value'>
                        {formatCount(item.supported_endpoint_count)}
                      </div>
                    </div>
                    <div className='admin-dashboard-model-metric'>
                      <div className='admin-dashboard-model-metric-label'>
                        {t('dashboard.admin.models.card.pass_rate')}
                      </div>
                      <div className='admin-dashboard-model-metric-value'>
                        {formatPercent(item.pass_rate)}
                      </div>
                    </div>
                    <div className='admin-dashboard-model-metric'>
                      <div className='admin-dashboard-model-metric-label'>
                        {t('dashboard.admin.models.card.avg_latency')}
                      </div>
                      <div className='admin-dashboard-model-metric-value'>
                        {item.avg_latency_ms > 0 ? `${item.avg_latency_ms} ms` : '-'}
                      </div>
                    </div>
                    <div className='admin-dashboard-model-metric'>
                      <div className='admin-dashboard-model-metric-label'>
                        {t('dashboard.admin.models.card.last_tested')}
                      </div>
                      <div className='admin-dashboard-model-metric-value'>
                        {formatUpdatedAt(item.last_tested_at)}
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </>
        )}
      </div>
    </AppSection>
  );

  return (
    <div className='dashboard-container admin-dashboard-container'>
      {renderPageHeader()}
      {activeSection === 'spending' ? renderSpendingSection() : null}
      {activeSection === 'channels' ? renderChannelsSection() : null}
      {activeSection === 'users' ? renderUsersSection() : null}
      {activeSection === 'models' ? renderModelsSection() : null}
    </div>
  );
};

export default AdminDashboard;
