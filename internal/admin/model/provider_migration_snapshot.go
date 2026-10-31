package model

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed provider_migration_snapshot.json
var providerMigrationSnapshotJSON []byte

//go:embed provider_catalog_expansion_202609.json
var providerCatalogExpansion202609JSON []byte

//go:embed provider_catalog_refresh_20261025.json
var providerCatalogRefresh20261025JSON []byte

//go:embed provider_catalog_correction_20261026.json
var providerCatalogCorrection20261026JSON []byte

//go:embed provider_catalog_refresh_20261029.json
var providerCatalogRefresh20261029JSON []byte

func LoadProviderMigrationSeeds(now int64) ([]ProviderSeed, error) {
	snapshotSeeds, err := unmarshalProviderMigrationSeeds(providerMigrationSnapshotJSON, "provider migration snapshot")
	if err != nil {
		return nil, err
	}
	expansionSeeds, err := unmarshalProviderMigrationSeeds(providerCatalogExpansion202609JSON, "provider catalog expansion 202609")
	if err != nil {
		return nil, err
	}
	refreshSeeds, err := unmarshalProviderMigrationSeeds(providerCatalogRefresh20261025JSON, "provider catalog refresh 20261025")
	if err != nil {
		return nil, err
	}
	correctionSeeds, err := unmarshalProviderMigrationSeeds(providerCatalogCorrection20261026JSON, "provider catalog correction 20261026")
	if err != nil {
		return nil, err
	}
	latestRefreshSeeds, err := unmarshalProviderMigrationSeeds(providerCatalogRefresh20261029JSON, "provider catalog refresh 20261029")
	if err != nil {
		return nil, err
	}
	seeds := mergeProviderMigrationSeedSets(snapshotSeeds, expansionSeeds, refreshSeeds, correctionSeeds, latestRefreshSeeds)
	normalized := make([]ProviderSeed, 0, len(seeds))
	for _, seed := range seeds {
		seed.ModelDetails = normalizeProviderMigrationSeedModelDetails(
			seed.Provider,
			resetProviderMigrationDetailTimestamps(seed.ModelDetails),
			now,
		)
		normalized = append(normalized, seed)
	}
	return normalized, nil
}

func unmarshalProviderMigrationSeeds(data []byte, label string) ([]ProviderSeed, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%s is empty", label)
	}
	seeds := make([]ProviderSeed, 0)
	if err := json.Unmarshal(data, &seeds); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", label, err)
	}
	return seeds, nil
}

// mergeProviderMigrationSeedSets builds one complete provider catalog from the
// baseline snapshot and later incremental reviews. Provider metadata is patched
// only when a later file supplies a non-zero value. A repeated model is a full
// replacement, which keeps lifecycle and pricing changes explicit and avoids
// accidentally retaining stale fields from the earlier snapshot.
func mergeProviderMigrationSeedSets(seedSets ...[]ProviderSeed) []ProviderSeed {
	merged := make([]ProviderSeed, 0)
	providerIndex := make(map[string]int)
	modelIndexes := make(map[string]map[string]int)

	for _, seeds := range seedSets {
		for _, rawSeed := range seeds {
			provider := strings.TrimSpace(strings.ToLower(rawSeed.Provider))
			if provider == "" {
				continue
			}
			idx, exists := providerIndex[provider]
			if !exists {
				next := rawSeed
				next.Provider = provider
				next.ModelDetails = append([]ProviderModelDetail(nil), rawSeed.ModelDetails...)
				providerIndex[provider] = len(merged)
				merged = append(merged, next)
				idx = len(merged) - 1
				indexes := make(map[string]int, len(next.ModelDetails))
				for modelIdx, detail := range next.ModelDetails {
					modelName := canonicalizeModelNameForProvider(provider, detail.Model)
					if modelName != "" {
						indexes[modelName] = modelIdx
					}
				}
				modelIndexes[provider] = indexes
				continue
			}

			seed := merged[idx]
			if strings.TrimSpace(rawSeed.Name) != "" {
				seed.Name = rawSeed.Name
			}
			if strings.TrimSpace(rawSeed.BaseURL) != "" {
				seed.BaseURL = rawSeed.BaseURL
			}
			if strings.TrimSpace(rawSeed.OfficialURL) != "" {
				seed.OfficialURL = rawSeed.OfficialURL
			}
			if strings.TrimSpace(rawSeed.PricingURL) != "" {
				seed.PricingURL = rawSeed.PricingURL
			}
			if rawSeed.SortOrder > 0 {
				seed.SortOrder = rawSeed.SortOrder
			}
			indexes := modelIndexes[provider]
			for _, detail := range rawSeed.ModelDetails {
				modelName := canonicalizeModelNameForProvider(provider, detail.Model)
				if modelName == "" {
					continue
				}
				if modelIdx, ok := indexes[modelName]; ok {
					seed.ModelDetails[modelIdx] = detail
					continue
				}
				indexes[modelName] = len(seed.ModelDetails)
				seed.ModelDetails = append(seed.ModelDetails, detail)
			}
			merged[idx] = seed
		}
	}

	return merged
}

func resetProviderMigrationDetailTimestamps(details []ProviderModelDetail) []ProviderModelDetail {
	cloned := make([]ProviderModelDetail, 0, len(details))
	for _, detail := range details {
		next := detail
		next.UpdatedAt = 0
		if len(next.PriceComponents) > 0 {
			priceComponents := make([]ProviderModelPriceComponentDetail, 0, len(next.PriceComponents))
			for _, component := range next.PriceComponents {
				nextComponent := component
				nextComponent.UpdatedAt = 0
				priceComponents = append(priceComponents, nextComponent)
			}
			next.PriceComponents = priceComponents
		}
		cloned = append(cloned, next)
	}
	return cloned
}
