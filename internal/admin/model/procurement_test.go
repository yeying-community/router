package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newProcurementTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&ChannelBillingSnapshot{}, &ChannelBillingSnapshotItem{}, &ChannelProcurementBatch{}, &RequestProcurementConsumption{}, &Log{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestConsumeChannelProcurementBatchesWithDB(t *testing.T) {
	db := newProcurementTestDB(t)
	batch, err := CreateChannelProcurementBatchWithDB(db, ChannelProcurementBatch{
		ChannelId:          "channel-1",
		ResourceType:       "quota",
		QuotaType:          "total",
		ScopeType:          "model",
		ScopeValue:         "gpt-5",
		CapacityUnit:       "token",
		CapacityTotal:      100,
		CapacityEffective:  100,
		CapacityRemaining:  100,
		PurchaseCostAmount: 10,
		CostPerUnitAmount:  0.1,
		CostSource:         ProcurementCostSourceActual,
		CostStatus:         ProcurementCostStatusActive,
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	result, err := ConsumeChannelProcurementBatchesWithDB(db, ProcurementConsumeInput{
		RequestLogID:        "log-1",
		ChannelID:           "channel-1",
		ScopeType:           "model",
		ScopeValue:          "gpt-5",
		CapacityUnit:        "token",
		Quantity:            40,
		SettlementTruthMode: "hybrid_usage_final",
	})
	if err != nil {
		t.Fatalf("consume procurement: %v", err)
	}
	if len(result.Consumptions) != 1 {
		t.Fatalf("consumptions len=%d, want 1", len(result.Consumptions))
	}
	if result.TotalCostAmount != 4 {
		t.Fatalf("TotalCostAmount=%v, want 4", result.TotalCostAmount)
	}

	var updated ChannelProcurementBatch
	if err := db.Where("id = ?", batch.Id).Take(&updated).Error; err != nil {
		t.Fatalf("load updated batch: %v", err)
	}
	if updated.CapacityRemaining != 60 {
		t.Fatalf("CapacityRemaining=%v, want 60", updated.CapacityRemaining)
	}
	if updated.CostStatus != ProcurementCostStatusActive {
		t.Fatalf("CostStatus=%q, want active", updated.CostStatus)
	}
}

func TestCountRequestProcurementConsumptionsBySourceSnapshotIDWithDB(t *testing.T) {
	db := newProcurementTestDB(t)
	snapshotID := "snapshot-1"
	batch, err := CreateChannelProcurementBatchWithDB(db, ChannelProcurementBatch{
		ChannelId:            "channel-1",
		ResourceType:         "quota",
		QuotaType:            "total",
		ScopeType:            "global",
		CapacityUnit:         "token",
		CapacityTotal:        100,
		CapacityEffective:    100,
		CapacityRemaining:    100,
		PurchaseCostAmount:   10,
		CostPerUnitAmount:    0.1,
		CostSource:           ProcurementCostSourceActual,
		CostStatus:           ProcurementCostStatusActive,
		SourceSnapshotId:     snapshotID,
		SourceSnapshotItemId: "item-1",
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	count, err := CountRequestProcurementConsumptionsBySourceSnapshotIDWithDB(db, snapshotID)
	if err != nil {
		t.Fatalf("count consumptions: %v", err)
	}
	if count != 0 {
		t.Fatalf("count=%d, want 0", count)
	}

	if err := db.Create(&RequestProcurementConsumption{
		Id:                 "consumption-1",
		RequestLogId:       "log-1",
		ChannelId:          "channel-1",
		ProcurementBatchId: batch.Id,
		ResourceType:       "quota",
		QuotaType:          "total",
		CapacityUnit:       "token",
		ConsumedQuantity:   1,
		UnitCostAmount:     0.1,
		ConsumedCostAmount: 0.1,
		CostSource:         ProcurementCostSourceActual,
	}).Error; err != nil {
		t.Fatalf("create consumption: %v", err)
	}

	count, err = CountRequestProcurementConsumptionsBySourceSnapshotIDWithDB(db, snapshotID)
	if err != nil {
		t.Fatalf("count consumptions after insert: %v", err)
	}
	if count != 1 {
		t.Fatalf("count=%d, want 1", count)
	}
}

func TestConsumeChannelProcurementBatchesMarksExhausted(t *testing.T) {
	db := newProcurementTestDB(t)
	batch, err := CreateChannelProcurementBatchWithDB(db, ChannelProcurementBatch{
		ChannelId:         "channel-1",
		ResourceType:      "quota",
		QuotaType:         "total",
		ScopeType:         "global",
		CapacityUnit:      "token",
		CapacityTotal:     10,
		CapacityEffective: 10,
		CapacityRemaining: 10,
		CostPerUnitAmount: 0.2,
		CostSource:        ProcurementCostSourceActual,
		CostStatus:        ProcurementCostStatusActive,
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	_, err = ConsumeChannelProcurementBatchesWithDB(db, ProcurementConsumeInput{
		RequestLogID: "log-1",
		ChannelID:    "channel-1",
		CapacityUnit: "token",
		Quantity:     10,
	})
	if err != nil {
		t.Fatalf("consume procurement: %v", err)
	}

	var updated ChannelProcurementBatch
	if err := db.Where("id = ?", batch.Id).Take(&updated).Error; err != nil {
		t.Fatalf("load updated batch: %v", err)
	}
	if updated.CapacityRemaining != 0 {
		t.Fatalf("CapacityRemaining=%v, want 0", updated.CapacityRemaining)
	}
	if updated.CostStatus != ProcurementCostStatusExhausted {
		t.Fatalf("CostStatus=%q, want exhausted", updated.CostStatus)
	}
}

func TestConsumeChannelProcurementBatchesPrefersModelScope(t *testing.T) {
	db := newProcurementTestDB(t)
	globalBatch, err := CreateChannelProcurementBatchWithDB(db, ChannelProcurementBatch{
		ChannelId:         "channel-1",
		ResourceType:      "quota",
		QuotaType:         "total",
		ScopeType:         "global",
		CapacityUnit:      "token",
		CapacityTotal:     100,
		CapacityEffective: 100,
		CapacityRemaining: 100,
		CostPerUnitAmount: 0.1,
		CostSource:        ProcurementCostSourceActual,
		CostStatus:        ProcurementCostStatusActive,
	})
	if err != nil {
		t.Fatalf("create global batch: %v", err)
	}
	modelBatch, err := CreateChannelProcurementBatchWithDB(db, ChannelProcurementBatch{
		ChannelId:         "channel-1",
		ResourceType:      "quota",
		QuotaType:         "total",
		ScopeType:         "model",
		ScopeValue:        "gpt-5",
		CapacityUnit:      "token",
		CapacityTotal:     100,
		CapacityEffective: 100,
		CapacityRemaining: 100,
		CostPerUnitAmount: 0.5,
		CostSource:        ProcurementCostSourceActual,
		CostStatus:        ProcurementCostStatusActive,
	})
	if err != nil {
		t.Fatalf("create model batch: %v", err)
	}

	result, err := ConsumeChannelProcurementBatchesWithDB(db, ProcurementConsumeInput{
		RequestLogID: "log-1",
		ChannelID:    "channel-1",
		ScopeType:    "model",
		ScopeValue:   "gpt-5",
		CapacityUnit: "token",
		Quantity:     10,
	})
	if err != nil {
		t.Fatalf("consume procurement: %v", err)
	}
	if len(result.Consumptions) != 1 {
		t.Fatalf("consumptions len=%d, want 1", len(result.Consumptions))
	}
	if result.Consumptions[0].ProcurementBatchId != modelBatch.Id {
		t.Fatalf("ProcurementBatchId=%q, want model batch %q", result.Consumptions[0].ProcurementBatchId, modelBatch.Id)
	}

	var updatedGlobal ChannelProcurementBatch
	if err := db.Where("id = ?", globalBatch.Id).Take(&updatedGlobal).Error; err != nil {
		t.Fatalf("load global batch: %v", err)
	}
	if updatedGlobal.CapacityRemaining != 100 {
		t.Fatalf("global CapacityRemaining=%v, want 100", updatedGlobal.CapacityRemaining)
	}
}

func TestEstimateChannelProcurementCostWithDBDoesNotConsume(t *testing.T) {
	db := newProcurementTestDB(t)
	batch, err := CreateChannelProcurementBatchWithDB(db, ChannelProcurementBatch{
		ChannelId:         "channel-1",
		ResourceType:      "quota",
		QuotaType:         "total",
		ScopeType:         "global",
		CapacityUnit:      "usd_equivalent",
		CapacityTotal:     100,
		CapacityEffective: 100,
		CapacityRemaining: 100,
		CostPerUnitAmount: 7,
		CostSource:        ProcurementCostSourceActual,
		CostStatus:        ProcurementCostStatusActive,
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	result, err := EstimateChannelProcurementCostWithDB(db, ProcurementConsumeInput{
		ChannelID:    "channel-1",
		CapacityUnit: "usd_equivalent",
		Quantity:     2,
	})
	if err != nil {
		t.Fatalf("estimate procurement: %v", err)
	}
	if result.TotalCostAmount != 14 {
		t.Fatalf("TotalCostAmount=%v, want 14", result.TotalCostAmount)
	}
	if result.CostSource != ProcurementCostSourceActual {
		t.Fatalf("CostSource=%q, want actual", result.CostSource)
	}
	if result.CoveredQuantity != 2 || result.MissingQuantity != 0 {
		t.Fatalf("coverage=%v missing=%v, want 2/0", result.CoveredQuantity, result.MissingQuantity)
	}

	var updated ChannelProcurementBatch
	if err := db.Where("id = ?", batch.Id).Take(&updated).Error; err != nil {
		t.Fatalf("load updated batch: %v", err)
	}
	if updated.CapacityRemaining != 100 {
		t.Fatalf("CapacityRemaining=%v, want 100", updated.CapacityRemaining)
	}
	var count int64
	if err := db.Model(&RequestProcurementConsumption{}).Count(&count).Error; err != nil {
		t.Fatalf("count consumptions: %v", err)
	}
	if count != 0 {
		t.Fatalf("consumption count=%d, want 0", count)
	}
}

func TestEstimateChannelProcurementCostWithDBPrefersModelScope(t *testing.T) {
	db := newProcurementTestDB(t)
	_, err := CreateChannelProcurementBatchWithDB(db, ChannelProcurementBatch{
		ChannelId:         "channel-1",
		ResourceType:      "quota",
		QuotaType:         "total",
		ScopeType:         "global",
		CapacityUnit:      "token",
		CapacityTotal:     100,
		CapacityEffective: 100,
		CapacityRemaining: 100,
		CostPerUnitAmount: 0.1,
		CostSource:        ProcurementCostSourceActual,
		CostStatus:        ProcurementCostStatusActive,
	})
	if err != nil {
		t.Fatalf("create global batch: %v", err)
	}
	_, err = CreateChannelProcurementBatchWithDB(db, ChannelProcurementBatch{
		ChannelId:         "channel-1",
		ResourceType:      "quota",
		QuotaType:         "total",
		ScopeType:         "model",
		ScopeValue:        "gpt-5",
		CapacityUnit:      "token",
		CapacityTotal:     100,
		CapacityEffective: 100,
		CapacityRemaining: 100,
		CostPerUnitAmount: 0.5,
		CostSource:        ProcurementCostSourceActual,
		CostStatus:        ProcurementCostStatusActive,
	})
	if err != nil {
		t.Fatalf("create model batch: %v", err)
	}

	result, err := EstimateChannelProcurementCostWithDB(db, ProcurementConsumeInput{
		ChannelID:    "channel-1",
		ScopeType:    "model",
		ScopeValue:   "gpt-5",
		CapacityUnit: "token",
		Quantity:     10,
	})
	if err != nil {
		t.Fatalf("estimate procurement: %v", err)
	}
	if result.TotalCostAmount != 5 {
		t.Fatalf("TotalCostAmount=%v, want 5", result.TotalCostAmount)
	}
}

func TestEstimateChannelProcurementCostWithDBReportsPartialCoverage(t *testing.T) {
	db := newProcurementTestDB(t)
	_, err := CreateChannelProcurementBatchWithDB(db, ChannelProcurementBatch{
		ChannelId:         "channel-1",
		ResourceType:      "quota",
		QuotaType:         "total",
		ScopeType:         "global",
		CapacityUnit:      "token",
		CapacityTotal:     20,
		CapacityEffective: 20,
		CapacityRemaining: 20,
		CostPerUnitAmount: 0.25,
		CostSource:        ProcurementCostSourceEstimated,
		CostStatus:        ProcurementCostStatusActive,
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	result, err := EstimateChannelProcurementCostWithDB(db, ProcurementConsumeInput{
		ChannelID:    "channel-1",
		CapacityUnit: "token",
		Quantity:     50,
	})
	if err != nil {
		t.Fatalf("estimate procurement: %v", err)
	}
	if result.TotalCostAmount != 5 {
		t.Fatalf("TotalCostAmount=%v, want 5", result.TotalCostAmount)
	}
	if result.CostSource != ProcurementCostSourceEstimated {
		t.Fatalf("CostSource=%q, want estimated", result.CostSource)
	}
	if result.CoveredQuantity != 20 || result.MissingQuantity != 30 {
		t.Fatalf("coverage=%v missing=%v, want 20/30", result.CoveredQuantity, result.MissingQuantity)
	}
}

func TestUpdateLogProcurementCostObservationWithDB(t *testing.T) {
	db := newProcurementTestDB(t)
	logRow := Log{Id: "log-1", BillingSellBaseAmount: 10}
	if err := db.Create(&logRow).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	if err := UpdateLogProcurementCostObservationWithDB(db, "log-1", 4, ProcurementCostSourceActual, 10); err != nil {
		t.Fatalf("update log procurement cost: %v", err)
	}

	var updated Log
	if err := db.Where("id = ?", "log-1").Take(&updated).Error; err != nil {
		t.Fatalf("load updated log: %v", err)
	}
	if updated.BillingProcurementCostBaseAmount != 4 {
		t.Fatalf("BillingProcurementCostBaseAmount=%v, want 4", updated.BillingProcurementCostBaseAmount)
	}
	if updated.BillingGrossProfitBaseAmount != 6 {
		t.Fatalf("BillingGrossProfitBaseAmount=%v, want 6", updated.BillingGrossProfitBaseAmount)
	}
	if updated.BillingGrossMargin != 0.6 {
		t.Fatalf("BillingGrossMargin=%v, want 0.6", updated.BillingGrossMargin)
	}
}

func TestCreateBillingSnapshotItemsDoesNotCreateProcurementBatchForAPISnapshot(t *testing.T) {
	db := newProcurementTestDB(t)
	snapshot, err := CreateChannelBillingSnapshotWithDB(db, ChannelBillingSnapshot{
		ChannelId:  "channel-1",
		SourceType: ChannelBillingSnapshotSourceAPI,
		CreatedAt:  100,
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}

	items, err := CreateChannelBillingSnapshotItemsWithDB(db, snapshot.Id, "channel-1", []ChannelBillingSnapshotItem{
		{
			ResourceType:    ChannelBillingResourceTypeCredit,
			QuotaType:       "total",
			Amount:          100,
			LimitAmount:     100,
			RemainingAmount: 80,
			Currency:        "USD",
			ExpiresAt:       200,
			SourceRef:       "test_credit",
		},
		{
			ResourceType:    ChannelBillingResourceTypePlan,
			QuotaType:       "custom",
			RemainingAmount: 1,
			SourceRef:       "test_plan",
		},
	})
	if err != nil {
		t.Fatalf("create snapshot items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len=%d, want 2", len(items))
	}

	var batches []ChannelProcurementBatch
	if err := db.Order("created_at asc").Find(&batches).Error; err != nil {
		t.Fatalf("list procurement batches: %v", err)
	}
	if len(batches) != 0 {
		t.Fatalf("batches len=%d, want 0", len(batches))
	}
}

func TestCreateBillingSnapshotItemsCreatesUnconfiguredProcurementBatchForManualSnapshot(t *testing.T) {
	db := newProcurementTestDB(t)
	snapshot, err := CreateChannelBillingSnapshotWithDB(db, ChannelBillingSnapshot{
		ChannelId:  "channel-1",
		SourceType: ChannelBillingSnapshotSourceManual,
		CreatedAt:  100,
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}

	items, err := CreateChannelBillingSnapshotItemsWithDB(db, snapshot.Id, "channel-1", []ChannelBillingSnapshotItem{
		{
			ResourceType:    ChannelBillingResourceTypeCredit,
			QuotaType:       "total",
			Amount:          100,
			LimitAmount:     100,
			RemainingAmount: 80,
			Currency:        "USD",
			ExpiresAt:       200,
			SourceRef:       "test_credit",
		},
		{
			ResourceType:    ChannelBillingResourceTypePlan,
			QuotaType:       "custom",
			RemainingAmount: 1,
			SourceRef:       "test_plan",
		},
	})
	if err != nil {
		t.Fatalf("create snapshot items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len=%d, want 2", len(items))
	}

	var batches []ChannelProcurementBatch
	if err := db.Order("created_at asc").Find(&batches).Error; err != nil {
		t.Fatalf("list procurement batches: %v", err)
	}
	if len(batches) != 1 {
		t.Fatalf("batches len=%d, want 1", len(batches))
	}
	batch := batches[0]
	if batch.ChannelId != "channel-1" {
		t.Fatalf("ChannelId=%q, want channel-1", batch.ChannelId)
	}
	if batch.ResourceType != ChannelBillingResourceTypeCredit {
		t.Fatalf("ResourceType=%q, want credit", batch.ResourceType)
	}
	if batch.CapacityUnit != "usd_equivalent" {
		t.Fatalf("CapacityUnit=%q, want usd_equivalent", batch.CapacityUnit)
	}
	if batch.CapacityTotal != 100 {
		t.Fatalf("CapacityTotal=%v, want 100", batch.CapacityTotal)
	}
	if batch.CapacityRemaining != 80 {
		t.Fatalf("CapacityRemaining=%v, want 80", batch.CapacityRemaining)
	}
	if batch.CostSource != ProcurementCostSourceNone {
		t.Fatalf("CostSource=%q, want none", batch.CostSource)
	}
	if batch.CostStatus != ProcurementCostStatusCostUnconfigured {
		t.Fatalf("CostStatus=%q, want cost_unconfigured", batch.CostStatus)
	}
	if batch.SourceSnapshotId != snapshot.Id {
		t.Fatalf("SourceSnapshotId=%q, want %q", batch.SourceSnapshotId, snapshot.Id)
	}
	if batch.SourceSnapshotItemId != items[0].Id {
		t.Fatalf("SourceSnapshotItemId=%q, want %q", batch.SourceSnapshotItemId, items[0].Id)
	}
}

func TestCreateBillingSnapshotItemsCreatesSingleBatchForPeriodicQuotaRule(t *testing.T) {
	db := newProcurementTestDB(t)
	purchaseAt := int64(1700000000)
	snapshot, err := CreateChannelBillingSnapshotWithDB(db, ChannelBillingSnapshot{
		ChannelId:  "channel-1",
		SourceType: ChannelBillingSnapshotSourceManual,
		PurchaseAt: purchaseAt,
		CreatedAt:  purchaseAt,
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}

	items, err := CreateChannelBillingSnapshotItemsWithDB(db, snapshot.Id, "channel-1", []ChannelBillingSnapshotItem{
		{
			ResourceType:    ChannelBillingResourceTypeQuota,
			QuotaType:       "daily",
			QuotaLabel:      "30-day package daily quota",
			LimitAmount:     100,
			RemainingAmount: 100,
			Currency:        "USD",
			ExpiresAt:       purchaseAt + 30*24*60*60,
			SourceRef:       "plan_daily",
		},
	})
	if err != nil {
		t.Fatalf("create snapshot items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len=%d, want 1", len(items))
	}

	var batches []ChannelProcurementBatch
	if err := db.Order("created_at asc").Find(&batches).Error; err != nil {
		t.Fatalf("list procurement batches: %v", err)
	}
	if len(batches) != 1 {
		t.Fatalf("batches len=%d, want 1", len(batches))
	}
	batch := batches[0]
	if batch.CapacityTotal != 3000 {
		t.Fatalf("CapacityTotal=%v, want 3000", batch.CapacityTotal)
	}
	if batch.CapacityRemaining != 3000 {
		t.Fatalf("CapacityRemaining=%v, want 3000", batch.CapacityRemaining)
	}
	if batch.ResetCycle != "daily" {
		t.Fatalf("ResetCycle=%q, want daily", batch.ResetCycle)
	}
	if batch.ExpireAt != purchaseAt+30*24*60*60 {
		t.Fatalf("ExpireAt=%d, want %d", batch.ExpireAt, purchaseAt+30*24*60*60)
	}
}

func TestUnconfiguredProcurementBatchIsNotConsumed(t *testing.T) {
	db := newProcurementTestDB(t)
	_, err := CreateChannelProcurementBatchWithDB(db, ChannelProcurementBatch{
		ChannelId:         "channel-1",
		ResourceType:      "credit",
		QuotaType:         "total",
		ScopeType:         "global",
		CapacityUnit:      "token",
		CapacityTotal:     100,
		CapacityEffective: 100,
		CapacityRemaining: 100,
		CostSource:        ProcurementCostSourceNone,
		CostStatus:        ProcurementCostStatusCostUnconfigured,
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	result, err := ConsumeChannelProcurementBatchesWithDB(db, ProcurementConsumeInput{
		RequestLogID: "log-1",
		ChannelID:    "channel-1",
		CapacityUnit: "token",
		Quantity:     10,
	})
	if err != nil {
		t.Fatalf("consume procurement: %v", err)
	}
	if len(result.Consumptions) != 0 {
		t.Fatalf("consumptions len=%d, want 0", len(result.Consumptions))
	}
	if result.CostSource != ProcurementCostSourceNone {
		t.Fatalf("CostSource=%q, want none", result.CostSource)
	}

	var updated ChannelProcurementBatch
	if err := db.Where("channel_id = ?", "channel-1").Take(&updated).Error; err != nil {
		t.Fatalf("load updated batch: %v", err)
	}
	if updated.CapacityRemaining != 100 {
		t.Fatalf("CapacityRemaining=%v, want 100", updated.CapacityRemaining)
	}
}

func TestUpdateChannelProcurementBatchCostActivatesBatch(t *testing.T) {
	db := newProcurementTestDB(t)
	batch, err := CreateChannelProcurementBatchWithDB(db, ChannelProcurementBatch{
		ChannelId:         "channel-1",
		ResourceType:      "credit",
		QuotaType:         "total",
		ScopeType:         "global",
		CapacityUnit:      "usd_equivalent",
		CapacityTotal:     100,
		CapacityEffective: 100,
		CapacityRemaining: 80,
		CostSource:        ProcurementCostSourceNone,
		CostStatus:        ProcurementCostStatusCostUnconfigured,
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	updated, err := UpdateChannelProcurementBatchCostWithDB(db, batch.Id, ProcurementBatchCostUpdate{
		PurchaseCurrency:  "CNY",
		PurchaseAmount:    700,
		PurchaseFXRate:    1,
		CapacityEffective: 100,
		CostSource:        ProcurementCostSourceActual,
		CostStatus:        ProcurementCostStatusActive,
	})
	if err != nil {
		t.Fatalf("update batch cost: %v", err)
	}
	if updated.CostStatus != ProcurementCostStatusActive {
		t.Fatalf("CostStatus=%q, want active", updated.CostStatus)
	}
	if updated.CostSource != ProcurementCostSourceActual {
		t.Fatalf("CostSource=%q, want actual", updated.CostSource)
	}
	if updated.PurchaseCostAmount != 700 {
		t.Fatalf("PurchaseCostAmount=%v, want 700", updated.PurchaseCostAmount)
	}
	if updated.CostPerUnitAmount != 7 {
		t.Fatalf("CostPerUnitAmount=%v, want 7", updated.CostPerUnitAmount)
	}
}

func TestUpdateChannelProcurementBatchCostRejectsEffectiveCapacityBelowRemaining(t *testing.T) {
	db := newProcurementTestDB(t)
	batch, err := CreateChannelProcurementBatchWithDB(db, ChannelProcurementBatch{
		ChannelId:         "channel-1",
		ResourceType:      "credit",
		QuotaType:         "total",
		ScopeType:         "global",
		CapacityUnit:      "usd_equivalent",
		CapacityTotal:     100,
		CapacityEffective: 100,
		CapacityRemaining: 80,
		CostSource:        ProcurementCostSourceNone,
		CostStatus:        ProcurementCostStatusCostUnconfigured,
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	_, err = UpdateChannelProcurementBatchCostWithDB(db, batch.Id, ProcurementBatchCostUpdate{
		PurchaseCurrency:  "CNY",
		PurchaseAmount:    700,
		PurchaseFXRate:    1,
		CapacityEffective: 50,
	})
	if err == nil {
		t.Fatalf("update batch cost succeeded, want error")
	}
}

func TestUpdateChannelProcurementBatchStatusDisablesAndRestores(t *testing.T) {
	db := newProcurementTestDB(t)
	batch, err := CreateChannelProcurementBatchWithDB(db, ChannelProcurementBatch{
		ChannelId:          "channel-1",
		ResourceType:       "credit",
		QuotaType:          "total",
		ScopeType:          "global",
		CapacityUnit:       "usd_equivalent",
		CapacityTotal:      100,
		CapacityEffective:  100,
		CapacityRemaining:  80,
		PurchaseCurrency:   "CNY",
		PurchaseAmount:     700,
		PurchaseCostAmount: 700,
		CostPerUnitAmount:  7,
		CostSource:         ProcurementCostSourceActual,
		CostStatus:         ProcurementCostStatusActive,
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	disabled, err := UpdateChannelProcurementBatchStatusWithDB(db, batch.Id, ProcurementBatchStatusUpdate{
		CostStatus: ProcurementCostStatusDisabled,
	})
	if err != nil {
		t.Fatalf("disable batch: %v", err)
	}
	if disabled.CostStatus != ProcurementCostStatusDisabled {
		t.Fatalf("CostStatus=%q, want disabled", disabled.CostStatus)
	}

	restored, err := UpdateChannelProcurementBatchStatusWithDB(db, batch.Id, ProcurementBatchStatusUpdate{
		CostStatus: ProcurementCostStatusActive,
	})
	if err != nil {
		t.Fatalf("restore batch: %v", err)
	}
	if restored.CostStatus != ProcurementCostStatusActive {
		t.Fatalf("CostStatus=%q, want active", restored.CostStatus)
	}
}

func TestUpdateChannelProcurementBatchStatusRejectsUnconfiguredRestore(t *testing.T) {
	db := newProcurementTestDB(t)
	batch, err := CreateChannelProcurementBatchWithDB(db, ChannelProcurementBatch{
		ChannelId:         "channel-1",
		ResourceType:      "credit",
		QuotaType:         "total",
		ScopeType:         "global",
		CapacityUnit:      "usd_equivalent",
		CapacityTotal:     100,
		CapacityEffective: 100,
		CapacityRemaining: 80,
		CostSource:        ProcurementCostSourceNone,
		CostStatus:        ProcurementCostStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	_, err = UpdateChannelProcurementBatchStatusWithDB(db, batch.Id, ProcurementBatchStatusUpdate{
		CostStatus: ProcurementCostStatusActive,
	})
	if err == nil {
		t.Fatalf("restore batch succeeded, want error")
	}
}

func TestListRequestProcurementConsumptionsByBatchIDWithDB(t *testing.T) {
	db := newProcurementTestDB(t)
	batch, err := CreateChannelProcurementBatchWithDB(db, ChannelProcurementBatch{
		ChannelId:          "channel-1",
		ResourceType:       "quota",
		QuotaType:          "total",
		ScopeType:          "model",
		ScopeValue:         "gpt-5",
		CapacityUnit:       "token",
		CapacityTotal:      100,
		CapacityEffective:  100,
		CapacityRemaining:  100,
		PurchaseCostAmount: 10,
		CostPerUnitAmount:  0.1,
		CostSource:         ProcurementCostSourceActual,
		CostStatus:         ProcurementCostStatusActive,
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	if _, err := ConsumeChannelProcurementBatchesWithDB(db, ProcurementConsumeInput{
		RequestLogID:        "log-1",
		ChannelID:           "channel-1",
		ScopeType:           "model",
		ScopeValue:          "gpt-5",
		CapacityUnit:        "token",
		Quantity:            40,
		SettlementTruthMode: "hybrid_usage_final",
	}); err != nil {
		t.Fatalf("consume procurement: %v", err)
	}
	rows, err := ListRequestProcurementConsumptionsByBatchIDWithDB(db, batch.Id, 20)
	if err != nil {
		t.Fatalf("list consumptions: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("consumptions len=%d, want 1", len(rows))
	}
	if rows[0].ProcurementBatchId != batch.Id {
		t.Fatalf("ProcurementBatchId=%q, want %q", rows[0].ProcurementBatchId, batch.Id)
	}
}

func TestListProcurementReportWithDB(t *testing.T) {
	db := newProcurementTestDB(t)
	rows := []Log{
		{
			Id:                               "log-1",
			Type:                             LogTypeConsume,
			CreatedAt:                        100,
			ChannelId:                        "channel-1",
			ModelName:                        "gpt-5",
			BillingSellBaseAmount:            10,
			BillingProcurementCostBaseAmount: 4,
			BillingProcurementCostSource:     ProcurementCostSourceActual,
			BillingGrossProfitBaseAmount:     6,
		},
		{
			Id:                           "log-2",
			Type:                         LogTypeConsume,
			CreatedAt:                    120,
			ChannelId:                    "channel-1",
			ModelName:                    "gpt-5",
			BillingSellBaseAmount:        8,
			BillingProcurementCostSource: ProcurementCostSourceNone,
		},
		{
			Id:                           "log-3",
			Type:                         LogTypeConsume,
			CreatedAt:                    130,
			ChannelId:                    "channel-2",
			ModelName:                    "gpt-5-mini",
			BillingSellBaseAmount:        5,
			BillingProcurementCostSource: ProcurementCostSourceZeroCost,
			BillingGrossProfitBaseAmount: 5,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed logs: %v", err)
	}

	report, err := ListProcurementReportWithDB(db, ProcurementReportQuery{
		StartAt: 90,
		EndAt:   140,
		GroupBy: ProcurementReportGroupByChannel,
	})
	if err != nil {
		t.Fatalf("list report: %v", err)
	}
	if report.RequestCount != 3 {
		t.Fatalf("RequestCount=%d, want 3", report.RequestCount)
	}
	if report.ConfiguredCostRequestCount != 2 {
		t.Fatalf("ConfiguredCostRequestCount=%d, want 2", report.ConfiguredCostRequestCount)
	}
	if report.UnconfiguredCostRequestCount != 1 {
		t.Fatalf("UnconfiguredCostRequestCount=%d, want 1", report.UnconfiguredCostRequestCount)
	}
	if report.SellBaseAmount != 23 {
		t.Fatalf("SellBaseAmount=%v, want 23", report.SellBaseAmount)
	}
	if report.ConfiguredSellBaseAmount != 15 {
		t.Fatalf("ConfiguredSellBaseAmount=%v, want 15", report.ConfiguredSellBaseAmount)
	}
	if report.UnconfiguredSellBaseAmount != 8 {
		t.Fatalf("UnconfiguredSellBaseAmount=%v, want 8", report.UnconfiguredSellBaseAmount)
	}
	if report.ProcurementCostBaseAmount != 4 {
		t.Fatalf("ProcurementCostBaseAmount=%v, want 4", report.ProcurementCostBaseAmount)
	}
	if report.GrossProfitBaseAmount != 11 {
		t.Fatalf("GrossProfitBaseAmount=%v, want 11", report.GrossProfitBaseAmount)
	}
	if report.GrossMargin != 11.0/15.0 {
		t.Fatalf("GrossMargin=%v, want %v", report.GrossMargin, 11.0/15.0)
	}

	unconfiguredReport, err := ListProcurementReportWithDB(db, ProcurementReportQuery{
		StartAt:   90,
		EndAt:     140,
		GroupBy:   ProcurementReportGroupByChannel,
		CostScope: ProcurementReportCostScopeUnconfigured,
	})
	if err != nil {
		t.Fatalf("list unconfigured report: %v", err)
	}
	if unconfiguredReport.RequestCount != 1 {
		t.Fatalf("unconfigured RequestCount=%d, want 1", unconfiguredReport.RequestCount)
	}
	if unconfiguredReport.ConfiguredCostRequestCount != 0 {
		t.Fatalf("unconfigured ConfiguredCostRequestCount=%d, want 0", unconfiguredReport.ConfiguredCostRequestCount)
	}
	if unconfiguredReport.UnconfiguredCostRequestCount != 1 {
		t.Fatalf("unconfigured UnconfiguredCostRequestCount=%d, want 1", unconfiguredReport.UnconfiguredCostRequestCount)
	}
	if unconfiguredReport.SellBaseAmount != 8 {
		t.Fatalf("unconfigured SellBaseAmount=%v, want 8", unconfiguredReport.SellBaseAmount)
	}
	if unconfiguredReport.GrossProfitBaseAmount != 0 {
		t.Fatalf("unconfigured GrossProfitBaseAmount=%v, want 0", unconfiguredReport.GrossProfitBaseAmount)
	}
}
