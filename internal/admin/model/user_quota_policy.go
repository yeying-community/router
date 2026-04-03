package model

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type UserQuotaPolicy struct {
	DailyLimit            int64
	PackageEmergencyLimit int64
	Timezone              string
}

func normalizeUserDailyQuotaLimit(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func NormalizeUserDailyQuotaLimitForWrite(value int64) int64 {
	return normalizeUserDailyQuotaLimit(value)
}

func normalizeUserPackageEmergencyQuotaLimit(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func NormalizeUserPackageEmergencyQuotaLimitForWrite(value int64) int64 {
	return normalizeUserPackageEmergencyQuotaLimit(value)
}

func normalizeUserQuotaResetTimezone(value string) string {
	return normalizeGroupQuotaResetTimezone(value)
}

func NormalizeUserQuotaResetTimezoneForWrite(value string) string {
	return normalizeUserQuotaResetTimezone(value)
}

func ValidateUserQuotaResetTimezone(value string) (string, error) {
	return ValidateGroupQuotaResetTimezone(value)
}

func GetUserQuotaPolicyWithDB(db *gorm.DB, userID string) (UserQuotaPolicy, error) {
	if db == nil {
		return UserQuotaPolicy{}, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return UserQuotaPolicy{}, fmt.Errorf("用户 ID 不能为空")
	}
	var row User
	err := db.Select("id", "daily_quota_limit", "package_emergency_quota_limit", "quota_reset_timezone").
		First(&row, "id = ?", normalizedUserID).Error
	if err != nil {
		return UserQuotaPolicy{}, err
	}
	return UserQuotaPolicy{
		DailyLimit:            normalizeUserDailyQuotaLimit(row.DailyQuotaLimit),
		PackageEmergencyLimit: normalizeUserPackageEmergencyQuotaLimit(row.PackageEmergencyQuotaLimit),
		Timezone:              normalizeUserQuotaResetTimezone(row.QuotaResetTimezone),
	}, nil
}

func GetUserQuotaPolicy(userID string) (UserQuotaPolicy, error) {
	return GetUserQuotaPolicyWithDB(DB, userID)
}

func normalizeUserQuotaDate(rawDate string, timezone string) (string, error) {
	normalized := strings.TrimSpace(rawDate)
	if normalized == "" {
		return businessDateByTimezone(time.Now(), timezone), nil
	}
	parsed, err := time.Parse("2006-01-02", normalized)
	if err != nil {
		return "", fmt.Errorf("日期格式错误，应为 YYYY-MM-DD")
	}
	return parsed.Format("2006-01-02"), nil
}

func businessMonthByTimezone(now time.Time, timezone string) string {
	locationName := normalizeUserQuotaResetTimezone(timezone)
	location, err := time.LoadLocation(locationName)
	if err != nil {
		location = time.FixedZone(DefaultGroupQuotaResetTimezone, 8*3600)
	}
	return now.In(location).Format("2006-01")
}

func normalizeUserQuotaMonth(rawMonth string, timezone string) (string, error) {
	normalized := strings.TrimSpace(rawMonth)
	if normalized == "" {
		return businessMonthByTimezone(time.Now(), timezone), nil
	}
	parsed, err := time.Parse("2006-01", normalized)
	if err != nil {
		return "", fmt.Errorf("月份格式错误，应为 YYYY-MM")
	}
	return parsed.Format("2006-01"), nil
}
