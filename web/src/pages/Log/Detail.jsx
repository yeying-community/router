import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';
import { API, showError, timestamp2string } from '../../helpers';
import { renderDisplayAmount, YYC_SYMBOL } from '../../helpers/render';
import {
  AppDetailSection,
  AppFilterHeader,
  AppTag,
} from '../../router-ui';

function renderType(type, t) {
  switch (Number(type)) {
    case 1:
      return (
        <AppTag color='green' className='router-tag'>
          {t('log.type.topup')}
        </AppTag>
      );
    case 2:
      return (
        <AppTag color='olive' className='router-tag'>
          {t('log.type.usage')}
        </AppTag>
      );
    case 3:
      return (
        <AppTag color='orange' className='router-tag'>
          {t('log.type.admin')}
        </AppTag>
      );
    case 4:
      return (
        <AppTag color='purple' className='router-tag'>
          {t('log.type.system')}
        </AppTag>
      );
    case 5:
      return (
        <AppTag color='violet' className='router-tag'>
          {t('log.type.test')}
        </AppTag>
      );
    case 6:
      return (
        <AppTag color='red' className='router-tag'>
          {t('log.type.relay_failure')}
        </AppTag>
      );
    default:
      return (
        <AppTag color='black' className='router-tag'>
          -
        </AppTag>
      );
  }
}

function renderBoolean(value) {
  if (value === true) {
    return 'true';
  }
  if (value === false) {
    return 'false';
  }
  return '-';
}

function renderText(value) {
  const normalized = (value || '').toString().trim();
  return normalized || '-';
}

function renderBillingSource(value, t) {
  const normalized = (value || '').toString().trim().toLowerCase();
  if (normalized === 'package') {
    return t('log.detail.billing_sources.package');
  }
  if (normalized === 'balance') {
    return t('log.detail.billing_sources.balance');
  }
  return renderText(value);
}

function renderEstimatePrecision(value, t) {
  const normalized = (value || '').toString().trim().toLowerCase();
  if (normalized === 'high') {
    return t('log.detail.route.precision.high');
  }
  if (normalized === 'medium') {
    return t('log.detail.route.precision.medium');
  }
  if (normalized === 'low') {
    return t('log.detail.route.precision.low');
  }
  return renderText(value);
}

function renderRouteExplanationSummary(log, t) {
  if (!log) return '-';
  const channel = renderText(log.channel_name || log.channel);
  const model = renderText(log.actual_model_name || log.model_name);
  const source = renderText(log.billing_estimate_source);
  const settlement = renderText(log.billing_settlement_mode);
  const fallbackCount = Number(log.fallback_count || 0);
  const summaryKey =
    Number(log.type) === 6
      ? 'log.detail.route.failure_summary'
      : 'log.detail.route.summary';
  return t(summaryKey, {
    channel,
    model,
    source,
    settlement,
    fallbackCount,
  });
}

function parseFallbackAttempts(value) {
  if (Array.isArray(value)) {
    return value;
  }
  const raw = (value || '').toString().trim();
  if (!raw) {
    return [];
  }
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function renderRelayError(log) {
  const parts = [
    log?.relay_error_type,
    log?.relay_error_code,
    log?.relay_error_message,
  ]
    .map((item) => (item || '').toString().trim())
    .filter(Boolean);
  return parts.length > 0 ? parts.join(' / ') : '-';
}

function formatNumber(value, maximumFractionDigits = 6) {
  if (
    typeof value !== 'number' ||
    Number.isNaN(value) ||
    !Number.isFinite(value)
  ) {
    return '-';
  }
  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits,
  }).format(value);
}

function renderAmount(value, currency) {
  if (
    typeof value !== 'number' ||
    Number.isNaN(value) ||
    !Number.isFinite(value)
  ) {
    return '-';
  }
  const suffix = renderText(currency);
  return suffix === '-'
    ? formatNumber(value, 8)
    : `${formatNumber(value, 8)} ${suffix}`;
}

function renderRate(rate, currency) {
  if (
    typeof rate !== 'number' ||
    Number.isNaN(rate) ||
    !Number.isFinite(rate) ||
    rate <= 0
  ) {
    return '-';
  }
  const suffix = renderText(currency);
  return suffix === '-'
    ? formatNumber(rate, 6)
    : `${formatNumber(rate, 6)} ${YYC_SYMBOL}/${suffix}`;
}

function normalizeLogDetail(data) {
  return {
    ...(data || {}),
    // Prefer charge-amount fields, fall back to legacy quota payloads for old logs.
    chargeAmount: Number(data?.charge_amount ?? data?.quota ?? 0),
    userDailyChargeAmount: Number(data?.user_daily_charge_amount ?? data?.user_daily_quota ?? 0),
    userEmergencyChargeAmount: Number(
      data?.user_emergency_charge_amount ?? data?.user_emergency_quota ?? 0,
    ),
    billingChargeAmount: Number(data?.billing_charge_amount ?? 0),
    estimatedChargeAmount: Number(data?.estimated_charge_amount ?? 0),
    billingChargeDeltaAmount: Number(data?.billing_charge_delta_amount ?? 0),
    billingPromptTokenDelta: Number(data?.billing_prompt_token_delta ?? 0),
    billingOutputTokenDelta: Number(data?.billing_output_token_delta ?? 0),
    billingImageToolCalls: Number(data?.billing_image_tool_calls ?? 0),
    billingImageToolOutputTokens: Number(
      data?.billing_image_tool_output_tokens ?? 0,
    ),
    billingImageToolAmount: Number(data?.billing_image_tool_amount ?? 0),
    billingImageToolChargeAmount: Number(data?.billing_image_tool_charge_amount ?? 0),
  };
}

function renderRouteOutcomeTags(log, fallbackAttempts, t) {
  const failed = Number(log?.type) === 6;
  const fallbackCount = Math.max(
    Number(log?.fallback_count || 0),
    Array.isArray(fallbackAttempts) ? fallbackAttempts.length : 0,
  );
  return (
    <div className='router-route-explain-tags'>
      <AppTag color={failed ? 'red' : 'green'} className='router-tag'>
        {failed
          ? t('log.detail.route.outcome.failed')
          : t('log.detail.route.outcome.succeeded')}
      </AppTag>
      <AppTag color={fallbackCount > 0 ? 'orange' : 'blue'} className='router-tag'>
        {fallbackCount > 0
          ? t('log.detail.route.outcome.fallback', { count: fallbackCount })
          : t('log.detail.route.outcome.direct')}
      </AppTag>
    </div>
  );
}

function renderFallbackAttemptCards(attempts, t) {
  if (!Array.isArray(attempts) || attempts.length === 0) {
    return <div className='router-route-attempt-empty'>-</div>;
  }
  return (
    <div className='router-route-attempt-list'>
      {attempts.map((attempt, index) => {
        const attemptNo = Number(attempt?.attempt || 0) || index + 1;
        return (
          <div
            className='router-route-attempt-card'
            key={`${attemptNo}-${attempt?.channel_id || index}`}
          >
            <div className='router-route-attempt-head'>
              <span>
                {t('log.detail.route.attempt_title', {
                  attempt: attemptNo,
                })}
              </span>
              <AppTag color='red' className='router-tag'>
                HTTP {attempt?.status || '-'}
              </AppTag>
            </div>
            <div className='router-route-attempt-grid'>
              <span>{t('log.detail.route.attempt_fields.channel')}</span>
              <strong>{renderText(attempt?.channel_name || attempt?.channel_id)}</strong>
              <span>{t('log.detail.route.attempt_fields.model')}</span>
              <strong>{renderText(attempt?.model)}</strong>
              <span>{t('log.detail.route.attempt_fields.endpoint')}</span>
              <strong>{renderText(attempt?.endpoint)}</strong>
              <span>{t('log.detail.route.attempt_fields.protocol')}</span>
              <strong>{renderText(attempt?.protocol)}</strong>
              <span>{t('log.detail.route.attempt_fields.error_code')}</span>
              <strong>{renderText(attempt?.error_code)}</strong>
              <span>{t('log.detail.route.attempt_fields.error')}</span>
              <strong>{renderText(attempt?.error)}</strong>
            </div>
          </div>
        );
      })}
    </div>
  );
}

const LogDetail = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const currentPagePath = `${location.pathname}${location.search}${location.hash}`;
  const { id } = useParams();
  const isAdminPage = location.pathname.startsWith('/admin/');
  const [loading, setLoading] = useState(true);
  const [log, setLog] = useState(null);

  const listPath = useMemo(
    () =>
      `${isAdminPage ? '/admin/log' : '/workspace/log'}${location.search || ''}`,
    [isAdminPage, location.search],
  );

  const fallbackAttempts = useMemo(
    () => parseFallbackAttempts(log?.fallback_attempts),
    [log?.fallback_attempts],
  );

  const routeExplanationItems = useMemo(
    () => [
      {
        key: 'channel',
        label: t('log.detail.route.fields.channel_target'),
        value: isAdminPage ? (
          log?.channel ? (
            <AppTag
              className='router-tag'
              as={Link}
              to={`/admin/channel/detail/${log.channel}`}
              state={{ from: currentPagePath }}
            >
              {log?.channel_name || log?.channel}
            </AppTag>
          ) : (
            '-'
          )
        ) : (
          renderText(log?.channel_name || log?.channel)
        ),
      },
      {
        key: 'model',
        label: t('log.detail.route.fields.request_model'),
        value: renderText(log?.request_model_name || log?.model_name),
      },
      {
        key: 'actual_model',
        label: t('log.detail.route.fields.actual_model'),
        value: renderText(log?.actual_model_name || log?.model_name),
      },
      {
        key: 'upstream_endpoint',
        label: t('log.detail.route.fields.upstream_endpoint'),
        value: renderText(log?.upstream_endpoint),
      },
      {
        key: 'upstream_protocol',
        label: t('log.detail.route.fields.upstream_protocol'),
        value: renderText(log?.upstream_protocol),
      },
      {
        key: 'stream',
        label: t('log.detail.route.fields.stream_mode'),
        value: renderBoolean(log?.is_stream),
      },
      {
        key: 'latency',
        label: t('log.detail.route.fields.elapsed_time'),
        value: log?.elapsed_time ? `${log.elapsed_time} ms` : '-',
      },
      {
        key: 'estimate_source',
        label: t('log.detail.route.fields.estimate_source'),
        value: renderText(log?.billing_estimate_source),
      },
      {
        key: 'estimate_estimator',
        label: t('log.detail.route.fields.estimate_estimator'),
        value: renderText(log?.billing_estimate_estimator),
      },
      {
        key: 'estimate_precision',
        label: t('log.detail.route.fields.estimate_precision'),
        value: renderEstimatePrecision(log?.billing_estimate_precision, t),
      },
      {
        key: 'settlement_mode',
        label: t('log.detail.route.fields.settlement_mode'),
        value: renderText(log?.billing_settlement_mode),
      },
      {
        key: 'trace_id',
        label: t('log.detail.route.fields.trace_id'),
        value: renderText(log?.trace_id),
      },
      {
        key: 'fallback_count',
        label: t('log.detail.route.fields.fallback_count'),
        value: Number(log?.fallback_count || 0),
      },
      {
        key: 'relay_error',
        label: t('log.detail.route.fields.relay_error'),
        value: renderRelayError(log),
        span: true,
      },
      {
        key: 'fallback_attempts',
        label: t('log.detail.route.fields.fallback_attempts'),
        value: renderFallbackAttemptCards(fallbackAttempts, t),
        span: true,
      },
    ],
    [currentPagePath, fallbackAttempts, isAdminPage, log, t],
  );

  const loadDetail = useCallback(async () => {
    setLoading(true);
    try {
      const endpoint = isAdminPage
        ? `/api/v1/admin/log/${id}`
        : `/api/v1/public/log/${id}`;
      const res = await API.get(endpoint);
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('log.messages.load_failed'));
        return;
      }
      setLog(normalizeLogDetail(data || null));
    } catch (error) {
      showError(error?.message || t('log.messages.load_failed'));
    } finally {
      setLoading(false);
    }
  }, [id, isAdminPage, t]);

  useEffect(() => {
    loadDetail().then();
  }, [loadDetail]);

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={[
          {
            key: 'section',
            label: isAdminPage
              ? t('header.platform_operation')
              : t('header.records'),
          },
          {
            key: 'log-list',
            label: t('header.log'),
            onClick: () => navigate(listPath),
          },
          {
            key: 'log-current',
            label: renderText(log?.id || id),
            active: true,
          },
        ]}
        title={t('log.detail.title')}
      />
      <div className='router-entity-detail-page'>
        {loading ? (
          <div className='router-empty-cell'>{t('common.loading')}</div>
        ) : (
          <>
            <AppDetailSection title={t('log.detail.sections.basic')} titleTag='div'>
                  <div className='router-detail-grid'>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.id')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderText(log?.id)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.time')}
                      </div>
                      <pre className='router-detail-value'>
                        {log?.created_at
                          ? timestamp2string(log.created_at)
                          : '-'}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.type')}
                      </div>
                      <div className='router-detail-value'>
                        {renderType(log?.type, t)}
                      </div>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.channel')}
                      </div>
                      <div className='router-detail-value'>
                          {isAdminPage ? (
                            log?.channel ? (
                            <AppTag
                              className='router-tag'
                              as={Link}
                              to={`/admin/channel/detail/${log.channel}`}
                              state={{ from: currentPagePath }}
                            >
                              {log?.channel_name || log?.channel}
                            </AppTag>
                          ) : (
                            '-'
                          )
                        ) : (
                          '-'
                        )}
                      </div>
                    </div>
                    {isAdminPage ? (
                      <div className='router-detail-item'>
                        <div className='router-detail-label'>
                          {t('log.detail.fields.group')}
                        </div>
                        <div className='router-detail-value'>
                          {log?.group_id ? (
                            <AppTag
                              className='router-tag'
                              as={Link}
                              to={`/admin/group/detail/${log.group_id}`}
                              state={{ from: currentPagePath }}
                            >
                              {log?.group_name || log?.group_id}
                            </AppTag>
                          ) : (
                            '-'
                          )}
                        </div>
                      </div>
                    ) : null}
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.model')}
                      </div>
                      <pre className='router-detail-value router-monospace-value'>
                        {renderText(log?.model_name)}
                      </pre>
                    </div>
                    {isAdminPage ? (
                      <div className='router-detail-item'>
                        <div className='router-detail-label'>
                          {t('log.detail.fields.username')}
                        </div>
                        <pre className='router-detail-value'>
                          {renderText(log?.username)}
                        </pre>
                      </div>
                    ) : null}
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.token_name')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderText(log?.token_name)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.trace_id')}
                      </div>
                      <pre className='router-detail-value router-monospace-value'>
                        {renderText(log?.trace_id)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.prompt_tokens')}
                      </div>
                      <pre className='router-detail-value'>
                        {log?.prompt_tokens ?? '-'}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.completion_tokens')}
                      </div>
                      <pre className='router-detail-value'>
                        {log?.completion_tokens ?? '-'}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.quota')}
                      </div>
                      <div className='router-detail-value'>
                        {typeof log?.chargeAmount === 'number'
                          ? renderDisplayAmount(log.chargeAmount, t, 6)
                          : '-'}
                      </div>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_source')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderBillingSource(log?.billing_source, t)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.user_daily_quota')}
                      </div>
                      <div className='router-detail-value'>
                        {typeof log?.userDailyChargeAmount === 'number'
                          ? renderDisplayAmount(log.userDailyChargeAmount, t, 6)
                          : '-'}
                      </div>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.user_emergency_quota')}
                      </div>
                      <div className='router-detail-value'>
                        {typeof log?.userEmergencyChargeAmount === 'number'
                          ? renderDisplayAmount(log.userEmergencyChargeAmount, t, 6)
                          : '-'}
                      </div>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.elapsed_time')}
                      </div>
                      <pre className='router-detail-value'>
                        {log?.elapsed_time ? `${log.elapsed_time} ms` : '-'}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.is_stream')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderBoolean(log?.is_stream)}
                      </pre>
                    </div>
                  </div>
            </AppDetailSection>

            <AppDetailSection title={t('log.detail.sections.route')} titleTag='div'>
                  <div className='router-detail-grid'>
                    <div className='router-detail-item router-detail-item-span-2'>
                      <div className='router-route-explain-header'>
                        <div className='router-detail-label'>
                          {t('log.detail.route.summary_title')}
                        </div>
                        {renderRouteOutcomeTags(log, fallbackAttempts, t)}
                      </div>
                      <pre className='router-detail-value'>
                        {renderRouteExplanationSummary(log, t)}
                      </pre>
                    </div>
                    {routeExplanationItems.map((item) => (
                      <div
                        key={item.key}
                        className={`router-detail-item${item.span ? ' router-detail-item-span-2' : ''}`}
                      >
                        <div className='router-detail-label'>{item.label}</div>
                        <div className='router-detail-value'>{item.value}</div>
                      </div>
                    ))}
                  </div>
            </AppDetailSection>

            <AppDetailSection title={t('log.detail.sections.billing')} titleTag='div'>
                  <div className='router-detail-grid'>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_price_unit')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderText(log?.billing_price_unit)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_currency')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderText(log?.billing_currency)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_pricing_source')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderText(log?.billing_pricing_source)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_usage_source')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderText(log?.billing_usage_source)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_estimate_source')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderText(log?.billing_estimate_source)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_estimate_estimator')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderText(log?.billing_estimate_estimator)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_estimate_precision')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderText(log?.billing_estimate_precision)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.estimated_prompt_tokens')}
                      </div>
                      <pre className='router-detail-value'>
                        {log?.estimated_prompt_tokens ?? '-'}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.estimated_output_tokens')}
                      </div>
                      <pre className='router-detail-value'>
                        {log?.estimated_output_tokens ?? '-'}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_settlement_mode')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderText(log?.billing_settlement_mode)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_group_ratio')}
                      </div>
                      <pre className='router-detail-value'>
                        {formatNumber(log?.billing_group_ratio, 6)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_charge_rate')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderRate(
                          log?.billing_charge_rate,
                          log?.billing_currency,
                        )}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_input_quantity')}
                      </div>
                      <pre className='router-detail-value'>
                        {formatNumber(log?.billing_input_quantity, 6)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_output_quantity')}
                      </div>
                      <pre className='router-detail-value'>
                        {formatNumber(log?.billing_output_quantity, 6)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_cache_read_quantity')}
                      </div>
                      <pre className='router-detail-value'>
                        {formatNumber(log?.billing_cache_read_quantity, 6)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_cache_write_quantity')}
                      </div>
                      <pre className='router-detail-value'>
                        {formatNumber(log?.billing_cache_write_quantity, 6)}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_input_amount')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderAmount(
                          log?.billing_input_amount,
                          log?.billing_currency,
                        )}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_output_amount')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderAmount(
                          log?.billing_output_amount,
                          log?.billing_currency,
                        )}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_cache_read_amount')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderAmount(
                          log?.billing_cache_read_amount,
                          log?.billing_currency,
                        )}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_cache_write_amount')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderAmount(
                          log?.billing_cache_write_amount,
                          log?.billing_currency,
                        )}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_amount')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderAmount(
                          log?.billing_amount,
                          log?.billing_currency,
                        )}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_charge_amount')}
                      </div>
                      <div className='router-detail-value'>
                        {typeof log?.billingChargeAmount === 'number'
                          ? renderDisplayAmount(log.billingChargeAmount, t, 6)
                          : '-'}
                      </div>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.estimated_charge_amount')}
                      </div>
                      <div className='router-detail-value'>
                        {typeof log?.estimatedChargeAmount === 'number'
                          ? renderDisplayAmount(log.estimatedChargeAmount, t, 6)
                          : '-'}
                      </div>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_charge_delta_amount')}
                      </div>
                      <div className='router-detail-value'>
                        {typeof log?.billingChargeDeltaAmount === 'number'
                          ? renderDisplayAmount(
                              log.billingChargeDeltaAmount,
                              t,
                              6,
                            )
                          : '-'}
                      </div>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_prompt_token_delta')}
                      </div>
                      <pre className='router-detail-value'>
                        {Number.isFinite(log?.billingPromptTokenDelta)
                          ? formatNumber(log.billingPromptTokenDelta, 0)
                          : '-'}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_output_token_delta')}
                      </div>
                      <pre className='router-detail-value'>
                        {Number.isFinite(log?.billingOutputTokenDelta)
                          ? formatNumber(log.billingOutputTokenDelta, 0)
                          : '-'}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_image_tool_calls')}
                      </div>
                      <pre className='router-detail-value'>
                        {log?.billingImageToolCalls > 0
                          ? log.billingImageToolCalls
                          : '-'}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t(
                          'log.detail.fields.billing_image_tool_output_tokens',
                        )}
                      </div>
                      <pre className='router-detail-value'>
                        {log?.billingImageToolOutputTokens > 0
                          ? formatNumber(log.billingImageToolOutputTokens, 0)
                          : '-'}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_image_tool_amount')}
                      </div>
                      <pre className='router-detail-value'>
                        {log?.billingImageToolAmount > 0
                          ? renderAmount(
                              log.billingImageToolAmount,
                              log?.billing_currency,
                            )
                          : '-'}
                      </pre>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.billing_image_tool_charge_amount')}
                      </div>
                      <div className='router-detail-value'>
                        {log?.billingImageToolChargeAmount > 0
                          ? renderDisplayAmount(
                              log.billingImageToolChargeAmount,
                              t,
                              6,
                            )
                          : '-'}
                      </div>
                    </div>
                  </div>
            </AppDetailSection>

            <AppDetailSection title={t('log.detail.sections.content')} titleTag='div'>
                  <pre className='router-detail-pre'>
                    {renderText(log?.content)}
                  </pre>
            </AppDetailSection>
          </>
        )}
      </div>
    </div>
  );
};

export default LogDetail;
