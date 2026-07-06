package billing

import (
	"math"
	"testing"

	adminmodel "github.com/yeying-community/router/internal/admin/model"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

func TestResolveImageBillingMode(t *testing.T) {
	tests := []struct {
		name    string
		pricing adminmodel.ResolvedModelPricing
		want    ImageBillingMode
	}{
		{
			name: "per image",
			pricing: adminmodel.ResolvedModelPricing{
				PriceUnit: adminmodel.ProviderPriceUnitPerImage,
			},
			want: ImageBillingModePerImage,
		},
		{
			name: "per request",
			pricing: adminmodel.ResolvedModelPricing{
				PriceUnit: adminmodel.ProviderPriceUnitPerRequest,
			},
			want: ImageBillingModePerCall,
		},
		{
			name: "per task",
			pricing: adminmodel.ResolvedModelPricing{
				PriceUnit: adminmodel.ProviderPriceUnitPerTask,
			},
			want: ImageBillingModePerCall,
		},
		{
			name: "token based",
			pricing: adminmodel.ResolvedModelPricing{
				PriceUnit: adminmodel.ProviderPriceUnitPer1KTokens,
			},
			want: ImageBillingModeTokenBased,
		},
		{
			name: "unknown",
			pricing: adminmodel.ResolvedModelPricing{
				PriceUnit: "per_pixel",
			},
			want: ImageBillingModeUnsupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveImageBillingMode(tt.pricing); got != tt.want {
				t.Fatalf("ResolveImageBillingMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestComputeImageBillingSnapshotByMode(t *testing.T) {
	t.Run("per image uses image count and multiplier", func(t *testing.T) {
		pricing := adminmodel.ResolvedModelPricing{
			Model:      "dall-e-3",
			PriceUnit:  adminmodel.ProviderPriceUnitPerImage,
			InputPrice: 0.04,
			Currency:   adminmodel.ProviderPriceCurrencyUSD,
		}

		snapshot, err := ComputeImageBillingSnapshot(2, 1.5, pricing, 1)
		if err != nil {
			t.Fatalf("ComputeImageBillingSnapshot() error = %v", err)
		}
		if snapshot.InputQuantity != 3 {
			t.Fatalf("InputQuantity = %v, want 3", snapshot.InputQuantity)
		}
		if snapshot.InputAmount != 0.12 {
			t.Fatalf("InputAmount = %v, want 0.12", snapshot.InputAmount)
		}
	})

	t.Run("per call ignores image count multiplier", func(t *testing.T) {
		pricing := adminmodel.ResolvedModelPricing{
			Model:      "foo-image",
			PriceUnit:  adminmodel.ProviderPriceUnitPerRequest,
			InputPrice: 0.5,
			Currency:   adminmodel.ProviderPriceCurrencyUSD,
		}

		snapshot, err := ComputeImageBillingSnapshot(4, 3, pricing, 1)
		if err != nil {
			t.Fatalf("ComputeImageBillingSnapshot() error = %v", err)
		}
		if snapshot.InputQuantity != 1 {
			t.Fatalf("InputQuantity = %v, want 1", snapshot.InputQuantity)
		}
		if snapshot.InputAmount != 0.5 {
			t.Fatalf("InputAmount = %v, want 0.5", snapshot.InputAmount)
		}
	})

	t.Run("token based returns explicit error", func(t *testing.T) {
		pricing := adminmodel.ResolvedModelPricing{
			Model:      "gpt-image-2",
			PriceUnit:  adminmodel.ProviderPriceUnitPer1KTokens,
			InputPrice: 0.008,
			Currency:   adminmodel.ProviderPriceCurrencyUSD,
		}

		if _, err := ComputeImageBillingSnapshot(1, 1, pricing, 1); err == nil {
			t.Fatal("ComputeImageBillingSnapshot() error = nil, want error")
		}
	})
}

func TestComputeTraditionalImageTokenBasedBillingSnapshot(t *testing.T) {
	pricing := adminmodel.ResolvedModelPricing{
		Model:       "gpt-image-2",
		PriceUnit:   adminmodel.ProviderPriceUnitPer1KTokens,
		InputPrice:  0.008,
		OutputPrice: 0.03,
		Currency:    adminmodel.ProviderPriceCurrencyUSD,
	}

	snapshot, err := ComputeTraditionalImageTokenBasedBillingSnapshot(100, 1056, pricing, 1)
	if err != nil {
		t.Fatalf("ComputeTraditionalImageTokenBasedBillingSnapshot() error = %v", err)
	}
	if snapshot.InputQuantity != 100 {
		t.Fatalf("InputQuantity = %v, want 100", snapshot.InputQuantity)
	}
	if snapshot.OutputQuantity != 1056 {
		t.Fatalf("OutputQuantity = %v, want 1056", snapshot.OutputQuantity)
	}
	if snapshot.Amount <= 0 {
		t.Fatalf("Amount = %v, want > 0", snapshot.Amount)
	}
	if snapshot.ChargeAmount <= 0 {
		t.Fatalf("ChargeAmount = %d, want > 0", snapshot.ChargeAmount)
	}
}

func TestComputeTokenBasedBillingSnapshot(t *testing.T) {
	pricing := adminmodel.ResolvedModelPricing{
		Model:       "gpt-image-2",
		PriceUnit:   adminmodel.ProviderPriceUnitPer1KTokens,
		InputPrice:  0.005,
		OutputPrice: 0.03,
		Currency:    adminmodel.ProviderPriceCurrencyUSD,
	}

	snapshot, err := ComputeTokenBasedBillingSnapshot(100, 7033.333333333333, pricing, 1)
	if err != nil {
		t.Fatalf("ComputeTokenBasedBillingSnapshot() error = %v", err)
	}
	if snapshot.InputAmount != 0.0005 {
		t.Fatalf("InputAmount = %v, want 0.0005", snapshot.InputAmount)
	}
	if math.Abs(snapshot.OutputAmount-0.211) > 1e-9 {
		t.Fatalf("OutputAmount = %v, want 0.211", snapshot.OutputAmount)
	}
}

func TestComputeTextBillingSnapshotWithUsageSplitsCachePricing(t *testing.T) {
	pricing := adminmodel.ResolvedModelPricing{
		Model:       "gpt-5.4",
		PriceUnit:   adminmodel.ProviderPriceUnitPer1KTokens,
		InputPrice:  0.01,
		OutputPrice: 0.03,
		Currency:    adminmodel.ProviderPriceCurrencyUSD,
		PriceComponents: []adminmodel.ProviderModelPriceComponentDetail{
			{
				Component:  adminmodel.ProviderModelPriceComponentTextCacheRead,
				InputPrice: 0.002,
				PriceUnit:  adminmodel.ProviderPriceUnitPer1KTokens,
				Currency:   adminmodel.ProviderPriceCurrencyUSD,
			},
			{
				Component:  adminmodel.ProviderModelPriceComponentTextCacheWrite,
				InputPrice: 0.012,
				PriceUnit:  adminmodel.ProviderPriceUnitPer1KTokens,
				Currency:   adminmodel.ProviderPriceCurrencyUSD,
			},
		},
	}
	usage := relaymodel.Usage{
		PromptTokens:     1000,
		CompletionTokens: 2000,
		PromptTokensDetails: &relaymodel.PromptTokensDetails{
			CacheReadTokens:     300,
			CacheCreationTokens: 100,
		},
	}

	snapshot, err := ComputeTextBillingSnapshotWithUsage(usage, pricing, 1)
	if err != nil {
		t.Fatalf("ComputeTextBillingSnapshotWithUsage() error = %v", err)
	}
	if snapshot.InputQuantity != 1000 {
		t.Fatalf("InputQuantity = %v, want 1000", snapshot.InputQuantity)
	}
	if snapshot.OutputQuantity != 2000 {
		t.Fatalf("OutputQuantity = %v, want 2000", snapshot.OutputQuantity)
	}
	if snapshot.CacheReadQuantity != 300 {
		t.Fatalf("CacheReadQuantity = %v, want 300", snapshot.CacheReadQuantity)
	}
	if snapshot.CacheWriteQuantity != 100 {
		t.Fatalf("CacheWriteQuantity = %v, want 100", snapshot.CacheWriteQuantity)
	}
	wantInputAmount := 600*0.01/1000 + 300*0.002/1000 + 100*0.012/1000
	wantCacheReadAmount := 300 * 0.002 / 1000
	wantCacheWriteAmount := 100 * 0.012 / 1000
	if math.Abs(snapshot.InputAmount-wantInputAmount) > 1e-12 {
		t.Fatalf("InputAmount = %v, want %v", snapshot.InputAmount, wantInputAmount)
	}
	if math.Abs(snapshot.CacheReadAmount-wantCacheReadAmount) > 1e-12 {
		t.Fatalf("CacheReadAmount = %v, want %v", snapshot.CacheReadAmount, wantCacheReadAmount)
	}
	if math.Abs(snapshot.CacheWriteAmount-wantCacheWriteAmount) > 1e-12 {
		t.Fatalf("CacheWriteAmount = %v, want %v", snapshot.CacheWriteAmount, wantCacheWriteAmount)
	}
	if math.Abs(snapshot.OutputAmount-0.06) > 1e-12 {
		t.Fatalf("OutputAmount = %v, want 0.06", snapshot.OutputAmount)
	}
	if math.Abs(snapshot.Amount-(wantInputAmount+0.06)) > 1e-12 {
		t.Fatalf("Amount = %v, want %v", snapshot.Amount, wantInputAmount+0.06)
	}
}

func TestComputeTextBillingSnapshotWithUsageFallsBackToInputPriceForCache(t *testing.T) {
	pricing := adminmodel.ResolvedModelPricing{
		Model:       "gpt-5.4",
		PriceUnit:   adminmodel.ProviderPriceUnitPer1KTokens,
		InputPrice:  0.01,
		OutputPrice: 0.03,
		Currency:    adminmodel.ProviderPriceCurrencyUSD,
	}
	usage := relaymodel.Usage{
		PromptTokens:     1000,
		CompletionTokens: 0,
		PromptTokensDetails: &relaymodel.PromptTokensDetails{
			CachedTokens: 400,
		},
	}

	snapshot, err := ComputeTextBillingSnapshotWithUsage(usage, pricing, 1)
	if err != nil {
		t.Fatalf("ComputeTextBillingSnapshotWithUsage() error = %v", err)
	}
	if math.Abs(snapshot.InputAmount-0.01) > 1e-12 {
		t.Fatalf("InputAmount = %v, want 0.01", snapshot.InputAmount)
	}
}

func TestComputeTextBillingSnapshotWithUsageSelectsTieredCachePricing(t *testing.T) {
	pricing := adminmodel.ResolvedModelPricing{
		Model:       "gemini-2.5-pro",
		PriceUnit:   adminmodel.ProviderPriceUnitPer1KTokens,
		InputPrice:  0.00125,
		OutputPrice: 0.01,
		Currency:    adminmodel.ProviderPriceCurrencyUSD,
		PriceComponents: []adminmodel.ProviderModelPriceComponentDetail{
			{
				Component:  adminmodel.ProviderModelPriceComponentTextCacheRead,
				Condition:  "mode=standard;prompt_tokens_lte=200000",
				InputPrice: 0.000125,
				PriceUnit:  adminmodel.ProviderPriceUnitPer1KTokens,
				Currency:   adminmodel.ProviderPriceCurrencyUSD,
			},
			{
				Component:  adminmodel.ProviderModelPriceComponentTextCacheRead,
				Condition:  "mode=standard;prompt_tokens_gt=200000",
				InputPrice: 0.00025,
				PriceUnit:  adminmodel.ProviderPriceUnitPer1KTokens,
				Currency:   adminmodel.ProviderPriceCurrencyUSD,
			},
		},
	}
	usage := relaymodel.Usage{
		PromptTokens:     200001,
		CompletionTokens: 0,
		PromptTokensDetails: &relaymodel.PromptTokensDetails{
			CachedTokens: 1000,
		},
	}

	snapshot, err := ComputeTextBillingSnapshotWithUsage(usage, pricing, 1)
	if err != nil {
		t.Fatalf("ComputeTextBillingSnapshotWithUsage() error = %v", err)
	}
	wantCacheReadAmount := 1000 * 0.00025 / 1000
	if math.Abs(snapshot.CacheReadAmount-wantCacheReadAmount) > 1e-12 {
		t.Fatalf("CacheReadAmount = %v, want %v", snapshot.CacheReadAmount, wantCacheReadAmount)
	}
}

func TestComputeResponseImageToolTokenBasedBillingSnapshot(t *testing.T) {
	pricing := adminmodel.ResolvedModelPricing{
		Model:       "gpt-image-2",
		PriceUnit:   adminmodel.ProviderPriceUnitPer1KTokens,
		InputPrice:  0.008,
		OutputPrice: 0.03,
		Currency:    adminmodel.ProviderPriceCurrencyUSD,
	}

	snapshot, err := ComputeResponseImageToolTokenBasedBillingSnapshot(7033.333333333333, pricing, 1)
	if err != nil {
		t.Fatalf("ComputeResponseImageToolTokenBasedBillingSnapshot() error = %v", err)
	}
	if snapshot.InputQuantity != 0 {
		t.Fatalf("InputQuantity = %v, want 0", snapshot.InputQuantity)
	}
	if math.Abs(snapshot.OutputQuantity-7033.333333333333) > 1e-9 {
		t.Fatalf("OutputQuantity = %v, want %v", snapshot.OutputQuantity, 7033.333333333333)
	}
	if snapshot.InputAmount != 0 {
		t.Fatalf("InputAmount = %v, want 0", snapshot.InputAmount)
	}
	if math.Abs(snapshot.OutputAmount-0.211) > 1e-9 {
		t.Fatalf("OutputAmount = %v, want 0.211", snapshot.OutputAmount)
	}
}

func TestComputeExplicitAmountBillingSnapshot(t *testing.T) {
	pricing := adminmodel.ResolvedModelPricing{
		Model:     "gpt-image-2",
		PriceUnit: adminmodel.ProviderPriceUnitPer1KTokens,
		Currency:  adminmodel.ProviderPriceCurrencyUSD,
	}
	snapshot, err := ComputeExplicitAmountBillingSnapshot(4454, 7033.333333333333, 0.035332, 0.211, pricing, 1, true)
	if err != nil {
		t.Fatalf("ComputeExplicitAmountBillingSnapshot() error = %v", err)
	}
	if math.Abs(snapshot.InputAmount-0.035332) > 1e-9 {
		t.Fatalf("InputAmount = %v, want 0.035332", snapshot.InputAmount)
	}
	if math.Abs(snapshot.OutputAmount-0.211) > 1e-9 {
		t.Fatalf("OutputAmount = %v, want 0.211", snapshot.OutputAmount)
	}
	if math.Abs(snapshot.Amount-0.246332) > 1e-9 {
		t.Fatalf("Amount = %v, want 0.246332", snapshot.Amount)
	}
}

func TestDecidePricingKeepsCurrentChargeForOfficialAnchor(t *testing.T) {
	decision := DecidePricing(PricingDecisionInput{
		OfficialAnchor: MoneyAmount{Amount: 1, Currency: adminmodel.BillingCurrencyCodeCNY},
		CurrentCharge:  MoneyAmount{Amount: 123, Currency: adminmodel.BillingCurrencyCodeYYC},
		Policy: PricingPolicy{
			OfficialMarkup: 1,
		},
	})
	if decision.Reason != PricingDecisionReasonOfficialAnchor {
		t.Fatalf("Reason=%q, want %q", decision.Reason, PricingDecisionReasonOfficialAnchor)
	}
	if decision.SelectedCharge.Amount != 123 {
		t.Fatalf("SelectedCharge.Amount=%v, want 123", decision.SelectedCharge.Amount)
	}
	if decision.SelectedCharge.Currency != adminmodel.BillingCurrencyCodeYYC {
		t.Fatalf("SelectedCharge.Currency=%q, want YYC", decision.SelectedCharge.Currency)
	}
}

func TestDecidePricingUsesCostFloorWhenHigher(t *testing.T) {
	decision := DecidePricing(PricingDecisionInput{
		OfficialAnchor:  MoneyAmount{Amount: 1, Currency: adminmodel.BillingCurrencyCodeCNY},
		CurrentCharge:   MoneyAmount{Amount: 1, Currency: adminmodel.BillingCurrencyCodeYYC},
		ProcurementCost: MoneyAmount{Amount: 10, Currency: adminmodel.BillingCurrencyCodeCNY},
		Policy: PricingPolicy{
			OfficialMarkup: 1,
			TargetMargin:   0.2,
			RiskBuffer:     0.1,
		},
	})
	if decision.Reason != PricingDecisionReasonCostFloor {
		t.Fatalf("Reason=%q, want %q", decision.Reason, PricingDecisionReasonCostFloor)
	}
	want := 10 * 1.1 / 0.8
	if math.Abs(decision.CostFloor.Amount-want) > 1e-9 {
		t.Fatalf("CostFloor.Amount=%v, want %v", decision.CostFloor.Amount, want)
	}
	if decision.SelectedSell.Currency != adminmodel.BillingCurrencyCodeCNY {
		t.Fatalf("SelectedSell.Currency=%q, want CNY", decision.SelectedSell.Currency)
	}
	if decision.SelectedCharge.Currency != adminmodel.BillingCurrencyCodeYYC {
		t.Fatalf("SelectedCharge.Currency=%q, want YYC", decision.SelectedCharge.Currency)
	}
}

func TestProcurementConsumptionCandidatesFromSnapshotPreferCurrencyEquivalent(t *testing.T) {
	snapshot := &BillingSnapshot{
		PriceUnit:      adminmodel.ProviderPriceUnitPer1KTokens,
		Currency:       adminmodel.ProviderPriceCurrencyUSD,
		InputQuantity:  1000,
		OutputQuantity: 2000,
		Amount:         0.25,
	}

	got := procurementConsumptionCandidatesFromSnapshot(snapshot)

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
