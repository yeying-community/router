import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
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
  convertChargeAmountToDisplayAmount,
  formatCompactDisplayAmount,
  loadPublicDisplayCurrencyCatalog,
} from '../../helpers/billing';
import { formatAmountWithUnit } from '../../helpers/render';
import {
  AppButton,
  AppFilterHeader,
  AppInput,
  AppInputNumber,
  AppSegmented,
  AppSection,
  AppSelect,
  AppToolbar,
} from '../../router-ui';
import './Dashboard.css';

// 在 Dashboard 组件内添加自定义配置
const chartConfig = {
  lineChart: {
    style: {
      background: '#fff',
      borderRadius: '8px',
    },
    line: {
      strokeWidth: 2,
      dot: false,
      activeDot: { r: 4 },
    },
    grid: {
      vertical: false,
      horizontal: true,
      opacity: 0.1,
    },
  },
  colors: {
    requests: '#4318FF',
    cost: '#00B5D8',
    tokens: '#6C63FF',
  },
  barColors: [
    '#4318FF', // 深紫色
    '#00B5D8', // 青色
    '#6C63FF', // 紫色
    '#05CD99', // 绿色
    '#FFB547', // 橙色
    '#FF5E7D', // 粉色
    '#41B883', // 翠绿
    '#7983FF', // 淡紫
    '#FF8F6B', // 珊瑚色
    '#49BEFF', // 天蓝
  ],
};

const spanLimits = {
  hour: 720,
  day: 365,
  week: 52,
  month: 36,
  year: 10,
};

const calendarSpanDefaults = {
  hour: 24,
  day: 30,
  week: 12,
  month: 12,
  year: 5,
};

const pad2 = (value) => String(value).padStart(2, '0');

const formatDateInput = (date) => {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
};

const parseDateInput = (value) => {
  if (!value) return null;
  return new Date(`${value}T00:00:00`);
};

const addDays = (date, days) => {
  const next = new Date(date);
  next.setDate(next.getDate() + days);
  return next;
};

const addMonths = (date, months) => {
  const next = new Date(date);
  const day = next.getDate();
  next.setDate(1);
  next.setMonth(next.getMonth() + months);
  const maxDay = new Date(next.getFullYear(), next.getMonth() + 1, 0).getDate();
  next.setDate(Math.min(day, maxDay));
  return next;
};

const addYears = (date, years) => {
  const next = new Date(date);
  const month = next.getMonth();
  const day = next.getDate();
  next.setFullYear(next.getFullYear() + years, month, 1);
  const maxDay = new Date(next.getFullYear(), month + 1, 0).getDate();
  next.setDate(Math.min(day, maxDay));
  return next;
};

const startOfDay = (date) => {
  const next = new Date(date);
  next.setHours(0, 0, 0, 0);
  return next;
};

const endOfDay = (date) => {
  const next = startOfDay(date);
  next.setDate(next.getDate() + 1);
  next.setMilliseconds(-1);
  return next;
};

const normalizeDashboardRow = (item) => ({
  ...item,
  chargeAmount: Number(item?.charge_amount ?? item?.Quota ?? 0),
});

const normalizeOverviewSummary = (data) => ({
  ...(data || {}),
  today_cost: Number(data?.today_charge_amount ?? data?.today_cost ?? 0),
  today_revenue: Number(data?.today_revenue_amount ?? data?.today_revenue ?? 0),
  yesterday_cost: Number(data?.yesterday_charge_amount ?? data?.yesterday_cost ?? 0),
  yesterday_revenue: Number(data?.yesterday_revenue_amount ?? data?.yesterday_revenue ?? 0),
  period_cost: Number(data?.period_charge_amount ?? data?.period_cost ?? 0),
  period_revenue: Number(data?.period_revenue_amount ?? data?.period_revenue ?? 0),
});

const CALENDAR_VIEW_OPTIONS = [
  { value: 'calendar', labelKey: 'dashboard.spending.calendar.view.calendar' },
  { value: 'bar', labelKey: 'dashboard.spending.calendar.view.bar' },
];

const CALENDAR_GRANULARITY_OPTIONS = [
  'hour',
  'day',
  'week',
  'month',
  'year',
].map((value) => ({
  value,
  labelKey: `dashboard.spending.calendar.granularity.${value}`,
}));

const CALENDAR_UNIT_OPTIONS = [
  { value: 'usd', labelKey: 'dashboard.spending.calendar.unit.usd' },
  { value: 'token', labelKey: 'dashboard.spending.calendar.unit.token' },
];

const startOfWeek = (date) => {
  const next = startOfDay(date);
  const day = next.getDay() || 7;
  next.setDate(next.getDate() - (day - 1));
  return next;
};

const formatHourLabel = (date) => {
  const year = date.getFullYear();
  const month = pad2(date.getMonth() + 1);
  const day = pad2(date.getDate());
  const hour = pad2(date.getHours());
  return `${year}-${month}-${day} ${hour}`;
};

const formatMonthLabel = (date) => {
  const year = date.getFullYear();
  const month = pad2(date.getMonth() + 1);
  return `${year}-${month}`;
};

const formatYearLabel = (date) => `${date.getFullYear()}`;

const getISOWeekLabel = (date) => {
  const temp = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()));
  const day = temp.getUTCDay() || 7;
  temp.setUTCDate(temp.getUTCDate() + 4 - day);
  const yearStart = new Date(Date.UTC(temp.getUTCFullYear(), 0, 1));
  const week = Math.ceil(((temp - yearStart) / 86400000 + 1) / 7);
  return `${temp.getUTCFullYear()}-W${pad2(week)}`;
};

const buildBucketLabels = (startTimestamp, endTimestamp, granularity) => {
  if (!startTimestamp || !endTimestamp) return [];
  let start = new Date(startTimestamp * 1000);
  let end = new Date(endTimestamp * 1000);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return [];
  if (start > end) {
    const temp = start;
    start = end;
    end = temp;
  }
  const labels = [];
  switch (granularity) {
    case 'hour': {
      start.setMinutes(0, 0, 0);
      end.setMinutes(59, 59, 999);
      for (let d = new Date(start); d <= end; d.setHours(d.getHours() + 1)) {
        labels.push(formatHourLabel(d));
      }
      break;
    }
    case 'week': {
      const weekStart = startOfWeek(start);
      const weekEnd = startOfWeek(end);
      for (let d = new Date(weekStart); d <= weekEnd; d.setDate(d.getDate() + 7)) {
        labels.push(getISOWeekLabel(d));
      }
      break;
    }
    case 'month': {
      const monthStart = new Date(start.getFullYear(), start.getMonth(), 1);
      const monthEnd = new Date(end.getFullYear(), end.getMonth(), 1);
      for (let d = new Date(monthStart); d <= monthEnd; d.setMonth(d.getMonth() + 1)) {
        labels.push(formatMonthLabel(d));
      }
      break;
    }
    case 'year': {
      const yearStart = new Date(start.getFullYear(), 0, 1);
      const yearEnd = new Date(end.getFullYear(), 0, 1);
      for (let d = new Date(yearStart); d <= yearEnd; d.setFullYear(d.getFullYear() + 1)) {
        labels.push(formatYearLabel(d));
      }
      break;
    }
    default: {
      const dayStart = startOfDay(start);
      const dayEnd = startOfDay(end);
      for (let d = new Date(dayStart); d <= dayEnd; d.setDate(d.getDate() + 1)) {
        labels.push(formatDateInput(d));
      }
      break;
    }
  }
  return labels;
};

const getCalendarRange = (granularity) => {
  const span = calendarSpanDefaults[granularity] || 30;
  const now = new Date();
  if (granularity === 'hour') {
    const end = new Date(now);
    end.setMinutes(59, 59, 999);
    const start = new Date(end);
    start.setHours(start.getHours() - (span - 1), 0, 0, 0);
    return {
      start: Math.floor(start.getTime() / 1000),
      end: Math.floor(end.getTime() / 1000),
    };
  }
  const end = endOfDay(now);
  let start = new Date(end);
  switch (granularity) {
    case 'week':
      start = startOfWeek(end);
      start.setDate(start.getDate() - (span - 1) * 7);
      break;
    case 'month':
      start = new Date(end.getFullYear(), end.getMonth(), 1);
      start.setMonth(start.getMonth() - (span - 1));
      break;
    case 'year':
      start = new Date(end.getFullYear(), 0, 1);
      start.setFullYear(start.getFullYear() - (span - 1));
      break;
    default:
      start = startOfDay(end);
      start.setDate(start.getDate() - (span - 1));
      break;
  }
  return {
    start: Math.floor(start.getTime() / 1000),
    end: Math.floor(end.getTime() / 1000),
  };
};

const calculateEndDateFromStart = (startDate, granularity, span) => {
  let end = new Date(startDate);
  switch (granularity) {
    case 'week':
      end = addDays(end, span * 7 - 1);
      break;
    case 'month':
      end = addMonths(end, span);
      end = addDays(end, -1);
      break;
    case 'year':
      end = addYears(end, span);
      end = addDays(end, -1);
      break;
    default:
      end = addDays(end, span - 1);
      break;
  }
  return end;
};

const calculateSpanFromRange = (startDate, endDate, granularity) => {
  if (!startDate || !endDate) return 1;
  const start = new Date(startDate);
  const end = new Date(endDate);
  if (end < start) return 1;
  const diffDays = Math.floor((end - start) / (24 * 60 * 60 * 1000));
  switch (granularity) {
    case 'week':
      return Math.ceil((diffDays + 1) / 7);
    case 'month': {
      const months =
        (end.getFullYear() - start.getFullYear()) * 12 +
        (end.getMonth() - start.getMonth());
      return months + 1;
    }
    case 'year':
      return end.getFullYear() - start.getFullYear() + 1;
    default:
      return diffDays + 1;
  }
};

const getDefaultRange = (granularity, span) => {
  const end = new Date();
  end.setHours(0, 0, 0, 0);
  let start = new Date(end);
  switch (granularity) {
    case 'week':
      start = addDays(end, -(span * 7 - 1));
      break;
    case 'month':
      start = addDays(addMonths(end, -span), 1);
      break;
    case 'year':
      start = addDays(addYears(end, -span), 1);
      break;
    default:
      start = addDays(end, -(span - 1));
      break;
  }
  const endDate = calculateEndDateFromStart(start, granularity, span);
  return {
    start: formatDateInput(start),
    end: formatDateInput(endDate),
  };
};

const Dashboard = () => {
  const { t } = useTranslation();
  const [data, setData] = useState([]);
  const [providers, setProviders] = useState({});
  const [selectedModels, setSelectedModels] = useState([]);
  const [selectionReady, setSelectionReady] = useState(false);
  const [granularity, setGranularity] = useState('day');
  const [span, setSpan] = useState(7);
  const [spanAuto, setSpanAuto] = useState(true);
  const [endAuto, setEndAuto] = useState(true);
  const initialRange = useMemo(() => getDefaultRange('day', 7), []);
  const [startDate, setStartDate] = useState(initialRange.start);
  const [endDate, setEndDate] = useState(initialRange.end);
  const [overviewPeriod, setOverviewPeriod] = useState('last_30_days');
  const [overviewSummary, setOverviewSummary] = useState(null);
  const [overviewMetric, setOverviewMetric] = useState('cost');
  const [overviewTrendData, setOverviewTrendData] = useState([]);
  const [calendarView, setCalendarView] = useState('calendar');
  const [calendarGranularity, setCalendarGranularity] = useState('day');
  const [calendarUnit, setCalendarUnit] = useState('usd');
  const [calendarData, setCalendarData] = useState([]);
  const [activeFilters, setActiveFilters] = useState(['time']);
  const [dailyPackageBalanceSummary, setDailyPackageBalanceSummary] = useState(null);
  const [dailyPackageBalanceLoading, setDailyPackageBalanceLoading] = useState(false);
  const [displayCurrencyIndex, setDisplayCurrencyIndex] = useState(() =>
    buildPublicDisplayCurrencyIndex([])
  );

  const loadDisplayCurrencies = useCallback(async () => {
    const { currencyIndex } = await loadPublicDisplayCurrencyCatalog();
    setDisplayCurrencyIndex(currencyIndex);
  }, []);

  const allModels = useMemo(
    () => Array.from(new Set(Object.values(providers).flat())),
    [providers]
  );

  const selectedModelsKey = useMemo(
    () => selectedModels.slice().sort().join(','),
    [selectedModels]
  );
  const overviewGranularity = useMemo(() => {
    switch (overviewPeriod) {
      case 'today':
        return 'hour';
      case 'last_7_days':
      case 'last_30_days':
      case 'this_month':
      case 'last_month':
        return 'day';
      default:
        return 'month';
    }
  }, [overviewPeriod]);
  const overviewRangeKey = useMemo(() => {
    if (!overviewSummary) return '0-0';
    return `${overviewSummary.period_start || 0}-${overviewSummary.period_end || 0}`;
  }, [overviewSummary]);

  useEffect(() => {
    if (!startDate || !endDate) return;
    if (selectionReady && selectedModels.length === 0) {
      setData([]);
      return;
    }
    fetchDashboardData();
  }, [granularity, startDate, endDate, selectedModelsKey, selectionReady]);

  useEffect(() => {
    if (allModels.length === 0) return;
    if (!selectionReady) {
      setSelectedModels(allModels);
      setSelectionReady(true);
      return;
    }
    setSelectedModels((prev) => prev.filter((model) => allModels.includes(model)));
  }, [allModels, selectionReady]);

  useEffect(() => {
    fetchOverviewSummary();
  }, [overviewPeriod]);

  useEffect(() => {
    if (!overviewSummary?.period_start || !overviewSummary?.period_end) {
      setOverviewTrendData([]);
      return;
    }
    fetchOverviewTrendData();
  }, [overviewRangeKey, overviewGranularity, selectedModelsKey, selectionReady]);

  useEffect(() => {
    fetchCalendarData();
  }, [calendarGranularity, selectedModelsKey, selectionReady]);

  useEffect(() => {
    fetchDailyPackageBalanceSummary();
  }, []);

  useEffect(() => {
    loadDisplayCurrencies().then();
  }, [loadDisplayCurrencies]);

  const toStartTimestamp = (dateStr) => {
    const date = parseDateInput(dateStr);
    if (!date) return 0;
    return Math.floor(date.getTime() / 1000);
  };

  const toEndTimestamp = (dateStr) => {
    if (!dateStr) return 0;
    const date = new Date(`${dateStr}T23:59:59`);
    return Math.floor(date.getTime() / 1000);
  };

  const fetchDashboardData = async () => {
    try {
      const startTimestamp = toStartTimestamp(startDate);
      const endTimestamp = toEndTimestamp(endDate);
      if (!startTimestamp || !endTimestamp) return;
      const params = {
        start_timestamp: startTimestamp,
        end_timestamp: endTimestamp,
        granularity,
        include_meta: 1,
      };
      const requestedModels = getRequestedModels();
      if (requestedModels.length > 0) {
        params.models = requestedModels.join(',');
      }
      const response = await API.get('/api/v1/public/user/dashboard', { params });
      if (response.data.success) {
        const dashboardData = Array.isArray(response.data.data)
          ? response.data.data.map(normalizeDashboardRow)
          : [];
        const meta = response.data.meta || {};
        setData(dashboardData);
        if (meta.providers) {
          setProviders(meta.providers);
        }
      }
    } catch (error) {
      console.error('Failed to fetch dashboard data:', error);
      setData([]);
    }
  };

  const getRequestedModels = () => {
    if (!selectionReady || selectedModels.length === 0) {
      return [];
    }
    if (allModels.length > 0 && selectedModels.length >= allModels.length) {
      return [];
    }
    return selectedModels;
  };

  const fetchOverviewSummary = async () => {
    try {
      const response = await API.get('/api/v1/public/user/spend/overview', {
        params: { period: overviewPeriod },
      });
      if (response.data.success) {
        setOverviewSummary(normalizeOverviewSummary(response.data.data || null));
        return;
      }
      setOverviewSummary(null);
    } catch (error) {
      console.error('Failed to fetch spend overview:', error);
      setOverviewSummary(null);
    }
  };

  const fetchOverviewTrendData = async () => {
    try {
      if (selectionReady && selectedModels.length === 0) {
        setOverviewTrendData([]);
        return;
      }
      const params = {
        start_timestamp: overviewSummary?.period_start || 0,
        end_timestamp: overviewSummary?.period_end || 0,
        granularity: overviewGranularity,
      };
      const requestedModels = getRequestedModels();
      if (requestedModels.length > 0) {
        params.models = requestedModels.join(',');
      }
      if (!params.start_timestamp || !params.end_timestamp) {
        setOverviewTrendData([]);
        return;
      }
      const response = await API.get('/api/v1/public/user/dashboard', { params });
      if (response.data.success) {
        setOverviewTrendData(
          Array.isArray(response.data.data)
            ? response.data.data.map(normalizeDashboardRow)
            : []
        );
        return;
      }
      setOverviewTrendData([]);
    } catch (error) {
      console.error('Failed to fetch spend trend data:', error);
      setOverviewTrendData([]);
    }
  };

  const fetchCalendarData = async () => {
    try {
      if (selectionReady && selectedModels.length === 0) {
        setCalendarData([]);
        return;
      }
      const range = getCalendarRange(calendarGranularity);
      if (!range.start || !range.end) {
        setCalendarData([]);
        return;
      }
      const requestedModels = getRequestedModels();
      const response = await API.get('/api/v1/public/user/dashboard', {
        params: {
          start_timestamp: range.start,
          end_timestamp: range.end,
          granularity: calendarGranularity,
          ...(requestedModels.length > 0 ? { models: requestedModels.join(',') } : {}),
        },
      });
      if (response.data.success) {
        setCalendarData(
          Array.isArray(response.data.data)
            ? response.data.data.map(normalizeDashboardRow)
            : []
        );
        return;
      }
      setCalendarData([]);
    } catch (error) {
      console.error('Failed to fetch calendar data:', error);
      setCalendarData([]);
    }
  };

  const fetchDailyPackageBalanceSummary = async () => {
    setDailyPackageBalanceLoading(true);
    try {
      const response = await API.get('/api/v1/public/user/quota/daily');
      if (response.data?.success) {
        const data = response.data.data || null;
        if (!data) {
          setDailyPackageBalanceSummary(null);
          return;
        }
        setDailyPackageBalanceSummary({
          ...data,
          limitAmount: Number(data?.limit_amount ?? data?.limit ?? 0),
          usedAmount: Number(data?.consumed_amount ?? data?.consumed_quota ?? 0),
          reservedAmount: Number(data?.reserved_amount ?? data?.reserved_quota ?? 0),
          remainingAmount: Number(data?.remaining_amount ?? data?.remaining_quota ?? 0),
        });
        return;
      }
      setDailyPackageBalanceSummary(null);
    } catch (error) {
      console.error('Failed to fetch daily package usage:', error);
      setDailyPackageBalanceSummary(null);
    } finally {
      setDailyPackageBalanceLoading(false);
    }
  };

  const toUsd = useCallback(
    (chargeAmount) => {
      const amount = convertChargeAmountToDisplayAmount(
        chargeAmount,
        'USD',
        displayCurrencyIndex
      );
      if (!Number.isFinite(amount)) {
        return 0;
      }
      return amount;
    },
    [displayCurrencyIndex]
  );

  const formatUsdAmount = useCallback(
    (amount) =>
      formatCompactDisplayAmount(amount, {
        compactLabel: t('dashboard.spending.labels.ten_thousand'),
      }),
    [t]
  );

  const formatYycAsUsd = useCallback(
    (chargeAmount) => formatUsdAmount(toUsd(chargeAmount)),
    [formatUsdAmount, toUsd]
  );

  const formatChargeAsUsdWithUnit = useCallback(
    (chargeAmount) => {
      const usdAmount = toUsd(chargeAmount);
      if (!Number.isFinite(usdAmount)) {
        return '-';
      }
      return formatAmountWithUnit(usdAmount, 'USD', 6);
    },
    [toUsd]
  );

  const formatCountValue = (value) => {
    const num = Number(value);
    if (!Number.isFinite(num)) return '0';
    return Math.round(num).toLocaleString();
  };

  const aggregateBucketData = (items) => {
    const map = new Map();
    items.forEach((item) => {
      const key = item.Day;
      if (!key) return;
      if (!map.has(key)) {
        map.set(key, { requests: 0, tokens: 0, chargeAmount: 0 });
      }
      const target = map.get(key);
      target.requests += item.RequestCount || 0;
      target.chargeAmount += item.chargeAmount || 0;
      target.tokens += (item.PromptTokens || 0) + (item.CompletionTokens || 0);
    });
    return map;
  };

  // 处理数据以供堆叠柱状图使用
  const processModelData = () => {
    const timeData = {};
    const models = [...new Set(data.map((item) => item.ModelName))];

    if (granularity === 'day') {
      const start = parseDateInput(startDate);
      const end = parseDateInput(endDate);
      if (!start || !end) return [];
      for (let d = new Date(start); d <= end; d.setDate(d.getDate() + 1)) {
        const dateStr = formatDateInput(d);
        timeData[dateStr] = {
          date: dateStr,
        };
        models.forEach((model) => {
          timeData[dateStr][model] = 0;
        });
      }
    }

    data.forEach((item) => {
      if (!timeData[item.Day]) {
        timeData[item.Day] = {
          date: item.Day,
        };
        models.forEach((model) => {
          timeData[item.Day][model] = 0;
        });
      }
      timeData[item.Day][item.ModelName] =
        item.PromptTokens + item.CompletionTokens;
    });

    return Object.values(timeData).sort((a, b) => a.date.localeCompare(b.date));
  };

  // 获取所有唯一的模型名称
  const getUniqueModels = () => {
    return [...new Set(data.map((item) => item.ModelName))];
  };

  const clampSpan = (value, unit) => {
    const limit = spanLimits[unit] || 1;
    if (!Number.isFinite(value)) return 1;
    return Math.min(Math.max(value, 1), limit);
  };

  const granularityOptions = [
    { key: 'hour', text: t('dashboard.filters.granularity_options.hour'), value: 'hour' },
    { key: 'day', text: t('dashboard.filters.granularity_options.day'), value: 'day' },
    { key: 'week', text: t('dashboard.filters.granularity_options.week'), value: 'week' },
    { key: 'month', text: t('dashboard.filters.granularity_options.month'), value: 'month' },
    { key: 'year', text: t('dashboard.filters.granularity_options.year'), value: 'year' },
  ];

  const overviewPeriodOptions = [
    { key: 'today', text: t('dashboard.spending.period.today'), value: 'today' },
    { key: 'last_7_days', text: t('dashboard.spending.period.last_7_days'), value: 'last_7_days' },
    { key: 'last_30_days', text: t('dashboard.spending.period.last_30_days'), value: 'last_30_days' },
    { key: 'this_month', text: t('dashboard.spending.period.this_month'), value: 'this_month' },
    { key: 'last_month', text: t('dashboard.spending.period.last_month'), value: 'last_month' },
    { key: 'this_year', text: t('dashboard.spending.period.this_year'), value: 'this_year' },
    { key: 'all_time', text: t('dashboard.spending.period.all_time'), value: 'all_time' },
  ];

  const handleGranularityChange = (e, { value }) => {
    const nextGranularity = value;
    const nextSpan = clampSpan(span, nextGranularity);
    const range = getDefaultRange(nextGranularity, nextSpan);
    setGranularity(nextGranularity);
    setSpan(nextSpan);
    setStartDate(range.start);
    setEndDate(range.end);
    setSpanAuto(true);
    setEndAuto(true);
  };

  const handleSpanChange = (e, { value }) => {
    const parsed = parseInt(value, 10);
    const nextSpan = clampSpan(Number.isNaN(parsed) ? 1 : parsed, granularity);
    setSpan(nextSpan);
    setSpanAuto(false);
    setEndAuto(true);
    if (!startDate) {
      const range = getDefaultRange(granularity, nextSpan);
      setStartDate(range.start);
      setEndDate(range.end);
      return;
    }
    const start = parseDateInput(startDate);
    if (!start) return;
    const end = calculateEndDateFromStart(start, granularity, nextSpan);
    setEndDate(formatDateInput(end));
  };

  const handleStartChange = (e, { value }) => {
    setStartDate(value);
    if (!value) return;
    const start = parseDateInput(value);
    if (!start) return;
    if (endAuto) {
      const end = calculateEndDateFromStart(start, granularity, span);
      setEndDate(formatDateInput(end));
      return;
    }
    if (!endDate) return;
    const end = parseDateInput(endDate);
    if (!end) return;
    const rawSpan = calculateSpanFromRange(start, end, granularity);
    const nextSpan = clampSpan(rawSpan, granularity);
    setSpan(nextSpan);
    setSpanAuto(false);
    if (nextSpan !== rawSpan) {
      const nextEnd = calculateEndDateFromStart(start, granularity, nextSpan);
      setEndDate(formatDateInput(nextEnd));
      setEndAuto(true);
    }
  };

  const handleEndChange = (e, { value }) => {
    setEndDate(value);
    setEndAuto(false);
    if (!value || !startDate) return;
    const start = parseDateInput(startDate);
    const end = parseDateInput(value);
    if (!start || !end) return;
    const rawSpan = calculateSpanFromRange(start, end, granularity);
    const nextSpan = clampSpan(rawSpan, granularity);
    setSpan(nextSpan);
    setSpanAuto(false);
    if (nextSpan !== rawSpan) {
      const nextEnd = calculateEndDateFromStart(start, granularity, nextSpan);
      setEndDate(formatDateInput(nextEnd));
      setEndAuto(true);
    }
  };

  const modelFilterOptions = useMemo(
    () =>
      allModels.map((model) => ({
        key: model,
        text: model,
        value: model,
      })),
    [allModels]
  );
  const addConditionOptions = useMemo(() => {
    const options = [
      {
        key: 'time',
        text: t('dashboard.filters.conditions.time'),
        value: 'time',
      },
      {
        key: 'models',
        text: t('dashboard.filters.conditions.models'),
        value: 'models',
      },
    ];
    return options.filter((option) => !activeFilters.includes(option.value));
  }, [activeFilters, t]);
  const hasTimeFilter = activeFilters.includes('time');
  const hasModelFilter = activeFilters.includes('models');

  const analysisScopeText = useMemo(() => {
    if (!selectionReady || selectedModels.length === 0) {
      return t('dashboard.spending.overview.scope.none');
    }
    if (allModels.length === 0 || selectedModels.length >= allModels.length) {
      return t('dashboard.spending.overview.scope.all');
    }
    return t('dashboard.spending.overview.scope.selected', {
      count: selectedModels.length,
    });
  }, [allModels, selectedModels, selectionReady, t]);

  const periodLabel = useMemo(
    () => t(`dashboard.spending.period.${overviewPeriod}`),
    [overviewPeriod, t]
  );

  const overviewLineData = useMemo(() => {
    if (!overviewSummary?.period_start || !overviewSummary?.period_end) return [];
    const bucketMap = aggregateBucketData(overviewTrendData);
    const labels = buildBucketLabels(
      overviewSummary.period_start,
      overviewSummary.period_end,
      overviewGranularity
    );
    const ordered = labels.length ? labels : Array.from(bucketMap.keys()).sort();
    return ordered.map((label) => {
      const bucket = bucketMap.get(label) || { requests: 0, tokens: 0, chargeAmount: 0 };
      return {
        date: label,
        requests: bucket.requests,
        tokens: bucket.tokens,
        cost: toUsd(bucket.chargeAmount),
      };
    });
  }, [overviewTrendData, overviewSummary, overviewGranularity, toUsd]);

  const calendarBuckets = useMemo(() => {
    const bucketMap = aggregateBucketData(calendarData);
    const range = getCalendarRange(calendarGranularity);
    const labels = buildBucketLabels(range.start, range.end, calendarGranularity);
    const ordered = labels.length ? labels : Array.from(bucketMap.keys()).sort();
    return ordered.map((label) => {
      const bucket = bucketMap.get(label) || { requests: 0, tokens: 0, chargeAmount: 0 };
      const value = calendarUnit === 'usd' ? toUsd(bucket.chargeAmount) : bucket.tokens;
      return {
        label,
        value,
        requests: bucket.requests,
      };
    });
  }, [calendarData, calendarGranularity, calendarUnit, toUsd]);

  const overviewMetricConfig = {
    requests: {
      key: 'requests',
      color: chartConfig.colors.requests,
      formatter: (value) => formatCountValue(value),
      label: t('dashboard.spending.metrics.requests'),
    },
    tokens: {
      key: 'tokens',
      color: chartConfig.colors.tokens,
      formatter: (value) => formatCountValue(value),
      label: t('dashboard.spending.metrics.tokens'),
    },
    cost: {
      key: 'cost',
      color: chartConfig.colors.cost,
      formatter: (value) => formatUsdAmount(value),
      label: t('dashboard.spending.metrics.cost'),
    },
  };
  const overviewMetricSetting =
    overviewMetricConfig[overviewMetric] || overviewMetricConfig.cost;
  const formatCalendarValue = (value) => {
    if (calendarUnit === 'usd') return formatUsdAmount(value);
    return formatCountValue(value);
  };

  const handleAddFilterCondition = (e, { value }) => {
    if (!value) return;
    setActiveFilters((prev) => {
      if (prev.includes(value)) return prev;
      return [...prev, value];
    });
  };

  const handleRemoveFilterCondition = (key) => {
    setActiveFilters((prev) => prev.filter((item) => item !== key));
    if (key === 'models') {
      setSelectedModels(allModels);
    }
  };

  const handleClearFilterConditions = () => {
    setActiveFilters([]);
    setSelectedModels(allModels);
  };

  const handleModelsFilterChange = (e, { value }) => {
    setSelectedModels(Array.isArray(value) ? value : []);
  };

  const modelData = processModelData();
  const models = getUniqueModels();

  // 生成随机颜色
  const getRandomColor = (index) => {
    return chartConfig.barColors[index % chartConfig.barColors.length];
  };

  const formatAxisLabel = (value) => {
    if (!value) return '';
    if (granularity !== 'day') return value;
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleDateString('zh-CN', {
      month: 'numeric',
      day: 'numeric',
    });
  };

  // 修改所有 XAxis 配置
  const xAxisConfig = {
    dataKey: 'date',
    axisLine: false,
    tickLine: false,
    tick: {
      fontSize: 12,
      fill: '#A3AED0',
      textAnchor: 'middle', // 文本居中对齐
    },
    tickFormatter: formatAxisLabel,
    interval: 0,
    minTickGap: 5,
    padding: { left: 30, right: 30 }, // 增加两侧的内边距，确保首尾标签完整显示
  };

  const packageLimitDisplay = useMemo(() => {
    if (!dailyPackageBalanceSummary) return '-';
    if (dailyPackageBalanceSummary.unlimited) return t('common.unlimited');
    return formatChargeAsUsdWithUnit(dailyPackageBalanceSummary.limitAmount || 0);
  }, [dailyPackageBalanceSummary, formatChargeAsUsdWithUnit, t]);

  const packageRemainingDisplay = useMemo(() => {
    if (!dailyPackageBalanceSummary) return '-';
    if (dailyPackageBalanceSummary.unlimited) return t('common.unlimited');
    return formatChargeAsUsdWithUnit(dailyPackageBalanceSummary.remainingAmount || 0);
  }, [dailyPackageBalanceSummary, formatChargeAsUsdWithUnit, t]);

  const packageUsedDisplay = useMemo(() => {
    if (!dailyPackageBalanceSummary) return '-';
    return formatChargeAsUsdWithUnit(dailyPackageBalanceSummary.usedAmount || 0);
  }, [dailyPackageBalanceSummary, formatChargeAsUsdWithUnit]);

  const calendarViewOptions = useMemo(
    () =>
      CALENDAR_VIEW_OPTIONS.map((item) => ({
        value: item.value,
        label: t(item.labelKey),
      })),
    [t],
  );

  const calendarGranularityOptions = useMemo(
    () =>
      CALENDAR_GRANULARITY_OPTIONS.map((item) => ({
        value: item.value,
        label: t(item.labelKey),
      })),
    [t],
  );

  const calendarUnitOptions = useMemo(
    () =>
      CALENDAR_UNIT_OPTIONS.map((item) => ({
        value: item.value,
        label: t(item.labelKey),
      })),
    [t],
  );

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'workspace', label: t('header.user_workspace') },
          { key: 'service', label: t('header.service') },
          { key: 'dashboard', label: t('header.dashboard'), active: true },
        ]}
        title={t('header.dashboard')}
      />
      <AppSection
        className='dashboard-spend-card'
        title={t('dashboard.spending.package_daily.title')}
        extra={
          <AppButton
            type='button'
            className='router-inline-button'
            loading={dailyPackageBalanceLoading}
            onClick={() => fetchDailyPackageBalanceSummary()}
          >
            {t('dashboard.admin.buttons.refresh')}
          </AppButton>
        }
      >
          <div className='dashboard-spend-summary'>
            <div className='dashboard-spend-period-row'>
              <div className='dashboard-spend-label'>
                {t('dashboard.spending.package_daily.group')}
                : {dailyPackageBalanceSummary?.group_name || '-'}
              </div>
            </div>
            <div className='dashboard-spend-summary-row dashboard-package-metrics-row'>
              <div className='dashboard-spend-metric'>
                <div className='dashboard-spend-label'>
                  {t('dashboard.spending.package_daily.remaining')}
                </div>
                <div className='dashboard-spend-value'>{packageRemainingDisplay}</div>
              </div>
              <div className='dashboard-spend-metric'>
                <div className='dashboard-spend-label'>
                  {t('dashboard.spending.package_daily.consumed')}
                </div>
                <div className='dashboard-spend-value'>{packageUsedDisplay}</div>
              </div>
              <div className='dashboard-spend-metric'>
                <div className='dashboard-spend-label'>
                  {t('dashboard.spending.package_daily.limit')}
                </div>
                <div className='dashboard-spend-value'>{packageLimitDisplay}</div>
              </div>
              <div className='dashboard-spend-metric'>
                <div className='dashboard-spend-label'>
                  {t('dashboard.spending.package_daily.biz_date')}
                </div>
                <div className='dashboard-spend-value'>
                  {dailyPackageBalanceSummary?.biz_date || '-'}
                </div>
              </div>
            </div>
          </div>
      </AppSection>
      <div className='dashboard-spend-section'>
        <div className='dashboard-spend-stack'>
          <AppSection
            className='dashboard-spend-card'
            title={t('dashboard.spending.calendar.title')}
          >
              <AppToolbar
                className='dashboard-calendar-toolbar'
                start={
                  <div className='calendar-view-toggle'>
                    <AppSegmented
                      className='dashboard-calendar-segmented'
                      options={calendarViewOptions}
                      value={calendarView}
                      onChange={(e, { value }) => setCalendarView(value)}
                    />
                  </div>
                }
                end={
                  <>
                    <div className='calendar-granularity-toggle'>
                      <AppSegmented
                        className='dashboard-calendar-segmented'
                        options={calendarGranularityOptions}
                        value={calendarGranularity}
                        onChange={(e, { value }) => setCalendarGranularity(value)}
                      />
                    </div>
                    <div className='calendar-unit-toggle'>
                      <AppSegmented
                        className='dashboard-calendar-segmented'
                        options={calendarUnitOptions}
                        value={calendarUnit}
                        onChange={(e, { value }) => setCalendarUnit(value)}
                      />
                    </div>
                  </>
                }
              />
              {calendarView === 'calendar' ? (
                <>
                  {calendarGranularity === 'day' && (
                    <div className='dashboard-calendar-weekdays'>
                      {['日', '一', '二', '三', '四', '五', '六'].map((label) => (
                        <div key={label} className='dashboard-calendar-weekday'>
                          {label}
                        </div>
                      ))}
                    </div>
                  )}
                  <div
                    className={`dashboard-calendar-grid ${
                      calendarGranularity === 'day' ? 'is-day' : 'is-card'
                    }`}
                  >
                    {calendarBuckets.length === 0 ? (
                      <div className='dashboard-calendar-empty'>
                        {t('dashboard.spending.calendar.empty')}
                      </div>
                    ) : (
                      calendarBuckets.map((item, index) => (
                        <div
                          key={item.label}
                          className={`dashboard-calendar-cell ${
                            index === 0 ? 'is-active' : ''
                          }`}
                        >
                          <div className='dashboard-calendar-label'>{item.label}</div>
                          <div className='dashboard-calendar-value'>
                            {formatCalendarValue(item.value)}
                          </div>
                        </div>
                      ))
                    )}
                  </div>
                </>
              ) : (
                <div className='chart-container'>
                  <ResponsiveContainer width='100%' height={220}>
                    <BarChart data={calendarBuckets}>
                      <CartesianGrid strokeDasharray='3 3' vertical={false} opacity={0.1} />
                      <XAxis
                        dataKey='label'
                        axisLine={false}
                        tickLine={false}
                        tick={{ fontSize: 12, fill: '#A3AED0' }}
                        minTickGap={10}
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
                        formatter={(value) => formatCalendarValue(value)}
                        labelFormatter={(label) =>
                          `${t('dashboard.statistics.tooltip.date')}: ${label}`
                        }
                      />
                      <Bar
                        dataKey='value'
                        fill={
                          calendarUnit === 'usd'
                            ? chartConfig.colors.cost
                            : chartConfig.colors.tokens
                        }
                        radius={[4, 4, 0, 0]}
                      />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              )}
          </AppSection>
        </div>
      </div>
      <AppSection
        className='dashboard-analysis-card'
        title={t('dashboard.spending.analysis.title')}
      >
          <AppFilterHeader
            className='dashboard-filter-toolbar router-block-gap-xs'
            picker={
              <AppSelect
                className='dashboard-add-condition'
                options={addConditionOptions}
                placeholder={t('dashboard.filters.add_condition')}
                disabled={addConditionOptions.length === 0}
                onChange={handleAddFilterCondition}
              />
            }
            actions={
              activeFilters.length > 0 ? (
                <AppButton
                  className='router-inline-button'
                  type='button'
                  onClick={handleClearFilterConditions}
                >
                  {t('dashboard.filters.clear_conditions')}
                </AppButton>
              ) : null
            }
          />
          {hasTimeFilter && (
            <div className='dashboard-active-condition dashboard-time-condition'>
              <span className='dashboard-condition-tag'>
                {t('dashboard.filters.conditions.time')}
              </span>
              <div className='dashboard-inline-field'>
                <span className='dashboard-inline-label'>{t('dashboard.filters.granularity')}</span>
                <AppSelect
                  className='dashboard-inline-dropdown'
                  options={granularityOptions}
                  value={granularity}
                  onChange={handleGranularityChange}
                />
              </div>
              <div className='dashboard-inline-field'>
                <span className='dashboard-inline-label'>{t('dashboard.filters.start')}</span>
                <AppInput
                  className='dashboard-inline-input'
                  type='date'
                  value={startDate}
                  onChange={handleStartChange}
                />
              </div>
              <div className='dashboard-inline-field'>
                <span className='dashboard-inline-label'>{t('dashboard.filters.span')}</span>
                <AppInputNumber
                  className={`dashboard-inline-input dashboard-inline-number ${spanAuto ? 'dashboard-muted' : ''}`.trim()}
                  min={1}
                  max={spanLimits[granularity]}
                  value={span}
                  onChange={handleSpanChange}
                />
              </div>
              <div className='dashboard-inline-field'>
                <span className='dashboard-inline-label'>{t('dashboard.filters.end')}</span>
                <AppInput
                  className={`dashboard-inline-input ${endAuto ? 'dashboard-muted' : ''}`.trim()}
                  type='date'
                  value={endDate}
                  onChange={handleEndChange}
                />
              </div>
              <AppButton
                className='router-inline-button'
                type='button'
                onClick={() => handleRemoveFilterCondition('time')}
              >
                {t('dashboard.filters.remove_condition')}
              </AppButton>
            </div>
          )}
          {hasModelFilter && (
            <div className='dashboard-active-condition'>
              <span className='dashboard-condition-tag'>
                {t('dashboard.filters.conditions.models')}
              </span>
              <AppSelect
                className='dashboard-model-condition-dropdown'
                multiple
                search
                options={modelFilterOptions}
                value={selectedModels}
                placeholder={t('dashboard.filters.model_placeholder')}
                noResultsMessage={t('dashboard.filters.no_providers')}
                onChange={handleModelsFilterChange}
              />
              <AppButton
                className='router-inline-button'
                type='button'
                onClick={() => handleRemoveFilterCondition('models')}
              >
                {t('dashboard.filters.remove_condition')}
              </AppButton>
            </div>
          )}
          {!hasTimeFilter && !hasModelFilter && (
            <div className='dashboard-filter-hint'>
              {t('dashboard.filters.hint')}
            </div>
          )}
          {hasModelFilter && modelFilterOptions.length === 0 && (
            <div className='dashboard-provider-empty'>{t('dashboard.filters.no_providers')}</div>
          )}
          {hasModelFilter && modelFilterOptions.length > 0 && (
            <div className='dashboard-filter-hint'>
              {t('dashboard.filters.model_note', { scope: analysisScopeText })}
            </div>
          )}
          <div className='chart-container'>
            <ResponsiveContainer width='100%' height={300}>
              <BarChart data={modelData}>
                <CartesianGrid
                  strokeDasharray='3 3'
                  vertical={false}
                  opacity={0.1}
                />
                <XAxis {...xAxisConfig} />
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
                  labelFormatter={(label) =>
                    `${t('dashboard.statistics.tooltip.date')}: ${label}`
                  }
                />
                <Legend
                  wrapperStyle={{
                    paddingTop: '20px',
                  }}
                />
                {models.map((model, index) => (
                  <Bar
                    key={model}
                    dataKey={model}
                    stackId='a'
                    fill={getRandomColor(index)}
                    name={model}
                    radius={[4, 4, 0, 0]}
                  />
                ))}
              </BarChart>
            </ResponsiveContainer>
          </div>
      </AppSection>
    </div>
  );
};

export default Dashboard;
