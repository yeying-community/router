import React from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { Button, Card, Statistic } from 'semantic-ui-react';
import RedeemCodePage from './RedeemCodePage';
import {
  renderTopupIntegerAmountWithExactPopup,
  useTopUpWorkspace,
} from './shared.jsx';

const BalanceStatusPage = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const {
    userBalanceYYC,
    topupBalanceYYC,
    redeemBalanceYYC,
    displayCurrency,
    displayCurrencyIndex,
  } = useTopUpWorkspace();

  return (
    <div style={{ display: 'grid', gap: '1rem' }}>
      <Card fluid className='router-soft-card router-soft-card-fill'>
        <Card.Content className='router-card-fill'>
          <Card.Description className='router-card-fill'>
            <div className='router-card-body-spread'>
              <div
                style={{
                  display: 'grid',
                  gap: '1rem',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
                  alignItems: 'center',
                }}
              >
                <div className='router-center-panel' style={{ paddingTop: 0 }}>
                  <Statistic className='router-accent-statistic' size='small'>
                    <Statistic.Value className='router-topup-statistic-value'>
                      {renderTopupIntegerAmountWithExactPopup({
                        yycAmount: userBalanceYYC,
                        displayCurrency,
                        displayCurrencyIndex,
                      })}
                    </Statistic.Value>
                    <Statistic.Label>
                      {t('topup.external_topup.total_balance')}
                    </Statistic.Label>
                  </Statistic>
                </div>
                <div className='router-center-panel' style={{ paddingTop: 0 }}>
                  <Statistic size='small'>
                    <Statistic.Value className='router-topup-statistic-value'>
                      {renderTopupIntegerAmountWithExactPopup({
                        yycAmount: topupBalanceYYC,
                        displayCurrency,
                        displayCurrencyIndex,
                      })}
                    </Statistic.Value>
                    <Statistic.Label>
                      {t('topup.external_topup.topup_balance')}
                    </Statistic.Label>
                  </Statistic>
                </div>
                <div className='router-center-panel' style={{ paddingTop: 0 }}>
                  <Statistic size='small'>
                    <Statistic.Value className='router-topup-statistic-value'>
                      {renderTopupIntegerAmountWithExactPopup({
                        yycAmount: redeemBalanceYYC,
                        displayCurrency,
                        displayCurrencyIndex,
                      })}
                    </Statistic.Value>
                    <Statistic.Label>
                      {t('topup.external_topup.redeem_balance')}
                    </Statistic.Label>
                  </Statistic>
                </div>
              </div>
              <div className='router-action-footer'>
                <Button
                  primary
                  fluid
                  className='router-section-button'
                  onClick={() => navigate('/workspace/service/pricing')}
                >
                  {t('topup.record_nav.topup')}
                </Button>
              </div>
            </div>
          </Card.Description>
        </Card.Content>
      </Card>
      <RedeemCodePage />
    </div>
  );
};

export default BalanceStatusPage;
