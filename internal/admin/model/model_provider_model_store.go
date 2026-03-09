package model

import (
	"strings"

	commonutils "github.com/yeying-community/router/common/utils"
	"gorm.io/gorm"
)

var providersUseFlatModelIDs = map[string]struct{}{
	"openai":      {},
	"google":      {},
	"anthropic":   {},
	"xai":         {},
	"mistral":     {},
	"cohere":      {},
	"deepseek":    {},
	"qwen":        {},
	"zhipu":       {},
	"hunyuan":     {},
	"volcengine":  {},
	"minimax":     {},
	"baidu":       {},
	"baidu-v2":    {},
	"moonshot":    {},
	"baichuan":    {},
	"lingyiwanwu": {},
	"stepfun":     {},
	"groq":        {},
	"xunfei":      {},
}

func canonicalizeModelNameForProvider(provider string, modelName string) string {
	normalizedProvider := commonutils.NormalizeModelProvider(provider)
	if normalizedProvider == "" {
		normalizedProvider = strings.TrimSpace(strings.ToLower(provider))
	}
	name := strings.TrimSpace(modelName)
	if name == "" {
		return ""
	}
	if _, ok := providersUseFlatModelIDs[normalizedProvider]; !ok {
		return name
	}
	prefix := normalizedProvider + "/"
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, prefix) {
		trimmed := strings.TrimSpace(name[len(prefix):])
		if trimmed != "" {
			return trimmed
		}
	}
	return name
}

func LoadModelProviderModelDetailsMap(db *gorm.DB) (map[string][]ModelProviderModelDetail, error) {
	return LoadModelProviderModelDetailsMapForProviders(db, nil)
}

func LoadModelProviderModelDetailsMapForProviders(db *gorm.DB, providers []string) (map[string][]ModelProviderModelDetail, error) {
	rows := make([]ModelProviderModel, 0)
	query := db.Order("provider asc, model asc")
	if len(providers) > 0 {
		query = query.Where("provider IN ?", providers)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string][]ModelProviderModelDetail, 0)
	for _, row := range rows {
		provider := commonutils.NormalizeModelProvider(row.Provider)
		if provider == "" {
			provider = strings.TrimSpace(strings.ToLower(row.Provider))
		}
		if provider == "" {
			continue
		}
		modelName := canonicalizeModelNameForProvider(provider, row.Model)
		if modelName == "" {
			continue
		}
		result[provider] = append(result[provider], ModelProviderModelDetail{
			Model:       modelName,
			Type:        strings.TrimSpace(strings.ToLower(row.Type)),
			InputPrice:  row.InputPrice,
			OutputPrice: row.OutputPrice,
			PriceUnit:   strings.TrimSpace(strings.ToLower(row.PriceUnit)),
			Currency:    strings.TrimSpace(strings.ToUpper(row.Currency)),
			Source:      strings.TrimSpace(strings.ToLower(row.Source)),
			UpdatedAt:   row.UpdatedAt,
		})
	}
	for provider, details := range result {
		result[provider] = NormalizeModelProviderModelDetails(details)
	}
	return result, nil
}

func BuildModelProviderModelRows(provider string, details []ModelProviderModelDetail, now int64) []ModelProviderModel {
	normalizedProvider := commonutils.NormalizeModelProvider(provider)
	if normalizedProvider == "" {
		normalizedProvider = strings.TrimSpace(strings.ToLower(provider))
	}
	if normalizedProvider == "" {
		return nil
	}
	detailInput := make([]ModelProviderModelDetail, 0, len(details))
	for _, detail := range details {
		detail.Model = canonicalizeModelNameForProvider(normalizedProvider, detail.Model)
		if strings.TrimSpace(detail.Model) == "" {
			continue
		}
		detailInput = append(detailInput, detail)
	}
	normalizedDetails := NormalizeModelProviderModelDetails(detailInput)
	rows := make([]ModelProviderModel, 0, len(normalizedDetails))
	for _, detail := range normalizedDetails {
		updatedAt := detail.UpdatedAt
		if updatedAt == 0 {
			updatedAt = now
		}
		rows = append(rows, ModelProviderModel{
			Provider:    normalizedProvider,
			Model:       detail.Model,
			Type:        detail.Type,
			InputPrice:  detail.InputPrice,
			OutputPrice: detail.OutputPrice,
			PriceUnit:   detail.PriceUnit,
			Currency:    detail.Currency,
			Source:      detail.Source,
			UpdatedAt:   updatedAt,
		})
	}
	return rows
}
