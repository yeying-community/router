import React, {
  Suspense,
  lazy,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { API } from '../../helpers/api';
import { copy, showError, showSuccess } from '../../helpers';
import { buildLogDrilldownPath } from '../../components/LogsTable.helpers';
import { useIsAdmin } from '../../hooks/useAuth';
import useUrlState from '../../hooks/useUrlState';
import {
  AppAlert,
  AppButton,
  AppFilterHeader,
  AppIcon,
  AppInput,
  AppPopover,
  AppSelect,
  AppSection,
  AppSkeleton,
  AppSpin,
  AppTag,
  AppTabs,
  AppTooltip,
  AppToolbar,
} from '../../router-ui';
import './WorkspaceModels.css';
import '../../components/SectionTabs.css';
import '../AdminDashboard/Dashboard.css';
import '../AdminDashboard/AdminDashboard.css';

const MODEL_HEALTH_HISTORY_SIZE = 60;
const ModelOperationsSection = lazy(() =>
  import('../AdminDashboard/sections/ModelOperationsSection'),
);

const POINT_COLORS = {
  success: '#22c55e',
  warning: '#f59e0b',
  failure: '#ef4444',
  unknown: '#cbd5e1',
};

const HEALTH_TAG_COLORS = {
  healthy: 'green',
  warning: 'orange',
  critical: 'red',
  unknown: 'grey',
};

const SORT_OPTIONS = ['health', 'name', 'latency', 'pass_rate'];
const FILTER_OPTIONS = ['all', 'healthy', 'warning', 'critical', 'unknown'];

const EMPTY_PAYLOAD = {
  group: '',
  generated_at: 0,
  summary: {
    model_count: 0,
    healthy_model_count: 0,
    warning_model_count: 0,
    critical_model_count: 0,
    unknown_model_count: 0,
    avg_pass_rate: 0,
    avg_latency_ms: 0,
  },
  models: [],
};

const toNumber = (value) => {
  const num = Number(value || 0);
  return Number.isFinite(num) ? num : 0;
};

const toPercentValue = (value) => {
  const num = toNumber(value);
  return num <= 1 ? num * 100 : num;
};

const formatPercent = (value) => `${toPercentValue(value).toFixed(1)}%`;

const formatCount = (value) => toNumber(value).toLocaleString('zh-CN');

const formatUpdatedAt = (value) => {
  const timestamp = toNumber(value);
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString('zh-CN', {
    hour12: false,
  });
};

const formatTimeRange = (start, end) => {
  const startTs = toNumber(start);
  const endTs = toNumber(end);
  if (!startTs || !endTs) return '-';
  return `${formatUpdatedAt(startTs)} - ${formatUpdatedAt(endTs)}`;
};

const normalizePointState = (point) => {
  const state = String(
    typeof point === 'string' ? point : point?.state || '',
  )
    .trim()
    .toLowerCase();
  if (state === 'success' || state === 'warning' || state === 'failure') {
    return state;
  }
  return 'unknown';
};

const normalizePayload = (payload) => {
  const data = payload && typeof payload === 'object' ? payload : {};
  const summary = data.summary || {};
  const models = Array.isArray(data.models) ? data.models : [];
  return {
    ...EMPTY_PAYLOAD,
    ...data,
    summary: {
      ...EMPTY_PAYLOAD.summary,
      ...summary,
      model_count: toNumber(summary.model_count),
      healthy_model_count: toNumber(summary.healthy_model_count),
      warning_model_count: toNumber(summary.warning_model_count),
      critical_model_count: toNumber(summary.critical_model_count),
      unknown_model_count: toNumber(summary.unknown_model_count),
      avg_pass_rate: toNumber(summary.avg_pass_rate),
      avg_latency_ms: toNumber(summary.avg_latency_ms),
    },
    models: models.map((item) => ({
      ...item,
      model: String(item?.model || '').trim(),
      provider: String(item?.provider || '').trim(),
      status: String(item?.status || 'unknown').trim().toLowerCase(),
      health_level: String(item?.health_level || 'unknown').trim().toLowerCase(),
      health_score: toNumber(item?.health_score),
      health_source: ['traffic', 'probe', 'none'].includes(item?.health_source)
        ? item.health_source
        : 'none',
      channel_count: toNumber(item?.channel_count),
      channel_ids: Array.isArray(item?.channel_ids)
        ? item.channel_ids
            .map((id) => String(id || '').trim())
            .filter(Boolean)
        : [],
      tested_channel_count: toNumber(item?.tested_channel_count),
      tested_endpoint_count: toNumber(item?.tested_endpoint_count),
      supported_count: toNumber(item?.supported_count),
      unsupported_count: toNumber(item?.unsupported_count),
      pass_rate: toNumber(item?.pass_rate),
      avg_latency_ms: toNumber(item?.avg_latency_ms),
      last_signal_at: toNumber(item?.last_signal_at),
      supported_endpoints: Array.isArray(item?.supported_endpoints)
        ? item.supported_endpoints
        : [],
      tags: Array.isArray(item?.tags) ? item.tags : [],
      health_points: Array.isArray(item?.health_points)
        ? item.health_points.map((point) => ({
            ...point,
            state: normalizePointState(point),
            bucket_start: toNumber(point?.bucket_start),
            bucket_end: toNumber(point?.bucket_end),
            success_count: toNumber(point?.success_count),
            failure_count: toNumber(point?.failure_count),
            total_count: toNumber(point?.total_count),
            avg_latency_ms: toNumber(point?.avg_latency_ms),
            last_observed_at: toNumber(point?.last_observed_at),
            pass_rate: toNumber(point?.pass_rate),
          }))
        : [],
    })),
  };
};

const buildHealthHistory = (points) => {
  const normalized = Array.isArray(points)
    ? points.slice(0, MODEL_HEALTH_HISTORY_SIZE)
    : [];
  const paddingCount = Math.max(
    0,
    MODEL_HEALTH_HISTORY_SIZE - normalized.length,
  );
  return [
    ...Array.from({ length: paddingCount }, () => ({
      state: 'unknown',
      observed: false,
    })),
    ...normalized
      .slice(0, MODEL_HEALTH_HISTORY_SIZE)
      .map((point) => ({
        ...point,
        state: normalizePointState(point),
        observed: toNumber(point?.total_count) > 0,
      })),
  ].map((point, index) => ({
    ...point,
    key: `${index}-${point.state}-${point.bucket_start || 0}`,
  }));
};

const renderHealthPointTooltip = (point, stateLabel, t) => {
  const latency = toNumber(point?.avg_latency_ms);
  const successCount = toNumber(point?.success_count);
  const failureCount = toNumber(point?.failure_count);
  const totalCount = toNumber(point?.total_count);
  return (
    <div className='workspace-model-health-tooltip'>
      <div className='workspace-model-health-tooltip-header'>
        <span className={`workspace-model-health-tooltip-status workspace-model-health-tooltip-status-${point?.state || 'unknown'}`}>
          {stateLabel}
        </span>
        <span className='workspace-model-health-tooltip-time'>
          {formatTimeRange(point?.bucket_start, point?.bucket_end)}
        </span>
      </div>
      <div className='workspace-model-health-tooltip-divider' />
      <div className='workspace-model-health-tooltip-row'>
        <span>{t('workspace_models.tooltip.success_count')}</span>
        <strong>{formatCount(successCount)}</strong>
      </div>
      <div className='workspace-model-health-tooltip-row'>
        <span>{t('workspace_models.tooltip.failure_count')}</span>
        <strong>{formatCount(failureCount)}</strong>
      </div>
      <div className='workspace-model-health-tooltip-row'>
        <span>{t('workspace_models.tooltip.total_count')}</span>
        <strong>{formatCount(totalCount)}</strong>
      </div>
      <div className='workspace-model-health-tooltip-row'>
        <span>{t('workspace_models.tooltip.pass_rate')}</span>
        <strong>{totalCount > 0 ? formatPercent(point?.pass_rate) : '-'}</strong>
      </div>
      <div className='workspace-model-health-tooltip-row'>
        <span>{t('workspace_models.tooltip.avg_latency')}</span>
        <strong>{latency > 0 ? `${latency} ms` : '-'}</strong>
      </div>
      <div className='workspace-model-health-tooltip-summary'>
        {t('workspace_models.tooltip.summary.bucket')}
      </div>
    </div>
  );
};

const WorkspaceModels = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const hasAdminAccess = useIsAdmin();
  const [payload, setPayload] = useState(EMPTY_PAYLOAD);
  const [loading, setLoading] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);
  // Keyword / health / sort all persist to the URL so a refresh or back-nav
  // restores the operator's view, and cross-page CTAs (e.g. the admin dashboard
  // "at-risk models" headline) can deep-link straight to a filtered view.
  const [{ keyword, healthFilter, sortBy, tab }, patchFilters] = useUrlState({
    tab: {
      param: 'tab',
      default: 'health',
      parse: (raw) => (raw === 'operations' ? 'operations' : 'health'),
    },
    keyword: { param: 'q', default: '' },
    healthFilter: {
      param: 'health',
      default: 'all',
      parse: (raw) => {
        const normalized = String(raw || '').trim().toLowerCase();
        return FILTER_OPTIONS.includes(normalized) ? normalized : 'all';
      },
    },
    sortBy: {
      param: 'sort',
      default: 'health',
      parse: (raw) => {
        const normalized = String(raw || '').trim().toLowerCase();
        return SORT_OPTIONS.includes(normalized) ? normalized : 'health';
      },
    },
  });
  const showOperations = hasAdminAccess && tab === 'operations';
  // Admins can jump straight from a model to the publish tab of a channel that
  // serves it (channel_ids comes from /api/v1/public/model/status). Single
  // channel → direct deep link; multiple → a popover of per-channel links.
  const goPublishChannel = useCallback(
    (channelId) => {
      const id = String(channelId || '').trim();
      if (!id) return;
      navigate(`/admin/channel/detail/${id}?tab=publish`);
    },
    [navigate],
  );

  const handleCopyModel = useCallback(
    async (model) => {
      const value = String(model || '').trim();
      if (!value) return;
      const ok = await copy(value);
      if (ok) {
        showSuccess(t('workspace_models.card.copied', { model: value }));
      } else {
        showError(t('workspace_models.card.copy_failed'));
      }
    },
    [t],
  );

  const renderChannelCount = useCallback(
    (item) => {
      const value = `${formatCount(item.tested_channel_count)} / ${formatCount(item.channel_count)}`;
      const ids = Array.isArray(item.channel_ids) ? item.channel_ids : [];
      if (!hasAdminAccess || ids.length === 0) {
        return <strong>{value}</strong>;
      }
      if (ids.length === 1) {
        return (
          <strong>
            <AppButton
              type='link'
              className='workspace-model-channel-link'
              title={t('channel.edit.detail_tabs.publish')}
              onClick={() => goPublishChannel(ids[0])}
            >
              {value}
            </AppButton>
          </strong>
        );
      }
      return (
        <strong>
          <AppPopover
            trigger='click'
            content={
              <div className='workspace-model-channel-popover'>
                {ids.map((id, index) => (
                  <AppButton
                    key={id}
                    type='link'
                    onClick={() => goPublishChannel(id)}
                  >
                    {`${t('header.channel')} ${index + 1}`}
                  </AppButton>
                ))}
              </div>
            }
          >
            <AppButton type='link' className='workspace-model-channel-link'>
              {value}
            </AppButton>
          </AppPopover>
        </strong>
      );
    },
    [goPublishChannel, hasAdminAccess, t],
  );

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/v1/public/model/status');
      if (res.data?.success) {
        setPayload(normalizePayload(res.data.data || {}));
        setLoadFailed(false);
      } else {
        setLoadFailed(true);
      }
    } catch (error) {
      console.error('Failed to load workspace model status:', error);
      setLoadFailed(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!showOperations) loadData();
  }, [loadData, showOperations]);

  const healthFilterOptions = useMemo(
    () =>
      FILTER_OPTIONS.map((value) => ({
        value,
        label: t(`workspace_models.filters.health.${value}`),
      })),
    [t],
  );

  const sortOptions = useMemo(
    () =>
      SORT_OPTIONS.map((value) => ({
        value,
        label: t(`workspace_models.sort.${value}`),
      })),
    [t],
  );

  const filteredModels = useMemo(() => {
    const query = keyword.trim().toLowerCase();
    const rows = payload.models.filter((item) => {
      if (healthFilter !== 'all' && item.health_level !== healthFilter) {
        return false;
      }
      if (!query) {
        return true;
      }
      return [
        item.model,
        item.provider,
        ...(item.tags || []),
        ...(item.supported_endpoints || []),
      ]
        .join(' ')
        .toLowerCase()
        .includes(query);
    });
    rows.sort((left, right) => {
      if (sortBy === 'name') {
        return left.model.localeCompare(right.model);
      }
      if (sortBy === 'latency') {
        const leftLatency = left.avg_latency_ms || Number.MAX_SAFE_INTEGER;
        const rightLatency = right.avg_latency_ms || Number.MAX_SAFE_INTEGER;
        if (leftLatency !== rightLatency) {
          return leftLatency - rightLatency;
        }
      } else if (sortBy === 'pass_rate') {
        if (left.pass_rate !== right.pass_rate) {
          return right.pass_rate - left.pass_rate;
        }
      } else if (left.health_score !== right.health_score) {
        return right.health_score - left.health_score;
      }
      return left.model.localeCompare(right.model);
    });
    return rows;
  }, [healthFilter, keyword, payload.models, sortBy]);

  const isFiltered = healthFilter !== 'all' || keyword.trim().length > 0;

  const renderHealthTag = (level) => {
    const normalized = String(level || 'unknown').trim().toLowerCase();
    return (
      <AppTag
        color={HEALTH_TAG_COLORS[normalized] || HEALTH_TAG_COLORS.unknown}
        className='router-tag'
      >
        {t(`workspace_models.health.${normalized}`, {
          defaultValue: t('workspace_models.health.unknown'),
        })}
      </AppTag>
    );
  };

  const renderHealthSource = (source) => (
    <AppTag className='router-tag workspace-model-source-tag'>
      {t(`workspace_models.source.${source || 'none'}`)}
    </AppTag>
  );

  const summary = payload.summary || EMPTY_PAYLOAD.summary;

  return (
    <div className='dashboard-container workspace-models-page'>
      <AppFilterHeader
        breadcrumbs={[
          {
            key: 'service',
            label: t(hasAdminAccess ? 'header.supply' : 'header.service'),
          },
          {
            key: 'models',
            label: t('header.model'),
            active: true,
          },
        ]}
        title={t('header.model')}
      />
      {hasAdminAccess && (
        <AppTabs
          className='section-tabs'
          activeKey={tab}
          items={['health', 'operations'].map((key) => ({
            key,
            label: t(`workspace_models.tabs.${key}`),
          }))}
          onChange={(value) => patchFilters({ tab: value })}
        />
      )}
      {showOperations ? (
        <Suspense fallback={<AppSkeleton variant='list' count={3} />}>
          <ModelOperationsSection />
        </Suspense>
      ) : (
        <AppSpin spinning={loading}>
          <AppSection className='workspace-models-section'>
            {loadFailed && (
              <AppAlert
                type='error'
                showIcon
                title={t('workspace_models.load_failed')}
                action={
                  <AppButton size='small' loading={loading} onClick={loadData}>
                    {t('workspace_models.retry')}
                  </AppButton>
                }
              />
            )}
            <div className='workspace-models-toolbar'>
              <AppToolbar
                className='workspace-models-controls'
                start={
                  <div className='workspace-models-search-row'>
                    <AppInput
                      className='workspace-models-search'
                      value={keyword}
                      aria-label={t('workspace_models.search_placeholder')}
                      placeholder={t('workspace_models.search_placeholder')}
                      onChange={(e, { value }) =>
                        patchFilters({ keyword: value })
                      }
                    />
                    <AppTooltip title={t('workspace_models.refresh')}>
                      <AppButton
                        type='button'
                        className='router-inline-button workspace-models-refresh'
                        aria-label={t('workspace_models.refresh')}
                        loading={loading}
                        onClick={loadData}
                        icon={<AppIcon name='exchange' />}
                      />
                    </AppTooltip>
                    <span className='workspace-models-count'>
                      {isFiltered
                        ? t('workspace_models.summary.count_filtered', {
                            shown: formatCount(filteredModels.length),
                            total: formatCount(summary.model_count),
                          })
                        : t('workspace_models.summary.count', {
                            total: formatCount(summary.model_count),
                          })}
                    </span>
                  </div>
                }
                end={
                  <div className='workspace-models-filter-row'>
                    <AppSelect
                      className='workspace-models-select'
                      aria-label={t('workspace_models.health_filter')}
                      options={healthFilterOptions}
                      value={healthFilter}
                      onChange={(e, { value }) =>
                        patchFilters({ healthFilter: value })
                      }
                    />
                    <AppSelect
                      className='workspace-models-select'
                      aria-label={t('workspace_models.sort_label')}
                      options={sortOptions}
                      value={sortBy}
                      onChange={(e, { value }) =>
                        patchFilters({ sortBy: value })
                      }
                    />
                  </div>
                }
              />
              <div className='workspace-models-meta'>
                {t('workspace_models.generated_at', {
                  time: formatUpdatedAt(payload.generated_at),
                })}
              </div>
            </div>
            <div className='workspace-models-legend'>
              {['success', 'warning', 'failure', 'unknown'].map((state) => (
                <span key={state} className='workspace-models-legend-item'>
                  <span
                    className='workspace-models-legend-dot'
                    style={{ background: POINT_COLORS[state] }}
                  />
                  {t(`workspace_models.history.${state}`)}
                </span>
              ))}
            </div>
            {filteredModels.length === 0 ? (
              loading ? (
                <AppSkeleton variant='list' count={6} />
              ) : loadFailed ? null : (
                <div className='workspace-models-empty'>
                  {t(
                    isFiltered
                      ? 'workspace_models.no_matches'
                      : 'workspace_models.empty',
                  )}
                  {isFiltered && (
                    <AppButton
                      type='link'
                      onClick={() =>
                        patchFilters({ keyword: '', healthFilter: 'all' })
                      }
                    >
                      {t('workspace_models.clear_filters')}
                    </AppButton>
                  )}
                </div>
              )
            ) : (
              <div className='workspace-models-list'>
                {filteredModels.map((item) => {
                  const history = buildHealthHistory(item.health_points);
                  return (
                    <div
                      key={`${item.provider || 'unknown'}-${item.model}`}
                      className='workspace-model-card'
                    >
                      <div className='workspace-model-card-header'>
                        <div className='workspace-model-card-title-block'>
                          <div className='workspace-model-card-title-row'>
                            <div
                              className='workspace-model-card-title'
                              title={item.model}
                            >
                              {item.model || '-'}
                            </div>
                            <AppTooltip
                              title={t('workspace_models.card.copy_model')}
                            >
                              <AppButton
                                type='button'
                                className='workspace-model-card-icon-button'
                                aria-label={t(
                                  'workspace_models.card.copy_model',
                                )}
                                icon={<AppIcon name='copy outline' />}
                                onClick={() => handleCopyModel(item.model)}
                                disabled={!item.model}
                              />
                            </AppTooltip>
                          </div>
                          <div className='workspace-model-card-subtitle'>
                            <span>{item.provider || '-'}</span>
                            {item.tags.map((tag) => (
                              <AppTag
                                key={`${item.model}-${tag}`}
                                className='router-tag workspace-model-tag'
                              >
                                {tag}
                              </AppTag>
                            ))}
                          </div>
                        </div>
                        <div className='workspace-model-card-status'>
                          {renderHealthTag(item.health_level)}
                          {renderHealthSource(item.health_source)}
                          <AppTooltip title={t('log.drilldown.view')}>
                            <AppButton
                              type='button'
                              className='workspace-model-card-icon-button workspace-model-card-log-button'
                              aria-label={t('log.drilldown.view')}
                              icon={<AppIcon name='book' />}
                              onClick={() =>
                                navigate(
                                  buildLogDrilldownPath(
                                    hasAdminAccess ? 'admin' : 'workspace',
                                    { model_name: item.model },
                                  ),
                                )
                              }
                              disabled={!item.model}
                            />
                          </AppTooltip>
                        </div>
                      </div>
                      <div className='workspace-model-health-row'>
                        <div className='workspace-model-health-strip-wrap'>
                          <div
                            className='workspace-model-health-strip'
                            aria-label={t('workspace_models.history_label', {
                              model: item.model,
                            })}
                          >
                            {history.map((point) => {
                              const stateLabel = point.observed
                                ? t(`workspace_models.history.${point.state}`)
                                : t('workspace_models.history.no_data');
                              const title = point.observed
                                ? renderHealthPointTooltip(point, stateLabel, t)
                                : t('workspace_models.tooltip.no_sample');
                              return (
                                <AppTooltip
                                  key={point.key}
                                  title={title}
                                  placement='top'
                                  color='#ffffff'
                                  classNames={{
                                    root: 'workspace-model-health-tooltip-popup',
                                  }}
                                >
                                  <span
                                    className={`workspace-model-health-cell workspace-model-health-cell-${point.state}`}
                                    style={{
                                      background:
                                        POINT_COLORS[point.state] ||
                                        POINT_COLORS.unknown,
                                    }}
                                  />
                                </AppTooltip>
                              );
                            })}
                          </div>
                          <div className='workspace-model-health-axis'>
                            <span>{t('workspace_models.history.past')}</span>
                            <span>{t('workspace_models.history.now')}</span>
                          </div>
                        </div>
                        <div className='workspace-model-metrics'>
                          <div className='workspace-model-metric'>
                            <span>
                              {t('workspace_models.card.health_score')}
                            </span>
                            <strong>{formatCount(item.health_score)}</strong>
                          </div>
                          <div className='workspace-model-metric'>
                            <span>{t('workspace_models.card.pass_rate')}</span>
                            <strong>{formatPercent(item.pass_rate)}</strong>
                          </div>
                          <div className='workspace-model-metric'>
                            <span>{t('workspace_models.card.latency')}</span>
                            <strong>
                              {item.avg_latency_ms > 0
                                ? `${formatCount(item.avg_latency_ms)} ms`
                                : '-'}
                            </strong>
                          </div>
                          <div className='workspace-model-metric'>
                            <span>{t('workspace_models.card.channels')}</span>
                            {renderChannelCount(item)}
                          </div>
                        </div>
                      </div>
                      <div className='workspace-model-footer'>
                        <div className='workspace-model-endpoints'>
                          {item.supported_endpoints.length > 0
                            ? item.supported_endpoints.map((endpoint) => (
                                <AppTag
                                  key={`${item.model}-${endpoint}`}
                                  className='router-tag workspace-model-endpoint-tag'
                                >
                                  {endpoint}
                                </AppTag>
                              ))
                            : t('workspace_models.card.no_endpoints')}
                        </div>
                        <div className='workspace-model-last-tested'>
                          {t(
                            `workspace_models.card.last_signal.${item.health_source}`,
                            {
                              time: formatUpdatedAt(item.last_signal_at),
                            },
                          )}
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </AppSection>
        </AppSpin>
      )}
    </div>
  );
};

export default WorkspaceModels;
