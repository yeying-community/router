import React, { useMemo, useState } from 'react';
import {
  AppAlert,
  AppButton,
  AppDetailSection,
  AppField,
  AppFormRow,
  AppInput,
  AppInputNumber,
  AppSelect,
  AppTable,
  AppTag,
} from '../../../router-ui';

const formatCapabilities = (capabilities, t) =>
  (Array.isArray(capabilities) ? capabilities : []).map((item) => ({
    key: item,
    text: t(`channel.edit.billing.capabilities.${item}`, { defaultValue: item }),
  }));

const buildManualQuotaItem = () => ({
  quota_type: 'total',
  quota_label: '',
  amount: null,
  currency: 'USD',
  expires_at_input: '',
});

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

const formatExpiresAtText = (item, timestamp2string, t) => {
  const expiresAt = Number(item?.expires_at || 0);
  if (expiresAt <= 0) {
    return t('channel.edit.billing.no_expire');
  }
  return timestamp2string(expiresAt);
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
  billingActions,
  billingReadonly,
  billingSubmitting,
  onOpenActivatePage,
  onManualSnapshotUpdate,
  timestamp2string,
}) => {
  const [cdk, setCDK] = useState('');
  const [manualMessage, setManualMessage] = useState('');
  const [manualItems, setManualItems] = useState([buildManualQuotaItem()]);

  const capabilityItems = useMemo(
    () => formatCapabilities(billingSummary?.action_capabilities, t),
    [billingSummary?.action_capabilities, t],
  );
  const quotaItems = Array.isArray(billingSummary?.quota_items)
    ? billingSummary.quota_items
    : [];

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

  return (
    <AppDetailSection title={t('channel.edit.billing.title')} titleTag='span'>
      <div>
        <AppAlert
          type='info'
          showIcon
          className='router-section-message'
          title={t('channel.edit.billing.hint')}
        />
        <AppFormRow>
          <AppField label={t('channel.edit.billing.updated_at')} readOnly>
            <AppInput
              className='router-section-input'
              value={
                billingSummary?.latest_snapshot_at
                  ? timestamp2string(billingSummary.latest_snapshot_at)
                  : '-'
              }
              readOnly
            />
          </AppField>
        </AppFormRow>
        <div className='router-block-gap-sm'>
          {capabilityItems.length > 0 ? (
            capabilityItems.map((item) => (
              <AppTag key={item.key} color='grey'>
                {item.text}
              </AppTag>
            ))
          ) : (
            <AppTag color='grey'>
              {t('channel.edit.billing.no_capabilities')}
            </AppTag>
          )}
        </div>
        <AppDetailSection
          title={t('channel.edit.billing.current_quotas_title')}
          titleTag='span'
        >
          <AppTable
            className='router-detail-table'
            pagination={false}
            loading={billingLoading}
            dataSource={quotaItems}
            rowKey={(row, index) => `${row?.quota_label || row?.quota_type || 'quota'}-${index}`}
            columns={[
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
                dataIndex: 'amount',
                key: 'amount',
                width: 180,
                render: (_, row) => formatAmountText(row),
              },
              {
                title: t('channel.edit.billing.quota_table.expires_at'),
                dataIndex: 'expires_at',
                key: 'expires_at',
                render: (_, row) => formatExpiresAtText(row, timestamp2string, t),
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
              <AppField label={t('channel.edit.billing.cdk')} required>
                <AppInput
                  className='router-section-input'
                  value={cdk}
                  onChange={(e, { value }) => setCDK((value || '').toString())}
                  readOnly={billingReadonly || billingSubmitting}
                />
              </AppField>
            </AppFormRow>
            <AppButton
              type='button'
              className='router-page-button'
              color='blue'
              loading={billingSubmitting}
              disabled={billingReadonly || billingSubmitting}
              onClick={() => onOpenActivatePage(cdk)}
            >
              {t('channel.edit.billing.open_activate_page')}
            </AppButton>
          </AppDetailSection>
        )}
        {billingSummary?.manual_update_supported && (
          <AppDetailSection
            title={t('channel.edit.billing.manual_update_title')}
            titleTag='span'
          >
            {manualItems.map((item, index) => (
              <AppFormRow key={`manual-quota-${index}`}>
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
              <AppButton
                type='button'
                className='router-page-button'
                color='blue'
                loading={billingSubmitting}
                disabled={billingReadonly || billingSubmitting}
                onClick={() =>
                  onManualSnapshotUpdate({
                    items: manualItems.map((manualItem) => ({
                      quota_type: manualItem.quota_type,
                      quota_label: manualItem.quota_label,
                      amount: manualItem.amount,
                      currency: manualItem.currency,
                      expires_at: toUnixTimestamp(manualItem.expires_at_input),
                    })),
                    message: manualMessage,
                  })
                }
              >
                {t('channel.edit.billing.confirm_manual_snapshot')}
              </AppButton>
            </div>
          </AppDetailSection>
        )}
        <AppDetailSection
          title={t('channel.edit.billing.snapshots_title')}
          titleTag='span'
        >
          <AppTable
            className='router-detail-table'
            pagination={false}
            loading={billingLoading}
            dataSource={billingSnapshots}
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
                title: t('channel.edit.billing.snapshot_table.source_type'),
                dataIndex: 'source_type',
                key: 'source_type',
                width: 120,
              },
              {
                title: t('channel.edit.billing.snapshot_table.quota_items'),
                dataIndex: 'items',
                key: 'items',
                render: (items) =>
                  Array.isArray(items) && items.length > 0
                    ? items.map((row) => `${row.quota_label || row.quota_type}: ${formatAmountText(row)}`).join(' / ')
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
          title={t('channel.edit.billing.actions_title')}
          titleTag='span'
        >
          <AppTable
            className='router-detail-table'
            pagination={false}
            loading={billingLoading}
            dataSource={billingActions}
            rowKey={(row) => row.id}
            columns={[
              {
                title: t('channel.edit.billing.action_table.created_at'),
                dataIndex: 'created_at',
                key: 'created_at',
                width: 180,
                render: (value) => (value ? timestamp2string(value) : '-'),
              },
              {
                title: t('channel.edit.billing.action_table.action_type'),
                dataIndex: 'action_type',
                key: 'action_type',
                width: 180,
              },
              {
                title: t('channel.edit.billing.action_table.status'),
                dataIndex: 'status',
                key: 'status',
                width: 120,
              },
              {
                title: t('channel.edit.billing.action_table.message'),
                dataIndex: 'message',
                key: 'message',
                render: (value) => value || '-',
              },
            ]}
          />
        </AppDetailSection>
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
