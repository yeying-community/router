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
		ChannelId:         "channel-1",
		ResourceType:      "quota",
		QuotaType:         "total",
		ScopeType:         "model",
		ScopeValue:        "gpt-5",
		CapacityUnit:      "token",
		CapacityTotal:     100,
		CapacityEffective: 100,
		CapacityRemaining: 100,
		PurchaseCostCNY:   10,
		CostPerUnitCNY:    0.1,
		CostSource:        ProcurementCostSourceActual,
		CostStatus:        ProcurementCostStatusActive,
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
	if result.TotalCostCNY != 4 {
		t.Fatalf("TotalCostCNY=%v, want 4", result.TotalCostCNY)
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
		CostPerUnitCNY:    0.2,
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

func TestUpdateLogProcurementCostObservationWithDB(t *testing.T) {
	db := newProcurementTestDB(t)
	logRow := Log{Id: "log-1", BillingSellAmountCNY: 10}
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
	if updated.BillingProcurementCostCNY != 4 {
		t.Fatalf("BillingProcurementCostCNY=%v, want 4", updated.BillingProcurementCostCNY)
	}
	if updated.BillingGrossProfitCNY != 6 {
		t.Fatalf("BillingGrossProfitCNY=%v, want 6", updated.BillingGrossProfitCNY)
	}
	if updated.BillingGrossMargin != 0.6 {
		t.Fatalf("BillingGrossMargin=%v, want 0.6", updated.BillingGrossMargin)
	}
}

func TestCreateBillingSnapshotItemsCreatesUnconfiguredProcurementBatch(t *testing.T) {
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
	if updated.PurchaseCostCNY != 700 {
		t.Fatalf("PurchaseCostCNY=%v, want 700", updated.PurchaseCostCNY)
	}
	if updated.CostPerUnitCNY != 7 {
		t.Fatalf("CostPerUnitCNY=%v, want 7", updated.CostPerUnitCNY)
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
		ChannelId:         "channel-1",
		ResourceType:      "credit",
		QuotaType:         "total",
		ScopeType:         "global",
		CapacityUnit:      "usd_equivalent",
		CapacityTotal:     100,
		CapacityEffective: 100,
		CapacityRemaining: 80,
		PurchaseCurrency:  "CNY",
		PurchaseAmount:    700,
		PurchaseCostCNY:   700,
		CostPerUnitCNY:    7,
		CostSource:        ProcurementCostSourceActual,
		CostStatus:        ProcurementCostStatusActive,
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
		ChannelId:         "channel-1",
		ResourceType:      "quota",
		QuotaType:         "total",
		ScopeType:         "model",
		ScopeValue:        "gpt-5",
		CapacityUnit:      "token",
		CapacityTotal:     100,
		CapacityEffective: 100,
		CapacityRemaining: 100,
		PurchaseCostCNY:   10,
		CostPerUnitCNY:    0.1,
		CostSource:        ProcurementCostSourceActual,
		CostStatus:        ProcurementCostStatusActive,
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
