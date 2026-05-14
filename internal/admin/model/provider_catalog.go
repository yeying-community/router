package model

import (
	"sort"
	"strings"

	commonutils "github.com/yeying-community/router/common/utils"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
)

const (
	ProviderModelTypeText      = "text"
	ProviderModelTypeImage     = "image"
	ProviderModelTypeAudio     = "audio"
	ProviderModelTypeVideo     = "video"
	ProviderModelTypeEmbedding = "embedding"

	ProviderPriceUnitPer1KTokens = "per_1k_tokens"
	ProviderPriceUnitPer1KChars  = "per_1k_chars"
	ProviderPriceUnitPerImage    = "per_image"
	ProviderPriceUnitPerVideo    = "per_video"
	ProviderPriceUnitPerMinute   = "per_minute"
	ProviderPriceUnitPerSecond   = "per_second"
	ProviderPriceUnitPerRequest  = "per_request"
	ProviderPriceUnitPerTask     = "per_task"

	ProviderPriceCurrencyUSD = "USD"

	ProviderModelPriceComponentText            = "text"
	ProviderModelPriceComponentImageGeneration = "image_generation"
	ProviderModelPriceComponentAudioInput      = "audio_input"
	ProviderModelPriceComponentAudioOutput     = "audio_output"
	ProviderModelPriceComponentVideoGeneration = "video_generation"
	ProviderModelPriceComponentRealtimeText    = "realtime_text"
	ProviderModelPriceComponentRealtimeAudio   = "realtime_audio"
)

type ProviderModelPriceComponentDetail struct {
	Component   string  `json:"component"`
	Condition   string  `json:"condition,omitempty"`
	InputPrice  float64 `json:"input_price,omitempty"`
	OutputPrice float64 `json:"output_price,omitempty"`
	PriceUnit   string  `json:"price_unit,omitempty"`
	Currency    string  `json:"currency,omitempty"`
	Source      string  `json:"source,omitempty"`
	SourceURL   string  `json:"source_url,omitempty"`
	SortOrder   int     `json:"sort_order,omitempty"`
	UpdatedAt   int64   `json:"updated_at,omitempty"`
}

type ProviderModelDetail struct {
	Model              string                              `json:"model"`
	Type               string                              `json:"type,omitempty"`
	Status             string                              `json:"status,omitempty"`
	Description        string                              `json:"description,omitempty"`
	IsDeleted          bool                                `json:"is_deleted,omitempty"`
	SupportedEndpoints []string                            `json:"supported_endpoints,omitempty"`
	InputPrice         float64                             `json:"input_price,omitempty"`
	OutputPrice        float64                             `json:"output_price,omitempty"`
	PriceUnit          string                              `json:"price_unit,omitempty"`
	Currency           string                              `json:"currency,omitempty"`
	Source             string                              `json:"source,omitempty"`
	UpdatedAt          int64                               `json:"updated_at,omitempty"`
	PriceComponents    []ProviderModelPriceComponentDetail `json:"price_components,omitempty"`
}

type ProviderCatalogSeed struct {
	Provider     string
	Name         string
	BaseURL      string
	OfficialURL  string
	SortOrder    int
	ModelDetails []ProviderModelDetail
}

func NormalizeProviderModelDetails(details []ProviderModelDetail) []ProviderModelDetail {
	index := make(map[string]int, len(details))
	normalized := make([]ProviderModelDetail, 0, len(details))
	for _, detail := range details {
		modelName := strings.TrimSpace(detail.Model)
		if modelName == "" {
			continue
		}
		t := normalizeModelType(detail.Type, modelName)
		priceUnit := strings.TrimSpace(strings.ToLower(detail.PriceUnit))
		if priceUnit == "" {
			priceUnit = defaultPriceUnitByType(t, modelName)
		}
		currency := strings.TrimSpace(strings.ToUpper(detail.Currency))
		if currency == "" {
			currency = ProviderPriceCurrencyUSD
		}
		source := strings.TrimSpace(strings.ToLower(detail.Source))
		if source == "" {
			source = "manual"
		}
		status := normalizeProviderModelStatus(detail.Status)
		inputPrice := detail.InputPrice
		if inputPrice < 0 {
			inputPrice = 0
		}
		outputPrice := detail.OutputPrice
		if outputPrice < 0 {
			outputPrice = 0
		}
		entry := ProviderModelDetail{
			Model:              modelName,
			Type:               t,
			Status:             status,
			Description:        strings.TrimSpace(detail.Description),
			IsDeleted:          detail.IsDeleted,
			SupportedEndpoints: NormalizeProviderModelSupportedEndpoints(t, detail.SupportedEndpoints),
			InputPrice:         inputPrice,
			OutputPrice:        outputPrice,
			PriceUnit:          priceUnit,
			Currency:           currency,
			Source:             source,
			UpdatedAt:          detail.UpdatedAt,
			PriceComponents:    NormalizeProviderModelPriceComponents(detail.PriceComponents),
		}
		if idx, ok := index[modelName]; ok {
			existing := normalized[idx]
			if existing.Type == "" {
				existing.Type = entry.Type
			}
			if existing.Status == "" {
				existing.Status = entry.Status
			}
			if existing.Description == "" {
				existing.Description = entry.Description
			}
			if entry.IsDeleted {
				existing.IsDeleted = true
			}
			if existing.PriceUnit == "" {
				existing.PriceUnit = entry.PriceUnit
			}
			if existing.Currency == "" {
				existing.Currency = entry.Currency
			}
			if existing.InputPrice <= 0 && entry.InputPrice > 0 {
				existing.InputPrice = entry.InputPrice
			}
			if existing.OutputPrice <= 0 && entry.OutputPrice > 0 {
				existing.OutputPrice = entry.OutputPrice
			}
			if entry.Source != "default" {
				existing.Source = entry.Source
			}
			if entry.UpdatedAt > existing.UpdatedAt {
				existing.UpdatedAt = entry.UpdatedAt
			}
			existing.SupportedEndpoints = NormalizeProviderModelSupportedEndpoints(existing.Type, append(existing.SupportedEndpoints, entry.SupportedEndpoints...))
			existing.PriceComponents = NormalizeProviderModelPriceComponents(append(existing.PriceComponents, entry.PriceComponents...))
			normalized[idx] = existing
			continue
		}
		index[modelName] = len(normalized)
		normalized = append(normalized, entry)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].Model < normalized[j].Model
	})
	return normalized
}

func FilterActiveProviderModelDetails(details []ProviderModelDetail) []ProviderModelDetail {
	if len(details) == 0 {
		return []ProviderModelDetail{}
	}
	filtered := make([]ProviderModelDetail, 0, len(details))
	for _, detail := range NormalizeProviderModelDetails(details) {
		if detail.IsDeleted {
			continue
		}
		filtered = append(filtered, detail)
	}
	return filtered
}

func normalizeProviderModelStatus(raw string) string {
	status := strings.TrimSpace(strings.ToLower(raw))
	switch status {
	case ProviderModelStatusDeprecated:
		return ProviderModelStatusDeprecated
	case ProviderModelStatusActive, "":
		return ProviderModelStatusActive
	default:
		return ProviderModelStatusActive
	}
}

func NormalizeProviderModelPriceComponents(details []ProviderModelPriceComponentDetail) []ProviderModelPriceComponentDetail {
	index := make(map[string]int, len(details))
	normalized := make([]ProviderModelPriceComponentDetail, 0, len(details))
	for _, detail := range details {
		component := strings.TrimSpace(strings.ToLower(detail.Component))
		if component == "" {
			continue
		}
		condition := strings.TrimSpace(detail.Condition)
		priceUnit := strings.TrimSpace(strings.ToLower(detail.PriceUnit))
		if priceUnit == "" {
			priceUnit = defaultPriceUnitByComponent(component)
		}
		currency := strings.TrimSpace(strings.ToUpper(detail.Currency))
		if currency == "" {
			currency = ProviderPriceCurrencyUSD
		}
		source := strings.TrimSpace(strings.ToLower(detail.Source))
		if source == "" {
			source = "manual"
		}
		entry := ProviderModelPriceComponentDetail{
			Component:   component,
			Condition:   condition,
			InputPrice:  maxProviderPrice(detail.InputPrice),
			OutputPrice: maxProviderPrice(detail.OutputPrice),
			PriceUnit:   priceUnit,
			Currency:    currency,
			Source:      source,
			SourceURL:   strings.TrimSpace(detail.SourceURL),
			SortOrder:   detail.SortOrder,
			UpdatedAt:   detail.UpdatedAt,
		}
		key := component + "\x00" + condition
		if idx, ok := index[key]; ok {
			existing := normalized[idx]
			if existing.InputPrice <= 0 && entry.InputPrice > 0 {
				existing.InputPrice = entry.InputPrice
			}
			if existing.OutputPrice <= 0 && entry.OutputPrice > 0 {
				existing.OutputPrice = entry.OutputPrice
			}
			if existing.PriceUnit == "" {
				existing.PriceUnit = entry.PriceUnit
			}
			if existing.Currency == "" {
				existing.Currency = entry.Currency
			}
			if existing.Source == "" || existing.Source == "default" {
				existing.Source = entry.Source
			}
			if existing.SourceURL == "" {
				existing.SourceURL = entry.SourceURL
			}
			if existing.SortOrder == 0 {
				existing.SortOrder = entry.SortOrder
			}
			if entry.UpdatedAt > existing.UpdatedAt {
				existing.UpdatedAt = entry.UpdatedAt
			}
			normalized[idx] = existing
			continue
		}
		index[key] = len(normalized)
		normalized = append(normalized, entry)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].SortOrder != normalized[j].SortOrder {
			if normalized[i].SortOrder == 0 {
				return false
			}
			if normalized[j].SortOrder == 0 {
				return true
			}
			return normalized[i].SortOrder < normalized[j].SortOrder
		}
		if normalized[i].Component != normalized[j].Component {
			return normalized[i].Component < normalized[j].Component
		}
		return normalized[i].Condition < normalized[j].Condition
	})
	return normalized
}

func NormalizeProviderModelSupportedEndpoints(modelType string, endpoints []string) []string {
	normalizedType := normalizeModelType(modelType, "")
	seen := make(map[string]struct{}, len(endpoints))
	result := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(endpoint)
		if normalizedEndpoint == "" || !IsChannelModelEndpointAllowedForType(normalizedType, normalizedEndpoint) {
			continue
		}
		if _, exists := seen[normalizedEndpoint]; exists {
			continue
		}
		seen[normalizedEndpoint] = struct{}{}
		result = append(result, normalizedEndpoint)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return channelModelEndpointSortRank(result[i]) < channelModelEndpointSortRank(result[j])
	})
	return result
}

func IsChannelModelEndpointAllowedForType(modelType string, endpoint string) bool {
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(endpoint)
	if normalizedEndpoint == "" {
		return false
	}
	switch normalizeModelType(modelType, "") {
	case ProviderModelTypeImage:
		switch normalizedEndpoint {
		case ChannelModelEndpointResponses, ChannelModelEndpointBatches, ChannelModelEndpointImageEdit, ChannelModelEndpointImages:
			return true
		default:
			return false
		}
	case ProviderModelTypeAudio:
		return normalizedEndpoint == ChannelModelEndpointAudio || normalizedEndpoint == ChannelModelEndpointRealtime
	case ProviderModelTypeVideo:
		return normalizedEndpoint == ChannelModelEndpointVideos
	case ProviderModelTypeEmbedding:
		return normalizedEndpoint == ChannelModelEndpointEmbeddings
	default:
		switch normalizedEndpoint {
		case ChannelModelEndpointChat, ChannelModelEndpointResponses, ChannelModelEndpointMessages:
			return true
		default:
			return false
		}
	}
}

func defaultPriceUnitByComponent(component string) string {
	switch strings.TrimSpace(strings.ToLower(component)) {
	case ProviderModelPriceComponentImageGeneration:
		return ProviderPriceUnitPerImage
	case ProviderModelPriceComponentVideoGeneration:
		return ProviderPriceUnitPerSecond
	case ProviderModelPriceComponentAudioInput, ProviderModelPriceComponentAudioOutput,
		ProviderModelPriceComponentRealtimeAudio, ProviderModelPriceComponentRealtimeText:
		return ProviderPriceUnitPer1KTokens
	default:
		return ProviderPriceUnitPer1KTokens
	}
}

func maxProviderPrice(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func inferProviderByModel(modelName string, channelProtocol int, hasChannelProtocol bool) string {
	provider := commonutils.NormalizeProvider(commonutils.ResolveProvider(modelName))
	if provider != "" && provider != "unknown" {
		return provider
	}

	if strings.Contains(modelName, "/") {
		parts := strings.SplitN(modelName, "/", 2)
		prefix := commonutils.NormalizeProvider(parts[0])
		if prefix != "" && prefix != "unknown" {
			return prefix
		}
		plainPrefix := strings.TrimSpace(strings.ToLower(parts[0]))
		if plainPrefix != "" {
			return plainPrefix
		}
	}

	if hasChannelProtocol && channelProtocol > 0 && channelProtocol < len(relaychannel.ChannelProtocolNames) {
		rawProvider := strings.TrimSpace(relaychannel.ChannelProtocolNames[channelProtocol])
		normalized := commonutils.NormalizeProvider(rawProvider)
		if normalized != "" && normalized != "unknown" {
			return normalized
		}
		if rawProvider != "" && rawProvider != "unknown" {
			return strings.ToLower(rawProvider)
		}
	}

	lower := strings.ToLower(strings.TrimSpace(modelName))
	switch {
	case strings.HasPrefix(lower, "ernie-"):
		return "baidu"
	case strings.HasPrefix(lower, "spark-"):
		return "xunfei"
	case strings.HasPrefix(lower, "moonshot-") || strings.HasPrefix(lower, "kimi-"):
		return "moonshot"
	case strings.HasPrefix(lower, "baichuan-"):
		return "baichuan"
	case strings.HasPrefix(lower, "yi-"):
		return "lingyiwanwu"
	case strings.HasPrefix(lower, "step-"):
		return "stepfun"
	case strings.HasPrefix(lower, "groq-"):
		return "groq"
	case strings.HasPrefix(lower, "ollama-"):
		return "ollama"
	}
	return "other"
}

func normalizeModelType(raw string, modelName string) string {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	switch trimmed {
	case ProviderModelTypeText, ProviderModelTypeImage, ProviderModelTypeAudio, ProviderModelTypeVideo, ProviderModelTypeEmbedding:
		return trimmed
	}
	lower := strings.ToLower(strings.TrimSpace(modelName))
	if lower == "" {
		return ProviderModelTypeText
	}
	switch {
	case strings.Contains(lower, "embedding"),
		strings.HasPrefix(lower, "text-embedding"),
		strings.HasPrefix(lower, "seed1.6-embedding"):
		return ProviderModelTypeEmbedding
	}
	switch {
	case strings.HasPrefix(lower, "veo"),
		strings.Contains(lower, "text-to-video"),
		strings.Contains(lower, "video-generation"),
		strings.Contains(lower, "video_generation"),
		strings.Contains(lower, "video"):
		return ProviderModelTypeVideo
	}
	if isKnownImageModel(modelName) {
		return ProviderModelTypeImage
	}
	switch {
	case strings.Contains(lower, "whisper"),
		strings.HasPrefix(lower, "tts-"),
		strings.Contains(lower, "audio"):
		return ProviderModelTypeAudio
	case strings.HasPrefix(lower, "dall-e"),
		strings.HasPrefix(lower, "cogview"),
		strings.Contains(lower, "stable-diffusion"),
		strings.HasPrefix(lower, "wanx"),
		strings.HasPrefix(lower, "step-1x"),
		strings.Contains(lower, "flux"):
		return ProviderModelTypeImage
	default:
		return ProviderModelTypeText
	}
}

func isKnownImageModel(modelName string) bool {
	switch strings.TrimSpace(strings.ToLower(modelName)) {
	case "dall-e-2",
		"dall-e-3",
		"gpt-image-1",
		"gpt-image-2",
		"ali-stable-diffusion-xl",
		"ali-stable-diffusion-v1.5",
		"wanx-v1",
		"cogview-3",
		"step-1x-medium":
		return true
	default:
		return false
	}
}

func InferModelType(modelName string) string {
	return normalizeModelType("", modelName)
}

func defaultPriceUnitByType(modelType string, modelName string) string {
	t := normalizeModelType(modelType, modelName)
	switch t {
	case ProviderModelTypeImage:
		return ProviderPriceUnitPerImage
	case ProviderModelTypeVideo:
		return ProviderPriceUnitPerVideo
	case ProviderModelTypeEmbedding:
		return ProviderPriceUnitPer1KTokens
	case ProviderModelTypeAudio:
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(modelName)), "tts-") {
			return ProviderPriceUnitPer1KChars
		}
		return ProviderPriceUnitPer1KTokens
	default:
		return ProviderPriceUnitPer1KTokens
	}
}
