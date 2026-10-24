import React, { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import {
  AppErrorState,
  AppSection,
  AppSegmented,
  AppSpin,
  AppTag,
  AppToolbar,
  chartAxisStyle,
  chartGridStyle,
  chartTooltipStyle,
  chartTooltipLabelStyle,
  chartTooltipItemStyle,
} from '../../../router-ui';
import {
  HEALTH_LEVEL_COLORS,
  MODEL_SORT_OPTIONS,
  PERIOD_OPTIONS,
  formatCount,
  formatPercent,
  formatUpdatedAt,
  useAdminDashboardData,
  useUsdFormatter,
} from '../dashboardShared';
import { DashboardSectionControls } from '../DashboardSectionControls';
import { buildLogDrilldownPath } from '../../../components/LogsTable.helpers';

// Model operations analytics, extracted from AdminDashboard's models section so
// the model shell (/workspace/service/models?tab=operations) can host it inline.
// Self-contained: owns period/sort state and fetches its own data. Only mounted
// for admins by the shell, so the admin dashboard API call is gated upstream.
const ModelOperationsSection = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { formatUsd } = useUsdFormatter();
  const [period, setPeriod] = useState('last_7_days');
  const [modelSort, setModelSort] = useState('spend');

  const { dashboard, loading, error, reload } = useAdminDashboardData(
    'models',
    {
      period,
    }
  );

  const periodOptions = useMemo(
    () =>
      PERIOD_OPTIONS.map((value) => ({
        key: value,
        value,
        text: t(`dashboard.spending.period.${value}`),
      })),
    [t]
  );

  const modelSortOptions = useMemo(
    () =>
      MODEL_SORT_OPTIONS.map((value) => ({
        value,
        label: t(`dashboard.admin.models.sort.${value}`),
      })),
    [t]
  );

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
    [t]
  );

  const sortedModels = useMemo(() => {
    const items = Array.isArray(dashboard.top_models)
      ? [...dashboard.top_models]
      : [];
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
      } else if (left.spend_amount !== right.spend_amount) {
        return right.spend_amount - left.spend_amount;
      }
      if (left.request_count !== right.request_count) {
        return right.request_count - left.request_count;
      }
      return String(left.model || '').localeCompare(String(right.model || ''));
    });
    return items;
  }, [dashboard.top_models, modelSort]);

  const modelLeaderboardData = useMemo(() => {
    return sortedModels.slice(0, 8).map((item) => {
      let value = Number(item.spend_amount || 0);
      let displayValue = formatUsd(item.spend_amount);
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
  }, [formatUsd, modelSort, sortedModels, t]);

  if (error && !loading) {
    return (
      <AppSection className='admin-dashboard-section'>
        <AppErrorState
          message={error?.message || t('common.load_failed')}
          onRetry={reload}
          retryText={t('common.retry')}
        />
      </AppSection>
    );
  }

  return (
    <AppSpin spinning={loading} className='admin-dashboard-content-spin'>
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
            className='admin-dashboard-section-toolbar'
            end={
              <DashboardSectionControls
                period={period}
                periodOptions={periodOptions}
                onPeriodChange={setPeriod}
                generatedAt={dashboard.generated_at}
                loading={loading}
                onRefresh={reload}
                extra={
                  <AppSegmented
                    className='admin-dashboard-segmented'
                    options={modelSortOptions}
                    value={modelSort}
                    onChange={(e, { value }) => setModelSort(value)}
                  />
                }
              />
            }
          />
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
                          {...chartGridStyle()}
                          horizontal={false}
                        />
                        <XAxis type='number' {...chartAxisStyle()} />
                        <YAxis
                          type='category'
                          dataKey='short_model'
                          width={128}
                          {...chartAxisStyle()}
                        />
                        <Tooltip
                          contentStyle={chartTooltipStyle()}
                          labelStyle={chartTooltipLabelStyle()}
                          itemStyle={chartTooltipItemStyle()}
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
              </div>
              <div className='admin-dashboard-model-grid'>
                {sortedModels.map((item) => (
                  <div
                    key={`${item.provider || 'unknown'}-${item.model}`}
                    className='admin-dashboard-model-card'
                  >
                    <div className='admin-dashboard-model-card-header'>
                      <div className='admin-dashboard-model-card-main'>
                        <button
                          type='button'
                          className='admin-dashboard-model-card-title admin-dashboard-model-card-title-button'
                          title={item.model || '-'}
                          onClick={() =>
                            navigate(
                              `/workspace/service/models?q=${encodeURIComponent(
                                item.model || ''
                              )}`
                            )
                          }
                        >
                          {item.model || '-'}
                        </button>
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
                          {formatUsd(item.spend_amount)}
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
                          {item.avg_latency_ms > 0
                            ? `${item.avg_latency_ms} ms`
                            : '-'}
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
                    <div className='admin-dashboard-model-card-actions'>
                      <button
                        type='button'
                        className='admin-dashboard-model-card-action'
                        onClick={() =>
                          navigate(
                            buildLogDrilldownPath('admin', {
                              model_name: item.model,
                            })
                          )
                        }
                      >
                        {t('log.drilldown.view')}
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </AppSection>
    </AppSpin>
  );
};

export default ModelOperationsSection;
