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
			if len(detail.PriceComponents) != 9 {
				t.Fatalf("gpt-image-1 price_components=%d, want 9", len(detail.PriceComponents))
			}
			return
		}
		t.Fatalf("expected openai seed to include gpt-image-1")
	}
	t.Fatalf("expected openai provider to exist")
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

func TestInferModelTypeAndPriceUnitForVideo(t *testing.T) {
	modelName := "veo-3.0-generate-preview"
	if got := InferModelType(modelName); got != ProviderModelTypeVideo {
		t.Fatalf("InferModelType(%q)=%q, want %q", modelName, got, ProviderModelTypeVideo)
	}
	if got := defaultPriceUnitByType("", modelName); got != ProviderPriceUnitPerVideo {
		t.Fatalf("defaultPriceUnitByType(%q)=%q, want %q", modelName, got, ProviderPriceUnitPerVideo)
	}
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
