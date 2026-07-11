import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import {
  Bar,
  BarChart,
  CartesianGrid,
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
import {
  AppSection,
  AppSegmented,
  AppToolbar,
} from '../../router-ui';
import './SpendingCalendar.css';

const chartColors = {
  cost: '#00B5D8',
  tokens: '#6C63FF',
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
  const month = pad2(date.getMonth() + 1);
  const day = pad2(date.getDate());
  return `${year}-${month}-${day}`;
};

const formatDatetimeLocalInput = (date) => {
  const year = date.getFullYear();
  const month = pad2(date.getMonth() + 1);
  const day = pad2(date.getDate());
  const hour = pad2(date.getHours());
  const minute = pad2(date.getMinutes());
  return `${year}-${month}-${day}T${hour}:${minute}`;
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

const formatMonthLabel = (date) => `${date.getFullYear()}-${pad2(date.getMonth() + 1)}`;

const formatYearLabel = (date) => `${date.getFullYear()}`;

const getISOWeekLabel = (date) => {
  const temp = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()));
  const day = temp.getUTCDay() || 7;
  temp.setUTCDate(temp.getUTCDate() + 4 - day);
  const yearStart = new Date(Date.UTC(temp.getUTCFullYear(), 0, 1));
  const week = Math.ceil(((temp - yearStart) / 86400000 + 1) / 7);
  return `${temp.getUTCFullYear()}-W${pad2(week)}`;
};

const parseISOWeekStart = (label) => {
  const matched = /^(\d{4})-W(\d{2})$/.exec(String(label || '').trim());
  if (!matched) return null;
  const year = Number(matched[1]);
  const week = Number(matched[2]);
  if (!Number.isFinite(year) || !Number.isFinite(week) || week < 1) {
    return null;
  }
  const jan4 = new Date(year, 0, 4);
  const jan4Day = jan4.getDay() || 7;
  const weekOneStart = startOfDay(addDays(jan4, -(jan4Day - 1)));
  return addDays(weekOneStart, (week - 1) * 7);
};

const getCalendarBucketDateRange = (label, granularity) => {
  const normalizedLabel = String(label || '').trim();
  if (normalizedLabel === '') return null;
  let start = null;
  let end = null;
  switch (granularity) {
    case 'hour': {
      const parsed = new Date(`${normalizedLabel}:00:00`);
      if (Number.isNaN(parsed.getTime())) return null;
      start = parsed;
      end = new Date(parsed);
      end.setMinutes(59, 59, 999);
      break;
    }
    case 'week': {
      const weekStart = parseISOWeekStart(normalizedLabel);
      if (!weekStart) return null;
      start = weekStart;
      end = endOfDay(addDays(weekStart, 6));
      break;
    }
    case 'month': {
      const matched = /^(\d{4})-(\d{2})$/.exec(normalizedLabel);
      if (!matched) return null;
      start = new Date(Number(matched[1]), Number(matched[2]) - 1, 1);
      end = endOfDay(new Date(Number(matched[1]), Number(matched[2]), 0));
      break;
    }
    case 'year': {
      const year = Number(normalizedLabel);
      if (!Number.isFinite(year)) return null;
      start = new Date(year, 0, 1);
      end = endOfDay(new Date(year, 11, 31));
      break;
    }
    default: {
      const parsed = parseDateInput(normalizedLabel);
      if (!parsed) return null;
      start = parsed;
      end = endOfDay(parsed);
      break;
    }
  }
  return { start, end };
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

const normalizeDashboardRow = (item) => ({
  ...item,
  chargeAmount: Number(item?.charge_amount ?? item?.Quota ?? 0),
});

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

const SpendingCalendar = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [calendarView, setCalendarView] = useState('calendar');
  const [calendarGranularity, setCalendarGranularity] = useState('day');
  const [calendarUnit, setCalendarUnit] = useState('usd');
  const [calendarData, setCalendarData] = useState([]);
  const [displayCurrencyIndex, setDisplayCurrencyIndex] = useState(() =>
    buildPublicDisplayCurrencyIndex([]),
  );

  const loadDisplayCurrencies = useCallback(async () => {
    const { currencyIndex } = await loadPublicDisplayCurrencyCatalog();
    setDisplayCurrencyIndex(currencyIndex);
  }, []);

  const toUsd = useCallback(
    (chargeAmount) => {
      const amount = convertChargeAmountToDisplayAmount(
        chargeAmount,
        'USD',
        displayCurrencyIndex,
      );
      return Number.isFinite(amount) ? amount : 0;
    },
    [displayCurrencyIndex],
  );

  const formatUsdAmount = useCallback(
    (amount) =>
      formatCompactDisplayAmount(amount, {
        compactLabel: t('dashboard.spending.labels.ten_thousand'),
      }),
    [t],
  );

  const formatCountValue = (value) => {
    const num = Number(value);
    if (!Number.isFinite(num)) return '0';
    return Math.round(num).toLocaleString();
  };

  const fetchCalendarData = useCallback(async () => {
    try {
      const range = getCalendarRange(calendarGranularity);
      if (!range.start || !range.end) {
        setCalendarData([]);
        return;
      }
      const response = await API.get('/api/v1/public/user/dashboard', {
        params: {
          start_timestamp: range.start,
          end_timestamp: range.end,
          granularity: calendarGranularity,
        },
      });
      if (response.data.success) {
        setCalendarData(
          Array.isArray(response.data.data)
            ? response.data.data.map(normalizeDashboardRow)
            : [],
        );
        return;
      }
      setCalendarData([]);
    } catch (error) {
      console.error('Failed to fetch calendar data:', error);
      setCalendarData([]);
    }
  }, [calendarGranularity]);

  useEffect(() => {
    loadDisplayCurrencies().then();
  }, [loadDisplayCurrencies]);

  useEffect(() => {
    fetchCalendarData().then();
  }, [fetchCalendarData]);

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

  const formatCalendarValue = (value) => {
    if (calendarUnit === 'usd') return formatUsdAmount(value);
    return formatCountValue(value);
  };

  const handleCalendarBucketClick = useCallback(
    (bucket) => {
      const range = getCalendarBucketDateRange(bucket?.label, calendarGranularity);
      if (!range?.start || !range?.end) {
        return;
      }
      const searchParams = new URLSearchParams();
      searchParams.set('source', 'quota');
      searchParams.set('log_type', '2');
      searchParams.set('start_timestamp', formatDatetimeLocalInput(range.start));
      searchParams.set('end_timestamp', formatDatetimeLocalInput(range.end));
      navigate(`/workspace/log?${searchParams.toString()}`);
    },
    [calendarGranularity, navigate],
  );

  return (
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
              calendarBuckets.map((item) => (
                <button
                  type='button'
                  key={item.label}
                  className='dashboard-calendar-cell dashboard-calendar-cell-button'
                  onClick={() => handleCalendarBucketClick(item)}
                >
                  <div className='dashboard-calendar-label'>{item.label}</div>
                  <div className='dashboard-calendar-value'>
                    {formatCalendarValue(item.value)}
                  </div>
                </button>
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
                fill={calendarUnit === 'usd' ? chartColors.cost : chartColors.tokens}
                radius={[4, 4, 0, 0]}
              />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </AppSection>
  );
};

export default SpendingCalendar;
