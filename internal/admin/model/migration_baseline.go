package model

import (
	"fmt"
	"strings"

	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"gorm.io/gorm"
)

func runMainBaselineMigrationWithDB(tx *gorm.DB) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}

	if err := syncRedemptionCodeColumnWithDB(tx); err != nil {
		return err
	}

	if err := tx.AutoMigrate(
		&User{},
		&Channel{},
		&ChannelModel{},
		&ChannelCapabilityResult{},
		&Token{},
		&Redemption{},
		&Ability{},
		&Option{},
		&ModelProvider{},
		&ModelProviderModel{},
		&ChannelProtocolCatalog{},
		&GroupCatalog{},
		&Log{},
	); err != nil {
		return err
	}

	if err := syncChannelProtocolsWithDB(tx); err != nil {
		return err
	}
	if err := syncChannelProtocolCatalogWithDB(tx); err != nil {
		return err
	}
	if err := syncModelProviderCatalogWithDB(tx); err != nil {
		return err
	}
	if err := syncChannelModelTypesWithDB(tx); err != nil {
		return err
	}
	if err := syncAbilityUpstreamModelsWithDB(tx); err != nil {
		return err
	}
	return syncChannelTestModelsWithDB(tx)
}

func syncRedemptionCodeColumnWithDB(tx *gorm.DB) error {
	if tx == nil || !tx.Migrator().HasTable(&Redemption{}) {
		return nil
	}

	hasKey, err := hasTableColumn(tx, "redemptions", "key")
	if err != nil {
		return err
	}
	hasCode, err := hasTableColumn(tx, "redemptions", "code")
	if err != nil {
		return err
	}

	switch {
	case hasKey && !hasCode:
		return tx.Exec(`ALTER TABLE redemptions RENAME COLUMN "key" TO code`).Error
	case hasKey && hasCode:
		if err := tx.Exec(`UPDATE redemptions SET code = "key" WHERE COALESCE(code, '') = ''`).Error; err != nil {
			return err
		}
		return tx.Exec(`ALTER TABLE redemptions DROP COLUMN "key"`).Error
	default:
		return nil
	}
}

func hasTableColumn(tx *gorm.DB, tableName string, columnName string) (bool, error) {
	type result struct {
		Count int64
	}
	row := result{}
	if err := tx.Raw(
		`SELECT COUNT(*) AS count FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
		tableName,
		columnName,
	).Scan(&row).Error; err != nil {
		return false, err
	}
	return row.Count > 0, nil
}

func runLogBaselineMigrationWithDB(tx *gorm.DB) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	return tx.AutoMigrate(&Log{})
}

func syncChannelProtocolsWithDB(tx *gorm.DB) error {
	rows := make([]Channel, 0)
	if err := tx.Select("id", "protocol").Find(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		normalized := relaychannel.NormalizeProtocolName(row.Protocol)
		if normalized == "" {
			normalized = "openai"
		}
		current := strings.TrimSpace(strings.ToLower(row.Protocol))
		if current == normalized {
			continue
		}
		if err := tx.Model(&Channel{}).
			Where("id = ?", row.Id).
			Update("protocol", normalized).Error; err != nil {
			return err
		}
	}
	return nil
}

func syncChannelTestModelsWithDB(db *gorm.DB) error {
	channels := make([]Channel, 0)
	if err := db.Select("id").Find(&channels).Error; err != nil {
		return err
	}

	for _, channel := range channels {
		if err := EnsureChannelTestModelWithDB(db, channel.Id); err != nil {
			return err
		}
	}
	return nil
}

func syncChannelModelTypesWithDB(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&ChannelModel{}) {
		return nil
	}

	rows := make([]ChannelModel, 0)
	if err := db.Find(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		channelProtocol, err := loadChannelProtocolByChannelIDWithDB(db, row.ChannelId)
		if err != nil {
			return err
		}
		normalizedType := resolveChannelModelType(row.Type, channelProtocol, row.UpstreamModel, row.Model)
		normalizedPriceUnit := normalizeChannelModelPriceUnit(row.PriceUnit, normalizedType, channelProtocol, row.UpstreamModel, row.Model)
		currentType := strings.TrimSpace(strings.ToLower(row.Type))
		currentPriceUnit := strings.TrimSpace(strings.ToLower(row.PriceUnit))
		if currentType == normalizedType && currentPriceUnit == normalizedPriceUnit {
			continue
		}
		updates := map[string]any{
			"type":       normalizedType,
			"price_unit": normalizedPriceUnit,
		}
		if err := db.Model(&ChannelModel{}).
			Where("channel_id = ? AND model = ?", row.ChannelId, row.Model).
			Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func syncAbilityUpstreamModelsWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	return db.Exec(`UPDATE group_model_channels SET upstream_model = model WHERE COALESCE(upstream_model, '') = ''`).Error
}
