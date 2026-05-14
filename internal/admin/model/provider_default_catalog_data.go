package model

import (
	"sort"
	"strings"
)

var defaultProviderCatalogTemplates = normalizeDefaultProviderCatalogTemplates([]ProviderCatalogSeed{
	{
		Provider:    "openai",
		Name:        "OpenAI",
		BaseURL:     "https://api.openai.com",
		OfficialURL: "https://platform.openai.com/docs/pricing",
		SortOrder:   100,
		ModelDetails: []ProviderModelDetail{
			{Model: "gpt-5", Type: ProviderModelTypeText, InputPrice: 0.00125, OutputPrice: 0.01, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5.1", Type: ProviderModelTypeText, InputPrice: 0.00125, OutputPrice: 0.01, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5.1-codex", Type: ProviderModelTypeText, InputPrice: 0.00125, OutputPrice: 0.01, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5.1-codex-max", Type: ProviderModelTypeText, InputPrice: 0.00125, OutputPrice: 0.01, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5.1-codex-mini", Type: ProviderModelTypeText, InputPrice: 0.00025, OutputPrice: 0.002, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5.2", Type: ProviderModelTypeText, InputPrice: 0.00175, OutputPrice: 0.014, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5.2-codex", Type: ProviderModelTypeText, InputPrice: 0.00175, OutputPrice: 0.014, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5.3-codex", Type: ProviderModelTypeText, InputPrice: 0.00175, OutputPrice: 0.014, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5.4", Type: ProviderModelTypeText, InputPrice: 0.0025, OutputPrice: 0.015, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5.4-mini", Type: ProviderModelTypeText, InputPrice: 0.00075, OutputPrice: 0.0045, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5.4-nano", Type: ProviderModelTypeText, InputPrice: 0.0001, OutputPrice: 0.000625, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5.4-pro", Type: ProviderModelTypeText, InputPrice: 0.015, OutputPrice: 0.09, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5.5", Type: ProviderModelTypeText, InputPrice: 0.005, OutputPrice: 0.03, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5.5-pro", Type: ProviderModelTypeText, InputPrice: 0.015, OutputPrice: 0.09, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5-mini", Type: ProviderModelTypeText, InputPrice: 0.00025, OutputPrice: 0.002, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5-nano", Type: ProviderModelTypeText, InputPrice: 0.00005, OutputPrice: 0.0004, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5-pro", Type: ProviderModelTypeText, InputPrice: 0.015, OutputPrice: 0.12, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-5-codex", Type: ProviderModelTypeText, InputPrice: 0.00125, OutputPrice: 0.01, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "codex-mini-latest", Type: ProviderModelTypeText, InputPrice: 0.0015, OutputPrice: 0.006, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-4.1", Type: ProviderModelTypeText, InputPrice: 0.002, OutputPrice: 0.008, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-4.1-mini", Type: ProviderModelTypeText, InputPrice: 0.0004, OutputPrice: 0.0016, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-4.1-nano", Type: ProviderModelTypeText, InputPrice: 0.0001, OutputPrice: 0.0004, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-4o", Type: ProviderModelTypeText, InputPrice: 0.0025, OutputPrice: 0.01, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-4o-mini", Type: ProviderModelTypeText, InputPrice: 0.00015, OutputPrice: 0.0006, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "o3", Type: ProviderModelTypeText, InputPrice: 0.002, OutputPrice: 0.008, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "o4-mini", Type: ProviderModelTypeText, InputPrice: 0.0011, OutputPrice: 0.0044, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{
				Model:              "gpt-image-2",
				Type:               ProviderModelTypeImage,
				SupportedEndpoints: []string{ChannelModelEndpointResponses},
				InputPrice:         0.008,
				OutputPrice:        0.03,
				PriceUnit:          ProviderPriceUnitPer1KTokens,
				Currency:           ProviderPriceCurrencyUSD,
				Source:             "default",
				PriceComponents: []ProviderModelPriceComponentDetail{
					{Component: ProviderModelPriceComponentText, InputPrice: 0.005, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://openai.com/api/pricing/", SortOrder: 10},
					{Component: ProviderModelPriceComponentImageGeneration, InputPrice: 0.008, OutputPrice: 0.03, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://openai.com/api/pricing/", SortOrder: 20},
				},
			},
			{
				Model:              "gpt-image-1",
				Type:               ProviderModelTypeImage,
				SupportedEndpoints: []string{ChannelModelEndpointResponses, ChannelModelEndpointImages, ChannelModelEndpointImageEdit},
				InputPrice:         0.011,
				PriceUnit:          ProviderPriceUnitPerImage,
				Currency:           ProviderPriceCurrencyUSD,
				Source:             "default",
				PriceComponents: []ProviderModelPriceComponentDetail{
					{Component: ProviderModelPriceComponentImageGeneration, Condition: "quality=low;size=1024x1024", InputPrice: 0.011, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://platform.openai.com/docs/pricing", SortOrder: 10},
					{Component: ProviderModelPriceComponentImageGeneration, Condition: "quality=low;size=1024x1536", InputPrice: 0.016, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://platform.openai.com/docs/pricing", SortOrder: 20},
					{Component: ProviderModelPriceComponentImageGeneration, Condition: "quality=low;size=1536x1024", InputPrice: 0.016, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://platform.openai.com/docs/pricing", SortOrder: 30},
					{Component: ProviderModelPriceComponentImageGeneration, Condition: "quality=medium;size=1024x1024", InputPrice: 0.042, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://platform.openai.com/docs/pricing", SortOrder: 40},
					{Component: ProviderModelPriceComponentImageGeneration, Condition: "quality=medium;size=1024x1536", InputPrice: 0.063, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://platform.openai.com/docs/pricing", SortOrder: 50},
					{Component: ProviderModelPriceComponentImageGeneration, Condition: "quality=medium;size=1536x1024", InputPrice: 0.063, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://platform.openai.com/docs/pricing", SortOrder: 60},
					{Component: ProviderModelPriceComponentImageGeneration, Condition: "quality=high;size=1024x1024", InputPrice: 0.167, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://platform.openai.com/docs/pricing", SortOrder: 70},
					{Component: ProviderModelPriceComponentImageGeneration, Condition: "quality=high;size=1024x1536", InputPrice: 0.25, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://platform.openai.com/docs/pricing", SortOrder: 80},
					{Component: ProviderModelPriceComponentImageGeneration, Condition: "quality=high;size=1536x1024", InputPrice: 0.25, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://platform.openai.com/docs/pricing", SortOrder: 90},
				},
			},
			{
				Model:              "gpt-image-1.5",
				Type:               ProviderModelTypeImage,
				SupportedEndpoints: []string{ChannelModelEndpointResponses, ChannelModelEndpointImages, ChannelModelEndpointImageEdit},
				InputPrice:         0.008,
				OutputPrice:        0.032,
				PriceUnit:          ProviderPriceUnitPer1KTokens,
				Currency:           ProviderPriceCurrencyUSD,
				Source:             "default",
				PriceComponents: []ProviderModelPriceComponentDetail{
					{Component: ProviderModelPriceComponentText, InputPrice: 0.005, OutputPrice: 0.01, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://openai.com/api/pricing/", SortOrder: 10},
					{Component: ProviderModelPriceComponentImageGeneration, InputPrice: 0.008, OutputPrice: 0.032, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://openai.com/api/pricing/", SortOrder: 20},
				},
			},
			{
				Model:              "gpt-image-1-mini",
				Type:               ProviderModelTypeImage,
				SupportedEndpoints: []string{ChannelModelEndpointResponses, ChannelModelEndpointImages, ChannelModelEndpointImageEdit},
				InputPrice:         0.0025,
				OutputPrice:        0.008,
				PriceUnit:          ProviderPriceUnitPer1KTokens,
				Currency:           ProviderPriceCurrencyUSD,
				Source:             "default",
				PriceComponents: []ProviderModelPriceComponentDetail{
					{Component: ProviderModelPriceComponentText, InputPrice: 0.002, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://openai.com/api/pricing/", SortOrder: 10},
					{Component: ProviderModelPriceComponentImageGeneration, InputPrice: 0.0025, OutputPrice: 0.008, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://openai.com/api/pricing/", SortOrder: 20},
				},
			},
			{
				Model:      "dall-e-3",
				Type:       ProviderModelTypeImage,
				InputPrice: 0.04,
				PriceUnit:  ProviderPriceUnitPerImage,
				Currency:   ProviderPriceCurrencyUSD,
				Source:     "default",
				PriceComponents: []ProviderModelPriceComponentDetail{
					{Component: ProviderModelPriceComponentImageGeneration, Condition: "quality=standard;size=1024x1024", InputPrice: 0.04, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://platform.openai.com/docs/pricing", SortOrder: 10},
					{Component: ProviderModelPriceComponentImageGeneration, Condition: "quality=standard;size=1024x1792", InputPrice: 0.08, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://platform.openai.com/docs/pricing", SortOrder: 20},
					{Component: ProviderModelPriceComponentImageGeneration, Condition: "quality=standard;size=1792x1024", InputPrice: 0.08, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://platform.openai.com/docs/pricing", SortOrder: 30},
					{Component: ProviderModelPriceComponentImageGeneration, Condition: "quality=hd;size=1024x1024", InputPrice: 0.08, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://platform.openai.com/docs/pricing", SortOrder: 40},
					{Component: ProviderModelPriceComponentImageGeneration, Condition: "quality=hd;size=1024x1792", InputPrice: 0.12, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://platform.openai.com/docs/pricing", SortOrder: 50},
					{Component: ProviderModelPriceComponentImageGeneration, Condition: "quality=hd;size=1792x1024", InputPrice: 0.12, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://platform.openai.com/docs/pricing", SortOrder: 60},
				},
			},
			{Model: "gpt-realtime", Type: ProviderModelTypeAudio, InputPrice: 0.004, OutputPrice: 0.016, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-realtime-2", Type: ProviderModelTypeAudio, InputPrice: 0.004, OutputPrice: 0.024, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-realtime-1.5", Type: ProviderModelTypeAudio, InputPrice: 0.0006, OutputPrice: 0.0024, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-realtime-mini", Type: ProviderModelTypeAudio, InputPrice: 0.0006, OutputPrice: 0.0024, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-realtime-translate", Type: ProviderModelTypeAudio, InputPrice: 0.0015, OutputPrice: 0.006, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-4o-mini-tts", Type: ProviderModelTypeAudio, InputPrice: 0.0006, PriceUnit: ProviderPriceUnitPer1KChars, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-audio", Type: ProviderModelTypeAudio, InputPrice: 0.032, OutputPrice: 0.064, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "gpt-audio-mini", Type: ProviderModelTypeAudio, InputPrice: 0.01, OutputPrice: 0.02, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "whisper-1", Type: ProviderModelTypeAudio, InputPrice: 0.006, PriceUnit: ProviderPriceUnitPerMinute, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "tts-1", Type: ProviderModelTypeAudio, InputPrice: 0.015, PriceUnit: ProviderPriceUnitPer1KChars, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "tts-1-hd", Type: ProviderModelTypeAudio, InputPrice: 0.03, PriceUnit: ProviderPriceUnitPer1KChars, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "sora-2", Type: ProviderModelTypeVideo, InputPrice: 0.1, PriceUnit: ProviderPriceUnitPerSecond, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "sora-2-pro", Type: ProviderModelTypeVideo, InputPrice: 0.3, PriceUnit: ProviderPriceUnitPerSecond, Currency: ProviderPriceCurrencyUSD, Source: "default"},
		},
	},
	{
		Provider:    "baidu",
		Name:        "Baidu",
		BaseURL:     "https://qianfan.baidubce.com/v2",
		OfficialURL: "https://cloud.baidu.com/product-s/qianfan_modelbuilder",
		SortOrder:   105,
		ModelDetails: []ProviderModelDetail{
			{Model: "ernie-4.5-turbo-128k", Type: ProviderModelTypeText, InputPrice: 0.0008, OutputPrice: 0.0032, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "ernie-x1.1-preview", Type: ProviderModelTypeText, InputPrice: 0.001, OutputPrice: 0.004, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "ernie-4.5-vl-32k-preview", Type: ProviderModelTypeImage, SupportedEndpoints: []string{ChannelModelEndpointResponses}, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
		},
	},
	{
		Provider:    "anthropic",
		Name:        "Anthropic",
		BaseURL:     "https://api.anthropic.com",
		OfficialURL: "https://platform.claude.com/docs/en/about-claude/pricing",
		SortOrder:   110,
		ModelDetails: []ProviderModelDetail{
			{Model: "claude-opus-4-7", Type: ProviderModelTypeText, InputPrice: 0.005, OutputPrice: 0.025, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "claude-opus-4-6", Type: ProviderModelTypeText, InputPrice: 0.005, OutputPrice: 0.025, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "claude-opus-4-6-thinking", Type: ProviderModelTypeText, InputPrice: 0.005, OutputPrice: 0.025, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "claude-sonnet-4-6", Type: ProviderModelTypeText, InputPrice: 0.003, OutputPrice: 0.015, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "claude-opus-4-5", Type: ProviderModelTypeText, InputPrice: 0.005, OutputPrice: 0.025, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "claude-opus-4-5-20251101", Type: ProviderModelTypeText, InputPrice: 0.005, OutputPrice: 0.025, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "claude-opus-4-1", Type: ProviderModelTypeText, InputPrice: 0.015, OutputPrice: 0.075, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "claude-opus-4-1-20250805", Type: ProviderModelTypeText, InputPrice: 0.015, OutputPrice: 0.075, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "claude-sonnet-4-5", Type: ProviderModelTypeText, InputPrice: 0.003, OutputPrice: 0.015, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "claude-sonnet-4-5-20250929", Type: ProviderModelTypeText, InputPrice: 0.003, OutputPrice: 0.015, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "claude-haiku-4-5", Type: ProviderModelTypeText, InputPrice: 0.001, OutputPrice: 0.005, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "claude-haiku-4-5-20251001", Type: ProviderModelTypeText, InputPrice: 0.001, OutputPrice: 0.005, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "claude-3-5-haiku-20241022", Type: ProviderModelTypeText, InputPrice: 0.0008, OutputPrice: 0.004, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
		},
	},
	{
		Provider:    "deepseek",
		Name:        "DeepSeek",
		BaseURL:     "https://api.deepseek.com",
		OfficialURL: "https://api-docs.deepseek.com/quick_start/pricing/",
		SortOrder:   115,
		ModelDetails: []ProviderModelDetail{
			{Model: "deepseek-chat", Type: ProviderModelTypeText, InputPrice: 0.00028, OutputPrice: 0.00042, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "deepseek-reasoner", Type: ProviderModelTypeText, InputPrice: 0.00028, OutputPrice: 0.00042, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "deepseek-v3.1", Type: ProviderModelTypeText, InputPrice: 0.00056, OutputPrice: 0.00168, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
		},
	},
	{
		Provider:    "google",
		Name:        "Google",
		BaseURL:     "https://generativelanguage.googleapis.com/v1beta/openai",
		OfficialURL: "https://ai.google.dev/gemini-api/docs/pricing",
		SortOrder:   120,
		ModelDetails: []ProviderModelDetail{
			{
				Model:       "gemini-2.5-pro",
				Type:        ProviderModelTypeText,
				InputPrice:  0.00125,
				OutputPrice: 0.01,
				PriceUnit:   ProviderPriceUnitPer1KTokens,
				Currency:    ProviderPriceCurrencyUSD,
				Source:      "default",
				PriceComponents: []ProviderModelPriceComponentDetail{
					{Component: ProviderModelPriceComponentText, Condition: "mode=standard;prompt_tokens_lte=200000", InputPrice: 0.00125, OutputPrice: 0.01, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 10},
					{Component: ProviderModelPriceComponentText, Condition: "mode=standard;prompt_tokens_gt=200000", InputPrice: 0.0025, OutputPrice: 0.015, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 20},
					{Component: ProviderModelPriceComponentText, Condition: "mode=batch;prompt_tokens_lte=200000", InputPrice: 0.000625, OutputPrice: 0.005, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 30},
					{Component: ProviderModelPriceComponentText, Condition: "mode=batch;prompt_tokens_gt=200000", InputPrice: 0.00125, OutputPrice: 0.0075, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 40},
					{Component: "context_cache", Condition: "mode=standard;prompt_tokens_lte=200000", InputPrice: 0.000125, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 50},
					{Component: "context_cache", Condition: "mode=standard;prompt_tokens_gt=200000", InputPrice: 0.00025, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 60},
				},
			},
			{
				Model:       "gemini-2.5-flash",
				Type:        ProviderModelTypeText,
				InputPrice:  0.0003,
				OutputPrice: 0.0025,
				PriceUnit:   ProviderPriceUnitPer1KTokens,
				Currency:    ProviderPriceCurrencyUSD,
				Source:      "default",
				PriceComponents: []ProviderModelPriceComponentDetail{
					{Component: ProviderModelPriceComponentText, Condition: "mode=standard;input_type=text_image_video", InputPrice: 0.0003, OutputPrice: 0.0025, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 10},
					{Component: ProviderModelPriceComponentAudioInput, Condition: "mode=standard", InputPrice: 0.001, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 20},
					{Component: "context_cache", Condition: "mode=standard;input_type=text_image_video", InputPrice: 0.00003, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 30},
					{Component: "context_cache", Condition: "mode=standard;input_type=audio", InputPrice: 0.0001, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 40},
					{Component: "context_cache_storage", Condition: "mode=standard", InputPrice: 1, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 50},
					{Component: ProviderModelPriceComponentText, Condition: "mode=batch;input_type=text_image_video", InputPrice: 0.00015, OutputPrice: 0.00125, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 60},
					{Component: ProviderModelPriceComponentAudioInput, Condition: "mode=batch", InputPrice: 0.0005, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 70},
					{Component: "context_cache", Condition: "mode=batch;input_type=text_image_video", InputPrice: 0.00003, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 80},
					{Component: "context_cache", Condition: "mode=batch;input_type=audio", InputPrice: 0.0001, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 90},
					{Component: "context_cache_storage", Condition: "mode=batch", InputPrice: 1, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 100},
				},
			},
			{
				Model:       "gemini-2.5-flash-lite",
				Type:        ProviderModelTypeText,
				InputPrice:  0.0001,
				OutputPrice: 0.0004,
				PriceUnit:   ProviderPriceUnitPer1KTokens,
				Currency:    ProviderPriceCurrencyUSD,
				Source:      "default",
				PriceComponents: []ProviderModelPriceComponentDetail{
					{Component: ProviderModelPriceComponentText, Condition: "mode=standard;input_type=text_image_video", InputPrice: 0.0001, OutputPrice: 0.0004, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 10},
					{Component: ProviderModelPriceComponentAudioInput, Condition: "mode=standard", InputPrice: 0.0003, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 20},
					{Component: "context_cache", Condition: "mode=standard;input_type=text_image_video", InputPrice: 0.00001, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 30},
					{Component: "context_cache", Condition: "mode=standard;input_type=audio", InputPrice: 0.00003, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 40},
					{Component: "context_cache_storage", Condition: "mode=standard", InputPrice: 1, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 50},
					{Component: ProviderModelPriceComponentText, Condition: "mode=batch;input_type=text_image_video", InputPrice: 0.00005, OutputPrice: 0.0002, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 60},
					{Component: ProviderModelPriceComponentAudioInput, Condition: "mode=batch", InputPrice: 0.00015, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 70},
					{Component: "context_cache", Condition: "mode=batch;input_type=text_image_video", InputPrice: 0.00001, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 80},
					{Component: "context_cache", Condition: "mode=batch;input_type=audio", InputPrice: 0.00003, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 90},
					{Component: "context_cache_storage", Condition: "mode=batch", InputPrice: 1, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/gemini-api/docs/pricing", SortOrder: 100},
				},
			},
			{
				Model:     "gemini-live-2.5-flash-preview",
				Type:      ProviderModelTypeAudio,
				PriceUnit: ProviderPriceUnitPer1KTokens,
				Currency:  ProviderPriceCurrencyUSD,
				Source:    "default",
				PriceComponents: []ProviderModelPriceComponentDetail{
					{Component: ProviderModelPriceComponentText, Condition: "input_mode=text_only", InputPrice: 0.0005, OutputPrice: 0.002, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/pricing", SortOrder: 10},
					{Component: ProviderModelPriceComponentAudioInput, InputPrice: 0.003, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/pricing", SortOrder: 20},
					{Component: "image_input", InputPrice: 0.003, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/pricing", SortOrder: 30},
					{Component: "video_input", InputPrice: 0.003, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/pricing", SortOrder: 40},
					{Component: ProviderModelPriceComponentAudioOutput, OutputPrice: 0.012, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default", SourceURL: "https://ai.google.dev/pricing", SortOrder: 50},
				},
			},
			{Model: "gemini-2.5-flash-image-preview", Type: ProviderModelTypeImage, InputPrice: 0.039, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "imagen-4.0-generate-preview-06-06", Type: ProviderModelTypeImage, InputPrice: 0.04, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "veo-3.0-generate-preview", Type: ProviderModelTypeVideo, InputPrice: 0.4, PriceUnit: ProviderPriceUnitPerSecond, Currency: ProviderPriceCurrencyUSD, Source: "default"},
		},
	},
	{
		Provider:    "hunyuan",
		Name:        "Tencent Hunyuan",
		BaseURL:     "https://api.hunyuan.cloud.tencent.com/v1",
		OfficialURL: "https://cloud.tencent.com/document/product/1729/97731",
		SortOrder:   125,
		ModelDetails: []ProviderModelDetail{
			{Model: "Hunyuan-T1", Type: ProviderModelTypeText, InputPrice: 0.001, OutputPrice: 0.004, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "Hunyuan-T1-latest", Type: ProviderModelTypeText, InputPrice: 0.001, OutputPrice: 0.004, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "Hunyuan-TurboS", Type: ProviderModelTypeText, InputPrice: 0.0008, OutputPrice: 0.002, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "Hunyuan-TurboS-latest", Type: ProviderModelTypeText, InputPrice: 0.0008, OutputPrice: 0.002, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "Hunyuan-Image", Type: ProviderModelTypeImage, InputPrice: 0.2, PriceUnit: ProviderPriceUnitPerImage, Currency: "CNY", Source: "default"},
			{Model: "Hunyuan-Video", Type: ProviderModelTypeVideo, PriceUnit: ProviderPriceUnitPerSecond, Currency: "CNY", Source: "default"},
		},
	},
	{
		Provider:    "minimax",
		Name:        "MiniMax",
		BaseURL:     "https://api.minimax.io/v1",
		OfficialURL: "https://www.minimax.io/pricing",
		SortOrder:   127,
		ModelDetails: []ProviderModelDetail{
			{Model: "MiniMax-M2.1", Type: ProviderModelTypeText, InputPrice: 0.0003, OutputPrice: 0.0012, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "MiniMax-M2.1-highspeed", Type: ProviderModelTypeText, InputPrice: 0.0003, OutputPrice: 0.0024, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "MiniMax-M2", Type: ProviderModelTypeText, InputPrice: 0.0003, OutputPrice: 0.0012, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "speech-2.5-hd-preview", Type: ProviderModelTypeAudio, InputPrice: 0.1, PriceUnit: ProviderPriceUnitPer1KChars, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "image-01", Type: ProviderModelTypeImage, InputPrice: 0.0035, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "video-01", Type: ProviderModelTypeVideo, PriceUnit: ProviderPriceUnitPerSecond, Currency: ProviderPriceCurrencyUSD, Source: "default"},
		},
	},
	{
		Provider:    "volcengine",
		Name:        "Volcengine Ark",
		BaseURL:     "https://ark.cn-beijing.volces.com/api/v3",
		OfficialURL: "https://www.volcengine.com/docs/82379/1544106?lang=zh",
		SortOrder:   126,
		ModelDetails: []ProviderModelDetail{
			{
				Model:       "doubao-seed-1.6",
				Type:        ProviderModelTypeText,
				InputPrice:  0.0008,
				OutputPrice: 0.008,
				PriceUnit:   ProviderPriceUnitPer1KTokens,
				Currency:    "CNY",
				Source:      "default",
				PriceComponents: []ProviderModelPriceComponentDetail{
					{Component: ProviderModelPriceComponentText, Condition: "prompt_tokens_lte=32000", InputPrice: 0.0008, OutputPrice: 0.008, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default", SourceURL: "https://developer.volcengine.com/articles/7517188354925199414", SortOrder: 10},
					{Component: ProviderModelPriceComponentText, Condition: "prompt_tokens_gt=32000;prompt_tokens_lte=128000", InputPrice: 0.0012, OutputPrice: 0.016, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default", SourceURL: "https://developer.volcengine.com/articles/7517188354925199414", SortOrder: 20},
					{Component: ProviderModelPriceComponentText, Condition: "prompt_tokens_gt=128000;prompt_tokens_lte=256000", InputPrice: 0.0024, OutputPrice: 0.024, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default", SourceURL: "https://developer.volcengine.com/articles/7517188354925199414", SortOrder: 30},
				},
			},
			{
				Model:       "doubao-seed-1.6-thinking",
				Type:        ProviderModelTypeText,
				InputPrice:  0.0008,
				OutputPrice: 0.008,
				PriceUnit:   ProviderPriceUnitPer1KTokens,
				Currency:    "CNY",
				Source:      "default",
				PriceComponents: []ProviderModelPriceComponentDetail{
					{Component: ProviderModelPriceComponentText, Condition: "prompt_tokens_lte=32000", InputPrice: 0.0008, OutputPrice: 0.008, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default", SourceURL: "https://developer.volcengine.com/articles/7516755904465731593", SortOrder: 10},
					{Component: ProviderModelPriceComponentText, Condition: "prompt_tokens_gt=32000;prompt_tokens_lte=128000", InputPrice: 0.0012, OutputPrice: 0.016, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default", SourceURL: "https://developer.volcengine.com/articles/7516755904465731593", SortOrder: 20},
					{Component: ProviderModelPriceComponentText, Condition: "prompt_tokens_gt=128000;prompt_tokens_lte=256000", InputPrice: 0.0024, OutputPrice: 0.024, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default", SourceURL: "https://developer.volcengine.com/articles/7516755904465731593", SortOrder: 30},
				},
			},
			{Model: "doubao-seed-1.6-flash", Type: ProviderModelTypeText, InputPrice: 0.00015, OutputPrice: 0.0015, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "Seed1.6-Embedding", Type: ProviderModelTypeEmbedding, SupportedEndpoints: []string{ChannelModelEndpointEmbeddings}, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{
				Model:       "doubao-seed-code-preview-latest",
				Type:        ProviderModelTypeText,
				InputPrice:  0.0012,
				OutputPrice: 0.008,
				PriceUnit:   ProviderPriceUnitPer1KTokens,
				Currency:    "CNY",
				Source:      "default",
				PriceComponents: []ProviderModelPriceComponentDetail{
					{Component: ProviderModelPriceComponentText, Condition: "prompt_tokens_lte=32000", InputPrice: 0.0012, OutputPrice: 0.008, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default", SourceURL: "https://developer.volcengine.com/articles/7573909942734684166", SortOrder: 10},
					{Component: ProviderModelPriceComponentText, Condition: "prompt_tokens_gt=32000;prompt_tokens_lte=128000", InputPrice: 0.0024, OutputPrice: 0.016, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default", SourceURL: "https://www.volcengine.com/docs/82379/1544106?lang=zh", SortOrder: 20},
					{Component: ProviderModelPriceComponentText, Condition: "prompt_tokens_gt=128000;prompt_tokens_lte=256000", InputPrice: 0.0048, OutputPrice: 0.032, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default", SourceURL: "https://www.volcengine.com/docs/82379/1544106?lang=zh", SortOrder: 30},
				},
			},
		},
	},
	{
		Provider:    "xai",
		Name:        "xAI",
		BaseURL:     "https://api.x.ai",
		OfficialURL: "https://docs.x.ai/docs/models",
		SortOrder:   130,
		ModelDetails: []ProviderModelDetail{
			{Model: "grok-4.20", Type: ProviderModelTypeText, InputPrice: 0.00125, OutputPrice: 0.0025, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "grok-4.3", Type: ProviderModelTypeText, InputPrice: 0.00125, OutputPrice: 0.0025, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "grok-4", Type: ProviderModelTypeText, InputPrice: 0.003, OutputPrice: 0.015, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "grok-4-fast-non-reasoning", Type: ProviderModelTypeText, InputPrice: 0.0002, OutputPrice: 0.0005, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "grok-4-fast-reasoning", Type: ProviderModelTypeText, InputPrice: 0.0002, OutputPrice: 0.0005, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "grok-4-1-fast-non-reasoning", Type: ProviderModelTypeText, InputPrice: 0.0002, OutputPrice: 0.0005, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "grok-4-1-fast-reasoning", Type: ProviderModelTypeText, InputPrice: 0.0002, OutputPrice: 0.0005, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "grok-code-fast-1", Type: ProviderModelTypeText, InputPrice: 0.0002, OutputPrice: 0.0015, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "grok-2-image-1212", Type: ProviderModelTypeImage, InputPrice: 0.07, PriceUnit: ProviderPriceUnitPerImage, Currency: ProviderPriceCurrencyUSD, Source: "default"},
		},
	},
	{
		Provider:    "qwen",
		Name:        "Qwen",
		BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
		OfficialURL: "https://help.aliyun.com/zh/model-studio/models",
		SortOrder:   135,
		ModelDetails: []ProviderModelDetail{
			{Model: "qwen-max-latest", Type: ProviderModelTypeText, InputPrice: 0.011743, OutputPrice: 0.046971, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "qwen-plus-latest", Type: ProviderModelTypeText, InputPrice: 0.002936, OutputPrice: 0.008807, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "qwen-turbo-latest", Type: ProviderModelTypeText, InputPrice: 0.000367, OutputPrice: 0.001468, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "qwen-vl-max-latest", Type: ProviderModelTypeImage, SupportedEndpoints: []string{ChannelModelEndpointResponses}, InputPrice: 0.0016, OutputPrice: 0.004, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "qvq-max-latest", Type: ProviderModelTypeImage, SupportedEndpoints: []string{ChannelModelEndpointResponses}, InputPrice: 0.008, OutputPrice: 0.032, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "qwen-tts-latest", Type: ProviderModelTypeAudio, InputPrice: 0.0016, OutputPrice: 0.01, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{
				Model:     "qwen-omni-turbo-latest",
				Type:      ProviderModelTypeAudio,
				PriceUnit: ProviderPriceUnitPer1KTokens,
				Currency:  "CNY",
				Source:    "default",
				PriceComponents: []ProviderModelPriceComponentDetail{
					{Component: ProviderModelPriceComponentText, Condition: "input_mode=text_only", InputPrice: 0.000514, OutputPrice: 0.001982, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default", SourceURL: "https://help.aliyun.com/zh/model-studio/model-pricing", SortOrder: 10},
					{Component: ProviderModelPriceComponentAudioInput, InputPrice: 0.032586, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default", SourceURL: "https://help.aliyun.com/zh/model-studio/model-pricing", SortOrder: 20},
					{Component: "image_input", InputPrice: 0.001541, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default", SourceURL: "https://help.aliyun.com/zh/model-studio/model-pricing", SortOrder: 30},
					{Component: "video_input", InputPrice: 0.001541, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default", SourceURL: "https://help.aliyun.com/zh/model-studio/model-pricing", SortOrder: 40},
					{Component: ProviderModelPriceComponentText, Condition: "input_mode=multimodal", OutputPrice: 0.004624, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default", SourceURL: "https://help.aliyun.com/zh/model-studio/model-pricing", SortOrder: 50},
					{Component: ProviderModelPriceComponentAudioOutput, Condition: "input_mode=audio_only", OutputPrice: 0.065246, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default", SourceURL: "https://help.aliyun.com/zh/model-studio/model-pricing", SortOrder: 60},
				},
			},
		},
	},
	{
		Provider:    "stepfun",
		Name:        "StepFun",
		BaseURL:     "https://api.stepfun.com/v1",
		OfficialURL: "https://platform.stepfun.com/docs/pricing/details",
		SortOrder:   145,
		ModelDetails: []ProviderModelDetail{
			{Model: "step-3.5-flash", Type: ProviderModelTypeText, InputPrice: 0.0007, OutputPrice: 0.0021, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "step-2-mini", Type: ProviderModelTypeText, InputPrice: 0.001, OutputPrice: 0.002, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "step-1o-turbo-vision", Type: ProviderModelTypeImage, SupportedEndpoints: []string{ChannelModelEndpointResponses}, InputPrice: 0.0025, OutputPrice: 0.008, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "step-1o-audio", Type: ProviderModelTypeAudio, InputPrice: 0.025, OutputPrice: 0.06, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "step-1x-medium", Type: ProviderModelTypeImage, InputPrice: 0.1, PriceUnit: ProviderPriceUnitPerImage, Currency: "CNY", Source: "default"},
		},
	},
	{
		Provider:    "zhipu",
		Name:        "Zhipu",
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4",
		OfficialURL: "https://docs.bigmodel.cn/cn/guide/models/text/glm-4.5",
		SortOrder:   155,
		ModelDetails: []ProviderModelDetail{
			{Model: "glm-4.5-air", Type: ProviderModelTypeText, InputPrice: 0.0008, OutputPrice: 0.002, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "glm-4v-plus-0111", Type: ProviderModelTypeImage, SupportedEndpoints: []string{ChannelModelEndpointResponses}, InputPrice: 0.004, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "glm-4-voice", Type: ProviderModelTypeAudio, InputPrice: 0.08, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "default"},
			{Model: "cogview-4-250304", Type: ProviderModelTypeImage, InputPrice: 0.06, PriceUnit: ProviderPriceUnitPerImage, Currency: "CNY", Source: "default"},
			{Model: "cogvideox-flash", Type: ProviderModelTypeVideo, PriceUnit: ProviderPriceUnitPerSecond, Currency: "CNY", Source: "default"},
		},
	},
	{
		Provider:    "mistral",
		Name:        "Mistral",
		BaseURL:     "https://api.mistral.ai",
		OfficialURL: "https://docs.mistral.ai/getting-started/models/models_overview/",
		SortOrder:   140,
		ModelDetails: []ProviderModelDetail{
			{Model: "mistral-large-latest", Type: ProviderModelTypeText, InputPrice: 0.002, OutputPrice: 0.006, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "mistral-medium-latest", Type: ProviderModelTypeText, InputPrice: 0.0004, OutputPrice: 0.002, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "pixtral-large-latest", Type: ProviderModelTypeImage, SupportedEndpoints: []string{ChannelModelEndpointResponses}, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "voxtral-mini-latest", Type: ProviderModelTypeAudio, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
		},
	},
	{
		Provider:    "cohere",
		Name:        "Cohere",
		BaseURL:     "https://api.cohere.com/compatibility/v1",
		OfficialURL: "https://docs.cohere.com/docs/how-does-cohere-pricing-work",
		SortOrder:   150,
		ModelDetails: []ProviderModelDetail{
			{Model: "command-a-03-2025", Type: ProviderModelTypeText, InputPrice: 0.0025, OutputPrice: 0.01, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
			{Model: "command-r7b-12-2024", Type: ProviderModelTypeText, InputPrice: 0.0000375, OutputPrice: 0.00015, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: ProviderPriceCurrencyUSD, Source: "default"},
		},
	},
})

func normalizeDefaultProviderCatalogTemplates(rows []ProviderCatalogSeed) []ProviderCatalogSeed {
	normalized := make([]ProviderCatalogSeed, 0, len(rows))
	seenProviders := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		provider := strings.TrimSpace(strings.ToLower(row.Provider))
		if provider == "" {
			continue
		}
		if _, exists := seenProviders[provider]; exists {
			continue
		}
		seenProviders[provider] = struct{}{}
		name := strings.TrimSpace(row.Name)
		if name == "" {
			name = provider
		}
		normalized = append(normalized, ProviderCatalogSeed{
			Provider:     provider,
			Name:         name,
			BaseURL:      strings.TrimSpace(row.BaseURL),
			OfficialURL:  strings.TrimSpace(row.OfficialURL),
			SortOrder:    row.SortOrder,
			ModelDetails: normalizeDefaultProviderSeedModelDetails(provider, row.ModelDetails, 0),
		})
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		leftOrder := normalized[i].SortOrder
		rightOrder := normalized[j].SortOrder
		switch {
		case leftOrder > 0 && rightOrder > 0:
			if leftOrder != rightOrder {
				return leftOrder < rightOrder
			}
		case leftOrder > 0:
			return true
		case rightOrder > 0:
			return false
		}
		return normalized[i].Provider < normalized[j].Provider
	})

	nextOrder := 10
	for i := range normalized {
		if normalized[i].SortOrder <= 0 {
			normalized[i].SortOrder = nextOrder
		}
		nextOrder = normalized[i].SortOrder + 10
	}
	return normalized
}

func BuildDefaultProviderCatalogSeeds(now int64) []ProviderCatalogSeed {
	seeds := make([]ProviderCatalogSeed, 0, len(defaultProviderCatalogTemplates))
	for _, template := range defaultProviderCatalogTemplates {
		details := normalizeDefaultProviderSeedModelDetails(template.Provider, template.ModelDetails, now)
		seeds = append(seeds, ProviderCatalogSeed{
			Provider:     template.Provider,
			Name:         template.Name,
			BaseURL:      template.BaseURL,
			OfficialURL:  template.OfficialURL,
			SortOrder:    template.SortOrder,
			ModelDetails: details,
		})
	}
	return seeds
}

func normalizeDefaultProviderSeedModelDetails(provider string, details []ProviderModelDetail, now int64) []ProviderModelDetail {
	normalizedProvider := strings.TrimSpace(strings.ToLower(provider))
	cloned := make([]ProviderModelDetail, 0, len(details))
	for _, detail := range details {
		next := detail
		next.Model = canonicalizeModelNameForProvider(normalizedProvider, next.Model)
		if strings.TrimSpace(next.Model) == "" {
			continue
		}
		if next.UpdatedAt <= 0 {
			next.UpdatedAt = now
		}
		next.Status = defaultProviderModelStatus(normalizedProvider, next.Model)
		if strings.TrimSpace(next.Description) == "" {
			next.Description = defaultProviderModelDescription(normalizedProvider, next.Model, next.Type)
		}
		next.IsDeleted = defaultProviderModelDeleted(normalizedProvider, next.Model)
		if len(next.SupportedEndpoints) == 0 {
			next.SupportedEndpoints = DefaultProviderModelSupportedEndpoints(normalizedProvider, next.Type, next.Model)
		} else {
			next.SupportedEndpoints = NormalizeProviderModelSupportedEndpoints(next.Type, next.SupportedEndpoints)
		}
		cloned = append(cloned, next)
	}
	return NormalizeProviderModelDetails(cloned)
}

func DefaultProviderModelSupportedEndpoints(provider string, modelType string, modelName string) []string {
	normalizedProvider := strings.TrimSpace(strings.ToLower(provider))
	normalizedModelName := strings.TrimSpace(strings.ToLower(modelName))
	if normalizedProvider == "openai" && strings.HasPrefix(normalizedModelName, "gpt-realtime") {
		return []string{ChannelModelEndpointRealtime}
	}
	switch normalizeModelType(modelType, modelName) {
	case ProviderModelTypeImage:
		return []string{ChannelModelEndpointImages}
	case ProviderModelTypeAudio:
		return []string{ChannelModelEndpointAudio}
	case ProviderModelTypeVideo:
		return []string{ChannelModelEndpointVideos}
	case ProviderModelTypeEmbedding:
		return []string{ChannelModelEndpointEmbeddings}
	}
	switch normalizedProvider {
	case "anthropic":
		return []string{ChannelModelEndpointMessages}
	case "openai":
		return []string{ChannelModelEndpointResponses, ChannelModelEndpointChat}
	default:
		return []string{ChannelModelEndpointChat}
	}
}
