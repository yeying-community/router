package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type channelModelProviderBackfillRow struct {
	ChannelID     string `gorm:"column:channel_id"`
	Model         string `gorm:"column:model"`
	UpstreamModel string `gorm:"column:upstream_model"`
	Provider      string `gorm:"column:provider"`
}

func backfillChannelModelProviderFromCatalogWithDB(tx *gorm.DB) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	if err := tx.AutoMigrate(&ChannelModel{}, &ProviderModel{}); err != nil {
		return err
	}

	rows := make([]channelModelProviderBackfillRow, 0)
	if err := tx.
		Model(&ChannelModel{}).
		Select("channel_id", "model", "upstream_model", "provider").
		Order("channel_id asc, sort_order asc, model asc").
		Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return syncGroupModelProvidersAfterChannelProviderBackfillWithDB(tx)
	}

	candidates := make([]string, 0, len(rows)*2)
	for _, row := range rows {
		if modelName := strings.TrimSpace(row.Model); modelName != "" {
			candidates = append(candidates, modelName)
		}
		if upstreamModel := strings.TrimSpace(row.UpstreamModel); upstreamModel != "" {
			candidates = append(candidates, upstreamModel)
		}
	}
	providerByModel, err := LoadUniqueProviderMapByModelsWithDB(tx, candidates)
	if err != nil {
		return err
	}

	for _, row := range rows {
		resolvedProvider := ResolveProviderFromCatalogMap(providerByModel, row.Model, row.UpstreamModel)
		resolvedProvider = NormalizeGroupModelProviderValue(resolvedProvider)
		currentProvider := NormalizeGroupModelProviderValue(row.Provider)
		if currentProvider == resolvedProvider {
			continue
		}
		if err := tx.Model(&ChannelModel{}).
			Where("channel_id = ? AND model = ?", strings.TrimSpace(row.ChannelID), strings.TrimSpace(row.Model)).
			Update("provider", resolvedProvider).Error; err != nil {
			return err
		}
	}
	return syncGroupModelProvidersAfterChannelProviderBackfillWithDB(tx)
}

func syncGroupModelProvidersAfterChannelProviderBackfillWithDB(tx *gorm.DB) error {
	groupIDs := make([]string, 0)
	groupCol := `"group"`
	if err := tx.Model(&Ability{}).
		Distinct(groupCol).
		Where("channel_id <> ''").
		Pluck(groupCol, &groupIDs).Error; err != nil {
		return err
	}
	return SyncGroupModelProvidersForGroupsWithDB(tx, groupIDs...)
}
