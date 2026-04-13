package model

import "gorm.io/gorm"

func dropLegacyUserQuotaSnapshotColumnsWithDB(tx *gorm.DB) error {
	if tx == nil {
		return nil
	}
	if !tx.Migrator().HasTable("users") {
		return nil
	}
	if tx.Migrator().HasColumn("users", "daily_quota_limit") {
		if err := tx.Migrator().DropColumn("users", "daily_quota_limit"); err != nil {
			return err
		}
	}
	if tx.Migrator().HasColumn("users", "package_emergency_quota_limit") {
		if err := tx.Migrator().DropColumn("users", "package_emergency_quota_limit"); err != nil {
			return err
		}
	}
	return nil
}
