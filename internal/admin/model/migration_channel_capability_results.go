package model

import (
	"fmt"

	"gorm.io/gorm"
)

func runChannelCapabilityResultsMigrationWithDB(tx *gorm.DB) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	return tx.AutoMigrate(&ChannelCapabilityResult{})
}
