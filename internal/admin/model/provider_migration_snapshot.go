package model

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed provider_migration_snapshot.json
var providerMigrationSnapshotJSON []byte

func LoadProviderMigrationSeeds(now int64) ([]ProviderSeed, error) {
	if len(providerMigrationSnapshotJSON) == 0 {
		return nil, fmt.Errorf("provider migration snapshot is empty")
	}
	seeds := make([]ProviderSeed, 0)
	if err := json.Unmarshal(providerMigrationSnapshotJSON, &seeds); err != nil {
		return nil, fmt.Errorf("unmarshal provider migration snapshot: %w", err)
	}
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
