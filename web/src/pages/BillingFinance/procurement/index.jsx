import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { API, showError, showSuccess, timestamp2string, withCardLabels } from '../../../helpers';
import { exportCSV } from '../../../helpers/csv';
import { formatDecimalNumber, formatCreditAmount } from '../../../helpers/render';
import {
  AppButton,
  AppErrorState,
  AppFilterHeader,
  AppInput,
  AppPopconfirm,
  AppSelect,
  AppSegmented,
  AppSpin,
  AppTable,
  AppTag,
  chartAxisStyle,
  chartCategoricalPalette,
  chartGridStyle,
  chartNeutralColor,
  chartStatusPalette,
  chartTooltipStyle,
  colorForKey,
  getActiveChartTheme,
  formatCnyChart,
  formatCnyFixed,
  formatCsvCurrency,
  formatCsvPercent,
  formatBillingPercent,
  BILLING_PERCENT_DECIMALS,
} from '../../../router-ui';
import './BillingProcurementReport.css';

const toDateTimeLocalValue = (date) => {
  const pad = (value) => String(value).padStart(2, '0');
  return [
    date.getFullYear(),
    '-',
    pad(date.getMonth() + 1),
    '-',
    pad(date.getDate()),
    'T',
    pad(date.getHours()),
    ':',
    pad(date.getMinutes()),
  ].join('');
};

const timestampFromDateTimeLocal = (value) => {
  const date = new Date(value || '');
  const timestamp = Math.floor(date.getTime() / 1000);
  return Number.isFinite(timestamp) && timestamp > 0 ? timestamp : 0;
};

const createLastSevenDaysRange = () => {
  const end = new Date();
  const start = new Date(end.getTime() - 7 * 24 * 60 * 60 * 1000);
  return {
    startAt: toDateTimeLocalValue(start),
    endAt: toDateTimeLocalValue(end),
  };
};

const formatCNY = formatCnyFixed;
const formatYYC = (value) => formatCreditAmount(value || 0, true);
const formatCount = (value) => formatDecimalNumber(value || 0, 0);
const formatPercent = (value) => formatBillingPercent(value, BILLING_PERCENT_DECIMALS);

const normalizeReport = (payload) => {
  const items = Array.isArray(payload?.items) ? payload.items : [];
  return {
    group_by: payload?.group_by || 'channel',
    provider: payload?.provider || '',
    request_count: Number(payload?.request_count || 0),
    router_consumed_yyc: Number(payload?.router_consumed_yyc || 0),
    configured_cost_request_count: Number(payload?.configured_cost_request_count || 0),
    unconfigured_cost_request_count: Number(payload?.unconfigured_cost_request_count || 0),
    pending_cost_request_count: Number(payload?.pending_cost_request_count || 0),
    retry_cost_request_count: Number(payload?.retry_cost_request_count || 0),
    sell_base_amount: Number(payload?.sell_base_amount || 0),
    configured_sell_base_amount: Number(payload?.configured_sell_base_amount || 0),
    unconfigured_sell_base_amount: Number(payload?.unconfigured_sell_base_amount || 0),
    procurement_cost_base_amount: Number(payload?.procurement_cost_base_amount || 0),
    gross_profit_base_amount: Number(payload?.gross_profit_base_amount || 0),
    gross_profit_yyc: Number(payload?.gross_profit_yyc || 0),
    gross_margin: Number(payload?.gross_margin || 0),
    items: items.map((item) => ({
      ...item,
      unconfigured_channels: Array.isArray(item?.unconfigured_channels)
        ? item.unconfigured_channels.map((channel) => ({
            ...channel,
            request_count: Number(channel?.request_count || 0),
            last_request_at: Number(channel?.last_request_at || 0),
          }))
        : [],
      unconfigured_channel_count: Number(item?.unconfigured_channel_count || 0),
      request_count: Number(item?.request_count || 0),
      router_consumed_yyc: Number(item?.router_consumed_yyc || 0),
      configured_cost_request_count: Number(item?.configured_cost_request_count || 0),
      unconfigured_cost_request_count: Number(item?.unconfigured_cost_request_count || 0),
      pending_cost_request_count: Number(item?.pending_cost_request_count || 0),
      retry_cost_request_count: Number(item?.retry_cost_request_count || 0),
      sell_base_amount: Number(item?.sell_base_amount || 0),
      configured_sell_base_amount: Number(item?.configured_sell_base_amount || 0),
      unconfigured_sell_base_amount: Number(item?.unconfigured_sell_base_amount || 0),
      procurement_cost_base_amount: Number(item?.procurement_cost_base_amount || 0),
      gross_profit_base_amount: Number(item?.gross_profit_base_amount || 0),
      gross_profit_yyc: Number(item?.gross_profit_yyc || 0),
      gross_margin: Number(item?.gross_margin || 0),
    })),
  };
};

const normalizeHealth = (payload) => ({
  status: payload?.status || 'ok',
  checked_at: Number(payload?.checked_at || 0),
  critical_count: Number(payload?.critical_count || 0),
  warning_count: Number(payload?.warning_count || 0),
  issues: Array.isArray(payload?.issues)
    ? payload.issues.map((item) => ({
        ...item,
        count: Number(item?.count || 0),
      }))
    : [],
});

function BillingProcurementReport({ embedded = false }) {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const initialRange = useMemo(() => createLastSevenDaysRange(), []);
  const initialContext = useMemo(() => {
    const params = new URLSearchParams(location.search);
    const groupByValue = params.get('group_by');
    const costScopeValue = params.get('cost_scope');
    const toLocal = (key, fallback) => {
      const timestamp = Number(params.get(key) || 0);
      return Number.isFinite(timestamp) && timestamp > 0 ? toDateTimeLocalValue(new Date(timestamp * 1000)) : fallback;
    };
    return {
      groupBy: ['channel', 'model', 'endpoint'].includes(groupByValue) ? groupByValue : 'channel',
      costScope: ['all', 'unconfigured'].includes(costScopeValue) ? costScopeValue : 'all',
      groupID: params.get('group_id') || '',
      provider: params.get('provider') || '',
      model: params.get('model') || '',
      returnTo: params.get('return_to') || '',
      startAt: toLocal('start_at', initialRange.startAt),
      endAt: toLocal('end_at', initialRange.endAt),
    };
  }, []);
  const [groupBy, setGroupBy] = useState(initialContext.groupBy);
  const [costScope, setCostScope] = useState(initialContext.costScope);
  const [groupID, setGroupID] = useState(initialContext.groupID);
  const [provider, setProvider] = useState(initialContext.provider);
  const [model, setModel] = useState(initialContext.model);
  const [groupOptions, setGroupOptions] = useState([]);
  const [providerOptions, setProviderOptions] = useState([]);
  const [modelOptions, setModelOptions] = useState([]);
  const [startAt, setStartAt] = useState(initialContext.startAt);
  const [endAt, setEndAt] = useState(initialContext.endAt);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const [healthLoading, setHealthLoading] = useState(false);
  const [report, setReport] = useState(() => normalizeReport({}));
  const [health, setHealth] = useState(() => normalizeHealth({}));
  const [retryItems, setRetryItems] = useState([]);
  const [retryLoading, setRetryLoading] = useState(false);
  const [retryingLogID, setRetryingLogID] = useState('');
  const channelProcurementPath = useCallback(
    (channelID) =>
      `/admin/channel/detail/${encodeURIComponent(
        String(channelID || '')
      )}?tab=procurement`,
    []
  );
  // Legacy bookmarks pointed per-channel procurement at the finance tab
  // (`?tab=procurement&channel_id=…`). That drill now lives in the channel
  // detail tab, so redirect those old links on mount.
  const legacyChannelRedirectedRef = useRef(false);
  useEffect(() => {
    if (legacyChannelRedirectedRef.current) {
      return;
    }
    legacyChannelRedirectedRef.current = true;
    const bookmarkedChannelID = new URLSearchParams(location.search).get(
      'channel_id'
    );
    if (bookmarkedChannelID) {
      navigate(channelProcurementPath(bookmarkedChannelID), { replace: true });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const loadRetries = useCallback(async () => {
    const startTimestamp = timestampFromDateTimeLocal(startAt);
    const endTimestamp = timestampFromDateTimeLocal(endAt);
    setRetryLoading(true);
    try {
      const response = await API.get('/api/v1/admin/billing/procurement/retries', {
        params: {
          start_at: startTimestamp,
          end_at: endTimestamp,
          group_id: groupID,
          provider,
          model,
          limit: 50,
        },
      });
      if (!response.data?.success) throw new Error(response.data?.message);
      const items = Array.isArray(response.data?.data?.items) ? response.data.data.items : [];
      setRetryItems(items.map((item) => ({
        ...item,
        created_at: Number(item?.created_at || 0),
        sell_base_amount: Number(item?.sell_base_amount || 0),
        retry_count: Number(item?.retry_count || 0),
        last_retry_at: Number(item?.last_retry_at || 0),
      })));
    } catch (error) {
      showError(error?.message || t('billing.procurement_report.retry.load_failed'));
    } finally {
      setRetryLoading(false);
    }
  }, [endAt, groupID, model, provider, startAt, t]);

  const loadGroups = async () => {
    try {
      const res = await API.get('/api/v1/admin/groups', {
        params: {
          page: 1,
          page_size: 200,
        },
      });
      const { success, data } = res.data || {};
      if (!success) {
        return;
      }
      const items = Array.isArray(data?.items) ? data.items : [];
      setGroupOptions(
        items.map((group) => ({
          key: group.id,
          value: group.id,
          text: group.name || group.id,
        })),
      );
    } catch {
      // Ignore non-critical filter bootstrap failure.
    }
  };

  const loadReport = async () => {
    const startTimestamp = timestampFromDateTimeLocal(startAt);
    const endTimestamp = timestampFromDateTimeLocal(endAt);
    if (!startTimestamp || !endTimestamp || endTimestamp < startTimestamp) {
      showError(t('billing.procurement_report.messages.invalid_time'));
      return;
    }
    setLoading(true);
    setLoadError(false);
    try {
      const res = await API.get('/api/v1/admin/billing/procurement-report', {
        params: {
          start_at: startTimestamp,
          end_at: endTimestamp,
          group_by: groupBy,
          cost_scope: costScope,
          group_id: groupID,
          provider,
          model,
        },
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        setLoadError(true);
        showError(message || t('billing.procurement_report.messages.load_failed'));
        return;
      }
      setReport(normalizeReport(data));
    } catch (error) {
      setLoadError(true);
      showError(error?.message || t('billing.procurement_report.messages.load_failed'));
    } finally {
      setLoading(false);
    }
  };

  const retryProcurementAttribution = useCallback(async (logID) => {
    const id = String(logID || '').trim();
    if (!id) return;
    setRetryingLogID(id);
    try {
      const response = await API.post(`/api/v1/admin/billing/procurement/retries/${encodeURIComponent(id)}/retry`);
      if (!response.data?.success) throw new Error(response.data?.message);
      await Promise.all([loadRetries(), loadReport()]);
      showSuccess(t('billing.procurement_report.retry.retry_success'));
    } catch (error) {
      showError(error?.message || t('billing.procurement_report.retry.retry_failed'));
      await loadRetries();
    } finally {
      setRetryingLogID('');
    }
  }, [loadRetries, t]);

  const loadHealth = async () => {
    setHealthLoading(true);
    try {
      const res = await API.get('/api/v1/admin/billing/health');
      const { success, data } = res.data || {};
      if (success) {
        setHealth(normalizeHealth(data));
      } else {
        setHealth(
          normalizeHealth({
            status: 'unknown',
            issues: [
              {
                key: 'health_load_failed',
                level: 'warning',
                title: t('billing.procurement_report.health.load_failed'),
                message: t('billing.procurement_report.health.load_failed_hint'),
              },
            ],
          }),
        );
      }
    } catch {
      setHealth(
        normalizeHealth({
          status: 'unknown',
          issues: [
            {
              key: 'health_load_failed',
              level: 'warning',
              title: t('billing.procurement_report.health.load_failed'),
              message: t('billing.procurement_report.health.load_failed_hint'),
            },
          ],
        }),
      );
    } finally {
      setHealthLoading(false);
    }
  };

  useEffect(() => {
    loadGroups().then();
    loadHealth().then();
    const loadProviders = async () => {
      try {
        const items = [];
        let page = 1;
        let total = 0;
        while (page <= 20) {
          const response = await API.get('/api/v1/admin/providers', {
            params: { page, page_size: 100 },
          });
          const data = response.data?.success ? response.data?.data : null;
          const pageItems = Array.isArray(data?.items) ? data.items : [];
          items.push(...pageItems);
          total = Number(data?.total || items.length);
          if (pageItems.length === 0 || items.length >= total || pageItems.length < 100) {
            break;
          }
          page += 1;
        }
        setProviderOptions(
          items
            .map((item) => {
              const value = String(item?.id || '').trim();
              if (!value) return null;
              return {
                key: value,
                value,
                text: item?.name ? `${item.name} (${value})` : value,
              };
            })
            .filter(Boolean),
        );
      } catch (error) {
        showError(error?.message || t('common.load_failed'));
      }
    };
    loadProviders().then();

    // Populate the model dropdown from the logs options endpoint so the model
    // filter becomes a searchable picker instead of free text. Empty/failed
    // fetch is non-fatal: the URL-bound value still drives the request and
    // AppSelect renders unknown values as raw strings.
    API.get('/api/v1/admin/log/options', { params: { field: 'model_name' } })
      .then((response) => {
        if (!response.data?.success) return;
        const names = Array.isArray(response.data?.data?.model_names)
          ? response.data.data.model_names
          : [];
        setModelOptions(
          names.filter((name) => typeof name === 'string' && name !== '').map((name) => ({
            key: name,
            value: name,
            text: name,
          })),
        );
      })
      .catch((error) => showError(error?.message || t('common.load_failed')));
  }, []);

  useEffect(() => {
    const params = new URLSearchParams();
    const startTimestamp = timestampFromDateTimeLocal(startAt);
    const endTimestamp = timestampFromDateTimeLocal(endAt);
    if (startTimestamp) params.set('start_at', String(startTimestamp));
    if (endTimestamp) params.set('end_at', String(endTimestamp));
    params.set('group_by', groupBy);
    params.set('cost_scope', costScope);
    if (groupID) params.set('group_id', groupID);
    if (provider) params.set('provider', provider);
    if (model) params.set('model', model);
    if (initialContext.returnTo) params.set('return_to', initialContext.returnTo);
    // Preserve the shell tab so the finance layout keeps this page active when it
    // rewrites its own filter params.
    const currentTab = new URLSearchParams(location.search).get('tab');
    if (currentTab) params.set('tab', currentTab);
    navigate({ pathname: location.pathname, search: `?${params.toString()}` }, { replace: true });
  }, [costScope, endAt, groupBy, groupID, initialContext.returnTo, location.pathname, model, navigate, provider, startAt]);

  useEffect(() => {
    loadReport().then();
    loadRetries().then();
  }, [groupBy, costScope, groupID, model, provider, loadRetries]);

  // 8 summary cards were collapsed into a single headline + side stats.
  // The two big numbers on the left are gross profit + margin (actionable);
  // the right column keeps the operational numbers that used to live in the
  // 4-column grid.
  const headlineProfit = formatCNY(report.gross_profit_base_amount);
  const headlineMargin = formatPercent(report.gross_margin);
  const headlineTone =
    report.gross_profit_base_amount < 0
      ? 'negative'
      : report.gross_margin < 0.1
        ? 'warning'
        : 'positive';
  const attributionProcessing = report.pending_cost_request_count + report.retry_cost_request_count;
  const headlineSideStats = [
    {
      key: 'profit_yyc',
      label: t('billing.procurement_report.summary.gross_profit_yyc'),
      value: formatYYC(report.gross_profit_yyc),
    },
    {
      key: 'revenue',
      label: t('billing.procurement_report.summary.sell_amount'),
      value: formatCNY(report.sell_base_amount),
    },
    {
      key: 'cost',
      label: t('billing.procurement_report.summary.procurement_cost'),
      value: formatCNY(report.procurement_cost_base_amount),
    },
    {
      key: 'unconfigured',
      label: t('billing.procurement_report.summary.unconfigured'),
      value: formatCount(report.unconfigured_cost_request_count),
      tone: report.unconfigured_cost_request_count > 0 ? 'warning' : undefined,
    },
    {
      key: 'attribution_processing',
      label: t('billing.procurement_report.summary.attribution_processing'),
      value: formatCount(attributionProcessing),
      tone: report.retry_cost_request_count > 0 ? 'warning' : undefined,
    },
  ];
  // The advanced analytics card carries the provider pie + margin distribution
  // — both are the headline visuals of this page, so default to expanded.
  const [showAdvancedAnalytics, setShowAdvancedAnalytics] = useState(true);

  const providerBreakdown = useMemo(() => {
    const buckets = new Map();
    (Array.isArray(report.items) ? report.items : []).forEach((row) => {
      const cost = Number(row?.procurement_cost_base_amount || 0);
      if (cost <= 0) return;
      const raw = (row?.provider || '').toString().trim();
      const key = raw || '__unassigned__';
      const prev = buckets.get(key) || { key, label: raw || t('common.unassigned'), cost: 0 };
      prev.cost += cost;
      buckets.set(key, prev);
    });
    const arr = Array.from(buckets.values()).sort((left, right) => right.cost - left.cost);
    if (arr.length > 8) {
      const head = arr.slice(0, 8);
      const otherCost = arr.slice(8).reduce((sum, item) => sum + item.cost, 0);
      if (otherCost > 0) {
        head.push({ key: '__other__', label: t('common.other'), cost: otherCost });
      }
      return head;
    }
    return arr;
  }, [report.items, t]);

  const marginDistribution = useMemo(() => {
    const buckets = {
      loss: 0,
      low: 0,
      ok: 0,
      high: 0,
      unknown: 0,
    };
    (Array.isArray(report.items) ? report.items : []).forEach((row) => {
      const configured = Number(row?.configured_cost_request_count || 0);
      const cost = Number(row?.procurement_cost_base_amount || 0);
      if (configured <= 0 || cost <= 0) {
        buckets.unknown += 1;
        return;
      }
      const margin = Number(row?.gross_margin || 0);
      if (margin < 0) buckets.loss += 1;
      else if (margin < 0.1) buckets.low += 1;
      else if (margin < 0.3) buckets.ok += 1;
      else buckets.high += 1;
    });
    return [
      { key: 'loss', label: t('billing.procurement_report.analytics.margin_bucket.loss'), count: buckets.loss, color: chartStatusPalette.danger },
      { key: 'low', label: t('billing.procurement_report.analytics.margin_bucket.low'), count: buckets.low, color: chartStatusPalette.warning },
      { key: 'ok', label: t('billing.procurement_report.analytics.margin_bucket.ok'), count: buckets.ok, color: chartStatusPalette.success },
      { key: 'high', label: t('billing.procurement_report.analytics.margin_bucket.high'), count: buckets.high, color: chartCategoricalPalette[0] },
      { key: 'unknown', label: t('billing.procurement_report.analytics.margin_bucket.unknown'), count: buckets.unknown, color: chartNeutralColor() },
    ];
  }, [report.items, t]);

  const lossLeaderboard = useMemo(() => {
    const items = (Array.isArray(report.items) ? report.items : [])
      .filter((row) => Number(row?.gross_profit_base_amount || 0) < 0)
      .sort(
        (left, right) =>
          Number(left?.gross_profit_base_amount || 0) - Number(right?.gross_profit_base_amount || 0),
      )
      .slice(0, 10)
      .map((row) => {
        const configured = Number(row?.configured_cost_request_count || 0);
        const cost = Number(row?.procurement_cost_base_amount || 0);
        const isUnknown = configured <= 0 || cost <= 0;
        return {
          key: `${row?.dimension_type || ''}-${row?.dimension_key || ''}`,
          label: row?.dimension_name || row?.dimension_key || '-',
          cost: cost,
          profit: Number(row?.gross_profit_base_amount || 0),
          margin: isUnknown ? null : Number(row?.gross_margin || 0),
          unknown: isUnknown,
        };
      });
    return items;
  }, [report.items]);

  const renderUnconfiguredChannels = (row) => {
    const channels = Array.isArray(row?.unconfigured_channels)
      ? row.unconfigured_channels
      : [];
    if (channels.length === 0) {
      return '-';
    }
    const total = Number(row?.unconfigured_channel_count || channels.length);
    return (
      <div className='billing-procurement-report-channel-links'>
        {channels.map((channel) => {
          const channelID = (channel?.id || '').toString().trim();
          if (!channelID) {
            return null;
          }
          const label = (channel?.name || channelID).toString().trim();
          return (
            <Link
              key={channelID}
              className='billing-procurement-report-link'
              to={channelProcurementPath(channelID)}
              title={t('billing.procurement_report.actions.configure_cost')}
            >
              {label}
            </Link>
          );
        })}
        {total > channels.length ? (
          <AppTag className='router-tag'>
            {t('billing.procurement_report.columns.more_channels', {
              count: total - channels.length,
            })}
          </AppTag>
        ) : null}
      </div>
    );
  };

  const columns = [
    {
      title:
        groupBy === 'model'
          ? t('billing.procurement_report.columns.model')
          : groupBy === 'endpoint'
            ? t('billing.procurement_report.columns.endpoint')
            : t('billing.procurement_report.columns.channel'),
      key: 'dimension',
      width: 240,
      render: (_, row) => {
        const label = row.dimension_name || row.dimension_key || '-';
        const key = (row.dimension_key || '').toString().trim();
        if (groupBy === 'channel' && key && key !== '-') {
          return (
            <Link
              className='billing-procurement-report-link'
              to={channelProcurementPath(key)}
            >
              {label}
            </Link>
          );
        }
        return label;
      },
    },
    ...(groupBy === 'model'
      ? [
          {
            title: t('billing.procurement_report.columns.related_channels'),
            key: 'unconfigured_channels',
            width: 220,
            render: (_, row) => renderUnconfiguredChannels(row),
          },
        ]
      : []),
    {
      title: t('billing.procurement_report.columns.router_consumed_yyc'),
      dataIndex: 'router_consumed_yyc',
      width: 132,
      align: 'right',
      render: formatCount,
    },
    {
      title: t('billing.procurement_report.columns.request_count'),
      dataIndex: 'request_count',
      width: 100,
      align: 'right',
      render: formatCount,
    },
    {
      title: t('billing.procurement_report.columns.configured_count'),
      dataIndex: 'configured_cost_request_count',
      width: 132,
      align: 'right',
      render: formatCount,
    },
    {
      title: t('billing.procurement_report.columns.unconfigured_count'),
      dataIndex: 'unconfigured_cost_request_count',
      width: 132,
      align: 'right',
      render: (value) =>
        Number(value || 0) > 0 ? (
          <AppTag color='orange'>{formatCount(value)}</AppTag>
        ) : (
          '-'
        ),
    },
    {
      title: t('billing.procurement_report.columns.attribution_processing'),
      key: 'attribution_processing',
      width: 150,
      render: (_, row) => {
        const pending = Number(row.pending_cost_request_count || 0);
        const retry = Number(row.retry_cost_request_count || 0);
        if (pending <= 0 && retry <= 0) return '-';
        return (
          <div className='billing-procurement-report-channel-links'>
            {pending > 0 ? <AppTag>{t('billing.procurement_report.status.pending', { count: formatCount(pending) })}</AppTag> : null}
            {retry > 0 ? <AppTag color='red'>{t('billing.procurement_report.status.retry', { count: formatCount(retry) })}</AppTag> : null}
          </div>
        );
      },
    },
    {
      title: t('billing.procurement_report.columns.sell_amount'),
      dataIndex: 'sell_base_amount',
      width: 132,
      align: 'right',
      render: formatCNY,
    },
    {
      title: t('billing.procurement_report.columns.procurement_cost'),
      dataIndex: 'procurement_cost_base_amount',
      width: 132,
      align: 'right',
      render: formatCNY,
    },
    {
      title: t('billing.procurement_report.columns.gross_profit'),
      dataIndex: 'gross_profit_base_amount',
      width: 132,
      align: 'right',
      render: formatCNY,
    },
    {
      title: t('billing.procurement_report.columns.gross_profit_yyc'),
      dataIndex: 'gross_profit_yyc',
      width: 132,
      align: 'right',
      render: formatYYC,
    },
    {
      title: t('billing.procurement_report.columns.gross_margin'),
      dataIndex: 'gross_margin',
      width: 100,
      align: 'right',
      render: formatPercent,
    },
  ];

  const retryColumns = [
    {
      title: t('billing.procurement_report.retry.columns.created_at'),
      dataIndex: 'created_at',
      width: 168,
      render: (value) => (value ? timestamp2string(value) : '-'),
    },
    {
      title: t('billing.procurement_report.retry.columns.channel'),
      key: 'channel',
      width: 180,
      render: (_, row) => {
        const channelID = String(row.channel_id || '').trim();
        const label = String(row.channel_name || channelID || '-').trim();
        return channelID ? (
          <Link className='billing-procurement-report-link' to={channelProcurementPath(channelID)}>
            {label}
          </Link>
        ) : label;
      },
    },
    {
      title: t('billing.procurement_report.retry.columns.model'),
      dataIndex: 'model',
      width: 180,
      render: (value) => value || '-',
    },
    {
      title: t('billing.procurement_report.retry.columns.endpoint'),
      dataIndex: 'endpoint',
      width: 150,
      render: (value) => value || '-',
    },
    {
      title: t('billing.procurement_report.retry.columns.sell_amount'),
      dataIndex: 'sell_base_amount',
      width: 120,
      align: 'right',
      render: formatCNY,
    },
    {
      title: t('billing.procurement_report.retry.columns.retry_count'),
      dataIndex: 'retry_count',
      width: 96,
      align: 'right',
      render: formatCount,
    },
    {
      title: t('billing.procurement_report.retry.columns.last_retry_at'),
      dataIndex: 'last_retry_at',
      width: 168,
      render: (value) => (value ? timestamp2string(value) : '-'),
    },
    {
      title: t('billing.procurement_report.retry.columns.last_error'),
      dataIndex: 'last_error',
      width: 260,
      render: (value) => (
        <span className='billing-procurement-report-error' title={value || ''}>
          {value || '-'}
        </span>
      ),
    },
    {
      title: t('billing.procurement_report.retry.columns.action'),
      key: 'action',
      width: 150,
      fixed: 'right',
      render: (_, row) => (
        <div className='billing-procurement-report-actions'>
          <Link className='billing-procurement-report-link' to={`/admin/log/${encodeURIComponent(row.id)}`}>
            {t('common.view')}
          </Link>
          <AppPopconfirm
            title={t('billing.procurement_report.retry.retry_confirm')}
            onConfirm={() => retryProcurementAttribution(row.id)}
          >
            <AppButton
              className='router-table-button'
              size='small'
              loading={retryingLogID === row.id}
            >
              {t('billing.procurement_report.retry.retry_now')}
            </AppButton>
          </AppPopconfirm>
        </div>
      ),
    },
  ];

  const healthStatusClass = `is-${health.status || 'ok'}`;
  const healthIssues = health.issues.slice(0, 4);
  // Detect the drill-down source from `return_to`, tolerant of both the new
  // `?tab=` form and the legacy `/admin/finance/<seg>` pathname form.
  const returnTo = initialContext.returnTo || '';
  const returnToTabKey = returnTo.includes('?')
    ? new URLSearchParams(returnTo.slice(returnTo.indexOf('?') + 1)).get('tab')
    : null;
  const fromOverview = returnToTabKey === 'overview' || returnTo.startsWith('/admin/finance/overview');
  const fromProfit = returnToTabKey === 'profit' || returnTo.startsWith('/admin/finance/profit');
  const baseBreadcrumbs = [
    { key: 'finance', label: t('header.finance') },
    ...(fromOverview ? [{ key: 'overview', label: t('billing.overview.title'), onClick: () => navigate(initialContext.returnTo) }] : []),
    ...(fromProfit ? [{ key: 'profit', label: t('billing.pricing_analysis.title'), onClick: () => navigate(initialContext.returnTo) }] : []),
  ];

  return (
    <div className={`${embedded ? '' : 'dashboard-container '}billing-procurement-report-page`}>
      <AppFilterHeader
        breadcrumbs={embedded ? undefined : [
          ...baseBreadcrumbs,
          {
            key: 'procurement-report',
            label: t('billing.procurement_report.title'),
            active: true,
          },
        ]}
        actions={
          <>
            <AppButton
              className='router-page-button'
              loading={loading || healthLoading}
              onClick={() => {
                exportCSV(
                  `procurement-report-${startAt.replace(/[:T]/g, '-')}_${endAt.replace(/[:T]/g, '-')}.csv`,
                  [
                    { key: 'dimension_key', label: t('billing.procurement_report.columns.dimension') },
                    { key: 'request_count', label: t('billing.procurement_report.columns.request_count') },
                    { key: 'configured_cost_request_count', label: t('billing.procurement_report.columns.configured_count') },
                    { key: 'unconfigured_cost_request_count', label: t('billing.procurement_report.columns.unconfigured_count') },
                    { key: 'sell_base_amount', label: t('billing.procurement_report.columns.sell_amount'), format: formatCsvCurrency },
                    { key: 'procurement_cost_base_amount', label: t('billing.procurement_report.columns.procurement_cost'), format: formatCsvCurrency },
                    { key: 'gross_profit_base_amount', label: t('billing.procurement_report.columns.gross_profit'), format: formatCsvCurrency },
                    { key: 'gross_profit_yyc', label: t('billing.procurement_report.columns.gross_profit_yyc'), format: formatCsvCurrency },
                    { key: 'gross_margin', label: t('billing.procurement_report.columns.gross_margin'), format: formatCsvPercent },
                  ],
                  report.items,
                );
                showSuccess(t('billing.export.success', { count: report.items.length }));
              }}
            >
              {t('common.export_csv')}
            </AppButton>
            <AppButton
              className='router-page-button'
              color='blue'
              loading={loading || healthLoading}
              onClick={() => {
                loadHealth().then();
                loadReport().then();
                loadRetries().then();
              }}
            >
              {t('common.refresh')}
            </AppButton>
          </>
        }
        query={
          <div className='billing-procurement-report-filters'>
            <AppSegmented
              className='billing-procurement-report-segmented'
              options={[
                { value: 'channel' },
                { value: 'model' },
                { value: 'endpoint' },
              ].map((item) => ({
                ...item,
                label: t(`billing.procurement_report.group_by.${item.value}`),
              }))}
              value={groupBy}
              onChange={(e, { value }) => setGroupBy(value)}
            />
            <AppSegmented
              className='billing-procurement-report-segmented'
              options={[
                { value: 'all' },
                { value: 'unconfigured' },
              ].map((item) => ({
                ...item,
                label: t(`billing.procurement_report.cost_scope.${item.value}`),
              }))}
              value={costScope}
              onChange={(e, { value }) => setCostScope(value)}
            />
            <AppInput
              className='router-section-input billing-procurement-report-time-input'
              type='datetime-local'
              value={startAt}
              onChange={(e, { value }) => setStartAt(value)}
            />
            <AppInput
              className='router-section-input billing-procurement-report-time-input'
              type='datetime-local'
              value={endAt}
              onChange={(e, { value }) => setEndAt(value)}
            />
            <AppSelect
              className='router-section-input billing-procurement-report-group-select'
              clearable
              search
              options={groupOptions}
              value={groupID}
              placeholder={t('billing.procurement_report.filters.group')}
              onChange={(e, { value }) => setGroupID((value || '').toString())}
            />
            <AppSelect
              className='router-section-input billing-procurement-report-group-select'
              clearable
              search
              options={providerOptions}
              value={provider}
              placeholder={t('billing.procurement_report.filters.provider')}
              onChange={(e, { value }) => setProvider((value || '').toString())}
            />
            <AppSelect
              className='router-section-input billing-procurement-report-group-select'
              clearable
              search
              options={modelOptions}
              value={model}
              placeholder={t('billing.procurement_report.filters.model')}
              onChange={(e, { value }) => setModel((value || '').toString())}
            />
          </div>
        }
      />
      <AppSpin spinning={loading}>
        {loadError ? (
            <AppErrorState
              message={t('billing.procurement_report.messages.load_failed')}
              onRetry={() => loadReport().then()}
              retryText={t('common.retry')}
            />
          ) : (
          <div className='billing-procurement-report-overview'>
          <div className={`billing-procurement-report-health ${healthStatusClass}`}>
            <div className='billing-procurement-report-health-main'>
              <div className='billing-procurement-report-health-title'>
                {t(`billing.procurement_report.health.status.${health.status || 'ok'}`)}
              </div>
              <div className='billing-procurement-report-health-meta'>
                {health.status === 'unknown'
                  ? t('billing.procurement_report.health.summary_unknown')
                  : t('billing.procurement_report.health.summary', {
                      critical: health.critical_count,
                      warning: health.warning_count,
                    })}
              </div>
            </div>
            <div className='billing-procurement-report-health-issues'>
              {healthIssues.length === 0 ? (
                <span className='billing-procurement-report-health-ok'>
                  {t('billing.procurement_report.health.no_issue')}
                </span>
              ) : (
                healthIssues.map((issue) => {
                  const content = (
                    <>
                      <span className={`billing-procurement-report-health-level is-${issue.level || 'warning'}`}>
                        {t(`billing.procurement_report.health.level.${issue.level || 'warning'}`)}
                      </span>
                      <span className='billing-procurement-report-health-text'>
                        {issue.title}
                        {issue.count > 0 ? ` (${formatCount(issue.count)})` : ''}
                      </span>
                    </>
                  );
                  return issue.link ? (
                    <Link
                      key={issue.key}
                      className='billing-procurement-report-health-issue'
                      to={issue.link}
                      title={issue.message}
                    >
                      {content}
                    </Link>
                  ) : (
                    <span
                      key={issue.key}
                      className='billing-procurement-report-health-issue'
                      title={issue.message}
                    >
                      {content}
                    </span>
                  );
                })
              )}
            </div>
          </div>
          <div className={`billing-procurement-report-headline is-${headlineTone}`}>
            <div className='billing-procurement-report-headline-main'>
              <span className='billing-procurement-report-headline-label'>{t('billing.procurement_report.summary.gross_profit')}</span>
              <span className='billing-procurement-report-headline-value'>{headlineProfit}</span>
              <span className='billing-procurement-report-headline-sub'>{t('billing.procurement_report.summary.gross_margin')}: {headlineMargin}</span>
            </div>
            <div className='billing-procurement-report-headline-side'>
              {headlineSideStats.map((stat) => (
                <div
                  key={stat.key}
                  className={`billing-procurement-report-headline-stat${stat.tone ? ` is-${stat.tone}` : ''}`}
                >
                  <span className='billing-procurement-report-headline-stat-label'>{stat.label}</span>
                  <span className='billing-procurement-report-headline-stat-value'>{stat.value}</span>
                </div>
              ))}
            </div>
          </div>
          {lossLeaderboard.length > 0 ? (
            <section className='billing-procurement-report-loss-section'>
              <div className='billing-procurement-report-analytics-head'>
                <h3>{t('billing.procurement_report.analytics.loss_leaderboard_title')}</h3>
                <span>{t('billing.procurement_report.analytics.loss_leaderboard_hint')}</span>
              </div>
              <div className='billing-procurement-report-loss-list'>
                {lossLeaderboard.slice(0, 5).map((row) => (
                  <div
                    key={row.key}
                    className={`billing-procurement-report-loss-row${row.unknown ? ' is-unknown' : ''}`}
                  >
                    <span className='billing-procurement-report-loss-label' title={row.label}>{row.label}</span>
                    <span className='billing-procurement-report-loss-profit'>{formatCNY(row.profit)}</span>
                    <span className='billing-procurement-report-loss-margin'>
                      {row.unknown
                        ? t('billing.procurement_report.analytics.margin_bucket.unknown')
                        : formatPercent(row.margin)}
                    </span>
                  </div>
                ))}
              </div>
            </section>
          ) : null}
          {(providerBreakdown.length > 0 || marginDistribution.some((item) => item.count > 0)) ? (
            <section className='billing-procurement-report-advanced'>
              <button
                type='button'
                className='billing-procurement-report-advanced-toggle'
                onClick={() => setShowAdvancedAnalytics((prev) => !prev)}
                aria-expanded={showAdvancedAnalytics}
                aria-label={t('billing.procurement_report.analytics.advanced_toggle')}
              >
                <span>{t('billing.procurement_report.analytics.advanced_toggle')}</span>
                <span className='billing-procurement-report-advanced-chevron' aria-hidden='true'>{showAdvancedAnalytics ? '▾' : '▸'}</span>
              </button>
              {showAdvancedAnalytics ? (
                <div className='billing-procurement-report-analytics'>
                  <div className='billing-procurement-report-analytics-card'>
                    <div className='billing-procurement-report-analytics-title'>
                      {t('billing.procurement_report.analytics.by_provider_title')}
                    </div>
                    <div className='billing-procurement-report-analytics-hint'>
                      {t('billing.procurement_report.analytics.by_provider_hint')}
                    </div>
                    {providerBreakdown.length === 0 ? (
                      <div className='billing-procurement-report-empty'>
                        {t('billing.procurement_report.empty')}
                      </div>
                    ) : (
                      <div className='chart-container billing-procurement-report-pie'>
                        <ResponsiveContainer width='100%' height={220}>
                          <PieChart>
                            <Tooltip
                              contentStyle={chartTooltipStyle()}
                              formatter={(value, key) => [formatCnyChart(value), key]}
                            />
                            <Legend
                              layout='vertical'
                              align='right'
                              verticalAlign='middle'
                              wrapperStyle={{ fontSize: 12 }}
                            />
                            <Pie
                              data={providerBreakdown.map((item) => ({
                                ...item,
                                value: item.cost,
                              }))}
                              dataKey='value'
                              nameKey='label'
                              innerRadius={48}
                              outerRadius={80}
                              paddingAngle={2}
                              stroke={getActiveChartTheme().surface}
                              strokeWidth={2}
                            >
                              {providerBreakdown.map((item) => (
                                <Cell key={item.key} fill={colorForKey(item.key)} />
                              ))}
                            </Pie>
                          </PieChart>
                        </ResponsiveContainer>
                      </div>
                    )}
                  </div>
                  <div className='billing-procurement-report-analytics-card'>
                    <div className='billing-procurement-report-analytics-title'>
                      {t('billing.procurement_report.analytics.margin_distribution_title')}
                    </div>
                    <div className='billing-procurement-report-analytics-hint'>
                      {t('billing.procurement_report.analytics.margin_distribution_hint')}
                    </div>
                    <div className='chart-container'>
                      <ResponsiveContainer width='100%' height={220}>
                        <BarChart data={marginDistribution}>
                          <CartesianGrid {...chartGridStyle()} />
                          <XAxis
                            dataKey='label'
                            {...chartAxisStyle()}
                            interval={0}
                            angle={-12}
                            dy={10}
                            height={50}
                          />
                          <YAxis {...chartAxisStyle()} allowDecimals={false} />
                          <Tooltip contentStyle={chartTooltipStyle()} />
                          <Bar dataKey='count' radius={[4, 4, 0, 0]}>
                            {marginDistribution.map((item) => (
                              <Cell key={item.key} fill={item.color} />
                            ))}
                          </Bar>
                        </BarChart>
                      </ResponsiveContainer>
                    </div>
                  </div>
                </div>
              ) : null}
            </section>
          ) : null}
          <div className='billing-overview-section-heading'>
            <h2>{t('billing.procurement_report.title')}</h2>
          </div>
          <AppTable
            className='router-detail-table router-table-fit-page billing-procurement-report-table router-table-cardify'
            rowKey={(row) => `${row.dimension_type}-${row.dimension_key}`}
            dataSource={report.items}
            columns={withCardLabels(columns)}
            pagination={false}
            scroll={{ x: groupBy === 'model' ? 1470 : 1250 }}
            locale={{
              emptyText: loading
                ? t('common.loading')
                : t('billing.procurement_report.empty'),
            }}
          />
        {retryItems.length > 0 ? (
          <div className='billing-procurement-report-retries'>
          <div className='billing-overview-section-heading'>
            <h2>{t('billing.procurement_report.retry.title')}</h2>
            <AppButton
              className='router-page-button'
              loading={retryLoading}
              onClick={() => loadRetries().then()}
            >
              {t('common.refresh')}
            </AppButton>
          </div>
          <AppTable
            className='router-detail-table router-table-fit-page billing-procurement-report-table router-table-cardify'
            rowKey='id'
            dataSource={retryItems}
            columns={withCardLabels(retryColumns)}
            pagination={false}
            loading={retryLoading}
            scroll={{ x: 1470 }}
            locale={{
              emptyText: retryLoading
                ? t('common.loading')
                : t('billing.procurement_report.retry.empty'),
            }}
          />
          </div>
        ) : null}
          </div>
          )}
      </AppSpin>
    </div>
  );
}

export default BillingProcurementReport;
