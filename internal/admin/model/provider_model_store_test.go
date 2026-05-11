package model

import "testing"

func TestCanonicalizeModelNameForProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		model    string
		want     string
	}{
		{
			name:     "strip openai self prefix",
			provider: "openai",
			model:    "openai/gpt-4o-mini",
			want:     "gpt-4o-mini",
		},
		{
			name:     "keep openrouter namespace model",
			provider: "openrouter",
			model:    "openai/gpt-4o-mini",
			want:     "openai/gpt-4o-mini",
		},
		{
			name:     "keep plain model",
			provider: "openai",
			model:    "gpt-4o-mini",
			want:     "gpt-4o-mini",
		},
		{
			name:     "strip black-forest-labs self prefix",
			provider: "black-forest-labs",
			model:    "black-forest-labs/flux-1.1-pro",
			want:     "flux-1.1-pro",
		},
		{
			name:     "strip x-ai alias prefix for xai provider",
			provider: "xai",
			model:    "x-ai/grok-beta",
			want:     "grok-beta",
		},
		{
			name:     "strip meta alias prefix for meta provider",
			provider: "meta",
			model:    "meta/llama-2-13b-chat",
			want:     "llama-2-13b-chat",
		},
		{
			name:     "strip embedded meta prefix after namespace removal",
			provider: "meta",
			model:    "meta/meta-llama-3-70b",
			want:     "llama-3-70b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canonicalizeModelNameForProvider(tt.provider, tt.model)
			if got != tt.want {
				t.Fatalf("canonicalizeModelNameForProvider(%q,%q)=%q, want %q", tt.provider, tt.model, got, tt.want)
			}
		})
	}
}

func TestBuildProviderModelRows_CanonicalizeAndMergeDuplicates(t *testing.T) {
	rows := BuildProviderModelRows("openai", []ProviderModelDetail{
		{
			Model:       "gpt-3.5-turbo-0613",
			Type:        ProviderModelTypeText,
			Description: "测试模型描述",
			IsDeleted:   true,
			InputPrice:  0,
			OutputPrice: 0.001,
			PriceUnit:   ProviderPriceUnitPer1KTokens,
			Currency:    ProviderPriceCurrencyUSD,
			Source:      "manual",
			UpdatedAt:   100,
		},
		{
			Model:       "openai/gpt-3.5-turbo-0613",
			Type:        ProviderModelTypeText,
			InputPrice:  0.002,
			OutputPrice: 0,
			PriceUnit:   ProviderPriceUnitPer1KTokens,
			Currency:    ProviderPriceCurrencyUSD,
			Source:      "default",
			UpdatedAt:   200,
		},
	}, 300)

	if len(rows) != 1 {
		t.Fatalf("expected 1 canonicalized row, got %d", len(rows))
	}
	if rows[0].Model != "gpt-3.5-turbo-0613" {
		t.Fatalf("expected canonical model name, got %q", rows[0].Model)
	}
	if rows[0].InputPrice <= 0 {
		t.Fatalf("expected merged positive input price, got %f", rows[0].InputPrice)
	}
	if rows[0].Description != "测试模型描述" {
		t.Fatalf("expected description to be preserved, got %q", rows[0].Description)
	}
	if !rows[0].IsDeleted {
		t.Fatalf("expected is_deleted to be preserved")
	}
	if rows[0].OutputPrice <= 0 {
		t.Fatalf("expected existing output price to be preserved, got %f", rows[0].OutputPrice)
	}
}

func TestBuildProviderModelStoreRows_IncludesPriceComponents(t *testing.T) {
	store := BuildProviderModelStoreRows("openai", []ProviderModelDetail{
		{
			Model:      "dall-e-3",
			Type:       ProviderModelTypeImage,
			InputPrice: 0.04,
			PriceUnit:  ProviderPriceUnitPerImage,
			Currency:   ProviderPriceCurrencyUSD,
			PriceComponents: []ProviderModelPriceComponentDetail{
				{
					Component:  ProviderModelPriceComponentImageGeneration,
					Condition:  "quality=standard;size=1024x1024",
					InputPrice: 0.04,
					PriceUnit:  ProviderPriceUnitPerImage,
					Currency:   ProviderPriceCurrencyUSD,
					Source:     "default",
					SourceURL:  "https://platform.openai.com/docs/pricing",
					SortOrder:  10,
				},
			},
		},
	}, 300)

	if len(store.Models) != 1 {
		t.Fatalf("expected 1 model row, got %d", len(store.Models))
	}
	if len(store.PriceComponents) != 1 {
		t.Fatalf("expected 1 price component row, got %d", len(store.PriceComponents))
	}
	if store.PriceComponents[0].Component != ProviderModelPriceComponentImageGeneration {
		t.Fatalf("unexpected component %q", store.PriceComponents[0].Component)
	}
}
