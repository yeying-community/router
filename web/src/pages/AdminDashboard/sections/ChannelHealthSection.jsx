import React, { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
import {
  AppErrorState,
  AppSegmented,
  AppSelect,
  AppSpin,
  AppTag,
  AppToolbar,
  AppTooltip,
} from '../../../router-ui';
import {
  CHANNEL_HEALTH_POINT_COLORS,
  buildChannelHealthHistory,
  formatCount,
  formatPercent,
  formatTimeRange,
  formatUpdatedAt,
  isActiveCircuitBreaker,
  normalizeCircuitBreakerState,
  toPercent,
  useAdminDashboardData,
} from '../dashboardShared';
import { DashboardSectionControls } from '../DashboardSectionControls';

const HEALTH_LEVEL_RANK = {
  critical: 0,
  warning: 1,
  unknown: 2,
  healthy: 3,
};

const PERIOD_OPTIONS = [
  { value: 'last_hour', label: '1 小时' },
  { value: 'last_24_hours', label: '24 小时' },
  { value: 'last_7_days', label: '7 天' },
  { value: 'last_30_days', label: '30 天' },
];

const OPERATOR_OPTION_DEFINITIONS = [
  { value: 'all', labelKey: 'dashboard.admin.channels.operator.all' },
  {
    value: 'platform',
    labelKey: 'dashboard.admin.channels.operator.platform',
  },
  {
    value: 'independent',
    labelKey: 'dashboard.admin.channels.operator.independent',
  },
];

const normalizeOperatorType = (operatorType, ownershipType, sourceKind) => {
  const normalizedOperatorType = String(operatorType || '').trim();
  if (normalizedOperatorType === 'platform') return 'platform';
  if (
    normalizedOperatorType === 'independent' ||
    normalizedOperatorType === 'third_party'
  ) {
    return 'independent';
  }

  const normalizedOwnershipType = String(ownershipType || '').trim();
  if (normalizedOwnershipType === 'platform') return 'platform';
  if (
    normalizedOwnershipType === 'independent' ||
    normalizedOwnershipType === 'third_party'
  ) {
    return 'independent';
  }
  return sourceKind === 'managed' ? 'platform' : 'independent';
};

const compareRiskFirst = (left, right) => {
  if (left.is_active_circuit !== right.is_active_circuit) {
    return left.is_active_circuit ? -1 : 1;
  }

  const leftLevel = HEALTH_LEVEL_RANK[left.health_level] ?? 2;
  const rightLevel = HEALTH_LEVEL_RANK[right.health_level] ?? 2;
  if (leftLevel !== rightLevel) return leftLevel - rightLevel;

  if (left.needs_retest !== right.needs_retest) {
    return left.needs_retest ? -1 : 1;
  }
  if (left.is_high_latency !== right.is_high_latency) {
    return left.is_high_latency ? -1 : 1;
  }
  if (left.health_score !== right.health_score) {
    return left.health_score - right.health_score;
  }
  if (left.request_count !== right.request_count) {
    return right.request_count - left.request_count;
  }
  return String(left.name || '').localeCompare(String(right.name || ''));
};

// Channel health is an observation surface, not a second incident queue. It
// stays intentionally flat: one diagnostic matrix, one strong visual signal
// (the five-hour heatmap), and a direct hand-off to the duty station.
const ChannelHealthSection = ({ injectedData = null }) => {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const [period, setPeriod] = useState('last_24_hours');
  const [operatorFilter, setOperatorFilter] = useState('all');

  const self = useAdminDashboardData(
    'channels',
    { period },
    {
      enabled: !injectedData,
    }
  );
  const { dashboard, loading, error, reload } = injectedData || self;

  const operatorOptions = useMemo(
    () =>
      OPERATOR_OPTION_DEFINITIONS.map((option) => ({
        value: option.value,
        label: t(option.labelKey),
      })),
    [t]
  );

  const renderCapabilities = useCallback(
    (raw) => {
      if (!Array.isArray(raw) || raw.length === 0) return '-';
      return raw
        .map((item) =>
          t(`dashboard.admin.capabilities.${item}`, { defaultValue: item })
        )
        .join(' / ');
    },
    [t]
  );

  const channelHealthData = useMemo(
    () =>
      (dashboard.top_channels || []).map((row, index) => {
        const healthLevel = String(row.health_level || 'unknown')
          .trim()
          .toLowerCase();
        const successRatePercent = toPercent(row.success_rate);
        const verificationPassRatePercent = toPercent(
          row.verification_pass_rate
        );
        const coverageRatePercent = toPercent(row.coverage_rate);
        const selectedModelCount = Number(row.selected_model_count || 0);
        const testedModelCount = Number(row.tested_model_count || 0);
        const hasTestData = Boolean(row.has_test_data);
        const activeCircuit = isActiveCircuitBreaker(row.circuit_breaker);
        const sourceKind =
          String(row.source_kind || 'managed').trim() || 'managed';
        const ownershipType =
          String(row.ownership_type || '').trim() ||
          (sourceKind === 'managed' ? 'platform' : 'third_party');
        const operatorType = normalizeOperatorType(
          row.operator_type,
          ownershipType,
          sourceKind
        );
        const channelPurpose =
          String(row.channel_purpose || '').trim() ||
          (sourceKind === 'managed'
            ? 'platform_service'
            : sourceKind === 'personal'
            ? 'self_use'
            : 'supply');
        const needsRetest =
          channelPurpose === 'platform_service'
            ? !hasTestData ||
              testedModelCount < selectedModelCount ||
              coverageRatePercent < 100 ||
              verificationPassRatePercent < 100
            : String(row.verification_status || '') !== 'verified';
        const hasRuntimeData = Boolean(row.has_runtime_data);
        const isRisk =
          ['critical', 'warning'].includes(healthLevel) ||
          (hasRuntimeData && successRatePercent < 80);
        const isHighLatency = Number(row.avg_latency_ms || 0) >= 8000;

        return {
          id: row.id || `channel-${index}`,
          channel_id: row.id || '',
          source_kind: sourceKind,
          operator_type: operatorType,
          ownership_type: ownershipType,
          channel_purpose: channelPurpose,
          access_scope: row.access_scope || row.visibility || '',
          owner_name: row.owner_name || row.owner_id || '',
          visibility: row.visibility || '',
          group_names: Array.isArray(row.group_names) ? row.group_names : [],
          models: Array.isArray(row.models) ? row.models : [],
          offer_count: Number(row.offer_count || 0),
          published_offer_count: Number(row.published_offer_count || 0),
          supply_status: row.supply_status || '',
          name: row.name || row.id || '-',
          status: Number(row.status || 0),
          capabilities: renderCapabilities(row.capabilities),
          health_score: Number(row.health_score || 0),
          request_count: Number(row.recent_request_count || 0),
          health_level: healthLevel,
          success_rate_percent: successRatePercent,
          has_runtime_data: hasRuntimeData,
          verification_status: row.verification_status || 'unverified',
          verification_pass_rate_percent: verificationPassRatePercent,
          coverage_rate_percent: coverageRatePercent,
          avg_latency_ms: Number(row.avg_latency_ms || 0),
          selected_model_count: selectedModelCount,
          tested_model_count: testedModelCount,
          tested_endpoint_count: Number(row.tested_endpoint_count || 0),
          has_test_data: hasTestData,
          supported_count: Number(row.supported_count || 0),
          unsupported_count: Number(row.unsupported_count || 0),
          last_tested_at: Number(row.last_tested_at || 0),
          circuit_breaker: row.circuit_breaker || null,
          health_history: buildChannelHealthHistory(row.health_points),
          is_active_circuit: activeCircuit,
          needs_retest: needsRetest,
          is_risk: isRisk,
          is_high_latency: isHighLatency,
          needs_attention:
            activeCircuit || needsRetest || isRisk || isHighLatency,
        };
      }),
    [dashboard.top_channels, renderCapabilities]
  );

  const sortedChannelHealthData = useMemo(
    () =>
      channelHealthData
        .filter(
          (item) =>
            operatorFilter === 'all' || item.operator_type === operatorFilter
        )
        .sort(compareRiskFirst),
    [channelHealthData, operatorFilter]
  );

  const renderHealthTag = useCallback(
    (level) => {
      const normalized = String(level || 'unknown')
        .trim()
        .toLowerCase();
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

  const renderCircuitBreakerTag = useCallback(
    (circuitBreaker) => {
      const state = normalizeCircuitBreakerState(circuitBreaker?.state);
      if (!state || state === 'recovered') return null;

      const color =
        state === 'open' ? 'red' : state === 'half_open' ? 'orange' : 'grey';
      const details = [
        t(`dashboard.admin.channels.circuit.state.${state}`, {
          defaultValue: t('dashboard.admin.channels.circuit.state.unknown'),
        }),
        circuitBreaker?.reason
          ? `${t('dashboard.admin.channels.circuit.reason')}: ${
              circuitBreaker.reason
            }`
          : null,
        circuitBreaker?.success_rate !== null &&
        circuitBreaker?.success_rate !== undefined
          ? `${t(
              'dashboard.admin.channels.circuit.success_rate'
            )}: ${formatPercent(circuitBreaker.success_rate)}`
          : null,
        circuitBreaker?.disabled_at
          ? `${t(
              'dashboard.admin.channels.circuit.disabled_at'
            )}: ${formatUpdatedAt(circuitBreaker.disabled_at)}`
          : null,
        circuitBreaker?.recover_after
          ? `${t(
              'dashboard.admin.channels.circuit.recover_after'
            )}: ${formatUpdatedAt(circuitBreaker.recover_after)}`
          : null,
      ].filter(Boolean);

      return (
        <AppTooltip title={details.join(' / ')}>
          <AppTag color={color} className='router-tag'>
            {t(`dashboard.admin.channels.circuit.state.${state}`, {
              defaultValue: t('dashboard.admin.channels.circuit.state.unknown'),
            })}
          </AppTag>
        </AppTooltip>
      );
    },
    [t]
  );

  if (error && !loading) {
    return (
      <section className='admin-dashboard-channel-health-surface'>
        <AppErrorState
          message={error?.message || t('common.load_failed')}
          onRetry={reload}
          retryText={t('common.retry')}
        />
      </section>
    );
  }

  return (
    <AppSpin spinning={loading} className='admin-dashboard-content-spin'>
      <section className='admin-dashboard-channel-health-surface'>
        <header className='admin-dashboard-channel-health-page-header'>
          <div className='admin-dashboard-channel-health-page-heading'>
            <h1>{t('dashboard.admin.channels.title')}</h1>
            <p>{t('dashboard.admin.channels.page_hint')}</p>
          </div>
          <AppToolbar
            className='admin-dashboard-section-toolbar'
            start={
              <div className='admin-dashboard-channel-health-filters'>
                <AppSegmented
                  options={PERIOD_OPTIONS}
                  value={period}
                  onChange={(_, detail) => setPeriod(detail.value)}
                />
                <AppSelect
                  value={operatorFilter}
                  options={operatorOptions}
                  onChange={(_, detail) => setOperatorFilter(detail.value)}
                />
              </div>
            }
            end={
              <DashboardSectionControls
                generatedAt={dashboard.generated_at}
                loading={loading}
                onRefresh={reload}
                extra={
                  <button
                    type='button'
                    className='admin-dashboard-channel-duty-link'
                    onClick={() =>
                      navigate(
                        '/admin/dashboard?view=incidents&domain=channels'
                      )
                    }
                  >
                    {t('dashboard.admin.channels.open_duty')}
                  </button>
                }
              />
            }
          />
        </header>

        <div className='admin-dashboard-channel-health-list-header'>
          <div className='admin-dashboard-health-strip-legend'>
            {['success', 'warning', 'failure', 'unknown'].map((state) => (
              <span
                key={state}
                className='admin-dashboard-health-strip-legend-item'
              >
                <span
                  aria-hidden='true'
                  className='admin-dashboard-health-strip-legend-dot'
                  style={{
                    background:
                      CHANNEL_HEALTH_POINT_COLORS[state] ||
                      CHANNEL_HEALTH_POINT_COLORS.unknown,
                  }}
                />
                {t(`dashboard.admin.channels.history.state.${state}`)}
              </span>
            ))}
          </div>
        </div>

        {sortedChannelHealthData.length === 0 ? (
          <div className='admin-dashboard-channel-health-empty'>
            <strong>{t('dashboard.admin.empty.channels')}</strong>
          </div>
        ) : (
          <div className='admin-dashboard-channel-health-matrix'>
            <div
              className='admin-dashboard-channel-health-matrix-header'
              aria-hidden='true'
            >
              <span>{t('dashboard.admin.channels.columns.channel')}</span>
              <span>{t('dashboard.admin.channels.columns.history')}</span>
              <div>
                <span>{t('dashboard.admin.health.chart.health_score')}</span>
                <span>{t('dashboard.admin.health.chart.pass_rate')}</span>
                <span>{t('dashboard.admin.health.chart.avg_latency')}</span>
                <span>{t('dashboard.admin.health.chart.request_count')}</span>
              </div>
            </div>

            {sortedChannelHealthData.map((item) => {
              const statusText = t(
                `dashboard.admin.channel_status.${Number(item.status)}`,
                {
                  defaultValue: t('dashboard.admin.channel_status.default'),
                }
              );
              const lastTested = item.last_tested_at
                ? formatUpdatedAt(item.last_tested_at)
                : '-';
              const canOpenDetail =
                item.operator_type === 'platform' &&
                item.source_kind === 'managed' &&
                Boolean(item.channel_id);
              const operatorLabel = t(
                `dashboard.admin.channels.operator.${item.operator_type}`,
                { defaultValue: item.operator_type }
              );
              const purposeLabel = t(
                `dashboard.admin.channels.purpose.${item.channel_purpose}`,
                { defaultValue: item.channel_purpose }
              );
              const accessScopeLabel = t(
                `dashboard.admin.channels.access_scope.${item.access_scope}`,
                { defaultValue: item.access_scope }
              );
              const supplyStatusLabel = t(
                `dashboard.admin.channels.supply_status.${item.supply_status}`,
                { defaultValue: item.supply_status }
              );
              const modelSummary =
                item.models.length > 3
                  ? `${item.models.slice(0, 3).join(' / ')} +${
                      item.models.length - 3
                    }`
                  : item.models.join(' / ');
              const historyCounts = item.health_history.reduce(
                (counts, point) => ({
                  ...counts,
                  [point.state]: (counts[point.state] || 0) + 1,
                }),
                { success: 0, warning: 0, failure: 0, unknown: 0 }
              );
              const historyLabel = t(
                'dashboard.admin.channels.history.accessible_summary',
                {
                  channel: item.name,
                  success: historyCounts.success,
                  warning: historyCounts.warning,
                  failure: historyCounts.failure,
                  unknown: historyCounts.unknown,
                }
              );

              return (
                <div
                  key={item.id}
                  className={`admin-dashboard-channel-health-row${
                    item.is_active_circuit
                      ? ' admin-dashboard-channel-health-row-circuit'
                      : ''
                  }${
                    item.needs_attention
                      ? ' admin-dashboard-channel-health-row-attention'
                      : ''
                  }`}
                >
                  <div className='admin-dashboard-channel-health-info'>
                    <div className='admin-dashboard-channel-health-title-row'>
                      <button
                        type='button'
                        onClick={() => {
                          if (!canOpenDetail) return;
                          navigate(
                            `/admin/channel/detail/${encodeURIComponent(
                              item.channel_id
                            )}${item.needs_retest ? '?tab=tests' : ''}`,
                            {
                              state: {
                                from: `${location.pathname}${location.search}`,
                              },
                            }
                          );
                        }}
                        className='admin-dashboard-channel-health-name'
                        title={item.name}
                        disabled={!canOpenDetail}
                      >
                        {item.name}
                      </button>
                      {renderHealthTag(item.health_level)}
                      {renderCircuitBreakerTag(item.circuit_breaker)}
                    </div>
                    <div className='admin-dashboard-channel-health-subtitle'>
                      <span>{statusText}</span>
                      <span>{operatorLabel}</span>
                      <span>{purposeLabel}</span>
                      {item.owner_name ? (
                        <span>
                          {t('dashboard.admin.channels.metadata.owner')}：
                          {item.owner_name}
                        </span>
                      ) : null}
                      {item.channel_purpose === 'supply' ? (
                        <>
                          <span>
                            {t('dashboard.admin.channels.metadata.access')}：
                            {accessScopeLabel}
                          </span>
                          <span>
                            {t('dashboard.admin.channels.metadata.offers')}：
                            {item.published_offer_count} / {item.offer_count}
                          </span>
                          <span>
                            {t(
                              'dashboard.admin.channels.metadata.supply_status'
                            )}
                            ：{supplyStatusLabel}
                          </span>
                        </>
                      ) : null}
                      {item.group_names.length ? (
                        <span>
                          {t('dashboard.admin.channels.metadata.groups')}：
                          {item.group_names.join(' / ')}
                        </span>
                      ) : null}
                      {item.models.length ? (
                        <span>
                          {t('dashboard.admin.channels.metadata.models')}：
                          {modelSummary}
                        </span>
                      ) : null}
                      <span>{item.capabilities}</span>
                      <span>
                        {t('dashboard.admin.channels.metadata.verification')}：
                        {item.verification_status === 'verified'
                          ? t('dashboard.admin.channels.verification.verified')
                          : item.verification_status === 'failed'
                          ? t('dashboard.admin.channels.verification.failed')
                          : t(
                              'dashboard.admin.channels.verification.unverified'
                            )}{' '}
                        · {lastTested}
                      </span>
                    </div>
                  </div>

                  <div className='admin-dashboard-channel-health-strip-wrap'>
                    <div
                      className='admin-dashboard-health-strip'
                      role='img'
                      aria-label={historyLabel}
                    >
                      {item.health_history.map((point) => {
                        const stateLabel = t(
                          `dashboard.admin.channels.history.state.${point.state}`
                        );
                        const title = point.observed ? (
                          <div>
                            <div>{`${item.name}: ${stateLabel}`}</div>
                            <div>{`${t(
                              'dashboard.admin.channels.history.window'
                            )}: ${formatTimeRange(
                              point.bucket_start,
                              point.bucket_end
                            )}`}</div>
                            <div>{`${t(
                              'dashboard.admin.channels.history.success_count'
                            )}: ${formatCount(point.success_count)}`}</div>
                            <div>{`${t(
                              'dashboard.admin.channels.history.failure_count'
                            )}: ${formatCount(point.failure_count)}`}</div>
                            <div>{`${t(
                              'dashboard.admin.channels.history.total_count'
                            )}: ${formatCount(point.total_count)}`}</div>
                            <div>{`${t(
                              'dashboard.admin.channels.history.pass_rate'
                            )}: ${formatPercent(point.pass_rate)}`}</div>
                            <div>{`${t(
                              'dashboard.admin.channels.history.avg_latency'
                            )}: ${
                              point.avg_latency_ms > 0
                                ? `${formatCount(point.avg_latency_ms)} ms`
                                : '-'
                            }`}</div>
                          </div>
                        ) : (
                          `${item.name}: ${t(
                            'dashboard.admin.channels.history.no_data'
                          )}`
                        );

                        return (
                          <AppTooltip key={point.key} title={title}>
                            <span
                              aria-hidden='true'
                              className={`admin-dashboard-health-cell admin-dashboard-health-cell-${point.state}`}
                              style={{
                                background:
                                  CHANNEL_HEALTH_POINT_COLORS[point.state] ||
                                  CHANNEL_HEALTH_POINT_COLORS.unknown,
                              }}
                            />
                          </AppTooltip>
                        );
                      })}
                    </div>
                  </div>

                  <div className='admin-dashboard-channel-health-metrics'>
                    <div className='admin-dashboard-channel-health-metric'>
                      <span>
                        {t('dashboard.admin.health.chart.health_score')}
                      </span>
                      <strong>{item.health_score.toFixed(0)}</strong>
                    </div>
                    <div className='admin-dashboard-channel-health-metric'>
                      <span>{t('dashboard.admin.health.chart.pass_rate')}</span>
                      <strong>
                        {item.has_runtime_data
                          ? formatPercent(item.success_rate_percent)
                          : '-'}
                      </strong>
                    </div>
                    <div className='admin-dashboard-channel-health-metric'>
                      <span>
                        {t('dashboard.admin.health.chart.avg_latency')}
                      </span>
                      <strong>
                        {item.avg_latency_ms > 0
                          ? `${formatCount(item.avg_latency_ms)} ms`
                          : '-'}
                      </strong>
                    </div>
                    <div className='admin-dashboard-channel-health-metric'>
                      <span>
                        {t('dashboard.admin.health.chart.request_count')}
                      </span>
                      <strong>{formatCount(item.request_count)}</strong>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </section>
    </AppSpin>
  );
};

export default ChannelHealthSection;
