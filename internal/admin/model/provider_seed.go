package model

import (
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func normalizeProviderSortOrderValue(sortOrder int) int {
	if sortOrder > 0 {
		return sortOrder
	}
	return 0
}

func replaceProviderMigrationSeedsWithDB(db *gorm.DB) error {
	if err := db.AutoMigrate(&Provider{}, &ProviderModel{}, &ProviderModelPriceComponent{}); err != nil {
		return err
	}
	seeds, err := LoadProviderMigrationSeeds(helper.GetTimestamp())
	if err != nil {
		return err
	}
	return replaceProviderSeedsToTable(db, seeds)
}

func upsertProviderMigrationProvidersWithDB(db *gorm.DB, providers ...string) error {
	normalizedProviders := normalizeTrimmedValuesPreserveOrder(providers)
	if len(normalizedProviders) == 0 {
		return fmt.Errorf("at least one provider is required")
	}
	return upsertProviderMigrationSeedsWithDB(db, normalizedProviders)
}

func replaceProviderSeedsToTable(db *gorm.DB, seeds []ProviderSeed) error {
	now := helper.GetTimestamp()
	providerRows := make([]Provider, 0, len(seeds))
	modelRows := make([]ProviderModel, 0)
	componentRows := make([]ProviderModelPriceComponent, 0)
	providerIDs := make([]string, 0, len(seeds))
	existingCreatedAtByProvider := make(map[string]int64)
	existingRows := make([]Provider, 0)
	if err := db.Select("id", "created_at").Find(&existingRows).Error; err != nil {
		return err
	}
	for _, row := range existingRows {
		provider := strings.TrimSpace(strings.ToLower(row.Id))
		if provider == "" || row.CreatedAt <= 0 {
			continue
		}
		existingCreatedAtByProvider[provider] = row.CreatedAt
	}
	for _, seed := range seeds {
		provider := strings.TrimSpace(strings.ToLower(seed.Provider))
		if provider == "" {
			continue
		}
		providerIDs = append(providerIDs, provider)
		details := normalizeProviderMigrationSeedModelDetails(provider, seed.ModelDetails, now)
		providerRows = append(providerRows, Provider{
			Id:          provider,
			Name:        strings.TrimSpace(seed.Name),
			BaseURL:     strings.TrimSpace(seed.BaseURL),
			OfficialURL: strings.TrimSpace(seed.OfficialURL),
			SortOrder:   normalizeProviderSortOrderValue(seed.SortOrder),
			Source:      "migration",
			CreatedAt: func() int64 {
				if existingCreatedAtByProvider[provider] > 0 {
					return existingCreatedAtByProvider[provider]
				}
				return now
			}(),
			UpdatedAt: now,
		})
		storeRows := BuildProviderModelStoreRows(provider, details, now)
		modelRows = append(modelRows, storeRows.Models...)
		componentRows = append(componentRows, storeRows.PriceComponents...)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		existingMigrationProviderIDs := make([]string, 0)
		if err := tx.Model(&Provider{}).Where("source = ?", "migration").Pluck("id", &existingMigrationProviderIDs).Error; err != nil {
			return err
		}
		deleteProviderIDs := mergeProviderIDs(existingMigrationProviderIDs, providerIDs)
		if len(deleteProviderIDs) > 0 {
			if err := tx.Where("provider IN ?", deleteProviderIDs).Delete(&ProviderModelPriceComponent{}).Error; err != nil {
				return err
			}
			if err := tx.Where("provider IN ?", deleteProviderIDs).Delete(&ProviderModel{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id IN ?", deleteProviderIDs).Delete(&Provider{}).Error; err != nil {
				return err
			}
		}
		if len(providerRows) > 0 {
			if err := tx.Create(&providerRows).Error; err != nil {
				return err
			}
		}
		if len(modelRows) > 0 {
			if err := tx.Create(&modelRows).Error; err != nil {
				return err
			}
		}
		if len(componentRows) > 0 {
			if err := tx.Create(&componentRows).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func upsertProviderMigrationSeedsWithDB(db *gorm.DB, providers []string) error {
	if err := db.AutoMigrate(&Provider{}, &ProviderModel{}, &ProviderModelPriceComponent{}); err != nil {
		return err
	}
	seeds, err := loadSelectedProviderMigrationSeeds(helper.GetTimestamp(), providers)
	if err != nil {
		return err
	}
	now := helper.GetTimestamp()
	providerRows := make([]Provider, 0, len(seeds))
	modelRows := make([]ProviderModel, 0)
	componentRows := make([]ProviderModelPriceComponent, 0)
	modelsByProvider := make(map[string]map[string]struct{}, len(seeds))
	existingCreatedAtByProvider := make(map[string]int64)
	existingRows := make([]Provider, 0)
	if err := db.Select("id", "created_at").Find(&existingRows).Error; err != nil {
		return err
	}
	for _, row := range existingRows {
		provider := strings.TrimSpace(strings.ToLower(row.Id))
		if provider == "" || row.CreatedAt <= 0 {
			continue
		}
		existingCreatedAtByProvider[provider] = row.CreatedAt
	}
	for _, seed := range seeds {
		provider := strings.TrimSpace(strings.ToLower(seed.Provider))
		if provider == "" {
			continue
		}
		if _, ok := modelsByProvider[provider]; !ok {
			modelsByProvider[provider] = make(map[string]struct{})
		}
		details := normalizeProviderMigrationSeedModelDetails(provider, seed.ModelDetails, now)
		for _, detail := range details {
			modelName := canonicalizeModelNameForProvider(provider, detail.Model)
			if modelName == "" {
				continue
			}
			modelsByProvider[provider][modelName] = struct{}{}
		}
		providerRows = append(providerRows, Provider{
			Id:          provider,
			Name:        strings.TrimSpace(seed.Name),
			BaseURL:     strings.TrimSpace(seed.BaseURL),
			OfficialURL: strings.TrimSpace(seed.OfficialURL),
			SortOrder:   normalizeProviderSortOrderValue(seed.SortOrder),
			Source:      "migration",
			CreatedAt: func() int64 {
				if existingCreatedAtByProvider[provider] > 0 {
					return existingCreatedAtByProvider[provider]
				}
				return now
			}(),
			UpdatedAt: now,
		})
		storeRows := BuildProviderModelStoreRows(provider, details, now)
		modelRows = append(modelRows, storeRows.Models...)
		componentRows = append(componentRows, storeRows.PriceComponents...)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if len(providerRows) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"name",
					"base_url",
					"official_url",
					"sort_order",
					"source",
					"updated_at",
				}),
			}).Create(&providerRows).Error; err != nil {
				return err
			}
		}
		for provider, modelSet := range modelsByProvider {
			targetModels := make([]string, 0, len(modelSet))
			for modelName := range modelSet {
				targetModels = append(targetModels, modelName)
			}
			if err := pruneMissingProviderMigrationModelsWithDB(tx, provider, targetModels); err != nil {
				return err
			}
			if len(targetModels) == 0 {
				continue
			}
			if err := tx.Where(
				"provider = ? AND model IN ? AND source = ? AND component NOT IN ?",
				provider,
				targetModels,
				"migration",
				[]string{
					ProviderModelPriceComponentTextCacheRead,
					ProviderModelPriceComponentTextCacheWrite,
				},
			).
				Delete(&ProviderModelPriceComponent{}).Error; err != nil {
				return err
			}
		}
		if len(modelRows) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "provider"}, {Name: "model"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"tags",
					"status",
					"description",
					"specification",
					"is_deleted",
					"supported_endpoints",
					"input_price",
					"output_price",
					"price_unit",
					"currency",
					"source",
					"updated_at",
				}),
			}).Create(&modelRows).Error; err != nil {
				return err
			}
		}
		if len(componentRows) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "provider"},
					{Name: "model"},
					{Name: "component"},
					{Name: "condition"},
				},
				DoUpdates: clause.AssignmentColumns([]string{
					"input_price",
					"output_price",
					"price_unit",
					"currency",
					"source",
					"source_url",
					"sort_order",
					"updated_at",
				}),
			}).Create(&componentRows).Error; err != nil {
				return err
			}
		}
		return upsertProviderTextCachePricingComponentsInTransaction(tx)
	})
}

func normalizeProviderMigrationLegacySourcesWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	updates := []struct {
		model any
	}{
		{model: &Provider{}},
		{model: &ProviderModel{}},
		{model: &ProviderModelPriceComponent{}},
	}
	for _, item := range updates {
		if err := db.Model(item.model).
			Where("source = ?", "default").
			Update("source", "migration").Error; err != nil {
			return err
		}
	}
	return nil
}

func loadSelectedProviderMigrationSeeds(now int64, providers []string) ([]ProviderSeed, error) {
	allSeeds, err := LoadProviderMigrationSeeds(now)
	if err != nil {
		return nil, err
	}
	if len(providers) == 0 {
		return nil, fmt.Errorf("at least one provider is required")
	}
	seedByProvider := make(map[string]ProviderSeed, len(allSeeds))
	for _, seed := range allSeeds {
		provider := strings.TrimSpace(strings.ToLower(seed.Provider))
		if provider == "" {
			continue
		}
		seedByProvider[provider] = seed
	}
	selected := make([]ProviderSeed, 0, len(providers))
	for _, rawProvider := range providers {
		provider := strings.TrimSpace(strings.ToLower(rawProvider))
		if provider == "" {
			continue
		}
		seed, ok := seedByProvider[provider]
		if !ok {
			return nil, fmt.Errorf("provider migration snapshot missing provider %q", provider)
		}
		selected = append(selected, seed)
	}
	return selected, nil
}

func pruneMissingProviderMigrationModelsWithDB(tx *gorm.DB, provider string, targetModels []string) error {
	existingRows := make([]ProviderModel, 0)
	if err := tx.Select("model").
		Where("provider = ? AND source = ?", provider, "migration").
		Find(&existingRows).Error; err != nil {
		return err
	}
	if len(existingRows) == 0 {
		return nil
	}
	targetSet := make(map[string]struct{}, len(targetModels))
	for _, modelName := range targetModels {
		normalizedModel := canonicalizeModelNameForProvider(provider, modelName)
		if normalizedModel == "" {
			continue
		}
		targetSet[normalizedModel] = struct{}{}
	}
	deleteModels := make([]string, 0)
	for _, row := range existingRows {
		modelName := canonicalizeModelNameForProvider(provider, row.Model)
		if modelName == "" {
			continue
		}
		if _, ok := targetSet[modelName]; ok {
			continue
		}
		deleteModels = append(deleteModels, modelName)
	}
	if len(deleteModels) == 0 {
		return nil
	}
	if err := tx.Where("provider = ? AND model IN ?", provider, deleteModels).
		Delete(&ProviderModelPriceComponent{}).Error; err != nil {
		return err
	}
	return tx.Where("provider = ? AND model IN ?", provider, deleteModels).
		Delete(&ProviderModel{}).Error
}

func mergeProviderIDs(left []string, right []string) []string {
	seen := make(map[string]struct{}, len(left)+len(right))
	result := make([]string, 0, len(left)+len(right))
	for _, item := range left {
		provider := strings.TrimSpace(strings.ToLower(item))
		if provider == "" {
			continue
		}
		if _, exists := seen[provider]; exists {
			continue
		}
		seen[provider] = struct{}{}
		result = append(result, provider)
	}
	for _, item := range right {
		provider := strings.TrimSpace(strings.ToLower(item))
		if provider == "" {
			continue
		}
		if _, exists := seen[provider]; exists {
			continue
		}
		seen[provider] = struct{}{}
		result = append(result, provider)
	}
	return result
}
