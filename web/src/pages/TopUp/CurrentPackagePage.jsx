import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { API, showError, showInfo, timestamp2string } from '../../helpers';
import {
  AppButton,
  AppModal,
  AppSection,
  AppTag,
} from '../../router-ui';
import {
  buildTopUpReturnURL,
  renderTopupIntegerAmountWithExactPopup,
  SupportedModelsSummary,
  useTopUpWorkspace,
} from './shared.jsx';
import {
  formatUserFacingPackageConcurrency,
  formatRequestCount,
  getServicePackagePeriodLabel,
  getServicePackageTypeLabel,
  isRequestQuotaPackage,
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

const createEmptyActivePackage = () => ({
  has_active_packages: false,
  active_packages: [],
});

const PACKAGE_ORDER_PENDING_STATUSES = new Set(['created', 'pending', 'paid']);
const PACKAGE_ORDER_FINAL_STATUSES = new Set(['fulfilled', 'failed', 'canceled']);
const PACKAGE_ORDER_POLL_INTERVAL_MS = 3000;
const PACKAGE_ORDER_POLL_ATTEMPTS = 10;

const sleep = (durationMs) =>
  new Promise((resolve) => {
    window.setTimeout(resolve, durationMs);
  });

const normalizeTopupOrderStatus = (value) =>
  String(value || '').trim().toLowerCase();

const normalizePackageView = (raw) => {
  if (!raw) {
    return null;
  }
  const supportedModels = Array.isArray(raw.supported_models)
    ? raw.supported_models
        .map((item) => (item || '').toString().trim())
        .filter((item) => item !== '')
    : [];
  return {
    id: (raw.id || '').toString().trim(),
    package_id: (raw.package_id || '').toString().trim(),
    package_name: (raw.package_name || '').toString().trim(),
    group_id: (raw.group_id || '').toString().trim(),
    group_name: (raw.group_name || '').toString().trim(),
    source: (raw.source || '').toString().trim(),
    status: Number(raw.status || 0),
    package_type: (raw.package_type || '').toString().trim(),
    quota_metric: (raw.quota_metric || '').toString().trim(),
    period_type: (raw.period_type || '').toString().trim(),
    period_limit: Number(raw.period_limit || 0),
    max_concurrency_per_user: Number(raw.max_concurrency_per_user || 0),
    max_concurrency_per_package: Number(raw.max_concurrency_per_package || 0),
    allow_balance_fallback: raw.allow_balance_fallback === true,
    daily_quota_limit: Number(raw.daily_quota_limit || 0),
    package_emergency_quota_limit: Number(
      raw.package_emergency_quota_limit || 0,
    ),
    usage: raw.usage || null,
    daily_usage: raw.daily_usage || null,
    emergency_usage: raw.emergency_usage || null,
    quota_reset_timezone: (raw.quota_reset_timezone || '').toString().trim(),
    started_at: Number(raw.started_at || 0),
    expires_at: Number(raw.expires_at || 0),
    supported_models: supportedModels,
  };
};

const normalizeActivePackage = (raw) => {
  const activePackages = Array.isArray(raw?.active_packages)
    ? raw.active_packages.map(normalizePackageView).filter(Boolean)
    : [];
  return {
    has_active_packages: activePackages.length > 0,
    active_packages: activePackages,
  };
};

const renderPackageStatus = (status, t) => {
  switch (Number(status || 0)) {
    case 1:
      return (
        <AppTag color='green' className='router-tag'>
          {t('user.detail.package_status_types.active')}
        </AppTag>
      );
    case 2:
      return (
        <AppTag color='grey' className='router-tag'>
          {t('user.detail.package_status_types.expired')}
        </AppTag>
      );
    case 3:
      return (
        <AppTag color='grey' className='router-tag'>
          {t('user.detail.package_status_types.replaced')}
        </AppTag>
      );
    case 4:
      return (
        <AppTag color='red' className='router-tag'>
          {t('user.detail.package_status_types.canceled')}
        </AppTag>
      );
    default:
      return (
        <AppTag className='router-tag'>
          {t('user.detail.package_status_types.unknown')}
        </AppTag>
      );
  }
};

const formatPackageDateValue = (value, t) => {
  const normalized = Number(value || 0);
  if (!Number.isFinite(normalized) || normalized <= 0) {
    return t('common.unlimited');
  }
  return timestamp2string(normalized);
};

const PackageSummaryCard = ({
  item,
  renewingPackageId = '',
  onRenew,
  renderIntegerAmount,
  t,
}) => {
  const requestQuotaPackage = isRequestQuotaPackage(item);
  const requestUsage = item?.usage || null;
  const dailyUsage = item?.daily_usage || null;
  const emergencyUsage = item?.emergency_usage || null;
  const packageID = String(item?.package_id || '').trim();
  const entitlementValue = requestQuotaPackage
    ? `${formatRequestCount(item?.period_limit || 0)} ${t(
      'package_manage.request_unit',
    )} / ${getServicePackagePeriodLabel(item?.period_type, t)}`
    : renderIntegerAmount(item?.daily_quota_limit || 0);
  const infoItems = [
    {
      key: 'type',
      label: t('package_manage.table.package_type'),
      value: getServicePackageTypeLabel(item, t),
    },
    {
      key: 'entitlement',
      label: requestQuotaPackage
        ? t('package_manage.table.period_entitlement')
        : t('user.detail.package_daily_limit'),
      value: entitlementValue,
    },
    {
      key: 'concurrency',
      label: t('package_manage.table.concurrency_limit'),
      value: formatUserFacingPackageConcurrency(item, t, t('common.unlimited')),
    },
    {
      key: 'timezone',
      label: t('user.detail.package_timezone'),
      value: item?.quota_reset_timezone || '-',
    },
    {
      key: 'started_at',
      label: t('user.detail.package_started_at'),
      value: item?.started_at ? timestamp2string(item.started_at) : '-',
    },
    {
      key: 'expires_at',
      label: t('user.detail.package_expires_at'),
      value: formatPackageDateValue(item?.expires_at, t),
    },
    {
      key: 'supported_models',
      value: (
        <SupportedModelsSummary
          models={item?.supported_models}
          t={t}
          label={t('user.detail.package_supported_models')}
        />
      ),
      fullWidth: true,
    },
  ];
  if (!requestQuotaPackage) {
    infoItems.splice(3, 0, {
      key: 'emergency',
      label: t('user.detail.package_emergency_limit'),
      value: renderIntegerAmount(item?.package_emergency_quota_limit || 0),
    });
  }
  const usageItems = requestQuotaPackage && requestUsage
    ? [
      {
        key: 'period',
        label: t('topup.package_status.period'),
        value: requestUsage.period_key || '-',
      },
      {
        key: 'used',
        label: t('user.detail.used_amount'),
        value: formatRequestCount(requestUsage.consumed_amount || 0),
      },
      {
        key: 'remaining',
        label: t('user.detail.remaining_amount'),
        value: requestUsage.unlimited
          ? t('common.unlimited')
          : formatRequestCount(requestUsage.remaining_amount || 0),
      },
    ]
    : [
      dailyUsage
        ? {
          key: 'daily_used',
          label: t('user.detail.package_daily_used'),
          value: renderIntegerAmount(dailyUsage.consumed_amount || 0),
        }
        : null,
      dailyUsage
        ? {
          key: 'daily_remaining',
          label: t('user.detail.package_daily_remaining'),
          value: dailyUsage.unlimited
            ? t('common.unlimited')
            : renderIntegerAmount(dailyUsage.remaining_amount || 0),
        }
        : null,
      emergencyUsage
        ? {
          key: 'emergency_used',
          label: t('user.detail.package_emergency_used'),
          value: renderIntegerAmount(emergencyUsage.consumed_amount || 0),
        }
        : null,
      emergencyUsage
        ? {
          key: 'emergency_remaining',
          label: t('user.detail.package_emergency_remaining'),
          value: emergencyUsage.unlimited
            ? t('common.unlimited')
            : renderIntegerAmount(emergencyUsage.remaining_amount || 0),
        }
        : null,
    ].filter(Boolean);

  return (
    <div className='router-package-purchase-card'>
      <div className='router-package-purchase-card-header'>
        <div>
          <div className='router-package-purchase-card-title'>
            {item?.package_name || packageID || '-'}
          </div>
        </div>
        <div className='router-inline-actions'>
          {renderPackageStatus(item?.status, t)}
          <AppButton
            className='router-section-button'
            basic
            loading={renewingPackageId === packageID}
            disabled={renewingPackageId !== ''}
            onClick={() => onRenew(item)}
          >
            {t('topup.external_topup.package_operation.renew')}
          </AppButton>
        </div>
      </div>
      <div className='router-current-package-info-grid'>
        {infoItems.map((infoItem) => (
          <div
            key={infoItem.key}
            className={[
              'router-current-package-info-card',
              infoItem.fullWidth ? 'router-current-package-info-card-wide' : '',
            ]
              .filter(Boolean)
              .join(' ')}
          >
            {infoItem.label ? (
              <div className='router-current-package-info-label'>
                {infoItem.label}
              </div>
            ) : null}
            <div className='router-current-package-info-value'>
              {infoItem.value}
            </div>
          </div>
        ))}
      </div>
      {usageItems.length > 0 ? (
        <div className='router-current-package-info-grid'>
          {usageItems.map((usageItem) => (
            <div key={usageItem.key} className='router-current-package-info-card'>
              <div className='router-current-package-info-label'>
                {usageItem.label}
              </div>
              <div className='router-current-package-info-value'>
                {usageItem.value}
              </div>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
};

const CurrentPackagePage = () => {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const {
    displayCurrency,
    displayCurrencyIndex,
    previewPackagePurchase,
    createTopupOrder,
  } = useTopUpWorkspace();
  const [loading, setLoading] = useState(false);
  const [activePackage, setActivePackage] = useState(createEmptyActivePackage());
  const [renewingPackageId, setRenewingPackageId] = useState('');
  const [submittingPackagePurchase, setSubmittingPackagePurchase] = useState(false);
  const [packagePreviewState, setPackagePreviewState] = useState({
    open: false,
    packageId: '',
    preview: null,
  });
  const mountedRef = useRef(false);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const renderIntegerAmount = useCallback(
    (chargeAmount) =>
      renderTopupIntegerAmountWithExactPopup({
        chargeAmount,
        displayCurrency,
        displayCurrencyIndex,
      }),
    [displayCurrency, displayCurrencyIndex],
  );

  const refreshPackageOrderStatus = useCallback(async (orderID) => {
    const normalizedOrderID = String(orderID || '').trim();
    if (normalizedOrderID === '') {
      return null;
    }
    try {
      const res = await API.post(
        `/api/v1/public/user/topup/orders/${encodeURIComponent(normalizedOrderID)}/refresh`,
      );
      const { success, data } = res?.data || {};
      if (!success) {
        return null;
      }
      return data || null;
    } catch (error) {
      return null;
    }
  }, []);

  const pollPackageOrderUntilFinal = useCallback(
    async (orderID, { attempts = PACKAGE_ORDER_POLL_ATTEMPTS } = {}) => {
      const normalizedOrderID = String(orderID || '').trim();
      if (normalizedOrderID === '') {
        return null;
      }
      let latestOrder = null;
      for (let index = 0; index < attempts; index += 1) {
        if (!mountedRef.current) {
          return latestOrder;
        }
        if (index > 0) {
          await sleep(PACKAGE_ORDER_POLL_INTERVAL_MS);
        }
        latestOrder = await refreshPackageOrderStatus(normalizedOrderID);
        const status = normalizeTopupOrderStatus(latestOrder?.status);
        if (PACKAGE_ORDER_FINAL_STATUSES.has(status)) {
          return latestOrder;
        }
      }
      return latestOrder;
    },
    [refreshPackageOrderStatus],
  );

  const reconcilePendingPackageOrders = useCallback(async () => {
    try {
      const res = await API.get('/api/v1/public/user/topup/orders', {
        params: {
          page: 1,
          page_size: 20,
          business_type: 'package_purchase',
        },
      });
      const { success, data } = res?.data || {};
      if (!success) {
        return false;
      }
      const orders = Array.isArray(data?.items) ? data.items : [];
      const pendingOrders = orders.filter((order) =>
        PACKAGE_ORDER_PENDING_STATUSES.has(
          normalizeTopupOrderStatus(order?.status),
        ),
      );
      if (pendingOrders.length === 0) {
        return false;
      }
      const refreshedOrders = await Promise.all(
        pendingOrders.map((order) => refreshPackageOrderStatus(order?.id)),
      );
      const unresolvedOrders = refreshedOrders
        .map((order, index) => order || pendingOrders[index])
        .filter((order) =>
          PACKAGE_ORDER_PENDING_STATUSES.has(
            normalizeTopupOrderStatus(order?.status),
          ),
        );
      await Promise.all(
        unresolvedOrders.map((order) =>
          pollPackageOrderUntilFinal(order?.id, { attempts: 3 }),
        ),
      );
      return true;
    } catch (error) {
      return false;
    }
  }, [pollPackageOrderUntilFinal, refreshPackageOrderStatus]);

  const loadPackageStatus = useCallback(async ({ reconcileOrders = false } = {}) => {
    if (mountedRef.current) {
      setLoading(true);
    }
    try {
      if (reconcileOrders) {
        await reconcilePendingPackageOrders();
      }
      const res = await API.get('/api/v1/public/user/package/subscription');
      const { success, message, data } = res?.data || {};
      if (!success) {
        throw new Error(message || t('user.messages.active_package_load_failed'));
      }
      const normalizedPackage = normalizeActivePackage(data);
      if (mountedRef.current) {
        setActivePackage(normalizedPackage);
      }
    } catch (error) {
      if (mountedRef.current) {
        showError(error?.message || t('user.messages.active_package_load_failed'));
      }
    } finally {
      if (mountedRef.current) {
        setLoading(false);
      }
    }
  }, [reconcilePendingPackageOrders, t]);

  useEffect(() => {
    loadPackageStatus({ reconcileOrders: true }).then();
  }, [loadPackageStatus]);

  const activePackages = activePackage.active_packages || [];

  const goPricing = useCallback(
    () => {
      navigate('/workspace/service/pricing');
    },
    [navigate],
  );

  const openPackagePurchasePreview = useCallback(
    async (packageID, requestedOperationType = '') => {
      const normalizedPackageID = (packageID || '').toString().trim();
      if (normalizedPackageID === '') {
        showInfo(t('topup.external_topup.package_select_required'));
        return false;
      }
      const preview = await previewPackagePurchase({
        package_id: normalizedPackageID,
        operation_type: (requestedOperationType || '').toString().trim(),
      });
      if (!preview) {
        return false;
      }
      setPackagePreviewState({
        open: true,
        packageId: normalizedPackageID,
        preview,
      });
      return true;
    },
    [previewPackagePurchase, t],
  );

  const closePackagePreviewModal = useCallback(() => {
    if (submittingPackagePurchase) {
      return;
    }
    setPackagePreviewState({
      open: false,
      packageId: '',
      preview: null,
    });
  }, [submittingPackagePurchase]);

  const handleConfirmPackagePurchase = useCallback(async () => {
    const packageID = (packagePreviewState.packageId || '').trim();
    const operationType = String(
      packagePreviewState?.preview?.operation_type || '',
    ).trim();
    if (packageID === '') {
      showInfo(t('topup.external_topup.package_select_required'));
      return;
    }
    setSubmittingPackagePurchase(true);
    try {
      const created = await createTopupOrder({
        business_type: 'package_purchase',
        operation_type: operationType,
        package_id: packageID,
        return_url: buildTopUpReturnURL(),
      });
      if (created) {
        closePackagePreviewModal();
        const orderID = String(created?.id || '').trim();
        const orderStatus = normalizeTopupOrderStatus(created?.status);
        if (orderID !== '' && PACKAGE_ORDER_PENDING_STATUSES.has(orderStatus)) {
          pollPackageOrderUntilFinal(orderID).then(() => {
            if (mountedRef.current) {
              loadPackageStatus().then();
            }
          });
        } else {
          await loadPackageStatus();
        }
      }
    } finally {
      setSubmittingPackagePurchase(false);
    }
  }, [
    closePackagePreviewModal,
    createTopupOrder,
    loadPackageStatus,
    packagePreviewState.packageId,
    packagePreviewState?.preview?.operation_type,
    pollPackageOrderUntilFinal,
    t,
  ]);

  const handleRenew = useCallback(async (item) => {
    const packageID = (item?.package_id || '').toString().trim();
    if (packageID === '') {
      showInfo(t('topup.package_status.no_active_package'));
      return;
    }
    setRenewingPackageId(packageID);
    try {
      await openPackagePurchasePreview(packageID, 'renew');
    } finally {
      setRenewingPackageId('');
    }
  }, [openPackagePurchasePreview, t]);

  const previewOperationType = String(packagePreviewState?.preview?.operation_type || '').trim();
  const previewOperationLabel = resolvePackageOperationLabel(previewOperationType, t, i18n);
  const previewConfirmLabel = t('common.confirm');

  return (
    <div className='router-topup-balance-layout'>
      <AppSection
        title={
          <div className='router-title-accent-positive'>
            {t('user.detail.package_title')}
          </div>
        }
        extra={
          <>
          <AppButton
            className='router-section-button'
            onClick={goPricing}
          >
            {t('topup.package_status.view_pricing')}
          </AppButton>
          </>
        }
      >
        {loading ? (
          <div className='router-text-muted'>{t('common.loading')}</div>
        ) : activePackages.length === 0 ? (
          <div className='router-current-package-empty'>
            <div className='router-text-muted'>
              {t('topup.package_status.empty_description')}
            </div>
          </div>
        ) : (
          <div className='router-package-purchase-list'>
            {activePackages.map((item) => (
              <PackageSummaryCard
                key={item.id || item.package_id}
                item={item}
                renewingPackageId={renewingPackageId}
                onRenew={handleRenew}
                renderIntegerAmount={renderIntegerAmount}
                t={t}
              />
            ))}
          </div>
        )}
      </AppSection>

      <AppModal
        size='small'
        open={packagePreviewState.open}
        onClose={closePackagePreviewModal}
        closeOnDimmerClick={!submittingPackagePurchase}
        title={t('topup.external_topup.package_preview_title')}
        footer={[
          <AppButton
            key='cancel'
            onClick={closePackagePreviewModal}
            disabled={submittingPackagePurchase}
          >
            {t('common.cancel')}
          </AppButton>,
          <AppButton
            key='confirm'
            color='blue'
            className='router-section-button'
            loading={submittingPackagePurchase}
            disabled={submittingPackagePurchase}
            onClick={handleConfirmPackagePurchase}
          >
            {previewConfirmLabel}
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
          <div>{previewOperationLabel}</div>

          <div className='router-text-muted'>
            {t('topup.external_topup.package_preview_slot_active_package')}
          </div>
          <div>{packagePreviewState?.preview?.slot_active_package_name || '-'}</div>

          <div className='router-text-muted'>
            {t('topup.external_topup.package_preview_target_package')}
          </div>
          <div>{packagePreviewState?.preview?.target_package_name || '-'}</div>

          <div className='router-text-muted'>
            {t('topup.external_topup.package_preview_target_package_type')}
          </div>
          <div>
            {getServicePackageTypeLabel(
              {
                package_type: packagePreviewState?.preview?.target_package_type,
                quota_metric: packagePreviewState?.preview?.target_quota_metric,
              },
              t,
            )}
          </div>

          <div className='router-text-muted'>
            {t('topup.external_topup.package_preview_slot_active_expire_at')}
          </div>
          <div>
            {formatTimeValue(
              packagePreviewState?.preview?.slot_active_package_expires_at,
              t,
            )}
          </div>

          <div className='router-text-muted'>
            {t('topup.external_topup.package_preview_effective_at')}
          </div>
          <div>{formatTimeValue(packagePreviewState?.preview?.start_at, t)}</div>

          <div className='router-text-muted'>
            {t('topup.external_topup.package_preview_expires_at')}
          </div>
          <div>{formatTimeValue(packagePreviewState?.preview?.expires_at, t)}</div>

          <div className='router-text-muted'>
            {t('topup.external_topup.package_preview_target_price')}
          </div>
          <div>
            {formatMoney(
              packagePreviewState?.preview?.target_package_amount,
              packagePreviewState?.preview?.payable_currency,
            )}
          </div>

          {Number(packagePreviewState?.preview?.slot_active_package_credit_amount || 0) > 0 ? (
            <>
              <div className='router-text-muted'>
                {t('topup.external_topup.package_preview_slot_active_package_credit')}
              </div>
              <div>
                {formatMoney(
                  packagePreviewState?.preview?.slot_active_package_credit_amount,
                  packagePreviewState?.preview?.payable_currency,
                )}
              </div>
            </>
          ) : null}

          <div className='router-text-muted'>
            {t('topup.external_topup.package_preview_payable')}
          </div>
          <div>
            {formatMoney(
              packagePreviewState?.preview?.payable_amount,
              packagePreviewState?.preview?.payable_currency,
            )}
          </div>
        </div>
      </AppModal>
    </div>
  );
};

export default CurrentPackagePage;
