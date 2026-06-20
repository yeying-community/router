import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showInfo, timestamp2string } from '../../helpers';
import { buildTopUpReturnURL, useTopUpWorkspace } from './shared.jsx';
import { AppButton, AppModal, AppSection } from '../../router-ui';

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
    <>
      <AppSection
        className='router-section-fill'
      title={
        <div className='router-title-accent-positive'>
          {t('topup.external_topup.package_title')}
        </div>
      }
    >
        <div className='router-section-stack-spread'>
          <div className='router-pricing-section-hint router-pricing-section-hint-package'>
            {t('topup.pricing.package_hint')}
          </div>
          <div className='router-package-purchase-content'>
            {loading ? (
              <div className='router-text-muted'>{t('common.loading')}</div>
            ) : packages.length === 0 ? (
              <div className='router-text-muted'>{t('topup.external_topup.package_empty')}</div>
            ) : (
              <div className='router-package-purchase-list'>
                {packages.map((item) => {
                  return (
                    <div key={item?.id || '-'} className='router-package-purchase-card'>
                        <div className='router-package-purchase-card-header'>
                          <div>
                            <div className='router-package-purchase-card-title'>{item?.name || '-'}</div>
                            <div
                              className='router-text-muted router-package-purchase-description'
                            >
                              {item?.description || '-'}
                            </div>
                          </div>
                        </div>

                        <div className='router-package-purchase-price'>
                          {`${item?.sale_currency || 'CNY'} ${Number(item?.sale_price ?? 0).toFixed(2)}`}
                        </div>

                        <div className='router-package-purchase-meta-grid'>
                          <div className='router-package-purchase-meta-card'>
                            <div className='router-package-purchase-meta-label'>
                              {t('package_manage.table.duration_days')}
                            </div>
                            <div className='router-package-purchase-meta-value'>
                              {Number(item?.duration_days || 0) || '-'}
                            </div>
                          </div>
                          <div className='router-package-purchase-meta-card'>
                            <div className='router-package-purchase-meta-label'>
                              {t('user.detail.package_daily_limit')}
                            </div>
                            <div className='router-package-purchase-meta-value'>
                              {renderDisplayAmount(item?.daily_quota_limit || 0)}
                            </div>
                          </div>
                          <div className='router-package-purchase-meta-card'>
                            <div className='router-package-purchase-meta-label'>
                              {t('user.detail.package_emergency_limit')}
                            </div>
                            <div className='router-package-purchase-meta-value'>
                              {renderDisplayAmount(
                                item?.package_emergency_quota_limit || 0,
                              )}
                            </div>
                          </div>
                        </div>

                        <AppButton
                          className='router-section-button'
                          color='blue'
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
                        </AppButton>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </AppSection>
      <AppModal
        size='small'
        open={previewState.open}
        onClose={closePreviewModal}
        closeOnDimmerClick={creatingPackageId === ''}
        title={t('topup.external_topup.package_preview_title')}
        footer={[
          <AppButton
            key='cancel'
            onClick={closePreviewModal}
            disabled={creatingPackageId !== ''}
          >
            {t('common.cancel')}
          </AppButton>,
          <AppButton
            key='confirm'
            color='blue'
            className='router-section-button'
            loading={creatingPackageId !== ''}
            disabled={creatingPackageId !== ''}
            onClick={handleConfirmPurchase}
          >
            {t('topup.external_topup.package_confirm_button')}
          </AppButton>,
        ]}
      >
          <div className='router-text-muted router-package-preview-desc'>
            {t('topup.external_topup.package_preview_desc')}
          </div>
          <div className='router-package-preview-grid'>
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
              {t('topup.external_topup.package_preview_payable_charge_amount')}
            </div>
            <div>{renderDisplayAmount(previewState?.preview?.payable_charge_amount || 0)}</div>
          </div>
      </AppModal>
    </>
  );
};

export default PackagePurchasePage;
