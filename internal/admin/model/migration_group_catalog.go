package model

import (
	"gorm.io/gorm"
)

func runGroupCatalogMigrationsWithDB(db *gorm.DB) error {
	return db.AutoMigrate(&GroupCatalog{})
}
