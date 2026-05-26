import React, { useMemo, useState } from 'react';
import {
  AppAlert,
  AppButton,
  AppDetailSection,
  AppField,
  AppFormActions,
  AppFormRow,
  AppInput,
  AppInputNumber,
  AppModal,
  AppSelect,
  AppTable,
} from '../../../router-ui';

const buildManualQuotaItem = () => ({
  resource_type: 'quota',
  quota_type: 'total',
  quota_label: '',
  amount: null,
  currency: 'USD',
  expires_at_input: '',
});

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

const formatResourceTypeText = (item, t) => {
  const resourceType = (item?.resource_type || '').toString().trim().toLowerCase();
  switch (resourceType) {
    case 'balance':
      return t('channel.edit.billing.resource_types.balance');
    case 'credit':
      return t('channel.edit.billing.resource_types.credit');
    case 'plan':
      return t('channel.edit.billing.resource_types.plan');
    case 'quota':
    default:
      return t('channel.edit.billing.resource_types.quota');
  }
};

const formatExpiresAtText = (item, timestamp2string, t) => {
  const expiresAt = Number(item?.expires_at || 0);
  if (expiresAt <= 0) {
    return t('channel.edit.billing.no_expire');
  }
  return timestamp2string(expiresAt);
};

const formatResetAtText = (item, timestamp2string, t) => {
  const resetAt = Number(item?.reset_at || 0);
  if (resetAt <= 0) {
    return '-';
  }
  return timestamp2string(resetAt);
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

const formatRemainingRatioText = (item) => {
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

const formatAlertTypeText = (row, t) => {
  const eventType = (row?.event_type || '').toString().trim().toLowerCase();
  switch (eventType) {
    case 'expiring_soon':
      return t('channel.edit.billing.alert_table.event_expiring_soon');
    case 'low_remaining':
      return t('channel.edit.billing.alert_table.event_low_remaining');
    default:
      return eventType || '-';
  }
};

const formatAlertStatusText = (row, t) => {
  const status = (row?.status || '').toString().trim().toLowerCase();
  switch (status) {
    case 'failed':
      return t('channel.edit.billing.alert_table.status_failed');
    case 'sent':
    default:
      return t('channel.edit.billing.alert_table.status_sent');
  }
};

const formatAlertQuotaLabelText = (row, t) => {
  const payload = row?.payload || {};
  const quotaLabel = (payload?.quota_label || '').toString().trim();
  if (quotaLabel !== '') {
    return quotaLabel;
  }
  const quotaType = (payload?.quota_type || '').toString().trim().toLowerCase();
  if (quotaType !== '') {
    return t(`channel.edit.billing.quota_types.${quotaType}`, {
      defaultValue: quotaType,
    });
  }
  return '-';
};

const formatAlertQuotaValueText = (row) => {
  const payload = row?.payload || {};
  const remaining = Number(payload?.remaining_amount || 0);
  const limit = Number(payload?.limit_amount || 0);
  const currency = (payload?.currency || '').toString().trim();
  if (limit > 0) {
    return `${remaining} / ${limit}${currency ? ` ${currency}` : ''}`;
  }
  if (remaining > 0) {
    return `${remaining}${currency ? ` ${currency}` : ''}`;
  }
  return '-';
};

const formatAlertRatioText = (row) => {
  const payload = row?.payload || {};
  const remaining = Number(payload?.remaining_amount || 0);
  const limit = Number(payload?.limit_amount || 0);
  if (!(limit > 0)) {
    return '-';
  }
  return `${((remaining / limit) * 100).toFixed(2)}%`;
};

const formatAlertExpiresAtText = (row, timestamp2string, t) => {
  const payload = row?.payload || {};
  return formatExpiresAtText(
    {
      expires_at: Number(payload?.expires_at || 0),
    },
    timestamp2string,
    t,
  );
};

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

const ChannelDetailBillingTab = ({
  t,
  billingSummary,
  billingLoading,
  billingError,
  billingSnapshots,
  billingAlerts,
  billingReadonly,
  billingSubmitting,
  onRefreshBilling,
  onOpenActivatePage,
  onManualSnapshotUpdate,
  timestamp2string,
}) => {
  const [activateCredential, setActivateCredential] = useState('');
  const [manualMessage, setManualMessage] = useState('');
  const [manualItems, setManualItems] = useState([buildManualQuotaItem()]);
  const [manualModalOpen, setManualModalOpen] = useState(false);

  const purchaseRecords = useMemo(
    () =>
      (Array.isArray(billingSnapshots) ? billingSnapshots : []).filter(
        (snapshot) => (snapshot?.source_type || '').toString().trim() === 'manual',
      ),
    [billingSnapshots],
  );
  const quotaItems = Array.isArray(billingSummary?.quota_items)
    ? billingSummary.quota_items
    : [];
  const alertRecords = Array.isArray(billingAlerts) ? billingAlerts : [];

  const appendManualItem = () => {
    setManualItems((prev) => [...prev, buildManualQuotaItem()]);
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
          : item,
      ),
    );
  };

  const closeManualModal = () => {
    if (!billingSubmitting) {
      setManualModalOpen(false);
    }
  };

  const submitManualSnapshot = async () => {
    const saved = await onManualSnapshotUpdate({
      items: manualItems.map((manualItem) => ({
        resource_type: manualItem.resource_type,
        quota_type: manualItem.quota_type,
        quota_label: manualItem.quota_label,
        amount: manualItem.amount,
        currency: manualItem.currency,
        expires_at: toUnixTimestamp(manualItem.expires_at_input),
      })),
      message: manualMessage,
    });
    if (saved) {
      setManualModalOpen(false);
      setManualMessage('');
      setManualItems([buildManualQuotaItem()]);
    }
  };

  const renderManualSnapshotForm = () => (
    <div>
      {manualItems.map((item, index) => (
        <AppFormRow key={`manual-quota-${index}`}>
          <AppField label={t('channel.edit.billing.manual_resource_type')} required>
            <AppSelect
              className='router-section-input'
              options={resourceTypeOptions(t)}
              value={item.resource_type}
              onChange={(e, { value }) =>
                updateManualItem(index, {
                  resource_type: (value || '').toString(),
                })
              }
              disabled={billingReadonly || billingSubmitting}
            />
          </AppField>
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
          <AppField label={t('channel.edit.billing.manual_quota_label')} required>
            <AppInput
              className='router-section-input'
              value={item.quota_label}
              onChange={(e, { value }) =>
                updateManualItem(index, {
                  quota_label: (value || '').toString(),
                })
              }
              readOnly={billingReadonly || billingSubmitting}
            />
          </AppField>
          <AppField label={t('channel.edit.billing.manual_quota_amount')} required>
            <AppInputNumber
              className='router-section-input'
              fluid
              value={item.amount}
              min={0}
              onChange={(e, { value }) =>
                updateManualItem(index, {
                  amount: value,
                })
              }
              disabled={billingReadonly || billingSubmitting}
            />
          </AppField>
          <AppField label={t('channel.edit.billing.currency')}>
            <AppInput
              className='router-section-input'
              value={item.currency}
              onChange={(e, { value }) =>
                updateManualItem(index, {
                  currency: (value || '').toString(),
                })
              }
              readOnly={billingReadonly || billingSubmitting}
            />
          </AppField>
          <AppField label={t('channel.edit.billing.manual_quota_expires_at')}>
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
          <AppField label={t('channel.edit.billing.row_action')}>
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
          </AppField>
        </AppFormRow>
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

  return (
    <AppDetailSection
      title={t('channel.edit.billing.title')}
      titleTag='span'
    >
      <div>
        <AppAlert
          type='info'
          showIcon
          className='router-section-message'
          title={t('channel.edit.billing.hint')}
        />
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
          <AppTable
            className='router-detail-table'
            pagination={false}
            loading={billingLoading}
            dataSource={quotaItems}
            rowKey={(row, index) => `${row?.quota_label || row?.quota_type || 'quota'}-${index}`}
            columns={[
              {
                title: t('channel.edit.billing.quota_table.resource_type'),
                dataIndex: 'resource_type',
                key: 'resource_type',
                width: 120,
                render: (_, row) => formatResourceTypeText(row, t),
              },
              {
                title: t('channel.edit.billing.quota_table.quota_label'),
                dataIndex: 'quota_label',
                key: 'quota_label',
                width: 180,
                render: (value, row) =>
                  value ||
                  t(`channel.edit.billing.quota_types.${row?.quota_type || 'custom'}`, {
                    defaultValue: row?.quota_type || '-',
                  }),
              },
              {
                title: t('channel.edit.billing.quota_table.amount'),
                dataIndex: 'remaining_amount',
                key: 'remaining_amount',
                width: 180,
                render: (_, row) => formatUsageText(row),
              },
              {
                title: t('channel.edit.billing.quota_table.remaining_ratio'),
                dataIndex: 'limit_amount',
                key: 'remaining_ratio',
                width: 120,
                render: (_, row) => formatRemainingRatioText(row),
              },
              {
                title: t('channel.edit.billing.quota_table.reset_at'),
                dataIndex: 'reset_at',
                key: 'reset_at',
                width: 180,
                render: (_, row) => formatResetAtText(row, timestamp2string, t),
              },
              {
                title: t('channel.edit.billing.quota_table.expires_at'),
                dataIndex: 'expires_at',
                key: 'expires_at',
                width: 180,
                render: (_, row) => formatExpiresAtText(row, timestamp2string, t),
              },
              {
                title: t('channel.edit.billing.quota_table.status'),
                dataIndex: 'status',
                key: 'status',
                width: 120,
                render: (_, row) => formatItemStatusText(row, t),
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
              <AppField label={t('channel.edit.billing.activate_input')} required>
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
          title={t('channel.edit.billing.snapshots_title')}
          titleTag='span'
          headerEnd={
            billingSummary?.manual_update_supported ? (
              <AppButton
                type='button'
                className='router-page-button'
                color='blue'
                disabled={billingReadonly || billingSubmitting}
                onClick={() => setManualModalOpen(true)}
              >
                {t('channel.edit.billing.add_purchase_record')}
              </AppButton>
            ) : null
          }
        >
          <AppTable
            className='router-detail-table'
            pagination={false}
            loading={billingLoading}
            dataSource={purchaseRecords}
            rowKey={(row) => row.id}
            columns={[
              {
                title: t('channel.edit.billing.snapshot_table.created_at'),
                dataIndex: 'created_at',
                key: 'created_at',
                width: 180,
                render: (value) => (value ? timestamp2string(value) : '-'),
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
                            `${row.quota_label || row.quota_type}: ${formatUsageText(row)}`,
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
            ]}
          />
        </AppDetailSection>
        <AppDetailSection
          title={t('channel.edit.billing.alerts_title')}
          titleTag='span'
        >
          <AppTable
            className='router-detail-table'
            pagination={false}
            loading={billingLoading}
            dataSource={alertRecords}
            rowKey={(row) => row.id}
            columns={[
              {
                title: t('channel.edit.billing.alert_table.created_at'),
                dataIndex: 'created_at',
                key: 'created_at',
                width: 180,
                render: (value) => (value ? timestamp2string(value) : '-'),
              },
              {
                title: t('channel.edit.billing.alert_table.event_type'),
                dataIndex: 'event_type',
                key: 'event_type',
                width: 160,
                render: (_, row) => formatAlertTypeText(row, t),
              },
              {
                title: t('channel.edit.billing.alert_table.quota_label'),
                dataIndex: 'payload',
                key: 'quota_label',
                width: 140,
                render: (_, row) => formatAlertQuotaLabelText(row, t),
              },
              {
                title: t('channel.edit.billing.alert_table.amount'),
                dataIndex: 'payload',
                key: 'amount',
                width: 180,
                render: (_, row) => formatAlertQuotaValueText(row),
              },
              {
                title: t('channel.edit.billing.alert_table.remaining_ratio'),
                dataIndex: 'payload',
                key: 'remaining_ratio',
                width: 120,
                render: (_, row) => formatAlertRatioText(row),
              },
              {
                title: t('channel.edit.billing.alert_table.expires_at'),
                dataIndex: 'payload',
                key: 'expires_at',
                width: 180,
                render: (_, row) =>
                  formatAlertExpiresAtText(row, timestamp2string, t),
              },
              {
                title: t('channel.edit.billing.alert_table.status'),
                dataIndex: 'status',
                key: 'status',
                width: 120,
                render: (_, row) => formatAlertStatusText(row, t),
              },
              {
                title: t('channel.edit.billing.alert_table.title'),
                dataIndex: 'title',
                key: 'title',
                render: (value) => value || '-',
              },
            ]}
            locale={{
              emptyText: t('channel.edit.billing.alert_table.empty'),
            }}
          />
        </AppDetailSection>
        <AppModal
          size='large'
          open={manualModalOpen}
          onClose={closeManualModal}
          title={t('channel.edit.billing.manual_update_title')}
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
