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

	if err := tx.AutoMigrate(
		&User{},
		&Channel{},
		&ChannelModel{},
		&ChannelCapabilityProfile{},
		&ChannelCapabilityResult{},
		&Token{},
		&Redemption{},
		&Ability{},
		&Log{},
		&Option{},
		&ModelProvider{},
		&ModelProviderModel{},
		&ClientProfile{},
		&ChannelProtocolCatalog{},
		&GroupCatalog{},
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
	if err := syncClientProfilesWithDB(tx); err != nil {
		return err
	}
	return syncChannelTestModelsWithDB(tx)
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
