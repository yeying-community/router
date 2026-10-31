package model

import "testing"

func TestProviderCatalogRefresh20261029QwenRealtimePricing(t *testing.T) {
	details := providerMigrationDetailsByProvider(t)["qwen"]
	wants := map[string]struct {
		input, output float64
		unit          string
	}{
		"qwen3.5-omni-plus-realtime":           {0.01, 0.08, ProviderPriceUnitPer1KTokens},
		"qwen3.5-omni-flash-realtime":          {0.0033, 0.027, ProviderPriceUnitPer1KTokens},
		"qwen3.8-omni-flash-realtime":          {0.0015, 0.0045, ProviderPriceUnitPer1KTokens},
		"qwen3.5-livetranslate-flash-realtime": {0.04, 0.1, ProviderPriceUnitPer1KTokens},
		"qwen3.8-livetranslate-flash-realtime": {0.04, 0.1, ProviderPriceUnitPer1KTokens},
		"qwen-tts-realtime":                    {0.0024, 0.012, ProviderPriceUnitPer1KTokens},
		"qwen3-tts-instruct-flash-realtime":    {0.1, 0, ProviderPriceUnitPer1KChars},
		"qwen3-tts-flash-realtime":             {0.1, 0, ProviderPriceUnitPer1KChars},
	}
	for modelName, want := range wants {
		got, ok := details[modelName]
		if !ok {
			t.Fatalf("Qwen model %s missing from merged catalog", modelName)
		}
		if got.Status != ProviderModelStatusActive || got.InputPrice != want.input || got.OutputPrice != want.output || got.PriceUnit != want.unit || got.Currency != "CNY" {
			t.Fatalf("Qwen model %s status/pricing=%q/%v/%v/%q/%q", modelName, got.Status, got.InputPrice, got.OutputPrice, got.PriceUnit, got.Currency)
		}
	}

	componentWants := map[string]map[string][2]float64{
		"qwen3.5-omni-plus-realtime": {
			ProviderModelPriceComponentRealtimeText:  {0.01, 0.08},
			ProviderModelPriceComponentRealtimeAudio: {0.06, 0.3},
		},
		"qwen3.8-omni-flash-realtime": {
			ProviderModelPriceComponentRealtimeText:  {0.0015, 0.0045},
			ProviderModelPriceComponentRealtimeAudio: {0.006, 0.012},
		},
	}
	for modelName, components := range componentWants {
		got := details[modelName]
		for _, component := range got.PriceComponents {
			want, ok := components[component.Component]
			if !ok {
				continue
			}
			if component.InputPrice != want[0] || component.OutputPrice != want[1] || component.PriceUnit != ProviderPriceUnitPer1KTokens || component.Currency != "CNY" {
				t.Fatalf("Qwen model %s component %s=%#v, want %v", modelName, component.Component, component, want)
			}
			delete(components, component.Component)
		}
		if len(components) != 0 {
			t.Fatalf("Qwen model %s missing realtime components: %#v", modelName, components)
		}
	}
}

func TestProviderCatalogRefresh20261029OfficialPricesAndLifecycle(t *testing.T) {
	details := providerMigrationDetailsByProvider(t)
	tests := []struct {
		provider, model, status string
		deleted                 bool
		input, output           float64
		unit                    string
	}{
		{"xai", "grok-tts", ProviderModelStatusActive, false, 0.015, 0, ProviderPriceUnitPer1KChars},
		{"xai", "grok-voice-2.0", ProviderModelStatusDeprecated, false, 0.08, 0, ProviderPriceUnitPerMinute},
		{"xai", "grok-voice-3.0", ProviderModelStatusActive, false, 0.08, 0, ProviderPriceUnitPerMinute},
		{"xai", "grok-voice-transcribe-2.0", ProviderModelStatusActive, false, 0.0016666667, 0, ProviderPriceUnitPerMinute},
		{"deepgram", "nova-2", ProviderModelStatusDeprecated, false, 0.0058333333, 0, ProviderPriceUnitPerMinute},
		{"deepgram", "nova-3", ProviderModelStatusActive, false, 0.0077, 0, ProviderPriceUnitPerMinute},
		{"minimax", "MiniMax-H3", ProviderModelStatusActive, false, 0.08, 0.08, ProviderPriceUnitPerSecond},
		{"minimax", "MiniMax-H3-Max", ProviderModelStatusActive, false, 0.143, 0.08, ProviderPriceUnitPerSecond},
		{"minimax", "MiniMax-M2.1", ProviderModelStatusDeprecated, true, 0.0003, 0.0012, ProviderPriceUnitPer1KTokens},
	}
	for _, tt := range tests {
		got, ok := details[tt.provider][tt.model]
		if !ok {
			t.Fatalf("%s/%s missing from merged catalog", tt.provider, tt.model)
		}
		if got.Status != tt.status || got.IsDeleted != tt.deleted || got.InputPrice != tt.input || got.OutputPrice != tt.output || got.PriceUnit != tt.unit {
			t.Fatalf("%s/%s status/deleted/pricing=%q/%t/%v/%v/%q, want %q/%t/%v/%v/%q", tt.provider, tt.model, got.Status, got.IsDeleted, got.InputPrice, got.OutputPrice, got.PriceUnit, tt.status, tt.deleted, tt.input, tt.output, tt.unit)
		}
	}
}

func TestProviderCatalogRefresh20261029MiniMaxH3ResolutionPricing(t *testing.T) {
	detail := providerMigrationDetailsByProvider(t)["minimax"]["MiniMax-H3-Lite"]
	base := ResolvedModelPricing{
		Model: detail.Model, Provider: "minimax", Type: detail.Type,
		InputPrice: detail.InputPrice, OutputPrice: detail.OutputPrice,
		PriceUnit: detail.PriceUnit, Currency: detail.Currency,
		PriceComponents: detail.PriceComponents,
	}
	low := ResolveVideoRequestPricing(base, map[string]string{"resolution": "480p"})
	high := ResolveVideoRequestPricing(base, map[string]string{"resolution": "768p"})
	if low.InputPrice != 0.025 || low.OutputPrice != 0.025 || low.PriceUnit != ProviderPriceUnitPerSecond {
		t.Fatalf("MiniMax-H3-Lite 480P pricing=%#v", low)
	}
	if high.InputPrice != 0.04 || high.OutputPrice != 0.04 || high.PriceUnit != ProviderPriceUnitPerSecond {
		t.Fatalf("MiniMax-H3-Lite 768P pricing=%#v", high)
	}
}

func TestProviderCatalogRefresh20261029UpsertIsIdempotent(t *testing.T) {
	db := newProviderMigrationTestDB(t)
	providers := []string{"qwen", "minimax", "xai", "deepgram"}
	if err := upsertProviderMigrationProvidersWithDB(db, providers...); err != nil {
		t.Fatalf("first provider catalog refresh: %v", err)
	}
	var modelsBefore, componentsBefore int64
	if err := db.Model(&ProviderModel{}).Where("provider IN ?", providers).Count(&modelsBefore).Error; err != nil {
		t.Fatalf("count provider models: %v", err)
	}
	if err := db.Model(&ProviderModelPriceComponent{}).Where("provider IN ?", providers).Count(&componentsBefore).Error; err != nil {
		t.Fatalf("count provider price components: %v", err)
	}
	if err := upsertProviderMigrationProvidersWithDB(db, providers...); err != nil {
		t.Fatalf("second provider catalog refresh: %v", err)
	}
	var modelsAfter, componentsAfter int64
	db.Model(&ProviderModel{}).Where("provider IN ?", providers).Count(&modelsAfter)
	db.Model(&ProviderModelPriceComponent{}).Where("provider IN ?", providers).Count(&componentsAfter)
	if modelsAfter != modelsBefore || componentsAfter != componentsBefore {
		t.Fatalf("provider refresh changed counts: models %d -> %d, components %d -> %d", modelsBefore, modelsAfter, componentsBefore, componentsAfter)
	}
	if db.Migrator().HasTable(&ChannelModel{}) {
		t.Fatal("provider refresh must not create or enable channel models")
	}
}
