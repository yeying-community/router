import { formatDecimalNumber } from './render';

export const SERVICE_PACKAGE_TYPE_YYC_QUOTA = 'yyc_quota';
export const SERVICE_PACKAGE_TYPE_REQUEST_QUOTA = 'request_quota';

export const SERVICE_PACKAGE_QUOTA_METRIC_YYC = 'yyc';
export const SERVICE_PACKAGE_QUOTA_METRIC_REQUEST_COUNT = 'request_count';

export const SERVICE_PACKAGE_PERIOD_DAILY = 'daily';
export const SERVICE_PACKAGE_PERIOD_WEEKLY = 'weekly';
export const SERVICE_PACKAGE_PERIOD_MONTHLY = 'monthly';
export const SERVICE_PACKAGE_PERIOD_PACKAGE_TOTAL = 'package_total';

export const normalizeServicePackageType = (value, quotaMetric = '') => {
  const normalized = (value || '').toString().trim().toLowerCase();
  if (normalized === SERVICE_PACKAGE_TYPE_REQUEST_QUOTA) {
    return SERVICE_PACKAGE_TYPE_REQUEST_QUOTA;
  }
  if (normalized === SERVICE_PACKAGE_TYPE_YYC_QUOTA) {
    return SERVICE_PACKAGE_TYPE_YYC_QUOTA;
  }
  return (quotaMetric || '').toString().trim().toLowerCase() ===
    SERVICE_PACKAGE_QUOTA_METRIC_REQUEST_COUNT
    ? SERVICE_PACKAGE_TYPE_REQUEST_QUOTA
    : SERVICE_PACKAGE_TYPE_YYC_QUOTA;
};

export const isRequestQuotaPackage = (item) =>
  normalizeServicePackageType(item?.package_type, item?.quota_metric) ===
  SERVICE_PACKAGE_TYPE_REQUEST_QUOTA;

export const normalizeServicePackagePeriodType = (value) => {
  const normalized = (value || '').toString().trim().toLowerCase();
  switch (normalized) {
    case SERVICE_PACKAGE_PERIOD_DAILY:
    case SERVICE_PACKAGE_PERIOD_WEEKLY:
    case SERVICE_PACKAGE_PERIOD_MONTHLY:
    case SERVICE_PACKAGE_PERIOD_PACKAGE_TOTAL:
      return normalized;
    default:
      return SERVICE_PACKAGE_PERIOD_MONTHLY;
  }
};

export const formatRequestCount = (value) => {
  const normalized = Number(value || 0);
  if (!Number.isFinite(normalized)) {
    return '0';
  }
  return formatDecimalNumber(Math.trunc(normalized), 0);
};

export const getServicePackageTypeLabel = (item, t) =>
  isRequestQuotaPackage(item)
    ? t('package_manage.package_type.request_quota')
    : t('package_manage.package_type.yyc_quota');

export const getServicePackagePeriodLabel = (periodType, t) => {
  switch (normalizeServicePackagePeriodType(periodType)) {
    case SERVICE_PACKAGE_PERIOD_DAILY:
      return t('package_manage.period_type.daily');
    case SERVICE_PACKAGE_PERIOD_WEEKLY:
      return t('package_manage.period_type.weekly');
    case SERVICE_PACKAGE_PERIOD_PACKAGE_TOTAL:
      return t('package_manage.period_type.package_total');
    case SERVICE_PACKAGE_PERIOD_MONTHLY:
    default:
      return t('package_manage.period_type.monthly');
  }
};

export const getServicePackagePeriodOptions = (t) => [
  {
    key: SERVICE_PACKAGE_PERIOD_MONTHLY,
    value: SERVICE_PACKAGE_PERIOD_MONTHLY,
    text: t('package_manage.period_type.monthly'),
  },
  {
    key: SERVICE_PACKAGE_PERIOD_DAILY,
    value: SERVICE_PACKAGE_PERIOD_DAILY,
    text: t('package_manage.period_type.daily'),
  },
  {
    key: SERVICE_PACKAGE_PERIOD_WEEKLY,
    value: SERVICE_PACKAGE_PERIOD_WEEKLY,
    text: t('package_manage.period_type.weekly'),
  },
  {
    key: SERVICE_PACKAGE_PERIOD_PACKAGE_TOTAL,
    value: SERVICE_PACKAGE_PERIOD_PACKAGE_TOTAL,
    text: t('package_manage.period_type.package_total'),
  },
];

export const formatRequestQuotaEntitlement = (item, t) => {
  const limit = Number(item?.period_limit ?? item?.usage?.limit_amount ?? 0);
  const period = getServicePackagePeriodLabel(item?.period_type, t);
  if (!Number.isFinite(limit) || limit <= 0) {
    return t('common.unlimited');
  }
  return `${formatRequestCount(limit)} ${t('package_manage.request_unit')} / ${period}`;
};

export const formatRequestQuotaConcurrency = (item, t) => {
  const perUser = Number(item?.max_concurrency_per_user || 0);
  const perPackage = Number(item?.max_concurrency_per_package || 0);
  if (perUser <= 0 && perPackage <= 0) {
    return t('common.unlimited');
  }
  const parts = [];
  if (perUser > 0) {
    parts.push(`${t('package_manage.form.max_concurrency_per_user')}: ${perUser}`);
  }
  if (perPackage > 0) {
    parts.push(`${t('package_manage.form.max_concurrency_per_package')}: ${perPackage}`);
  }
  return parts.join(' / ');
};

export const formatPackageConcurrencyLimit = (item, t, emptyLabel = '-') => {
  const perUser = Number(item?.max_concurrency_per_user || 0);
  const perPackage = Number(item?.max_concurrency_per_package || 0);
  const parts = [];
  if (perUser > 0) {
    parts.push(`${t('package_manage.form.max_concurrency_per_user')}: ${perUser}`);
  }
  if (perPackage > 0) {
    parts.push(`${t('package_manage.form.max_concurrency_per_package')}: ${perPackage}`);
  }
  if (parts.length === 0) {
    return emptyLabel;
  }
  return parts.join(' / ');
};

export const formatUserFacingPackageConcurrency = (
  item,
  t,
  emptyLabel = '-',
) => {
  const perPackage = Number(item?.max_concurrency_per_package || 0);
  if (Number.isFinite(perPackage) && perPackage > 0) {
    return `${perPackage}`;
  }
  const perUser = Number(item?.max_concurrency_per_user || 0);
  if (Number.isFinite(perUser) && perUser > 0) {
    return `${perUser}`;
  }
  return emptyLabel === t?.('common.unlimited') ? emptyLabel : emptyLabel;
};

export const formatPackageExtraEntitlement = (item, t, emergencyValue = '') => {
  if (isRequestQuotaPackage(item)) {
    return formatRequestQuotaConcurrency(item, t);
  }
  const normalizedEmergency = `${emergencyValue ?? ''}`.trim();
  if (normalizedEmergency === '') {
    return t('common.unlimited');
  }
  return `${t('user.detail.package_emergency_limit')}: ${normalizedEmergency}`;
};
