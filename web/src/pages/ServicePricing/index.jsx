import React from 'react';
import { useTranslation } from 'react-i18next';
import BalanceTopUpPage from '../TopUp/BalanceTopUpPage';
import PackagePurchasePage from '../TopUp/PackagePurchasePage';
import TopUpWorkspaceProvider from '../TopUp/provider.jsx';
import { AppFilterHeader } from '../../router-ui';

const ServicePricing = () => {
  const { t } = useTranslation();

  return (
    <TopUpWorkspaceProvider>
      <div className='dashboard-container router-service-pricing-page'>
        <AppFilterHeader
          breadcrumbs={[
            { key: 'workspace', label: t('header.user_workspace') },
            { key: 'service', label: t('header.service') },
            { key: 'pricing', label: t('topup.pricing.title'), active: true },
          ]}
          title={t('topup.pricing.page_title')}
        />
        <div id='pricing-package-section'>
          <PackagePurchasePage />
        </div>
        <div id='pricing-balance-section'>
          <BalanceTopUpPage showCurrentBalance={false} />
        </div>
      </div>
    </TopUpWorkspaceProvider>
  );
};

export default ServicePricing;
