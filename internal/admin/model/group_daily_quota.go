package model

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	DefaultGroupQuotaResetTimezone = "Asia/Shanghai"
)

func normalizeGroupDailyQuotaLimit(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func normalizeGroupQuotaResetTimezone(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return DefaultGroupQuotaResetTimezone
	}
	if _, err := time.LoadLocation(normalized); err != nil {
		return DefaultGroupQuotaResetTimezone
	}
	return normalized
}

func ValidateGroupQuotaResetTimezone(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return DefaultGroupQuotaResetTimezone, nil
	}
	if _, err := time.LoadLocation(normalized); err != nil {
		return "", fmt.Errorf("重置时区不合法")
	}
	return normalized, nil
}

func syncGroupRuntimeCachesWithDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	return syncGroupBillingRatiosRuntimeWithDB(db)
}

func SyncGroupRuntimeCachesWithDB(db *gorm.DB) error {
	return syncGroupRuntimeCachesWithDB(db)
}
