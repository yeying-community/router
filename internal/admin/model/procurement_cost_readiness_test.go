package model

import (
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newProcurementCostReadinessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ChannelProcurementBatch{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return db
}

func TestValidateChannelModelProcurementCostReadyAcceptsActualAndZeroCost(t *testing.T) {
	for _, source := range []string{ProcurementCostSourceActual, ProcurementCostSourceZeroCost} {
		t.Run(source, func(t *testing.T) {
			db := newProcurementCostReadinessTestDB(t)
			unitCost := 0.0
			if source == ProcurementCostSourceActual {
				unitCost = 0.001
			}
			if err := db.Create(&ChannelProcurementBatch{
				Id:                "batch-" + source,
				ChannelId:         "channel-1",
				ScopeType:         "model",
				ScopeValue:        "model-1",
				CapacityUnit:      "token",
				CapacityTotal:     1000,
				CapacityEffective: 1000,
				CapacityRemaining: 1000,
				CostPerUnitAmount: unitCost,
				CostSource:        source,
				CostStatus:        ProcurementCostStatusActive,
			}).Error; err != nil {
				t.Fatalf("create batch: %v", err)
			}
			err := ValidateChannelModelProcurementCostReadyWithDB(db, ChannelModel{
				ChannelId: "channel-1",
				Model:     "model-1",
				PriceUnit: ProviderPriceUnitPer1KTokens,
				Currency:  ProviderPriceCurrencyUSD,
			})
			if err != nil {
				t.Fatalf("ValidateChannelModelProcurementCostReadyWithDB: %v", err)
			}
		})
	}
}

func TestValidateChannelModelProcurementCostReadyRejectsEstimatedAndUnitMismatch(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		source     string
		unit       string
		wantReason string
	}{
		{name: "estimated", source: ProcurementCostSourceEstimated, unit: "token", wantReason: "缺少正式采购成本"},
		{name: "unit mismatch", source: ProcurementCostSourceActual, unit: "request", wantReason: "容量单位必须匹配"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			db := newProcurementCostReadinessTestDB(t)
			if err := db.Create(&ChannelProcurementBatch{
				Id:                "batch-1",
				ChannelId:         "channel-1",
				ScopeType:         "global",
				CapacityUnit:      testCase.unit,
				CapacityTotal:     1000,
				CapacityEffective: 1000,
				CapacityRemaining: 1000,
				CostPerUnitAmount: 0.001,
				CostSource:        testCase.source,
				CostStatus:        ProcurementCostStatusActive,
			}).Error; err != nil {
				t.Fatalf("create batch: %v", err)
			}
			err := ValidateChannelModelProcurementCostReadyWithDB(db, ChannelModel{
				ChannelId: "channel-1",
				Model:     "model-1",
				PriceUnit: ProviderPriceUnitPer1KTokens,
				Currency:  ProviderPriceCurrencyUSD,
			})
			if err == nil || !strings.Contains(err.Error(), testCase.wantReason) {
				t.Fatalf("error=%v, want reason %q", err, testCase.wantReason)
			}
		})
	}
}
