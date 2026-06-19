package model

import (
	"slices"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func mustLoadProviderMigrationSeeds(t *testing.T) []ProviderSeed {
	t.Helper()
	seeds, err := LoadProviderMigrationSeeds(1700000000)
	if err != nil {
		t.Fatalf("LoadProviderMigrationSeeds failed: %v", err)
	}
	return seeds
}

func newProviderMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func TestBuildProviderMigrationSeeds_OpenAIIncludesDALLE3(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	for _, seed := range seeds {
		if seed.Provider != "openai" {
			continue
		}
		for _, detail := range seed.ModelDetails {
			if detail.Model != "dall-e-3" {
				continue
			}
			if detail.Type != ProviderModelTypeImage {
				t.Fatalf("dall-e-3 type=%q, want %q", detail.Type, ProviderModelTypeImage)
			}
			if detail.InputPrice != 0.04 {
				t.Fatalf("dall-e-3 input_price=%v, want 0.04", detail.InputPrice)
			}
			if detail.PriceUnit != ProviderPriceUnitPerImage {
				t.Fatalf("dall-e-3 price_unit=%q, want %q", detail.PriceUnit, ProviderPriceUnitPerImage)
			}
			if detail.Currency != ProviderPriceCurrencyUSD {
				t.Fatalf("dall-e-3 currency=%q, want %q", detail.Currency, ProviderPriceCurrencyUSD)
			}
			if strings.TrimSpace(detail.Description) == "" {
				t.Fatalf("dall-e-3 description should not be empty")
			}
			if len(detail.PriceComponents) != 6 {
				t.Fatalf("dall-e-3 price_components=%d, want 6", len(detail.PriceComponents))
			}
			return
		}
		t.Fatalf("expected openai seed to include dall-e-3")
	}
	t.Fatalf("expected openai provider to exist")
}

func TestUpsertProviderMigrationProvidersPrunesStaleMigrationModelsOnlyForTargetProvider(t *testing.T) {
	db := newProviderMigrationTestDB(t)
	if err := upsertProviderMigrationProvidersWithDB(db, "openai", "anthropic"); err != nil {
		t.Fatalf("seed provider catalogs: %v", err)
	}
	staleModel := ProviderModel{
		Provider:  "openai",
		Model:     "stale-model",
		Tags:      ProviderModelTypeText,
		Status:    ProviderModelStatusActive,
		Source:    "migration",
		UpdatedAt: 1,
	}
	if err := db.Create(&staleModel).Error; err != nil {
		t.Fatalf("create stale model: %v", err)
	}
	if err := db.Create(&ProviderModelPriceComponent{
		Provider:  "openai",
		Model:     "stale-model",
		Component: ProviderModelPriceComponentText,
		Source:    "migration",
		UpdatedAt: 1,
	}).Error; err != nil {
		t.Fatalf("create stale component: %v", err)
	}
	if err := upsertProviderMigrationProvidersWithDB(db, "openai"); err != nil {
		t.Fatalf("refresh openai provider: %v", err)
	}
	count := int64(0)
	if err := db.Model(&ProviderModel{}).
		Where("provider = ? AND model = ?", "openai", "stale-model").
		Count(&count).Error; err != nil {
		t.Fatalf("count stale openai model: %v", err)
	}
	if count != 0 {
		t.Fatalf("stale openai model count=%d, want 0", count)
	}
	if err := db.Model(&ProviderModel{}).
		Where("provider = ?", "anthropic").
		Count(&count).Error; err != nil {
		t.Fatalf("count anthropic models: %v", err)
	}
	if count == 0 {
		t.Fatal("expected anthropic models to remain after openai-only refresh")
	}
}

func TestNormalizeProviderMigrationLegacySourcesWithDB(t *testing.T) {
	db := newProviderMigrationTestDB(t)
	if err := db.AutoMigrate(&Provider{}, &ProviderModel{}, &ProviderModelPriceComponent{}); err != nil {
		t.Fatalf("auto migrate provider tables: %v", err)
	}
	if err := db.Create(&Provider{
		Id:     "openai",
		Source: "default",
	}).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if err := db.Create(&ProviderModel{
		Provider: "openai",
		Model:    "gpt-4.1",
		Source:   "default",
	}).Error; err != nil {
		t.Fatalf("create provider model: %v", err)
	}
	if err := db.Create(&ProviderModelPriceComponent{
		Provider:  "openai",
		Model:     "gpt-4.1",
		Component: ProviderModelPriceComponentText,
		Source:    "default",
	}).Error; err != nil {
		t.Fatalf("create provider model price component: %v", err)
	}
	if err := db.Create(&Provider{
		Id:     "manual-provider",
		Source: "manual",
	}).Error; err != nil {
		t.Fatalf("create manual provider: %v", err)
	}
	if err := normalizeProviderMigrationLegacySourcesWithDB(db); err != nil {
		t.Fatalf("normalize provider migration legacy sources: %v", err)
	}
	provider := Provider{}
	if err := db.First(&provider, "id = ?", "openai").Error; err != nil {
		t.Fatalf("query provider: %v", err)
	}
	if provider.Source != "migration" {
		t.Fatalf("provider source=%q, want migration", provider.Source)
	}
	providerModel := ProviderModel{}
	if err := db.First(&providerModel, "provider = ? AND model = ?", "openai", "gpt-4.1").Error; err != nil {
		t.Fatalf("query provider model: %v", err)
	}
	if providerModel.Source != "migration" {
		t.Fatalf("provider model source=%q, want migration", providerModel.Source)
	}
	component := ProviderModelPriceComponent{}
	if err := db.First(&component, "provider = ? AND model = ? AND component = ?", "openai", "gpt-4.1", ProviderModelPriceComponentText).Error; err != nil {
		t.Fatalf("query provider model price component: %v", err)
	}
	if component.Source != "migration" {
		t.Fatalf("provider model price component source=%q, want migration", component.Source)
	}
	manualProvider := Provider{}
	if err := db.First(&manualProvider, "id = ?", "manual-provider").Error; err != nil {
		t.Fatalf("query manual provider: %v", err)
	}
	if manualProvider.Source != "manual" {
		t.Fatalf("manual provider source=%q, want manual", manualProvider.Source)
	}
}

func TestNormalizeProviderPricingLegacySourcesWithDB(t *testing.T) {
	db := newProviderMigrationTestDB(t)
	if err := db.AutoMigrate(&Log{}); err != nil {
		t.Fatalf("auto migrate logs: %v", err)
	}
	if err := db.Create(&Log{
		Id:                   "log-1",
		BillingPricingSource: "provider_default",
	}).Error; err != nil {
		t.Fatalf("create provider default log: %v", err)
	}
	if err := db.Create(&Log{
		Id:                   "log-2",
		BillingPricingSource: "channel_override",
	}).Error; err != nil {
		t.Fatalf("create channel override log: %v", err)
	}
	if err := normalizeProviderPricingLegacySourcesWithDB(db); err != nil {
		t.Fatalf("normalize provider pricing legacy sources: %v", err)
	}
	defaultLog := Log{}
	if err := db.First(&defaultLog, "id = ?", "log-1").Error; err != nil {
		t.Fatalf("query provider default log: %v", err)
	}
	if defaultLog.BillingPricingSource != "provider_migration" {
		t.Fatalf("billing pricing source=%q, want provider_migration", defaultLog.BillingPricingSource)
	}
	channelOverrideLog := Log{}
	if err := db.First(&channelOverrideLog, "id = ?", "log-2").Error; err != nil {
		t.Fatalf("query channel override log: %v", err)
	}
	if channelOverrideLog.BillingPricingSource != "channel_override" {
		t.Fatalf("channel override source=%q, want channel_override", channelOverrideLog.BillingPricingSource)
	}
}

func TestLoadProviderMigrationSeedsFromSnapshot(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	if len(seeds) == 0 {
		t.Fatal("expected non-empty provider migration seeds from snapshot")
	}
	if seeds[0].Provider == "" {
		t.Fatal("expected first provider seed to include provider id")
	}
}

func TestBuildProviderMigrationSeeds_OpenAIIncludesGPTImage1ComplexPricing(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	for _, seed := range seeds {
		if seed.Provider != "openai" {
			continue
		}
		for _, detail := range seed.ModelDetails {
			if detail.Model != "gpt-image-1" {
				continue
			}
			if detail.Type != ProviderModelTypeImage {
				t.Fatalf("gpt-image-1 type=%q, want %q", detail.Type, ProviderModelTypeImage)
			}
			if detail.InputPrice != 0.011 {
				t.Fatalf("gpt-image-1 input_price=%v, want 0.011", detail.InputPrice)
			}
			if detail.PriceUnit != ProviderPriceUnitPerImage {
				t.Fatalf("gpt-image-1 price_unit=%q, want %q", detail.PriceUnit, ProviderPriceUnitPerImage)
			}
			if detail.Currency != ProviderPriceCurrencyUSD {
				t.Fatalf("gpt-image-1 currency=%q, want %q", detail.Currency, ProviderPriceCurrencyUSD)
			}
			if len(detail.SupportedEndpoints) != 3 {
				t.Fatalf("gpt-image-1 supported_endpoints=%#v, want 3 endpoints", detail.SupportedEndpoints)
			}
			if len(detail.PriceComponents) != 9 {
				t.Fatalf("gpt-image-1 price_components=%d, want 9", len(detail.PriceComponents))
			}
			return
		}
		t.Fatalf("expected openai seed to include gpt-image-1")
	}
	t.Fatalf("expected openai provider to exist")
}

func TestBuildProviderMigrationSeeds_OpenAIIncludesGPTImage2Pricing(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	for _, seed := range seeds {
		if seed.Provider != "openai" {
			continue
		}
		for _, detail := range seed.ModelDetails {
			if detail.Model != "gpt-image-2" {
				continue
			}
			if detail.Type != ProviderModelTypeImage {
				t.Fatalf("gpt-image-2 type=%q, want %q", detail.Type, ProviderModelTypeImage)
			}
			if detail.InputPrice != 0.008 {
				t.Fatalf("gpt-image-2 input_price=%v, want 0.008", detail.InputPrice)
			}
			if detail.OutputPrice != 0.03 {
				t.Fatalf("gpt-image-2 output_price=%v, want 0.03", detail.OutputPrice)
			}
			if detail.PriceUnit != ProviderPriceUnitPer1KTokens {
				t.Fatalf("gpt-image-2 price_unit=%q, want %q", detail.PriceUnit, ProviderPriceUnitPer1KTokens)
			}
			if len(detail.SupportedEndpoints) != 3 {
				t.Fatalf("gpt-image-2 supported_endpoints=%#v, want 3 endpoints", detail.SupportedEndpoints)
			}
			for _, endpoint := range []string{ChannelModelEndpointResponses, ChannelModelEndpointImages, ChannelModelEndpointImageEdit} {
				if !slices.Contains(detail.SupportedEndpoints, endpoint) {
					t.Fatalf("gpt-image-2 supported_endpoints=%#v, missing %s", detail.SupportedEndpoints, endpoint)
				}
			}
			if len(detail.PriceComponents) != 2 {
				t.Fatalf("gpt-image-2 price_components=%d, want 2", len(detail.PriceComponents))
			}
			return
		}
		t.Fatalf("expected openai seed to include gpt-image-2")
	}
	t.Fatalf("expected openai provider to exist")
}

func TestBuildProviderMigrationSeeds_TokenBasedImageModelsUseResponsesEndpoint(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	expected := map[string]string{
		"gpt-image-2":              "openai",
		"ernie-4.5-vl-32k-preview": "baidu",
		"step-1o-turbo-vision":     "stepfun",
		"glm-4v-plus-0111":         "zhipu",
		"pixtral-large-latest":     "mistral",
	}

	found := make(map[string]bool, len(expected))
	for _, seed := range seeds {
		for _, detail := range seed.ModelDetails {
			provider, ok := expected[detail.Model]
			if !ok || provider != seed.Provider {
				continue
			}
			if detail.Type != ProviderModelTypeImage {
				t.Fatalf("%s type=%q, want %q", detail.Model, detail.Type, ProviderModelTypeImage)
			}
			if detail.PriceUnit != ProviderPriceUnitPer1KTokens {
				t.Fatalf("%s price_unit=%q, want %q", detail.Model, detail.PriceUnit, ProviderPriceUnitPer1KTokens)
			}
			if detail.Model == "gpt-image-2" {
				if len(detail.SupportedEndpoints) != 3 {
					t.Fatalf("%s supported_endpoints=%#v, want 3 endpoints", detail.Model, detail.SupportedEndpoints)
				}
				for _, endpoint := range []string{ChannelModelEndpointResponses, ChannelModelEndpointImages, ChannelModelEndpointImageEdit} {
					if !slices.Contains(detail.SupportedEndpoints, endpoint) {
						t.Fatalf("%s supported_endpoints=%#v, missing %s", detail.Model, detail.SupportedEndpoints, endpoint)
					}
				}
			} else if seed.Provider == "zhipu" {
				if len(detail.SupportedEndpoints) != 1 || detail.SupportedEndpoints[0] != ChannelModelEndpointChat {
					t.Fatalf("%s supported_endpoints=%#v, want [%s]", detail.Model, detail.SupportedEndpoints, ChannelModelEndpointChat)
				}
			} else if len(detail.SupportedEndpoints) != 1 || detail.SupportedEndpoints[0] != ChannelModelEndpointResponses {
				t.Fatalf("%s supported_endpoints=%#v, want [%s]", detail.Model, detail.SupportedEndpoints, ChannelModelEndpointResponses)
			}
			found[detail.Model] = true
		}
	}
	for modelName := range expected {
		if !found[modelName] {
			t.Fatalf("expected provider migration seeds to include %s", modelName)
		}
	}
}

func TestBuildProviderMigrationSeeds_QwenUsesExplicitEndpointTruthTable(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	for _, seed := range seeds {
		if seed.Provider != "qwen" {
			continue
		}
		found := map[string]bool{}
		for _, detail := range seed.ModelDetails {
			switch detail.Model {
			case "qwen3.7-max", "qwen3.6-plus", "qwen3.6-flash", "qwen3.5-plus", "qwen3.5-flash", "qwen3-max":
				if len(detail.SupportedEndpoints) != 2 || detail.SupportedEndpoints[0] != ChannelModelEndpointChat || detail.SupportedEndpoints[1] != ChannelModelEndpointResponses {
					t.Fatalf("%s supported_endpoints=%#v, want [chat responses]", detail.Model, detail.SupportedEndpoints)
				}
				found[detail.Model] = true
			case "qwen-image-2.0", "qwen-image-2.0-pro":
				if len(detail.SupportedEndpoints) != 2 || detail.SupportedEndpoints[0] != ChannelModelEndpointImages || detail.SupportedEndpoints[1] != ChannelModelEndpointImageEdit {
					t.Fatalf("%s supported_endpoints=%#v, want [images edits]", detail.Model, detail.SupportedEndpoints)
				}
				found[detail.Model] = true
			}
		}
		for _, modelName := range []string{
			"qwen3.7-max",
			"qwen3.6-plus",
			"qwen3.6-flash",
			"qwen3.5-plus",
			"qwen3.5-flash",
			"qwen3-max",
			"qwen-image-2.0",
			"qwen-image-2.0-pro",
		} {
			if !found[modelName] {
				t.Fatalf("expected qwen seed to include %s", modelName)
			}
		}
		return
	}
	t.Fatalf("expected qwen provider to exist")
}

func TestBuildProviderMigrationSeeds_QwenUsesConcreteModelVersions(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	for _, seed := range seeds {
		if seed.Provider != "qwen" {
			continue
		}
		for _, detail := range seed.ModelDetails {
			if strings.HasSuffix(detail.Model, "-latest") {
				t.Fatalf("qwen provider model %s uses floating latest alias", detail.Model)
			}
		}
		return
	}
	t.Fatalf("expected qwen provider to exist")
}

func TestBuildProviderMigrationSeeds_OpenAIIncludesGPT5xPricing(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	expected := map[string]struct {
		input  float64
		output float64
	}{
		"gpt-5.4-mini":       {input: 0.00075, output: 0.0045},
		"gpt-5.1":            {input: 0.00125, output: 0.01},
		"gpt-5.1-codex":      {input: 0.00125, output: 0.01},
		"gpt-5.1-codex-max":  {input: 0.00125, output: 0.01},
		"gpt-5.1-codex-mini": {input: 0.00025, output: 0.002},
	}

	for _, seed := range seeds {
		if seed.Provider != "openai" {
			continue
		}
		found := make(map[string]bool, len(expected))
		for _, detail := range seed.ModelDetails {
			want, ok := expected[detail.Model]
			if !ok {
				continue
			}
			if detail.Type != ProviderModelTypeText {
				t.Fatalf("%s type=%q, want %q", detail.Model, detail.Type, ProviderModelTypeText)
			}
			if detail.InputPrice != want.input {
				t.Fatalf("%s input_price=%v, want %v", detail.Model, detail.InputPrice, want.input)
			}
			if detail.OutputPrice != want.output {
				t.Fatalf("%s output_price=%v, want %v", detail.Model, detail.OutputPrice, want.output)
			}
			if detail.PriceUnit != ProviderPriceUnitPer1KTokens {
				t.Fatalf("%s price_unit=%q, want %q", detail.Model, detail.PriceUnit, ProviderPriceUnitPer1KTokens)
			}
			if detail.Currency != ProviderPriceCurrencyUSD {
				t.Fatalf("%s currency=%q, want %q", detail.Model, detail.Currency, ProviderPriceCurrencyUSD)
			}
			found[detail.Model] = true
		}
		for modelName := range expected {
			if !found[modelName] {
				t.Fatalf("expected openai seed to include %s", modelName)
			}
		}
		return
	}
	t.Fatalf("expected openai provider to exist")
}

func TestBuildProviderMigrationSeeds_OpenAIIncludesGPT55Pricing(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	for _, seed := range seeds {
		if seed.Provider != "openai" {
			continue
		}
		for _, detail := range seed.ModelDetails {
			if detail.Model != "gpt-5.5" {
				continue
			}
			if detail.Type != ProviderModelTypeText {
				t.Fatalf("gpt-5.5 type=%q, want %q", detail.Type, ProviderModelTypeText)
			}
			if detail.InputPrice != 0.005 {
				t.Fatalf("gpt-5.5 input_price=%v, want 0.005", detail.InputPrice)
			}
			if detail.OutputPrice != 0.03 {
				t.Fatalf("gpt-5.5 output_price=%v, want 0.03", detail.OutputPrice)
			}
			if detail.PriceUnit != ProviderPriceUnitPer1KTokens {
				t.Fatalf("gpt-5.5 price_unit=%q, want %q", detail.PriceUnit, ProviderPriceUnitPer1KTokens)
			}
			if detail.Currency != ProviderPriceCurrencyUSD {
				t.Fatalf("gpt-5.5 currency=%q, want %q", detail.Currency, ProviderPriceCurrencyUSD)
			}
			return
		}
		t.Fatalf("expected openai seed to include gpt-5.5")
	}
	t.Fatalf("expected openai provider to exist")
}

func TestBuildProviderMigrationSeeds_OpenAIIncludesNewOfficialModels(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	expected := map[string]struct {
		modelType string
	}{
		"gpt-5.4-nano":           {modelType: ProviderModelTypeText},
		"gpt-5.4-pro":            {modelType: ProviderModelTypeText},
		"gpt-5.5-pro":            {modelType: ProviderModelTypeText},
		"gpt-image-1.5":          {modelType: ProviderModelTypeImage},
		"gpt-image-1-mini":       {modelType: ProviderModelTypeImage},
		"gpt-realtime-translate": {modelType: ProviderModelTypeAudio},
		"gpt-4o-mini-tts":        {modelType: ProviderModelTypeAudio},
		"sora-2-pro":             {modelType: ProviderModelTypeVideo},
	}

	for _, seed := range seeds {
		if seed.Provider != "openai" {
			continue
		}
		found := make(map[string]bool, len(expected))
		for _, detail := range seed.ModelDetails {
			want, ok := expected[detail.Model]
			if !ok {
				continue
			}
			if detail.Type != want.modelType {
				t.Fatalf("%s type=%q, want %q", detail.Model, detail.Type, want.modelType)
			}
			if strings.TrimSpace(detail.Description) == "" {
				t.Fatalf("%s description should not be empty", detail.Model)
			}
			found[detail.Model] = true
		}
		for modelName := range expected {
			if !found[modelName] {
				t.Fatalf("expected openai seed to include %s", modelName)
			}
		}
		return
	}
	t.Fatalf("expected openai provider to exist")
}

func TestBuildProviderMigrationSeeds_XAIIncludesNewOfficialModels(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	expected := map[string]bool{
		"grok-4.20": false,
		"grok-4.3":  false,
	}

	for _, seed := range seeds {
		if seed.Provider != "xai" {
			continue
		}
		for _, detail := range seed.ModelDetails {
			if _, ok := expected[detail.Model]; !ok {
				continue
			}
			if strings.TrimSpace(detail.Description) == "" {
				t.Fatalf("%s description should not be empty", detail.Model)
			}
			expected[detail.Model] = true
		}
		for modelName, found := range expected {
			if !found {
				t.Fatalf("expected xai seed to include %s", modelName)
			}
		}
		return
	}
	t.Fatalf("expected xai provider to exist")
}

func TestBuildProviderMigrationSeeds_UnknownOrLegacyDescriptionsStayEmpty(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	checks := map[string]map[string]bool{
		"anthropic": {
			"claude-3-5-haiku-20241022": false,
		},
		"google": {
			"gemini-live-2.5-flash-preview": false,
		},
		"xai": {
			"grok-2-image-1212": false,
		},
	}

	for _, seed := range seeds {
		providerChecks, ok := checks[seed.Provider]
		if !ok {
			continue
		}
		for _, detail := range seed.ModelDetails {
			if _, exists := providerChecks[detail.Model]; !exists {
				continue
			}
			if strings.TrimSpace(detail.Description) != "" {
				t.Fatalf("%s/%s description=%q, want empty", seed.Provider, detail.Model, detail.Description)
			}
			if detail.Status != ProviderModelStatusDeprecated {
				t.Fatalf("%s/%s status=%q, want deprecated", seed.Provider, detail.Model, detail.Status)
			}
			if !detail.IsDeleted {
				t.Fatalf("%s/%s is_deleted=false, want true", seed.Provider, detail.Model)
			}
			providerChecks[detail.Model] = true
		}
	}

	for provider, models := range checks {
		for modelName, found := range models {
			if !found {
				t.Fatalf("expected %s seed to include %s", provider, modelName)
			}
		}
	}
}

func TestBuildProviderMigrationSeeds_DeprecatedStatusApplied(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	checks := map[string]map[string]bool{
		"openai": {
			"codex-mini-latest": false,
		},
		"anthropic": {
			"claude-3-5-haiku-20241022": false,
		},
		"google": {
			"gemini-live-2.5-flash-preview": false,
		},
		"xai": {
			"grok-2-image-1212": false,
		},
	}
	for _, seed := range seeds {
		providerChecks, ok := checks[seed.Provider]
		if !ok {
			continue
		}
		for _, detail := range seed.ModelDetails {
			if _, exists := providerChecks[detail.Model]; !exists {
				continue
			}
			if detail.Status != ProviderModelStatusDeprecated {
				t.Fatalf("%s/%s status=%q, want deprecated", seed.Provider, detail.Model, detail.Status)
			}
			providerChecks[detail.Model] = true
		}
	}
	for provider, models := range checks {
		for modelName, found := range models {
			if !found {
				t.Fatalf("expected %s seed to include %s", provider, modelName)
			}
		}
	}
}

func TestBuildProviderMigrationSeeds_AllDescriptionsReviewed(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	allowedEmpty := map[string]map[string]struct{}{
		"anthropic": {
			"claude-3-5-haiku-20241022": {},
		},
		"google": {
			"gemini-live-2.5-flash-preview": {},
		},
		"xai": {
			"grok-2-image-1212": {},
		},
	}

	for _, seed := range seeds {
		for _, detail := range seed.ModelDetails {
			description := strings.TrimSpace(detail.Description)
			if description != "" {
				continue
			}
			if !detail.IsDeleted {
				t.Fatalf("empty description without is_deleted for %s/%s", seed.Provider, detail.Model)
			}
			if providerAllowed, ok := allowedEmpty[seed.Provider]; ok {
				if _, exists := providerAllowed[detail.Model]; exists {
					continue
				}
			}
			t.Fatalf("unexpected empty description for %s/%s", seed.Provider, detail.Model)
		}
	}
}

func TestBuildProviderMigrationSeeds_OpenAIIncludesRealtime15And2Pricing(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	expected := map[string]struct {
		input  float64
		output float64
	}{
		"gpt-realtime-2":   {input: 0.004, output: 0.024},
		"gpt-realtime-1.5": {input: 0.0006, output: 0.0024},
	}

	for _, seed := range seeds {
		if seed.Provider != "openai" {
			continue
		}
		found := make(map[string]bool, len(expected))
		for _, detail := range seed.ModelDetails {
			want, ok := expected[detail.Model]
			if !ok {
				continue
			}
			if detail.Type != ProviderModelTypeAudio {
				t.Fatalf("%s type=%q, want %q", detail.Model, detail.Type, ProviderModelTypeAudio)
			}
			if detail.InputPrice != want.input {
				t.Fatalf("%s input_price=%v, want %v", detail.Model, detail.InputPrice, want.input)
			}
			if detail.OutputPrice != want.output {
				t.Fatalf("%s output_price=%v, want %v", detail.Model, detail.OutputPrice, want.output)
			}
			if detail.PriceUnit != ProviderPriceUnitPer1KTokens {
				t.Fatalf("%s price_unit=%q, want %q", detail.Model, detail.PriceUnit, ProviderPriceUnitPer1KTokens)
			}
			if detail.Currency != ProviderPriceCurrencyUSD {
				t.Fatalf("%s currency=%q, want %q", detail.Model, detail.Currency, ProviderPriceCurrencyUSD)
			}
			if len(detail.SupportedEndpoints) != 1 || detail.SupportedEndpoints[0] != ChannelModelEndpointRealtime {
				t.Fatalf("%s supported_endpoints=%#v, want [%s]", detail.Model, detail.SupportedEndpoints, ChannelModelEndpointRealtime)
			}
			found[detail.Model] = true
		}
		for modelName := range expected {
			if !found[modelName] {
				t.Fatalf("expected openai seed to include %s", modelName)
			}
		}
		return
	}
	t.Fatalf("expected openai provider to exist")
}

func TestBuildProviderMigrationSeeds_OfficialPricingBackfillForPreviouslyUnpricedModels(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	expected := map[string]map[string]struct {
		modelType string
		input     float64
		output    float64
		priceUnit string
		currency  string
	}{
		"openai": {
			"gpt-audio":      {modelType: ProviderModelTypeAudio, input: 0.032, output: 0.064, priceUnit: ProviderPriceUnitPer1KTokens, currency: ProviderPriceCurrencyUSD},
			"gpt-audio-mini": {modelType: ProviderModelTypeAudio, input: 0.01, output: 0.02, priceUnit: ProviderPriceUnitPer1KTokens, currency: ProviderPriceCurrencyUSD},
			"whisper-1":      {modelType: ProviderModelTypeAudio, input: 0.006, priceUnit: ProviderPriceUnitPerMinute, currency: ProviderPriceCurrencyUSD},
			"tts-1":          {modelType: ProviderModelTypeAudio, input: 0.015, priceUnit: ProviderPriceUnitPer1KChars, currency: ProviderPriceCurrencyUSD},
			"tts-1-hd":       {modelType: ProviderModelTypeAudio, input: 0.03, priceUnit: ProviderPriceUnitPer1KChars, currency: ProviderPriceCurrencyUSD},
			"sora-2":         {modelType: ProviderModelTypeVideo, input: 0.1, priceUnit: ProviderPriceUnitPerSecond, currency: ProviderPriceCurrencyUSD},
			"sora-2-pro":     {modelType: ProviderModelTypeVideo, input: 0.3, priceUnit: ProviderPriceUnitPerSecond, currency: ProviderPriceCurrencyUSD},
		},
		"google": {
			"gemini-2.5-pro":                    {modelType: ProviderModelTypeText, input: 0.00125, output: 0.01, priceUnit: ProviderPriceUnitPer1KTokens, currency: ProviderPriceCurrencyUSD},
			"gemini-2.5-flash-image-preview":    {modelType: ProviderModelTypeImage, input: 0.039, priceUnit: ProviderPriceUnitPerImage, currency: ProviderPriceCurrencyUSD},
			"imagen-4.0-generate-preview-06-06": {modelType: ProviderModelTypeImage, input: 0.04, priceUnit: ProviderPriceUnitPerImage, currency: ProviderPriceCurrencyUSD},
			"veo-3.0-generate-preview":          {modelType: ProviderModelTypeVideo, input: 0.4, priceUnit: ProviderPriceUnitPerSecond, currency: ProviderPriceCurrencyUSD},
		},
		"xai": {
			"grok-2-image-1212": {modelType: ProviderModelTypeImage, input: 0.07, priceUnit: ProviderPriceUnitPerImage, currency: ProviderPriceCurrencyUSD},
		},
		"deepseek": {
			"deepseek-v4-flash": {modelType: ProviderModelTypeText, input: 0.00014, output: 0.00028, priceUnit: ProviderPriceUnitPer1KTokens, currency: ProviderPriceCurrencyUSD},
			"deepseek-v4-pro":   {modelType: ProviderModelTypeText, input: 0.000435, output: 0.00087, priceUnit: ProviderPriceUnitPer1KTokens, currency: ProviderPriceCurrencyUSD},
			"deepseek-chat":     {modelType: ProviderModelTypeText, input: 0.00014, output: 0.00028, priceUnit: ProviderPriceUnitPer1KTokens, currency: ProviderPriceCurrencyUSD},
			"deepseek-reasoner": {modelType: ProviderModelTypeText, input: 0.00014, output: 0.00028, priceUnit: ProviderPriceUnitPer1KTokens, currency: ProviderPriceCurrencyUSD},
		},
		"stepfun": {
			"step-1o-turbo-vision": {modelType: ProviderModelTypeImage, input: 0.0025, output: 0.008, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"step-1o-audio":        {modelType: ProviderModelTypeAudio, input: 0.025, output: 0.06, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"step-1x-medium":       {modelType: ProviderModelTypeImage, input: 0.1, priceUnit: ProviderPriceUnitPerImage, currency: "CNY"},
		},
		"qwen": {
			"qwen3.7-max":        {modelType: ProviderModelTypeText, input: 0.012, output: 0.036, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"qwen3.6-plus":       {modelType: ProviderModelTypeText, input: 0.002, output: 0.012, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"qwen3.6-flash":      {modelType: ProviderModelTypeText, input: 0.0012, output: 0.0072, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"qwen3.5-plus":       {modelType: ProviderModelTypeText, input: 0.0008, output: 0.0048, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"qwen3.5-flash":      {modelType: ProviderModelTypeText, input: 0.0002, output: 0.002, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"qwen3-max":          {modelType: ProviderModelTypeText, input: 0.0025, output: 0.01, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"qwen-image-2.0":     {modelType: ProviderModelTypeImage, input: 0.2, priceUnit: ProviderPriceUnitPerImage, currency: "CNY"},
			"qwen-image-2.0-pro": {modelType: ProviderModelTypeImage, input: 0.5, priceUnit: ProviderPriceUnitPerImage, currency: "CNY"},
		},
		"minimax": {
			"speech-2.5-hd-preview": {modelType: ProviderModelTypeAudio, input: 0.1, priceUnit: ProviderPriceUnitPer1KChars, currency: ProviderPriceCurrencyUSD},
			"image-01":              {modelType: ProviderModelTypeImage, input: 0.0035, priceUnit: ProviderPriceUnitPerImage, currency: ProviderPriceCurrencyUSD},
		},
		"zhipu": {
			"glm-5.2":          {modelType: ProviderModelTypeText, input: 0.008, output: 0.028, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"glm-5.1":          {modelType: ProviderModelTypeText, input: 0.006, output: 0.024, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"glm-5-turbo":      {modelType: ProviderModelTypeText, input: 0.005, output: 0.022, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"glm-5":            {modelType: ProviderModelTypeText, input: 0.004, output: 0.018, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"glm-5v-turbo":     {modelType: ProviderModelTypeImage, input: 0.005, output: 0.022, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"glm-4.7":          {modelType: ProviderModelTypeText, input: 0.002, output: 0.008, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"glm-4.7-flashx":   {modelType: ProviderModelTypeText, input: 0.0005, output: 0.003, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"glm-4.6v":         {modelType: ProviderModelTypeImage, input: 0.001, output: 0.003, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"glm-4.6v-flashx":  {modelType: ProviderModelTypeImage, input: 0.00015, output: 0.0015, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"glm-4.5v":         {modelType: ProviderModelTypeImage, input: 0.002, output: 0.006, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"glm-4.5-air":      {modelType: ProviderModelTypeText, input: 0.0008, output: 0.002, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"glm-4v-plus-0111": {modelType: ProviderModelTypeImage, input: 0.004, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"glm-4-voice":      {modelType: ProviderModelTypeAudio, input: 0.08, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"cogview-4-250304": {modelType: ProviderModelTypeImage, input: 0.06, priceUnit: ProviderPriceUnitPerImage, currency: "CNY"},
			"glm-image":        {modelType: ProviderModelTypeImage, input: 0.1, priceUnit: ProviderPriceUnitPerImage, currency: "CNY"},
			"cogvideox-2":      {modelType: ProviderModelTypeVideo, input: 0.5, priceUnit: ProviderPriceUnitPerVideo, currency: "CNY"},
			"embedding-2":      {modelType: ProviderModelTypeEmbedding, input: 0.0005, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"embedding-3":      {modelType: ProviderModelTypeEmbedding, input: 0.0005, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
		},
		"hunyuan": {
			"Hunyuan-Image": {modelType: ProviderModelTypeImage, input: 0.2, priceUnit: ProviderPriceUnitPerImage, currency: "CNY"},
		},
		"volcengine": {
			"doubao-embedding-vision-251215":      {modelType: ProviderModelTypeEmbedding, input: 0.0007, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-1.6":                     {modelType: ProviderModelTypeText, input: 0.0008, output: 0.008, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-1-6-vision-250815":       {modelType: ProviderModelTypeText, input: 0.0008, output: 0.008, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-1.6-thinking":            {modelType: ProviderModelTypeText, input: 0.0008, output: 0.008, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-1.6-flash":               {modelType: ProviderModelTypeText, input: 0.00015, output: 0.0015, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-1-8-251228":              {modelType: ProviderModelTypeText, input: 0.0008, output: 0.008, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-2-0-code-preview-260215": {modelType: ProviderModelTypeText, input: 0.0032, output: 0.016, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-2-0-lite-260428":         {modelType: ProviderModelTypeText, input: 0.0006, output: 0.0036, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-2-0-mini-260428":         {modelType: ProviderModelTypeText, input: 0.0002, output: 0.002, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-2-0-pro-260215":          {modelType: ProviderModelTypeText, input: 0.0032, output: 0.016, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-character-251128":        {modelType: ProviderModelTypeText, input: 0.0008, output: 0.002, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-code-preview-251028":     {modelType: ProviderModelTypeText, input: 0.0012, output: 0.008, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-code-preview-latest":     {modelType: ProviderModelTypeText, input: 0.0012, output: 0.008, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-translation-250915":      {modelType: ProviderModelTypeText, input: 0.0012, output: 0.0036, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seedream-5-0-lite-260128":     {modelType: ProviderModelTypeImage, input: 0.22, priceUnit: ProviderPriceUnitPerImage, currency: "CNY"},
			"doubao-seedream-4-5-251128":          {modelType: ProviderModelTypeImage, input: 0.25, priceUnit: ProviderPriceUnitPerImage, currency: "CNY"},
			"doubao-seedream-4-0-250828":          {modelType: ProviderModelTypeImage, input: 0.2, priceUnit: ProviderPriceUnitPerImage, currency: "CNY"},
			"doubao-seedance-2-0-260128":          {modelType: ProviderModelTypeVideo, output: 0.046, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seedance-2-0-fast-260128":     {modelType: ProviderModelTypeVideo, output: 0.037, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seedance-1-5-pro-251215":      {modelType: ProviderModelTypeVideo, output: 0.016, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seedance-1-0-pro-250528":      {modelType: ProviderModelTypeVideo, output: 0.015, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seedance-1-0-pro-fast-251015": {modelType: ProviderModelTypeVideo, output: 0.0042, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
		},
	}

	for _, seed := range seeds {
		providerExpected, ok := expected[seed.Provider]
		if !ok {
			continue
		}
		for _, detail := range seed.ModelDetails {
			want, exists := providerExpected[detail.Model]
			if !exists {
				continue
			}
			if detail.Type != want.modelType {
				t.Fatalf("%s/%s type=%q, want %q", seed.Provider, detail.Model, detail.Type, want.modelType)
			}
			if detail.InputPrice != want.input {
				t.Fatalf("%s/%s input_price=%v, want %v", seed.Provider, detail.Model, detail.InputPrice, want.input)
			}
			if detail.OutputPrice != want.output {
				t.Fatalf("%s/%s output_price=%v, want %v", seed.Provider, detail.Model, detail.OutputPrice, want.output)
			}
			if detail.PriceUnit != want.priceUnit {
				t.Fatalf("%s/%s price_unit=%q, want %q", seed.Provider, detail.Model, detail.PriceUnit, want.priceUnit)
			}
			if detail.Currency != want.currency {
				t.Fatalf("%s/%s currency=%q, want %q", seed.Provider, detail.Model, detail.Currency, want.currency)
			}
			delete(providerExpected, detail.Model)
		}
	}

	for provider, providerExpected := range expected {
		for modelName := range providerExpected {
			t.Fatalf("expected %s seed to include priced model %s", provider, modelName)
		}
	}
}

func TestBuildProviderMigrationSeeds_ComplexPricingComponentsForLiveAndOmniModels(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	checks := map[string]map[string]struct {
		componentCount int
	}{
		"google": {
			"gemini-2.5-pro":                {componentCount: 6},
			"gemini-2.5-flash":              {componentCount: 10},
			"gemini-2.5-flash-lite":         {componentCount: 10},
			"gemini-live-2.5-flash-preview": {componentCount: 5},
		},
		"volcengine": {
			"doubao-embedding-vision-251215":      {componentCount: 2},
			"doubao-seed-1.6":                     {componentCount: 4},
			"doubao-seed-1-6-vision-250815":       {componentCount: 3},
			"doubao-seed-1.6-thinking":            {componentCount: 4},
			"doubao-seed-1.6-flash":               {componentCount: 3},
			"doubao-seed-1-8-251228":              {componentCount: 4},
			"doubao-seed-2-0-code-preview-260215": {componentCount: 3},
			"doubao-seed-2-0-lite-260428":         {componentCount: 3},
			"doubao-seed-2-0-mini-260428":         {componentCount: 3},
			"doubao-seed-2-0-pro-260215":          {componentCount: 3},
			"doubao-seed-character-251128":        {componentCount: 2},
			"doubao-seed-code-preview-251028":     {componentCount: 3},
			"doubao-seed-code-preview-latest":     {componentCount: 3},
			"doubao-seed-translation-250915":      {componentCount: 1},
			"doubao-seedream-5-0-lite-260128":     {componentCount: 1},
			"doubao-seedream-4-5-251128":          {componentCount: 1},
			"doubao-seedream-4-0-250828":          {componentCount: 1},
			"doubao-seedance-2-0-260128":          {componentCount: 4},
			"doubao-seedance-2-0-fast-260128":     {componentCount: 2},
			"doubao-seedance-1-5-pro-251215":      {componentCount: 4},
			"doubao-seedance-1-0-pro-250528":      {componentCount: 2},
			"doubao-seedance-1-0-pro-fast-251015": {componentCount: 2},
		},
		"zhipu": {
			"glm-5.2":         {componentCount: 2},
			"glm-5.1":         {componentCount: 2},
			"glm-5-turbo":     {componentCount: 2},
			"glm-5":           {componentCount: 2},
			"glm-5v-turbo":    {componentCount: 2},
			"glm-4.7":         {componentCount: 3},
			"glm-4.6v":        {componentCount: 2},
			"glm-4.6v-flashx": {componentCount: 2},
			"glm-4.5v":        {componentCount: 2},
			"glm-4.5-air":     {componentCount: 3},
		},
	}

	for _, seed := range seeds {
		providerChecks, ok := checks[seed.Provider]
		if !ok {
			continue
		}
		for _, detail := range seed.ModelDetails {
			want, exists := providerChecks[detail.Model]
			if !exists {
				continue
			}
			if len(detail.PriceComponents) != want.componentCount {
				t.Fatalf("%s/%s price_components=%d, want %d", seed.Provider, detail.Model, len(detail.PriceComponents), want.componentCount)
			}
			wantPriceUnit := ProviderPriceUnitPer1KTokens
			if seed.Provider == "volcengine" && detail.Type == ProviderModelTypeImage {
				wantPriceUnit = ProviderPriceUnitPerImage
			}
			if detail.PriceUnit != wantPriceUnit {
				t.Fatalf("%s/%s price_unit=%q, want %q", seed.Provider, detail.Model, detail.PriceUnit, wantPriceUnit)
			}
			if seed.Provider == "google" && (detail.Model == "gemini-2.5-pro" || detail.Model == "gemini-2.5-flash" || detail.Model == "gemini-2.5-flash-lite") {
				expectedConditions := map[string]bool{}
				switch detail.Model {
				case "gemini-2.5-pro":
					expectedConditions = map[string]bool{
						"mode=standard;prompt_tokens_lte=200000": false,
						"mode=standard;prompt_tokens_gt=200000":  false,
						"mode=batch;prompt_tokens_lte=200000":    false,
						"mode=batch;prompt_tokens_gt=200000":     false,
					}
				case "gemini-2.5-flash", "gemini-2.5-flash-lite":
					expectedConditions = map[string]bool{
						"mode=standard;input_type=text_image_video": false,
						"mode=batch;input_type=text_image_video":    false,
						"mode=standard":                             false,
						"mode=batch":                                false,
					}
				}
				for _, component := range detail.PriceComponents {
					if _, ok := expectedConditions[component.Condition]; ok {
						expectedConditions[component.Condition] = true
					}
				}
				for condition, found := range expectedConditions {
					if !found {
						t.Fatalf("expected %s/%s to include condition %s", seed.Provider, detail.Model, condition)
					}
				}
			}
			delete(providerChecks, detail.Model)
		}
	}

	for provider, providerChecks := range checks {
		for modelName := range providerChecks {
			t.Fatalf("expected %s seed to include complex pricing model %s", provider, modelName)
		}
	}
}

func TestBuildProviderMigrationSeeds_DeepSeekTextModelsSupportChatAndMessages(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	expectedModels := map[string]struct {
		found     bool
		reasoning bool
	}{
		"deepseek-v4-flash": {reasoning: true},
		"deepseek-v4-pro":   {reasoning: true},
		"deepseek-chat":     {},
		"deepseek-reasoner": {reasoning: true},
	}

	for _, seed := range seeds {
		if seed.Provider != "deepseek" {
			continue
		}
		for _, detail := range seed.ModelDetails {
			expected, ok := expectedModels[detail.Model]
			if !ok {
				continue
			}
			if len(detail.SupportedEndpoints) != 2 ||
				detail.SupportedEndpoints[0] != ChannelModelEndpointChat ||
				detail.SupportedEndpoints[1] != ChannelModelEndpointMessages {
				t.Fatalf("%s supported_endpoints=%#v, want [chat messages]", detail.Model, detail.SupportedEndpoints)
			}
			if expected.reasoning && !providerModelTagsContain(detail.Tags, ProviderModelTagReasoning) {
				t.Fatalf("%s tags=%#v, want reasoning tag", detail.Model, detail.Tags)
			}
			expected.found = true
			expectedModels[detail.Model] = expected
		}
	}

	for modelName, expected := range expectedModels {
		if !expected.found {
			t.Fatalf("expected deepseek seed to include %s", modelName)
		}
	}
}

func TestBuildProviderMigrationSeeds_ZhipuClaudeCompatibleModelsExposeMessagesEndpoint(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	expectedModels := map[string]bool{
		"glm-5.2": false,
		"glm-5.1": false,
		"glm-5":   false,
		"glm-4.7": false,
	}

	for _, seed := range seeds {
		if seed.Provider != "zhipu" {
			continue
		}
		for _, detail := range seed.ModelDetails {
			if providerModelEndpointsContain(detail.SupportedEndpoints, ChannelModelEndpointResponses) {
				t.Fatalf("%s supported_endpoints=%#v, zhipu should not expose responses endpoint", detail.Model, detail.SupportedEndpoints)
			}
			if _, ok := expectedModels[detail.Model]; !ok {
				continue
			}
			if len(detail.SupportedEndpoints) != 2 ||
				detail.SupportedEndpoints[0] != ChannelModelEndpointChat ||
				detail.SupportedEndpoints[1] != ChannelModelEndpointMessages {
				t.Fatalf("%s supported_endpoints=%#v, want [chat messages]", detail.Model, detail.SupportedEndpoints)
			}
			expectedModels[detail.Model] = true
		}
	}

	for modelName, found := range expectedModels {
		if !found {
			t.Fatalf("expected zhipu seed to include %s", modelName)
		}
	}
}

func TestBuildProviderMigrationSeeds_ZhipuIncludesGLM52OfficialPricing(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	for _, seed := range seeds {
		if seed.Provider != "zhipu" {
			continue
		}
		for _, detail := range seed.ModelDetails {
			if detail.Model != "glm-5.2" {
				continue
			}
			if detail.Type != ProviderModelTypeText {
				t.Fatalf("glm-5.2 type=%q, want %q", detail.Type, ProviderModelTypeText)
			}
			if !providerModelTagsContain(detail.Tags, ProviderModelTagReasoning) {
				t.Fatalf("glm-5.2 tags=%#v, want reasoning tag", detail.Tags)
			}
			if len(detail.SupportedEndpoints) != 2 ||
				detail.SupportedEndpoints[0] != ChannelModelEndpointChat ||
				detail.SupportedEndpoints[1] != ChannelModelEndpointMessages {
				t.Fatalf("glm-5.2 supported_endpoints=%#v, want [chat messages]", detail.SupportedEndpoints)
			}
			if detail.InputPrice != 0.008 || detail.OutputPrice != 0.028 {
				t.Fatalf("glm-5.2 pricing input=%v output=%v, want 0.008/0.028", detail.InputPrice, detail.OutputPrice)
			}
			if detail.PriceUnit != ProviderPriceUnitPer1KTokens || detail.Currency != "CNY" {
				t.Fatalf("glm-5.2 billing unit=%q currency=%q, want per_1k_tokens/CNY", detail.PriceUnit, detail.Currency)
			}
			if len(detail.PriceComponents) != 2 {
				t.Fatalf("glm-5.2 price_components=%d, want 2", len(detail.PriceComponents))
			}
			expectedComponents := map[string]bool{
				"text|mode=standard":          false,
				"context_cache|mode=standard": false,
			}
			for _, component := range detail.PriceComponents {
				key := component.Component + "|" + component.Condition
				if _, ok := expectedComponents[key]; ok {
					expectedComponents[key] = true
				}
			}
			for key, found := range expectedComponents {
				if !found {
					t.Fatalf("glm-5.2 missing price component %s", key)
				}
			}
			return
		}
		t.Fatalf("expected zhipu seed to include glm-5.2")
	}
	t.Fatalf("expected zhipu provider to exist")
}

func TestBuildProviderMigrationSeeds_ZhipuIncludesGLMImageSpecification(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	for _, seed := range seeds {
		if seed.Provider != "zhipu" {
			continue
		}
		for _, detail := range seed.ModelDetails {
			if detail.Model != "glm-image" {
				continue
			}
			if detail.Type != ProviderModelTypeImage {
				t.Fatalf("glm-image type=%q, want %q", detail.Type, ProviderModelTypeImage)
			}
			if detail.Specification == nil {
				t.Fatal("glm-image specification should not be nil")
			}
			spec, ok := detail.Specification.Endpoints[ChannelModelEndpointImages]
			if !ok {
				t.Fatalf("glm-image endpoints=%#v, want /v1/images/generations", detail.Specification.Endpoints)
			}
			if spec.Constraints == nil || spec.Constraints.MinPixels == nil || *spec.Constraints.MinPixels != 3686400 {
				t.Fatalf("glm-image min_pixels=%v, want 3686400", spec.Constraints)
			}
			if len(spec.Parameters["size"].AllowedValues) != 3 {
				t.Fatalf("glm-image size allowed_values=%#v, want 3 values", spec.Parameters["size"].AllowedValues)
			}
			return
		}
		t.Fatalf("expected zhipu seed to include glm-image")
	}
	t.Fatalf("expected zhipu provider to exist")
}

func TestBuildProviderMigrationSeeds_ImageSpecificationsFromOfficialProviders(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	expected := map[string]func(detail ProviderModelDetail, t *testing.T){
		"openai/dall-e-3": func(detail ProviderModelDetail, t *testing.T) {
			spec := detail.Specification.Endpoints[ChannelModelEndpointImages]
			if spec.Parameters["n"].Max == nil || *spec.Parameters["n"].Max != 1 {
				t.Fatalf("dall-e-3 n max=%v, want 1", spec.Parameters["n"].Max)
			}
			if len(spec.Parameters["size"].AllowedValues) != 3 {
				t.Fatalf("dall-e-3 size allowed_values=%#v, want 3", spec.Parameters["size"].AllowedValues)
			}
		},
		"openai/gpt-image-1": func(detail ProviderModelDetail, t *testing.T) {
			genSpec := detail.Specification.Endpoints[ChannelModelEndpointImages]
			editSpec := detail.Specification.Endpoints[ChannelModelEndpointImageEdit]
			if len(genSpec.Parameters["quality"].AllowedValues) != 3 {
				t.Fatalf("gpt-image-1 quality allowed_values=%#v, want 3", genSpec.Parameters["quality"].AllowedValues)
			}
			if len(editSpec.InputModalities) != 2 {
				t.Fatalf("gpt-image-1 edit input_modalities=%#v, want image+text", editSpec.InputModalities)
			}
		},
		"google/gemini-2.5-flash-image-preview": func(detail ProviderModelDetail, t *testing.T) {
			spec := detail.Specification.Endpoints[ChannelModelEndpointImages]
			if len(spec.Parameters["size"].AllowedValues) != 10 {
				t.Fatalf("gemini image preview size allowed_values=%#v, want 10", spec.Parameters["size"].AllowedValues)
			}
		},
		"google/imagen-4.0-generate-preview-06-06": func(detail ProviderModelDetail, t *testing.T) {
			spec := detail.Specification.Endpoints[ChannelModelEndpointImages]
			if spec.Parameters["n"].Max == nil || *spec.Parameters["n"].Max != 4 {
				t.Fatalf("imagen-4 n max=%v, want 4", spec.Parameters["n"].Max)
			}
		},
		"hunyuan/Hunyuan-Image": func(detail ProviderModelDetail, t *testing.T) {
			spec := detail.Specification.Endpoints[ChannelModelEndpointImages]
			if len(spec.Parameters["size"].AllowedValues) != 5 {
				t.Fatalf("Hunyuan-Image size allowed_values=%#v, want 5", spec.Parameters["size"].AllowedValues)
			}
		},
		"minimax/image-01": func(detail ProviderModelDetail, t *testing.T) {
			spec := detail.Specification.Endpoints[ChannelModelEndpointImages]
			if spec.Constraints == nil || spec.Constraints.EdgeMultiple == nil || *spec.Constraints.EdgeMultiple != 8 {
				t.Fatalf("image-01 edge_multiple=%v, want 8", spec.Constraints)
			}
			if spec.Parameters["width"].Min == nil || *spec.Parameters["width"].Min != 512 {
				t.Fatalf("image-01 width min=%v, want 512", spec.Parameters["width"].Min)
			}
		},
		"qwen/qwen-image-2.0": func(detail ProviderModelDetail, t *testing.T) {
			spec := detail.Specification.Endpoints[ChannelModelEndpointImages]
			if spec.Parameters["n"].Max == nil || *spec.Parameters["n"].Max != 6 {
				t.Fatalf("qwen-image-2.0 n max=%v, want 6", spec.Parameters["n"].Max)
			}
			if spec.Constraints == nil || spec.Constraints.MaxPixels == nil || *spec.Constraints.MaxPixels != 4194304 {
				t.Fatalf("qwen-image-2.0 max_pixels=%v, want 4194304", spec.Constraints)
			}
		},
		"stepfun/step-1x-medium": func(detail ProviderModelDetail, t *testing.T) {
			spec := detail.Specification.Endpoints[ChannelModelEndpointImages]
			if len(spec.Parameters["size"].AllowedValues) != 6 {
				t.Fatalf("step-1x-medium size allowed_values=%#v, want 6", spec.Parameters["size"].AllowedValues)
			}
		},
		"volcengine/doubao-seedream-4-0-250828": func(detail ProviderModelDetail, t *testing.T) {
			spec := detail.Specification.Endpoints[ChannelModelEndpointImages]
			if spec.Constraints == nil || spec.Constraints.MinPixels == nil || *spec.Constraints.MinPixels != 921600 {
				t.Fatalf("doubao-seedream-4-0 min_pixels=%v, want 921600", spec.Constraints)
			}
		},
		"volcengine/doubao-seedream-5-0-lite-260128": func(detail ProviderModelDetail, t *testing.T) {
			spec := detail.Specification.Endpoints[ChannelModelEndpointImages]
			if len(spec.Parameters["size"].AllowedValues) < 20 {
				t.Fatalf("doubao-seedream-5-0-lite size allowed_values=%#v, want >=20", spec.Parameters["size"].AllowedValues)
			}
			if spec.Constraints == nil || spec.Constraints.MinPixels == nil || *spec.Constraints.MinPixels != 3686400 {
				t.Fatalf("doubao-seedream-5-0-lite min_pixels=%v, want 3686400", spec.Constraints)
			}
		},
	}

	found := make(map[string]bool, len(expected))
	for _, seed := range seeds {
		for _, detail := range seed.ModelDetails {
			key := seed.Provider + "/" + detail.Model
			check, ok := expected[key]
			if !ok {
				continue
			}
			if detail.Specification == nil {
				t.Fatalf("%s specification should not be nil", key)
			}
			check(detail, t)
			found[key] = true
		}
	}
	for key := range expected {
		if !found[key] {
			t.Fatalf("expected provider migration seed to include specification for %s", key)
		}
	}
}

func TestBuildProviderMigrationSeeds_ZhipuEmbeddingModelsUseEmbeddingsEndpoint(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	expectedModels := map[string]bool{
		"embedding-2": false,
		"embedding-3": false,
	}

	for _, seed := range seeds {
		if seed.Provider != "zhipu" {
			continue
		}
		for _, detail := range seed.ModelDetails {
			if _, ok := expectedModels[detail.Model]; !ok {
				continue
			}
			if detail.Type != ProviderModelTypeEmbedding {
				t.Fatalf("%s type=%q, want %q", detail.Model, detail.Type, ProviderModelTypeEmbedding)
			}
			if len(detail.SupportedEndpoints) != 1 || detail.SupportedEndpoints[0] != ChannelModelEndpointEmbeddings {
				t.Fatalf("%s supported_endpoints=%#v, want [%s]", detail.Model, detail.SupportedEndpoints, ChannelModelEndpointEmbeddings)
			}
			expectedModels[detail.Model] = true
		}
	}

	for modelName, found := range expectedModels {
		if !found {
			t.Fatalf("expected zhipu seed to include %s", modelName)
		}
	}
}

func providerModelTagsContain(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}

func providerModelEndpointsContain(endpoints []string, target string) bool {
	for _, endpoint := range endpoints {
		if endpoint == target {
			return true
		}
	}
	return false
}

func TestInferModelType_RecognizesGPTImageModels(t *testing.T) {
	if got := InferModelType("gpt-image-1"); got != ProviderModelTypeImage {
		t.Fatalf("InferModelType(gpt-image-1) = %q, want %q", got, ProviderModelTypeImage)
	}
	if got := InferModelType("gpt-image-2"); got != ProviderModelTypeImage {
		t.Fatalf("InferModelType(gpt-image-2) = %q, want %q", got, ProviderModelTypeImage)
	}
}

func TestBuildProviderMigrationSeeds_AnthropicIncludesClaude47AndLegacyPricing(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	expected := map[string]struct {
		input  float64
		output float64
	}{
		"claude-opus-4-8":            {input: 0.005, output: 0.025},
		"claude-opus-4-7":            {input: 0.005, output: 0.025},
		"claude-opus-4-6-thinking":   {input: 0.005, output: 0.025},
		"claude-sonnet-4-5-20250929": {input: 0.003, output: 0.015},
		"claude-sonnet-4-6":          {input: 0.003, output: 0.015},
		"claude-opus-4-6":            {input: 0.005, output: 0.025},
		"claude-haiku-4-5-20251001":  {input: 0.001, output: 0.005},
		"claude-opus-4-5-20251101":   {input: 0.005, output: 0.025},
		"claude-3-5-haiku-20241022":  {input: 0.0008, output: 0.004},
	}

	for _, seed := range seeds {
		if seed.Provider != "anthropic" {
			continue
		}
		found := make(map[string]bool, len(expected))
		for _, detail := range seed.ModelDetails {
			want, ok := expected[detail.Model]
			if !ok {
				continue
			}
			if detail.Type != ProviderModelTypeText {
				t.Fatalf("%s type=%q, want %q", detail.Model, detail.Type, ProviderModelTypeText)
			}
			if detail.InputPrice != want.input {
				t.Fatalf("%s input_price=%v, want %v", detail.Model, detail.InputPrice, want.input)
			}
			if detail.OutputPrice != want.output {
				t.Fatalf("%s output_price=%v, want %v", detail.Model, detail.OutputPrice, want.output)
			}
			if detail.PriceUnit != ProviderPriceUnitPer1KTokens {
				t.Fatalf("%s price_unit=%q, want %q", detail.Model, detail.PriceUnit, ProviderPriceUnitPer1KTokens)
			}
			if detail.Currency != ProviderPriceCurrencyUSD {
				t.Fatalf("%s currency=%q, want %q", detail.Model, detail.Currency, ProviderPriceCurrencyUSD)
			}
			found[detail.Model] = true
		}
		for modelName := range expected {
			if !found[modelName] {
				t.Fatalf("expected anthropic seed to include %s", modelName)
			}
		}
		return
	}
	t.Fatalf("expected anthropic provider to exist")
}

func TestBuildProviderMigrationSeeds_ModelDetailsMeta(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	if len(seeds) == 0 {
		t.Fatalf("expected non-empty provider seeds")
	}

	hasText := false
	hasImage := false
	hasAudio := false
	hasVideo := false
	hasEmbedding := false
	totalModels := 0

	for _, seed := range seeds {
		if strings.TrimSpace(seed.OfficialURL) == "" {
			t.Fatalf("official_url should not be empty for provider %q", seed.Provider)
		}
		for _, detail := range seed.ModelDetails {
			totalModels++
			switch detail.Type {
			case ProviderModelTypeText:
				hasText = true
			case ProviderModelTypeImage:
				hasImage = true
			case ProviderModelTypeAudio:
				hasAudio = true
			case ProviderModelTypeVideo:
				hasVideo = true
			case ProviderModelTypeEmbedding:
				hasEmbedding = true
			default:
				t.Fatalf("unexpected model type %q for model %q", detail.Type, detail.Model)
			}
			if detail.PriceUnit == "" {
				t.Fatalf("price_unit should not be empty for model %q", detail.Model)
			}
			if detail.Currency == "" {
				t.Fatalf("currency should not be empty for model %q", detail.Model)
			}
			if detail.InputPrice < 0 {
				t.Fatalf("input_price should not be negative for model %q", detail.Model)
			}
			if detail.OutputPrice < 0 {
				t.Fatalf("output_price should not be negative for model %q", detail.Model)
			}
		}
	}

	if totalModels == 0 {
		t.Fatalf("expected non-empty model details in provider seeds")
	}
	if !hasText {
		t.Fatalf("expected at least one text model")
	}
	if !hasImage {
		t.Fatalf("expected at least one image model")
	}
	if !hasAudio {
		t.Fatalf("expected at least one audio model")
	}
	if !hasVideo {
		t.Fatalf("expected at least one video model")
	}
	if !hasEmbedding {
		t.Fatalf("expected at least one embedding model")
	}
}

func TestBuildProviderMigrationSeeds_AssignsSortOrder(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	if len(seeds) == 0 {
		t.Fatalf("expected non-empty provider seeds")
	}
	prev := 0
	for _, seed := range seeds {
		if seed.SortOrder <= 0 {
			t.Fatalf("sort_order should be positive for provider %q", seed.Provider)
		}
		if seed.SortOrder <= prev {
			t.Fatalf("sort_order should be strictly ascending, prev=%d current=%d provider=%q", prev, seed.SortOrder, seed.Provider)
		}
		prev = seed.SortOrder
	}
}

func TestBuildProviderMigrationSeeds_RemainingUnpricedModelsAreExplicitlyTracked(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	expected := map[string]map[string]bool{
		"baidu": {
			"ernie-4.5-vl-32k-preview": false,
		},
		"hunyuan": {
			"Hunyuan-Video": false,
		},
		"minimax": {
			"video-01": false,
		},
		"zhipu": {
			"cogvideox-flash": false,
			"glm-4.6v-flash":  false,
			"glm-4.7-flash":   false,
		},
		"mistral": {
			"pixtral-large-latest": false,
			"voxtral-mini-latest":  false,
		},
		"volcengine": {
			"Seed1.6-Embedding": false,
		},
	}

	for _, seed := range seeds {
		providerExpected, ok := expected[seed.Provider]
		if !ok {
			continue
		}
		for _, detail := range seed.ModelDetails {
			found, exists := providerExpected[detail.Model]
			if !exists {
				continue
			}
			if found {
				t.Fatalf("duplicate unresolved model tracking for %s/%s", seed.Provider, detail.Model)
			}
			if detail.InputPrice != 0 || detail.OutputPrice != 0 {
				t.Fatalf("%s/%s unexpectedly has resolved pricing (%v, %v)", seed.Provider, detail.Model, detail.InputPrice, detail.OutputPrice)
			}
			providerExpected[detail.Model] = true
		}
	}

	for provider, providerExpected := range expected {
		for modelName, found := range providerExpected {
			if !found {
				t.Fatalf("expected unresolved pricing tracker to include %s/%s", provider, modelName)
			}
		}
	}
}

func TestInferModelTypeAndPriceUnitForVideo(t *testing.T) {
	modelName := "veo-3.0-generate-preview"
	if got := InferModelType(modelName); got != ProviderModelTypeVideo {
		t.Fatalf("InferModelType(%q)=%q, want %q", modelName, got, ProviderModelTypeVideo)
	}
	if got := defaultPriceUnitByType("", modelName); got != ProviderPriceUnitPerVideo {
		t.Fatalf("defaultPriceUnitByType(%q)=%q, want %q", modelName, got, ProviderPriceUnitPerVideo)
	}
}

func TestBuildProviderMigrationSeeds_IncludesVolcengineEmbeddingModel(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	for _, seed := range seeds {
		if seed.Provider != "volcengine" {
			continue
		}
		for _, detail := range seed.ModelDetails {
			if detail.Model != "Seed1.6-Embedding" {
				continue
			}
			if detail.Type != ProviderModelTypeEmbedding {
				t.Fatalf("Seed1.6-Embedding type=%q, want %q", detail.Type, ProviderModelTypeEmbedding)
			}
			if len(detail.SupportedEndpoints) != 1 || detail.SupportedEndpoints[0] != ChannelModelEndpointEmbeddings {
				t.Fatalf("Seed1.6-Embedding supported_endpoints=%#v, want [%s]", detail.SupportedEndpoints, ChannelModelEndpointEmbeddings)
			}
			if detail.PriceUnit != ProviderPriceUnitPer1KTokens {
				t.Fatalf("Seed1.6-Embedding price_unit=%q, want %q", detail.PriceUnit, ProviderPriceUnitPer1KTokens)
			}
			if detail.Currency != "CNY" {
				t.Fatalf("Seed1.6-Embedding currency=%q, want %q", detail.Currency, "CNY")
			}
			return
		}
		t.Fatalf("expected volcengine Seed1.6-Embedding to exist")
	}
	t.Fatalf("expected volcengine provider to exist")
}

func TestBuildProviderMigrationSeeds_VolcengineUsesOfficialModelIDs(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	expected := map[string]string{
		"doubao-seed-2-0-pro-260215":          ProviderModelTypeText,
		"doubao-seed-2-0-lite-260428":         ProviderModelTypeText,
		"doubao-seed-2-0-mini-260428":         ProviderModelTypeText,
		"doubao-seed-2-0-code-preview-260215": ProviderModelTypeText,
		"doubao-seed-1-8-251228":              ProviderModelTypeText,
		"doubao-seed-1-6-vision-250815":       ProviderModelTypeText,
		"doubao-seed-code-preview-251028":     ProviderModelTypeText,
		"doubao-seed-translation-250915":      ProviderModelTypeText,
		"doubao-seed-character-251128":        ProviderModelTypeText,
		"doubao-embedding-vision-251215":      ProviderModelTypeEmbedding,
		"doubao-seedream-5-0-lite-260128":     ProviderModelTypeImage,
		"doubao-seedance-2-0-260128":          ProviderModelTypeVideo,
	}
	legacyNames := map[string]bool{
		"doubao-seed-2.0-pro":     true,
		"doubao-seed-2.0-lite":    true,
		"doubao-seed-2.0-mini":    true,
		"doubao-seed-2.0-code":    true,
		"doubao-seed-1.8":         true,
		"doubao-seed-1.6-vision":  true,
		"doubao-seed-code":        true,
		"doubao-seed-translation": true,
		"doubao-seed-character":   true,
		"doubao-embedding-vision": true,
	}
	for _, seed := range seeds {
		if seed.Provider != "volcengine" {
			continue
		}
		found := make(map[string]bool, len(expected))
		for _, detail := range seed.ModelDetails {
			if legacyNames[detail.Model] {
				t.Fatalf("volcengine model %q should use official model id", detail.Model)
			}
			wantType, ok := expected[detail.Model]
			if !ok {
				continue
			}
			if detail.Type != wantType {
				t.Fatalf("%s type=%q, want %q", detail.Model, detail.Type, wantType)
			}
			switch wantType {
			case ProviderModelTypeEmbedding:
				if len(detail.SupportedEndpoints) != 1 || detail.SupportedEndpoints[0] != ChannelModelEndpointEmbeddings {
					t.Fatalf("%s supported_endpoints=%#v, want embeddings", detail.Model, detail.SupportedEndpoints)
				}
			case ProviderModelTypeImage:
				if len(detail.SupportedEndpoints) != 1 || detail.SupportedEndpoints[0] != ChannelModelEndpointImages {
					t.Fatalf("%s supported_endpoints=%#v, want images", detail.Model, detail.SupportedEndpoints)
				}
			case ProviderModelTypeVideo:
				if len(detail.SupportedEndpoints) != 0 {
					t.Fatalf("%s supported_endpoints=%#v, want empty endpoints until volcengine video task protocol is supported", detail.Model, detail.SupportedEndpoints)
				}
			default:
				if len(detail.SupportedEndpoints) != 2 ||
					detail.SupportedEndpoints[0] != ChannelModelEndpointChat ||
					detail.SupportedEndpoints[1] != ChannelModelEndpointResponses {
					t.Fatalf("%s supported_endpoints=%#v, want chat+responses", detail.Model, detail.SupportedEndpoints)
				}
				if providerModelEndpointsContain(detail.SupportedEndpoints, ChannelModelEndpointMessages) {
					t.Fatalf("%s supported_endpoints=%#v should not expose messages", detail.Model, detail.SupportedEndpoints)
				}
			}
			found[detail.Model] = true
		}
		for modelName := range expected {
			if !found[modelName] {
				t.Fatalf("expected volcengine seed to include official model %s", modelName)
			}
		}
		return
	}
	t.Fatalf("expected volcengine provider to exist")
}

func TestBuildProviderMigrationSeeds_HasUniqueCanonicalProviders(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	seen := make(map[string]struct{}, len(seeds))
	for _, seed := range seeds {
		if _, ok := seen[seed.Provider]; ok {
			t.Fatalf("duplicate canonical provider found in seeds: %q", seed.Provider)
		}
		seen[seed.Provider] = struct{}{}
	}
	if _, ok := seen["anthropic"]; !ok {
		t.Fatalf("expected anthropic provider to exist")
	}
	if _, ok := seen["cohere"]; !ok {
		t.Fatalf("expected cohere provider to exist")
	}
	if _, ok := seen["google"]; !ok {
		t.Fatalf("expected google provider to exist")
	}
	if _, ok := seen["openai"]; !ok {
		t.Fatalf("expected openai provider to exist")
	}
	if _, ok := seen["xai"]; !ok {
		t.Fatalf("expected xai provider to exist")
	}
	if _, ok := seen["mistral"]; !ok {
		t.Fatalf("expected mistral provider to exist")
	}
	if _, ok := seen["volcengine"]; !ok {
		t.Fatalf("expected volcengine provider to exist")
	}
}

func TestBuildProviderMigrationSeeds_StripsSelfPrefixes(t *testing.T) {
	seeds := mustLoadProviderMigrationSeeds(t)
	for _, seed := range seeds {
		for _, detail := range seed.ModelDetails {
			if !strings.Contains(detail.Model, "/") {
				continue
			}
			parts := strings.SplitN(detail.Model, "/", 2)
			if len(parts) != 2 {
				continue
			}
			if commonProvider := strings.ToLower(parts[0]); commonProvider == seed.Provider {
				t.Fatalf("provider %q still contains self-prefixed model %q", seed.Provider, detail.Model)
			}
		}
	}
}
