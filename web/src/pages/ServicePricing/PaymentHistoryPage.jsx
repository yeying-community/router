import React from 'react';
import { useTranslation } from 'react-i18next';
import { AppFilterHeader } from '../../router-ui';
import TopUpRecordsPage from '../TopUp/TopUpRecordsPage';
import TopUpWorkspaceProvider from '../TopUp/provider.jsx';

const PaymentHistoryPageInner = () => {
  const { t } = useTranslation();

  return (
    <div className='dashboard-container router-payment-history-page'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'service', label: t('header.service') },
          { key: 'pricing', label: t('topup.pricing.title') },
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
