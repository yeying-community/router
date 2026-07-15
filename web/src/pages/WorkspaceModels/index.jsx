import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API } from '../../helpers/api';
import {
  AppButton,
  AppFilterHeader,
  AppIcon,
  AppInput,
  AppSegmented,
  AppSection,
  AppTag,
  AppTooltip,
  AppToolbar,
} from '../../router-ui';
import './WorkspaceModels.css';

const MODEL_HEALTH_HISTORY_SIZE = 60;

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
      channel_count: toNumber(item?.channel_count),
      tested_channel_count: toNumber(item?.tested_channel_count),
      tested_endpoint_count: toNumber(item?.tested_endpoint_count),
      supported_count: toNumber(item?.supported_count),
      unsupported_count: toNumber(item?.unsupported_count),
      pass_rate: toNumber(item?.pass_rate),
      avg_latency_ms: toNumber(item?.avg_latency_ms),
      last_tested_at: toNumber(item?.last_tested_at),
      supported_endpoints: Array.isArray(item?.supported_endpoints)
        ? item.supported_endpoints
        : [],
      tags: Array.isArray(item?.tags) ? item.tags : [],
      health_points: Array.isArray(item?.health_points)
        ? item.health_points.map((point) => ({
            ...point,
            state: normalizePointState(point),
            tested_at: toNumber(point?.tested_at),
            latency_ms: toNumber(point?.latency_ms),
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
      .reverse()
      .map((point) => ({
        ...point,
        state: normalizePointState(point),
        observed: true,
      })),
  ].map((point, index) => ({
    ...point,
    key: `${index}-${point.state}-${point.tested_at || 0}`,
  }));
};

const renderHealthPointTooltip = (point, stateLabel, t, formatUpdatedAt) => {
  const endpoint = String(point?.endpoint || '').trim();
  const latency = toNumber(point?.latency_ms);
  const source = String(point?.source || 'manual_test').trim();
  const summaryKey = point?.state === 'success'
    ? 'success'
    : point?.state === 'warning'
      ? 'warning'
      : 'failure';
  return (
    <div className='workspace-model-health-tooltip'>
      <div className='workspace-model-health-tooltip-header'>
        <span className={`workspace-model-health-tooltip-status workspace-model-health-tooltip-status-${point?.state || 'unknown'}`}>
          {stateLabel}
        </span>
        <span className='workspace-model-health-tooltip-time'>
          {formatUpdatedAt(point?.tested_at)}
        </span>
      </div>
      <div className='workspace-model-health-tooltip-divider' />
      <div className='workspace-model-health-tooltip-row'>
        <span>{t('workspace_models.tooltip.response_time')}</span>
        <strong>{latency > 0 ? `${latency} ms` : '-'}</strong>
      </div>
      <div className='workspace-model-health-tooltip-row'>
        <span>{t('workspace_models.tooltip.endpoint')}</span>
        <strong>{endpoint || '-'}</strong>
      </div>
      <div className='workspace-model-health-tooltip-row'>
        <span>{t('workspace_models.tooltip.source')}</span>
        <strong>{t(`workspace_models.tooltip.sources.${source}`, {
          defaultValue: source,
        })}</strong>
      </div>
      <div className='workspace-model-health-tooltip-summary'>
        {t(`workspace_models.tooltip.summary.${summaryKey}`, {
          latency: latency > 0 ? latency : '-',
        })}
      </div>
    </div>
  );
};

const WorkspaceModels = () => {
  const { t } = useTranslation();
  const [payload, setPayload] = useState(EMPTY_PAYLOAD);
  const [loading, setLoading] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [healthFilter, setHealthFilter] = useState('all');
  const [sortBy, setSortBy] = useState('health');

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/v1/public/user/model_status');
      if (res.data?.success) {
        setPayload(normalizePayload(res.data.data || {}));
      } else {
        setPayload(EMPTY_PAYLOAD);
      }
    } catch (error) {
      console.error('Failed to load workspace model status:', error);
      setPayload(EMPTY_PAYLOAD);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

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

  const summary = payload.summary || EMPTY_PAYLOAD.summary;

  return (
    <div className='dashboard-container workspace-models-page'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'workspace', label: t('header.user_workspace') },
          { key: 'service', label: t('header.service') },
          { key: 'models', label: t('workspace_models.title'), active: true },
        ]}
        title={t('workspace_models.title')}
      />
      <AppSection className='workspace-models-section'>
        <div className='workspace-models-toolbar'>
          <div className='workspace-models-summary-grid'>
            <div className='workspace-models-summary-item'>
              <span>{t('workspace_models.summary.total')}</span>
              <strong>{formatCount(summary.model_count)}</strong>
            </div>
            <div className='workspace-models-summary-item workspace-models-summary-item-green'>
              <span>{t('workspace_models.summary.healthy')}</span>
              <strong>{formatCount(summary.healthy_model_count)}</strong>
            </div>
            <div className='workspace-models-summary-item workspace-models-summary-item-orange'>
              <span>{t('workspace_models.summary.warning')}</span>
              <strong>{formatCount(summary.warning_model_count)}</strong>
            </div>
            <div className='workspace-models-summary-item workspace-models-summary-item-red'>
              <span>{t('workspace_models.summary.critical')}</span>
              <strong>{formatCount(summary.critical_model_count)}</strong>
            </div>
            <div className='workspace-models-summary-item'>
              <span>{t('workspace_models.summary.pass_rate')}</span>
              <strong>{formatPercent(summary.avg_pass_rate)}</strong>
            </div>
            <div className='workspace-models-summary-item'>
              <span>{t('workspace_models.summary.latency')}</span>
              <strong>
                {summary.avg_latency_ms > 0
                  ? `${formatCount(summary.avg_latency_ms)} ms`
                  : '-'}
              </strong>
            </div>
          </div>
          <AppToolbar
            className='workspace-models-controls'
            start={
              <div className='workspace-models-search-row'>
                <AppInput
                  className='workspace-models-search'
                  value={keyword}
                  placeholder={t('workspace_models.search_placeholder')}
                  onChange={(e, { value }) => setKeyword(value)}
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
              </div>
            }
            end={
              <div className='workspace-models-filter-row'>
                <AppSegmented
                  className='workspace-models-segmented'
                  options={healthFilterOptions}
                  value={healthFilter}
                  onChange={(e, { value }) => setHealthFilter(value)}
                />
                <AppSegmented
                  className='workspace-models-segmented'
                  options={sortOptions}
                  value={sortBy}
                  onChange={(e, { value }) => setSortBy(value)}
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
          <div className='workspace-models-empty'>
            {loading ? t('common.loading') : t('workspace_models.empty')}
          </div>
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
                      <div
                        className='workspace-model-card-title'
                        title={item.model}
                      >
                        {item.model || '-'}
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
                        {history.map((point, index) => {
                          const stateLabel = point.observed
                            ? t(`workspace_models.history.${point.state}`)
                            : t('workspace_models.history.no_data');
                          const title = point.observed
                            ? renderHealthPointTooltip(
                                point,
                                stateLabel,
                                t,
                                formatUpdatedAt,
                              )
                            : t('workspace_models.tooltip.no_sample');
                          return (
                            <AppTooltip
                              key={point.key}
                              title={title}
                              placement='top'
                              color='#ffffff'
                              overlayClassName='workspace-model-health-tooltip-popup'
                              classNames={{ root: 'workspace-model-health-tooltip-popup' }}
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
                        <span>{t('workspace_models.card.health_score')}</span>
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
                        <strong>
                          {formatCount(item.tested_channel_count)} / {formatCount(item.channel_count)}
                        </strong>
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
                      {t('workspace_models.card.last_tested', {
                        time: formatUpdatedAt(item.last_tested_at),
                      })}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </AppSection>
    </div>
  );
};

export default WorkspaceModels;
