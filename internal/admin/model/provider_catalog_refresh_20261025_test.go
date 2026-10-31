package model

import (
	"slices"
	"testing"
)

func providerMigrationDetailsByProvider(t *testing.T) map[string]map[string]ProviderModelDetail {
	t.Helper()
	seeds := mustLoadProviderMigrationSeeds(t)
	result := make(map[string]map[string]ProviderModelDetail, len(seeds))
	for _, seed := range seeds {
		details := make(map[string]ProviderModelDetail, len(seed.ModelDetails))
		for _, detail := range seed.ModelDetails {
			details[detail.Model] = detail
		}
		result[seed.Provider] = details
	}
	return result
}

func TestMergeProviderMigrationSeedSetsMergesProvidersAndReplacesModels(t *testing.T) {
	baseline := []ProviderSeed{
		{
			Provider:    "openai",
			Name:        "OpenAI",
			BaseURL:     "https://api.openai.com",
			OfficialURL: "https://example.com/models",
			SortOrder:   100,
			ModelDetails: []ProviderModelDetail{
				{Model: "existing", Description: "old", InputPrice: 1},
			},
		},
		{Provider: "anthropic", Name: "Anthropic", SortOrder: 110},
	}
	delta := []ProviderSeed{
		{
			Provider:   " OpenAI ",
			PricingURL: "https://example.com/pricing",
			ModelDetails: []ProviderModelDetail{
				{Model: "existing", Description: "replacement"},
				{Model: "new-model", Description: "new"},
			},
		},
	}

	merged := mergeProviderMigrationSeedSets(baseline, delta)
	if len(merged) != 2 {
		t.Fatalf("merged provider count=%d, want 2", len(merged))
	}
	openAI := merged[0]
	if openAI.Provider != "openai" || openAI.Name != "OpenAI" || openAI.BaseURL != "https://api.openai.com" || openAI.SortOrder != 100 {
		t.Fatalf("baseline provider metadata was not preserved: %#v", openAI)
	}
	if openAI.PricingURL != "https://example.com/pricing" {
		t.Fatalf("pricing_url=%q, want delta value", openAI.PricingURL)
	}
	if len(openAI.ModelDetails) != 2 {
		t.Fatalf("merged model count=%d, want 2", len(openAI.ModelDetails))
	}
	if got := openAI.ModelDetails[0]; got.Description != "replacement" || got.InputPrice != 0 {
		t.Fatalf("repeated model must be fully replaced, got %#v", got)
	}
	if openAI.ModelDetails[1].Model != "new-model" {
		t.Fatalf("new delta model=%q, want new-model", openAI.ModelDetails[1].Model)
	}
}

func TestNormalizeProviderMigrationSeedModelTagsExplicitTypeWins(t *testing.T) {
	details := normalizeProviderMigrationSeedModelDetails("openai", []ProviderModelDetail{
		{
			Model:              "gpt-live-1",
			Tags:               []string{ProviderModelTagAudio, ProviderModelTagRealtime},
			SupportedEndpoints: []string{ChannelModelEndpointRealtime},
		},
	}, 1700000000)
	if len(details) != 1 {
		t.Fatalf("normalized detail count=%d, want 1", len(details))
	}
	if details[0].Type != ProviderModelTypeAudio {
		t.Fatalf("gpt-live-1 type=%q, want audio", details[0].Type)
	}
	if !slices.Contains(details[0].Tags, ProviderModelTagRealtime) {
		t.Fatalf("gpt-live-1 tags=%#v, want realtime", details[0].Tags)
	}
}

func TestProviderCatalogRefresh20261025IncludesVerifiedModelsAndPricing(t *testing.T) {
	details := providerMigrationDetailsByProvider(t)
	type expectedModel struct {
		modelType      string
		input          float64
		output         float64
		priceUnit      string
		componentCount int
	}
	expected := map[string]map[string]expectedModel{
		"openai": {
			"gpt-audio-1.5":          {modelType: ProviderModelTypeAudio, input: 0.032, output: 0.064, priceUnit: ProviderPriceUnitPer1KTokens},
			"gpt-live-1":             {modelType: ProviderModelTypeAudio, input: 0.05, priceUnit: ProviderPriceUnitPerMinute},
			"gpt-live-transcribe":    {modelType: ProviderModelTypeAudio, input: 0.017, priceUnit: ProviderPriceUnitPerMinute},
			"gpt-realtime-whisper":   {modelType: ProviderModelTypeAudio, input: 0.017, priceUnit: ProviderPriceUnitPerMinute},
			"gpt-4o-transcribe":      {modelType: ProviderModelTypeAudio, input: 0.006, priceUnit: ProviderPriceUnitPerMinute},
			"gpt-4o-mini-transcribe": {modelType: ProviderModelTypeAudio, input: 0.003, priceUnit: ProviderPriceUnitPerMinute},
		},
		"anthropic": {
			"claude-mythos-5-1":     {modelType: ProviderModelTypeText, input: 0.01, output: 0.05, priceUnit: ProviderPriceUnitPer1KTokens},
			"claude-fable-5":        {modelType: ProviderModelTypeText, input: 0.01, output: 0.05, priceUnit: ProviderPriceUnitPer1KTokens},
			"claude-mythos-5":       {modelType: ProviderModelTypeText, input: 0.01, output: 0.05, priceUnit: ProviderPriceUnitPer1KTokens},
			"claude-mythos-preview": {modelType: ProviderModelTypeText, priceUnit: ProviderPriceUnitPer1KTokens},
		},
		"moonshot": {
			"kimi-k3.1-pro":            {modelType: ProviderModelTypeText, input: 0.002, output: 0.01, priceUnit: ProviderPriceUnitPer1KTokens},
			"kimi-k3.1-pro-highspeed":  {modelType: ProviderModelTypeText, input: 0.004, output: 0.02, priceUnit: ProviderPriceUnitPer1KTokens},
			"kimi-k3":                  {modelType: ProviderModelTypeText, input: 0.003, output: 0.015, priceUnit: ProviderPriceUnitPer1KTokens},
			"kimi-k2.7-code-highspeed": {modelType: ProviderModelTypeText, input: 0.0019, output: 0.008, priceUnit: ProviderPriceUnitPer1KTokens},
		},
		"xai": {
			"grok-4.5":                    {modelType: ProviderModelTypeText, input: 0.002, output: 0.006, priceUnit: ProviderPriceUnitPer1KTokens},
			"grok-4.6":                    {modelType: ProviderModelTypeText, input: 0.002, output: 0.006, priceUnit: ProviderPriceUnitPer1KTokens},
			"grok-4.7":                    {modelType: ProviderModelTypeText, input: 0.002, output: 0.006, priceUnit: ProviderPriceUnitPer1KTokens},
			"grok-build-0.1":              {modelType: ProviderModelTypeText, input: 0.001, output: 0.002, priceUnit: ProviderPriceUnitPer1KTokens},
			"grok-imagine-image":          {modelType: ProviderModelTypeImage, input: 0.02, priceUnit: ProviderPriceUnitPerImage},
			"grok-imagine-image-quality":  {modelType: ProviderModelTypeImage, input: 0.05, priceUnit: ProviderPriceUnitPerImage},
			"grok-imagine-image-2.0":      {modelType: ProviderModelTypeImage, input: 0.06, priceUnit: ProviderPriceUnitPerImage},
			"grok-imagine-video":          {modelType: ProviderModelTypeVideo, input: 0.05, priceUnit: ProviderPriceUnitPerSecond, componentCount: 2},
			"grok-imagine-video-1.5":      {modelType: ProviderModelTypeVideo, input: 0.08, priceUnit: ProviderPriceUnitPerSecond, componentCount: 3},
			"grok-imagine-video-1.5-lite": {modelType: ProviderModelTypeVideo, input: 0.02, priceUnit: ProviderPriceUnitPerSecond, componentCount: 3},
			"grok-tts":                    {modelType: ProviderModelTypeAudio, input: 0.015, priceUnit: ProviderPriceUnitPer1KChars},
			"grok-voice-2.0":              {modelType: ProviderModelTypeAudio, input: 0.08, priceUnit: ProviderPriceUnitPerMinute},
			"grok-voice-3.0":              {modelType: ProviderModelTypeAudio, input: 0.08, priceUnit: ProviderPriceUnitPerMinute},
			"grok-voice-transcribe-2.0":   {modelType: ProviderModelTypeAudio, input: 0.0016666667, priceUnit: ProviderPriceUnitPerMinute},
		},
	}

	for provider, models := range expected {
		providerDetails, ok := details[provider]
		if !ok {
			t.Fatalf("provider %s missing from merged catalog", provider)
		}
		for modelName, want := range models {
			got, ok := providerDetails[modelName]
			if !ok {
				t.Fatalf("%s/%s missing from merged catalog", provider, modelName)
			}
			if got.Type != want.modelType || got.InputPrice != want.input || got.OutputPrice != want.output || got.PriceUnit != want.priceUnit {
				t.Fatalf("%s/%s type/pricing=%q/%v/%v/%q, want %q/%v/%v/%q", provider, modelName, got.Type, got.InputPrice, got.OutputPrice, got.PriceUnit, want.modelType, want.input, want.output, want.priceUnit)
			}
			if len(got.PriceComponents) != want.componentCount {
				t.Fatalf("%s/%s price component count=%d, want %d", provider, modelName, len(got.PriceComponents), want.componentCount)
			}
		}
	}
}

func TestProviderCatalogRefresh20261025LifecycleStatuses(t *testing.T) {
	details := providerMigrationDetailsByProvider(t)
	deprecated := map[string][]string{
		"openai": {
			"gpt-4o", "gpt-4o-mini", "gpt-4.1", "gpt-4.1-mini", "gpt-5.3-codex", "gpt-5.4-nano", "gpt-5.1",
			"tts-1", "tts-1-hd", "gpt-4o-mini-tts", "whisper-1", "gpt-realtime", "gpt-audio", "gpt-realtime-mini", "gpt-audio-mini",
			"gpt-5", "gpt-5-mini", "gpt-5-nano", "gpt-5-pro", "o3", "o3-pro", "gpt-4o-transcribe", "gpt-4o-mini-transcribe",
		},
		"anthropic": {"claude-mythos-preview", "claude-sonnet-4-5-20250929", "claude-opus-4-1-20250805"},
		"moonshot":  {"kimi-k2.5"},
	}
	for provider, models := range deprecated {
		for _, modelName := range models {
			got, ok := details[provider][modelName]
			if !ok {
				t.Fatalf("deprecated model %s/%s missing", provider, modelName)
			}
			if got.Status != ProviderModelStatusDeprecated {
				t.Fatalf("%s/%s status=%q, want deprecated", provider, modelName, got.Status)
			}
		}
	}

	deleted := map[string][]string{
		"openai": {
			"codex-mini-latest", "dall-e-3", "gpt-5-codex", "gpt-5.1-codex", "gpt-5.1-codex-max", "gpt-5.1-codex-mini",
			"gpt-5.2-codex", "gpt-4.1-nano", "gpt-4o-2024-05-13", "gpt-image-1", "o1-pro", "o4-mini",
		},
		"anthropic": {"claude-opus-4-1-20250805"},
		"moonshot":  {"kimi-k2.5"},
	}
	for provider, models := range deleted {
		for _, modelName := range models {
			got, ok := details[provider][modelName]
			if !ok {
				t.Fatalf("shutdown model %s/%s missing", provider, modelName)
			}
			if !got.IsDeleted {
				t.Fatalf("%s/%s is_deleted=false, want true", provider, modelName)
			}
		}
	}

	for _, modelName := range []string{"gpt-4o", "gpt-4o-mini", "gpt-4.1", "gpt-4.1-mini", "gpt-image-1-mini", "gpt-image-1.5"} {
		if details["openai"][modelName].IsDeleted {
			t.Fatalf("openai/%s has a future shutdown date and must not be deleted early", modelName)
		}
	}
	if details["anthropic"]["claude-sonnet-4-5-20250929"].IsDeleted {
		t.Fatal("claude-sonnet-4-5-20250929 must remain available until its announced retirement date")
	}
	if details["openai"]["gpt-live-transcribe"].Status != ProviderModelStatusActive {
		t.Fatal("gpt-live-transcribe is the recommended transcription replacement and must remain active")
	}
}

func TestProviderCatalogRefresh20261025UpsertIsIdempotentAndDoesNotEnableChannels(t *testing.T) {
	db := newProviderMigrationTestDB(t)
	providers := []string{"openai", "anthropic", "xai", "moonshot"}
	if err := upsertProviderMigrationProvidersWithDB(db, providers...); err != nil {
		t.Fatalf("first catalog refresh: %v", err)
	}
	modelCount := int64(0)
	if err := db.Model(&ProviderModel{}).Where("provider IN ?", providers).Count(&modelCount).Error; err != nil {
		t.Fatalf("count models after first refresh: %v", err)
	}
	componentCount := int64(0)
	if err := db.Model(&ProviderModelPriceComponent{}).Where("provider IN ?", providers).Count(&componentCount).Error; err != nil {
		t.Fatalf("count components after first refresh: %v", err)
	}

	if err := upsertProviderMigrationProvidersWithDB(db, providers...); err != nil {
		t.Fatalf("second catalog refresh: %v", err)
	}
	secondModelCount := int64(0)
	if err := db.Model(&ProviderModel{}).Where("provider IN ?", providers).Count(&secondModelCount).Error; err != nil {
		t.Fatalf("count models after second refresh: %v", err)
	}
	secondComponentCount := int64(0)
	if err := db.Model(&ProviderModelPriceComponent{}).Where("provider IN ?", providers).Count(&secondComponentCount).Error; err != nil {
		t.Fatalf("count components after second refresh: %v", err)
	}
	if secondModelCount != modelCount || secondComponentCount != componentCount {
		t.Fatalf("idempotent refresh changed counts: models %d -> %d, components %d -> %d", modelCount, secondModelCount, componentCount, secondComponentCount)
	}
	if db.Migrator().HasTable(&ChannelModel{}) {
		t.Fatal("provider catalog refresh must not create or enable channel models")
	}
}
