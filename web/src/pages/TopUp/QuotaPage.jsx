import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { API, showError } from '../../helpers';
import { AppButton, AppSection, AppStatistic } from '../../router-ui';
import QuotaCardItem from './QuotaCardItem';
import SpendingCalendar from './SpendingCalendar';
import {
  renderTopupIntegerAmountWithExactPopup,
  useTopUpWorkspace,
} from './shared.jsx';

const QuotaPage = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { displayCurrency, displayCurrencyIndex } = useTopUpWorkspace();
  const [overview, setOverview] = useState(null);
  const [cards, setCards] = useState([]);
  const [loading, setLoading] = useState(false);

  const loadQuotaPage = useCallback(async () => {
    setLoading(true);
    try {
      const [overviewResponse, cardsResponse] = await Promise.all([
        API.get('/api/v1/public/user/quota/overview'),
        API.get('/api/v1/public/user/quota/cards', {
          params: { scope: 'active', page: 1, page_size: 50 },
        }),
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
      );
    },
    [navigate],
  );

  return (
    <div className='router-topup-quota-layout'>
      <AppSection
        title={t('topup.quota_overview.title')}
        extra={
          <AppButton
            className='router-section-button'
            loading={loading}
            onClick={loadQuotaPage}
          >
            {t('common.refresh')}
          </AppButton>
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
        </div>
        <div className='router-form-hint router-quota-summary-hint'>
          {t('topup.quota_overview.hint')}
        </div>
      </AppSection>

      <div className='dashboard-spend-section'>
        <div className='dashboard-spend-stack'>
          <SpendingCalendar />
        </div>
      </div>

      <AppSection
        title={t('topup.quota_cards.active_title')}
        extra={
          <AppButton
            className='router-section-button'
            onClick={() => navigate('/workspace/topup/history')}
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
        ) : loading ? null : (
          <div className='router-empty'>
            {t('topup.quota_cards.active_empty')}
          </div>
        )}
      </AppSection>
    </div>
  );
};

export default QuotaPage;
