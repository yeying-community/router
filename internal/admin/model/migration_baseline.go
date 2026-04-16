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
		&TopupOrder{},
		&AsyncTask{},
		&UserTask{},
		&Token{},
		&Redemption{},
		&Ability{},
		&GroupModelProvider{},
		&Option{},
		&Provider{},
		&ProviderModel{},
		&ProviderModelPriceComponent{},
		&ChannelProtocolCatalog{},
		&GroupCatalog{},
		&ServicePackage{},
		&UserPackageSubscription{},
		&GroupQuotaCounter{},
		&UserQuotaCounter{},
		&Log{},
	); err != nil {
		return err
	}

	if err := ensureChannelProtocolCatalogSeededWithDB(tx); err != nil {
		return err
	}
	if err := syncDefaultProviderCatalogWithDB(tx); err != nil {
		return err
	}
	if err := tx.Exec(
		"UPDATE users SET has_password = TRUE WHERE has_password = FALSE AND wallet_address IS NULL AND COALESCE(password, '') <> ''",
	).Error; err != nil {
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
