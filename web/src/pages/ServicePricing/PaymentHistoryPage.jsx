import React from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { AppFilterHeader } from '../../router-ui';
import TopUpRecordsPage from '../TopUp/TopUpRecordsPage';
import TopUpWorkspaceProvider from '../TopUp/provider.jsx';

const PaymentHistoryPageInner = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();

  return (
    <div className='dashboard-container router-payment-history-page'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'service', label: t('header.service') },
          {
            key: 'pricing',
            label: t('topup.pricing.title'),
            onClick: () => navigate('/workspace/service/pricing'),
          },
          {
            key: 'payment-history',
            label: t('topup.payment_history.title'),
            active: true,
          },
        ]}
        title={t('topup.payment_history.title')}
      />
      <TopUpRecordsPage recordKey='payment' embedded />
    </div>
  );
};

const PaymentHistoryPage = () => (
  <TopUpWorkspaceProvider>
    <PaymentHistoryPageInner />
  </TopUpWorkspaceProvider>
);

export default PaymentHistoryPage;
