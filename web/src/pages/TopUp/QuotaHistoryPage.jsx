import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { API, showError } from '../../helpers';
import {
  AppButton,
  AppFilterHeader,
  AppPagination,
  AppSection,
} from '../../router-ui';
import QuotaCardItem from './QuotaCardItem';
import TopUpWorkspaceProvider from './provider.jsx';
import {
  renderTopupIntegerAmountWithExactPopup,
  useTopUpWorkspace,
} from './shared.jsx';

const PAGE_SIZE = 20;

const QuotaHistoryPageInner = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { displayCurrency, displayCurrencyIndex } = useTopUpWorkspace();
  const [cards, setCards] = useState([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);

  const loadCards = useCallback(
    async (nextPage = page) => {
      setLoading(true);
      try {
        const response = await API.get('/api/v1/public/user/quota/cards', {
          params: {
            scope: 'history',
            page: nextPage,
            page_size: PAGE_SIZE,
          },
        });
        const payload = response?.data || {};
        if (!payload.success) {
          throw new Error(
            payload.message || t('topup.quota_cards.load_failed'),
          );
        }
        setCards(
          Array.isArray(payload.data?.items) ? payload.data.items : [],
        );
        setPage(Number(payload.data?.page || nextPage) || 1);
        setTotal(Number(payload.data?.total || 0) || 0);
      } catch (error) {
        showError(error?.message || t('topup.quota_cards.load_failed'));
      } finally {
        setLoading(false);
      }
    },
    [page, t],
  );

  useEffect(() => {
    loadCards(page).then();
  }, [loadCards, page]);

  const renderAmount = useCallback(
    (amount) =>
      renderTopupIntegerAmountWithExactPopup({
        chargeAmount: Number(amount || 0),
        displayCurrency,
        displayCurrencyIndex,
      }),
    [displayCurrency, displayCurrencyIndex],
  );

  const totalPages = useMemo(
    () => Math.max(1, Math.ceil(total / PAGE_SIZE)),
    [total],
  );

  const openCardDetail = useCallback(
    (card) => {
      navigate(
        `/workspace/topup/cards/${encodeURIComponent(card.kind)}/${encodeURIComponent(card.id)}`,
      );
    },
    [navigate],
  );

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'mine', label: t('header.mine') },
          { key: 'quota', label: t('topup.mine.quota') },
          {
            key: 'history',
            label: t('topup.quota_cards.history_title'),
            active: true,
          },
        ]}
        title={t('topup.quota_cards.history_title')}
        actions={
          <AppButton onClick={() => navigate('/workspace/topup?tab=quota')}>
            {t('topup.quota_cards.back_to_quota')}
          </AppButton>
        }
      />
      <AppSection
        title={t('topup.quota_cards.history_section')}
        extra={
          <AppButton
            className='router-section-button'
            loading={loading}
            onClick={() => loadCards(page)}
          >
            {t('common.refresh')}
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
            {t('topup.quota_cards.history_empty')}
          </div>
        )}
        {totalPages > 1 ? (
          <div className='router-pagination-wrap-md'>
            <AppPagination
              activePage={page}
              totalPages={totalPages}
              onPageChange={(_, { activePage }) =>
                setPage(Number(activePage) || 1)
              }
            />
          </div>
        ) : null}
      </AppSection>
    </div>
  );
};

const QuotaHistoryPage = () => (
  <TopUpWorkspaceProvider>
    <QuotaHistoryPageInner />
  </TopUpWorkspaceProvider>
);

export default QuotaHistoryPage;
