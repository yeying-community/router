package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func migrateGroupModelProvidersWithDB(tx *gorm.DB) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	if err := tx.AutoMigrate(&GroupModelProvider{}); err != nil {
		return err
	}

	if err := tx.Where("1 = 1").Delete(&GroupModelProvider{}).Error; err != nil {
		return err
	}

	groupIDs := make([]string, 0)
	groupCol := `"group"`
	if err := tx.Model(&Ability{}).
		Distinct(groupCol).
		Where("channel_id <> ''").
		Pluck(groupCol, &groupIDs).Error; err != nil {
		return err
	}
	normalizedGroupIDs := make([]string, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		normalized := strings.TrimSpace(groupID)
		if normalized == "" {
			continue
		}
		normalizedGroupIDs = append(normalizedGroupIDs, normalized)
	}
	if len(normalizedGroupIDs) == 0 {
		return nil
	}
	if err := SyncGroupModelProvidersForGroupsWithDB(tx, normalizedGroupIDs...); err != nil {
		return fmt.Errorf("group_model_providers backfill failed: %w", err)
	}
	return nil
}
