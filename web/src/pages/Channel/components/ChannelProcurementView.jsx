import React, { useMemo, useState } from 'react';
import { showInfo } from '../../../helpers';
import {
  AppAlert,
  AppButton,
  AppFormActions,
  AppModal,
  AppSegmented,
  AppTag,
  AppTooltip,
} from '../../../router-ui';
import ConsumptionModal from './ConsumptionModal';
import CostQuoteReconcileTable from './CostQuoteReconcileTable';
import ManualSnapshotForm from './ManualSnapshotForm';
import ProcurementBatchTable from './ProcurementBatchTable';
import ProcurementCostForm from './ProcurementCostForm';
import SnapshotRecordsTable from './SnapshotRecordsTable';
import {
  applyPurchaseKindToItem,
  buildCostTrackingModeOptions,
  buildManualPurchaseRecord,
  buildManualPurchaseRecordFromSnapshot,
  buildManualQuotaItem,
  buildManualQuotaItemFromSnapshotItem,
  buildProcurementCostDraft,
  formatCostTrackingConsequence,
  normalizeCostTrackingModeValue,
  normalizeManualValidityInput,
  recordUsesAdvanced,
  resolveManualItemAmounts,
  toUnixTimestamp,
} from './channelBilling.helpers';

// Per-channel procurement workspace: manual purchase snapshots + procurement
// batches (cost/status/consumptions). Pure presentational — all data/handlers
// come from props. Shared by the channel detail procurement tab and the finance
// aggregate report's (legacy) channel drill-down.
const ChannelProcurementView = ({
  t,
  billingLoading,
  billingSnapshots,
  procurementBatches,
  billingReadonly,
  billingSubmitting,
  onRefreshBilling,
  onManualSnapshotUpdate,
  onManualSnapshotDelete,
  onProcurementBatchCostUpdate,
  onProcurementBatchStatusUpdate,
  onProcurementBatchConsumptionsLoad,
  timestamp2string,
  billingError,
  channelID,
  manualChannelOptions = [],
  requireManualChannelSelect = false,
  showProcurementBatches = true,
  costTrackingMode = null,
  onCostTrackingModeChange,
  costTrackingSubmitting = false,
  costMissingModelCount = 0,
  costQuotes = null,
  onSyncCostQuotes,
}) => {
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
  const [manualValidityTouched, setManualValidityTouched] = useState({
    valid_from_input: false,
    valid_until_input: false,
  });
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [billingView, setBillingView] = useState('records');

  const purchaseRecords = useMemo(
    () =>
      (Array.isArray(billingSnapshots) ? billingSnapshots : []).filter(
        (snapshot) =>
          (snapshot?.source_type || '').toString().trim() === 'manual'
      ),
    [billingSnapshots]
  );
  const procurementRows = Array.isArray(procurementBatches)
    ? procurementBatches
    : [];
  const parentPurchaseOptions = purchaseRecords
    .filter((item) => item?.id && item.id !== editingPurchaseRecord?.id)
    .map((item) => ({
      value: item.id,
      label: `${item.entitlement_name || item.id} ${item.purchase_at ? timestamp2string(item.purchase_at) : ''}`.trim(),
    }));

  // The cost-tracking control only renders when the parent supplies a mode (the
  // channel detail cost tab). The finance report drill-down omits these props, so
  // hasCostControl is false there and the procurement workspace shows unchanged.
  const hasCostControl = costTrackingMode != null;
  const effectiveCostMode = normalizeCostTrackingModeValue(costTrackingMode);
  // Records/batches are the "actual cost" facts; untracked/free don't use them,
  // so hide the whole workspace there and leave only the cost switch + its hint.
  const showProcurementWorkspace =
    !hasCostControl || effectiveCostMode === 'actual';

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

  // 切换采购类型:同步首条权益项的资源/额度类型;充值回到「新购」并丢弃升级关联。
  const changePurchaseKind = (kind) => {
    setManualPurchaseRecord((prev) => ({
      ...prev,
      purchase_kind: kind,
      ...(kind === 'recharge'
        ? { event_type: 'purchase', parent_snapshot_id: '', old_batch_disposition: 'keep' }
        : {}),
    }));
    setManualItems((prev) => {
      const list = prev.length > 0 ? prev : [buildManualQuotaItem()];
      return list.map((item, index) =>
        index === 0 ? applyPurchaseKindToItem(item) : item
      );
    });
  };

  const updateManualValidityInput = (field, value, defaultTime) => {
    const firstTouch = manualValidityTouched[field] !== true;
    updateManualPurchaseRecord({
      [field]: normalizeManualValidityInput(value, defaultTime, firstTouch),
    });
    if (firstTouch) {
      setManualValidityTouched((prev) => ({
        ...prev,
        [field]: true,
      }));
    }
  };

  const closeManualModal = () => {
    if (!billingSubmitting) {
      setManualModalOpen(false);
      setEditingPurchaseRecord(null);
    }
  };

  const openCreateManualModal = () => {
    setEditingPurchaseRecord(null);
    setManualPurchaseRecord({
      ...buildManualPurchaseRecord(),
      channel_id: (channelID || '').toString().trim(),
    });
    setManualValidityTouched({
      valid_from_input: false,
      valid_until_input: false,
    });
    setAdvancedOpen(false);
    setManualMessage('');
    setManualItems([
      applyPurchaseKindToItem(buildManualQuotaItem()),
    ]);
    setManualModalOpen(true);
  };

  const openEditManualModal = (row) => {
    setEditingPurchaseRecord(row);
    const nextRecord = {
      ...buildManualPurchaseRecordFromSnapshot(row),
      channel_id: (row?.channel_id || channelID || '').toString().trim(),
    };
    setManualPurchaseRecord(nextRecord);
    setManualValidityTouched({
      valid_from_input: true,
      valid_until_input: true,
    });
    setManualMessage((row?.message || '').toString());
    const items = Array.isArray(row?.items) ? row.items : [];
    const nextItems =
      items.length > 0
        ? items.map((item) => buildManualQuotaItemFromSnapshotItem(item))
        : [applyPurchaseKindToItem(buildManualQuotaItem())];
    setManualItems(nextItems);
    setAdvancedOpen(
      recordUsesAdvanced(nextRecord, nextItems) ||
        Boolean((row?.message || '').toString().trim())
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
    const targetChannelID = (manualPurchaseRecord.channel_id || channelID || '')
      .toString()
      .trim();
    if (requireManualChannelSelect && !targetChannelID) {
      showInfo(t('channel.edit.billing.manual_channel_required'));
      return;
    }
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
    // 权益名称选填:留空时用「采购类型 + 采购日期」兜底(后端要求非空)。
    const trimmedName = (manualPurchaseRecord.entitlement_name || '')
      .toString()
      .trim();
    const entitlementName =
      trimmedName ||
      [
        t(
          `channel.edit.billing.purchase_kinds.${manualPurchaseRecord.purchase_kind || 'recharge'}`
        ),
        (manualPurchaseRecord.purchase_at_input || '').toString().slice(0, 10),
      ]
        .filter(Boolean)
        .join(' ');
    const saved = await onManualSnapshotUpdate({
      channel_id: targetChannelID,
      id: editingPurchaseRecord?.id || '',
      purchase_at: toUnixTimestamp(manualPurchaseRecord.purchase_at_input),
      purchase_currency: purchaseCurrency,
      purchase_amount: purchaseAmount,
      purchase_fx_rate: purchaseFXRate,
      purchase_cost_amount: purchaseCostAmount,
      entitlement_name: entitlementName,
      event_type: manualPurchaseRecord.event_type,
      parent_snapshot_id: manualPurchaseRecord.parent_snapshot_id,
      old_batch_disposition: manualPurchaseRecord.old_batch_disposition,
      valid_from: toUnixTimestamp(manualPurchaseRecord.valid_from_input),
      valid_until: toUnixTimestamp(manualPurchaseRecord.valid_until_input),
      items: manualItems.map((manualItem) => {
        const amounts = resolveManualItemAmounts(manualItem);
        return {
          id: manualItem.id,
          resource_type: manualItem.resource_type,
          quota_type: manualItem.quota_type,
          quota_label: '',
          ...amounts,
          currency: manualItem.currency,
          reset_at: toUnixTimestamp(manualItem.reset_at_input),
          expires_at: toUnixTimestamp(manualItem.expires_at_input),
          source_ref: 'manual',
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

  return (
    <div className='router-billing-page'>
      {hasCostControl ? (
        <div className='router-cost-mode-control'>
          <div className='router-cost-mode-control-head'>
            <AppTooltip
              title={t('channel.edit.billing.cost_tracking_mode.tooltip')}
            >
              <span className='router-cost-mode-control-label'>
                {t('channel.edit.billing.cost_tracking_mode.label')}
              </span>
            </AppTooltip>
            <AppSegmented
              value={effectiveCostMode}
              onChange={(e, { value }) =>
                typeof onCostTrackingModeChange === 'function'
                  ? onCostTrackingModeChange(
                      normalizeCostTrackingModeValue(value)
                    )
                  : undefined
              }
              options={buildCostTrackingModeOptions(t)}
              disabled={billingReadonly || costTrackingSubmitting}
            />
            <span className='router-cost-mode-consequence'>
              {formatCostTrackingConsequence(t, effectiveCostMode)}
            </span>
            {effectiveCostMode === 'actual' &&
            Number(costMissingModelCount) > 0 ? (
              <AppTag
                className='router-tag'
                color='orange'
                title={t(
                  'channel.edit.billing.cost_tracking_mode.missing_cost_hint'
                )}
              >
                {t('channel.edit.billing.cost_tracking_mode.missing_cost', {
                  count: Number(costMissingModelCount),
                })}
              </AppTag>
            ) : null}
          </div>
          <div className='router-form-hint'>
            {t(
              `channel.edit.billing.cost_tracking_mode.hint.${effectiveCostMode}`
            )}
          </div>
        </div>
      ) : null}
      {showProcurementWorkspace ? (
        <>
          <div className='router-billing-workspace-toolbar'>
        {showProcurementBatches ? (
          <AppSegmented
            value={billingView}
            onChange={(e, { value }) => setBillingView(value)}
            options={[
              { value: 'records', label: t('channel.edit.billing.snapshots_title') },
              { value: 'batches', label: t('channel.edit.billing.procurement_title') },
            ]}
          />
        ) : null}
        {billingView === 'records' || (showProcurementBatches && billingView === 'batches') ? (
          <AppButton type='button' className='router-page-button' color='blue'
            disabled={billingReadonly || billingSubmitting}
            onClick={openCreateManualModal}>
            {t('channel.edit.billing.add_purchase_record')}
          </AppButton>
        ) : null}
        {typeof onRefreshBilling === 'function' ? (
          <AppButton
            type='button'
            className='router-page-button'
            loading={billingLoading}
            disabled={billingLoading || billingSubmitting}
            onClick={onRefreshBilling}
          >
            {t('common.refresh')}
          </AppButton>
        ) : null}
      </div>
      <AppAlert type='info' showIcon className='router-section-message' title={t('channel.edit.billing.structure_hint')} />
      {billingView === 'records' && (
        <SnapshotRecordsTable
          t={t}
          purchaseRecords={purchaseRecords}
          billingLoading={billingLoading}
          billingReadonly={billingReadonly}
          billingSubmitting={billingSubmitting}
          timestamp2string={timestamp2string}
          onEditRecord={openEditManualModal}
          onDeleteRecord={deleteManualSnapshot}
        />
      )}
      {showProcurementBatches && billingView === 'batches' && (
        <ProcurementBatchTable
          t={t}
          procurementRows={procurementRows}
          billingLoading={billingLoading}
          billingReadonly={billingReadonly}
          billingSubmitting={billingSubmitting}
          timestamp2string={timestamp2string}
          onViewConsumptions={openConsumptionModal}
          onEditCost={openCostModal}
          onUpdateStatus={updateProcurementBatchStatus}
        />
      )}
      {costQuotes ? (
        <CostQuoteReconcileTable
          t={t}
          costQuotes={costQuotes}
          onSyncCostQuotes={onSyncCostQuotes}
          syncSubmitting={billingSubmitting}
        />
      ) : null}
        </>
      ) : null}
      <div>
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
          <ManualSnapshotForm
            t={t}
            manualPurchaseRecord={manualPurchaseRecord}
            manualItems={manualItems}
            manualMessage={manualMessage}
            editingPurchaseRecord={editingPurchaseRecord}
            parentPurchaseOptions={parentPurchaseOptions}
            manualChannelOptions={manualChannelOptions}
            requireManualChannelSelect={requireManualChannelSelect}
            advancedOpen={advancedOpen}
            billingReadonly={billingReadonly}
            billingSubmitting={billingSubmitting}
            onChangePurchaseKind={changePurchaseKind}
            onToggleAdvanced={() => setAdvancedOpen((prev) => !prev)}
            onUpdateManualPurchaseRecord={updateManualPurchaseRecord}
            onUpdateManualValidityInput={updateManualValidityInput}
            onAppendManualItem={appendManualItem}
            onRemoveManualItem={removeManualItem}
            onUpdateManualItem={updateManualItem}
            onManualMessageChange={setManualMessage}
          />
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
          <ProcurementCostForm
            t={t}
            costDraft={costDraft}
            onUpdateCostDraft={updateCostDraft}
            billingReadonly={billingReadonly}
            billingSubmitting={billingSubmitting}
          />
        </AppModal>
        <ConsumptionModal
          t={t}
          open={consumptionModalOpen}
          onClose={closeConsumptionModal}
          loading={consumptionLoading}
          rows={consumptionRows}
          batch={viewingProcurementBatch}
          timestamp2string={timestamp2string}
        />
        {billingError && (
          <div className='router-error-text router-error-text-top'>
            {billingError}
          </div>
        )}
      </div>
    </div>
  );
};

export default ChannelProcurementView;
