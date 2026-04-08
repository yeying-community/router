import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Header, Statistic } from 'semantic-ui-react';
import { useTopUpWorkspace } from './shared.jsx';

const renderPlanAmount = (amount, currency) =>
  `${Number(amount || 0)} ${String(currency || 'CNY').toUpperCase()}`;

const renderPlanQuota = (amount, currency) =>
  `${Number(amount || 0)} ${String(currency || 'USD').toUpperCase()}`;

const BalanceTopUpPage = () => {
  const { t } = useTranslation();
  const {
    userBalanceYYC,
    topupPlans,
    renderDisplayAmount,
    createTopupOrder,
  } = useTopUpWorkspace();
  const [creatingPlanID, setCreatingPlanID] = useState('');

  const handleSubmit = async (planID) => {
    setCreatingPlanID(planID);
    try {
      await createTopupOrder({
        business_type: 'balance_topup',
        plan_id: planID,
        return_url: window.location.href,
      });
    } finally {
      setCreatingPlanID('');
    }
  };

  return (
    <Card fluid className='router-soft-card router-soft-card-fill'>
      <Card.Content className='router-card-fill'>
        <Card.Header className='router-card-header'>
          <Header as='h3' className='router-section-title router-title-accent-primary'>
            {t('topup.external_topup.title')}
          </Header>
        </Card.Header>
        <Card.Description className='router-card-fill'>
          <div className='router-card-body-spread'>
            <div className='router-center-panel'>
              <Statistic className='router-accent-statistic'>
                <Statistic.Value>{renderDisplayAmount(userBalanceYYC)}</Statistic.Value>
                <Statistic.Label>{t('topup.external_topup.current_balance')}</Statistic.Label>
              </Statistic>
              <div className='router-text-muted' style={{ marginTop: '0.75rem' }}>
                {t('topup.external_topup.description')}
              </div>
              <div className='router-text-muted' style={{ marginTop: '0.5rem' }}>
                {t('topup.external_topup.plan_hint')}
              </div>
            </div>

            <div className='router-grid-top-md' style={{ width: '100%' }}>
              <Card.Group itemsPerRow={5} stackable>
                {(Array.isArray(topupPlans) ? topupPlans : []).map((plan) => (
                  <Card key={plan.id} className='router-soft-card'>
                    <Card.Content>
                      <Card.Header>{plan.name || renderPlanAmount(plan.amount, plan.amount_currency)}</Card.Header>
                      <Card.Meta style={{ marginTop: '0.5rem' }}>
                        {t('topup.external_topup.pay_label', {
                          amount: renderPlanAmount(plan.amount, plan.amount_currency),
                        })}
                      </Card.Meta>
                      <Card.Description style={{ marginTop: '0.75rem' }}>
                        <div style={{ fontSize: '1.25rem', fontWeight: 600 }}>
                          {renderPlanQuota(plan.quota_amount, plan.quota_currency)}
                        </div>
                        <div className='router-text-muted' style={{ marginTop: '0.35rem' }}>
                          {t('topup.external_topup.credited_label')}
                        </div>
                      </Card.Description>
                    </Card.Content>
                    <Card.Content extra>
                      <Button
                        fluid
                        primary
                        className='router-section-button'
                        disabled={creatingPlanID !== ''}
                        loading={creatingPlanID === plan.id}
                        onClick={() => handleSubmit(plan.id)}
                      >
                        {creatingPlanID === plan.id
                          ? t('topup.external_topup.creating')
                          : t('topup.external_topup.button')}
                      </Button>
                    </Card.Content>
                  </Card>
                ))}
              </Card.Group>
            </div>
          </div>
        </Card.Description>
      </Card.Content>
    </Card>
  );
};

export default BalanceTopUpPage;
