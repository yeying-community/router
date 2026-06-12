package model

import "testing"

func setModelPricingIndexForTest(index providerModelPricingIndex) func() {
	modelPricingIndexLock.Lock()
	previous := modelPricingIndex
	modelPricingIndex = index
	modelPricingIndexLock.Unlock()
	return func() {
		modelPricingIndexLock.Lock()
		modelPricingIndex = previous
		modelPricingIndexLock.Unlock()
	}
}

func TestResolveChannelModelPricingUsesProviderDefaultAndChannelOverride(t *testing.T) {
	restore := setModelPricingIndexForTest(providerModelPricingIndex{
		byProviderAndModel: map[string]providerModelPricingEntry{
			"openai:gpt-4o": {
				Provider: "openai",
				Detail: ProviderModelDetail{
					Model:       "gpt-4o",
					Type:        ProviderModelTypeText,
					InputPrice:  0.005,
					OutputPrice: 0.015,
					PriceUnit:   ProviderPriceUnitPer1KTokens,
					Currency:    ProviderPriceCurrencyUSD,
				},
			},
		},
		byModel: map[string][]providerModelPricingEntry{
			"gpt-4o": {
				{
					Provider: "openai",
					Detail: ProviderModelDetail{
						Model:       "gpt-4o",
						Type:        ProviderModelTypeText,
						InputPrice:  0.005,
						OutputPrice: 0.015,
						PriceUnit:   ProviderPriceUnitPer1KTokens,
						Currency:    ProviderPriceCurrencyUSD,
					},
				},
			},
		},
	})
	defer restore()

	overrideInputPrice := 0.006
	pricing, err := ResolveChannelModelPricing(0, []ChannelModel{
		{
			Model:         "gpt-4o",
			UpstreamModel: "gpt-4o",
			Selected:      true,
			InputPrice:    &overrideInputPrice,
			PriceUnit:     ProviderPriceUnitPer1KTokens,
			Currency:      ProviderPriceCurrencyUSD,
		},
	}, "gpt-4o")
	if err != nil {
		t.Fatalf("ResolveChannelModelPricing returned error: %v", err)
	}
	if pricing.Source != "channel_override" {
		t.Fatalf("expected channel_override source, got %q", pricing.Source)
	}
	if !pricing.HasChannelOverride {
		t.Fatalf("expected HasChannelOverride to be true")
	}
	if pricing.InputPrice != overrideInputPrice {
		t.Fatalf("expected input override %.6f, got %.6f", overrideInputPrice, pricing.InputPrice)
	}
	if !pricing.HasChannelInputPriceOverride {
		t.Fatalf("expected HasChannelInputPriceOverride to be true")
	}
	if pricing.OutputPrice != 0.015 {
		t.Fatalf("expected provider output price 0.015000, got %.6f", pricing.OutputPrice)
	}
	if pricing.Type != ProviderModelTypeText {
		t.Fatalf("expected text type, got %q", pricing.Type)
	}
}

func TestResolveChannelModelPricingUsesVolcengineOfficialModelID(t *testing.T) {
	restore := setModelPricingIndexForTest(providerModelPricingIndex{
		byProviderAndModel: map[string]providerModelPricingEntry{
			"volcengine:doubao-seed-2-0-pro-260215": {
				Provider: "volcengine",
				Detail: ProviderModelDetail{
					Model:       "doubao-seed-2-0-pro-260215",
					Type:        ProviderModelTypeText,
					InputPrice:  0.0032,
					OutputPrice: 0.016,
					PriceUnit:   ProviderPriceUnitPer1KTokens,
					Currency:    "CNY",
				},
			},
		},
		byModel: map[string][]providerModelPricingEntry{
			"doubao-seed-2-0-pro-260215": {
				{
					Provider: "volcengine",
					Detail: ProviderModelDetail{
						Model:       "doubao-seed-2-0-pro-260215",
						Type:        ProviderModelTypeText,
						InputPrice:  0.0032,
						OutputPrice: 0.016,
						PriceUnit:   ProviderPriceUnitPer1KTokens,
						Currency:    "CNY",
					},
				},
			},
		},
	})
	defer restore()

	pricing, err := ResolveChannelModelPricing(0, []ChannelModel{
		{
			Model:         "doubao-seed-2-0-pro-260215",
			UpstreamModel: "doubao-seed-2-0-pro-260215",
			Selected:      true,
			PriceUnit:     ProviderPriceUnitPer1KTokens,
			Currency:      "CNY",
		},
	}, "doubao-seed-2-0-pro-260215")
	if err != nil {
		t.Fatalf("ResolveChannelModelPricing returned error: %v", err)
	}
	if pricing.Provider != "volcengine" {
		t.Fatalf("expected volcengine provider, got %q", pricing.Provider)
	}
	if pricing.InputPrice != 0.0032 || pricing.OutputPrice != 0.016 {
		t.Fatalf("unexpected pricing input=%v output=%v", pricing.InputPrice, pricing.OutputPrice)
	}
}

func TestResolveImageRequestPricingKeepsChannelOverrideAboveProviderComponent(t *testing.T) {
	overrideInputPrice := 0.02
	overrideOutputPrice := 0.05
	pricing := ResolveImageRequestPricing(ResolvedModelPricing{
		Model:                         "gpt-image-2",
		Provider:                      "openai",
		Type:                          ProviderModelTypeImage,
		InputPrice:                    overrideInputPrice,
		OutputPrice:                   overrideOutputPrice,
		PriceUnit:                     ProviderPriceUnitPer1KTokens,
		Currency:                      ProviderPriceCurrencyUSD,
		Source:                        "channel_override",
		HasChannelOverride:            true,
		HasChannelInputPriceOverride:  true,
		HasChannelOutputPriceOverride: true,
		PriceComponents: []ProviderModelPriceComponentDetail{
			{
				Component:   ProviderModelPriceComponentImageGeneration,
				Condition:   "quality=high;size=1024x1024",
				InputPrice:  0.008,
				OutputPrice: 0.03,
				PriceUnit:   ProviderPriceUnitPer1KTokens,
				Currency:    ProviderPriceCurrencyUSD,
			},
		},
	}, "1024x1024", "high")
	if pricing.InputPrice != overrideInputPrice {
		t.Fatalf("expected channel input override %.6f, got %.6f", overrideInputPrice, pricing.InputPrice)
	}
	if pricing.OutputPrice != overrideOutputPrice {
		t.Fatalf("expected channel output override %.6f, got %.6f", overrideOutputPrice, pricing.OutputPrice)
	}
	if pricing.Source != "channel_override" {
		t.Fatalf("expected channel_override source, got %q", pricing.Source)
	}
	if pricing.MatchedComponent != ProviderModelPriceComponentImageGeneration {
		t.Fatalf("expected matched image component, got %q", pricing.MatchedComponent)
	}
}

func TestResolveChannelModelPricingUsesChannelPriceComponents(t *testing.T) {
	restore := setModelPricingIndexForTest(providerModelPricingIndex{
		byProviderAndModel: map[string]providerModelPricingEntry{
			"openai:gpt-image-2": {
				Provider: "openai",
				Detail: ProviderModelDetail{
					Model:       "gpt-image-2",
					Type:        ProviderModelTypeImage,
					InputPrice:  0.008,
					OutputPrice: 0.03,
					PriceUnit:   ProviderPriceUnitPer1KTokens,
					Currency:    ProviderPriceCurrencyUSD,
					PriceComponents: []ProviderModelPriceComponentDetail{
						{Component: ProviderModelPriceComponentText, InputPrice: 0.005, PriceUnit: ProviderPriceUnitPer1KTokens},
						{Component: ProviderModelPriceComponentImageGeneration, OutputPrice: 0.03, PriceUnit: ProviderPriceUnitPer1KTokens},
					},
				},
			},
		},
		byModel: map[string][]providerModelPricingEntry{
			"gpt-image-2": {
				{
					Provider: "openai",
					Detail: ProviderModelDetail{
						Model:       "gpt-image-2",
						Type:        ProviderModelTypeImage,
						InputPrice:  0.008,
						OutputPrice: 0.03,
						PriceUnit:   ProviderPriceUnitPer1KTokens,
						Currency:    ProviderPriceCurrencyUSD,
						PriceComponents: []ProviderModelPriceComponentDetail{
							{Component: ProviderModelPriceComponentText, InputPrice: 0.005, PriceUnit: ProviderPriceUnitPer1KTokens},
							{Component: ProviderModelPriceComponentImageGeneration, OutputPrice: 0.03, PriceUnit: ProviderPriceUnitPer1KTokens},
						},
					},
				},
			},
		},
	})
	defer restore()

	pricing, err := ResolveChannelModelPricing(0, []ChannelModel{
		{
			Model:         "gpt-image-2",
			UpstreamModel: "gpt-image-2",
			Selected:      true,
			PriceComponents: []ProviderModelPriceComponentDetail{
				{Component: ProviderModelPriceComponentText, InputPrice: 0.006, PriceUnit: ProviderPriceUnitPer1KTokens},
				{Component: ProviderModelPriceComponentImageGeneration, OutputPrice: 0.05, PriceUnit: ProviderPriceUnitPer1KTokens},
			},
		},
	}, "gpt-image-2")
	if err != nil {
		t.Fatalf("ResolveChannelModelPricing returned error: %v", err)
	}
	if !pricing.HasChannelComponentOverride {
		t.Fatal("expected channel component override")
	}
	text, ok := selectProviderPriceComponent(pricing.PriceComponents, ProviderModelPriceComponentText, nil)
	if !ok || text.InputPrice != 0.006 {
		t.Fatalf("expected text component input override 0.006, got %#v", text)
	}
	image, ok := selectProviderPriceComponent(pricing.PriceComponents, ProviderModelPriceComponentImageGeneration, nil)
	if !ok || image.OutputPrice != 0.05 {
		t.Fatalf("expected image component output override 0.05, got %#v", image)
	}
	if text.Source != "channel_override" || image.Source != "channel_override" {
		t.Fatalf("expected channel_override sources, got text=%q image=%q", text.Source, image.Source)
	}
}

func TestResolveChannelModelPricingRequiresPositivePrice(t *testing.T) {
	restore := setModelPricingIndexForTest(providerModelPricingIndex{
		byProviderAndModel: map[string]providerModelPricingEntry{},
		byModel:            map[string][]providerModelPricingEntry{},
	})
	defer restore()

	zero := 0.0
	_, err := ResolveChannelModelPricing(0, []ChannelModel{
		{
			Model:         "missing-model",
			UpstreamModel: "missing-model",
			Selected:      true,
			InputPrice:    &zero,
			OutputPrice:   &zero,
			PriceUnit:     ProviderPriceUnitPer1KTokens,
			Currency:      ProviderPriceCurrencyUSD,
		},
	}, "missing-model")
	if err == nil {
		t.Fatalf("expected error when neither provider default nor positive channel override exists")
	}
}

func TestResolveChannelModelPricingCarriesPriceComponents(t *testing.T) {
	restore := setModelPricingIndexForTest(providerModelPricingIndex{
		byProviderAndModel: map[string]providerModelPricingEntry{
			"openai:dall-e-3": {
				Provider: "openai",
				Detail: ProviderModelDetail{
					Model:      "dall-e-3",
					Type:       ProviderModelTypeImage,
					InputPrice: 0.04,
					PriceUnit:  ProviderPriceUnitPerImage,
					Currency:   ProviderPriceCurrencyUSD,
					PriceComponents: []ProviderModelPriceComponentDetail{
						{
							Component:  ProviderModelPriceComponentImageGeneration,
							Condition:  "quality=hd;size=1024x1024",
							InputPrice: 0.08,
							PriceUnit:  ProviderPriceUnitPerImage,
							Currency:   ProviderPriceCurrencyUSD,
							Source:     "migration",
						},
					},
				},
			},
		},
		byModel: map[string][]providerModelPricingEntry{
			"dall-e-3": {
				{
					Provider: "openai",
					Detail: ProviderModelDetail{
						Model:      "dall-e-3",
						Type:       ProviderModelTypeImage,
						InputPrice: 0.04,
						PriceUnit:  ProviderPriceUnitPerImage,
						Currency:   ProviderPriceCurrencyUSD,
						PriceComponents: []ProviderModelPriceComponentDetail{
							{
								Component:  ProviderModelPriceComponentImageGeneration,
								Condition:  "quality=hd;size=1024x1024",
								InputPrice: 0.08,
								PriceUnit:  ProviderPriceUnitPerImage,
								Currency:   ProviderPriceCurrencyUSD,
								Source:     "migration",
							},
						},
					},
				},
			},
		},
	})
	defer restore()

	pricing, err := ResolveChannelModelPricing(0, nil, "dall-e-3")
	if err != nil {
		t.Fatalf("ResolveChannelModelPricing returned error: %v", err)
	}
	if len(pricing.PriceComponents) != 1 {
		t.Fatalf("expected 1 price component, got %d", len(pricing.PriceComponents))
	}
	if pricing.PriceComponents[0].Condition != "quality=hd;size=1024x1024" {
		t.Fatalf("unexpected component condition %q", pricing.PriceComponents[0].Condition)
	}
}

func TestBuildModelPricingIndexFromProviderDetailsMapCarriesPriceComponents(t *testing.T) {
	index := buildModelPricingIndexFromProviderDetailsMap(map[string][]ProviderModelDetail{
		"openai": {
			{
				Model:       "gpt-image-2",
				Type:        ProviderModelTypeImage,
				InputPrice:  0.008,
				OutputPrice: 0.03,
				PriceUnit:   ProviderPriceUnitPer1KTokens,
				Currency:    ProviderPriceCurrencyUSD,
				PriceComponents: []ProviderModelPriceComponentDetail{
					{
						Component:  ProviderModelPriceComponentText,
						InputPrice: 0.005,
						PriceUnit:  ProviderPriceUnitPer1KTokens,
						Currency:   ProviderPriceCurrencyUSD,
					},
					{
						Component:   ProviderModelPriceComponentImageGeneration,
						OutputPrice: 0.03,
						PriceUnit:   ProviderPriceUnitPer1KTokens,
						Currency:    ProviderPriceCurrencyUSD,
					},
				},
			},
		},
	})
	entry, ok := index.byProviderAndModel["openai:gpt-image-2"]
	if !ok {
		t.Fatal("expected openai:gpt-image-2 entry")
	}
	if len(entry.Detail.PriceComponents) != 2 {
		t.Fatalf("expected 2 price components, got %d", len(entry.Detail.PriceComponents))
	}
	seen := make(map[string]bool, len(entry.Detail.PriceComponents))
	for _, component := range entry.Detail.PriceComponents {
		seen[component.Component] = true
	}
	if !seen[ProviderModelPriceComponentText] || !seen[ProviderModelPriceComponentImageGeneration] {
		t.Fatalf("expected text and image_generation components, got %#v", entry.Detail.PriceComponents)
	}
}

func TestResolveImageRequestPricingMatchesComponent(t *testing.T) {
	pricing := ResolveImageRequestPricing(ResolvedModelPricing{
		Model:      "dall-e-3",
		Provider:   "openai",
		Type:       ProviderModelTypeImage,
		InputPrice: 0.04,
		PriceUnit:  ProviderPriceUnitPerImage,
		Currency:   ProviderPriceCurrencyUSD,
		Source:     "provider_migration",
		PriceComponents: []ProviderModelPriceComponentDetail{
			{
				Component:  ProviderModelPriceComponentImageGeneration,
				Condition:  "quality=hd;size=1024x1024",
				InputPrice: 0.08,
				PriceUnit:  ProviderPriceUnitPerImage,
				Currency:   ProviderPriceCurrencyUSD,
				Source:     "migration",
			},
		},
	}, "1024x1024", "hd")

	if pricing.Source != "provider_component" {
		t.Fatalf("expected source provider_component, got %q", pricing.Source)
	}
	if pricing.InputPrice != 0.08 {
		t.Fatalf("expected input price 0.08, got %f", pricing.InputPrice)
	}
	if pricing.MatchedCondition != "quality=hd;size=1024x1024" {
		t.Fatalf("unexpected matched condition %q", pricing.MatchedCondition)
	}
}

func TestResolveImageRequestPricingFallsBackWhenNoComponentMatches(t *testing.T) {
	pricing := ResolveImageRequestPricing(ResolvedModelPricing{
		Model:      "dall-e-3",
		Provider:   "openai",
		Type:       ProviderModelTypeImage,
		InputPrice: 0.04,
		PriceUnit:  ProviderPriceUnitPerImage,
		Currency:   ProviderPriceCurrencyUSD,
		Source:     "provider_migration",
		PriceComponents: []ProviderModelPriceComponentDetail{
			{
				Component:  ProviderModelPriceComponentImageGeneration,
				Condition:  "quality=hd;size=1024x1024",
				InputPrice: 0.08,
				PriceUnit:  ProviderPriceUnitPerImage,
				Currency:   ProviderPriceCurrencyUSD,
				Source:     "migration",
			},
		},
	}, "1792x1024", "standard")

	if pricing.Source != "provider_migration" {
		t.Fatalf("expected source provider_migration, got %q", pricing.Source)
	}
	if pricing.InputPrice != 0.04 {
		t.Fatalf("expected base input price 0.04, got %f", pricing.InputPrice)
	}
	if pricing.MatchedCondition != "" {
		t.Fatalf("expected empty matched condition, got %q", pricing.MatchedCondition)
	}
}

func TestResolveTextRequestPricingMatchesEndpointComponent(t *testing.T) {
	pricing := ResolveTextRequestPricing(ResolvedModelPricing{
		Model:       "gpt-5",
		Provider:    "openai",
		Type:        ProviderModelTypeText,
		InputPrice:  0.001,
		OutputPrice: 0.002,
		PriceUnit:   ProviderPriceUnitPer1KTokens,
		Currency:    ProviderPriceCurrencyUSD,
		Source:      "provider_migration",
		PriceComponents: []ProviderModelPriceComponentDetail{
			{
				Component:   ProviderModelPriceComponentText,
				Condition:   "endpoint=/v1/responses",
				InputPrice:  0.003,
				OutputPrice: 0.004,
				PriceUnit:   ProviderPriceUnitPer1KTokens,
				Currency:    ProviderPriceCurrencyUSD,
				Source:      "official",
			},
		},
	}, "/v1/responses")

	if pricing.Source != "provider_component" {
		t.Fatalf("expected source provider_component, got %q", pricing.Source)
	}
	if pricing.MatchedComponent != ProviderModelPriceComponentText {
		t.Fatalf("unexpected matched component %q", pricing.MatchedComponent)
	}
	if pricing.MatchedCondition != "endpoint=/v1/responses" {
		t.Fatalf("unexpected matched condition %q", pricing.MatchedCondition)
	}
	if pricing.InputPrice != 0.003 || pricing.OutputPrice != 0.004 {
		t.Fatalf("unexpected matched prices input=%f output=%f", pricing.InputPrice, pricing.OutputPrice)
	}
}

func TestResolveTextRequestPricingFallsBackWhenNoEndpointMatch(t *testing.T) {
	pricing := ResolveTextRequestPricing(ResolvedModelPricing{
		Model:       "gpt-5",
		Provider:    "openai",
		Type:        ProviderModelTypeText,
		InputPrice:  0.001,
		OutputPrice: 0.002,
		PriceUnit:   ProviderPriceUnitPer1KTokens,
		Currency:    ProviderPriceCurrencyUSD,
		Source:      "provider_migration",
		PriceComponents: []ProviderModelPriceComponentDetail{
			{
				Component:   ProviderModelPriceComponentText,
				Condition:   "endpoint=/v1/responses",
				InputPrice:  0.003,
				OutputPrice: 0.004,
				PriceUnit:   ProviderPriceUnitPer1KTokens,
				Currency:    ProviderPriceCurrencyUSD,
				Source:      "official",
			},
		},
	}, "/v1/chat/completions")

	if pricing.Source != "provider_migration" {
		t.Fatalf("expected source provider_migration, got %q", pricing.Source)
	}
	if pricing.MatchedCondition != "" || pricing.MatchedComponent != "" {
		t.Fatalf("expected no matched component, got component=%q condition=%q", pricing.MatchedComponent, pricing.MatchedCondition)
	}
	if pricing.InputPrice != 0.001 || pricing.OutputPrice != 0.002 {
		t.Fatalf("unexpected fallback prices input=%f output=%f", pricing.InputPrice, pricing.OutputPrice)
	}
}

func TestResolveAudioRequestPricingMatchesAudioOutputComponent(t *testing.T) {
	pricing := ResolveAudioRequestPricing(ResolvedModelPricing{
		Model:      "gpt-4o-mini-tts",
		Provider:   "openai",
		Type:       ProviderModelTypeAudio,
		InputPrice: 0.015,
		PriceUnit:  ProviderPriceUnitPer1KChars,
		Currency:   ProviderPriceCurrencyUSD,
		Source:     "provider_migration",
		PriceComponents: []ProviderModelPriceComponentDetail{
			{
				Component:  ProviderModelPriceComponentAudioOutput,
				InputPrice: 0.03,
				PriceUnit:  ProviderPriceUnitPer1KChars,
				Currency:   ProviderPriceCurrencyUSD,
				Source:     "official",
			},
		},
	}, true)

	if pricing.Source != "provider_component" {
		t.Fatalf("expected source provider_component, got %q", pricing.Source)
	}
	if pricing.MatchedComponent != ProviderModelPriceComponentAudioOutput {
		t.Fatalf("unexpected matched component %q", pricing.MatchedComponent)
	}
	if pricing.InputPrice != 0.03 {
		t.Fatalf("expected input price 0.03, got %f", pricing.InputPrice)
	}
}

func TestResolveAudioRequestPricingMatchesAudioInputComponent(t *testing.T) {
	pricing := ResolveAudioRequestPricing(ResolvedModelPricing{
		Model:      "whisper-1",
		Provider:   "openai",
		Type:       ProviderModelTypeAudio,
		InputPrice: 0.006,
		PriceUnit:  ProviderPriceUnitPerMinute,
		Currency:   ProviderPriceCurrencyUSD,
		Source:     "provider_migration",
		PriceComponents: []ProviderModelPriceComponentDetail{
			{
				Component:  ProviderModelPriceComponentAudioInput,
				InputPrice: 0.01,
				PriceUnit:  ProviderPriceUnitPerMinute,
				Currency:   ProviderPriceCurrencyUSD,
				Source:     "official",
			},
		},
	}, false)

	if pricing.Source != "provider_component" {
		t.Fatalf("expected source provider_component, got %q", pricing.Source)
	}
	if pricing.MatchedComponent != ProviderModelPriceComponentAudioInput {
		t.Fatalf("unexpected matched component %q", pricing.MatchedComponent)
	}
	if pricing.InputPrice != 0.01 {
		t.Fatalf("expected input price 0.01, got %f", pricing.InputPrice)
	}
}

func TestResolveVideoRequestPricingMatchesComponent(t *testing.T) {
	pricing := ResolveVideoRequestPricing(ResolvedModelPricing{
		Model:      "veo-3.0-generate-preview",
		Provider:   "google",
		Type:       ProviderModelTypeVideo,
		InputPrice: 0.5,
		PriceUnit:  ProviderPriceUnitPerSecond,
		Currency:   ProviderPriceCurrencyUSD,
		Source:     "provider_migration",
		PriceComponents: []ProviderModelPriceComponentDetail{
			{
				Component:  ProviderModelPriceComponentVideoGeneration,
				Condition:  "resolution=720p",
				InputPrice: 0.4,
				PriceUnit:  ProviderPriceUnitPerSecond,
				Currency:   ProviderPriceCurrencyUSD,
				Source:     "official",
			},
		},
	}, map[string]string{
		"resolution": "720p",
	})

	if pricing.Source != "provider_component" {
		t.Fatalf("expected source provider_component, got %q", pricing.Source)
	}
	if pricing.MatchedComponent != ProviderModelPriceComponentVideoGeneration {
		t.Fatalf("unexpected matched component %q", pricing.MatchedComponent)
	}
	if pricing.MatchedCondition != "resolution=720p" {
		t.Fatalf("unexpected matched condition %q", pricing.MatchedCondition)
	}
	if pricing.InputPrice != 0.4 {
		t.Fatalf("expected input price 0.4, got %f", pricing.InputPrice)
	}
}

func TestResolveVideoRequestPricingFallsBackWhenNoComponentMatches(t *testing.T) {
	pricing := ResolveVideoRequestPricing(ResolvedModelPricing{
		Model:      "veo-3.0-generate-preview",
		Provider:   "google",
		Type:       ProviderModelTypeVideo,
		InputPrice: 0.5,
		PriceUnit:  ProviderPriceUnitPerSecond,
		Currency:   ProviderPriceCurrencyUSD,
		Source:     "provider_migration",
		PriceComponents: []ProviderModelPriceComponentDetail{
			{
				Component:  ProviderModelPriceComponentVideoGeneration,
				Condition:  "resolution=720p",
				InputPrice: 0.4,
				PriceUnit:  ProviderPriceUnitPerSecond,
				Currency:   ProviderPriceCurrencyUSD,
				Source:     "official",
			},
		},
	}, map[string]string{
		"resolution": "1080p",
	})

	if pricing.Source != "provider_migration" {
		t.Fatalf("expected source provider_migration, got %q", pricing.Source)
	}
	if pricing.MatchedComponent != "" || pricing.MatchedCondition != "" {
		t.Fatalf("expected no matched component, got component=%q condition=%q", pricing.MatchedComponent, pricing.MatchedCondition)
	}
	if pricing.InputPrice != 0.5 {
		t.Fatalf("expected input price 0.5, got %f", pricing.InputPrice)
	}
}
