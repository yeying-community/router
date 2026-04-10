import React from 'react';
import { useTranslation } from 'react-i18next';
import { Card } from 'semantic-ui-react';
import BalanceTopUpPage from '../TopUp/BalanceTopUpPage';
import PackagePurchasePage from '../TopUp/PackagePurchasePage';
import TopUpWorkspaceProvider from '../TopUp/provider.jsx';

const ServicePricing = () => {
  const { t } = useTranslation();

  return (
    <TopUpWorkspaceProvider>
      <div className='dashboard-container'>
        <Card fluid className='chart-card'>
          <Card.Content>
            <div
              style={{
                display: 'grid',
                gap: '1rem',
              }}
            >
              <div className='router-section-title router-title-accent-positive'>
                {t('topup.pricing.page_title')}
              </div>
              <div className='router-text-muted'>
                {t('topup.pricing.subtitle')}
              </div>
              <div
                style={{
                  display: 'grid',
                  gap: '0.85rem',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
                }}
              >
                <div
                  style={{
                    border: '1px solid #dbeafe',
                    background: '#eff6ff',
                    borderRadius: '14px',
                    padding: '1rem 1.1rem',
                  }}
                >
                  <div
                    style={{
                      fontSize: '1rem',
                      fontWeight: 600,
                      color: '#1d4ed8',
                      marginBottom: '0.4rem',
                    }}
                  >
                    {t('topup.pricing.package_mode_title')}
                  </div>
                  <div style={{ color: '#1f2937', lineHeight: 1.7 }}>
                    {t('topup.pricing.package_hint')}
                  </div>
                </div>
                <div
                  style={{
                    border: '1px solid #dcfce7',
                    background: '#f0fdf4',
                    borderRadius: '14px',
                    padding: '1rem 1.1rem',
                  }}
                >
                  <div
                    style={{
                      fontSize: '1rem',
                      fontWeight: 600,
                      color: '#15803d',
                      marginBottom: '0.4rem',
                    }}
                  >
                    {t('topup.pricing.balance_mode_title')}
                  </div>
                  <div style={{ color: '#1f2937', lineHeight: 1.7 }}>
                    {t('topup.pricing.balance_hint')}
                  </div>
                </div>
              </div>
            </div>
          </Card.Content>
        </Card>

        <PackagePurchasePage />
        <BalanceTopUpPage showCurrentBalance={false} />
      </div>
    </TopUpWorkspaceProvider>
  );
};

export default ServicePricing;
