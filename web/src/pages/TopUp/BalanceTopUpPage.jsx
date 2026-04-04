import React, { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Form, Header, Statistic } from 'semantic-ui-react';
import { showInfo } from '../../helpers';
import { useTopUpWorkspace } from './shared.jsx';

const BalanceTopUpPage = () => {
  const { t } = useTranslation();
  const {
    externalTopupLink,
    userBalanceYYC,
    displayCurrencyIndex,
    renderDisplayAmount,
    createTopupOrder,
  } = useTopUpWorkspace();
  const [topupAmount, setTopupAmount] = useState('0');
  const [creating, setCreating] = useState(false);

  const estimatedTopupYYC = useMemo(() => {
    const amount = Number(topupAmount || 0);
    if (!Number.isFinite(amount) || amount <= 0) {
      return 0;
    }
    const cnyCurrency = displayCurrencyIndex?.CNY;
    const yycPerUnit = Number(cnyCurrency?.yyc_per_unit || 0);
    if (!Number.isFinite(yycPerUnit) || yycPerUnit <= 0) {
      return 0;
    }
    return Math.round(amount * yycPerUnit);
  }, [displayCurrencyIndex, topupAmount]);

  const handleSubmit = async () => {
    const amount = Number(topupAmount || 0);
    if (!Number.isFinite(amount) || amount <= 0) {
      showInfo(t('topup.external_topup.amount_invalid'));
      return;
    }
    setCreating(true);
    try {
      await createTopupOrder({
        business_type: 'balance_topup',
        amount,
        currency: 'CNY',
        return_url: window.location.href,
      });
    } finally {
      setCreating(false);
    }
  };

  return (
    <Card fluid className='router-soft-card router-soft-card-fill'>
      <Card.Content className='router-card-fill'>
        <Card.Header className='router-card-header'>
          <Header as='h3' className='router-section-title router-title-accent-primary'>
            <i className='credit card icon' />
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
              <Form.Input
                className='router-section-input'
                fluid
                style={{ marginTop: '1rem', textAlign: 'left' }}
                label={t('topup.external_topup.amount')}
                type='number'
                min={0}
                step='0.01'
                placeholder={t('topup.external_topup.amount_placeholder')}
                value={topupAmount}
                onChange={(event) => setTopupAmount(event.target.value || '0')}
              />
              <div className='router-text-muted' style={{ marginTop: '0.5rem' }}>
                {t('topup.external_topup.credited_yyc')}：{renderDisplayAmount(estimatedTopupYYC)}
              </div>
            </div>

            <div className='router-action-footer'>
              <Button
                className='router-section-button router-action-button-wide'
                primary
                onClick={handleSubmit}
                loading={creating}
                disabled={creating || !externalTopupLink}
              >
                {creating
                  ? t('topup.external_topup.creating')
                  : t('topup.external_topup.button')}
              </Button>
            </div>
          </div>
        </Card.Description>
      </Card.Content>
    </Card>
  );
};

export default BalanceTopUpPage;
