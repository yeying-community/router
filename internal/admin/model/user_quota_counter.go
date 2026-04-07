package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const UserQuotaCountersTableName = "user_quota_counters"

const (
	UserQuotaCounterTypeDaily            = "daily"
	UserQuotaCounterTypePackageEmergency = "monthly_emergency"
)

type UserQuotaCounter struct {
	UserID        string `json:"user_id" gorm:"primaryKey;type:char(36)"`
	CounterType   string `json:"counter_type" gorm:"primaryKey;type:varchar(32)"`
	PeriodKey     string `json:"period_key" gorm:"primaryKey;type:varchar(32)"`
	ReservedQuota int64  `json:"reserved_quota" gorm:"type:bigint;not null;default:0"`
	ConsumedQuota int64  `json:"consumed_quota" gorm:"type:bigint;not null;default:0"`
	UpdatedAt     int64  `json:"updated_at" gorm:"bigint;index"`
}

func (UserQuotaCounter) TableName() string {
	return UserQuotaCountersTableName
}

type UserDailyQuotaSnapshot struct {
	UserID         string `json:"user_id"`
	BizDate        string `json:"biz_date"`
	Limit          int64  `json:"limit"`
	ConsumedQuota  int64  `json:"consumed_quota"`
	ReservedQuota  int64  `json:"reserved_quota"`
	RemainingQuota int64  `json:"remaining_quota"`
	Unlimited      bool   `json:"unlimited"`
	Timezone       string `json:"timezone"`
	UpdatedAt      int64  `json:"updated_at"`
}

type UserPackageEmergencyQuotaSnapshot struct {
	UserID         string `json:"user_id"`
	BizMonth       string `json:"biz_month"`
	Limit          int64  `json:"limit"`
	ConsumedQuota  int64  `json:"consumed_quota"`
	ReservedQuota  int64  `json:"reserved_quota"`
	RemainingQuota int64  `json:"remaining_quota"`
	Enabled        bool   `json:"enabled"`
	Timezone       string `json:"timezone"`
	UpdatedAt      int64  `json:"updated_at"`
}

type UserQuotaSummary struct {
	UserID           string                            `json:"user_id"`
	Daily            UserDailyQuotaSnapshot            `json:"daily"`
	PackageEmergency UserPackageEmergencyQuotaSnapshot `json:"package_emergency"`
}

func ensureUserQuotaCounterWithDB(tx *gorm.DB, userID string, counterType string, periodKey string) error {
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&UserQuotaCounter{
		UserID:      userID,
		CounterType: counterType,
		PeriodKey:   periodKey,
	}).Error
}

func loadUserQuotaCounterForUpdateWithDB(tx *gorm.DB, userID string, counterType string, periodKey string) (UserQuotaCounter, error) {
	if err := ensureUserQuotaCounterWithDB(tx, userID, counterType, periodKey); err != nil {
		return UserQuotaCounter{}, err
	}
	counter := UserQuotaCounter{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND counter_type = ? AND period_key = ?", userID, counterType, periodKey).
		Take(&counter).Error
	return counter, err
}

func GetUserDailyQuotaSnapshotWithDB(db *gorm.DB, userID string, bizDate string) (UserDailyQuotaSnapshot, error) {
	if db == nil {
		return UserDailyQuotaSnapshot{}, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return UserDailyQuotaSnapshot{}, fmt.Errorf("用户 ID 不能为空")
	}
	policy, err := GetUserQuotaPolicyWithDB(db, normalizedUserID)
	if err != nil {
		return UserDailyQuotaSnapshot{}, err
	}
	normalizedBizDate, err := normalizeUserQuotaDate(bizDate, policy.Timezone)
	if err != nil {
		return UserDailyQuotaSnapshot{}, err
	}
	counter := UserQuotaCounter{}
	err = db.Where("user_id = ? AND counter_type = ? AND period_key = ?", normalizedUserID, UserQuotaCounterTypeDaily, normalizedBizDate).First(&counter).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return UserDailyQuotaSnapshot{}, err
	}
	if err == gorm.ErrRecordNotFound {
		counter = UserQuotaCounter{UserID: normalizedUserID, CounterType: UserQuotaCounterTypeDaily, PeriodKey: normalizedBizDate}
	}
	consumed := counter.ConsumedQuota
	if consumed < 0 {
		consumed = 0
	}
	reserved := counter.ReservedQuota
	if reserved < 0 {
		reserved = 0
	}
	unlimited := policy.DailyLimit <= 0
	remaining := int64(0)
	if !unlimited {
		remaining = policy.DailyLimit - consumed - reserved
		if remaining < 0 {
			remaining = 0
		}
	}
	return UserDailyQuotaSnapshot{
		UserID:         normalizedUserID,
		BizDate:        normalizedBizDate,
		Limit:          policy.DailyLimit,
		ConsumedQuota:  consumed,
		ReservedQuota:  reserved,
		RemainingQuota: remaining,
		Unlimited:      unlimited,
		Timezone:       policy.Timezone,
		UpdatedAt:      counter.UpdatedAt,
	}, nil
}

func GetUserDailyQuotaSnapshot(userID string, bizDate string) (UserDailyQuotaSnapshot, error) {
	return GetUserDailyQuotaSnapshotWithDB(DB, userID, bizDate)
}

func GetUserPackageEmergencyQuotaSnapshotWithDB(db *gorm.DB, userID string, bizMonth string) (UserPackageEmergencyQuotaSnapshot, error) {
	if db == nil {
		return UserPackageEmergencyQuotaSnapshot{}, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return UserPackageEmergencyQuotaSnapshot{}, fmt.Errorf("用户 ID 不能为空")
	}
	policy, err := GetUserQuotaPolicyWithDB(db, normalizedUserID)
	if err != nil {
		return UserPackageEmergencyQuotaSnapshot{}, err
	}
	normalizedBizMonth, err := normalizeUserQuotaMonth(bizMonth, policy.Timezone)
	if err != nil {
		return UserPackageEmergencyQuotaSnapshot{}, err
	}
	counter := UserQuotaCounter{}
	err = db.Where("user_id = ? AND counter_type = ? AND period_key = ?", normalizedUserID, UserQuotaCounterTypePackageEmergency, normalizedBizMonth).First(&counter).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return UserPackageEmergencyQuotaSnapshot{}, err
	}
	if err == gorm.ErrRecordNotFound {
		counter = UserQuotaCounter{UserID: normalizedUserID, CounterType: UserQuotaCounterTypePackageEmergency, PeriodKey: normalizedBizMonth}
	}
	consumed := counter.ConsumedQuota
	if consumed < 0 {
		consumed = 0
	}
	reserved := counter.ReservedQuota
	if reserved < 0 {
		reserved = 0
	}
	enabled := policy.PackageEmergencyLimit > 0
	remaining := int64(0)
	if enabled {
		remaining = policy.PackageEmergencyLimit - consumed - reserved
		if remaining < 0 {
			remaining = 0
		}
	}
	return UserPackageEmergencyQuotaSnapshot{
		UserID:         normalizedUserID,
		BizMonth:       normalizedBizMonth,
		Limit:          policy.PackageEmergencyLimit,
		ConsumedQuota:  consumed,
		ReservedQuota:  reserved,
		RemainingQuota: remaining,
		Enabled:        enabled,
		Timezone:       policy.Timezone,
		UpdatedAt:      counter.UpdatedAt,
	}, nil
}

func GetUserPackageEmergencyQuotaSnapshot(userID string, bizMonth string) (UserPackageEmergencyQuotaSnapshot, error) {
	return GetUserPackageEmergencyQuotaSnapshotWithDB(DB, userID, bizMonth)
}

func GetUserQuotaSummaryWithDB(db *gorm.DB, userID string, bizDate string, bizMonth string) (UserQuotaSummary, error) {
	daily, err := GetUserDailyQuotaSnapshotWithDB(db, userID, bizDate)
	if err != nil {
		return UserQuotaSummary{}, err
	}
	packageEmergency, err := GetUserPackageEmergencyQuotaSnapshotWithDB(db, userID, bizMonth)
	if err != nil {
		return UserQuotaSummary{}, err
	}
	return UserQuotaSummary{
		UserID:           strings.TrimSpace(userID),
		Daily:            daily,
		PackageEmergency: packageEmergency,
	}, nil
}

func GetUserQuotaSummary(userID string, bizDate string, bizMonth string) (UserQuotaSummary, error) {
	return GetUserQuotaSummaryWithDB(DB, userID, bizDate, bizMonth)
}
