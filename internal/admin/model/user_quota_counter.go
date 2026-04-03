package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const UserQuotaCountersTableName = "user_quota_counters"

const (
	UserQuotaCounterTypeDaily            = "daily"
	UserQuotaCounterTypeMonthlyEmergency = "monthly_emergency"
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

type UserQuotaReservation struct {
	UserID                 string
	DailyBizDate           string
	DailyReservedQuota     int64
	EmergencyBizMonth      string
	EmergencyReservedQuota int64
}

func (reservation UserQuotaReservation) Active() bool {
	return strings.TrimSpace(reservation.UserID) != "" &&
		(reservation.DailyReservedQuota > 0 || reservation.EmergencyReservedQuota > 0)
}

type UserQuotaUsage struct {
	DailyQuotaUsed     int64 `json:"daily_quota_used"`
	EmergencyQuotaUsed int64 `json:"emergency_quota_used"`
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

type UserMonthlyEmergencyQuotaSnapshot struct {
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
	MonthlyEmergency UserMonthlyEmergencyQuotaSnapshot `json:"monthly_emergency"`
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

func splitUserQuotaConsumption(totalQuota int64, dailyCapacity int64, emergencyCapacity int64, dailyUnlimited bool, emergencyEnabled bool) UserQuotaUsage {
	if totalQuota <= 0 {
		return UserQuotaUsage{}
	}
	if dailyUnlimited {
		return UserQuotaUsage{DailyQuotaUsed: totalQuota}
	}
	if dailyCapacity < 0 {
		dailyCapacity = 0
	}
	if emergencyCapacity < 0 {
		emergencyCapacity = 0
	}
	usage := UserQuotaUsage{}
	usage.DailyQuotaUsed = minInt64(totalQuota, dailyCapacity)
	remain := totalQuota - usage.DailyQuotaUsed
	if remain <= 0 {
		return usage
	}
	if emergencyEnabled {
		usage.EmergencyQuotaUsed = minInt64(remain, emergencyCapacity)
		remain -= usage.EmergencyQuotaUsed
		if remain > 0 {
			usage.EmergencyQuotaUsed += remain
		}
		return usage
	}
	usage.DailyQuotaUsed += remain
	return usage
}

func minInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func ReserveUserQuotaWithDB(db *gorm.DB, userID string, quota int64) (UserQuotaReservation, bool, string, error) {
	if db == nil {
		return UserQuotaReservation{}, false, "", fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	normalizedQuota := quota
	if normalizedQuota < 0 {
		normalizedQuota = 0
	}
	if normalizedUserID == "" || normalizedQuota <= 0 {
		return UserQuotaReservation{}, true, "", nil
	}
	policy, err := GetUserQuotaPolicyWithDB(db, normalizedUserID)
	if err != nil {
		return UserQuotaReservation{}, false, "", err
	}
	if policy.DailyLimit <= 0 {
		return UserQuotaReservation{}, true, "", nil
	}
	now := time.Now()
	reservation := UserQuotaReservation{
		UserID:            normalizedUserID,
		DailyBizDate:      businessDateByTimezone(now, policy.Timezone),
		EmergencyBizMonth: businessMonthByTimezone(now, policy.Timezone),
	}
	allowed := true
	denyMessage := ""
	err = db.Transaction(func(tx *gorm.DB) error {
		dailyCounter, err := loadUserQuotaCounterForUpdateWithDB(tx, normalizedUserID, UserQuotaCounterTypeDaily, reservation.DailyBizDate)
		if err != nil {
			return err
		}
		dailyRemaining := policy.DailyLimit - dailyCounter.ConsumedQuota - dailyCounter.ReservedQuota
		if dailyRemaining < 0 {
			dailyRemaining = 0
		}
		reservation.DailyReservedQuota = minInt64(normalizedQuota, dailyRemaining)
		remainingNeed := normalizedQuota - reservation.DailyReservedQuota
		if remainingNeed > 0 {
			if policy.PackageEmergencyLimit <= 0 {
				allowed = false
				denyMessage = "当前用户今日额度已达上限，请明日再试"
				return nil
			}
			emergencyCounter, err := loadUserQuotaCounterForUpdateWithDB(tx, normalizedUserID, UserQuotaCounterTypeMonthlyEmergency, reservation.EmergencyBizMonth)
			if err != nil {
				return err
			}
			emergencyRemaining := policy.PackageEmergencyLimit - emergencyCounter.ConsumedQuota - emergencyCounter.ReservedQuota
			if emergencyRemaining < 0 {
				emergencyRemaining = 0
			}
			if emergencyRemaining < remainingNeed {
				allowed = false
				denyMessage = "当前用户今日额度已达上限，且套餐应急额度已用尽"
				return nil
			}
			reservation.EmergencyReservedQuota = remainingNeed
		}
		updatedAt := helper.GetTimestamp()
		if reservation.DailyReservedQuota > 0 {
			if err := tx.Model(&UserQuotaCounter{}).
				Where("user_id = ? AND counter_type = ? AND period_key = ?", normalizedUserID, UserQuotaCounterTypeDaily, reservation.DailyBizDate).
				Updates(map[string]any{
					"reserved_quota": gorm.Expr("reserved_quota + ?", reservation.DailyReservedQuota),
					"updated_at":     updatedAt,
				}).Error; err != nil {
				return err
			}
		}
		if reservation.EmergencyReservedQuota > 0 {
			if err := tx.Model(&UserQuotaCounter{}).
				Where("user_id = ? AND counter_type = ? AND period_key = ?", normalizedUserID, UserQuotaCounterTypeMonthlyEmergency, reservation.EmergencyBizMonth).
				Updates(map[string]any{
					"reserved_quota": gorm.Expr("reserved_quota + ?", reservation.EmergencyReservedQuota),
					"updated_at":     updatedAt,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return UserQuotaReservation{}, false, "", err
	}
	if !allowed {
		return UserQuotaReservation{}, false, denyMessage, nil
	}
	return reservation, true, "", nil
}

func ReserveUserQuota(userID string, quota int64) (UserQuotaReservation, bool, string, error) {
	return ReserveUserQuotaWithDB(DB, userID, quota)
}

func ReleaseUserQuotaReservationWithDB(db *gorm.DB, reservation UserQuotaReservation) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !reservation.Active() {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		updatedAt := helper.GetTimestamp()
		if reservation.DailyReservedQuota > 0 {
			if err := tx.Model(&UserQuotaCounter{}).
				Where("user_id = ? AND counter_type = ? AND period_key = ?", reservation.UserID, UserQuotaCounterTypeDaily, reservation.DailyBizDate).
				Updates(map[string]any{
					"reserved_quota": gorm.Expr("GREATEST(reserved_quota - ?, 0)", reservation.DailyReservedQuota),
					"updated_at":     updatedAt,
				}).Error; err != nil {
				return err
			}
		}
		if reservation.EmergencyReservedQuota > 0 {
			if err := tx.Model(&UserQuotaCounter{}).
				Where("user_id = ? AND counter_type = ? AND period_key = ?", reservation.UserID, UserQuotaCounterTypeMonthlyEmergency, reservation.EmergencyBizMonth).
				Updates(map[string]any{
					"reserved_quota": gorm.Expr("GREATEST(reserved_quota - ?, 0)", reservation.EmergencyReservedQuota),
					"updated_at":     updatedAt,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func ReleaseUserQuotaReservation(reservation UserQuotaReservation) error {
	return ReleaseUserQuotaReservationWithDB(DB, reservation)
}

func SettleUserQuotaReservationWithDB(db *gorm.DB, reservation UserQuotaReservation, consumedQuota int64) (UserQuotaUsage, error) {
	if db == nil {
		return UserQuotaUsage{}, fmt.Errorf("database handle is nil")
	}
	if strings.TrimSpace(reservation.UserID) == "" {
		return UserQuotaUsage{}, nil
	}
	policy, err := GetUserQuotaPolicyWithDB(db, reservation.UserID)
	if err != nil {
		return UserQuotaUsage{}, err
	}
	if policy.DailyLimit <= 0 {
		return UserQuotaUsage{}, nil
	}
	consumed := consumedQuota
	if consumed < 0 {
		consumed = 0
	}
	usage := UserQuotaUsage{}
	err = db.Transaction(func(tx *gorm.DB) error {
		dailyBizDate := strings.TrimSpace(reservation.DailyBizDate)
		if dailyBizDate == "" {
			dailyBizDate = businessDateByTimezone(time.Now(), policy.Timezone)
		}
		dailyCounter, err := loadUserQuotaCounterForUpdateWithDB(tx, reservation.UserID, UserQuotaCounterTypeDaily, dailyBizDate)
		if err != nil {
			return err
		}
		dailyReservedAfterRelease := dailyCounter.ReservedQuota - reservation.DailyReservedQuota
		if dailyReservedAfterRelease < 0 {
			dailyReservedAfterRelease = 0
		}
		dailyCapacity := policy.DailyLimit - dailyCounter.ConsumedQuota - dailyReservedAfterRelease
		if dailyCapacity < 0 {
			dailyCapacity = 0
		}

		emergencyCapacity := int64(0)
		emergencyMonth := strings.TrimSpace(reservation.EmergencyBizMonth)
		if emergencyMonth == "" {
			emergencyMonth = businessMonthByTimezone(time.Now(), policy.Timezone)
		}
		if policy.PackageEmergencyLimit > 0 {
			emergencyCounter, err := loadUserQuotaCounterForUpdateWithDB(tx, reservation.UserID, UserQuotaCounterTypeMonthlyEmergency, emergencyMonth)
			if err != nil {
				return err
			}
			emergencyReservedAfterRelease := emergencyCounter.ReservedQuota - reservation.EmergencyReservedQuota
			if emergencyReservedAfterRelease < 0 {
				emergencyReservedAfterRelease = 0
			}
			emergencyCapacity = policy.PackageEmergencyLimit - emergencyCounter.ConsumedQuota - emergencyReservedAfterRelease
			if emergencyCapacity < 0 {
				emergencyCapacity = 0
			}
		}

		usage = splitUserQuotaConsumption(consumed, dailyCapacity, emergencyCapacity, false, policy.PackageEmergencyLimit > 0)
		updatedAt := helper.GetTimestamp()
		if err := tx.Model(&UserQuotaCounter{}).
			Where("user_id = ? AND counter_type = ? AND period_key = ?", reservation.UserID, UserQuotaCounterTypeDaily, dailyBizDate).
			Updates(map[string]any{
				"reserved_quota": gorm.Expr("GREATEST(reserved_quota - ?, 0)", reservation.DailyReservedQuota),
				"consumed_quota": gorm.Expr("consumed_quota + ?", usage.DailyQuotaUsed),
				"updated_at":     updatedAt,
			}).Error; err != nil {
			return err
		}
		if policy.PackageEmergencyLimit > 0 {
			if err := tx.Model(&UserQuotaCounter{}).
				Where("user_id = ? AND counter_type = ? AND period_key = ?", reservation.UserID, UserQuotaCounterTypeMonthlyEmergency, emergencyMonth).
				Updates(map[string]any{
					"reserved_quota": gorm.Expr("GREATEST(reserved_quota - ?, 0)", reservation.EmergencyReservedQuota),
					"consumed_quota": gorm.Expr("consumed_quota + ?", usage.EmergencyQuotaUsed),
					"updated_at":     updatedAt,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return UserQuotaUsage{}, err
	}
	return usage, nil
}

func SettleUserQuotaReservation(reservation UserQuotaReservation, consumedQuota int64) (UserQuotaUsage, error) {
	return SettleUserQuotaReservationWithDB(DB, reservation, consumedQuota)
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

func GetUserMonthlyEmergencyQuotaSnapshotWithDB(db *gorm.DB, userID string, bizMonth string) (UserMonthlyEmergencyQuotaSnapshot, error) {
	if db == nil {
		return UserMonthlyEmergencyQuotaSnapshot{}, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return UserMonthlyEmergencyQuotaSnapshot{}, fmt.Errorf("用户 ID 不能为空")
	}
	policy, err := GetUserQuotaPolicyWithDB(db, normalizedUserID)
	if err != nil {
		return UserMonthlyEmergencyQuotaSnapshot{}, err
	}
	normalizedBizMonth, err := normalizeUserQuotaMonth(bizMonth, policy.Timezone)
	if err != nil {
		return UserMonthlyEmergencyQuotaSnapshot{}, err
	}
	counter := UserQuotaCounter{}
	err = db.Where("user_id = ? AND counter_type = ? AND period_key = ?", normalizedUserID, UserQuotaCounterTypeMonthlyEmergency, normalizedBizMonth).First(&counter).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return UserMonthlyEmergencyQuotaSnapshot{}, err
	}
	if err == gorm.ErrRecordNotFound {
		counter = UserQuotaCounter{UserID: normalizedUserID, CounterType: UserQuotaCounterTypeMonthlyEmergency, PeriodKey: normalizedBizMonth}
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
	return UserMonthlyEmergencyQuotaSnapshot{
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

func GetUserMonthlyEmergencyQuotaSnapshot(userID string, bizMonth string) (UserMonthlyEmergencyQuotaSnapshot, error) {
	return GetUserMonthlyEmergencyQuotaSnapshotWithDB(DB, userID, bizMonth)
}

func GetUserQuotaSummaryWithDB(db *gorm.DB, userID string, bizDate string, bizMonth string) (UserQuotaSummary, error) {
	daily, err := GetUserDailyQuotaSnapshotWithDB(db, userID, bizDate)
	if err != nil {
		return UserQuotaSummary{}, err
	}
	monthly, err := GetUserMonthlyEmergencyQuotaSnapshotWithDB(db, userID, bizMonth)
	if err != nil {
		return UserQuotaSummary{}, err
	}
	return UserQuotaSummary{
		UserID:           strings.TrimSpace(userID),
		Daily:            daily,
		MonthlyEmergency: monthly,
	}, nil
}

func GetUserQuotaSummary(userID string, bizDate string, bizMonth string) (UserQuotaSummary, error) {
	return GetUserQuotaSummaryWithDB(DB, userID, bizDate, bizMonth)
}
