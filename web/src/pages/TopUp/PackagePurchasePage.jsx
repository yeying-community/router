import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Header, Modal } from 'semantic-ui-react';
import { API, showError, showInfo, timestamp2string } from '../../helpers';
import { buildTopUpReturnURL, useTopUpWorkspace } from './shared.jsx';

const formatMoney = (amount, currency) =>
  `${Number(amount || 0).toFixed(2)} ${String(currency || 'USD').toUpperCase()}`;

const formatTimeValue = (value, t) => {
  const normalized = Number(value || 0);
  if (!Number.isFinite(normalized) || normalized <= 0) {
    return t('common.unlimited');
  }
  return timestamp2string(normalized);
};

const PackagePurchasePage = () => {
  const { t, i18n } = useTranslation();
  const { renderDisplayAmount, createTopupOrder, previewPackagePurchase } =
    useTopUpWorkspace();
  const [packages, setPackages] = useState([]);
  const [loading, setLoading] = useState(false);
  const [previewingPackageId, setPreviewingPackageId] = useState('');
  const [creatingPackageId, setCreatingPackageId] = useState('');
  const [previewState, setPreviewState] = useState({
    open: false,
    packageId: '',
    preview: null,
  });

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
      } catch (error) {
        showError(error?.message || t('topup.external_topup.request_failed'));
      } finally {
        setLoading(false);
      }
    };
    loadPackages().then();
  }, [t]);

  const handlePurchase = async (packageId = '') => {
    const packageID = (packageId || '').trim();
    if (!packageID) {
      showInfo(t('topup.external_topup.package_select_required'));
      return;
    }
    setPreviewingPackageId(packageID);
    try {
      const preview = await previewPackagePurchase({
        package_id: packageID,
      });
      if (!preview) {
        return;
      }
      setPreviewState({
        open: true,
        packageId: packageID,
        preview,
      });
    } finally {
      setPreviewingPackageId('');
    }
  };

  const closePreviewModal = () => {
    if (creatingPackageId !== '') {
      return;
    }
    setPreviewState({
      open: false,
      packageId: '',
      preview: null,
    });
  };

  const handleConfirmPurchase = async () => {
    const packageID = (previewState.packageId || '').trim();
    const operationType = String(
      previewState?.preview?.operation_type || '',
    ).trim();
    if (!packageID) {
      showInfo(t('topup.external_topup.package_select_required'));
      return;
    }
    setCreatingPackageId(packageID);
    try {
      const created = await createTopupOrder({
        business_type: 'package_purchase',
        operation_type: operationType,
        package_id: packageID,
        return_url: buildTopUpReturnURL(),
      });
      if (created) {
        closePreviewModal();
      }
    } finally {
      setCreatingPackageId('');
    }
  };

  const operationType = String(previewState?.preview?.operation_type || '').trim();
  const operationKey = operationType
    ? `topup.external_topup.package_operation.${operationType}`
    : '';
  const operationLabel =
    operationKey && i18n.exists(operationKey)
      ? t(operationKey)
      : operationType || '-';

  return (
    <Card fluid className='router-soft-card router-soft-card-fill'>
      <Card.Content className='router-card-fill'>
        <Card.Header className='router-card-header'>
          <Header as='h3' className='router-section-title router-title-accent-positive'>
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
                    return (
                      <div
                        key={item?.id || '-'}
                        style={{
                          flex: '0 0 320px',
                          minWidth: '320px',
                          border: '1px solid #e5e7eb',
                          borderRadius: '16px',
                          padding: '1rem 1rem 0.95rem',
                          cursor: 'default',
                          background: '#fff',
                          textAlign: 'left',
                          boxShadow: '0 8px 24px rgba(15, 23, 42, 0.06)',
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
                        </div>

                        <div
                          style={{
                            fontSize: '1.5rem',
                            fontWeight: 700,
                            color: '#111827',
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
                              background: '#f8fafc',
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
                              background: '#f8fafc',
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
                              background: '#f8fafc',
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
                          loading={
                            previewingPackageId === (item?.id || '') ||
                            creatingPackageId === (item?.id || '')
                          }
                          disabled={
                            previewingPackageId !== '' || creatingPackageId !== ''
                          }
                        >
                          {previewingPackageId === (item?.id || '') ||
                          creatingPackageId === (item?.id || '')
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
      <Modal
        size='small'
        open={previewState.open}
        onClose={closePreviewModal}
        closeOnDimmerClick={creatingPackageId === ''}
      >
        <Modal.Header>
          {t('topup.external_topup.package_preview_title')}
        </Modal.Header>
        <Modal.Content>
          <div className='router-text-muted' style={{ marginBottom: '0.75rem' }}>
            {t('topup.external_topup.package_preview_desc')}
          </div>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '160px minmax(0, 1fr)',
              rowGap: '0.65rem',
              columnGap: '0.75rem',
              alignItems: 'center',
            }}
          >
            <div className='router-text-muted'>
              {t('topup.external_topup.package_preview_operation')}
            </div>
            <div>{operationLabel}</div>

            <div className='router-text-muted'>
              {t('topup.external_topup.package_preview_current_package')}
            </div>
            <div>{previewState?.preview?.current_package_name || '-'}</div>

            <div className='router-text-muted'>
              {t('topup.external_topup.package_preview_target_package')}
            </div>
            <div>{previewState?.preview?.target_package_name || '-'}</div>

            <div className='router-text-muted'>
              {t('topup.external_topup.package_preview_current_expire_at')}
            </div>
            <div>
              {formatTimeValue(previewState?.preview?.current_expires_at, t)}
            </div>

            <div className='router-text-muted'>
              {t('topup.external_topup.package_preview_effective_at')}
            </div>
            <div>{formatTimeValue(previewState?.preview?.start_at, t)}</div>

            <div className='router-text-muted'>
              {t('topup.external_topup.package_preview_expires_at')}
            </div>
            <div>{formatTimeValue(previewState?.preview?.expires_at, t)}</div>

            <div className='router-text-muted'>
              {t('topup.external_topup.package_preview_payable')}
            </div>
            <div>
              {formatMoney(
                previewState?.preview?.payable_amount,
                previewState?.preview?.payable_currency,
              )}
            </div>

            <div className='router-text-muted'>
              {t('topup.external_topup.package_preview_payable_yyc')}
            </div>
            <div>{renderDisplayAmount(previewState?.preview?.payable_yyc || 0)}</div>
          </div>
        </Modal.Content>
        <Modal.Actions>
          <Button onClick={closePreviewModal} disabled={creatingPackageId !== ''}>
            {t('common.cancel')}
          </Button>
          <Button
            primary
            className='router-section-button'
            loading={creatingPackageId !== ''}
            disabled={creatingPackageId !== ''}
            onClick={handleConfirmPurchase}
          >
            {t('topup.external_topup.package_confirm_button')}
          </Button>
        </Modal.Actions>
      </Modal>
    </Card>
  );
};

export default PackagePurchasePage;
