import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useParams } from 'react-router-dom';
import { API, showError, timestamp2string } from '../../helpers';
import {
  formatRequestCount,
  getServicePackagePeriodLabel,
  getServicePackageTypeLabel,
} from '../../helpers/package';
import {
  AppButton,
  AppFilterHeader,
  AppSection,
  AppStatistic,
  AppTag,
} from '../../router-ui';
import TopUpWorkspaceProvider from './provider.jsx';
import {
  renderTopupIntegerAmountWithExactPopup,
  SupportedModelsSummary,
  useTopUpWorkspace,
} from './shared.jsx';

const renderStatus = (status, t) => {
  const color =
    status === 'active'
      ? 'green'
      : status === 'canceled'
        ? 'red'
        : status === 'replaced'
          ? 'blue'
          : 'grey';
  return (
    <AppTag color={color}>
      {t(`topup.quota_cards.status.${status || 'unknown'}`)}
    </AppTag>
  );
};

const QuotaCardDetailPageInner = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { kind, id } = useParams();
  const { displayCurrency, displayCurrencyIndex } = useTopUpWorkspace();
  const [card, setCard] = useState(null);
  const [loading, setLoading] = useState(false);

  const loadCard = useCallback(async () => {
    setLoading(true);
    try {
      const response = await API.get(
        `/api/v1/public/user/quota/cards/${encodeURIComponent(kind || '')}/${encodeURIComponent(id || '')}`,
      );
      const payload = response?.data || {};
      if (!payload.success) {
        throw new Error(
          payload.message || t('topup.quota_cards.detail_load_failed'),
        );
      }
      setCard(payload.data || null);
    } catch (error) {
      showError(
        error?.message || t('topup.quota_cards.detail_load_failed'),
      );
    } finally {
      setLoading(false);
    }
  }, [id, kind, t]);

  useEffect(() => {
    loadCard().then();
  }, [loadCard]);

  const renderAmount = useCallback(
    (amount) =>
      renderTopupIntegerAmountWithExactPopup({
        chargeAmount: Number(amount || 0),
        displayCurrency,
        displayCurrencyIndex,
      }),
    [displayCurrency, displayCurrencyIndex],
  );

  const isRequestCount = card?.metric === 'request_count';
  const unlimited =
    isRequestCount &&
    (card?.package?.usage?.unlimited === true ||
      Number(card?.package?.period_limit || 0) <= 0);
  const formatValue = useCallback(
    (value, allowUnlimited = false) => {
      if (isRequestCount) {
        if (allowUnlimited && unlimited) {
          return t('common.unlimited');
        }
        return `${formatRequestCount(value)} ${t('package_manage.request_unit')}`;
      }
      return renderAmount(value);
    },
    [isRequestCount, renderAmount, t, unlimited],
  );

  const packageDetail = card?.package || null;
  const balanceDetail = card?.balance_lot || null;
  const sourceDetail = balanceDetail?.source_detail || null;
  const detailRows = useMemo(() => {
    if (packageDetail) {
      return [
        [
          t('package_manage.table.package_type'),
          getServicePackageTypeLabel(packageDetail, t),
        ],
        [
          t('package_manage.table.period_entitlement'),
          isRequestCount
            ? formatValue(packageDetail.period_limit, true)
            : renderAmount(packageDetail.daily_quota_limit),
        ],
        [
          t('topup.quota_cards.period'),
          getServicePackagePeriodLabel(packageDetail.period_type, t),
        ],
        [t('user.detail.package_group'), packageDetail.group_name || '-'],
        [
          t('user.detail.package_timezone'),
          packageDetail.quota_reset_timezone || '-',
        ],
      ];
    }
    return [
      [t('topup.quota_cards.source_title'), sourceDetail?.title || '-'],
      [t('topup.quota_cards.source_status'), sourceDetail?.status || '-'],
      [
        t('topup.quota_cards.credit_origin'),
        sourceDetail?.credit_origin || card?.kind || '-',
      ],
      [
        t('topup.quota_cards.source_amount'),
        sourceDetail?.amount
          ? `${sourceDetail.amount} ${sourceDetail.currency || ''}`.trim()
          : '-',
      ],
    ];
  }, [
    card?.kind,
    formatValue,
    isRequestCount,
    packageDetail,
    renderAmount,
    sourceDetail,
    t,
  ]);

  const cardName = card?.name || t('topup.quota_cards.detail_title');

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'mine', label: t('header.mine') },
          { key: 'quota', label: t('topup.mine.quota') },
          { key: 'card', label: cardName, active: true },
        ]}
        title={cardName}
        actions={
          <AppButton onClick={() => navigate(-1)}>
            {t('common.back')}
          </AppButton>
        }
      />
      <AppSection
        title={cardName}
        extra={card ? renderStatus(card.status, t) : null}
      >
        {card ? (
          <>
            <div className='router-quota-summary-grid'>
              <div className='router-quota-summary-item'>
                <AppStatistic
                  title={t('topup.quota_cards.total')}
                  value={0}
                  formatter={() => formatValue(card.total_amount, true)}
                />
              </div>
              <div className='router-quota-summary-item'>
                <AppStatistic
                  title={t('topup.quota_cards.used')}
                  value={0}
                  formatter={() => formatValue(card.used_amount)}
                />
              </div>
              <div className='router-quota-summary-item'>
                <AppStatistic
                  title={t('topup.quota_cards.remaining')}
                  value={0}
                  formatter={() =>
                    formatValue(card.remaining_amount, card.status === 'active')
                  }
                />
              </div>
            </div>
            <div className='router-quota-detail-grid'>
              <div>
                <span>{t('topup.quota_cards.activated_at')}</span>
                <strong>
                  {card.activated_at
                    ? timestamp2string(card.activated_at)
                    : '-'}
                </strong>
              </div>
              <div>
                <span>{t('topup.quota_cards.expires_at')}</span>
                <strong>
                  {Number(card.expires_at || 0) > 0
                    ? timestamp2string(card.expires_at)
                    : t('common.never')}
                </strong>
              </div>
              {detailRows.map(([label, value]) => (
                <div key={label}>
                  <span>{label}</span>
                  <strong>{value}</strong>
                </div>
              ))}
            </div>
            {packageDetail ? (
              <SupportedModelsSummary
                models={packageDetail.supported_models}
                t={t}
                label={t('user.detail.package_supported_models')}
              />
            ) : null}
          </>
        ) : loading ? null : (
          <div className='router-empty'>
            {t('topup.quota_cards.detail_empty')}
          </div>
        )}
      </AppSection>
    </div>
  );
};

const QuotaCardDetailPage = () => (
  <TopUpWorkspaceProvider>
    <QuotaCardDetailPageInner />
  </TopUpWorkspaceProvider>
);

export default QuotaCardDetailPage;
