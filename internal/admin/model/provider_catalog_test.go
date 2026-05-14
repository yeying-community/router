package model

import (
	"strings"
	"testing"
)

func TestBuildDefaultProviderCatalogSeeds_OpenAIIncludesDALLE3(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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

func TestBuildDefaultProviderCatalogSeeds_OpenAIIncludesGPTImage1ComplexPricing(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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

func TestBuildDefaultProviderCatalogSeeds_OpenAIIncludesGPTImage2Pricing(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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
			if len(detail.SupportedEndpoints) != 1 || detail.SupportedEndpoints[0] != ChannelModelEndpointResponses {
				t.Fatalf("gpt-image-2 supported_endpoints=%#v, want [%s]", detail.SupportedEndpoints, ChannelModelEndpointResponses)
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

func TestBuildDefaultProviderCatalogSeeds_TokenBasedImageModelsUseResponsesEndpoint(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
	expected := map[string]string{
		"gpt-image-2":              "openai",
		"ernie-4.5-vl-32k-preview": "baidu",
		"qwen-vl-max-latest":       "qwen",
		"qvq-max-latest":           "qwen",
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
			if len(detail.SupportedEndpoints) != 1 || detail.SupportedEndpoints[0] != ChannelModelEndpointResponses {
				t.Fatalf("%s supported_endpoints=%#v, want [%s]", detail.Model, detail.SupportedEndpoints, ChannelModelEndpointResponses)
			}
			found[detail.Model] = true
		}
	}
	for modelName := range expected {
		if !found[modelName] {
			t.Fatalf("expected default catalog to include %s", modelName)
		}
	}
}

func TestBuildDefaultProviderCatalogSeeds_OpenAIIncludesGPT5xPricing(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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

func TestBuildDefaultProviderCatalogSeeds_OpenAIIncludesGPT55Pricing(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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

func TestBuildDefaultProviderCatalogSeeds_OpenAIIncludesNewOfficialModels(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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

func TestBuildDefaultProviderCatalogSeeds_XAIIncludesNewOfficialModels(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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

func TestBuildDefaultProviderCatalogSeeds_UnknownOrLegacyDescriptionsStayEmpty(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
	checks := map[string]map[string]bool{
		"anthropic": {
			"claude-3-5-haiku-20241022": false,
		},
		"google": {
			"gemini-live-2.5-flash-preview": false,
		},
		"qwen": {
			"qwen-omni-turbo-latest": false,
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

func TestBuildDefaultProviderCatalogSeeds_DeprecatedStatusApplied(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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
		"qwen": {
			"qwen-omni-turbo-latest": false,
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

func TestBuildDefaultProviderCatalogSeeds_AllDescriptionsReviewed(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
	allowedEmpty := map[string]map[string]struct{}{
		"anthropic": {
			"claude-3-5-haiku-20241022": {},
		},
		"google": {
			"gemini-live-2.5-flash-preview": {},
		},
		"qwen": {
			"qwen-omni-turbo-latest": {},
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

func TestBuildDefaultProviderCatalogSeeds_OpenAIIncludesRealtime15And2Pricing(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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

func TestBuildDefaultProviderCatalogSeeds_OfficialPricingBackfillForPreviouslyUnpricedModels(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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
		"stepfun": {
			"step-1o-turbo-vision": {modelType: ProviderModelTypeImage, input: 0.0025, output: 0.008, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"step-1o-audio":        {modelType: ProviderModelTypeAudio, input: 0.025, output: 0.06, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"step-1x-medium":       {modelType: ProviderModelTypeImage, input: 0.1, priceUnit: ProviderPriceUnitPerImage, currency: "CNY"},
		},
		"qwen": {
			"qwen-vl-max-latest": {modelType: ProviderModelTypeImage, input: 0.0016, output: 0.004, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"qvq-max-latest":     {modelType: ProviderModelTypeImage, input: 0.008, output: 0.032, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"qwen-tts-latest":    {modelType: ProviderModelTypeAudio, input: 0.0016, output: 0.01, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
		},
		"minimax": {
			"speech-2.5-hd-preview": {modelType: ProviderModelTypeAudio, input: 0.1, priceUnit: ProviderPriceUnitPer1KChars, currency: ProviderPriceCurrencyUSD},
			"image-01":              {modelType: ProviderModelTypeImage, input: 0.0035, priceUnit: ProviderPriceUnitPerImage, currency: ProviderPriceCurrencyUSD},
		},
		"zhipu": {
			"glm-4v-plus-0111": {modelType: ProviderModelTypeImage, input: 0.004, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"glm-4-voice":      {modelType: ProviderModelTypeAudio, input: 0.08, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"cogview-4-250304": {modelType: ProviderModelTypeImage, input: 0.06, priceUnit: ProviderPriceUnitPerImage, currency: "CNY"},
		},
		"hunyuan": {
			"Hunyuan-Image": {modelType: ProviderModelTypeImage, input: 0.2, priceUnit: ProviderPriceUnitPerImage, currency: "CNY"},
		},
		"volcengine": {
			"doubao-seed-1.6":                 {modelType: ProviderModelTypeText, input: 0.0008, output: 0.008, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-1.6-thinking":        {modelType: ProviderModelTypeText, input: 0.0008, output: 0.008, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-1.6-flash":           {modelType: ProviderModelTypeText, input: 0.00015, output: 0.0015, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
			"doubao-seed-code-preview-latest": {modelType: ProviderModelTypeText, input: 0.0012, output: 0.008, priceUnit: ProviderPriceUnitPer1KTokens, currency: "CNY"},
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

func TestBuildDefaultProviderCatalogSeeds_ComplexPricingComponentsForLiveAndOmniModels(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
	checks := map[string]map[string]struct {
		componentCount int
	}{
		"google": {
			"gemini-2.5-pro":                {componentCount: 6},
			"gemini-2.5-flash":              {componentCount: 10},
			"gemini-2.5-flash-lite":         {componentCount: 10},
			"gemini-live-2.5-flash-preview": {componentCount: 5},
		},
		"qwen": {
			"qwen-omni-turbo-latest": {componentCount: 6},
		},
		"volcengine": {
			"doubao-seed-1.6":                 {componentCount: 3},
			"doubao-seed-1.6-thinking":        {componentCount: 3},
			"doubao-seed-code-preview-latest": {componentCount: 3},
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
			if detail.PriceUnit != ProviderPriceUnitPer1KTokens {
				t.Fatalf("%s/%s price_unit=%q, want %q", seed.Provider, detail.Model, detail.PriceUnit, ProviderPriceUnitPer1KTokens)
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

func TestInferModelType_RecognizesGPTImageModels(t *testing.T) {
	if got := InferModelType("gpt-image-1"); got != ProviderModelTypeImage {
		t.Fatalf("InferModelType(gpt-image-1) = %q, want %q", got, ProviderModelTypeImage)
	}
	if got := InferModelType("gpt-image-2"); got != ProviderModelTypeImage {
		t.Fatalf("InferModelType(gpt-image-2) = %q, want %q", got, ProviderModelTypeImage)
	}
}

func TestBuildDefaultProviderCatalogSeeds_AnthropicIncludesClaude47AndLegacyPricing(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
	expected := map[string]struct {
		input  float64
		output float64
	}{
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

func TestBuildDefaultProviderCatalogSeeds_ModelDetailsMeta(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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

func TestBuildDefaultProviderCatalogSeeds_AssignsSortOrder(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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

func TestBuildDefaultProviderCatalogSeeds_RemainingUnpricedModelsAreExplicitlyTracked(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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

func TestBuildDefaultProviderCatalogSeeds_IncludesVolcengineEmbeddingModel(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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

func TestBuildDefaultProviderCatalogSeeds_HasUniqueCanonicalProviders(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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

func TestBuildDefaultProviderCatalogSeeds_StripsSelfPrefixes(t *testing.T) {
	seeds := BuildDefaultProviderCatalogSeeds(1700000000)
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
