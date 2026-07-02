package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const UserPackageUsageCountersTableName = "user_package_usage_counters"

type UserPackageUsageCounter struct {
	Id             string `json:"id" gorm:"primaryKey;type:char(36)"`
	SubscriptionID string `json:"subscription_id" gorm:"type:char(36);not null;uniqueIndex:idx_user_package_usage_unique,priority:1;index"`
	UserID         string `json:"user_id" gorm:"type:char(36);not null;index"`
	PackageID      string `json:"package_id" gorm:"type:char(36);not null;index"`
	GroupID        string `json:"group_id" gorm:"type:char(36);not null;default:'';index"`
	Metric         string `json:"metric" gorm:"type:varchar(32);not null;uniqueIndex:idx_user_package_usage_unique,priority:2;index"`
	ScopeType      string `json:"scope_type" gorm:"type:varchar(32);not null;default:'all';uniqueIndex:idx_user_package_usage_unique,priority:3;index"`
	ScopeProvider  string `json:"scope_provider" gorm:"type:varchar(64);not null;default:'';uniqueIndex:idx_user_package_usage_unique,priority:4;index"`
	ScopeModel     string `json:"scope_model" gorm:"type:varchar(191);not null;default:'';uniqueIndex:idx_user_package_usage_unique,priority:5;index"`
	ScopeEndpoint  string `json:"scope_endpoint" gorm:"type:varchar(191);not null;default:'';uniqueIndex:idx_user_package_usage_unique,priority:6"`
	PeriodType     string `json:"period_type" gorm:"type:varchar(32);not null;default:'none';uniqueIndex:idx_user_package_usage_unique,priority:7;index"`
	PeriodKey      string `json:"period_key" gorm:"type:varchar(32);not null;default:'';uniqueIndex:idx_user_package_usage_unique,priority:8;index"`
	LimitAmount    int64  `json:"limit_amount" gorm:"type:bigint;not null;default:0"`
	ConsumedAmount int64  `json:"consumed_amount" gorm:"type:bigint;not null;default:0"`
	ReservedAmount int64  `json:"reserved_amount" gorm:"type:bigint;not null;default:0"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint;index"`
}

func (UserPackageUsageCounter) TableName() string {
	return UserPackageUsageCountersTableName
}

func (item *UserPackageUsageCounter) EnsureID() {
	if item == nil {
		return
	}
	if strings.TrimSpace(item.Id) == "" {
		item.Id = random.GetUUID()
	}
}

type PackageScopeRequest struct {
	UserID        string
	GroupID       string
	RequestAmount int64
	Now           time.Time
}

type RequestPackageReservation struct {
	CounterID      string
	SubscriptionID string
	UserID         string
	PackageID      string
	GroupID        string
	Metric         string
	ScopeType      string
	ScopeProvider  string
	ScopeModel     string
	ScopeEndpoint  string
	PeriodType     string
	PeriodKey      string
	ReservedAmount int64
	LimitAmount    int64
	Concurrency    EntitlementConcurrencyReservation
}

func (reservation RequestPackageReservation) Active() bool {
	return strings.TrimSpace(reservation.CounterID) != "" &&
		strings.TrimSpace(reservation.SubscriptionID) != "" &&
		reservation.ReservedAmount > 0
}

type RequestPackageReserveResult struct {
	Matched      bool
	Allowed      bool
	Subscription UserPackageSubscription
	Reservation  RequestPackageReservation
	Concurrency  EntitlementConcurrencyReservation
	Reason       string
	Remaining    int64
}

type RequestPackageUsageSnapshot struct {
	Metric          string `json:"metric"`
	PeriodType      string `json:"period_type"`
	PeriodKey       string `json:"period_key"`
	LimitAmount     int64  `json:"limit_amount"`
	ConsumedAmount  int64  `json:"consumed_amount"`
	ReservedAmount  int64  `json:"reserved_amount"`
	RemainingAmount int64  `json:"remaining_amount"`
	Unlimited       bool   `json:"unlimited"`
	UpdatedAt       int64  `json:"updated_at"`
}

func normalizePackageScopeRequest(input PackageScopeRequest) PackageScopeRequest {
	return PackageScopeRequest{
		UserID:        strings.TrimSpace(input.UserID),
		GroupID:       strings.TrimSpace(input.GroupID),
		RequestAmount: maxInt64(input.RequestAmount, 1),
		Now:           input.Now,
	}
}

func requestPackagePeriodKey(periodType string, now time.Time, timezone string) string {
	if now.IsZero() {
		now = time.Now()
	}
	switch normalizeServicePackagePeriodType(periodType, ServicePackageQuotaMetricRequestCount) {
	case ServicePackagePeriodDaily:
		return businessDateByTimezone(now, timezone)
	case ServicePackagePeriodWeekly:
		return businessWeekByTimezone(now, timezone)
	case ServicePackagePeriodMonthly:
		return businessMonthByTimezone(now, timezone)
	case ServicePackagePeriodPackageTotal:
		return "total"
	default:
		return ""
	}
}

func findMatchingRequestPackageSubscriptionsWithDB(db *gorm.DB, req PackageScopeRequest) ([]UserPackageSubscription, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	if strings.TrimSpace(req.UserID) == "" || strings.TrimSpace(req.GroupID) == "" {
		return []UserPackageSubscription{}, nil
	}
	now := helper.GetTimestamp()
	if !req.Now.IsZero() {
		now = req.Now.Unix()
	}
	if err := syncUserPackageSubscriptionsWithDB(db, req.UserID, now); err != nil {
		return nil, err
	}
	rows := make([]UserPackageSubscription, 0)
	query := db.
		Where("user_id = ? AND status = ? AND started_at <= ? AND (expires_at = 0 OR expires_at > ?)",
			req.UserID,
			UserPackageSubscriptionStatusActive,
			now,
			now,
		).
		Where("package_type = ? AND quota_metric = ? AND group_id = ?",
			ServicePackageTypeRequestQuota,
			ServicePackageQuotaMetricRequestCount,
			req.GroupID,
		)
	if err := query.Order("updated_at desc, started_at desc, id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	matches := make([]UserPackageSubscription, 0, len(rows))
	for _, row := range rows {
		normalizeServicePackageSubscriptionScopeAndQuota(&row)
		matches = append(matches, row)
	}
	return matches, nil
}

func normalizeServicePackageSubscriptionScopeAndQuota(subscription *UserPackageSubscription) {
	if subscription == nil {
		return
	}
	proxy := ServicePackage{
		PackageType:              subscription.PackageType,
		ScopeType:                subscription.ScopeType,
		ScopeProvider:            subscription.ScopeProvider,
		ScopeModel:               subscription.ScopeModel,
		ScopeEndpoint:            subscription.ScopeEndpoint,
		QuotaMetric:              subscription.QuotaMetric,
		PeriodType:               subscription.PeriodType,
		PeriodLimit:              subscription.PeriodLimit,
		MaxConcurrencyPerUser:    subscription.MaxConcurrencyPerUser,
		MaxConcurrencyPerPackage: subscription.MaxConcurrencyPerPackage,
		AllowBalanceFallback:     subscription.AllowBalanceFallback,
		DailyQuotaLimit:          subscription.DailyQuotaLimit,
	}
	normalizeServicePackageScopeAndQuota(&proxy)
	subscription.PackageType = proxy.PackageType
	subscription.ScopeType = proxy.ScopeType
	subscription.ScopeProvider = proxy.ScopeProvider
	subscription.ScopeModel = proxy.ScopeModel
	subscription.ScopeEndpoint = proxy.ScopeEndpoint
	subscription.QuotaMetric = proxy.QuotaMetric
	subscription.PeriodType = proxy.PeriodType
	subscription.PeriodLimit = proxy.PeriodLimit
	subscription.MaxConcurrencyPerUser = proxy.MaxConcurrencyPerUser
	subscription.MaxConcurrencyPerPackage = proxy.MaxConcurrencyPerPackage
	subscription.AllowBalanceFallback = proxy.AllowBalanceFallback
}

func ensureUserPackageUsageCounterWithDB(tx *gorm.DB, subscription UserPackageSubscription, periodKey string) (UserPackageUsageCounter, error) {
	counter := UserPackageUsageCounter{
		SubscriptionID: strings.TrimSpace(subscription.Id),
		UserID:         strings.TrimSpace(subscription.UserID),
		PackageID:      strings.TrimSpace(subscription.PackageID),
		GroupID:        strings.TrimSpace(subscription.GroupID),
		Metric:         strings.TrimSpace(subscription.QuotaMetric),
		ScopeType:      strings.TrimSpace(subscription.ScopeType),
		ScopeProvider:  strings.TrimSpace(subscription.ScopeProvider),
		ScopeModel:     strings.TrimSpace(subscription.ScopeModel),
		ScopeEndpoint:  strings.TrimSpace(subscription.ScopeEndpoint),
		PeriodType:     strings.TrimSpace(subscription.PeriodType),
		PeriodKey:      strings.TrimSpace(periodKey),
		LimitAmount:    normalizeServicePackagePeriodLimit(subscription.PeriodLimit, subscription.QuotaMetric, subscription.PeriodType, subscription.DailyQuotaLimit),
		CreatedAt:      helper.GetTimestamp(),
		UpdatedAt:      helper.GetTimestamp(),
	}
	counter.EnsureID()
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&counter).Error; err != nil {
		return UserPackageUsageCounter{}, err
	}
	row := UserPackageUsageCounter{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("subscription_id = ? AND metric = ? AND scope_type = ? AND scope_provider = ? AND scope_model = ? AND scope_endpoint = ? AND period_type = ? AND period_key = ?",
			counter.SubscriptionID,
			counter.Metric,
			counter.ScopeType,
			counter.ScopeProvider,
			counter.ScopeModel,
			counter.ScopeEndpoint,
			counter.PeriodType,
			counter.PeriodKey,
		).
		Take(&row).Error
	return row, err
}

func GetRequestPackageUsageSnapshotWithDB(db *gorm.DB, subscription UserPackageSubscription, now time.Time) (RequestPackageUsageSnapshot, error) {
	if db == nil {
		return RequestPackageUsageSnapshot{}, fmt.Errorf("database handle is nil")
	}
	normalizeServicePackageSubscriptionScopeAndQuota(&subscription)
	limit := normalizeServicePackagePeriodLimit(subscription.PeriodLimit, subscription.QuotaMetric, subscription.PeriodType, subscription.DailyQuotaLimit)
	periodKey := requestPackagePeriodKey(subscription.PeriodType, now, subscription.QuotaResetTimezone)
	snapshot := RequestPackageUsageSnapshot{
		Metric:      strings.TrimSpace(subscription.QuotaMetric),
		PeriodType:  strings.TrimSpace(subscription.PeriodType),
		PeriodKey:   strings.TrimSpace(periodKey),
		LimitAmount: limit,
		Unlimited:   limit <= 0,
	}
	if strings.TrimSpace(subscription.Id) == "" ||
		strings.TrimSpace(subscription.QuotaMetric) != ServicePackageQuotaMetricRequestCount {
		return snapshot, nil
	}
	counter := UserPackageUsageCounter{}
	err := db.
		Where("subscription_id = ? AND metric = ? AND scope_type = ? AND scope_provider = ? AND scope_model = ? AND scope_endpoint = ? AND period_type = ? AND period_key = ?",
			strings.TrimSpace(subscription.Id),
			strings.TrimSpace(subscription.QuotaMetric),
			strings.TrimSpace(subscription.ScopeType),
			strings.TrimSpace(subscription.ScopeProvider),
			strings.TrimSpace(subscription.ScopeModel),
			strings.TrimSpace(subscription.ScopeEndpoint),
			strings.TrimSpace(subscription.PeriodType),
			strings.TrimSpace(periodKey),
		).
		Take(&counter).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if !snapshot.Unlimited {
				snapshot.RemainingAmount = limit
			}
			return snapshot, nil
		}
		return RequestPackageUsageSnapshot{}, err
	}
	if counter.LimitAmount > 0 {
		snapshot.LimitAmount = counter.LimitAmount
		snapshot.Unlimited = false
	}
	snapshot.ConsumedAmount = maxInt64(counter.ConsumedAmount, 0)
	snapshot.ReservedAmount = maxInt64(counter.ReservedAmount, 0)
	snapshot.UpdatedAt = counter.UpdatedAt
	if snapshot.LimitAmount > 0 {
		snapshot.RemainingAmount = snapshot.LimitAmount - snapshot.ConsumedAmount - snapshot.ReservedAmount
		if snapshot.RemainingAmount < 0 {
			snapshot.RemainingAmount = 0
		}
	}
	return snapshot, nil
}

func GetRequestPackageUsageSnapshot(subscription UserPackageSubscription) (RequestPackageUsageSnapshot, error) {
	return GetRequestPackageUsageSnapshotWithDB(DB, subscription, time.Now())
}

func ReserveRequestPackageWithDB(db *gorm.DB, input PackageScopeRequest) (RequestPackageReserveResult, error) {
	req := normalizePackageScopeRequest(input)
	if db == nil {
		return RequestPackageReserveResult{}, fmt.Errorf("database handle is nil")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return RequestPackageReserveResult{}, fmt.Errorf("用户 ID 不能为空")
	}
	matches, err := findMatchingRequestPackageSubscriptionsWithDB(db, req)
	if err != nil {
		return RequestPackageReserveResult{}, err
	}
	if len(matches) == 0 {
		return RequestPackageReserveResult{Matched: false, Allowed: false}, nil
	}
	var firstDenied RequestPackageReserveResult
	for _, subscription := range matches {
		result, err := reserveRequestPackageSubscriptionWithDB(db, subscription, req)
		if err != nil {
			return RequestPackageReserveResult{}, err
		}
		if result.Allowed {
			return result, nil
		}
		if !firstDenied.Matched {
			firstDenied = result
		}
	}
	return firstDenied, nil
}

func reserveRequestPackageSubscriptionWithDB(db *gorm.DB, subscription UserPackageSubscription, req PackageScopeRequest) (RequestPackageReserveResult, error) {
	normalizeServicePackageSubscriptionScopeAndQuota(&subscription)
	periodKey := requestPackagePeriodKey(subscription.PeriodType, req.Now, subscription.QuotaResetTimezone)
	result := RequestPackageReserveResult{
		Matched:      true,
		Allowed:      false,
		Subscription: subscription,
		Reason:       "request_quota_exhausted",
	}
	limit := normalizeServicePackagePeriodLimit(subscription.PeriodLimit, subscription.QuotaMetric, subscription.PeriodType, subscription.DailyQuotaLimit)
	if limit <= 0 {
		result.Reason = "request_quota_limit_unconfigured"
		return result, nil
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		counter, err := ensureUserPackageUsageCounterWithDB(tx, subscription, periodKey)
		if err != nil {
			return err
		}
		result.Remaining = limit - counter.ConsumedAmount - counter.ReservedAmount
		if result.Remaining < 0 {
			result.Remaining = 0
		}
		if counter.ConsumedAmount+counter.ReservedAmount+req.RequestAmount > limit {
			return nil
		}
		concurrencyResult, err := ReserveEntitlementConcurrencyWithDB(tx, EntitlementConcurrencyReserveInput{
			SourceType:               EntitlementConcurrencySourceServicePackage,
			SourceID:                 strings.TrimSpace(subscription.PackageID),
			SourceName:               strings.TrimSpace(subscription.PackageName),
			UserID:                   strings.TrimSpace(subscription.UserID),
			RequestCount:             req.RequestAmount,
			MaxConcurrencyPerUser:    subscription.MaxConcurrencyPerUser,
			MaxConcurrencyPerPackage: subscription.MaxConcurrencyPerPackage,
		})
		if err != nil {
			return err
		}
		if !concurrencyResult.Allowed {
			switch concurrencyResult.Reason {
			case EntitlementConcurrencyReasonPerUserExceeded:
				result.Reason = "request_concurrency_per_user_exceeded"
			case EntitlementConcurrencyReasonPerSourceExceeded:
				result.Reason = "request_concurrency_per_package_exceeded"
			}
			return nil
		}
		updatedAt := helper.GetTimestamp()
		if err := tx.Model(&UserPackageUsageCounter{}).
			Where("id = ?", counter.Id).
			Updates(map[string]any{
				"reserved_amount": gorm.Expr("reserved_amount + ?", req.RequestAmount),
				"limit_amount":    limit,
				"updated_at":      updatedAt,
			}).Error; err != nil {
			return err
		}
		result.Allowed = true
		result.Reason = ""
		result.Concurrency = concurrencyResult.Reservation
		result.Remaining = limit - counter.ConsumedAmount - counter.ReservedAmount - req.RequestAmount
		if result.Remaining < 0 {
			result.Remaining = 0
		}
		result.Reservation = RequestPackageReservation{
			CounterID:      counter.Id,
			SubscriptionID: strings.TrimSpace(subscription.Id),
			UserID:         strings.TrimSpace(subscription.UserID),
			PackageID:      strings.TrimSpace(subscription.PackageID),
			GroupID:        strings.TrimSpace(subscription.GroupID),
			Metric:         strings.TrimSpace(subscription.QuotaMetric),
			ScopeType:      strings.TrimSpace(subscription.ScopeType),
			ScopeProvider:  strings.TrimSpace(subscription.ScopeProvider),
			ScopeModel:     strings.TrimSpace(subscription.ScopeModel),
			ScopeEndpoint:  strings.TrimSpace(subscription.ScopeEndpoint),
			PeriodType:     strings.TrimSpace(subscription.PeriodType),
			PeriodKey:      strings.TrimSpace(periodKey),
			ReservedAmount: req.RequestAmount,
			LimitAmount:    limit,
			Concurrency:    concurrencyResult.Reservation,
		}
		return nil
	})
	return result, err
}

func ReserveRequestPackage(input PackageScopeRequest) (RequestPackageReserveResult, error) {
	return ReserveRequestPackageWithDB(DB, input)
}

func ReleaseRequestPackageReservationWithDB(db *gorm.DB, reservation RequestPackageReservation) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !reservation.Active() {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&UserPackageUsageCounter{}).
			Where("id = ?", strings.TrimSpace(reservation.CounterID)).
			Updates(map[string]any{
				"reserved_amount": gorm.Expr("CASE WHEN reserved_amount >= ? THEN reserved_amount - ? ELSE 0 END", reservation.ReservedAmount, reservation.ReservedAmount),
				"updated_at":      helper.GetTimestamp(),
			}).Error; err != nil {
			return err
		}
		return ReleaseEntitlementConcurrencyReservationWithDB(tx, reservation.Concurrency)
	})
}

func ReleaseRequestPackageReservation(reservation RequestPackageReservation) error {
	return ReleaseRequestPackageReservationWithDB(DB, reservation)
}

func SettleRequestPackageReservationWithDB(db *gorm.DB, reservation RequestPackageReservation, consumedAmount int64) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database handle is nil")
	}
	if !reservation.Active() {
		return 0, nil
	}
	consumed := consumedAmount
	if consumed < 0 {
		consumed = 0
	}
	if consumed == 0 {
		consumed = reservation.ReservedAmount
	}
	var settled int64
	err := db.Transaction(func(tx *gorm.DB) error {
		counter := UserPackageUsageCounter{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", strings.TrimSpace(reservation.CounterID)).
			Take(&counter).Error; err != nil {
			return err
		}
		headroom := reservation.LimitAmount - counter.ConsumedAmount
		if headroom < 0 {
			headroom = 0
		}
		settled = minInt64(consumed, headroom)
		return tx.Model(&UserPackageUsageCounter{}).
			Where("id = ?", counter.Id).
			Updates(map[string]any{
				"reserved_amount": gorm.Expr("CASE WHEN reserved_amount >= ? THEN reserved_amount - ? ELSE 0 END", reservation.ReservedAmount, reservation.ReservedAmount),
				"consumed_amount": gorm.Expr("consumed_amount + ?", settled),
				"limit_amount":    reservation.LimitAmount,
				"updated_at":      helper.GetTimestamp(),
			}).Error
	})
	if err == nil {
		err = ReleaseEntitlementConcurrencyReservationWithDB(db, reservation.Concurrency)
	}
	return settled, err
}

func SettleRequestPackageReservation(reservation RequestPackageReservation, consumedAmount int64) (int64, error) {
	return SettleRequestPackageReservationWithDB(DB, reservation, consumedAmount)
}
