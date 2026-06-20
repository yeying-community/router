import React, { useMemo, useState } from 'react';
import UnitDropdown from '../../../components/UnitDropdown';
import {
  AppAlert,
  AppButton,
  AppCompact,
  AppDetailSection,
  AppDivider,
  AppField,
  AppFormActions,
  AppFormRow,
  AppInput,
  AppInputNumber,
  AppModal,
  AppPopconfirm,
  AppSelect,
  AppTable,
  AppTableActionButton,
  AppTag,
  AppTooltip,
} from '../../../router-ui';

const buildManualQuotaItem = () => ({
  resource_type: 'quota',
  quota_type: 'total',
  quota_label: '',
  limit_amount: null,
  used_amount: 0,
  remaining_amount: null,
  currency: 'USD',
  reset_at_input: '',
  expires_at_input: '',
  source_ref: 'manual',
});

const toDateTimeLocalValue = (date) => {
  const pad = (value) => String(value).padStart(2, '0');
  return [
    date.getFullYear(),
    '-',
    pad(date.getMonth() + 1),
    '-',
    pad(date.getDate()),
    'T',
    pad(date.getHours()),
    ':',
    pad(date.getMinutes()),
  ].join('');
};

const buildManualPurchaseRecord = () => ({
  purchase_at_input: toDateTimeLocalValue(new Date()),
  purchase_currency: 'CNY',
  purchase_amount: null,
  purchase_fx_rate: 1,
  purchase_cost_amount: null,
});

const MANUAL_CURRENCY_OPTIONS = ['USD', 'CNY', 'YYC'].map((value) => ({
  value,
  label: value,
}));

const PROCUREMENT_CURRENCY_OPTIONS = ['CNY', 'USD'].map((value) => ({
  value,
  label: value,
}));

const procurementScopeOptions = (t) => [
  { value: 'global', label: t('channel.edit.billing.procurement_scopes.global') },
  { value: 'model', label: t('channel.edit.billing.procurement_scopes.model') },
];

const ensureUnitOption = (options, value) => {
  const normalized = (value || '').toString().trim().toUpperCase();
  const items = Array.isArray(options) ? options : [];
  if (!normalized || items.some((option) => option?.value === normalized)) {
    return items;
  }
  return [...items, { value: normalized, label: normalized }];
};

const resourceTypeOptions = (t) => [
  { value: 'quota', label: t('channel.edit.billing.resource_types.quota') },
  { value: 'balance', label: t('channel.edit.billing.resource_types.balance') },
  { value: 'credit', label: t('channel.edit.billing.resource_types.credit') },
  { value: 'plan', label: t('channel.edit.billing.resource_types.plan') },
];

const quotaTypeOptions = (t) => [
  { value: 'daily', label: t('channel.edit.billing.quota_types.daily') },
  { value: 'weekly', label: t('channel.edit.billing.quota_types.weekly') },
  { value: 'monthly', label: t('channel.edit.billing.quota_types.monthly') },
  { value: 'total', label: t('channel.edit.billing.quota_types.total') },
  { value: 'custom', label: t('channel.edit.billing.quota_types.custom') },
];

const formatAmountText = (item) => {
  const amount = Number(item?.amount || 0);
  const currency = (item?.currency || '').toString().trim();
  if (currency !== '') {
    return `${amount} ${currency}`;
  }
  return `${amount}`;
};

const normalizeBillingValue = (value) =>
  (value || '').toString().trim().toLowerCase();

const buildQuotaItemRowKey = (row) =>
  [
    row?.id || '',
    row?.quota_label || '',
    row?.quota_type || '',
    row?.resource_type || '',
    row?.billing_cycle || '',
  ].join('-');

const isPeriodicQuotaType = (quotaType) =>
  ['daily', 'weekly', 'monthly'].includes(normalizeBillingValue(quotaType));

const isPlanEntitlement = (item) =>
  normalizeBillingValue(item?.resource_type) === 'plan';

const isManualPlanItem = (item) =>
  normalizeBillingValue(item?.resource_type) === 'plan';

const isManualPeriodicItem = (item) =>
  normalizeBillingValue(item?.resource_type) === 'quota' &&
  isPeriodicQuotaType(item?.quota_type);

const shouldShowManualQuotaType = (item) =>
  normalizeBillingValue(item?.resource_type) === 'quota';

const shouldShowManualAmountFields = (item) => !isManualPlanItem(item);

const shouldShowManualRemainingFields = (item) => !isManualPeriodicItem(item);

const resolveManualResourceHint = (item, t) => {
  const resourceType = normalizeBillingValue(item?.resource_type);
  if (resourceType === 'plan') {
    return t('channel.edit.billing.manual_resource_hints.plan');
  }
  if (resourceType === 'credit') {
    return t('channel.edit.billing.manual_resource_hints.credit');
  }
  if (resourceType === 'balance') {
    return t('channel.edit.billing.manual_resource_hints.balance');
  }
  if (resourceType === 'quota') {
    const quotaType = normalizeBillingValue(item?.quota_type);
    if (quotaType === 'daily' || quotaType === 'weekly' || quotaType === 'monthly') {
      return t('channel.edit.billing.manual_resource_hints.periodic');
    }
    if (quotaType === 'total') {
      return t('channel.edit.billing.manual_resource_hints.total');
    }
    return t('channel.edit.billing.manual_resource_hints.quota');
  }
  return t('channel.edit.billing.manual_resource_hints.default');
};

const resolveManualAmountLabel = (item, t) => {
  const resourceType = normalizeBillingValue(item?.resource_type);
  if (resourceType === 'plan') {
    return t('channel.edit.billing.manual_quota_expires_at');
  }
  if (resourceType === 'balance') {
    return t('channel.edit.billing.manual_balance_amount');
  }
  if (resourceType === 'credit') {
    return t('channel.edit.billing.manual_credit_amount');
  }
  if (isManualPeriodicItem(item)) {
    return t('channel.edit.billing.manual_periodic_quota_amount');
  }
  return t('channel.edit.billing.manual_quota_limit_amount');
};

const updateManualResourceTypeDefaults = (item, nextType) => {
  const resourceType = normalizeBillingValue(nextType) || 'quota';
  if (resourceType === 'plan') {
    return {
      resource_type: 'plan',
      quota_type: 'plan',
      amount: 0,
      limit_amount: 0,
      used_amount: 0,
      remaining_amount: 0,
      reset_at_input: '',
    };
  }
  if (resourceType === 'balance' || resourceType === 'credit') {
    return {
      resource_type: resourceType,
      quota_type: 'total',
      reset_at_input: '',
    };
  }
  return {
    resource_type: 'quota',
    quota_type: isPeriodicQuotaType(item?.quota_type) ? item.quota_type : 'total',
  };
};

const resolveManualItemAmounts = (item) => {
  if (isManualPlanItem(item)) {
    return {
      amount: 0,
      limit_amount: 0,
      used_amount: 0,
      remaining_amount: 0,
    };
  }
  const limitAmount = Number(item?.limit_amount || 0);
  const usedAmount = Number(item?.used_amount || 0);
  let remainingAmount = Number(item?.remaining_amount || 0);
  if (remainingAmount <= 0 && limitAmount > 0 && usedAmount >= 0) {
    remainingAmount = Math.max(limitAmount - usedAmount, 0);
  }
  const amount = remainingAmount > 0 ? remainingAmount : limitAmount;
  return {
    amount,
    limit_amount: limitAmount,
    used_amount: usedAmount,
    remaining_amount: remainingAmount,
  };
};

const classifyEntitlementItem = (item, t) => {
  const resourceType = normalizeBillingValue(item?.resource_type);
  const quotaType = normalizeBillingValue(item?.quota_type);
  if (resourceType === 'plan') {
    return {
      key: 'plan',
      color: 'purple',
      label: t('channel.edit.billing.entitlement_kinds.package'),
    };
  }
  if (isPeriodicQuotaType(quotaType)) {
    return {
      key: 'periodic',
      color: 'blue',
      label: t(`channel.edit.billing.quota_types.${quotaType}`, {
        defaultValue: quotaType,
      }),
    };
  }
  if (
    resourceType === 'balance' ||
    resourceType === 'credit' ||
    quotaType === 'total'
  ) {
    return {
      key: 'metered',
      color: 'cyan',
      label: t('channel.edit.billing.entitlement_kinds.metered'),
    };
  }
  return {
    key: 'custom',
    color: 'default',
    label: t('channel.edit.billing.entitlement_kinds.custom'),
  };
};

const summarizeEntitlementMode = (items, t) => {
  const rows = Array.isArray(items) ? items : [];
  const hasPlan = rows.some(
    (row) => normalizeBillingValue(row?.resource_type) === 'plan'
  );
  const hasPeriodic = rows.some((row) => isPeriodicQuotaType(row?.quota_type));
  const hasMetered = rows.some((row) => {
    const resourceType = normalizeBillingValue(row?.resource_type);
    const quotaType = normalizeBillingValue(row?.quota_type);
    return (
      resourceType === 'balance' ||
      resourceType === 'credit' ||
      quotaType === 'total'
    );
  });
  if (hasPlan || hasPeriodic) {
    return {
      kind: 'package',
      color: 'blue',
      label: t('channel.edit.billing.mode_summary.package_title'),
      description: t('channel.edit.billing.mode_summary.package_description'),
    };
  }
  if (hasMetered) {
    return {
      kind: 'metered',
      color: 'cyan',
      label: t('channel.edit.billing.mode_summary.metered_title'),
      description: t('channel.edit.billing.mode_summary.metered_description'),
    };
  }
  return {
    kind: 'unknown',
    color: 'default',
    label: t('channel.edit.billing.mode_summary.unknown_title'),
    description: t('channel.edit.billing.mode_summary.unknown_description'),
  };
};

const formatExpiresAtText = (item, timestamp2string, t) => {
  const expiresAt = Number(item?.expires_at || 0);
  if (expiresAt <= 0) {
    return t('channel.edit.billing.no_expire');
  }
  return timestamp2string(expiresAt);
};

const formatValidityText = (item, timestamp2string, t) => {
  const expiresAt = Number(item?.expires_at || 0);
  const resetAt = Number(item?.reset_at || 0);
  const parts = [];
  if (expiresAt > 0) {
    parts.push(
      `${t('channel.edit.billing.quota_table.valid_until')}: ${timestamp2string(
        expiresAt
      )}`
    );
  }
  if (resetAt > 0) {
    parts.push(
      `${t('channel.edit.billing.quota_table.next_reset')}: ${timestamp2string(
        resetAt
      )}`
    );
  }
  if (parts.length === 0) {
    return t('channel.edit.billing.no_expire');
  }
  return parts.join(' / ');
};

const formatUsageText = (item) => {
  const remaining = Number(item?.remaining_amount || 0);
  const limit = Number(item?.limit_amount || 0);
  const currency = (item?.currency || '').toString().trim();
  if (limit > 0) {
    return `${remaining} / ${limit}${currency ? ` ${currency}` : ''}`;
  }
  return formatAmountText({
    amount: remaining || item?.amount || 0,
    currency,
  });
};

const formatEntitlementUsageText = (item, t) => {
  if (isPlanEntitlement(item)) {
    return formatItemStatusText(item, t);
  }
  return formatUsageText(item);
};

const formatUsedText = (item) => {
  if (isPlanEntitlement(item)) {
    return '-';
  }
  const used = Number(item?.used_amount || 0);
  const currency = (item?.currency || '').toString().trim();
  if (used <= 0) {
    return '-';
  }
  return `${used}${currency ? ` ${currency}` : ''}`;
};

const formatRemainingRatioText = (item) => {
  if (isPlanEntitlement(item)) {
    return '-';
  }
  const limit = Number(item?.limit_amount || 0);
  const remaining = Number(item?.remaining_amount || 0);
  if (!(limit > 0)) {
    return '-';
  }
  return `${((remaining / limit) * 100).toFixed(2)}%`;
};

const formatItemStatusText = (item, t) => {
  const status = (item?.status || '').toString().trim().toLowerCase();
  switch (status) {
    case 'low':
      return t('channel.edit.billing.quota_table.status_low');
    case 'depleted':
      return t('channel.edit.billing.quota_table.status_depleted');
    case 'expired':
      return t('channel.edit.billing.quota_table.status_expired');
    case 'active':
    default:
      return t('channel.edit.billing.quota_table.status_active');
  }
};

const statusColor = (item) => {
  const status = normalizeBillingValue(item?.status);
  switch (status) {
    case 'low':
      return 'orange';
    case 'depleted':
    case 'expired':
      return 'red';
    case 'active':
    default:
      return 'green';
  }
};

const renderEntitlementKind = (row, t) => {
  const kind = classifyEntitlementItem(row, t);
  return <AppTag color={kind.color}>{kind.label}</AppTag>;
};

const renderQuotaLabel = (value, row, t) => {
  return (
    value ||
    t(`channel.edit.billing.quota_types.${row?.quota_type || 'custom'}`, {
      defaultValue: row?.quota_type || '-',
    })
  );
};

const renderStatus = (row, t) => (
  <AppTag color={statusColor(row)}>{formatItemStatusText(row, t)}</AppTag>
);

const formatNumberText = (value, digits = 6) => {
  const amount = Number(value || 0);
  if (!Number.isFinite(amount)) {
    return '-';
  }
  return Number(amount.toFixed(digits)).toString();
};

const formatProcurementCapacityText = (row) => {
  const remaining = formatNumberText(row?.capacity_remaining, 6);
  const effective = formatNumberText(
    row?.capacity_effective || row?.capacity_total,
    6
  );
  const unit = (row?.capacity_unit || '').toString().trim();
  return `${remaining} / ${effective}${unit ? ` ${unit}` : ''}`;
};

const formatProcurementCostText = (row, t) => {
  const source = (row?.cost_source || '').toString().trim();
  const status = (row?.cost_status || '').toString().trim();
  if (status === 'cost_unconfigured' || source === 'none' || source === '') {
    return t('channel.edit.billing.procurement_table.cost_unconfigured');
  }
  return `${formatNumberText(row?.purchase_cost_amount, 6)} CNY`;
};

const formatProcurementUnitCostText = (row) => {
  const unit = (row?.capacity_unit || '').toString().trim();
  const value = formatNumberText(row?.cost_per_unit_amount, 8);
  if (value === '-') {
    return '-';
  }
  return unit ? `${value} CNY/${unit}` : `${value} CNY`;
};

const formatProcurementScopeText = (row, t) => {
  const scopeType = normalizeBillingValue(row?.scope_type || 'global') || 'global';
  const scopeValue = (row?.scope_value || '').toString().trim();
  if (scopeType === 'model') {
    return scopeValue || '-';
  }
  return t('channel.edit.billing.procurement_scopes.global');
};

const formatProcurementResourceText = (row, t) => {
  const resourceType = normalizeBillingValue(row?.resource_type);
  const quotaType = normalizeBillingValue(row?.quota_type);
  const resourceLabel = t(
    `channel.edit.billing.resource_types.${resourceType || 'unknown'}`,
    { defaultValue: resourceType || '-' }
  );
  const quotaLabel =
    quotaType && quotaType !== resourceType
      ? t(`channel.edit.billing.quota_types.${quotaType}`, {
          defaultValue: quotaType,
        })
      : '';
  return quotaLabel ? `${resourceLabel} / ${quotaLabel}` : resourceLabel;
};

const formatProcurementSourceText = (row) => {
  const sourceRef = (row?.source_ref || '').toString().trim();
  const snapshotID = (row?.source_snapshot_id || '').toString().trim();
  if (sourceRef !== '') {
    return sourceRef;
  }
  if (snapshotID !== '') {
    return snapshotID.slice(0, 8);
  }
  return '-';
};

const procurementStatusColor = (status) => {
  switch ((status || '').toString().trim()) {
    case 'active':
      return 'green';
    case 'cost_unconfigured':
      return 'orange';
    case 'exhausted':
    case 'expired':
      return 'red';
    case 'disabled':
      return 'default';
    default:
      return 'default';
  }
};

const buildProcurementCostDraft = (row) => ({
  purchase_currency:
    (row?.purchase_currency || 'CNY').toString().trim() || 'CNY',
  purchase_amount: Number(row?.purchase_amount || 0),
  purchase_fx_rate: Number(row?.purchase_fx_rate || 1) || 1,
  purchase_cost_amount: Number(row?.purchase_cost_amount || 0),
  capacity_effective: Number(
    row?.capacity_effective || row?.capacity_total || 0
  ),
  cost_source:
    (row?.cost_source || 'actual').toString().trim() === 'none'
      ? 'actual'
      : (row?.cost_source || 'actual').toString().trim(),
  cost_status: 'active',
  scope_type: (row?.scope_type || 'global').toString().trim() || 'global',
  scope_value: (row?.scope_value || '').toString().trim(),
});

const toUnixTimestamp = (value) => {
	const normalized = (value || '').toString().trim();
	if (normalized === '') {
		return 0;
	}
  const parsed = new Date(normalized);
  const millis = parsed.getTime();
  if (!Number.isFinite(millis) || Number.isNaN(millis)) {
    return 0;
  }
	return Math.floor(millis / 1000);
};

const toDateTimeLocalValueFromTimestamp = (value) => {
	const timestamp = Number(value || 0);
	if (timestamp <= 0) {
		return '';
	}
	return toDateTimeLocalValue(new Date(timestamp * 1000));
};

const buildManualPurchaseRecordFromSnapshot = (row) => ({
	purchase_at_input:
		toDateTimeLocalValueFromTimestamp(row?.purchase_at) ||
		buildManualPurchaseRecord().purchase_at_input,
	purchase_currency:
		(row?.purchase_currency || 'CNY').toString().trim().toUpperCase() || 'CNY',
	purchase_amount: Number(row?.purchase_amount || 0),
	purchase_fx_rate: Number(row?.purchase_fx_rate || 1) || 1,
	purchase_cost_amount: Number(row?.purchase_cost_amount || 0),
});

const buildManualQuotaItemFromSnapshotItem = (item) => ({
	resource_type: (item?.resource_type || 'quota').toString().trim() || 'quota',
	quota_type: (item?.quota_type || 'total').toString().trim() || 'total',
	quota_label: (item?.quota_label || '').toString(),
	amount: Number(item?.amount || item?.remaining_amount || 0),
	limit_amount: Number(item?.limit_amount || item?.amount || 0),
	used_amount: Number(item?.used_amount || 0),
	remaining_amount: Number(item?.remaining_amount || item?.amount || 0),
	currency: (item?.currency || 'USD').toString().trim() || 'USD',
	reset_at_input: toDateTimeLocalValueFromTimestamp(item?.reset_at),
	expires_at_input: toDateTimeLocalValueFromTimestamp(item?.expires_at),
	source_ref: (item?.source_ref || 'manual').toString().trim() || 'manual',
});

const isPurchaseCurrencyCNY = (record) =>
	(record?.purchase_currency || '').toString().trim().toUpperCase() === 'CNY';

const ChannelDetailBillingTab = ({
  t,
  billingSummary,
  billingLoading,
  billingError,
  billingSnapshots,
  procurementBatches,
  billingReadonly,
  billingSubmitting,
  onRefreshBilling,
  onOpenActivatePage,
  onManualSnapshotUpdate,
  onManualSnapshotDelete,
  onProcurementBatchCostUpdate,
  onProcurementBatchStatusUpdate,
  onProcurementBatchConsumptionsLoad,
  timestamp2string,
}) => {
  const [activateCredential, setActivateCredential] = useState('');
  const [manualPurchaseRecord, setManualPurchaseRecord] = useState(
    buildManualPurchaseRecord()
  );
  const [manualMessage, setManualMessage] = useState('');
  const [manualItems, setManualItems] = useState([buildManualQuotaItem()]);
  const [manualModalOpen, setManualModalOpen] = useState(false);
  const [editingPurchaseRecord, setEditingPurchaseRecord] = useState(null);
  const [costModalOpen, setCostModalOpen] = useState(false);
  const [editingProcurementBatch, setEditingProcurementBatch] = useState(null);
  const [costDraft, setCostDraft] = useState(buildProcurementCostDraft(null));
  const [consumptionModalOpen, setConsumptionModalOpen] = useState(false);
  const [consumptionRows, setConsumptionRows] = useState([]);
  const [consumptionLoading, setConsumptionLoading] = useState(false);
  const [viewingProcurementBatch, setViewingProcurementBatch] = useState(null);

  const purchaseRecords = useMemo(
    () =>
      (Array.isArray(billingSnapshots) ? billingSnapshots : []).filter(
        (snapshot) =>
          (snapshot?.source_type || '').toString().trim() === 'manual'
      ),
    [billingSnapshots]
  );
  const quotaItems = Array.isArray(billingSummary?.quota_items)
    ? billingSummary.quota_items
    : [];
  const procurementRows = Array.isArray(procurementBatches)
    ? procurementBatches
    : [];
  const latestSnapshotStatus = normalizeBillingValue(
    billingSummary?.latest_snapshot_status
  );
  const latestSnapshotMessage = (billingSummary?.latest_snapshot_message || '')
    .toString()
    .trim();
  const entitlementModeSummary = summarizeEntitlementMode(quotaItems, t);

  const appendManualItem = () => {
    setManualItems((prev) => [...prev, buildManualQuotaItem()]);
  };

  const appendManualPlanQuotaItem = (planItem) => {
    setManualItems((prev) => [
      ...prev,
      {
        ...buildManualQuotaItem(),
        quota_type: 'daily',
        expires_at_input: planItem?.expires_at_input || '',
        source_ref: planItem?.source_ref || 'manual',
      },
    ]);
  };

  const removeManualItem = (index) => {
    setManualItems((prev) => {
      if (prev.length <= 1) {
        return [buildManualQuotaItem()];
      }
      return prev.filter((_, itemIndex) => itemIndex !== index);
    });
  };

  const updateManualItem = (index, patch) => {
    setManualItems((prev) =>
      prev.map((item, itemIndex) =>
        itemIndex === index
          ? {
              ...item,
              ...patch,
            }
          : item
      )
    );
  };

  const updateManualPurchaseRecord = (patch) => {
    setManualPurchaseRecord((prev) => ({
      ...prev,
      ...(patch || {}),
    }));
  };

  const closeManualModal = () => {
    if (!billingSubmitting) {
      setManualModalOpen(false);
      setEditingPurchaseRecord(null);
    }
  };

  const openCreateManualModal = () => {
    setEditingPurchaseRecord(null);
    setManualPurchaseRecord(buildManualPurchaseRecord());
    setManualMessage('');
    setManualItems([buildManualQuotaItem()]);
    setManualModalOpen(true);
  };

  const openEditManualModal = (row) => {
    setEditingPurchaseRecord(row);
    setManualPurchaseRecord(buildManualPurchaseRecordFromSnapshot(row));
    setManualMessage((row?.message || '').toString());
    const items = Array.isArray(row?.items) ? row.items : [];
    setManualItems(
      items.length > 0
        ? items.map((item) => buildManualQuotaItemFromSnapshotItem(item))
        : [buildManualQuotaItem()]
    );
    setManualModalOpen(true);
  };

  const openCostModal = (row) => {
    setEditingProcurementBatch(row);
    setCostDraft(buildProcurementCostDraft(row));
    setCostModalOpen(true);
  };

  const closeCostModal = () => {
    if (!billingSubmitting) {
      setCostModalOpen(false);
      setEditingProcurementBatch(null);
    }
  };

  const updateCostDraft = (patch) => {
    setCostDraft((prev) => ({
      ...prev,
      ...(patch || {}),
    }));
  };

  const openConsumptionModal = async (row) => {
    setViewingProcurementBatch(row);
    setConsumptionRows([]);
    setConsumptionModalOpen(true);
    setConsumptionLoading(true);
    try {
      const rows = await onProcurementBatchConsumptionsLoad(row?.id);
      setConsumptionRows(Array.isArray(rows) ? rows : []);
    } finally {
      setConsumptionLoading(false);
    }
  };

  const closeConsumptionModal = () => {
    if (!consumptionLoading) {
      setConsumptionModalOpen(false);
      setViewingProcurementBatch(null);
      setConsumptionRows([]);
    }
  };

  const submitManualSnapshot = async () => {
    const purchaseAmount = Number(manualPurchaseRecord.purchase_amount || 0);
    const purchaseCurrency = (manualPurchaseRecord.purchase_currency || 'CNY')
      .toString()
      .trim()
      .toUpperCase();
    const purchaseFXRate = purchaseCurrency === 'CNY'
      ? 1
      : Number(manualPurchaseRecord.purchase_fx_rate || 0);
    const purchaseCostAmount = purchaseCurrency === 'CNY'
      ? purchaseAmount
      : Number(manualPurchaseRecord.purchase_cost_amount || 0);
    const saved = await onManualSnapshotUpdate({
      id: editingPurchaseRecord?.id || '',
      purchase_at: toUnixTimestamp(manualPurchaseRecord.purchase_at_input),
      purchase_currency: purchaseCurrency,
      purchase_amount: purchaseAmount,
      purchase_fx_rate: purchaseFXRate,
      purchase_cost_amount: purchaseCostAmount,
      items: manualItems.map((manualItem) => {
        const amounts = resolveManualItemAmounts(manualItem);
        return {
          resource_type: manualItem.resource_type,
          quota_type: manualItem.quota_type,
          quota_label: manualItem.quota_label,
          ...amounts,
          currency: manualItem.currency,
          reset_at: 0,
          expires_at: toUnixTimestamp(manualItem.expires_at_input),
          source_ref: manualItem.source_ref,
        };
      }),
      message: manualMessage,
    });
    if (saved) {
      setManualModalOpen(false);
      setEditingPurchaseRecord(null);
      setManualPurchaseRecord(buildManualPurchaseRecord());
      setManualMessage('');
      setManualItems([buildManualQuotaItem()]);
    }
  };

  const deleteManualSnapshot = async (row) => {
    if (!row?.id || !onManualSnapshotDelete) {
      return;
    }
    await onManualSnapshotDelete(row.id);
  };

  const submitProcurementBatchCost = async () => {
    if (!editingProcurementBatch?.id) {
      return;
    }
    const saved = await onProcurementBatchCostUpdate(
      editingProcurementBatch.id,
      costDraft
    );
    if (saved) {
      closeCostModal();
    }
  };

  const updateProcurementBatchStatus = async (row, status) => {
    if (!row?.id) {
      return;
    }
    await onProcurementBatchStatusUpdate(row.id, status);
  };

  const renderManualSnapshotForm = () => (
    <div>
      <div className='router-billing-manual-item-card'>
        <div className='router-billing-manual-item-header'>
          <div className='router-billing-manual-item-title'>
            {t('channel.edit.billing.manual_purchase_title')}
            <span className='router-billing-manual-item-title-hint'>
              （{t('channel.edit.billing.manual_purchase_hint')}）
            </span>
          </div>
        </div>
        <AppFormRow>
          <AppField label={t('channel.edit.billing.manual_purchase_at')} required>
            <AppInput
              className='router-section-input'
              type='datetime-local'
              value={manualPurchaseRecord.purchase_at_input}
              onChange={(e, { value }) =>
                updateManualPurchaseRecord({
                  purchase_at_input: (value || '').toString(),
                })
              }
              readOnly={billingReadonly || billingSubmitting}
            />
          </AppField>
          <AppField label={t('channel.edit.billing.manual_purchase_currency')} required>
            <AppSelect
              className='router-section-input'
              options={ensureUnitOption(
                PROCUREMENT_CURRENCY_OPTIONS,
                manualPurchaseRecord.purchase_currency || 'CNY'
              )}
              value={manualPurchaseRecord.purchase_currency || 'CNY'}
              onChange={(e, { value }) => {
                const nextCurrency = (value || 'CNY')
                  .toString()
                  .trim()
                  .toUpperCase();
                updateManualPurchaseRecord({
                  purchase_currency: nextCurrency,
                  purchase_fx_rate:
                    nextCurrency === 'CNY'
                      ? 1
                      : manualPurchaseRecord.purchase_fx_rate,
                  purchase_cost_amount:
                    nextCurrency === 'CNY'
                      ? Number(manualPurchaseRecord.purchase_amount || 0)
                      : manualPurchaseRecord.purchase_cost_amount,
                });
              }}
              disabled={billingReadonly || billingSubmitting}
            />
          </AppField>
          <AppField label={t('channel.edit.billing.manual_purchase_amount')} required>
            <AppInputNumber
              className='router-section-input'
              fluid
              min={0}
              value={manualPurchaseRecord.purchase_amount}
              onChange={(e, { value }) =>
                updateManualPurchaseRecord({
                  purchase_amount: Number(value || 0),
                  purchase_cost_amount: isPurchaseCurrencyCNY(manualPurchaseRecord)
                    ? Number(value || 0)
                    : Number(value || 0) *
                      Number(manualPurchaseRecord.purchase_fx_rate || 0),
                })
              }
              disabled={billingReadonly || billingSubmitting}
            />
          </AppField>
          {!isPurchaseCurrencyCNY(manualPurchaseRecord) && (
            <>
              <AppField label={t('channel.edit.billing.manual_purchase_fx_rate')} required>
                <AppInputNumber
                  className='router-section-input'
                  fluid
                  min={0}
                  value={manualPurchaseRecord.purchase_fx_rate}
                  onChange={(e, { value }) =>
                    updateManualPurchaseRecord({
                      purchase_fx_rate: Number(value || 0),
                      purchase_cost_amount:
                        Number(manualPurchaseRecord.purchase_amount || 0) *
                        Number(value || 0),
                    })
                  }
                  disabled={billingReadonly || billingSubmitting}
                />
              </AppField>
              <AppField label={t('channel.edit.billing.manual_purchase_cost_amount')} required>
                <AppInputNumber
                  className='router-section-input'
                  fluid
                  min={0}
                  value={manualPurchaseRecord.purchase_cost_amount}
                  onChange={(e, { value }) =>
                    updateManualPurchaseRecord({
                      purchase_cost_amount: Number(value || 0),
                    })
                  }
                  disabled={billingReadonly || billingSubmitting}
                />
              </AppField>
            </>
          )}
        </AppFormRow>
      </div>
      {manualItems.map((item, index) => (
        <div
          key={`manual-quota-${index}`}
          className='router-billing-manual-item-card'
        >
          <div className='router-billing-manual-item-header'>
            <div className='router-billing-manual-item-title'>
              {t('channel.edit.billing.manual_item_title', {
                index: index + 1,
              })}
              <AppTooltip title={resolveManualResourceHint(item, t)}>
                <span className='router-help-trigger router-billing-manual-item-help'>
                  ?
                </span>
              </AppTooltip>
            </div>
            <div className='router-billing-manual-item-actions'>
              {isManualPlanItem(item) ? (
                <AppButton
                  type='button'
                  className='router-page-button'
                  basic
                  disabled={billingReadonly || billingSubmitting}
                  onClick={() => appendManualPlanQuotaItem(item)}
                >
                  {t('channel.edit.billing.add_plan_quota_item')}
                </AppButton>
              ) : null}
              <AppButton
                type='button'
                className='router-page-button'
                basic
                danger
                disabled={billingReadonly || billingSubmitting}
                onClick={() => removeManualItem(index)}
              >
                {t('channel.edit.billing.remove_quota_item')}
              </AppButton>
            </div>
          </div>
          <AppFormRow>
          <AppField
            label={t('channel.edit.billing.manual_resource_type')}
            required
          >
            <AppSelect
              className='router-section-input'
              options={resourceTypeOptions(t)}
              value={item.resource_type}
              onChange={(e, { value }) =>
                updateManualItem(index, {
                  ...updateManualResourceTypeDefaults(item, value),
                })
              }
              disabled={billingReadonly || billingSubmitting}
            />
          </AppField>
          {shouldShowManualQuotaType(item) ? (
            <AppField label={t('channel.edit.billing.manual_quota_type')}>
              <AppSelect
                className='router-section-input'
                options={quotaTypeOptions(t)}
                value={item.quota_type}
                onChange={(e, { value }) =>
                  updateManualItem(index, {
                    quota_type: (value || 'custom').toString(),
                  })
                }
                disabled={billingReadonly || billingSubmitting}
              />
            </AppField>
          ) : null}
          <AppField
            label={
              normalizeBillingValue(item.resource_type) === 'plan'
                ? t('channel.edit.billing.manual_plan_name')
                : t('channel.edit.billing.manual_quota_label')
            }
            required
          >
            <AppInput
              className='router-section-input'
              value={item.quota_label}
              onChange={(e, { value }) =>
                updateManualItem(index, {
                  quota_label: (value || '').toString(),
                })
              }
              readOnly={billingReadonly || billingSubmitting}
              placeholder={
                normalizeBillingValue(item.resource_type) === 'plan'
                  ? t('channel.edit.billing.manual_plan_name_placeholder')
                  : ''
              }
            />
          </AppField>
          {normalizeBillingValue(item.resource_type) === 'plan' ? (
            <>
              <AppField label={t('channel.edit.billing.manual_quota_expires_at')} required>
                <AppInput
                  className='router-section-input'
                  type='datetime-local'
                  value={item.expires_at_input}
                  onChange={(e, { value }) =>
                    updateManualItem(index, {
                      expires_at_input: (value || '').toString(),
                    })
                  }
                  readOnly={billingReadonly || billingSubmitting}
                />
              </AppField>
              <AppField label={t('channel.edit.billing.manual_source_ref')}>
                <AppInput
                  className='router-section-input'
                  value={item.source_ref}
                  onChange={(e, { value }) =>
                    updateManualItem(index, {
                      source_ref: (value || '').toString(),
                    })
                  }
                  readOnly={billingReadonly || billingSubmitting}
                />
              </AppField>
            </>
          ) : (
            <>
              {shouldShowManualAmountFields(item) ? (
                <>
                  <AppField
                    label={
                      resolveManualAmountLabel(item, t)
                    }
                    required
                  >
                    <AppCompact className='router-section-input-with-unit' block>
                      <AppInputNumber
                        className='router-section-input router-section-input-with-unit-field'
                        fluid
                        value={item.limit_amount}
                        min={0}
                        onChange={(e, { value }) =>
                          updateManualItem(index, {
                            limit_amount: value,
                          })
                        }
                        disabled={billingReadonly || billingSubmitting}
                      />
                      <UnitDropdown
                        variant='inputUnit'
                        options={ensureUnitOption(
                          MANUAL_CURRENCY_OPTIONS,
                          item.currency || 'USD'
                        )}
                        value={item.currency || 'USD'}
                        onChange={(_, { value }) =>
                          updateManualItem(index, {
                            currency: (value || 'USD')
                              .toString()
                              .trim()
                              .toUpperCase(),
                          })
                        }
                        disabled={billingReadonly || billingSubmitting}
                        aria-label={t('channel.edit.billing.currency')}
                      />
                    </AppCompact>
                  </AppField>
                  {shouldShowManualRemainingFields(item) ? (
                    <>
                      <AppField
                        label={t('channel.edit.billing.manual_quota_used_amount')}
                      >
                        <AppInputNumber
                          className='router-section-input'
                          fluid
                          value={item.used_amount}
                          min={0}
                          onChange={(e, { value }) =>
                            updateManualItem(index, {
                              used_amount: value,
                            })
                          }
                          disabled={billingReadonly || billingSubmitting}
                        />
                      </AppField>
                      <AppField
                        label={t('channel.edit.billing.manual_quota_remaining_amount')}
                        required
                      >
                        <AppInputNumber
                          className='router-section-input'
                          fluid
                          value={item.remaining_amount}
                          min={0}
                          onChange={(e, { value }) =>
                            updateManualItem(index, {
                              remaining_amount: value,
                            })
                          }
                          disabled={billingReadonly || billingSubmitting}
                        />
                      </AppField>
                    </>
                  ) : null}
                </>
              ) : null}
              <AppField
                label={t('channel.edit.billing.manual_quota_expires_at')}
                required={isManualPeriodicItem(item)}
              >
                <AppInput
                  className='router-section-input'
                  type='datetime-local'
                  value={item.expires_at_input}
                  onChange={(e, { value }) =>
                    updateManualItem(index, {
                      expires_at_input: (value || '').toString(),
                    })
                  }
                  readOnly={billingReadonly || billingSubmitting}
                />
              </AppField>
              <AppField label={t('channel.edit.billing.manual_source_ref')}>
                <AppInput
                  className='router-section-input'
                  value={item.source_ref}
                  onChange={(e, { value }) =>
                    updateManualItem(index, {
                      source_ref: (value || '').toString(),
                    })
                  }
                  readOnly={billingReadonly || billingSubmitting}
                />
              </AppField>
            </>
          )}
          </AppFormRow>
        </div>
      ))}
      <AppFormRow>
        <AppField label={t('channel.edit.billing.message')}>
          <AppInput
            className='router-section-input'
            value={manualMessage}
            onChange={(e, { value }) =>
              setManualMessage((value || '').toString())
            }
            readOnly={billingReadonly || billingSubmitting}
          />
        </AppField>
      </AppFormRow>
      <div className='router-detail-actions'>
        <AppButton
          type='button'
          className='router-page-button'
          basic
          disabled={billingReadonly || billingSubmitting}
          onClick={appendManualItem}
        >
          {t('channel.edit.billing.add_quota_item')}
        </AppButton>
      </div>
    </div>
  );

  const renderProcurementCostForm = () => (
    <div>
      <AppFormRow>
        <AppField
          label={t('channel.edit.billing.procurement_table.capacity')}
          required
        >
          <AppInputNumber
            className='router-section-input'
            fluid
            min={0}
            value={costDraft.capacity_effective}
            onChange={(e, { value }) =>
              updateCostDraft({
                capacity_effective: Number(value || 0),
              })
            }
            disabled={billingReadonly || billingSubmitting}
          />
        </AppField>
        <AppField
          label={t('channel.edit.billing.procurement_table.purchase_currency')}
          required
        >
          <AppSelect
            className='router-section-input'
            options={ensureUnitOption(
              PROCUREMENT_CURRENCY_OPTIONS,
              costDraft.purchase_currency || 'CNY'
            )}
            value={costDraft.purchase_currency || 'CNY'}
            onChange={(e, { value }) =>
              updateCostDraft({
                purchase_currency: (value || 'CNY')
                  .toString()
                  .trim()
                  .toUpperCase(),
              })
            }
            disabled={billingReadonly || billingSubmitting}
          />
        </AppField>
        <AppField
          label={t('channel.edit.billing.procurement_table.purchase_amount')}
          required
        >
          <AppInputNumber
            className='router-section-input'
            fluid
            min={0}
            value={costDraft.purchase_amount}
            onChange={(e, { value }) =>
              updateCostDraft({
                purchase_amount: Number(value || 0),
              })
            }
            disabled={billingReadonly || billingSubmitting}
          />
        </AppField>
      </AppFormRow>
      <AppFormRow>
        <AppField label={t('channel.edit.billing.procurement_table.scope_type')}>
          <AppSelect
            className='router-section-input'
            options={procurementScopeOptions(t)}
            value={costDraft.scope_type || 'global'}
            onChange={(e, { value }) =>
              updateCostDraft({
                scope_type: (value || 'global').toString().trim(),
                scope_value:
                  (value || 'global').toString().trim() === 'global'
                    ? ''
                    : costDraft.scope_value,
              })
            }
            disabled={billingReadonly || billingSubmitting}
          />
        </AppField>
        <AppField label={t('channel.edit.billing.procurement_table.scope_value')}>
          <AppInput
            className='router-section-input'
            value={costDraft.scope_value || ''}
            onChange={(e, { value }) =>
              updateCostDraft({
                scope_value: (value || '').toString(),
              })
            }
            readOnly={
              billingReadonly ||
              billingSubmitting ||
              (costDraft.scope_type || 'global') === 'global'
            }
            placeholder={t(
              'channel.edit.billing.procurement_table.scope_value_placeholder'
            )}
          />
        </AppField>
        <AppField
          label={t('channel.edit.billing.procurement_table.purchase_fx_rate')}
          required
        >
          <AppInputNumber
            className='router-section-input'
            fluid
            min={0}
            value={costDraft.purchase_fx_rate}
            onChange={(e, { value }) =>
              updateCostDraft({
                purchase_fx_rate: Number(value || 0),
              })
            }
            disabled={billingReadonly || billingSubmitting}
          />
        </AppField>
        <AppField
          label={t('channel.edit.billing.procurement_table.purchase_cost_amount')}
        >
          <AppInputNumber
            className='router-section-input'
            fluid
            min={0}
            value={costDraft.purchase_cost_amount}
            onChange={(e, { value }) =>
              updateCostDraft({
                purchase_cost_amount: Number(value || 0),
              })
            }
            disabled={billingReadonly || billingSubmitting}
          />
        </AppField>
      </AppFormRow>
      <AppAlert
        type='info'
        showIcon
        className='router-section-message'
        title={t('channel.edit.billing.procurement_cost_hint')}
      />
    </div>
  );

  return (
    <AppDetailSection
      title={t('channel.edit.billing.title')}
      titleTag='span'
      bodyClassName='router-billing-page'
    >
      <div className='router-billing-overview-strip'>
        <div className='router-billing-overview-main'>
          <AppTag color={entitlementModeSummary.color}>
            {entitlementModeSummary.label}
          </AppTag>
          <span>{entitlementModeSummary.description}</span>
        </div>
        <div className='router-billing-overview-meta'>
          <span>{t('channel.edit.billing.latest_snapshot_at')}</span>
          <strong>
            {billingSummary?.latest_snapshot_at
              ? timestamp2string(billingSummary.latest_snapshot_at)
              : '-'}
          </strong>
        </div>
      </div>
      <div>
        <AppDetailSection
          title={t('channel.edit.billing.current_quotas_title')}
          titleTag='span'
          headerEnd={
            <div className='router-billing-quota-status-actions'>
              <span className='router-billing-snapshot-time'>
                {billingSummary?.latest_snapshot_at
                  ? timestamp2string(billingSummary.latest_snapshot_at)
                  : '-'}
              </span>
              {billingSummary?.refresh_supported ? (
                <AppButton
                  type='button'
                  className='router-page-button'
                  color='blue'
                  loading={billingSubmitting}
                  disabled={billingSubmitting}
                  onClick={onRefreshBilling}
                >
                  {t('channel.edit.billing.refresh_now')}
                </AppButton>
              ) : null}
            </div>
          }
        >
          {latestSnapshotStatus === 'failed' ? (
            <AppAlert
              type='warning'
              showIcon
              className='router-section-message'
              title={t('channel.edit.billing.latest_refresh_failed', {
                message:
                  latestSnapshotMessage ||
                  t('channel.edit.billing.latest_refresh_failed_unknown'),
              })}
            />
          ) : null}
          <AppTable
            className='router-detail-table'
            pagination={false}
            loading={billingLoading}
            dataSource={quotaItems}
            rowKey={(row) => buildQuotaItemRowKey(row)}
            columns={[
              {
                title: t('channel.edit.billing.quota_table.entitlement_kind'),
                dataIndex: 'resource_type',
                key: 'entitlement_kind',
                width: 170,
                render: (_, row) => renderEntitlementKind(row, t),
              },
              {
                title: t('channel.edit.billing.quota_table.quota_label'),
                dataIndex: 'quota_label',
                key: 'quota_label',
                width: 180,
                render: (value, row) => renderQuotaLabel(value, row, t),
              },
              {
                title: t('channel.edit.billing.quota_table.amount'),
                dataIndex: 'remaining_amount',
                key: 'remaining_amount',
                width: 180,
                render: (_, row) => formatEntitlementUsageText(row, t),
              },
              {
                title: t('channel.edit.billing.quota_table.used_amount'),
                dataIndex: 'used_amount',
                key: 'used_amount',
                width: 120,
                render: (_, row) => formatUsedText(row),
              },
              {
                title: t('channel.edit.billing.quota_table.remaining_ratio'),
                dataIndex: 'limit_amount',
                key: 'remaining_ratio',
                width: 120,
                render: (_, row) => formatRemainingRatioText(row),
              },
              {
                title: t('channel.edit.billing.quota_table.validity'),
                dataIndex: 'expires_at',
                key: 'validity',
                width: 260,
                render: (_, row) =>
                  formatValidityText(row, timestamp2string, t),
              },
              {
                title: t('channel.edit.billing.quota_table.status'),
                dataIndex: 'status',
                key: 'status',
                width: 100,
                render: (_, row) => renderStatus(row, t),
              },
            ]}
            locale={{
              emptyText: t('channel.edit.billing.no_quota_items'),
            }}
          />
        </AppDetailSection>
        {billingSummary?.activate_supported && (
          <AppDetailSection
            title={t('channel.edit.billing.activate_title')}
            titleTag='span'
          >
            <AppFormRow>
              <AppField
                label={t('channel.edit.billing.activate_input')}
                required
              >
                <AppInput
                  className='router-section-input'
                  type='password'
                  value={activateCredential}
                  onChange={(e, { value }) =>
                    setActivateCredential((value || '').toString())
                  }
                  readOnly={billingReadonly || billingSubmitting}
                  autoComplete='new-password'
                />
              </AppField>
            </AppFormRow>
            <AppButton
              type='button'
              className='router-page-button'
              color='blue'
              loading={billingSubmitting}
              disabled={billingReadonly || billingSubmitting}
              onClick={() => onOpenActivatePage(activateCredential)}
            >
              {t('channel.edit.billing.open_activate_page')}
            </AppButton>
          </AppDetailSection>
        )}
        <AppDetailSection
          className='router-billing-management-section'
          title={t('channel.edit.billing.management_title')}
          titleTag='span'
        >
          <div className='router-billing-subsection-header'>
            <div>
              <div className='router-billing-subsection-title'>
                {t('channel.edit.billing.snapshots_title')}
              </div>
              <div className='router-billing-subsection-description'>
                {t('channel.edit.billing.snapshots_hint')}
              </div>
            </div>
            {billingSummary?.manual_update_supported ? (
              <AppButton
                type='button'
                className='router-page-button'
                color='blue'
                disabled={billingReadonly || billingSubmitting}
                onClick={openCreateManualModal}
              >
                {t('channel.edit.billing.add_purchase_record')}
              </AppButton>
            ) : null}
          </div>
          <AppTable
            className='router-detail-table'
            pagination={false}
            loading={billingLoading}
            dataSource={purchaseRecords}
            rowKey={(row) => row.id}
            columns={[
              {
                title: t('channel.edit.billing.snapshot_table.purchase_at'),
                dataIndex: 'purchase_at',
                key: 'purchase_at',
                width: 180,
                render: (value) => (value ? timestamp2string(value) : '-'),
              },
              {
                title: t('channel.edit.billing.snapshot_table.purchase_amount'),
                key: 'purchase_amount',
                width: 180,
                render: (_, row) =>
                  row?.purchase_amount
                    ? `${formatNumberText(row.purchase_amount, 6)} ${
                        row.purchase_currency || ''
                      }`.trim()
                    : '-',
              },
              {
                title: t('channel.edit.billing.snapshot_table.purchase_cost_amount'),
                dataIndex: 'purchase_cost_amount',
                key: 'purchase_cost_amount',
                width: 160,
                render: (value) =>
                  Number(value || 0) > 0
                    ? `${formatNumberText(value, 6)} CNY`
                    : '-',
              },
              {
                title: t('channel.edit.billing.snapshot_table.quota_items'),
                dataIndex: 'items',
                key: 'items',
                render: (items) =>
                  Array.isArray(items) && items.length > 0
                    ? items
                        .map(
                          (row) =>
                            `${
                              row.quota_label || row.quota_type
                            }: ${formatUsageText(row)}`
                        )
                        .join(' / ')
                    : '-',
              },
              {
                title: t('channel.edit.billing.snapshot_table.message'),
                dataIndex: 'message',
                key: 'message',
                render: (value) => value || '-',
              },
              {
                title: t('channel.edit.billing.snapshot_table.actions'),
                key: 'actions',
                width: 110,
                render: (_, row) => (
                  <AppCompact>
                    <AppTableActionButton
                      title={t('channel.edit.billing.edit_purchase_record')}
                      icon='edit'
                      disabled={billingReadonly || billingSubmitting}
                      onClick={() => openEditManualModal(row)}
                    />
                    <AppPopconfirm
                      title={t(
                        'channel.edit.billing.delete_purchase_record_confirm'
                      )}
                      okText={t('common.confirm')}
                      cancelText={t('common.cancel')}
                      onConfirm={() => deleteManualSnapshot(row)}
                    >
                      <span>
                        <AppTableActionButton
                          title={t(
                            'channel.edit.billing.delete_purchase_record'
                          )}
                          icon='trash'
                          color='red'
                          disabled={billingReadonly || billingSubmitting}
                        />
                      </span>
                    </AppPopconfirm>
                  </AppCompact>
                ),
              },
            ]}
          />
          <AppDivider className='router-billing-subsection-divider' />
          <div className='router-billing-subsection-header'>
            <div>
              <div className='router-billing-subsection-title'>
                {t('channel.edit.billing.procurement_title')}
              </div>
              <div className='router-billing-subsection-description'>
                {t('channel.edit.billing.procurement_hint')}
              </div>
            </div>
          </div>
          <AppTable
            className='router-detail-table'
            pagination={false}
            loading={billingLoading}
            dataSource={procurementRows}
            rowKey={(row) => row.id}
            columns={[
              {
                title: t('channel.edit.billing.procurement_table.resource'),
                dataIndex: 'resource_type',
                key: 'resource',
                width: 190,
                render: (_, row) => formatProcurementResourceText(row, t),
              },
              {
                title: t('channel.edit.billing.procurement_table.source'),
                dataIndex: 'source_ref',
                key: 'source',
                width: 150,
                render: (_, row) => formatProcurementSourceText(row),
              },
              {
                title: t('channel.edit.billing.procurement_table.capacity'),
                dataIndex: 'capacity_remaining',
                key: 'capacity',
                width: 220,
                render: (_, row) => formatProcurementCapacityText(row),
              },
              {
                title: t('channel.edit.billing.procurement_table.cost'),
                dataIndex: 'purchase_cost_amount',
                key: 'cost',
                width: 150,
                render: (_, row) => formatProcurementCostText(row, t),
              },
              {
                title: t('channel.edit.billing.procurement_table.unit_cost'),
                dataIndex: 'cost_per_unit_amount',
                key: 'unit_cost',
                width: 180,
                render: (_, row) => formatProcurementUnitCostText(row),
              },
              {
                title: t('channel.edit.billing.procurement_table.scope'),
                key: 'scope',
                width: 160,
                render: (_, row) => formatProcurementScopeText(row, t),
              },
              {
                title: t('channel.edit.billing.procurement_table.expire_at'),
                dataIndex: 'expire_at',
                key: 'expire_at',
                width: 180,
                render: (value) =>
                  Number(value || 0) > 0 ? timestamp2string(value) : '-',
              },
              {
                title: t('channel.edit.billing.procurement_table.status'),
                dataIndex: 'cost_status',
                key: 'cost_status',
                width: 120,
                render: (value) => (
                  <AppTag color={procurementStatusColor(value)}>
                    {t(
                      `channel.edit.billing.procurement_status.${
                        value || 'unknown'
                      }`,
                      {
                        defaultValue: value || '-',
                      }
                    )}
                  </AppTag>
                ),
              },
              {
                title: t('channel.edit.billing.procurement_table.actions'),
                key: 'actions',
                width: 150,
                render: (_, row) => (
                  <AppCompact>
                    <AppTableActionButton
                      title={t(
                        'channel.edit.billing.procurement_view_consumptions'
                      )}
                      icon='eye'
                      disabled={billingSubmitting}
                      onClick={() => openConsumptionModal(row)}
                    />
                    <AppTableActionButton
                      title={t('channel.edit.billing.procurement_edit_cost')}
                      icon='edit'
                      disabled={billingReadonly || billingSubmitting}
                      onClick={() => openCostModal(row)}
                    />
                    {(row?.cost_status || '').toString().trim() ===
                    'disabled' ? (
                      <AppPopconfirm
                        title={t(
                          'channel.edit.billing.procurement_restore_confirm'
                        )}
                        okText={t('common.confirm')}
                        cancelText={t('common.cancel')}
                        onConfirm={() =>
                          updateProcurementBatchStatus(row, 'active')
                        }
                      >
                        <AppTableActionButton
                          title={t('channel.edit.billing.procurement_restore')}
                          icon='check'
                          disabled={billingReadonly || billingSubmitting}
                        />
                      </AppPopconfirm>
                    ) : (
                      <AppPopconfirm
                        title={t(
                          'channel.edit.billing.procurement_disable_confirm'
                        )}
                        okText={t('common.confirm')}
                        cancelText={t('common.cancel')}
                        onConfirm={() =>
                          updateProcurementBatchStatus(row, 'disabled')
                        }
                      >
                        <AppTableActionButton
                          title={t('channel.edit.billing.procurement_disable')}
                          icon='close'
                          disabled={billingReadonly || billingSubmitting}
                        />
                      </AppPopconfirm>
                    )}
                  </AppCompact>
                ),
              },
            ]}
            locale={{
              emptyText: t('channel.edit.billing.no_procurement_batches'),
            }}
          />
        </AppDetailSection>
        <AppModal
          size='large'
          open={manualModalOpen}
          onClose={closeManualModal}
          title={t(
            editingPurchaseRecord?.id
              ? 'channel.edit.billing.edit_purchase_record'
              : 'channel.edit.billing.manual_update_title'
          )}
          footer={
            <AppFormActions>
              <AppButton
                type='button'
                disabled={billingSubmitting}
                onClick={closeManualModal}
              >
                {t('common.cancel')}
              </AppButton>
              <AppButton
                type='button'
                color='blue'
                loading={billingSubmitting}
                disabled={billingReadonly || billingSubmitting}
                onClick={submitManualSnapshot}
              >
                {t('channel.edit.billing.confirm_manual_snapshot')}
              </AppButton>
            </AppFormActions>
          }
        >
          {renderManualSnapshotForm()}
        </AppModal>
        <AppModal
          size='small'
          open={costModalOpen}
          onClose={closeCostModal}
          title={t('channel.edit.billing.procurement_edit_cost')}
          footer={
            <AppFormActions>
              <AppButton
                type='button'
                disabled={billingSubmitting}
                onClick={closeCostModal}
              >
                {t('common.cancel')}
              </AppButton>
              <AppButton
                type='button'
                color='blue'
                loading={billingSubmitting}
                disabled={billingReadonly || billingSubmitting}
                onClick={submitProcurementBatchCost}
              >
                {t('common.save')}
              </AppButton>
            </AppFormActions>
          }
        >
          {renderProcurementCostForm()}
        </AppModal>
        <AppModal
          size='large'
          open={consumptionModalOpen}
          onClose={closeConsumptionModal}
          title={t('channel.edit.billing.procurement_consumptions_title', {
            batch:
              viewingProcurementBatch?.source_ref ||
              viewingProcurementBatch?.id ||
              '-',
          })}
        >
          <AppTable
            className='router-detail-table'
            pagination={false}
            loading={consumptionLoading}
            dataSource={consumptionRows}
            rowKey={(row) => row.id}
            columns={[
              {
                title: t(
                  'channel.edit.billing.procurement_consumption_table.request_log'
                ),
                dataIndex: 'request_log_id',
                key: 'request_log_id',
                width: 220,
                render: (value) => value || '-',
              },
              {
                title: t(
                  'channel.edit.billing.procurement_consumption_table.quantity'
                ),
                dataIndex: 'consumed_quantity',
                key: 'consumed_quantity',
                width: 150,
                render: (value, row) =>
                  `${formatNumberText(value, 6)} ${
                    row?.capacity_unit || ''
                  }`.trim(),
              },
              {
                title: t(
                  'channel.edit.billing.procurement_consumption_table.unit_cost'
                ),
                dataIndex: 'unit_cost_amount',
                key: 'unit_cost_amount',
                width: 150,
                render: (value) => `${formatNumberText(value, 8)} CNY`,
              },
              {
                title: t(
                  'channel.edit.billing.procurement_consumption_table.cost'
                ),
                dataIndex: 'consumed_cost_amount',
                key: 'consumed_cost_amount',
                width: 150,
                render: (value) => `${formatNumberText(value, 6)} CNY`,
              },
              {
                title: t(
                  'channel.edit.billing.procurement_consumption_table.truth_mode'
                ),
                dataIndex: 'settlement_truth_mode',
                key: 'settlement_truth_mode',
                render: (value) => value || '-',
              },
              {
                title: t(
                  'channel.edit.billing.procurement_consumption_table.created_at'
                ),
                dataIndex: 'created_at',
                key: 'created_at',
                width: 180,
                render: (value) =>
                  Number(value || 0) > 0 ? timestamp2string(value) : '-',
              },
            ]}
            locale={{
              emptyText: t('channel.edit.billing.no_procurement_consumptions'),
            }}
          />
        </AppModal>
        {billingError && (
          <div className='router-error-text router-error-text-top'>
            {billingError}
          </div>
        )}
      </div>
    </AppDetailSection>
  );
};

export default ChannelDetailBillingTab;
