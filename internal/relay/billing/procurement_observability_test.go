package billing

import (
	"testing"

	adminmodel "github.com/yeying-community/router/internal/admin/model"
)

func TestApplyProcurementCostObservationMapsSettlementTruthMode(t *testing.T) {
	cases := []struct {
		name           string
		settlementMode string
		priceUnit      string
		wantMode       string
		wantConfidence string
	}{
		{
			name:           "provider usage",
			settlementMode: "provider_usage_final",
			wantMode:       SettlementTruthModeReturnedUsageFinal,
			wantConfidence: ProcurementCostConfidenceReturnedUsage,
		},
		{
			name:           "local estimate",
			settlementMode: "local_estimate_final",
			wantMode:       SettlementTruthModeLocalEstimateFinal,
			wantConfidence: ProcurementCostConfidenceLocalEstimate,
		},
		{
			name:           "estimate only",
			settlementMode: "estimate_only",
			wantMode:       SettlementTruthModeUnitBasedFinal,
			wantConfidence: ProcurementCostConfidenceUnitBased,
		},
		{
			name:           "usage final",
			settlementMode: "usage_final",
			wantMode:       SettlementTruthModeHybridUsageFinal,
			wantConfidence: ProcurementCostConfidenceHybridUsage,
		},
		{
			name:           "unit price fallback",
			priceUnit:      adminmodel.ProviderPriceUnitPerImage,
			wantMode:       SettlementTruthModeUnitBasedFinal,
			wantConfidence: ProcurementCostConfidenceUnitBased,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logRow := &adminmodel.Log{
				BillingSettlementMode: tc.settlementMode,
				BillingPriceUnit:      tc.priceUnit,
			}
			ApplyProcurementCostObservation(logRow)
			if logRow.BillingSettlementTruthMode != tc.wantMode {
				t.Fatalf("BillingSettlementTruthMode=%q, want %q", logRow.BillingSettlementTruthMode, tc.wantMode)
			}
			if logRow.BillingProcurementCostConfidence != tc.wantConfidence {
				t.Fatalf("BillingProcurementCostConfidence=%q, want %q", logRow.BillingProcurementCostConfidence, tc.wantConfidence)
			}
		})
	}
}

func TestApplyProcurementCostObservationDoesNotInventGrossProfitWithoutCost(t *testing.T) {
	logRow := &adminmodel.Log{
		BillingAmount:         1,
		BillingCurrency:       adminmodel.BillingCurrencyCodeCNY,
		BillingYYCAmount:      1000,
		BillingSettlementMode: "usage_final",
	}

	ApplyProcurementCostObservation(logRow)

	if logRow.BillingProcurementCostSource != adminmodel.ProcurementCostSourceNone {
		t.Fatalf("BillingProcurementCostSource=%q, want %q", logRow.BillingProcurementCostSource, adminmodel.ProcurementCostSourceNone)
	}
	if logRow.BillingGrossProfitCNY != 0 {
		t.Fatalf("BillingGrossProfitCNY=%v, want 0 without procurement cost", logRow.BillingGrossProfitCNY)
	}
	if logRow.BillingGrossMargin != 0 {
		t.Fatalf("BillingGrossMargin=%v, want 0 without procurement cost", logRow.BillingGrossMargin)
	}
}

func TestProcurementCapacityUnit(t *testing.T) {
	cases := []struct {
		priceUnit string
		want      string
	}{
		{priceUnit: adminmodel.ProviderPriceUnitPerImage, want: "image"},
		{priceUnit: adminmodel.ProviderPriceUnitPerRequest, want: "request"},
		{priceUnit: adminmodel.ProviderPriceUnitPer1KChars, want: "char"},
		{priceUnit: adminmodel.ProviderPriceUnitPerSecond, want: "second"},
		{priceUnit: adminmodel.ProviderPriceUnitPerMinute, want: "minute"},
		{priceUnit: adminmodel.ProviderPriceUnitPerVideo, want: "video"},
		{priceUnit: adminmodel.ProviderPriceUnitPer1KTokens, want: "token"},
	}

	for _, tc := range cases {
		t.Run(tc.priceUnit, func(t *testing.T) {
			got := procurementCapacityUnit(&adminmodel.Log{BillingPriceUnit: tc.priceUnit})
			if got != tc.want {
				t.Fatalf("procurementCapacityUnit()=%q, want %q", got, tc.want)
			}
		})
	}
}
