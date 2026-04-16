package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type providerModelLookupRow struct {
	Provider string `gorm:"column:provider"`
	Model    string `gorm:"column:model"`
}

func LoadUniqueProviderMapByModels(modelNames []string) (map[string]string, error) {
	return LoadUniqueProviderMapByModelsWithDB(DB, modelNames)
}

func LoadUniqueProviderMapByModelsWithDB(db *gorm.DB, modelNames []string) (map[string]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	candidates := NormalizeProviderLookupCandidates(modelNames...)
	result := make(map[string]string, len(candidates))
	if len(candidates) == 0 {
		return result, nil
	}
	rows := make([]providerModelLookupRow, 0)
	if err := db.
		Model(&ProviderModel{}).
		Select("provider", "model").
		Where("model IN ?", candidates).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	providersByModel := make(map[string]map[string]struct{}, len(rows))
	for _, row := range rows {
		modelName := strings.TrimSpace(row.Model)
		if modelName == "" {
			continue
		}
		provider := NormalizeGroupModelProviderValue(row.Provider)
		if provider == "" {
			continue
		}
		if _, ok := providersByModel[modelName]; !ok {
			providersByModel[modelName] = make(map[string]struct{}, 1)
		}
		providersByModel[modelName][provider] = struct{}{}
	}
	for _, modelName := range candidates {
		candidateSet, ok := providersByModel[modelName]
		if !ok || len(candidateSet) != 1 {
			continue
		}
		for provider := range candidateSet {
			result[modelName] = provider
		}
	}
	return result, nil
}

func NormalizeProviderLookupCandidates(values ...string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values)*2)
	result := make([]string, 0, len(values)*2)
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; !ok {
			seen[normalized] = struct{}{}
			result = append(result, normalized)
		}
		if strings.Contains(normalized, "/") {
			parts := strings.SplitN(normalized, "/", 2)
			if len(parts) == 2 {
				suffix := strings.TrimSpace(parts[1])
				if suffix != "" {
					if _, ok := seen[suffix]; !ok {
						seen[suffix] = struct{}{}
						result = append(result, suffix)
					}
				}
			}
		}
	}
	return result
}

func ResolveProviderFromCatalogMap(providerByModel map[string]string, values ...string) string {
	if len(providerByModel) == 0 || len(values) == 0 {
		return ""
	}
	candidates := NormalizeProviderLookupCandidates(values...)
	for _, candidate := range candidates {
		provider := NormalizeGroupModelProviderValue(providerByModel[candidate])
		if provider != "" {
			return provider
		}
	}
	return ""
}
