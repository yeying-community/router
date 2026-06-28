import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showInfo, timestamp2string } from '../../helpers';
import { buildTopUpReturnURL, useTopUpWorkspace } from './shared.jsx';
import { AppButton, AppModal, AppSection, AppTag } from '../../router-ui';
import {
  formatPackageConcurrencyLimit,
  formatRequestQuotaEntitlement,
  getServicePackageTypeLabel,
  isRequestQuotaPackage,
  normalizeServicePackageType,
} from '../../helpers/package';

const formatMoney = (amount, currency) =>
  `${Number(amount || 0).toFixed(2)} ${String(currency || 'USD').toUpperCase()}`;

const formatTimeValue = (value, t) => {
  const normalized = Number(value || 0);
  if (!Number.isFinite(normalized) || normalized <= 0) {
    return t('common.unlimited');
  }
  return timestamp2string(normalized);
};

const resolvePackageOperationLabel = (operationType, t, i18n) => {
  const normalized = String(operationType || '').trim();
  const operationKey = normalized
    ? `topup.external_topup.package_operation.${normalized}`
    : '';
  if (operationKey && i18n.exists(operationKey)) {
    return t(operationKey);
  }
  return normalized || '-';
};

const renderPackageActionTag = (operationType, currentPackageID, targetPackageID, t, i18n) => {
  const normalizedOperationType = String(operationType || '').trim();
  const normalizedCurrentPackageID = String(currentPackageID || '').trim();
  const normalizedTargetPackageID = String(targetPackageID || '').trim();
  const actionKey = normalizedOperationType
    ? `topup.external_topup.package_operation.${normalizedOperationType}`
    : '';
  const actionLabel =
    actionKey && i18n.exists(actionKey)
      ? t(actionKey)
      : normalizedOperationType || '-';

  if (
    normalizedCurrentPackageID !== '' &&
    normalizedCurrentPackageID === normalizedTargetPackageID
  ) {
    return (
      <AppTag color='green' className='router-tag'>
        {t('topup.external_topup.package_current_tag')}
      </AppTag>
    );
  }

  const colorMap = {
    purchase: 'blue',
    new_purchase: 'blue',
    renew: 'green',
    upgrade: 'teal',
    downgrade: 'grey',
    convert: 'orange',
  };

  return (
    <AppTag
      color={colorMap[normalizedOperationType] || 'blue'}
      className='router-tag'
    >
      {actionLabel}
    </AppTag>
  );
};

const normalizePackageSubscription = (raw) => {
  if (!raw || typeof raw !== 'object') {
    return null;
  }
  return {
    package_id: (raw.package_id || raw.id || '').toString().trim(),
    package_type: (raw.package_type || '').toString().trim(),
    quota_metric: (raw.quota_metric || '').toString().trim(),
    sale_price: Number(raw.sale_price ?? 0) || 0,
  };
};

const resolvePackagePurchaseOperation = (currentPackage, targetPackage) => {
  if (!targetPackage || typeof targetPackage !== 'object') {
    return '';
  }
  if (!currentPackage) {
    return 'purchase';
  }
  const currentPackageID = (currentPackage.package_id || '').toString().trim();
  const targetPackageID = (targetPackage.id || '').toString().trim();
  if (currentPackageID !== '' && currentPackageID === targetPackageID) {
    return 'renew';
  }
  const currentType = normalizeServicePackageType(currentPackage.package_type, currentPackage.quota_metric);
  const targetType = normalizeServicePackageType(targetPackage.package_type, targetPackage.quota_metric);
  if (currentType !== targetType) {
    return 'convert';
  }
  const currentPrice = Number(currentPackage.sale_price ?? 0);
  const targetPrice = Number(targetPackage.sale_price ?? 0);
  if (targetPrice > currentPrice) {
    return 'upgrade';
  }
  if (targetPrice < currentPrice) {
    return 'downgrade';
  }
  return 'purchase';
};

const PackagePurchasePage = () => {
  const { t, i18n } = useTranslation();
  const { renderDisplayAmount, createTopupOrder, previewPackagePurchase } =
    useTopUpWorkspace();
  const [packages, setPackages] = useState([]);
  const [activePackage, setActivePackage] = useState(null);
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
        const [packagesRes, activePackageRes] = await Promise.all([
          API.get('/api/v1/public/user/packages'),
          API.get('/api/v1/public/user/package/subscription'),
        ]);
        const { success, message, data } = packagesRes?.data || {};
        if (!success) {
          showError(message || t('topup.external_topup.request_failed'));
          return;
        }
        const rows = Array.isArray(data) ? data : [];
        setPackages(rows);
        const activeData = activePackageRes?.data?.data || {};
        setActivePackage(
          activeData?.has_active_subscription
            ? normalizePackageSubscription(activeData.current_package || activeData.subscription)
            : null,
        );
      } catch (error) {
        showError(error?.message || t('topup.external_topup.request_failed'));
      } finally {
        setLoading(false);
      }
    };
    loadPackages().then();
  }, [t]);

  const handlePurchase = async (packageId = '', requestedOperationType = '') => {
    const packageID = (packageId || '').trim();
    if (!packageID) {
      showInfo(t('topup.external_topup.package_select_required'));
      return;
    }
    setPreviewingPackageId(packageID);
    try {
      const preview = await previewPackagePurchase({
        package_id: packageID,
        operation_type: (requestedOperationType || '').toString().trim(),
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
  const operationLabel = resolvePackageOperationLabel(operationType, t, i18n);
  const confirmLabel = t('common.confirm');

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
                  const requestQuotaPackage = isRequestQuotaPackage(item);
                  const operationType = resolvePackagePurchaseOperation(activePackage, item);
                  const itemID = String(item?.id || '').trim();
                  const activePackageID = String(activePackage?.package_id || '').trim();
                  const operationKey = operationType
                    ? `topup.external_topup.package_operation.${operationType}`
                    : '';
                  const buttonLabel =
                    operationKey && i18n.exists(operationKey)
                      ? t(operationKey)
                      : t('topup.external_topup.package_button');
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
                          {renderPackageActionTag(
                            operationType,
                            activePackageID,
                            itemID,
                            t,
                            i18n,
                          )}
                        </div>

                        <div className='router-package-purchase-price'>
                          {`${item?.sale_currency || 'CNY'} ${Number(item?.sale_price ?? 0).toFixed(2)}`}
                        </div>

                        <div className='router-package-purchase-meta-grid'>
                          <div className='router-package-purchase-meta-card'>
                            <div className='router-package-purchase-meta-label'>
                              {t('package_manage.table.package_type')}
                            </div>
                            <div className='router-package-purchase-meta-value'>
                              {getServicePackageTypeLabel(item, t)}
                            </div>
                          </div>
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
                              {requestQuotaPackage
                                ? t('package_manage.table.period_entitlement')
                                : t('user.detail.package_daily_limit')}
                            </div>
                            <div className='router-package-purchase-meta-value'>
                              {requestQuotaPackage
                                ? formatRequestQuotaEntitlement(item, t)
                                : renderDisplayAmount(item?.daily_quota_limit || 0)}
                            </div>
                          </div>
                          <div className='router-package-purchase-meta-card'>
                            <div className='router-package-purchase-meta-label'>
                              {t('package_manage.table.concurrency_limit')}
                            </div>
                            <div className='router-package-purchase-meta-value'>
                              {formatPackageConcurrencyLimit(item, t)}
                            </div>
                          </div>
                        </div>

                        <AppButton
                          className='router-section-button'
                          color='blue'
                          fluid
                          onClick={(event) => {
                            event.stopPropagation();
                            handlePurchase(item?.id || '', operationType);
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
                            : buttonLabel}
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
            {confirmLabel}
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
              {t('topup.external_topup.package_preview_target_package_type')}
            </div>
            <div>
              {getServicePackageTypeLabel(
                {
                  package_type: previewState?.preview?.target_package_type,
                  quota_metric: previewState?.preview?.target_quota_metric,
                },
                t,
              )}
            </div>

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
              {t('topup.external_topup.package_preview_target_price')}
            </div>
            <div>
              {formatMoney(
                previewState?.preview?.target_package_amount,
                previewState?.preview?.payable_currency,
              )}
            </div>

            {Number(previewState?.preview?.current_package_credit_amount || 0) > 0 ? (
              <>
                <div className='router-text-muted'>
                  {t('topup.external_topup.package_preview_current_package_credit')}
                </div>
                <div>
                  {formatMoney(
                    previewState?.preview?.current_package_credit_amount,
                    previewState?.preview?.payable_currency,
                  )}
                </div>
              </>
            ) : null}

            <div className='router-text-muted'>
              {t('topup.external_topup.package_preview_payable')}
            </div>
            <div>
              {formatMoney(
                previewState?.preview?.payable_amount,
                previewState?.preview?.payable_currency,
              )}
            </div>
          </div>
      </AppModal>
    </>
  );
};

export default PackagePurchasePage;
