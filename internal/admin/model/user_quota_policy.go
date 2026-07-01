package model

import (
	"fmt"
	"strings"
	"time"
)

func normalizeUserQuotaResetTimezone(value string) string {
	return normalizeGroupQuotaResetTimezone(value)
}

func NormalizeUserQuotaResetTimezoneForWrite(value string) string {
	return normalizeUserQuotaResetTimezone(value)
}

func ValidateUserQuotaResetTimezone(value string) (string, error) {
	return ValidateGroupQuotaResetTimezone(value)
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
