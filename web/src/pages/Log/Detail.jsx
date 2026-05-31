import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';
import { API, showError, timestamp2string } from '../../helpers';
import { renderDisplayAmount, YYC_SYMBOL } from '../../helpers/render';
import {
  AppDetailSection,
  AppFilterHeader,
  AppIcon,
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
    // Prefer YYC-native fields, fall back to legacy quota payloads for old logs.
    yycAmount: Number(data?.yyc_amount ?? data?.quota ?? 0),
    userDailyYYC: Number(data?.yyc_user_daily ?? data?.user_daily_quota ?? 0),
    userEmergencyYYC: Number(
      data?.yyc_user_emergency ?? data?.user_emergency_quota ?? 0,
    ),
    billingYYCAmount: Number(data?.billing_yyc_amount ?? 0),
    billingImageToolCalls: Number(data?.billing_image_tool_calls ?? 0),
    billingImageToolOutputTokens: Number(
      data?.billing_image_tool_output_tokens ?? 0,
    ),
    billingImageToolAmount: Number(data?.billing_image_tool_amount ?? 0),
    billingImageToolYYCAmount: Number(data?.billing_image_tool_yyc_amount ?? 0),
  };
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
                        {typeof log?.yycAmount === 'number'
                          ? renderDisplayAmount(log.yycAmount, t, 6)
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
                        {typeof log?.userDailyYYC === 'number'
                          ? renderDisplayAmount(log.userDailyYYC, t, 6)
                          : '-'}
                      </div>
                    </div>
                    <div className='router-detail-item'>
                      <div className='router-detail-label'>
                        {t('log.detail.fields.user_emergency_quota')}
                      </div>
                      <div className='router-detail-value'>
                        {typeof log?.userEmergencyYYC === 'number'
                          ? renderDisplayAmount(log.userEmergencyYYC, t, 6)
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
                        {t('log.detail.fields.billing_yyc_rate')}
                      </div>
                      <pre className='router-detail-value'>
                        {renderRate(
                          log?.billing_yyc_rate,
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
                        {t('log.detail.fields.billing_yyc_amount')}
                      </div>
                      <div className='router-detail-value'>
                        {typeof log?.billingYYCAmount === 'number'
                          ? renderDisplayAmount(log.billingYYCAmount, t, 6)
                          : '-'}
                      </div>
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
                        {t('log.detail.fields.billing_image_tool_yyc_amount')}
                      </div>
                      <div className='router-detail-value'>
                        {log?.billingImageToolYYCAmount > 0
                          ? renderDisplayAmount(
                              log.billingImageToolYYCAmount,
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
