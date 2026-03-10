package model

import (
	"fmt"
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
		&ChannelTest{},
		&Token{},
		&Redemption{},
		&Ability{},
		&Option{},
		&Provider{},
		&ProviderModel{},
		&ChannelProtocolCatalog{},
		&GroupCatalog{},
		&Log{},
	); err != nil {
		return err
	}
	if err := dropLegacyChannelTestPayloadColumnsWithDB(tx); err != nil {
		return err
	}
	if err := dropLegacyChannelModelTestColumnsWithDB(tx); err != nil {
		return err
	}

	if err := ensureChannelProtocolCatalogSeededWithDB(tx); err != nil {
		return err
	}
	if err := ensureProviderCatalogSeededWithDB(tx); err != nil {
		return err
	}
	return nil
}

func runLogBaselineMigrationWithDB(tx *gorm.DB) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	return tx.AutoMigrate(&Log{})
}

func dropLegacyChannelTestPayloadColumnsWithDB(tx *gorm.DB) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	for _, column := range []string{"input_payload", "output_payload"} {
		if !tx.Migrator().HasColumn(ChannelTestsTableName, column) {
			continue
		}
		if err := tx.Migrator().DropColumn(ChannelTestsTableName, column); err != nil {
			return err
		}
	}
	return nil
}

func dropLegacyChannelModelTestColumnsWithDB(tx *gorm.DB) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	for _, column := range []string{"test_status", "test_round", "tested_at", "latency_ms"} {
		if !tx.Migrator().HasColumn(ChannelModelsTableName, column) {
			continue
		}
		if err := tx.Migrator().DropColumn(ChannelModelsTableName, column); err != nil {
			return err
		}
	}
	return nil
}
