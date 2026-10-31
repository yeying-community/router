package model

import "testing"

func TestProviderCatalogCorrection20261026IncludesMiniMaxAndZhipuPricing(t *testing.T) {
	details := providerMigrationDetailsByProvider(t)

	tests := []struct {
		provider       string
		model          string
		input          float64
		output         float64
		currency       string
		componentCount int
		cacheRead      float64
	}{
		{"minimax", "MiniMax-M3.1-Flash", 0.00015, 0.0006, "USD", 1, 0.000015},
		{"minimax", "MiniMax-M3", 0.0003, 0.0012, "USD", 4, 0},
		{"zhipu", "glm-5.3", 0.008, 0.028, "CNY", 1, 0.002},
		{"zhipu", "glm-5.3-flash", 0.0008, 0.0028, "CNY", 1, 0.00023},
		{"zhipu", "glm-5.3-flashx", 0.002, 0.007, "CNY", 1, 0.00057},
	}

	for _, tt := range tests {
		got, ok := details[tt.provider][tt.model]
		if !ok {
			t.Fatalf("%s/%s missing from merged catalog", tt.provider, tt.model)
		}
		if got.Type != ProviderModelTypeText || got.InputPrice != tt.input || got.OutputPrice != tt.output || got.PriceUnit != ProviderPriceUnitPer1KTokens || got.Currency != tt.currency {
			t.Fatalf("%s/%s type/pricing=%q/%v/%v/%q/%q", tt.provider, tt.model, got.Type, got.InputPrice, got.OutputPrice, got.PriceUnit, got.Currency)
		}
		if len(got.PriceComponents) != tt.componentCount {
			t.Fatalf("%s/%s component count=%d, want %d", tt.provider, tt.model, len(got.PriceComponents), tt.componentCount)
		}
		if tt.cacheRead > 0 {
			component := got.PriceComponents[0]
			if component.Component != ProviderModelPriceComponentTextCacheRead || component.InputPrice != tt.cacheRead || component.PriceUnit != ProviderPriceUnitPer1KTokens || component.Currency != tt.currency {
				t.Fatalf("%s/%s cache pricing=%#v", tt.provider, tt.model, component)
			}
		}
	}
}

func TestProviderCatalogCorrection20261026MiniMaxM3TokenTiers(t *testing.T) {
	detail := providerMigrationDetailsByProvider(t)["minimax"]["MiniMax-M3"]
	wants := map[string]struct {
		component string
		input     float64
		output    float64
	}{
		"prompt_tokens_lte=512000": {ProviderModelPriceComponentText, 0.0003, 0.0012},
		"prompt_tokens_gt=512000":  {ProviderModelPriceComponentText, 0.0006, 0.0024},
		"cache_lte":                {ProviderModelPriceComponentTextCacheRead, 0.00006, 0},
		"cache_gt":                 {ProviderModelPriceComponentTextCacheRead, 0.00012, 0},
	}
	for _, component := range detail.PriceComponents {
		key := component.Condition
		if component.Component == ProviderModelPriceComponentTextCacheRead {
			if component.Condition == "prompt_tokens_lte=512000" {
				key = "cache_lte"
			} else {
				key = "cache_gt"
			}
		}
		want, ok := wants[key]
		if !ok {
			t.Fatalf("unexpected MiniMax-M3 price component %#v", component)
		}
		if component.Component != want.component || component.InputPrice != want.input || component.OutputPrice != want.output {
			t.Fatalf("MiniMax-M3 component %q=%#v", key, component)
		}
		delete(wants, key)
	}
	if len(wants) != 0 {
		t.Fatalf("missing MiniMax-M3 price components: %#v", wants)
	}

	base := ResolvedModelPricing{
		Model: detail.Model, Provider: "minimax", Type: detail.Type,
		InputPrice: detail.InputPrice, OutputPrice: detail.OutputPrice,
		PriceUnit: detail.PriceUnit, Currency: detail.Currency,
		PriceComponents: detail.PriceComponents,
	}
	low := ResolveTextUsagePricing(base, "/v1/chat/completions", 512000, 1)
	high := ResolveTextUsagePricing(base, "/v1/chat/completions", 512001, 1)
	if low.InputPrice != 0.0003 || low.OutputPrice != 0.0012 || low.MatchedCondition != "prompt_tokens_lte=512000" {
		t.Fatalf("low tier pricing=%#v", low)
	}
	if high.InputPrice != 0.0006 || high.OutputPrice != 0.0024 || high.MatchedCondition != "prompt_tokens_gt=512000" {
		t.Fatalf("high tier pricing=%#v", high)
	}
}

func TestProviderCatalogCorrection20261026UpsertIsIdempotentAndDoesNotEnableChannels(t *testing.T) {
	db := newProviderMigrationTestDB(t)
	providers := []string{"minimax", "zhipu"}
	if err := upsertProviderMigrationProvidersWithDB(db, providers...); err != nil {
		t.Fatalf("first catalog correction: %v", err)
	}
	var modelCount, componentCount int64
	if err := db.Model(&ProviderModel{}).Where("provider IN ?", providers).Count(&modelCount).Error; err != nil {
		t.Fatalf("count models: %v", err)
	}
	if err := db.Model(&ProviderModelPriceComponent{}).Where("provider IN ?", providers).Count(&componentCount).Error; err != nil {
		t.Fatalf("count components: %v", err)
	}
	if err := upsertProviderMigrationProvidersWithDB(db, providers...); err != nil {
		t.Fatalf("second catalog correction: %v", err)
	}
	var secondModelCount, secondComponentCount int64
	db.Model(&ProviderModel{}).Where("provider IN ?", providers).Count(&secondModelCount)
	db.Model(&ProviderModelPriceComponent{}).Where("provider IN ?", providers).Count(&secondComponentCount)
	if secondModelCount != modelCount || secondComponentCount != componentCount {
		t.Fatalf("idempotent correction changed counts: models %d -> %d, components %d -> %d", modelCount, secondModelCount, componentCount, secondComponentCount)
	}
	if db.Migrator().HasTable(&ChannelModel{}) {
		t.Fatal("provider catalog correction must not create or enable channel models")
	}
}
