import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Header } from 'semantic-ui-react';
import { API, showError, showInfo } from '../../helpers';
import { useTopUpWorkspace } from './shared.jsx';

const PackagePurchasePage = () => {
  const { t } = useTranslation();
  const { externalTopupLink, renderDisplayAmount, createTopupOrder } = useTopUpWorkspace();
  const [packages, setPackages] = useState([]);
  const [selectedPackageId, setSelectedPackageId] = useState('');
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    const loadPackages = async () => {
      setLoading(true);
      try {
        const res = await API.get('/api/v1/public/user/packages');
        const { success, message, data } = res?.data || {};
        if (!success) {
          showError(message || t('topup.external_topup.request_failed'));
          return;
        }
        const rows = Array.isArray(data) ? data : [];
        setPackages(rows);
        setSelectedPackageId((current) => {
          if (current && rows.some((item) => item?.id === current)) {
            return current;
          }
          return rows[0]?.id || '';
        });
      } catch (error) {
        showError(error?.message || t('topup.external_topup.request_failed'));
      } finally {
        setLoading(false);
      }
    };
    loadPackages().then();
  }, [t]);

  const handlePurchase = async () => {
    const packageID = (selectedPackageId || '').trim();
    if (!packageID) {
      showInfo(t('topup.external_topup.package_select_required'));
      return;
    }
    setCreating(true);
    try {
      await createTopupOrder({
        business_type: 'package_purchase',
        package_id: packageID,
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
          <Header as='h3' className='router-section-title router-title-accent-positive'>
            <i className='boxes icon' />
            {t('topup.external_topup.package_title')}
          </Header>
        </Card.Header>
        <Card.Description className='router-card-fill'>
          <div className='router-card-body-spread'>
            <div style={{ display: 'grid', gap: '0.75rem' }}>
              {loading ? (
                <div className='router-text-muted'>{t('common.loading')}</div>
              ) : packages.length === 0 ? (
                <div className='router-text-muted'>{t('topup.external_topup.package_empty')}</div>
              ) : (
                packages.map((item) => {
                  const selected = item?.id === selectedPackageId;
                  return (
                    <div
                      key={item?.id || '-'}
                      onClick={() => setSelectedPackageId(item?.id || '')}
                      style={{
                        border: selected ? '1px solid #2563eb' : '1px solid #e5e7eb',
                        borderRadius: '12px',
                        padding: '12px 14px',
                        cursor: 'pointer',
                        background: selected ? '#eff6ff' : '#fff',
                        textAlign: 'left',
                      }}
                    >
                      <div style={{ fontWeight: 600 }}>{item?.name || '-'}</div>
                      <div className='router-text-muted' style={{ marginTop: '0.35rem' }}>
                        {item?.description || '-'}
                      </div>
                      <div style={{ marginTop: '0.5rem', display: 'flex', gap: '1rem', flexWrap: 'wrap' }}>
                        <span>{`${item?.sale_currency || 'CNY'} ${Number(item?.sale_price ?? 0).toFixed(2)}`}</span>
                        <span className='router-text-muted'>
                          {t('user.detail.package_daily_limit')} {renderDisplayAmount(item?.daily_quota_limit || 0)}
                        </span>
                        <span className='router-text-muted'>
                          {t('user.detail.package_monthly_emergency_limit')} {renderDisplayAmount(item?.package_emergency_quota_limit || 0)}
                        </span>
                      </div>
                    </div>
                  );
                })
              )}
            </div>

            <div className='router-action-footer'>
              <Button
                className='router-section-button'
                color='green'
                fluid
                onClick={handlePurchase}
                loading={creating}
                disabled={creating || !externalTopupLink || packages.length === 0}
              >
                {creating
                  ? t('topup.external_topup.creating')
                  : t('topup.external_topup.package_button')}
              </Button>
            </div>
          </div>
        </Card.Description>
      </Card.Content>
    </Card>
  );
};

export default PackagePurchasePage;
