import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { API, showError, showInfo, timestamp2string } from '../../helpers';
import {
  AppButton,
  AppModal,
  AppSection,
  AppSelect,
  AppStatistic,
  AppTag,
} from '../../router-ui';
import {
  buildTopUpReturnURL,
  renderTopupIntegerAmountWithExactPopup,
  useTopUpWorkspace,
} from './shared.jsx';
import {
  formatUserFacingPackageConcurrency,
  formatRequestCount,
  getServicePackagePeriodLabel,
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

const createEmptyActivePackage = () => ({
  has_active_subscription: false,
  current_package: null,
  next_package: null,
  subscription: null,
});

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
    quota_reset_timezone: (raw.quota_reset_timezone || '').toString().trim(),
    started_at: Number(raw.started_at || 0),
    expires_at: Number(raw.expires_at || 0),
    supported_models: supportedModels,
  };
};

const normalizeActivePackage = (raw) => {
  const currentPackage = normalizePackageView(
    raw?.current_package || raw?.subscription,
  );
  const nextPackage = normalizePackageView(raw?.next_package);
  return {
    has_active_subscription:
      raw?.has_active_subscription === true && currentPackage !== null,
    current_package: currentPackage,
    next_package: nextPackage,
    subscription: currentPackage,
  };
};

const createEmptyDailySnapshot = () => ({
  biz_date: '',
  timezone: '',
  limit: 0,
  consumed_quota: 0,
  reserved_quota: 0,
  remaining_quota: 0,
  unlimited: false,
});

const createEmptyQuotaSummary = () => ({
  package_emergency: {
    biz_month: '',
    timezone: '',
    limit: 0,
    consumed_quota: 0,
    reserved_quota: 0,
    remaining_quota: 0,
    enabled: false,
  },
});

const resolvePackageTypeKey = (item) =>
  normalizeServicePackageType(item?.package_type, item?.quota_metric);

const resolvePackageSalePrice = (item) => {
  const normalized = Number(item?.sale_price ?? 0);
  return Number.isFinite(normalized) ? normalized : 0;
};

const normalizeDailySnapshot = (raw) => ({
  biz_date: (raw?.biz_date || '').toString().trim(),
  timezone: (raw?.timezone || '').toString().trim(),
  limit: Number(raw?.limit_amount ?? raw?.limit ?? 0) || 0,
  consumed_quota: Number(raw?.consumed_amount ?? raw?.consumed_quota ?? 0) || 0,
  reserved_quota: Number(raw?.reserved_amount ?? raw?.reserved_quota ?? 0) || 0,
  remaining_quota: Number(raw?.remaining_amount ?? raw?.remaining_quota ?? 0) || 0,
  unlimited: raw?.unlimited === true,
});

const normalizeQuotaSummary = (raw) => ({
  package_emergency: {
    biz_month: (raw?.package_emergency?.biz_month || '').toString().trim(),
    timezone: (raw?.package_emergency?.timezone || '').toString().trim(),
    limit: Number(
      raw?.package_emergency?.limit_amount ??
        raw?.package_emergency?.limit ??
        0,
    ) || 0,
    consumed_quota:
      Number(
        raw?.package_emergency?.consumed_amount ??
          raw?.package_emergency?.consumed_quota ??
          0,
      ) || 0,
    reserved_quota:
      Number(
        raw?.package_emergency?.reserved_amount ??
          raw?.package_emergency?.reserved_quota ??
          0,
      ) || 0,
    remaining_quota:
      Number(
        raw?.package_emergency?.remaining_amount ??
          raw?.package_emergency?.remaining_quota ??
          0,
      ) || 0,
    enabled: raw?.package_emergency?.enabled === true,
  },
});

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
    case 5:
      return (
        <AppTag color='teal' className='router-tag'>
          {t('user.detail.package_status_types.pending')}
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

const PackageUsageCard = ({ title, period, timezone, items, footer }) => (
  <AppSection title={title}>
    <div className='router-topup-usage-card-body'>
      <div className='router-topup-stat-grid'>
        {items.map((item) => (
          <AppStatistic
            key={item.key}
            className='router-topup-statistic'
            title={item.label}
            value={0}
            formatter={() => item.value}
          />
        ))}
      </div>
      <div className='router-topup-usage-meta'>
        <span>{period}</span>
        <span>{timezone}</span>
        {footer ? <span>{footer}</span> : null}
      </div>
    </div>
  </AppSection>
);

const PackageSupportedModelsSection = ({ title, models, t }) => {
  const normalizedModels = Array.isArray(models) ? models : [];
  return (
    <AppSection
      title={title}
      extra={
        normalizedModels.length > 0
          ? `${normalizedModels.length} ${t('user.detail.package_supported_models_count_unit')}`
          : null
      }
    >
      {normalizedModels.length === 0 ? (
        <div className='router-text-muted'>
          {t('user.detail.package_supported_models_empty')}
        </div>
      ) : (
        <div className='router-current-package-model-list'>
          {normalizedModels.map((modelName) => (
            <AppTag key={modelName} className='router-tag'>
              {modelName}
            </AppTag>
          ))}
        </div>
      )}
    </AppSection>
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
  const [dailySnapshot, setDailySnapshot] = useState(createEmptyDailySnapshot());
  const [quotaSummary, setQuotaSummary] = useState(createEmptyQuotaSummary());
  const [renewing, setRenewing] = useState(false);
  const [changeModalOpen, setChangeModalOpen] = useState(false);
  const [loadingChangeTargets, setLoadingChangeTargets] = useState(false);
  const [changeTargets, setChangeTargets] = useState([]);
  const [selectedChangePackageId, setSelectedChangePackageId] = useState('');
  const [submittingChange, setSubmittingChange] = useState(false);
  const [submittingPackagePurchase, setSubmittingPackagePurchase] = useState(false);
  const [packagePreviewState, setPackagePreviewState] = useState({
    open: false,
    packageId: '',
    preview: null,
  });
  const [changeOperationType, setChangeOperationType] = useState('');

  const renderIntegerAmount = useCallback(
    (chargeAmount) =>
      renderTopupIntegerAmountWithExactPopup({
        chargeAmount,
        displayCurrency,
        displayCurrencyIndex,
      }),
    [displayCurrency, displayCurrencyIndex],
  );

  const loadQuotaSummary = useCallback(async () => {
    const res = await API.get('/api/v1/public/user/quota/summary');
    const { success, message, data } = res?.data || {};
    if (!success) {
      throw new Error(message || t('user.messages.operation_failed'));
    }
    setQuotaSummary(normalizeQuotaSummary(data));
  }, [t]);

  const loadDailySnapshot = useCallback(
    async (groupId) => {
      const normalizedGroupId = (groupId || '').toString().trim();
      if (normalizedGroupId === '') {
        setDailySnapshot(createEmptyDailySnapshot());
        return;
      }
      const res = await API.get('/api/v1/public/user/quota/daily', {
        params: {
          group_id: normalizedGroupId,
        },
      });
      const { success, message, data } = res?.data || {};
      if (!success) {
        throw new Error(message || t('user.messages.operation_failed'));
      }
      setDailySnapshot(normalizeDailySnapshot(data));
    },
    [t],
  );

  const loadPackageStatus = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/v1/public/user/package/subscription');
      const { success, message, data } = res?.data || {};
      if (!success) {
        throw new Error(message || t('user.messages.active_package_load_failed'));
      }
      const normalizedPackage = normalizeActivePackage(data);
      setActivePackage(normalizedPackage);
      if (normalizedPackage.has_active_subscription) {
        if (isRequestQuotaPackage(normalizedPackage.subscription)) {
          setDailySnapshot(createEmptyDailySnapshot());
          setQuotaSummary(createEmptyQuotaSummary());
          return;
        }
        await Promise.all([
          loadDailySnapshot(normalizedPackage.subscription?.group_id),
          loadQuotaSummary(),
        ]);
        return;
      }
      setDailySnapshot(createEmptyDailySnapshot());
      setQuotaSummary(createEmptyQuotaSummary());
    } catch (error) {
      showError(error?.message || t('user.messages.active_package_load_failed'));
    } finally {
      setLoading(false);
    }
  }, [loadDailySnapshot, loadQuotaSummary, t]);

  useEffect(() => {
    loadPackageStatus().then();
  }, [loadPackageStatus]);

  const activeSubscription = activePackage.has_active_subscription
    ? activePackage.current_package
    : null;
  const nextSubscription = activePackage.next_package;
  const activeRequestQuotaPackage = isRequestQuotaPackage(activeSubscription);

  const infoItems = useMemo(() => {
    if (!activeSubscription) {
      return [];
    }
    const baseItems = [
      {
        key: 'package_name',
        label: t('user.detail.package_name'),
        value: activeSubscription.package_name || '-',
      },
      {
        key: 'status',
        label: t('user.detail.package_status'),
        value: renderPackageStatus(activeSubscription.status, t),
      },
      {
        key: 'package_type',
        label: t('package_manage.table.package_type'),
        value: getServicePackageTypeLabel(activeSubscription, t),
      },
    ];
    const entitlementItems = activeRequestQuotaPackage
      ? [
        {
          key: 'period_limit',
          label: t('package_manage.table.period_entitlement'),
          value: `${formatRequestCount(activeSubscription.period_limit || 0)} ${t(
            'package_manage.request_unit',
          )} / ${getServicePackagePeriodLabel(activeSubscription.period_type, t)}`,
        },
        {
          key: 'concurrency',
          label: t('package_manage.table.concurrency_limit'),
          value: formatUserFacingPackageConcurrency(
            activeSubscription,
            t,
            t('common.unlimited'),
          ),
        },
      ]
      : [
        {
          key: 'daily_limit',
          label: t('user.detail.package_daily_limit'),
          value: renderIntegerAmount(activeSubscription.daily_quota_limit || 0),
        },
        {
          key: 'emergency_limit',
          label: t('user.detail.package_emergency_limit'),
          value: renderIntegerAmount(
            activeSubscription.package_emergency_quota_limit || 0,
          ),
        },
        {
          key: 'concurrency',
          label: t('package_manage.table.concurrency_limit'),
          value: formatUserFacingPackageConcurrency(
            activeSubscription,
            t,
            t('common.unlimited'),
          ),
        },
      ];
    return [
      ...baseItems,
      ...entitlementItems,
      {
        key: 'timezone',
        label: t('user.detail.package_timezone'),
        value: activeSubscription.quota_reset_timezone || '-',
      },
      {
        key: 'started_at',
        label: t('user.detail.package_started_at'),
        value: activeSubscription.started_at
          ? timestamp2string(activeSubscription.started_at)
          : '-',
      },
      {
        key: 'expires_at',
        label: t('user.detail.package_expires_at'),
        value: activeSubscription.expires_at
          ? timestamp2string(activeSubscription.expires_at)
          : '-',
      },
    ];
  }, [activeRequestQuotaPackage, activeSubscription, renderIntegerAmount, t]);

  const nextInfoItems = useMemo(() => {
    if (!nextSubscription) {
      return [];
    }
    const entitlementItems = isRequestQuotaPackage(nextSubscription)
      ? [
        {
          key: 'period_limit',
          label: t('package_manage.table.period_entitlement'),
          value: `${formatRequestCount(nextSubscription.period_limit || 0)} ${t(
            'package_manage.request_unit',
          )} / ${getServicePackagePeriodLabel(nextSubscription.period_type, t)}`,
        },
        {
          key: 'concurrency',
          label: t('package_manage.table.concurrency_limit'),
          value: formatUserFacingPackageConcurrency(
            nextSubscription,
            t,
            t('common.unlimited'),
          ),
        },
      ]
      : [
        {
          key: 'daily_limit',
          label: t('user.detail.package_daily_limit'),
          value: renderIntegerAmount(nextSubscription.daily_quota_limit || 0),
        },
        {
          key: 'emergency_limit',
          label: t('user.detail.package_emergency_limit'),
          value: renderIntegerAmount(nextSubscription.package_emergency_quota_limit || 0),
        },
        {
          key: 'concurrency',
          label: t('package_manage.table.concurrency_limit'),
          value: formatUserFacingPackageConcurrency(
            nextSubscription,
            t,
            t('common.unlimited'),
          ),
        },
      ];
    return [
      {
        key: 'package_name',
        label: t('user.detail.package_name'),
        value: nextSubscription.package_name || '-',
      },
      {
        key: 'status',
        label: t('user.detail.package_status'),
        value: renderPackageStatus(nextSubscription.status, t),
      },
      {
        key: 'package_type',
        label: t('package_manage.table.package_type'),
        value: getServicePackageTypeLabel(nextSubscription, t),
      },
      ...entitlementItems,
      {
        key: 'started_at',
        label: t('topup.package_status.next_effective_at'),
        value: nextSubscription.started_at
          ? timestamp2string(nextSubscription.started_at)
          : '-',
      },
      {
        key: 'expires_at',
        label: t('user.detail.package_expires_at'),
        value: nextSubscription.expires_at
          ? timestamp2string(nextSubscription.expires_at)
          : '-',
      },
    ];
  }, [nextSubscription, renderIntegerAmount, t]);

  const dailyItems = useMemo(() => {
    if (!activeSubscription || activeRequestQuotaPackage) {
      return [];
    }
    return [
      {
        key: 'daily_limit',
        label: t('user.detail.package_daily_limit'),
        value: dailySnapshot.unlimited
          ? t('common.unlimited')
          : renderIntegerAmount(dailySnapshot.limit),
      },
      {
        key: 'daily_used',
        label: t('user.detail.used_amount'),
        value: renderIntegerAmount(dailySnapshot.consumed_quota),
      },
      {
        key: 'daily_remaining',
        label: t('user.detail.remaining_amount'),
        value: dailySnapshot.unlimited
          ? t('common.unlimited')
          : renderIntegerAmount(dailySnapshot.remaining_quota),
      },
    ];
  }, [activeRequestQuotaPackage, activeSubscription, dailySnapshot, renderIntegerAmount, t]);

  const emergencySnapshot = quotaSummary.package_emergency;
  const emergencyItems = useMemo(() => {
    if (!activeSubscription || activeRequestQuotaPackage) {
      return [];
    }
    return [
      {
        key: 'emergency_limit',
        label: t('user.detail.package_emergency_limit'),
        value: emergencySnapshot.enabled
          ? renderIntegerAmount(emergencySnapshot.limit)
          : '-',
      },
      {
        key: 'emergency_used',
        label: t('user.detail.used_amount'),
        value: emergencySnapshot.enabled
          ? renderIntegerAmount(emergencySnapshot.consumed_quota)
          : '-',
      },
      {
        key: 'emergency_remaining',
        label: t('user.detail.remaining_amount'),
        value: emergencySnapshot.enabled
          ? renderIntegerAmount(emergencySnapshot.remaining_quota)
          : '-',
      },
    ];
  }, [activeRequestQuotaPackage, activeSubscription, emergencySnapshot, renderIntegerAmount, t]);

  const requestUsage = activeSubscription?.usage || {};
  const requestUsageItems = useMemo(() => {
    if (!activeSubscription || !activeRequestQuotaPackage) {
      return [];
    }
    return [
      {
        key: 'request_limit',
        label: t('package_manage.form.period_limit'),
        value: requestUsage.unlimited
          ? t('common.unlimited')
          : formatRequestCount(requestUsage.limit_amount || activeSubscription.period_limit || 0),
      },
      {
        key: 'request_consumed',
        label: t('user.detail.used_amount'),
        value: formatRequestCount(requestUsage.consumed_amount || 0),
      },
      {
        key: 'request_remaining',
        label: t('user.detail.remaining_amount'),
        value: requestUsage.unlimited
          ? t('common.unlimited')
          : formatRequestCount(requestUsage.remaining_amount || 0),
      },
    ];
  }, [activeRequestQuotaPackage, activeSubscription, requestUsage, t]);

  const goPricing = useCallback(
    (intent = '') => {
      const normalizedIntent = String(intent || '').trim().toLowerCase();
      const search = new URLSearchParams();
      if (normalizedIntent === 'renew' || normalizedIntent === 'upgrade') {
        search.set('intent', normalizedIntent);
      }
      navigate(`/workspace/service/pricing${search.toString() ? `?${search.toString()}` : ''}`);
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
        setChangeModalOpen(false);
      }
    } finally {
      setSubmittingPackagePurchase(false);
    }
  }, [closePackagePreviewModal, createTopupOrder, packagePreviewState.packageId, packagePreviewState?.preview?.operation_type, t]);

  const handleRenew = useCallback(async () => {
    const packageID = (activeSubscription?.package_id || '').toString().trim();
    if (packageID === '') {
      showInfo(t('topup.package_status.no_active_package'));
      return;
    }
    setRenewing(true);
    try {
      await openPackagePurchasePreview(packageID, 'renew');
    } finally {
      setRenewing(false);
    }
  }, [activeSubscription?.package_id, openPackagePurchasePreview, t]);

  const buildPackageOptionText = useCallback((item) => {
    const name = String(item?.name || item?.package_name || item?.id || '-').trim();
    const price = Number(item?.sale_price ?? 0);
    const currency = String(item?.sale_currency || 'USD').toUpperCase();
    if (!Number.isFinite(price) || price <= 0) {
      return name;
    }
    return `${name} (${currency} ${price.toFixed(2)})`;
  }, []);

  const loadPackageChangeTargets = useCallback(async (operationType) => {
    const currentPackageID = (activeSubscription?.package_id || '').toString().trim();
    if (currentPackageID === '') {
      showInfo(t('topup.package_status.no_active_package'));
      return [];
    }
    setLoadingChangeTargets(true);
    try {
      const res = await API.get('/api/v1/public/user/packages');
      const { success, message, data } = res?.data || {};
      if (!success) {
        throw new Error(message || t('topup.external_topup.request_failed'));
      }
      const rows = Array.isArray(data) ? data : [];
      const currentType = resolvePackageTypeKey(activeSubscription);
      const currentPrice = resolvePackageSalePrice(activeSubscription);
      const normalizedOperationType = String(operationType || '').trim().toLowerCase();
      const candidates = rows.filter((row) => {
        const rowID = String(row?.id || '').trim();
        if (rowID === '' || rowID === currentPackageID) {
          return false;
        }
        const status = Number(row?.status ?? 1);
        if (Number.isFinite(status) && status !== 1) {
          return false;
        }
        const rowType = resolvePackageTypeKey(row);
        const rowPrice = resolvePackageSalePrice(row);
        if (normalizedOperationType === 'upgrade') {
          return rowType === currentType && rowPrice > currentPrice;
        }
        if (normalizedOperationType === 'downgrade') {
          return rowType === currentType && rowPrice < currentPrice;
        }
        if (normalizedOperationType === 'convert') {
          return rowType !== currentType;
        }
        return false;
      });
      return candidates;
    } catch (error) {
      showError(error?.message || t('topup.external_topup.request_failed'));
      return [];
    } finally {
      setLoadingChangeTargets(false);
    }
  }, [activeSubscription, t]);

  const openPackageChangeModal = useCallback(async (operationType) => {
    const candidates = await loadPackageChangeTargets(operationType);
    if (candidates.length === 0) {
      const messageKey =
        operationType === 'downgrade'
          ? 'topup.package_status.no_downgrade_target'
          : operationType === 'convert'
            ? 'topup.package_status.no_convert_target'
            : 'topup.package_status.no_upgrade_target';
      showInfo(t(messageKey));
      return;
    }
    const defaultTargetID = String(candidates[0]?.id || '').trim();
    setChangeTargets(candidates);
    setSelectedChangePackageId(defaultTargetID);
    setChangeOperationType(operationType);
    setChangeModalOpen(true);
  }, [loadPackageChangeTargets, t]);

  const handleUpgrade = useCallback(async () => {
    await openPackageChangeModal('upgrade');
  }, [openPackageChangeModal]);

  const handleDowngrade = useCallback(async () => {
    await openPackageChangeModal('downgrade');
  }, [openPackageChangeModal]);

  const handleConvert = useCallback(async () => {
    await openPackageChangeModal('convert');
  }, [openPackageChangeModal]);

  const handleConfirmChange = useCallback(async () => {
    const targetPackageID = (selectedChangePackageId || '').trim();
    if (targetPackageID === '') {
      showInfo(t('topup.external_topup.package_select_required'));
      return;
    }
    setSubmittingChange(true);
    try {
      const opened = await openPackagePurchasePreview(targetPackageID, changeOperationType);
      if (opened) {
        setChangeModalOpen(false);
      }
    } finally {
      setSubmittingChange(false);
    }
  }, [changeOperationType, openPackagePurchasePreview, selectedChangePackageId, t]);

  const changeOptions = useMemo(
    () =>
      (changeTargets || []).map((item) => ({
        key: String(item?.id || ''),
        value: String(item?.id || ''),
        label: buildPackageOptionText(item),
      })),
    [buildPackageOptionText, changeTargets],
  );

  const selectedChangePackageOption = useMemo(() => {
    const normalizedPackageID = String(selectedChangePackageId || '').trim();
    if (normalizedPackageID === '') {
      return undefined;
    }
    const matchedOption = changeOptions.find(
      (item) => String(item?.value || '').trim() === normalizedPackageID,
    );
    if (matchedOption) {
      return {
        value: matchedOption.value,
        label: matchedOption.label || matchedOption.text || matchedOption.value,
      };
    }
    return {
      value: normalizedPackageID,
      label: normalizedPackageID,
    };
  }, [changeOptions, selectedChangePackageId]);

  const changeTitleKey =
    changeOperationType === 'downgrade'
      ? 'topup.package_status.select_downgrade_target'
      : changeOperationType === 'convert'
        ? 'topup.package_status.select_convert_target'
        : 'topup.package_status.select_upgrade_target';
  const changeHintKey =
    changeOperationType === 'downgrade'
      ? 'topup.package_status.select_downgrade_target_hint'
      : changeOperationType === 'convert'
        ? 'topup.package_status.select_convert_target_hint'
        : 'topup.package_status.select_upgrade_target_hint';
  const changeConfirmKey =
    changeOperationType === 'downgrade'
      ? 'topup.package_status.downgrade_next_cycle'
      : changeOperationType === 'convert'
        ? 'topup.package_status.convert_next_cycle'
        : 'topup.package_status.upgrade_now';
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
            onClick={() => goPricing('')}
          >
            {t('topup.package_status.view_pricing')}
          </AppButton>
          <AppButton
            className='router-section-button'
            basic
            disabled={!activeSubscription || loadingChangeTargets}
            loading={renewing}
            onClick={handleRenew}
          >
            {t('topup.external_topup.package_operation.renew')}
          </AppButton>
          <AppButton
            className='router-section-button'
            basic
            disabled={!activeSubscription}
            loading={loadingChangeTargets || submittingChange}
            onClick={handleUpgrade}
          >
            {t('topup.external_topup.package_operation.upgrade')}
          </AppButton>
          <AppButton
            className='router-section-button'
            basic
            disabled={!activeSubscription}
            loading={loadingChangeTargets || submittingChange}
            onClick={handleDowngrade}
          >
            {t('topup.external_topup.package_operation.downgrade')}
          </AppButton>
          <AppButton
            className='router-section-button'
            basic
            disabled={!activeSubscription}
            loading={loadingChangeTargets || submittingChange}
            onClick={handleConvert}
          >
            {t('topup.external_topup.package_operation.convert')}
          </AppButton>
          </>
        }
      >
        {loading ? (
          <div className='router-text-muted'>{t('common.loading')}</div>
        ) : !activeSubscription ? (
          <div className='router-current-package-empty'>
            <div className='router-text-muted'>
              {t('topup.package_status.empty_description')}
            </div>
          </div>
        ) : (
          <div className='router-current-package-info-grid'>
            {infoItems.map((item) => (
              <div key={item.key} className='router-current-package-info-card'>
                <div className='router-current-package-info-label'>
                  {item.label}
                </div>
                <div className='router-current-package-info-value'>{item.value}</div>
              </div>
            ))}
          </div>
        )}
      </AppSection>

      {activeSubscription ? (
        <PackageSupportedModelsSection
          title={t('user.detail.package_supported_models')}
          models={activeSubscription.supported_models}
          t={t}
        />
      ) : null}

      {nextSubscription ? (
        <AppSection
          title={
            <div className='router-title-accent-positive'>
              {t('topup.package_status.next_package_title')}
            </div>
          }
        >
          <div className='router-current-package-info-grid'>
            {nextInfoItems.map((item) => (
              <div key={item.key} className='router-current-package-info-card'>
                <div className='router-current-package-info-label'>
                  {item.label}
                </div>
                <div className='router-current-package-info-value'>{item.value}</div>
              </div>
            ))}
          </div>
        </AppSection>
      ) : null}

      {nextSubscription ? (
        <PackageSupportedModelsSection
          title={t('topup.package_status.next_package_supported_models')}
          models={nextSubscription.supported_models}
          t={t}
        />
      ) : null}

      {activeSubscription && activeRequestQuotaPackage ? (
        <PackageUsageCard
          title={t('package_manage.table.period_entitlement')}
          period={`${t('topup.package_status.period')}: ${requestUsage.period_key || '-'}`}
          timezone={`${t('user.detail.package_timezone')}: ${activeSubscription.quota_reset_timezone || '-'}`}
          footer={
            <>
              {t('topup.package_status.reserved')}:{' '}
              {formatRequestCount(requestUsage.reserved_amount || 0)}
            </>
          }
          items={requestUsageItems}
        />
      ) : null}

      {activeSubscription && !activeRequestQuotaPackage ? (
        <>
          <PackageUsageCard
            title={t('topup.package_status.daily_title')}
            period={`${t('topup.package_status.period')}: ${dailySnapshot.biz_date || '-'}`}
            timezone={`${t('user.detail.package_timezone')}: ${dailySnapshot.timezone || activeSubscription.quota_reset_timezone || '-'}`}
            footer={
              <>
                {t('topup.package_status.reserved')}: {renderIntegerAmount(dailySnapshot.reserved_quota)}
              </>
            }
            items={dailyItems}
          />
          <PackageUsageCard
            title={t('topup.package_status.emergency_title')}
            period={`${t('topup.package_status.period')}: ${emergencySnapshot.biz_month || '-'}`}
            timezone={`${t('user.detail.package_timezone')}: ${emergencySnapshot.timezone || activeSubscription.quota_reset_timezone || '-'}`}
            footer={
              <>
                {t('topup.package_status.reserved')}:{' '}
                {emergencySnapshot.enabled
                  ? renderIntegerAmount(emergencySnapshot.reserved_quota)
                  : '-'}
              </>
            }
            items={emergencyItems}
          />
        </>
      ) : null}

      <AppModal
        size='small'
        open={changeModalOpen}
        onClose={() => {
          if (submittingChange) {
            return;
          }
          setChangeModalOpen(false);
        }}
        title={t(changeTitleKey)}
        footer={[
          <AppButton
            key='cancel'
            className='router-section-button'
            onClick={() => setChangeModalOpen(false)}
            disabled={submittingChange}
          >
            {t('common.cancel')}
          </AppButton>,
          <AppButton
            key='confirm'
            color='blue'
            className='router-section-button'
            loading={submittingChange}
            disabled={submittingChange || selectedChangePackageId === ''}
            onClick={handleConfirmChange}
          >
            {t(changeConfirmKey)}
          </AppButton>,
        ]}
      >
        <div className='router-current-package-upgrade-body'>
          <div className='router-text-muted'>
            {t(changeHintKey)}
          </div>
          <AppSelect
            className='router-page-dropdown'
            labelInValue
            options={changeOptions}
            value={selectedChangePackageOption}
            onChange={(_, data) => {
              const nextValue =
                typeof data?.value === 'object'
                  ? (data?.value?.value || '').toString()
                  : (data?.value || '').toString();
              setSelectedChangePackageId(nextValue);
            }}
          />
        </div>
      </AppModal>

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
            {t('topup.external_topup.package_preview_current_package')}
          </div>
          <div>{packagePreviewState?.preview?.current_package_name || '-'}</div>

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
            {t('topup.external_topup.package_preview_current_expire_at')}
          </div>
          <div>{formatTimeValue(packagePreviewState?.preview?.current_expires_at, t)}</div>

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

          {Number(packagePreviewState?.preview?.current_package_credit_amount || 0) > 0 ? (
            <>
              <div className='router-text-muted'>
                {t('topup.external_topup.package_preview_current_package_credit')}
              </div>
              <div>
                {formatMoney(
                  packagePreviewState?.preview?.current_package_credit_amount,
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
