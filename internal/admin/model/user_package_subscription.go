package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"gorm.io/gorm"
)

const UserPackageSubscriptionsTableName = "user_package_subscriptions"

const (
	UserPackageSubscriptionStatusActive   = 1
	UserPackageSubscriptionStatusExpired  = 2
	UserPackageSubscriptionStatusReplaced = 3
	UserPackageSubscriptionStatusCanceled = 4
)

type UserPackageSubscription struct {
	Id                         string `json:"id" gorm:"primaryKey;type:char(36)"`
	UserID                     string `json:"user_id" gorm:"type:char(36);not null;index"`
	PackageID                  string `json:"package_id" gorm:"type:char(36);not null;index"`
	PackageName                string `json:"package_name" gorm:"type:varchar(64);not null;default:''"`
	GroupID                    string `json:"group_id" gorm:"type:char(36);not null;index"`
	DailyQuotaLimit            int64  `json:"daily_quota_limit" gorm:"type:bigint;not null;default:0"`
	PackageEmergencyQuotaLimit int64  `json:"package_emergency_quota_limit" gorm:"column:package_emergency_quota_limit;type:bigint;not null;default:0"`
	QuotaResetTimezone         string `json:"quota_reset_timezone" gorm:"type:varchar(64);not null;default:'Asia/Shanghai'"`
	StartedAt                  int64  `json:"started_at" gorm:"bigint;not null;index"`
	ExpiresAt                  int64  `json:"expires_at" gorm:"bigint;not null;default:0;index"`
	Status                     int    `json:"status" gorm:"type:int;not null;default:1;index"`
	UpdatedAt                  int64  `json:"updated_at" gorm:"bigint;index"`
}

func (UserPackageSubscription) TableName() string {
	return UserPackageSubscriptionsTableName
}

func (item *UserPackageSubscription) EnsureID() {
	if item == nil {
		return
	}
	if strings.TrimSpace(item.Id) == "" {
		item.Id = random.GetUUID()
	}
}

func markExpiredUserPackageSubscriptionsWithDB(db *gorm.DB, userID string, now int64) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	query := db.Model(&UserPackageSubscription{}).
		Where("status = ? AND expires_at > 0 AND expires_at < ?", UserPackageSubscriptionStatusActive, now)
	if strings.TrimSpace(userID) != "" {
		query = query.Where("user_id = ?", strings.TrimSpace(userID))
	}
	return query.Updates(map[string]any{
		"status":     UserPackageSubscriptionStatusExpired,
		"updated_at": now,
	}).Error
}

func getActiveUserPackageSubscriptionWithDB(db *gorm.DB, userID string) (UserPackageSubscription, error) {
	if db == nil {
		return UserPackageSubscription{}, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return UserPackageSubscription{}, fmt.Errorf("用户 ID 不能为空")
	}
	now := helper.GetTimestamp()
	if err := markExpiredUserPackageSubscriptionsWithDB(db, normalizedUserID, now); err != nil {
		return UserPackageSubscription{}, err
	}
	row := UserPackageSubscription{}
	err := db.Where("user_id = ? AND status = ? AND started_at <= ? AND (expires_at = 0 OR expires_at >= ?)", normalizedUserID, UserPackageSubscriptionStatusActive, now, now).
		Order("updated_at desc").
		First(&row).Error
	if err != nil {
		return UserPackageSubscription{}, err
	}
	return row, nil
}

func getActiveUserPackageSubscriptionForGroupWithDB(db *gorm.DB, userID string, groupID string) (UserPackageSubscription, error) {
	if db == nil {
		return UserPackageSubscription{}, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return UserPackageSubscription{}, fmt.Errorf("用户 ID 不能为空")
	}
	normalizedGroupID := strings.TrimSpace(groupID)
	if normalizedGroupID == "" {
		return getActiveUserPackageSubscriptionWithDB(db, normalizedUserID)
	}
	now := helper.GetTimestamp()
	if err := markExpiredUserPackageSubscriptionsWithDB(db, normalizedUserID, now); err != nil {
		return UserPackageSubscription{}, err
	}
	row := UserPackageSubscription{}
	err := db.Where("user_id = ? AND group_id = ? AND status = ? AND started_at <= ? AND (expires_at = 0 OR expires_at >= ?)", normalizedUserID, normalizedGroupID, UserPackageSubscriptionStatusActive, now, now).
		Order("updated_at desc").
		First(&row).Error
	if err != nil {
		return UserPackageSubscription{}, err
	}
	return row, nil
}

func AssignServicePackageToUserWithDB(db *gorm.DB, packageID string, userID string, startAt int64) (UserPackageSubscription, error) {
	if db == nil {
		return UserPackageSubscription{}, fmt.Errorf("database handle is nil")
	}
	normalizedPackageID := strings.TrimSpace(packageID)
	if normalizedPackageID == "" {
		return UserPackageSubscription{}, fmt.Errorf("套餐 ID 不能为空")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return UserPackageSubscription{}, fmt.Errorf("用户 ID 不能为空")
	}
	user := User{}
	if err := db.Select("id").First(&user, "id = ?", normalizedUserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return UserPackageSubscription{}, fmt.Errorf("用户不存在")
		}
		return UserPackageSubscription{}, err
	}
	servicePackage, err := getServicePackageByIDWithDB(db, normalizedPackageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return UserPackageSubscription{}, fmt.Errorf("套餐不存在")
		}
		return UserPackageSubscription{}, err
	}
	if !servicePackage.Enabled {
		return UserPackageSubscription{}, fmt.Errorf("套餐已禁用")
	}
	effectiveStartAt := startAt
	if effectiveStartAt <= 0 {
		effectiveStartAt = helper.GetTimestamp()
	}
	effectiveDurationDays := normalizeServicePackageDurationDays(servicePackage.DurationDays)
	expiresAt := int64(0)
	if effectiveDurationDays > 0 {
		expiresAt = effectiveStartAt + int64(effectiveDurationDays)*86400 - 1
	}
	now := helper.GetTimestamp()
	subscription := UserPackageSubscription{
		UserID:                     normalizedUserID,
		PackageID:                  strings.TrimSpace(servicePackage.Id),
		PackageName:                strings.TrimSpace(servicePackage.Name),
		GroupID:                    strings.TrimSpace(servicePackage.GroupID),
		DailyQuotaLimit:            normalizeServicePackageDailyQuotaLimit(servicePackage.DailyQuotaLimit),
		PackageEmergencyQuotaLimit: normalizeServicePackagePackageEmergencyQuotaLimit(servicePackage.PackageEmergencyQuotaLimit),
		QuotaResetTimezone:         normalizeServicePackageTimezone(servicePackage.QuotaResetTimezone),
		StartedAt:                  effectiveStartAt,
		ExpiresAt:                  expiresAt,
		Status:                     UserPackageSubscriptionStatusActive,
		UpdatedAt:                  now,
	}
	subscription.EnsureID()

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := markExpiredUserPackageSubscriptionsWithDB(tx, normalizedUserID, now); err != nil {
			return err
		}
		if err := tx.Model(&UserPackageSubscription{}).
			Where("user_id = ? AND status = ? AND started_at <= ? AND (expires_at = 0 OR expires_at >= ?)", normalizedUserID, UserPackageSubscriptionStatusActive, effectiveStartAt, effectiveStartAt).
			Updates(map[string]any{
				"status":     UserPackageSubscriptionStatusReplaced,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		if err := tx.Create(&subscription).Error; err != nil {
			return err
		}
		return tx.Model(&User{}).Where("id = ?", normalizedUserID).Updates(map[string]any{
			"group":                         strings.TrimSpace(servicePackage.GroupID),
			"daily_quota_limit":             normalizeUserDailyQuotaLimit(subscription.DailyQuotaLimit),
			"package_emergency_quota_limit": normalizeUserPackageEmergencyQuotaLimit(subscription.PackageEmergencyQuotaLimit),
			"quota_reset_timezone":          normalizeUserQuotaResetTimezone(subscription.QuotaResetTimezone),
		}).Error
	})
	if err != nil {
		return UserPackageSubscription{}, err
	}
	return subscription, nil
}

func GetActiveUserPackageSubscription(userID string) (UserPackageSubscription, error) {
	return getActiveUserPackageSubscriptionWithDB(DB, userID)
}

func GetActiveUserPackageSubscriptionForGroup(userID string, groupID string) (UserPackageSubscription, error) {
	return getActiveUserPackageSubscriptionForGroupWithDB(DB, userID, groupID)
}

func ListActiveUserPackageSubscriptionsByUserIDs(userIDs []string) ([]UserPackageSubscription, error) {
	normalizedIDs := make([]string, 0, len(userIDs))
	seen := make(map[string]struct{}, len(userIDs))
	for _, userID := range userIDs {
		normalized := strings.TrimSpace(userID)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		normalizedIDs = append(normalizedIDs, normalized)
	}
	if len(normalizedIDs) == 0 {
		return []UserPackageSubscription{}, nil
	}
	now := helper.GetTimestamp()
	rows := make([]UserPackageSubscription, 0, len(normalizedIDs))
	if err := DB.
		Where("user_id IN ? AND status = ? AND started_at <= ? AND (expires_at = 0 OR expires_at >= ?)",
			normalizedIDs,
			UserPackageSubscriptionStatusActive,
			now,
			now,
		).
		Order("user_id asc, updated_at desc, id desc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]UserPackageSubscription, 0, len(rows))
	resolved := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		userID := strings.TrimSpace(row.UserID)
		if userID == "" {
			continue
		}
		if _, ok := resolved[userID]; ok {
			continue
		}
		resolved[userID] = struct{}{}
		items = append(items, row)
	}
	return items, nil
}

func AssignServicePackageToUser(packageID string, userID string, startAt int64) (UserPackageSubscription, error) {
	return AssignServicePackageToUserWithDB(DB, packageID, userID, startAt)
}
