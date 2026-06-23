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
		BillingChargeAmount:   1000,
		BillingSettlementMode: "usage_final",
	}

	ApplyProcurementCostObservation(logRow)

	if logRow.BillingProcurementCostSource != adminmodel.ProcurementCostSourceNone {
		t.Fatalf("BillingProcurementCostSource=%q, want %q", logRow.BillingProcurementCostSource, adminmodel.ProcurementCostSourceNone)
	}
	if logRow.BillingGrossProfitBaseAmount != 0 {
		t.Fatalf("BillingGrossProfitBaseAmount=%v, want 0 without procurement cost", logRow.BillingGrossProfitBaseAmount)
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

func TestProcurementConsumptionCandidatesPreferCurrencyEquivalent(t *testing.T) {
	logRow := &adminmodel.Log{
		BillingPriceUnit:      adminmodel.ProviderPriceUnitPer1KTokens,
		BillingCurrency:       adminmodel.ProviderPriceCurrencyUSD,
		BillingAmount:         0.25,
		BillingInputQuantity:  1000,
		BillingOutputQuantity: 2000,
	}

	got := procurementConsumptionCandidates(logRow)

	if len(got) != 2 {
		t.Fatalf("candidates len=%d, want 2", len(got))
	}
	if got[0].CapacityUnit != "usd_equivalent" || got[0].Quantity != 0.25 {
		t.Fatalf("first candidate=%+v, want usd_equivalent/0.25", got[0])
	}
	if got[1].CapacityUnit != "token" || got[1].Quantity != 3000 {
		t.Fatalf("second candidate=%+v, want token/3000", got[1])
	}
}

func TestProcurementConsumptionCandidatesFallbackToUsageUnit(t *testing.T) {
	logRow := &adminmodel.Log{
		BillingPriceUnit:      adminmodel.ProviderPriceUnitPerImage,
		BillingInputQuantity:  2,
		BillingOutputQuantity: 0,
	}

	got := procurementConsumptionCandidates(logRow)

	if len(got) != 1 {
		t.Fatalf("candidates len=%d, want 1", len(got))
	}
	if got[0].CapacityUnit != "image" || got[0].Quantity != 2 {
		t.Fatalf("candidate=%+v, want image/2", got[0])
	}
}
