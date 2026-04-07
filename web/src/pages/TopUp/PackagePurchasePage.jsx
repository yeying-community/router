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
  const [creatingPackageId, setCreatingPackageId] = useState('');

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

  const handlePurchase = async (packageId = '') => {
    const packageID = (packageId || selectedPackageId || '').trim();
    if (!packageID) {
      showInfo(t('topup.external_topup.package_select_required'));
      return;
    }
    setCreatingPackageId(packageID);
    try {
      await createTopupOrder({
        business_type: 'package_purchase',
        package_id: packageID,
        return_url: window.location.href,
      });
    } finally {
      setCreatingPackageId('');
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
            <div style={{ display: 'grid', gap: '0.85rem' }}>
              {loading ? (
                <div className='router-text-muted'>{t('common.loading')}</div>
              ) : packages.length === 0 ? (
                <div className='router-text-muted'>{t('topup.external_topup.package_empty')}</div>
              ) : (
                <div
                  style={{
                    display: 'flex',
                    gap: '0.85rem',
                    overflowX: 'auto',
                    paddingBottom: '0.35rem',
                    alignItems: 'stretch',
                    scrollSnapType: 'x proximity',
                  }}
                >
                  {packages.map((item) => {
                    const selected = item?.id === selectedPackageId;
                    return (
                      <div
                        key={item?.id || '-'}
                        onClick={() => setSelectedPackageId(item?.id || '')}
                        style={{
                          flex: '0 0 320px',
                          minWidth: '320px',
                          border: selected ? '1px solid #2563eb' : '1px solid #e5e7eb',
                          borderRadius: '16px',
                          padding: '1rem 1rem 0.95rem',
                          cursor: 'pointer',
                          background: selected ? '#eff6ff' : '#fff',
                          textAlign: 'left',
                          boxShadow: selected
                            ? '0 12px 30px rgba(37, 99, 235, 0.12)'
                            : '0 8px 24px rgba(15, 23, 42, 0.06)',
                          display: 'grid',
                          gap: '0.85rem',
                          scrollSnapAlign: 'start',
                        }}
                      >
                        <div
                          style={{
                            display: 'flex',
                            justifyContent: 'space-between',
                            alignItems: 'flex-start',
                            gap: '0.75rem',
                          }}
                        >
                          <div>
                            <div
                              style={{
                                fontSize: '1.05rem',
                                fontWeight: 700,
                                color: '#111827',
                              }}
                            >
                              {item?.name || '-'}
                            </div>
                            <div
                              className='router-text-muted'
                              style={{ marginTop: '0.35rem', lineHeight: 1.6 }}
                            >
                              {item?.description || '-'}
                            </div>
                          </div>
                          <div
                            style={{
                              fontSize: '0.8rem',
                              lineHeight: 1,
                              padding: '0.38rem 0.55rem',
                              borderRadius: '999px',
                              background: selected ? '#2563eb' : '#f3f4f6',
                              color: selected ? '#fff' : '#4b5563',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {selected
                              ? t('topup.pricing.selected')
                              : t('topup.pricing.select')}
                          </div>
                        </div>

                        <div
                          style={{
                            fontSize: '1.5rem',
                            fontWeight: 700,
                            color: selected ? '#1d4ed8' : '#111827',
                          }}
                        >
                          {`${item?.sale_currency || 'CNY'} ${Number(item?.sale_price ?? 0).toFixed(2)}`}
                        </div>

                        <div
                          style={{
                            display: 'grid',
                            gap: '0.6rem',
                            gridTemplateColumns: 'repeat(3, minmax(0, 1fr))',
                          }}
                        >
                          <div
                            style={{
                              borderRadius: '12px',
                              background: selected ? 'rgba(255,255,255,0.72)' : '#f8fafc',
                              padding: '0.75rem 0.8rem',
                            }}
                          >
                            <div
                              style={{
                                fontSize: '0.78rem',
                                color: '#6b7280',
                                marginBottom: '0.3rem',
                              }}
                            >
                              {t('package_manage.table.duration_days')}
                            </div>
                            <div style={{ color: '#111827', fontWeight: 600 }}>
                              {Number(item?.duration_days || 0) || '-'}
                            </div>
                          </div>
                          <div
                            style={{
                              borderRadius: '12px',
                              background: selected ? 'rgba(255,255,255,0.72)' : '#f8fafc',
                              padding: '0.75rem 0.8rem',
                            }}
                          >
                            <div
                              style={{
                                fontSize: '0.78rem',
                                color: '#6b7280',
                                marginBottom: '0.3rem',
                              }}
                            >
                              {t('user.detail.package_daily_limit')}
                            </div>
                            <div style={{ color: '#111827', fontWeight: 600 }}>
                              {renderDisplayAmount(item?.daily_quota_limit || 0)}
                            </div>
                          </div>
                          <div
                            style={{
                              borderRadius: '12px',
                              background: selected ? 'rgba(255,255,255,0.72)' : '#f8fafc',
                              padding: '0.75rem 0.8rem',
                            }}
                          >
                            <div
                              style={{
                                fontSize: '0.78rem',
                                color: '#6b7280',
                                marginBottom: '0.3rem',
                              }}
                            >
                              {t('user.detail.package_emergency_limit')}
                            </div>
                            <div style={{ color: '#111827', fontWeight: 600 }}>
                              {renderDisplayAmount(
                                item?.package_emergency_quota_limit || 0,
                              )}
                            </div>
                          </div>
                        </div>

                        <Button
                          className='router-section-button'
                          color='green'
                          fluid
                          onClick={(event) => {
                            event.stopPropagation();
                            handlePurchase(item?.id || '');
                          }}
                          loading={creatingPackageId === (item?.id || '')}
                          disabled={
                            !externalTopupLink ||
                            creatingPackageId !== ''
                          }
                        >
                          {creatingPackageId === (item?.id || '')
                            ? t('topup.external_topup.creating')
                            : t('topup.external_topup.package_button')}
                        </Button>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
        </Card.Description>
      </Card.Content>
    </Card>
  );
};

export default PackagePurchasePage;
