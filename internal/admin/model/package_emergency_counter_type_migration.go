package model

import "gorm.io/gorm"

const legacyUserQuotaCounterTypeMonthlyEmergency = "monthly_emergency"

func migrateUserQuotaCounterTypePackageEmergencyWithDB(tx *gorm.DB) error {
	if tx == nil {
		return gorm.ErrInvalidDB
	}
	if err := tx.AutoMigrate(&UserQuotaCounter{}); err != nil {
		return err
	}
	return tx.Exec(
		`UPDATE user_quota_counters
		 SET counter_type = ?
		 WHERE counter_type = ?`,
		UserQuotaCounterTypePackageEmergency,
		legacyUserQuotaCounterTypeMonthlyEmergency,
	).Error
}
