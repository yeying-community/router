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
	PackageType                string `json:"package_type" gorm:"type:varchar(32);not null;default:'yyc_quota';index"`
	ScopeType                  string `json:"scope_type" gorm:"type:varchar(32);not null;default:'all';index"`
	ScopeProvider              string `json:"scope_provider" gorm:"type:varchar(64);not null;default:'';index"`
	ScopeModel                 string `json:"scope_model" gorm:"type:varchar(191);not null;default:'';index"`
	ScopeEndpoint              string `json:"scope_endpoint" gorm:"type:varchar(191);not null;default:''"`
	QuotaMetric                string `json:"quota_metric" gorm:"type:varchar(32);not null;default:'yyc';index"`
	PeriodType                 string `json:"period_type" gorm:"type:varchar(32);not null;default:'daily';index"`
	PeriodLimit                int64  `json:"period_limit" gorm:"type:bigint;not null;default:0"`
	MaxConcurrencyPerUser      int    `json:"max_concurrency_per_user" gorm:"type:int;not null;default:0"`
	MaxConcurrencyPerPackage   int    `json:"max_concurrency_per_package" gorm:"type:int;not null;default:0"`
	AllowBalanceFallback       bool   `json:"allow_balance_fallback" gorm:"not null;default:false"`
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
		Where("status = ? AND expires_at > 0 AND expires_at <= ?", UserPackageSubscriptionStatusActive, now)
	if strings.TrimSpace(userID) != "" {
		query = query.Where("user_id = ?", strings.TrimSpace(userID))
	}
	return query.Updates(map[string]any{
		"status":     UserPackageSubscriptionStatusExpired,
		"updated_at": now,
	}).Error
}

func syncUserPackageSubscriptionsWithDB(db *gorm.DB, userID string, now int64) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return fmt.Errorf("用户 ID 不能为空")
	}
	effectiveNow := now
	if effectiveNow <= 0 {
		effectiveNow = helper.GetTimestamp()
	}
	return markExpiredUserPackageSubscriptionsWithDB(db, normalizedUserID, effectiveNow)
}

func applyUserPackageSubscriptionScopeIdentityFilter(query *gorm.DB, subscription UserPackageSubscription) *gorm.DB {
	normalizeServicePackageSubscriptionScopeAndQuota(&subscription)
	return query.
		Where("package_type = ?", strings.TrimSpace(subscription.PackageType)).
		Where("quota_metric = ?", strings.TrimSpace(subscription.QuotaMetric)).
		Where("COALESCE(group_id, '') = ?", strings.TrimSpace(subscription.GroupID))
}

func replaceUserPackageSubscriptionsForAssignment(tx *gorm.DB, userID string, subscription UserPackageSubscription, startAt int64, updatedAt int64) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return fmt.Errorf("用户 ID 不能为空")
	}
	normalizeServicePackageSubscriptionScopeAndQuota(&subscription)
	activeQuery := tx.Model(&UserPackageSubscription{}).
		Where("user_id = ? AND status = ? AND started_at <= ? AND (expires_at = 0 OR expires_at > ?)",
			normalizedUserID,
			UserPackageSubscriptionStatusActive,
			startAt,
			startAt,
		)
	activeQuery = applyUserPackageSubscriptionScopeIdentityFilter(activeQuery, subscription)
	if err := activeQuery.Updates(map[string]any{
		"status":     UserPackageSubscriptionStatusReplaced,
		"updated_at": updatedAt,
	}).Error; err != nil {
		return err
	}
	return nil
}

func syncUserFieldsForActivePackageSubscriptionWithDB(tx *gorm.DB, userID string, subscription UserPackageSubscription) error {
	return tx.Model(&User{}).Where("id = ?", strings.TrimSpace(userID)).Updates(map[string]any{
		"group":                strings.TrimSpace(subscription.GroupID),
		"quota_reset_timezone": normalizeUserQuotaResetTimezone(subscription.QuotaResetTimezone),
	}).Error
}

func getActiveUserPackageSubscriptionForPackageGroupWithDB(db *gorm.DB, userID string, servicePackage ServicePackage, now int64) (UserPackageSubscription, error) {
	if db == nil {
		return UserPackageSubscription{}, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return UserPackageSubscription{}, fmt.Errorf("用户 ID 不能为空")
	}
	normalizeServicePackageScopeAndQuota(&servicePackage)
	effectiveNow := now
	if effectiveNow <= 0 {
		effectiveNow = helper.GetTimestamp()
	}
	if err := syncUserPackageSubscriptionsWithDB(db, normalizedUserID, effectiveNow); err != nil {
		return UserPackageSubscription{}, err
	}
	probe := UserPackageSubscription{
		PackageType: servicePackage.PackageType,
		GroupID:     servicePackage.GroupID,
		QuotaMetric: servicePackage.QuotaMetric,
	}
	query := db.
		Where("user_id = ? AND status = ? AND started_at <= ? AND (expires_at = 0 OR expires_at > ?)",
			normalizedUserID,
			UserPackageSubscriptionStatusActive,
			effectiveNow,
			effectiveNow,
		)
	query = applyUserPackageSubscriptionScopeIdentityFilter(query, probe)
	row := UserPackageSubscription{}
	if err := query.Order("updated_at desc, started_at desc, id desc").First(&row).Error; err != nil {
		return UserPackageSubscription{}, err
	}
	return row, nil
}

func getActiveYYCUserPackageSubscriptionForGroupWithDB(db *gorm.DB, userID string, groupID string) (UserPackageSubscription, error) {
	if db == nil {
		return UserPackageSubscription{}, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return UserPackageSubscription{}, fmt.Errorf("用户 ID 不能为空")
	}
	normalizedGroupID := strings.TrimSpace(groupID)
	if normalizedGroupID == "" {
		return UserPackageSubscription{}, gorm.ErrRecordNotFound
	}
	now := helper.GetTimestamp()
	if err := syncUserPackageSubscriptionsWithDB(db, normalizedUserID, now); err != nil {
		return UserPackageSubscription{}, err
	}
	row := UserPackageSubscription{}
	query := db.Where(
		"user_id = ? AND group_id = ? AND status = ? AND package_type = ? AND quota_metric = ? AND started_at <= ? AND (expires_at = 0 OR expires_at > ?)",
		normalizedUserID,
		normalizedGroupID,
		UserPackageSubscriptionStatusActive,
		ServicePackageTypeYYCQuota,
		ServicePackageQuotaMetricYYC,
		now,
		now,
	)
	err := query.
		Order("updated_at desc, started_at desc, id desc").
		First(&row).Error
	if err != nil {
		return UserPackageSubscription{}, err
	}
	return row, nil
}

func assignServicePackageToUserWithExpiresAtDB(db *gorm.DB, packageID string, userID string, expiresAtOverride int64) (UserPackageSubscription, error) {
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
	now := helper.GetTimestamp()
	effectiveStartAt := now
	effectiveDurationDays := normalizeServicePackageDurationDays(servicePackage.DurationDays)
	expiresAt := int64(0)
	if effectiveDurationDays > 0 {
		expiresAt = effectiveStartAt + int64(effectiveDurationDays)*86400
	}
	if expiresAtOverride > 0 {
		expiresAt = expiresAtOverride
	}
	normalizeServicePackageScopeAndQuota(&servicePackage)
	subscription := UserPackageSubscription{
		UserID:                     normalizedUserID,
		PackageID:                  strings.TrimSpace(servicePackage.Id),
		PackageName:                strings.TrimSpace(servicePackage.Name),
		GroupID:                    strings.TrimSpace(servicePackage.GroupID),
		PackageType:                servicePackage.PackageType,
		ScopeType:                  servicePackage.ScopeType,
		ScopeProvider:              servicePackage.ScopeProvider,
		ScopeModel:                 servicePackage.ScopeModel,
		ScopeEndpoint:              servicePackage.ScopeEndpoint,
		QuotaMetric:                servicePackage.QuotaMetric,
		PeriodType:                 servicePackage.PeriodType,
		PeriodLimit:                servicePackage.PeriodLimit,
		MaxConcurrencyPerUser:      servicePackage.MaxConcurrencyPerUser,
		MaxConcurrencyPerPackage:   servicePackage.MaxConcurrencyPerPackage,
		AllowBalanceFallback:       servicePackage.AllowBalanceFallback,
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
		if err := replaceUserPackageSubscriptionsForAssignment(
			tx,
			normalizedUserID,
			subscription,
			effectiveStartAt,
			now,
		); err != nil {
			return err
		}
		if err := tx.Create(&subscription).Error; err != nil {
			return err
		}
		return syncUserFieldsForActivePackageSubscriptionWithDB(tx, normalizedUserID, subscription)
	})
	if err != nil {
		return UserPackageSubscription{}, err
	}
	RefreshUserGroupCaches(normalizedUserID)
	return subscription, nil
}

func AssignServicePackageToUserWithDB(db *gorm.DB, packageID string, userID string, startAt int64) (UserPackageSubscription, error) {
	return assignServicePackageToUserWithExpiresAtDB(db, packageID, userID, 0)
}

func RenewServicePackageForUserWithDB(db *gorm.DB, packageID string, userID string, now int64) (UserPackageSubscription, error) {
	if db == nil {
		return UserPackageSubscription{}, fmt.Errorf("database handle is nil")
	}
	effectiveNow := now
	if effectiveNow <= 0 {
		effectiveNow = helper.GetTimestamp()
	}
	normalizedPackageID := strings.TrimSpace(packageID)
	if normalizedPackageID == "" {
		return UserPackageSubscription{}, fmt.Errorf("套餐 ID 不能为空")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return UserPackageSubscription{}, fmt.Errorf("用户 ID 不能为空")
	}
	if err := syncUserPackageSubscriptionsWithDB(db, normalizedUserID, effectiveNow); err != nil {
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
	active, err := getActiveUserPackageSubscriptionForPackageGroupWithDB(db, normalizedUserID, servicePackage, effectiveNow)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AssignServicePackageToUserWithDB(db, normalizedPackageID, normalizedUserID, effectiveNow)
		}
		return UserPackageSubscription{}, err
	}
	if strings.TrimSpace(active.PackageID) != normalizedPackageID {
		return UserPackageSubscription{}, fmt.Errorf("同槽位生效套餐与续费套餐不一致，请使用升级")
	}
	if active.ExpiresAt <= 0 {
		return UserPackageSubscription{}, fmt.Errorf("同槽位生效套餐无到期时间，无法续费")
	}
	durationDays := normalizeServicePackageDurationDays(servicePackage.DurationDays)
	newExpiresAt := effectiveNow + int64(durationDays)*86400
	if active.ExpiresAt > effectiveNow {
		newExpiresAt = active.ExpiresAt + int64(durationDays)*86400
	}
	active.ExpiresAt = newExpiresAt
	active.UpdatedAt = effectiveNow
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&UserPackageSubscription{}).
			Where("id = ? AND status = ?", strings.TrimSpace(active.Id), UserPackageSubscriptionStatusActive).
			Updates(map[string]any{
				"expires_at": newExpiresAt,
				"updated_at": effectiveNow,
			}).Error; err != nil {
			return err
		}
		return syncUserFieldsForActivePackageSubscriptionWithDB(tx, normalizedUserID, active)
	})
	if err != nil {
		return UserPackageSubscription{}, err
	}
	RefreshUserGroupCaches(normalizedUserID)
	return active, nil
}

func UpgradeServicePackageForUserWithDB(db *gorm.DB, packageID string, userID string, now int64) (UserPackageSubscription, error) {
	if db == nil {
		return UserPackageSubscription{}, fmt.Errorf("database handle is nil")
	}
	effectiveNow := now
	if effectiveNow <= 0 {
		effectiveNow = helper.GetTimestamp()
	}
	normalizedPackageID := strings.TrimSpace(packageID)
	if normalizedPackageID == "" {
		return UserPackageSubscription{}, fmt.Errorf("套餐 ID 不能为空")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return UserPackageSubscription{}, fmt.Errorf("用户 ID 不能为空")
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
	active, err := getActiveUserPackageSubscriptionForPackageGroupWithDB(db, normalizedUserID, servicePackage, effectiveNow)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AssignServicePackageToUserWithDB(db, normalizedPackageID, normalizedUserID, effectiveNow)
		}
		return UserPackageSubscription{}, err
	}
	if strings.TrimSpace(active.PackageID) == normalizedPackageID {
		return UserPackageSubscription{}, fmt.Errorf("目标套餐与同槽位生效套餐一致，请使用续费")
	}
	expiresAtOverride := int64(0)
	if active.ExpiresAt > effectiveNow {
		expiresAtOverride = active.ExpiresAt
	}
	return assignServicePackageToUserWithExpiresAtDB(db, normalizedPackageID, normalizedUserID, expiresAtOverride)
}

func GetActiveYYCUserPackageSubscriptionForGroup(userID string, groupID string) (UserPackageSubscription, error) {
	return getActiveYYCUserPackageSubscriptionForGroupWithDB(DB, userID, groupID)
}

func listActiveUserPackageSubscriptionsWithDB(db *gorm.DB, userID string) ([]UserPackageSubscription, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return nil, fmt.Errorf("用户 ID 不能为空")
	}
	now := helper.GetTimestamp()
	if err := syncUserPackageSubscriptionsWithDB(db, normalizedUserID, now); err != nil {
		return nil, err
	}
	rows := make([]UserPackageSubscription, 0)
	if err := db.
		Where("user_id = ? AND status = ? AND started_at <= ? AND (expires_at = 0 OR expires_at > ?)",
			normalizedUserID,
			UserPackageSubscriptionStatusActive,
			now,
			now,
		).
		Order("updated_at desc, started_at desc, id desc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		normalizeServicePackageSubscriptionScopeAndQuota(&rows[i])
	}
	return rows, nil
}

func ListActiveUserPackageSubscriptions(userID string) ([]UserPackageSubscription, error) {
	return listActiveUserPackageSubscriptionsWithDB(DB, userID)
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
		Where("user_id IN ? AND status = ? AND started_at <= ? AND (expires_at = 0 OR expires_at > ?)",
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
	for _, row := range rows {
		normalizeServicePackageSubscriptionScopeAndQuota(&row)
		items = append(items, row)
	}
	return items, nil
}

func RenewServicePackageForUser(packageID string, userID string, now int64) (UserPackageSubscription, error) {
	return RenewServicePackageForUserWithDB(DB, packageID, userID, now)
}

func UpgradeServicePackageForUser(packageID string, userID string, now int64) (UserPackageSubscription, error) {
	return UpgradeServicePackageForUserWithDB(DB, packageID, userID, now)
}

func AssignServicePackageToUser(packageID string, userID string, startAt int64) (UserPackageSubscription, error) {
	return AssignServicePackageToUserWithDB(DB, packageID, userID, startAt)
}
