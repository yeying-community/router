import React from 'react';
import BalanceStatusPage from './BalanceStatusPage';
import CurrentPackagePage from './CurrentPackagePage';

const QuotaPage = () => {
  return (
    <div className='router-topup-quota-layout'>
      <CurrentPackagePage />
      <BalanceStatusPage />
    </div>
  );
};

export default QuotaPage;
