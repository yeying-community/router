package model

import "testing"

func intPointer(value int) *int {
	return &value
}

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
		{
			name:     "keep volcengine official upstream id",
			provider: "volcengine",
			model:    "doubao-seed-2-0-pro-260215",
			want:     "doubao-seed-2-0-pro-260215",
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
			Status:      ProviderModelStatusDeprecated,
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
			Source:      "migration",
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
	if rows[0].Status != ProviderModelStatusDeprecated {
		t.Fatalf("expected status to be preserved, got %q", rows[0].Status)
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
					Source:     "migration",
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

func TestBuildProviderModelStoreRows_RoundTripsSpecification(t *testing.T) {
	db := newProviderMigrationTestDB(t)
	if err := db.AutoMigrate(&ProviderModel{}, &ProviderModelPriceComponent{}); err != nil {
		t.Fatalf("auto migrate provider model tables: %v", err)
	}
	store := BuildProviderModelStoreRows("zhipu", []ProviderModelDetail{
		{
			Model: "glm-image",
			Type:  ProviderModelTypeImage,
			Tags:  []string{ProviderModelTagImage},
			Specification: &ProviderModelSpecification{
				Version: 1,
				Endpoints: map[string]ProviderModelEndpointSpecification{
					ChannelModelEndpointImages: {
						InputModalities:  []string{"text"},
						OutputModalities: []string{"image"},
						Parameters: map[string]ProviderModelParameterSpecification{
							"size": {
								Type:          "enum",
								AllowedValues: []string{"1920x1920", "1536x2304", "2304x1536"},
							},
						},
						Constraints: &ProviderModelConstraintSpecification{
							MinPixels:           intPointer(3686400),
							MaxPixels:           intPointer(16777216),
							AllowedAspectRatios: []string{"1:1", "2:3", "3:2"},
						},
					},
				},
			},
		},
	}, 300)
	if len(store.Models) != 1 {
		t.Fatalf("expected 1 model row, got %d", len(store.Models))
	}
	if store.Models[0].Specification == "" {
		t.Fatal("expected specification to be serialized into provider_models row")
	}
	if err := db.Create(&store.Models).Error; err != nil {
		t.Fatalf("create provider model rows: %v", err)
	}
	detailsMap, err := LoadProviderModelDetailsMapForProviders(db, []string{"zhipu"})
	if err != nil {
		t.Fatalf("LoadProviderModelDetailsMapForProviders failed: %v", err)
	}
	details := detailsMap["zhipu"]
	if len(details) != 1 {
		t.Fatalf("expected 1 loaded detail, got %d", len(details))
	}
	if details[0].Specification == nil {
		t.Fatal("expected specification to round-trip from database")
	}
	spec := details[0].Specification.Endpoints[ChannelModelEndpointImages]
	if spec.Constraints == nil || spec.Constraints.MinPixels == nil || *spec.Constraints.MinPixels != 3686400 {
		t.Fatalf("min_pixels=%v, want 3686400", spec.Constraints)
	}
	if spec.Constraints.MaxPixels == nil || *spec.Constraints.MaxPixels != 16777216 {
		t.Fatalf("max_pixels=%v, want 16777216", spec.Constraints)
	}
	if len(spec.Parameters["size"].AllowedValues) != 3 {
		t.Fatalf("size allowed_values=%#v, want 3 values", spec.Parameters["size"].AllowedValues)
	}
}

func TestNormalizeProviderModelDetails_UsesTagsAsCapabilityShape(t *testing.T) {
	details := NormalizeProviderModelDetails([]ProviderModelDetail{
		{
			Model: "gpt-5.5",
			Tags:  []string{ProviderModelTagText, ProviderModelTagReasoning, ProviderModelTagToolCalling},
		},
	})
	if len(details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(details))
	}
	if details[0].Type != ProviderModelTypeText {
		t.Fatalf("internal type = %q, want %q", details[0].Type, ProviderModelTypeText)
	}
	want := []string{ProviderModelTagText, ProviderModelTagToolCalling, ProviderModelTagReasoning}
	if len(details[0].Tags) != len(want) {
		t.Fatalf("tags = %#v, want %#v", details[0].Tags, want)
	}
	for i := range want {
		if details[0].Tags[i] != want[i] {
			t.Fatalf("tags = %#v, want %#v", details[0].Tags, want)
		}
	}
	rows := BuildProviderModelStoreRows("openai", details, 300)
	if len(rows.Models) != 1 {
		t.Fatalf("expected 1 model row, got %d", len(rows.Models))
	}
	if rows.Models[0].Tags != "text,tool_calling,reasoning" {
		t.Fatalf("stored tags = %q, want text,tool_calling,reasoning", rows.Models[0].Tags)
	}
}

func TestNormalizeProviderModelTags_DoesNotInventModelShape(t *testing.T) {
	tags := NormalizeProviderModelTags([]string{ProviderModelTagReasoning})
	if len(tags) != 1 || tags[0] != ProviderModelTagReasoning {
		t.Fatalf("tags = %#v, want [reasoning]", tags)
	}
	if got := ProviderModelTypeFromTags(tags); got != "" {
		t.Fatalf("ProviderModelTypeFromTags = %q, want empty", got)
	}
}
