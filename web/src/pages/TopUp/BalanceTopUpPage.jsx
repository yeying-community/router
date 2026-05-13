import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { showError } from '../../helpers';
import { buildTopUpReturnURL, useTopUpWorkspace } from './shared.jsx';
import { AppButton, AppSection } from '../../router-ui';

const renderPlanAmount = (amount, currency) =>
  `${Number(amount || 0)} ${String(currency || 'CNY').toUpperCase()}`;

const renderPlanQuota = (amount, currency) =>
  `${Number(amount || 0)} ${String(currency || 'USD').toUpperCase()}`;

const resolvePlanID = (plan) =>
  String(plan?.id || plan?.plan_id || plan?.Id || '')
    .trim();

const renderPlanValidity = (validityDays, t) => {
  const days = Number(validityDays || 0);
  if (Number.isFinite(days) && days > 0) {
    return `${days} ${t('common.day')}`;
  }
  return t('common.never');
};

const BalanceTopUpPage = () => {
  const { t } = useTranslation();
  const { topupPlans, createTopupOrder } = useTopUpWorkspace();
  const [creatingPlanID, setCreatingPlanID] = useState('');

  const handleSubmit = async (plan) => {
    const planID = resolvePlanID(plan);
    if (!planID) {
      showError(t('topup.external_topup.plan_invalid'));
      return;
    }
    setCreatingPlanID(planID);
    try {
      await createTopupOrder({
        business_type: 'balance_topup',
        plan_id: planID,
        return_url: buildTopUpReturnURL(),
      });
    } finally {
      setCreatingPlanID('');
    }
  };

  return (
    <AppSection
      className='router-section-fill'
      title={
        <div className='router-title-accent-primary'>
          {t('topup.external_topup.title')}
        </div>
      }
    >
      <div className='router-section-stack-spread'>
        <div className='router-pricing-section-hint router-pricing-section-hint-balance'>
          {t('topup.pricing.balance_hint')}
        </div>
        <div className='router-grid-top-md router-balance-topup-panel'>
          <div className='router-balance-topup-grid'>
            {(Array.isArray(topupPlans) ? topupPlans : []).map((plan, index) => {
              const planID = resolvePlanID(plan);
              return (
              <article
                key={planID || plan?.name || `plan-${index}`}
                className='router-balance-topup-card'
              >
                    <div>
                      <div className='router-balance-topup-card-title'>
                        {plan.name || renderPlanAmount(plan.amount, plan.amount_currency)}
                      </div>
                      <div className='router-balance-topup-payline'>
                        {t('topup.external_topup.pay_label', {
                          amount: renderPlanAmount(plan.amount, plan.amount_currency),
                        })}
                      </div>
                      <div className='router-balance-topup-summary'>
                        <div className='router-balance-topup-quota'>
                          {renderPlanQuota(plan.quota_amount, plan.quota_currency)}
                        </div>
                        <div className='router-text-muted router-balance-topup-muted-line'>
                          {t('topup.external_topup.credited_label')}
                        </div>
                        <div className='router-text-muted router-balance-topup-muted-line'>
                          {t('topup.external_topup.validity_label')}：{renderPlanValidity(plan.validity_days, t)}
                        </div>
                      </div>
                    </div>
                    <div className='router-balance-topup-card-footer'>
                      <AppButton
                        fluid
                        color='blue'
                        className='router-section-button'
                        disabled={creatingPlanID !== ''}
                        loading={creatingPlanID === planID}
                        onClick={() => handleSubmit(plan)}
                      >
                        {creatingPlanID === planID
                          ? t('topup.external_topup.creating')
                          : t('topup.external_topup.button')}
                      </AppButton>
                    </div>
                  </article>
              )})}
          </div>
        </div>
      </div>
    </AppSection>
  );
};

export default BalanceTopUpPage;
