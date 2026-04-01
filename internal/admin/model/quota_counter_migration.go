package model

import "gorm.io/gorm"

const (
	legacyGroupQuotaCountersTableName                = "user_group_daily_quota_counters"
	legacyUserDailyQuotaCountersTableName            = "user_daily_quota_counters"
	legacyUserMonthlyEmergencyQuotaCountersTableName = "user_monthly_emergency_quota_counters"
)

func migrateLegacyQuotaCountersToGenericWithDB(tx *gorm.DB) error {
	if tx == nil {
		return gorm.ErrInvalidDB
	}
	if err := tx.AutoMigrate(&GroupQuotaCounter{}, &UserQuotaCounter{}); err != nil {
		return err
	}
	if tx.Migrator().HasTable(legacyGroupQuotaCountersTableName) {
		if err := tx.Exec(
			`INSERT INTO group_quota_counters (group_id, user_id, counter_type, period_key, reserved_quota, consumed_quota, updated_at)
			 SELECT group_id, user_id, ?, biz_date, reserved_quota, consumed_quota, updated_at
			 FROM user_group_daily_quota_counters
			 ON CONFLICT (group_id, user_id, counter_type, period_key)
			 DO UPDATE SET
			   reserved_quota = EXCLUDED.reserved_quota,
			   consumed_quota = EXCLUDED.consumed_quota,
			   updated_at = EXCLUDED.updated_at`,
			GroupQuotaCounterTypeDaily,
		).Error; err != nil {
			return err
		}
		if err := tx.Migrator().DropTable(legacyGroupQuotaCountersTableName); err != nil {
			return err
		}
	}
	if tx.Migrator().HasTable(legacyUserDailyQuotaCountersTableName) {
		if err := tx.Exec(
			`INSERT INTO user_quota_counters (user_id, counter_type, period_key, reserved_quota, consumed_quota, updated_at)
			 SELECT user_id, ?, biz_date, reserved_quota, consumed_quota, updated_at
			 FROM user_daily_quota_counters
			 ON CONFLICT (user_id, counter_type, period_key)
			 DO UPDATE SET
			   reserved_quota = EXCLUDED.reserved_quota,
			   consumed_quota = EXCLUDED.consumed_quota,
			   updated_at = EXCLUDED.updated_at`,
			UserQuotaCounterTypeDaily,
		).Error; err != nil {
			return err
		}
		if err := tx.Migrator().DropTable(legacyUserDailyQuotaCountersTableName); err != nil {
			return err
		}
	}
	if tx.Migrator().HasTable(legacyUserMonthlyEmergencyQuotaCountersTableName) {
		if err := tx.Exec(
			`INSERT INTO user_quota_counters (user_id, counter_type, period_key, reserved_quota, consumed_quota, updated_at)
			 SELECT user_id, ?, biz_month, reserved_quota, consumed_quota, updated_at
			 FROM user_monthly_emergency_quota_counters
			 ON CONFLICT (user_id, counter_type, period_key)
			 DO UPDATE SET
			   reserved_quota = EXCLUDED.reserved_quota,
			   consumed_quota = EXCLUDED.consumed_quota,
			   updated_at = EXCLUDED.updated_at`,
			UserQuotaCounterTypeMonthlyEmergency,
		).Error; err != nil {
			return err
		}
		if err := tx.Migrator().DropTable(legacyUserMonthlyEmergencyQuotaCountersTableName); err != nil {
			return err
		}
	}
	return nil
}
