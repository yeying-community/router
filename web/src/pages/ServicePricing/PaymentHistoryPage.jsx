import React, { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { AppButton, AppFilterHeader, AppSection, AppTabs } from '../../router-ui';
import TopUpRecordsPage from '../TopUp/TopUpRecordsPage';
import TopUpWorkspaceProvider from '../TopUp/provider.jsx';

const PaymentHistoryPageInner = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const type = searchParams.get('type') === 'package' ? 'package' : 'topup';

  const items = useMemo(
    () => [
      {
        key: 'topup',
        label: t('topup.record_nav.topup'),
        children:
          type === 'topup' ? (
            <TopUpRecordsPage recordKey='topup' embedded />
          ) : null,
      },
      {
        key: 'package',
        label: t('topup.record_nav.package'),
        children:
          type === 'package' ? (
            <TopUpRecordsPage recordKey='package' embedded />
          ) : null,
      },
    ],
    [t, type],
  );

  return (
    <div className='dashboard-container'>
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
        actions={
          <AppButton onClick={() => navigate('/workspace/service/pricing')}>
            {t('topup.payment_history.back_to_pricing')}
          </AppButton>
        }
      />
      <AppSection title={t('topup.payment_history.status_title')}>
        <AppTabs
          activeKey={type}
          onChange={(nextType) =>
            navigate(
              `/workspace/service/pricing/history?type=${encodeURIComponent(nextType)}`,
            )
          }
          items={items}
        />
      </AppSection>
    </div>
  );
};

const PaymentHistoryPage = () => (
  <TopUpWorkspaceProvider>
    <PaymentHistoryPageInner />
  </TopUpWorkspaceProvider>
);

export default PaymentHistoryPage;
