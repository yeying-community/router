import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Card, Label } from 'semantic-ui-react';
import { useTranslation } from 'react-i18next';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';
import { API, showError, timestamp2string } from '../../helpers';
import { renderQuota } from '../../helpers/render';

function renderType(type, t) {
  switch (Number(type)) {
    case 1:
      return (
        <Label basic color='green' className='router-tag'>
          {t('log.type.topup')}
        </Label>
      );
    case 2:
      return (
        <Label basic color='olive' className='router-tag'>
          {t('log.type.usage')}
        </Label>
      );
    case 3:
      return (
        <Label basic color='orange' className='router-tag'>
          {t('log.type.admin')}
        </Label>
      );
    case 4:
      return (
        <Label basic color='purple' className='router-tag'>
          {t('log.type.system')}
        </Label>
      );
    case 5:
      return (
        <Label basic color='violet' className='router-tag'>
          {t('log.type.test')}
        </Label>
      );
    default:
      return (
        <Label basic color='black' className='router-tag'>
          -
        </Label>
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

function formatNumber(value, maximumFractionDigits = 6) {
  if (typeof value !== 'number' || Number.isNaN(value) || !Number.isFinite(value)) {
    return '-';
  }
  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits,
  }).format(value);
}

function renderAmount(value, currency) {
  if (typeof value !== 'number' || Number.isNaN(value) || !Number.isFinite(value)) {
    return '-';
  }
  const suffix = renderText(currency);
  return suffix === '-' ? formatNumber(value, 8) : `${formatNumber(value, 8)} ${suffix}`;
}

function renderRate(rate, currency) {
  if (typeof rate !== 'number' || Number.isNaN(rate) || !Number.isFinite(rate) || rate <= 0) {
    return '-';
  }
  const suffix = renderText(currency);
  return suffix === '-' ? formatNumber(rate, 6) : `${formatNumber(rate, 6)} YYC/${suffix}`;
}

function normalizeLogDetail(data) {
  return {
    ...(data || {}),
    quota: Number(data?.yyc_amount ?? data?.quota ?? 0),
    user_daily_quota: Number(data?.yyc_user_daily ?? data?.user_daily_quota ?? 0),
    user_emergency_quota: Number(data?.yyc_user_emergency ?? data?.user_emergency_quota ?? 0),
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
    () => `${isAdminPage ? '/admin/log' : '/workspace/log'}${location.search || ''}`,
    [isAdminPage, location.search]
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
      <Card fluid className='chart-card'>
        <Card.Content>
          <Card.Header className='header router-page-title'>
            {t('log.detail.title')}
          </Card.Header>
          <div className='router-toolbar router-block-gap-sm'>
            <div className='router-toolbar-start'>
              <Button
                type='button'
                className='router-page-button'
                onClick={() => navigate(listPath)}
              >
                {t('log.detail.buttons.back')}
              </Button>
            </div>
          </div>

          {loading ? (
            <div className='router-empty-cell'>{t('common.loading')}</div>
          ) : (
            <>
              <div className='router-detail-section'>
                <div className='router-detail-section-title'>
                  {t('log.detail.sections.basic')}
                </div>
                <div className='router-detail-grid'>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('log.detail.fields.id')}
                    </div>
                    <pre className='router-detail-value'>{renderText(log?.id)}</pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('log.detail.fields.time')}
                    </div>
                    <pre className='router-detail-value'>
                      {log?.created_at ? timestamp2string(log.created_at) : '-'}
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
                          <Label
                            basic
                            className='router-tag'
                            as={Link}
                            to={`/channel/detail/${log.channel}`}
                            state={{ from: currentPagePath }}
                          >
                            {log?.channel_name || log?.channel}
                          </Label>
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
                          <Label
                            basic
                            className='router-tag'
                            as={Link}
                            to={`/admin/group/detail/${log.group_id}`}
                            state={{ from: currentPagePath }}
                          >
                            {log?.group_name || log?.group_id}
                          </Label>
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
                    <pre className='router-detail-value'>
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
                    <pre className='router-detail-value'>
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
                      {typeof log?.quota === 'number'
                        ? renderQuota(log.quota, t, 6)
                        : '-'}
                    </div>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('log.detail.fields.user_daily_quota')}
                    </div>
                    <div className='router-detail-value'>
                      {typeof log?.user_daily_quota === 'number'
                        ? renderQuota(log.user_daily_quota, t, 6)
                        : '-'}
                    </div>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('log.detail.fields.user_emergency_quota')}
                    </div>
                    <div className='router-detail-value'>
                      {typeof log?.user_emergency_quota === 'number'
                        ? renderQuota(log.user_emergency_quota, t, 6)
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
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('log.detail.fields.system_prompt_reset')}
                    </div>
                    <pre className='router-detail-value'>
                      {renderBoolean(log?.system_prompt_reset)}
                    </pre>
                  </div>
                </div>
              </div>

              <div className='router-detail-section'>
                <div className='router-detail-section-title'>
                  {t('log.detail.sections.billing')}
                </div>
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
                      {renderRate(log?.billing_yyc_rate, log?.billing_currency)}
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
                      {renderAmount(log?.billing_input_amount, log?.billing_currency)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('log.detail.fields.billing_output_amount')}
                    </div>
                    <pre className='router-detail-value'>
                      {renderAmount(log?.billing_output_amount, log?.billing_currency)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('log.detail.fields.billing_amount')}
                    </div>
                    <pre className='router-detail-value'>
                      {renderAmount(log?.billing_amount, log?.billing_currency)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('log.detail.fields.billing_yyc_amount')}
                    </div>
                    <div className='router-detail-value'>
                      {typeof log?.billing_yyc_amount === 'number'
                        ? renderQuota(log.billing_yyc_amount, t, 6)
                        : '-'}
                    </div>
                  </div>
                </div>
              </div>

              <div className='router-detail-section'>
                <div className='router-detail-section-title'>
                  {t('log.detail.sections.content')}
                </div>
                <pre className='router-detail-pre'>{renderText(log?.content)}</pre>
              </div>
            </>
          )}
        </Card.Content>
      </Card>
    </div>
  );
};

export default LogDetail;
