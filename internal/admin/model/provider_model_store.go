package model

import (
	"strings"

	commonutils "github.com/yeying-community/router/common/utils"
	"gorm.io/gorm"
)

type ProviderModelStoreRows struct {
	Models          []ProviderModel
	PriceComponents []ProviderModelPriceComponent
}

func canonicalizeModelNameForProvider(provider string, modelName string) string {
	normalizedProvider := commonutils.NormalizeProvider(provider)
	if normalizedProvider == "" {
		normalizedProvider = strings.TrimSpace(strings.ToLower(provider))
	}
	name := strings.TrimSpace(modelName)
	if name == "" {
		return ""
	}
	if strings.Contains(name, "/") {
		parts := strings.SplitN(name, "/", 2)
		if len(parts) == 2 {
			prefix := commonutils.NormalizeProvider(parts[0])
			if prefix == "" || prefix == "unknown" {
				prefix = strings.TrimSpace(strings.ToLower(parts[0]))
			}
			if prefix == normalizedProvider {
				trimmed := strings.TrimSpace(parts[1])
				if trimmed != "" {
					name = trimmed
				}
			}
		}
	}
	lower := strings.ToLower(name)
	if normalizedProvider == "meta" && strings.HasPrefix(lower, "meta-") {
		trimmed := strings.TrimSpace(name[len("meta-"):])
		if trimmed != "" {
			name = trimmed
		}
	}
	return name
}

func LoadProviderModelDetailsMap(db *gorm.DB) (map[string][]ProviderModelDetail, error) {
	return LoadProviderModelDetailsMapForProviders(db, nil)
}

func LoadProviderModelDetailsMapForProviders(db *gorm.DB, providers []string) (map[string][]ProviderModelDetail, error) {
	rows := make([]ProviderModel, 0)
	query := db.Order("provider asc, model asc")
	if len(providers) > 0 {
		query = query.Where("provider IN ?", providers)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	componentRows := make([]ProviderModelPriceComponent, 0)
	componentQuery := db.Order("provider asc, model asc, sort_order asc, component asc, condition asc")
	if len(providers) > 0 {
		componentQuery = componentQuery.Where("provider IN ?", providers)
	}
	if err := componentQuery.Find(&componentRows).Error; err != nil {
		return nil, err
	}
	result := make(map[string][]ProviderModelDetail, 0)
	detailIndex := make(map[string]int, len(rows))
	for _, row := range rows {
		provider := commonutils.NormalizeProvider(row.Provider)
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
		detail := ProviderModelDetail{
			Model:              modelName,
			Tags:               NormalizeProviderModelTags(splitProviderModelTags(row.Tags)),
			Status:             normalizeProviderModelStatus(row.Status),
			Description:        strings.TrimSpace(row.Description),
			IsDeleted:          row.IsDeleted,
			SupportedEndpoints: splitProviderModelSupportedEndpoints(row.SupportedEndpoints),
			InputPrice:         row.InputPrice,
			OutputPrice:        row.OutputPrice,
			PriceUnit:          strings.TrimSpace(strings.ToLower(row.PriceUnit)),
			Currency:           strings.TrimSpace(strings.ToUpper(row.Currency)),
			Source:             strings.TrimSpace(strings.ToLower(row.Source)),
			UpdatedAt:          row.UpdatedAt,
		}
		detail.Type = ProviderModelTypeFromTags(detail.Tags)
		if len(detail.SupportedEndpoints) == 0 {
			if detail.Type == "" {
				continue
			}
			detail.SupportedEndpoints = DefaultProviderModelSupportedEndpoints(
				provider,
				detail.Type,
				detail.Model,
			)
		}
		result[provider] = append(result[provider], detail)
		detailIndex[provider+"\x00"+modelName] = len(result[provider]) - 1
	}
	for _, row := range componentRows {
		provider := commonutils.NormalizeProvider(row.Provider)
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
		key := provider + "\x00" + modelName
		index, ok := detailIndex[key]
		if !ok {
			continue
		}
		detail := result[provider][index]
		detail.PriceComponents = append(detail.PriceComponents, ProviderModelPriceComponentDetail{
			Component:   strings.TrimSpace(strings.ToLower(row.Component)),
			Condition:   strings.TrimSpace(row.Condition),
			InputPrice:  row.InputPrice,
			OutputPrice: row.OutputPrice,
			PriceUnit:   strings.TrimSpace(strings.ToLower(row.PriceUnit)),
			Currency:    strings.TrimSpace(strings.ToUpper(row.Currency)),
			Source:      strings.TrimSpace(strings.ToLower(row.Source)),
			SourceURL:   strings.TrimSpace(row.SourceURL),
			SortOrder:   row.SortOrder,
			UpdatedAt:   row.UpdatedAt,
		})
		result[provider][index] = detail
	}
	for provider, details := range result {
		result[provider] = NormalizeProviderModelDetails(details)
	}
	return result, nil
}

func BuildProviderModelRows(provider string, details []ProviderModelDetail, now int64) []ProviderModel {
	return BuildProviderModelStoreRows(provider, details, now).Models
}

func BuildProviderModelStoreRows(provider string, details []ProviderModelDetail, now int64) ProviderModelStoreRows {
	normalizedProvider := commonutils.NormalizeProvider(provider)
	if normalizedProvider == "" {
		normalizedProvider = strings.TrimSpace(strings.ToLower(provider))
	}
	if normalizedProvider == "" {
		return ProviderModelStoreRows{}
	}
	detailInput := make([]ProviderModelDetail, 0, len(details))
	for _, detail := range details {
		detail.Model = canonicalizeModelNameForProvider(normalizedProvider, detail.Model)
		if strings.TrimSpace(detail.Model) == "" {
			continue
		}
		detailInput = append(detailInput, detail)
	}
	normalizedDetails := NormalizeProviderModelDetails(detailInput)
	rows := make([]ProviderModel, 0, len(normalizedDetails))
	componentRows := make([]ProviderModelPriceComponent, 0)
	for _, detail := range normalizedDetails {
		updatedAt := detail.UpdatedAt
		if updatedAt == 0 {
			updatedAt = now
		}
		rows = append(rows, ProviderModel{
			Provider:           normalizedProvider,
			Model:              detail.Model,
			Tags:               joinProviderModelTags(detail.Model, detail.Tags),
			Status:             normalizeProviderModelStatus(detail.Status),
			Description:        strings.TrimSpace(detail.Description),
			IsDeleted:          detail.IsDeleted,
			SupportedEndpoints: joinProviderModelSupportedEndpoints(detail.Type, detail.SupportedEndpoints),
			InputPrice:         detail.InputPrice,
			OutputPrice:        detail.OutputPrice,
			PriceUnit:          detail.PriceUnit,
			Currency:           detail.Currency,
			Source:             detail.Source,
			UpdatedAt:          updatedAt,
		})
		for _, component := range NormalizeProviderModelPriceComponents(detail.PriceComponents) {
			componentUpdatedAt := component.UpdatedAt
			if componentUpdatedAt == 0 {
				componentUpdatedAt = updatedAt
			}
			componentRows = append(componentRows, ProviderModelPriceComponent{
				Provider:    normalizedProvider,
				Model:       detail.Model,
				Component:   component.Component,
				Condition:   component.Condition,
				InputPrice:  component.InputPrice,
				OutputPrice: component.OutputPrice,
				PriceUnit:   component.PriceUnit,
				Currency:    component.Currency,
				Source:      component.Source,
				SourceURL:   component.SourceURL,
				SortOrder:   component.SortOrder,
				UpdatedAt:   componentUpdatedAt,
			})
		}
	}
	return ProviderModelStoreRows{
		Models:          rows,
		PriceComponents: componentRows,
	}
}

func splitProviderModelSupportedEndpoints(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := NormalizeRequestedChannelModelEndpoint(part)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}

func joinProviderModelSupportedEndpoints(modelType string, values []string) string {
	normalized := NormalizeProviderModelSupportedEndpoints(modelType, values)
	if len(normalized) == 0 {
		return ""
	}
	return strings.Join(normalized, ",")
}

func splitProviderModelTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(strings.ToLower(part))
		if tag == "" {
			continue
		}
		result = append(result, tag)
	}
	return result
}

func joinProviderModelTags(modelName string, values []string) string {
	normalized := NormalizeProviderModelTags(values)
	if len(normalized) == 0 {
		return ""
	}
	return strings.Join(normalized, ",")
}
