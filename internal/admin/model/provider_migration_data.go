package model

import "strings"

func normalizeProviderMigrationSeedModelDetails(provider string, details []ProviderModelDetail, now int64) []ProviderModelDetail {
	normalizedProvider := strings.TrimSpace(strings.ToLower(provider))
	cloned := make([]ProviderModelDetail, 0, len(details))
	for _, detail := range details {
		next := detail
		next.Model = canonicalizeModelNameForProvider(normalizedProvider, next.Model)
		if strings.TrimSpace(next.Model) == "" {
			continue
		}
		if strings.TrimSpace(next.Type) == "" {
			next.Type = normalizeModelType("", next.Model)
		}
		next.Tags = NormalizeProviderModelTags(append([]string{next.Type}, next.Tags...))
		if next.UpdatedAt <= 0 {
			next.UpdatedAt = now
		}
		next.Status = defaultProviderModelStatus(normalizedProvider, next.Model)
		if strings.TrimSpace(next.Description) == "" {
			next.Description = defaultProviderModelDescription(normalizedProvider, next.Model, next.Type)
		}
		if strings.TrimSpace(strings.ToLower(next.Source)) == "" {
			next.Source = "migration"
		}
		for i := range next.PriceComponents {
			source := strings.TrimSpace(strings.ToLower(next.PriceComponents[i].Source))
			if source == "" {
				next.PriceComponents[i].Source = "migration"
			}
		}
		next.IsDeleted = defaultProviderModelDeleted(normalizedProvider, next.Model)
		if explicitEndpoints, handled := explicitProviderModelSupportedEndpoints(normalizedProvider, next.Type, next.Model, next.SupportedEndpoints); handled {
			next.SupportedEndpoints = explicitEndpoints
		} else if len(next.SupportedEndpoints) == 0 {
			next.SupportedEndpoints = DefaultProviderModelSupportedEndpoints(normalizedProvider, next.Type, next.Model)
		} else {
			next.SupportedEndpoints = NormalizeProviderModelSupportedEndpointsForModel(next.Type, next.Model, next.SupportedEndpoints)
		}
		cloned = append(cloned, next)
	}
	return NormalizeProviderModelDetails(cloned)
}

func explicitProviderModelSupportedEndpoints(provider string, modelType string, modelName string, current []string) ([]string, bool) {
	switch provider {
	case "qwen":
		return qwenProviderSupportedEndpoints(modelType, modelName, current)
	case "volcengine":
		return volcengineProviderSupportedEndpoints(modelType, modelName, current)
	default:
		return nil, false
	}
}

func volcengineProviderSupportedEndpoints(modelType string, modelName string, current []string) ([]string, bool) {
	normalizedType := normalizeModelType(modelType, modelName)
	switch normalizedType {
	case ProviderModelTypeEmbedding:
		return []string{ChannelModelEndpointEmbeddings}, true
	case ProviderModelTypeImage:
		return []string{ChannelModelEndpointImages}, true
	case ProviderModelTypeVideo:
		return []string{}, true
	case ProviderModelTypeText:
		return []string{ChannelModelEndpointChat, ChannelModelEndpointResponses}, true
	default:
		if len(current) > 0 {
			return NormalizeProviderModelSupportedEndpointsForModel(modelType, modelName, current), true
		}
		return nil, false
	}
}

func qwenProviderSupportedEndpoints(modelType string, modelName string, current []string) ([]string, bool) {
	normalizedModelName := strings.TrimSpace(strings.ToLower(modelName))
	if normalizedModelName == "" {
		return nil, false
	}
	switch {
	case strings.Contains(normalizedModelName, "tts"):
		return []string{}, true
	case strings.HasPrefix(normalizedModelName, "qwen-image"):
		return []string{ChannelModelEndpointImages, ChannelModelEndpointImageEdit}, true
	case strings.HasSuffix(normalizedModelName, "-realtime"),
		strings.Contains(normalizedModelName, "omni-realtime"):
		return []string{ChannelModelEndpointRealtime}, true
	case strings.Contains(normalizedModelName, "omni"),
		strings.Contains(normalizedModelName, "asr"),
		strings.HasPrefix(normalizedModelName, "qwen-vl"),
		strings.HasPrefix(normalizedModelName, "qvq-"):
		return []string{ChannelModelEndpointChat}, true
	case strings.HasPrefix(normalizedModelName, "qwen-max"),
		strings.HasPrefix(normalizedModelName, "qwen-plus"),
		strings.HasPrefix(normalizedModelName, "qwen-turbo"),
		strings.HasPrefix(normalizedModelName, "qwen3"),
		strings.HasPrefix(normalizedModelName, "qwen-mt"),
		strings.HasPrefix(normalizedModelName, "qwq-"):
		if len(current) > 0 {
			return NormalizeProviderModelSupportedEndpointsForModel(modelType, modelName, current), true
		}
		return []string{ChannelModelEndpointChat}, true
	default:
		if len(current) > 0 {
			return NormalizeProviderModelSupportedEndpointsForModel(modelType, modelName, current), true
		}
		return nil, false
	}
}

func DefaultProviderModelSupportedEndpoints(provider string, modelType string, modelName string) []string {
	normalizedProvider := strings.TrimSpace(strings.ToLower(provider))
	normalizedModelName := strings.TrimSpace(strings.ToLower(modelName))
	if explicitEndpoints, handled := explicitProviderModelSupportedEndpoints(normalizedProvider, modelType, modelName, nil); handled {
		return explicitEndpoints
	}
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
