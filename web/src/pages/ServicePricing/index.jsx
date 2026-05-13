import React from 'react';
import BalanceTopUpPage from '../TopUp/BalanceTopUpPage';
import PackagePurchasePage from '../TopUp/PackagePurchasePage';
import TopUpWorkspaceProvider from '../TopUp/provider.jsx';

const ServicePricing = () => {
  return (
    <TopUpWorkspaceProvider>
      <div className='dashboard-container router-service-pricing-page'>
        <PackagePurchasePage />
        <BalanceTopUpPage showCurrentBalance={false} />
      </div>
    </TopUpWorkspaceProvider>
  );
};

export default ServicePricing;
