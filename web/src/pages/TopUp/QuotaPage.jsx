import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
import { API, showError } from '../../helpers';
import { AppButton, AppSection, AppSkeleton, AppStatistic } from '../../router-ui';
import QuotaCardItem from './QuotaCardItem';
import SpendingCalendar from './SpendingCalendar';
import RedeemCodePage from './RedeemCodePage';
import {
  renderTopupIntegerAmountWithExactPopup,
  useTopUpWorkspace,
} from './shared.jsx';

const QuotaPage = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const currentPagePath = `${location.pathname}${location.search}${location.hash}`;
  const { displayCurrency, displayCurrencyIndex } = useTopUpWorkspace();
  const [overview, setOverview] = useState(null);
  const [cards, setCards] = useState([]);
  const [spend, setSpend] = useState(null);
  const [tokenTotal, setTokenTotal] = useState(null);
  const [loading, setLoading] = useState(false);
  const [redeemOpen, setRedeemOpen] = useState(false);

  const loadQuotaPage = useCallback(async () => {
    setLoading(true);
    try {
      const [overviewResponse, cardsResponse, spendResponse, tokenResponse] =
        await Promise.all([
          API.get('/api/v1/public/user/quota/overview'),
          API.get('/api/v1/public/user/quota/cards', {
            params: { scope: 'active', page: 1, page_size: 50 },
          }),
          API.get('/api/v1/public/user/spend/overview', {
            params: { period: 'last_30_days' },
          }).catch(() => null),
          API.get('/api/v1/public/token/?page=1').catch(() => null),
        ]);
      const overviewPayload = overviewResponse?.data || {};
      const cardsPayload = cardsResponse?.data || {};
      if (!overviewPayload.success) {
        throw new Error(
          overviewPayload.message || t('topup.quota_overview.load_failed'),
        );
      }
      if (!cardsPayload.success) {
        throw new Error(
          cardsPayload.message || t('topup.quota_cards.load_failed'),
        );
      }
      setOverview(overviewPayload.data || null);
      setCards(
        Array.isArray(cardsPayload.data?.items)
          ? cardsPayload.data.items
          : [],
      );
      const spendPayload = spendResponse?.data;
      setSpend(spendPayload?.success ? spendPayload.data || null : null);
      const tokenPayload = tokenResponse?.data;
      if (tokenPayload?.success) {
        setTokenTotal(
          Number(
            tokenPayload.meta?.total ||
              (Array.isArray(tokenPayload.data) ? tokenPayload.data.length : 0),
          ),
        );
      } else {
        setTokenTotal(null);
      }
    } catch (error) {
      showError(
        error?.message || t('topup.quota_overview.load_failed'),
      );
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadQuotaPage().then();
  }, [loadQuotaPage]);

  const renderAmount = useCallback(
    (amount) =>
      renderTopupIntegerAmountWithExactPopup({
        chargeAmount: Number(amount || 0),
        displayCurrency,
        displayCurrencyIndex,
      }),
    [displayCurrency, displayCurrencyIndex],
  );

  const openCardDetail = useCallback(
    (card) => {
      const kind = String(card?.kind || '').trim();
      const id = String(card?.id || '').trim();
      if (!kind || !id) {
        return;
      }
      navigate(
        `/workspace/topup/cards/${encodeURIComponent(kind)}/${encodeURIComponent(id)}`,
        { state: { from: currentPagePath } },
      );
    },
    [navigate, currentPagePath],
  );

  return (
    <div className='router-topup-quota-layout'>
      <AppSection
        title={t('topup.quota_overview.title')}
        extra={
          <>
            <AppButton
              className='router-section-button'
              color='blue'
              onClick={() => navigate('/workspace/service/pricing')}
            >
              {t('workspace_start.actions.view_pricing')}
            </AppButton>
            <AppButton
              className='router-section-button'
              onClick={() => setRedeemOpen(true)}
            >
              {t('topup.redeem.entry_button')}
            </AppButton>
            <AppButton
              className='router-section-button'
              loading={loading}
              onClick={loadQuotaPage}
            >
              {t('common.refresh')}
            </AppButton>
          </>
        }
      >
        <div className='router-quota-summary-grid'>
          <div className='router-quota-summary-item'>
            <AppStatistic
              className='router-accent-statistic router-topup-statistic'
              title={t('topup.quota_overview.total')}
              value={0}
              formatter={() => renderAmount(overview?.total_amount || 0)}
            />
          </div>
          <div className='router-quota-summary-item'>
            <AppStatistic
              className='router-topup-statistic'
              title={t('topup.quota_overview.used')}
              value={0}
              formatter={() => renderAmount(overview?.used_amount || 0)}
            />
          </div>
          <div className='router-quota-summary-item'>
            <AppStatistic
              className='router-topup-statistic'
              title={t('topup.quota_overview.remaining')}
              value={0}
              formatter={() => renderAmount(overview?.remaining_amount || 0)}
            />
          </div>
          <div className='router-quota-summary-item'>
            <AppStatistic
              className='router-topup-statistic'
              title={t('topup.quota_overview.token_count')}
              value={tokenTotal == null ? '-' : Number(tokenTotal).toLocaleString()}
            />
          </div>
          <div className='router-quota-summary-item'>
            <AppStatistic
              className='router-topup-statistic'
              title={t('topup.quota_overview.recent_requests')}
              value={
                spend == null
                  ? '-'
                  : Number(spend.period_requests || 0).toLocaleString()
              }
            />
          </div>
        </div>
        <div className='router-form-hint router-quota-summary-hint'>
          {t('topup.quota_overview.hint')}
        </div>
      </AppSection>

      <AppSection
        title={t('topup.quota_cards.active_title')}
        extra={
          <AppButton
            className='router-section-button'
            onClick={() => navigate('/workspace/topup?tab=history')}
          >
            {t('topup.quota_cards.history_button')}
          </AppButton>
        }
      >
        {cards.length > 0 ? (
          <div className='router-quota-card-grid'>
            {cards.map((card) => (
              <QuotaCardItem
                key={`${card.kind}-${card.id}`}
                card={card}
                renderAmount={renderAmount}
                onClick={openCardDetail}
                t={t}
              />
            ))}
          </div>
        ) : loading ? (
          <AppSkeleton variant='cards' count={4} />
        ) : (
          <div className='router-empty'>
            <div className='router-empty-cta'>
              <div className='router-empty-cta-text'>
                {t('topup.quota_cards.active_empty')}
              </div>
              <AppButton
                color='blue'
                onClick={() => navigate('/workspace/service/pricing')}
              >
                {t('workspace_start.actions.view_pricing')}
              </AppButton>
            </div>
          </div>
        )}
      </AppSection>

      <div className='dashboard-spend-section'>
        <div className='dashboard-spend-stack'>
          <SpendingCalendar />
        </div>
      </div>

      <RedeemCodePage
        open={redeemOpen}
        onClose={() => setRedeemOpen(false)}
        onRedeemed={() => loadQuotaPage()}
      />
    </div>
  );
};

export default QuotaPage;
