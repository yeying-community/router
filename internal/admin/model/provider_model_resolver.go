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

type providerModelEndpointLookupRow struct {
	Provider           string `gorm:"column:provider"`
	Model              string `gorm:"column:model"`
	Tags               string `gorm:"column:tags"`
	SupportedEndpoints string `gorm:"column:supported_endpoints"`
}

type providerModelTagLookupRow struct {
	Provider string `gorm:"column:provider"`
	Model    string `gorm:"column:model"`
	Tags     string `gorm:"column:tags"`
}

type providerModelSpecificationLookupRow struct {
	Provider      string `gorm:"column:provider"`
	Model         string `gorm:"column:model"`
	Specification string `gorm:"column:specification"`
}

func LoadUniqueProviderMapByModels(modelNames []string) (map[string]string, error) {
	return LoadUniqueProviderMapByModelsWithDB(DB, modelNames)
}

func LoadProviderModelTagMapByModelsWithDB(db *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string][]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	candidates := NormalizeProviderLookupCandidates(modelNames...)
	result := make(map[string][]string, len(candidates))
	if len(candidates) == 0 {
		return result, nil
	}
	rows := make([]providerModelTagLookupRow, 0)
	if err := db.
		Model(&ProviderModel{}).
		Select("provider", "model", "tags").
		Where("is_deleted = ? AND model IN ?", false, candidates).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		provider := NormalizeGroupModelProviderValue(row.Provider)
		modelName := canonicalizeModelNameForProvider(provider, row.Model)
		if modelName == "" {
			continue
		}
		expectedProvider := ResolveProviderFromModelMap(providerByModel, modelName)
		if expectedProvider == "" || expectedProvider != provider {
			continue
		}
		result[modelName] = NormalizeProviderModelTags(splitProviderModelTags(row.Tags))
	}
	return result, nil
}

func LoadProviderModelSpecificationMapByModelsWithDB(db *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string]*ProviderModelSpecification, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	candidates := NormalizeProviderLookupCandidates(modelNames...)
	result := make(map[string]*ProviderModelSpecification, len(candidates))
	if len(candidates) == 0 {
		return result, nil
	}
	rows := make([]providerModelSpecificationLookupRow, 0)
	if err := db.
		Model(&ProviderModel{}).
		Select("provider", "model", "specification").
		Where("is_deleted = ? AND model IN ?", false, candidates).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		provider := NormalizeGroupModelProviderValue(row.Provider)
		modelName := canonicalizeModelNameForProvider(provider, row.Model)
		if modelName == "" {
			continue
		}
		expectedProvider := ResolveProviderFromModelMap(providerByModel, modelName)
		if expectedProvider == "" || expectedProvider != provider {
			continue
		}
		specification, err := ParseProviderModelSpecification(row.Specification)
		if err != nil {
			return nil, fmt.Errorf("parse provider model specification for %s/%s: %w", provider, modelName, err)
		}
		result[modelName] = specification
	}
	return result, nil
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
		Where("is_deleted = ? AND model IN ?", false, candidates).
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

func ResolveProviderFromModelMap(providerByModel map[string]string, values ...string) string {
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

func LoadProviderModelEndpointMapByModelsWithDB(db *gorm.DB, provider string, modelNames []string) (map[string][]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedProvider := NormalizeGroupModelProviderValue(provider)
	candidates := NormalizeProviderLookupCandidates(modelNames...)
	result := make(map[string][]string, len(candidates))
	if normalizedProvider == "" || len(candidates) == 0 {
		return result, nil
	}
	rows := make([]providerModelEndpointLookupRow, 0)
	if err := db.
		Model(&ProviderModel{}).
		Select("provider", "model", "tags", "supported_endpoints").
		Where("provider = ? AND is_deleted = ? AND model IN ?", normalizedProvider, false, candidates).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		modelName := canonicalizeModelNameForProvider(normalizedProvider, row.Model)
		modelType := ProviderModelTypeFromTags(splitProviderModelTags(row.Tags))
		if modelType == "" {
			continue
		}
		endpoints := NormalizeProviderModelSupportedEndpoints(
			modelType,
			splitProviderModelSupportedEndpoints(row.SupportedEndpoints),
		)
		if len(endpoints) == 0 {
			endpoints = DefaultProviderModelSupportedEndpoints(
				normalizedProvider,
				modelType,
				modelName,
			)
		}
		if modelName == "" || len(endpoints) == 0 {
			continue
		}
		result[modelName] = endpoints
	}
	return result, nil
}
