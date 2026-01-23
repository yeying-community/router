import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, Checkbox, Dropdown, Form, Grid, Input } from 'semantic-ui-react';
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
import axios from 'axios';
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
    quota: '#00B5D8',
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
  day: 365,
  week: 52,
  month: 36,
  year: 10,
};

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
  const [granularity, setGranularity] = useState('week');
  const [span, setSpan] = useState(1);
  const [spanAuto, setSpanAuto] = useState(true);
  const [endAuto, setEndAuto] = useState(true);
  const initialRange = useMemo(() => getDefaultRange('week', 1), []);
  const [startDate, setStartDate] = useState(initialRange.start);
  const [endDate, setEndDate] = useState(initialRange.end);

  const selectedModelsKey = useMemo(
    () => selectedModels.slice().sort().join(','),
    [selectedModels]
  );

  useEffect(() => {
    if (!startDate || !endDate) return;
    if (selectionReady && selectedModels.length === 0) {
      setData([]);
      return;
    }
    fetchDashboardData();
  }, [granularity, startDate, endDate, selectedModelsKey, selectionReady]);

  useEffect(() => {
    const allModels = Object.values(providers).flat();
    if (allModels.length === 0) return;
    if (!selectionReady) {
      setSelectedModels(allModels);
      setSelectionReady(true);
      return;
    }
    setSelectedModels((prev) => prev.filter((model) => allModels.includes(model)));
  }, [providers, selectionReady]);

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
      const allModels = Object.values(providers).flat();
      const shouldFilter =
        selectedModels.length > 0 &&
        (allModels.length === 0 || selectedModels.length < allModels.length);
      if (shouldFilter) {
        params.models = selectedModels.join(',');
      }
      const response = await axios.get('/api/user/dashboard', { params });
      if (response.data.success) {
        const dashboardData = response.data.data || [];
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

  // 处理数据以供折线图使用，补充缺失的日期
  const processTimeSeriesData = () => {
    if (granularity !== 'day') {
      const grouped = {};
      data.forEach((item) => {
        const bucket = item.Day;
        if (!grouped[bucket]) {
          grouped[bucket] = {
            date: bucket,
            requests: 0,
            quota: 0,
            tokens: 0,
          };
        }
        grouped[bucket].requests += item.RequestCount;
        grouped[bucket].quota += item.Quota / 1000000;
        grouped[bucket].tokens += item.PromptTokens + item.CompletionTokens;
      });
      return Object.values(grouped).sort((a, b) => a.date.localeCompare(b.date));
    }

    const dailyData = {};
    const start = parseDateInput(startDate);
    const end = parseDateInput(endDate);
    if (!start || !end) return [];

    for (let d = new Date(start); d <= end; d.setDate(d.getDate() + 1)) {
      const dateStr = formatDateInput(d);
      dailyData[dateStr] = {
        date: dateStr,
        requests: 0,
        quota: 0,
        tokens: 0,
      };
    }

    data.forEach((item) => {
      if (!dailyData[item.Day]) {
        dailyData[item.Day] = {
          date: item.Day,
          requests: 0,
          quota: 0,
          tokens: 0,
        };
      }
      dailyData[item.Day].requests += item.RequestCount;
      dailyData[item.Day].quota += item.Quota / 1000000;
      dailyData[item.Day].tokens += item.PromptTokens + item.CompletionTokens;
    });

    return Object.values(dailyData).sort((a, b) => a.date.localeCompare(b.date));
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
    { key: 'day', text: t('dashboard.filters.granularity_options.day'), value: 'day' },
    { key: 'week', text: t('dashboard.filters.granularity_options.week'), value: 'week' },
    { key: 'month', text: t('dashboard.filters.granularity_options.month'), value: 'month' },
    { key: 'year', text: t('dashboard.filters.granularity_options.year'), value: 'year' },
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

  const providerEntries = useMemo(() => {
    return Object.entries(providers).sort(([a], [b]) => a.localeCompare(b));
  }, [providers]);

  const selectedSet = useMemo(() => new Set(selectedModels), [selectedModels]);

  const toggleProvider = (provider) => {
    const models = providers[provider] || [];
    const next = new Set(selectedModels);
    const allSelected = models.every((model) => next.has(model));
    if (allSelected) {
      models.forEach((model) => next.delete(model));
    } else {
      models.forEach((model) => next.add(model));
    }
    setSelectedModels(Array.from(next));
  };

  const toggleModel = (model) => {
    const next = new Set(selectedModels);
    if (next.has(model)) {
      next.delete(model);
    } else {
      next.add(model);
    }
    setSelectedModels(Array.from(next));
  };

  const timeSeriesData = processTimeSeriesData();
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

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card dashboard-filter-card'>
        <Card.Content>
          <Card.Header>{t('dashboard.filters.title')}</Card.Header>
          <Form>
            <Form.Group widths='equal' className='dashboard-filter-row'>
              <Form.Field>
                <label>{t('dashboard.filters.granularity')}</label>
                <Dropdown
                  selection
                  options={granularityOptions}
                  value={granularity}
                  onChange={handleGranularityChange}
                />
              </Form.Field>
              <Form.Field>
                <label>{t('dashboard.filters.span')}</label>
                <Input
                  type='number'
                  min={1}
                  max={spanLimits[granularity]}
                  value={span}
                  onChange={handleSpanChange}
                  className={spanAuto ? 'dashboard-muted' : ''}
                />
              </Form.Field>
              <Form.Field>
                <label>{t('dashboard.filters.start')}</label>
                <Input
                  type='date'
                  value={startDate}
                  onChange={handleStartChange}
                />
              </Form.Field>
              <Form.Field>
                <label>{t('dashboard.filters.end')}</label>
                <Input
                  type='date'
                  value={endDate}
                  onChange={handleEndChange}
                  className={endAuto ? 'dashboard-muted' : ''}
                />
              </Form.Field>
            </Form.Group>
          </Form>
          <div className='dashboard-provider-section'>
            <div className='dashboard-provider-title'>
              {t('dashboard.filters.providers')}
            </div>
            <div className='dashboard-provider-tree'>
              {providerEntries.length === 0 ? (
                <div className='dashboard-provider-empty'>
                  {t('dashboard.filters.no_providers')}
                </div>
              ) : (
                providerEntries.map(([provider, providerModels]) => {
                  const allSelected = providerModels.every((model) =>
                    selectedSet.has(model)
                  );
                  const someSelected =
                    !allSelected &&
                    providerModels.some((model) => selectedSet.has(model));
                  return (
                    <div key={provider} className='dashboard-provider-node'>
                      <Checkbox
                        label={provider}
                        checked={allSelected}
                        indeterminate={someSelected}
                        onChange={() => toggleProvider(provider)}
                      />
                      <div className='dashboard-provider-models'>
                        {providerModels.map((model) => (
                          <Checkbox
                            key={model}
                            label={model}
                            checked={selectedSet.has(model)}
                            onChange={() => toggleModel(model)}
                          />
                        ))}
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          </div>
        </Card.Content>
      </Card>
      {/* 三个并排的折线图 */}
      <Grid columns={3} stackable className='charts-grid'>
        <Grid.Column>
          <Card fluid className='chart-card'>
            <Card.Content>
              <Card.Header>
                {t('dashboard.charts.requests.title')}
                {/* <span className='stat-value'>{summaryData.todayRequests}</span> */}
              </Card.Header>
              <div className='chart-container'>
                <ResponsiveContainer
                  width='100%'
                  height={120}
                  margin={{ left: 10, right: 10 }} // 调整容器边距
                >
                  <LineChart data={timeSeriesData}>
                    <CartesianGrid
                      strokeDasharray='3 3'
                      vertical={chartConfig.lineChart.grid.vertical}
                      horizontal={chartConfig.lineChart.grid.horizontal}
                      opacity={chartConfig.lineChart.grid.opacity}
                    />
                    <XAxis {...xAxisConfig} />
                    <YAxis hide={true} />
                    <Tooltip
                      contentStyle={{
                        background: '#fff',
                        border: 'none',
                        borderRadius: '4px',
                        boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
                      }}
                      formatter={(value) => [
                        value,
                        t('dashboard.charts.requests.tooltip'),
                      ]}
                      labelFormatter={(label) =>
                        `${t('dashboard.statistics.tooltip.date')}: ${label}`
                      }
                    />
                    <Line
                      type='monotone'
                      dataKey='requests'
                      stroke={chartConfig.colors.requests}
                      strokeWidth={chartConfig.lineChart.line.strokeWidth}
                      dot={chartConfig.lineChart.line.dot}
                      activeDot={chartConfig.lineChart.line.activeDot}
                    />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </Card.Content>
          </Card>
        </Grid.Column>

        <Grid.Column>
          <Card fluid className='chart-card'>
            <Card.Content>
              <Card.Header>
                {t('dashboard.charts.quota.title')}
                {/* <span className='stat-value'>
                  ${summaryData.todayQuota.toFixed(3)}
                </span> */}
              </Card.Header>
              <div className='chart-container'>
                <ResponsiveContainer
                  width='100%'
                  height={120}
                  margin={{ left: 10, right: 10 }} // 调整容器边距
                >
                  <LineChart data={timeSeriesData}>
                    <CartesianGrid
                      strokeDasharray='3 3'
                      vertical={chartConfig.lineChart.grid.vertical}
                      horizontal={chartConfig.lineChart.grid.horizontal}
                      opacity={chartConfig.lineChart.grid.opacity}
                    />
                    <XAxis {...xAxisConfig} />
                    <YAxis hide={true} />
                    <Tooltip
                      contentStyle={{
                        background: '#fff',
                        border: 'none',
                        borderRadius: '4px',
                        boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
                      }}
                      formatter={(value) => [
                        value.toFixed(6),
                        t('dashboard.charts.quota.tooltip'),
                      ]}
                      labelFormatter={(label) =>
                        `${t('dashboard.statistics.tooltip.date')}: ${label}`
                      }
                    />
                    <Line
                      type='monotone'
                      dataKey='quota'
                      stroke={chartConfig.colors.quota}
                      strokeWidth={chartConfig.lineChart.line.strokeWidth}
                      dot={chartConfig.lineChart.line.dot}
                      activeDot={chartConfig.lineChart.line.activeDot}
                    />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </Card.Content>
          </Card>
        </Grid.Column>

        <Grid.Column>
          <Card fluid className='chart-card'>
            <Card.Content>
              <Card.Header>
                {t('dashboard.charts.tokens.title')}
                {/* <span className='stat-value'>{summaryData.todayTokens}</span> */}
              </Card.Header>
              <div className='chart-container'>
                <ResponsiveContainer
                  width='100%'
                  height={120}
                  margin={{ left: 10, right: 10 }} // 调整容器边距
                >
                  <LineChart data={timeSeriesData}>
                    <CartesianGrid
                      strokeDasharray='3 3'
                      vertical={chartConfig.lineChart.grid.vertical}
                      horizontal={chartConfig.lineChart.grid.horizontal}
                      opacity={chartConfig.lineChart.grid.opacity}
                    />
                    <XAxis {...xAxisConfig} />
                    <YAxis hide={true} />
                    <Tooltip
                      contentStyle={{
                        background: '#fff',
                        border: 'none',
                        borderRadius: '4px',
                        boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
                      }}
                      formatter={(value) => [
                        value,
                        t('dashboard.charts.tokens.tooltip'),
                      ]}
                      labelFormatter={(label) =>
                        `${t('dashboard.statistics.tooltip.date')}: ${label}`
                      }
                    />
                    <Line
                      type='monotone'
                      dataKey='tokens'
                      stroke={chartConfig.colors.tokens}
                      strokeWidth={chartConfig.lineChart.line.strokeWidth}
                      dot={chartConfig.lineChart.line.dot}
                      activeDot={chartConfig.lineChart.line.activeDot}
                    />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </Card.Content>
          </Card>
        </Grid.Column>
      </Grid>

      {/* 模型使用统计 */}
      <Card fluid className='chart-card'>
        <Card.Content>
          <Card.Header>{t('dashboard.statistics.title')}</Card.Header>
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
        </Card.Content>
      </Card>
    </div>
  );
};

export default Dashboard;
