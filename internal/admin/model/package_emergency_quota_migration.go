package model

import "gorm.io/gorm"

func migratePackageEmergencyQuotaColumnsWithDB(tx *gorm.DB) error {
	if tx == nil {
		return nil
	}

	targets := []struct {
		table   string
		model   any
		current string
		legacy  string
	}{
		{table: "users", model: &User{}, current: "package_emergency_quota_limit", legacy: "monthly_emergency_quota_limit"},
		{table: ServicePackagesTableName, model: &ServicePackage{}, current: "package_emergency_quota_limit", legacy: "monthly_emergency_quota_limit"},
		{table: UserPackageSubscriptionsTableName, model: &UserPackageSubscription{}, current: "package_emergency_quota_limit", legacy: "monthly_emergency_quota_limit"},
	}

	for _, target := range targets {
		hasCurrent := tx.Migrator().HasColumn(target.model, target.current)
		if !hasCurrent {
			if err := tx.Exec(
				"ALTER TABLE " + target.table + " ADD COLUMN " + target.current + " bigint NOT NULL DEFAULT 0",
			).Error; err != nil {
				return err
			}
		}

		hasLegacy := tx.Migrator().HasColumn(target.model, target.legacy)
		if !hasLegacy {
			continue
		}

		if err := tx.Exec(
			"UPDATE " + target.table + " SET " + target.current + " = " + target.legacy + " WHERE COALESCE(" + target.current + ", 0) = 0 AND COALESCE(" + target.legacy + ", 0) <> 0",
		).Error; err != nil {
			return err
		}

		if err := tx.Migrator().DropColumn(target.model, target.legacy); err != nil {
			return err
		}
	}

	return nil
}
