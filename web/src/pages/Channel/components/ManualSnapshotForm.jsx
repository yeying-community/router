import React from 'react';
import UnitDropdown from '../../../components/UnitDropdown';
import {
  AppAlert,
  AppButton,
  AppCompact,
  AppField,
  AppFormRow,
  AppInput,
  AppInputNumber,
  AppSegmented,
  AppSelect,
} from '../../../router-ui';
import {
  ensureUnitOption,
  isPurchaseCurrencyCNY,
  MANUAL_CURRENCY_OPTIONS,
  PROCUREMENT_CURRENCY_OPTIONS,
  PURCHASE_KINDS,
  resolveManualAmountLabel,
  shouldShowManualAmountFields,
} from './channelBilling.helpers';

// Create/edit form for a manual purchase snapshot. Organised around the two real
// procurement kinds — 充值 (recharge) and 订阅 (subscription) — so the common path
// stays short: a kind toggle, the purchase cost, one entitlement amount, and (for
// subscriptions) a validity window defaulting to 00:00:00. Low-frequency power
// (event types, upgrade lineage, multi-item entitlements, free resource-type
// choice) lives behind a collapsible 高级选项 section. Pure presentational — all
// draft state and mutators come from props.
const ManualSnapshotForm = ({
  t,
  manualPurchaseRecord,
  manualItems,
  manualMessage,
  editingPurchaseRecord,
  parentPurchaseOptions,
  manualChannelOptions,
  requireManualChannelSelect,
  advancedOpen,
  billingReadonly,
  billingSubmitting,
  onChangePurchaseKind,
  onToggleAdvanced,
  onUpdateManualPurchaseRecord,
  onUpdateManualValidityInput,
  onAppendManualItem,
  onRemoveManualItem,
  onUpdateManualItem,
  onManualMessageChange,
}) => {
  const purchaseKind = manualPurchaseRecord.purchase_kind || PURCHASE_KINDS[0];
  const isSubscription = purchaseKind === 'subscription';
  const isEditing = Boolean(editingPurchaseRecord?.id);
  const firstItem = manualItems[0] || {};
  const inputReadOnly = billingReadonly || billingSubmitting;

  const renderValidityFields = () => (
    <>
      <AppField label={t('channel.edit.billing.manual_valid_from')}>
        <AppInput
          className='router-section-input'
          type='datetime-local'
          step={1}
          value={manualPurchaseRecord.valid_from_input}
          onChange={(e, { value }) =>
            onUpdateManualValidityInput('valid_from_input', value, '00:00:00')
          }
          readOnly={inputReadOnly}
        />
      </AppField>
      <AppField label={t('channel.edit.billing.manual_valid_until')}>
        <AppInput
          className='router-section-input'
          type='datetime-local'
          step={1}
          value={manualPurchaseRecord.valid_until_input}
          onChange={(e, { value }) =>
            onUpdateManualValidityInput('valid_until_input', value, '00:00:00')
          }
          readOnly={inputReadOnly}
        />
      </AppField>
    </>
  );

  return (
    <div>
      <AppFormRow>
        <AppField label={t('channel.edit.billing.purchase_kind')} required>
          <AppSegmented
            options={PURCHASE_KINDS.map((value) => ({
              value,
              label: t(`channel.edit.billing.purchase_kinds.${value}`),
            }))}
            value={purchaseKind}
            onChange={(e, { value }) => onChangePurchaseKind(value)}
            disabled={inputReadOnly}
          />
        </AppField>
      </AppFormRow>
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
          {requireManualChannelSelect ? (
            <AppField label={t('channel.edit.billing.manual_channel')} required>
              <AppSelect
                className='router-section-input'
                search
                options={manualChannelOptions}
                value={manualPurchaseRecord.channel_id}
                placeholder={t('channel.edit.billing.manual_channel_placeholder')}
                onChange={(e, { value }) =>
                  onUpdateManualPurchaseRecord({
                    channel_id: (value || '').toString().trim(),
                  })
                }
                disabled={inputReadOnly || isEditing}
              />
            </AppField>
          ) : null}
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
                onUpdateManualPurchaseRecord({
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
              disabled={inputReadOnly}
            />
          </AppField>
          <AppField label={t('channel.edit.billing.manual_purchase_amount')} required>
            <AppInputNumber
              className='router-section-input'
              fluid
              min={0}
              value={manualPurchaseRecord.purchase_amount}
              onChange={(e, { value }) =>
                onUpdateManualPurchaseRecord({
                  purchase_amount: Number(value || 0),
                  purchase_cost_amount: isPurchaseCurrencyCNY(manualPurchaseRecord)
                    ? Number(value || 0)
                    : Number(value || 0) *
                      Number(manualPurchaseRecord.purchase_fx_rate || 0),
                })
              }
              disabled={inputReadOnly}
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
                    onUpdateManualPurchaseRecord({
                      purchase_fx_rate: Number(value || 0),
                      purchase_cost_amount:
                        Number(manualPurchaseRecord.purchase_amount || 0) *
                        Number(value || 0),
                    })
                  }
                  disabled={inputReadOnly}
                />
              </AppField>
              <AppField label={t('channel.edit.billing.manual_purchase_cost_amount')} required>
                <AppInputNumber
                  className='router-section-input'
                  fluid
                  min={0}
                  value={manualPurchaseRecord.purchase_cost_amount}
                  onChange={(e, { value }) =>
                    onUpdateManualPurchaseRecord({
                      purchase_cost_amount: Number(value || 0),
                    })
                  }
                  disabled={inputReadOnly}
                />
              </AppField>
            </>
          )}
        </AppFormRow>
      </div>
      <div className='router-billing-manual-item-card'>
        <div className='router-billing-manual-item-header'>
          <div className='router-billing-manual-item-title'>
            {t('channel.edit.billing.entitlement_info_title')}
          </div>
        </div>
        <AppFormRow>
          <AppField label={t('channel.edit.billing.entitlement_name')}>
            <AppInput
              className='router-section-input'
              value={manualPurchaseRecord.entitlement_name}
              onChange={(e, { value }) =>
                onUpdateManualPurchaseRecord({ entitlement_name: (value || '').toString() })
              }
              readOnly={inputReadOnly}
            />
          </AppField>
          {isSubscription ? renderValidityFields() : null}
          {!advancedOpen && shouldShowManualAmountFields(firstItem) ? (
            <AppField label={resolveManualAmountLabel(firstItem, t)} required>
              <AppCompact className='router-section-input-with-unit' block>
                <AppInputNumber
                  className='router-section-input router-section-input-with-unit-field'
                  fluid
                  value={firstItem.limit_amount}
                  min={0}
                  onChange={(e, { value }) =>
                    onUpdateManualItem(0, { limit_amount: value })
                  }
                  disabled={inputReadOnly}
                />
                <UnitDropdown
                  variant='inputUnit'
                  options={ensureUnitOption(
                    MANUAL_CURRENCY_OPTIONS,
                    firstItem.currency || 'USD'
                  )}
                  value={firstItem.currency || 'USD'}
                  onChange={(_, { value }) =>
                    onUpdateManualItem(0, {
                      currency: (value || 'USD').toString().trim().toUpperCase(),
                    })
                  }
                  disabled={inputReadOnly}
                  aria-label={t('channel.edit.billing.currency')}
                />
              </AppCompact>
            </AppField>
          ) : null}
        </AppFormRow>
      </div>
      <div className='router-billing-manual-item-header'>
        <AppButton
          type='button'
          className='router-page-button'
          basic
          onClick={onToggleAdvanced}
        >
          {`${advancedOpen ? '▾' : '▸'} ${t('channel.edit.billing.advanced_options')}`}
        </AppButton>
      </div>
      {advancedOpen ? (
        <>
          <div className='router-billing-manual-item-card'>
            <AppFormRow>
              <AppField label={t('channel.edit.billing.manual_purchase_at')} required>
                <AppInput
                  className='router-section-input'
                  type='datetime-local'
                  value={manualPurchaseRecord.purchase_at_input}
                  onChange={(e, { value }) =>
                    onUpdateManualPurchaseRecord({
                      purchase_at_input: (value || '').toString(),
                    })
                  }
                  readOnly={inputReadOnly}
                />
              </AppField>
              <AppField label={t('channel.edit.billing.message')}>
                <AppInput
                  className='router-section-input'
                  value={manualMessage}
                  onChange={(e, { value }) => onManualMessageChange((value || '').toString())}
                  readOnly={inputReadOnly}
                />
              </AppField>
            </AppFormRow>
          </div>
          {!isEditing ? (
            <div className='router-billing-manual-item-card'>
              <AppFormRow>
                <AppField label={t('channel.edit.billing.procurement_event_type')} required>
                  <AppSegmented
                    options={[
                      { value: 'purchase', label: t('channel.edit.billing.procurement_events.purchase') },
                      { value: 'renewal', label: t('channel.edit.billing.procurement_events.renewal') },
                      { value: 'upgrade', label: t('channel.edit.billing.procurement_events.upgrade') },
                      { value: 'quota_adjustment', label: t('channel.edit.billing.procurement_events.quota_adjustment') },
                    ]}
                    value={manualPurchaseRecord.event_type}
                    onChange={(e, { value }) =>
                      onUpdateManualPurchaseRecord({
                        event_type: value,
                        parent_snapshot_id: value === 'upgrade' ? manualPurchaseRecord.parent_snapshot_id : '',
                        old_batch_disposition: value === 'upgrade' ? manualPurchaseRecord.old_batch_disposition : 'keep',
                      })
                    }
                    disabled={inputReadOnly}
                  />
                </AppField>
              </AppFormRow>
              {manualPurchaseRecord.event_type === 'upgrade' ? (
                <AppFormRow>
                  <AppField label={t('channel.edit.billing.procurement_parent_record')} required>
                    <AppSelect
                      className='router-section-input'
                      search
                      options={parentPurchaseOptions}
                      value={manualPurchaseRecord.parent_snapshot_id}
                      placeholder={t('channel.edit.billing.procurement_parent_record_placeholder')}
                      onChange={(e, { value }) =>
                        onUpdateManualPurchaseRecord({ parent_snapshot_id: (value || '').toString() })
                      }
                      disabled={inputReadOnly}
                    />
                  </AppField>
                  <AppField label={t('channel.edit.billing.procurement_old_batch_disposition')} required>
                    <AppSegmented
                      options={[
                        { value: 'keep', label: t('channel.edit.billing.procurement_old_batch_dispositions.keep') },
                        { value: 'disable', label: t('channel.edit.billing.procurement_old_batch_dispositions.disable') },
                      ]}
                      value={manualPurchaseRecord.old_batch_disposition}
                      onChange={(e, { value }) =>
                        onUpdateManualPurchaseRecord({ old_batch_disposition: value })
                      }
                      disabled={inputReadOnly}
                    />
                  </AppField>
                </AppFormRow>
              ) : null}
            </div>
          ) : null}
          {!isSubscription ? (
            <div className='router-billing-manual-item-card'>
              <AppFormRow>{renderValidityFields()}</AppFormRow>
            </div>
          ) : null}
          <div className='router-billing-manual-item-header'>
            <div className='router-billing-manual-item-title'>
              {t('channel.edit.billing.entitlement_items_title')}
            </div>
            <AppButton
              type='button'
              className='router-page-button'
              basic
              disabled={inputReadOnly}
              onClick={onAppendManualItem}
            >
              {t('channel.edit.billing.add_entitlement_item')}
            </AppButton>
          </div>
          <AppAlert
            type='info'
            showIcon
            className='router-section-message'
            title={t('channel.edit.billing.manual_resource_hints.default')}
          />
          {manualItems.map((item, index) => (
            <div key={`manual-quota-${index}`} className='router-billing-manual-item-card'>
              <div className='router-billing-manual-item-header'>
                <div className='router-billing-manual-item-title'>
                  {t('channel.edit.billing.manual_item_title', { index: index + 1 })}
                </div>
                <div className='router-billing-manual-item-actions'>
                  <AppButton
                    type='button'
                    className='router-page-button'
                    basic
                    danger
                    disabled={inputReadOnly}
                    onClick={() => onRemoveManualItem(index)}
                  >
                    {t('channel.edit.billing.remove_quota_item')}
                  </AppButton>
                </div>
              </div>
              <AppFormRow>
                <AppField label={resolveManualAmountLabel(item, t)} required>
                  <AppCompact className='router-section-input-with-unit' block>
                    <AppInputNumber
                      className='router-section-input router-section-input-with-unit-field'
                      fluid
                      value={item.limit_amount}
                      min={0}
                      onChange={(e, { value }) =>
                        onUpdateManualItem(index, {
                          limit_amount: value,
                        })
                      }
                      disabled={inputReadOnly}
                    />
                    <UnitDropdown
                      variant='inputUnit'
                      options={ensureUnitOption(
                        MANUAL_CURRENCY_OPTIONS,
                        item.currency || 'USD'
                      )}
                      value={item.currency || 'USD'}
                      onChange={(_, { value }) =>
                        onUpdateManualItem(index, {
                          currency: (value || 'USD')
                            .toString()
                            .trim()
                            .toUpperCase(),
                        })
                      }
                      disabled={inputReadOnly}
                      aria-label={t('channel.edit.billing.currency')}
                    />
                  </AppCompact>
                </AppField>
              </AppFormRow>
            </div>
          ))}
        </>
      ) : null}
    </div>
  );
};

export default ManualSnapshotForm;
