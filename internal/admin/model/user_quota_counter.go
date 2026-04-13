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
	UserQuotaCounterTypePackageEmergency = "package_emergency"
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

type UserPackageEmergencyQuotaReservation struct {
	UserID        string
	BizMonth      string
	ReservedQuota int64
}

type PackageQuotaReservation struct {
	GroupDaily            GroupDailyQuotaReservation
	PackageEmergency      UserPackageEmergencyQuotaReservation
	DailyLimit            int64
	PackageEmergencyLimit int64
	Timezone              string
}

func (reservation UserPackageEmergencyQuotaReservation) Active() bool {
	return strings.TrimSpace(reservation.UserID) != "" &&
		strings.TrimSpace(reservation.BizMonth) != "" &&
		reservation.ReservedQuota > 0
}

func (reservation PackageQuotaReservation) Active() bool {
	return reservation.GroupDaily.Active() || reservation.PackageEmergency.Active()
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
	consumed := int64(0)
	reserved := int64(0)
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
		UpdatedAt:      0,
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

func ReservePackageQuotaWithDB(db *gorm.DB, groupID string, userID string, quota int64) (PackageQuotaReservation, bool, error) {
	if db == nil {
		return PackageQuotaReservation{}, false, fmt.Errorf("database handle is nil")
	}
	normalizedGroupID := strings.TrimSpace(groupID)
	normalizedUserID := strings.TrimSpace(userID)
	normalizedQuota := normalizeGroupDailyQuotaLimit(quota)
	if normalizedGroupID == "" || normalizedQuota <= 0 {
		return PackageQuotaReservation{}, true, nil
	}
	if normalizedUserID == "" {
		return PackageQuotaReservation{}, false, fmt.Errorf("用户 ID 不能为空")
	}
	subscription, err := getActiveUserPackageSubscriptionForGroupWithDB(db, normalizedUserID, normalizedGroupID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return PackageQuotaReservation{}, true, nil
		}
		return PackageQuotaReservation{}, false, err
	}
	timezone := normalizeServicePackageTimezone(subscription.QuotaResetTimezone)
	dailyLimit := normalizeServicePackageDailyQuotaLimit(subscription.DailyQuotaLimit)
	packageEmergencyLimit := normalizeServicePackagePackageEmergencyQuotaLimit(subscription.PackageEmergencyQuotaLimit)
	bizDate := businessDateByTimezone(time.Now(), timezone)
	bizMonth := businessMonthByTimezone(time.Now(), timezone)
	reservation := PackageQuotaReservation{
		DailyLimit:            dailyLimit,
		PackageEmergencyLimit: packageEmergencyLimit,
		Timezone:              timezone,
		GroupDaily: GroupDailyQuotaReservation{
			GroupID: normalizedGroupID,
			UserID:  normalizedUserID,
			BizDate: bizDate,
		},
		PackageEmergency: UserPackageEmergencyQuotaReservation{
			UserID:   normalizedUserID,
			BizMonth: bizMonth,
		},
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		groupCounter, err := loadGroupQuotaCounterForUpdateWithDB(tx, normalizedGroupID, normalizedUserID, GroupQuotaCounterTypeDaily, bizDate)
		if err != nil {
			return err
		}
		emergencyCounter, err := loadUserQuotaCounterForUpdateWithDB(tx, normalizedUserID, UserQuotaCounterTypePackageEmergency, bizMonth)
		if err != nil {
			return err
		}

		dailyRemaining := dailyLimit - maxInt64(groupCounter.ConsumedQuota, 0) - maxInt64(groupCounter.ReservedQuota, 0)
		if dailyRemaining < 0 {
			dailyRemaining = 0
		}
		emergencyRemaining := packageEmergencyLimit - maxInt64(emergencyCounter.ConsumedQuota, 0) - maxInt64(emergencyCounter.ReservedQuota, 0)
		if emergencyRemaining < 0 {
			emergencyRemaining = 0
		}
		if dailyRemaining+emergencyRemaining < normalizedQuota {
			return nil
		}

		reservation.GroupDaily.ReservedQuota = minInt64(normalizedQuota, dailyRemaining)
		reservation.PackageEmergency.ReservedQuota = normalizedQuota - reservation.GroupDaily.ReservedQuota
		updatedAt := helper.GetTimestamp()
		if reservation.GroupDaily.ReservedQuota > 0 {
			if err := tx.Model(&GroupQuotaCounter{}).
				Where("group_id = ? AND user_id = ? AND counter_type = ? AND period_key = ?", normalizedGroupID, normalizedUserID, GroupQuotaCounterTypeDaily, bizDate).
				Updates(map[string]any{
					"reserved_quota": gorm.Expr("reserved_quota + ?", reservation.GroupDaily.ReservedQuota),
					"updated_at":     updatedAt,
				}).Error; err != nil {
				return err
			}
		}
		if reservation.PackageEmergency.ReservedQuota > 0 {
			if err := tx.Model(&UserQuotaCounter{}).
				Where("user_id = ? AND counter_type = ? AND period_key = ?", normalizedUserID, UserQuotaCounterTypePackageEmergency, bizMonth).
				Updates(map[string]any{
					"reserved_quota": gorm.Expr("reserved_quota + ?", reservation.PackageEmergency.ReservedQuota),
					"updated_at":     updatedAt,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return PackageQuotaReservation{}, false, err
	}
	if !reservation.Active() {
		return PackageQuotaReservation{}, false, nil
	}
	return reservation, true, nil
}

func ReservePackageQuota(groupID string, userID string, quota int64) (PackageQuotaReservation, bool, error) {
	return ReservePackageQuotaWithDB(DB, groupID, userID, quota)
}

func ReleasePackageQuotaReservationWithDB(db *gorm.DB, reservation PackageQuotaReservation) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !reservation.Active() {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		updatedAt := helper.GetTimestamp()
		if reservation.GroupDaily.Active() {
			if err := tx.Model(&GroupQuotaCounter{}).
				Where("group_id = ? AND user_id = ? AND counter_type = ? AND period_key = ?", reservation.GroupDaily.GroupID, reservation.GroupDaily.UserID, GroupQuotaCounterTypeDaily, reservation.GroupDaily.BizDate).
				Updates(map[string]any{
					"reserved_quota": gorm.Expr("GREATEST(reserved_quota - ?, 0)", reservation.GroupDaily.ReservedQuota),
					"updated_at":     updatedAt,
				}).Error; err != nil {
				return err
			}
		}
		if reservation.PackageEmergency.Active() {
			if err := tx.Model(&UserQuotaCounter{}).
				Where("user_id = ? AND counter_type = ? AND period_key = ?", reservation.PackageEmergency.UserID, UserQuotaCounterTypePackageEmergency, reservation.PackageEmergency.BizMonth).
				Updates(map[string]any{
					"reserved_quota": gorm.Expr("GREATEST(reserved_quota - ?, 0)", reservation.PackageEmergency.ReservedQuota),
					"updated_at":     updatedAt,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func ReleasePackageQuotaReservation(reservation PackageQuotaReservation) error {
	return ReleasePackageQuotaReservationWithDB(DB, reservation)
}

func SettlePackageQuotaReservationWithDB(db *gorm.DB, reservation PackageQuotaReservation, consumedQuota int64) (int64, int64, error) {
	if db == nil {
		return 0, 0, fmt.Errorf("database handle is nil")
	}
	if !reservation.Active() {
		return 0, 0, nil
	}
	consumed := consumedQuota
	if consumed < 0 {
		consumed = 0
	}
	var dailyConsumed int64
	var emergencyConsumed int64
	err := db.Transaction(func(tx *gorm.DB) error {
		updatedAt := helper.GetTimestamp()
		if reservation.GroupDaily.Active() {
			groupCounter, err := loadGroupQuotaCounterForUpdateWithDB(tx, reservation.GroupDaily.GroupID, reservation.GroupDaily.UserID, GroupQuotaCounterTypeDaily, reservation.GroupDaily.BizDate)
			if err != nil {
				return err
			}
			dailyHeadroom := reservation.DailyLimit - maxInt64(groupCounter.ConsumedQuota, 0)
			if dailyHeadroom < 0 {
				dailyHeadroom = 0
			}
			dailyConsumed = minInt64(consumed, dailyHeadroom)
			if err := tx.Model(&GroupQuotaCounter{}).
				Where("group_id = ? AND user_id = ? AND counter_type = ? AND period_key = ?", reservation.GroupDaily.GroupID, reservation.GroupDaily.UserID, GroupQuotaCounterTypeDaily, reservation.GroupDaily.BizDate).
				Updates(map[string]any{
					"reserved_quota": gorm.Expr("GREATEST(reserved_quota - ?, 0)", reservation.GroupDaily.ReservedQuota),
					"consumed_quota": gorm.Expr("consumed_quota + ?", dailyConsumed),
					"updated_at":     updatedAt,
				}).Error; err != nil {
				return err
			}
		}
		remainingConsumed := consumed - dailyConsumed
		if remainingConsumed < 0 {
			remainingConsumed = 0
		}
		if reservation.PackageEmergency.Active() || remainingConsumed > 0 {
			emergencyCounter, err := loadUserQuotaCounterForUpdateWithDB(tx, reservation.PackageEmergency.UserID, UserQuotaCounterTypePackageEmergency, reservation.PackageEmergency.BizMonth)
			if err != nil {
				return err
			}
			emergencyHeadroom := reservation.PackageEmergencyLimit - maxInt64(emergencyCounter.ConsumedQuota, 0)
			if emergencyHeadroom < 0 {
				emergencyHeadroom = 0
			}
			emergencyConsumed = remainingConsumed
			if remainingConsumed <= reservation.PackageEmergency.ReservedQuota {
				emergencyConsumed = minInt64(remainingConsumed, emergencyHeadroom)
			}
			if err := tx.Model(&UserQuotaCounter{}).
				Where("user_id = ? AND counter_type = ? AND period_key = ?", reservation.PackageEmergency.UserID, UserQuotaCounterTypePackageEmergency, reservation.PackageEmergency.BizMonth).
				Updates(map[string]any{
					"reserved_quota": gorm.Expr("GREATEST(reserved_quota - ?, 0)", reservation.PackageEmergency.ReservedQuota),
					"consumed_quota": gorm.Expr("consumed_quota + ?", emergencyConsumed),
					"updated_at":     updatedAt,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return dailyConsumed, emergencyConsumed, nil
}

func SettlePackageQuotaReservation(reservation PackageQuotaReservation, consumedQuota int64) (int64, int64, error) {
	return SettlePackageQuotaReservationWithDB(DB, reservation, consumedQuota)
}

func minInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
