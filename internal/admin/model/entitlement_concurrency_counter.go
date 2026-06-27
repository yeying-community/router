package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const EntitlementConcurrencyCountersTableName = "entitlement_concurrency_counters"

const (
	EntitlementConcurrencySourceServicePackage = "service_package"
	EntitlementConcurrencySourceTopupPlan      = "topup_plan"

	EntitlementConcurrencyScopeUser   = "user"
	EntitlementConcurrencyScopeSource = "source"

	EntitlementConcurrencyReasonPerUserExceeded   = "concurrency_per_user_exceeded"
	EntitlementConcurrencyReasonPerSourceExceeded = "concurrency_per_source_exceeded"
)

type EntitlementConcurrencyCounter struct {
	SourceType  string `json:"source_type" gorm:"primaryKey;type:varchar(32)"`
	SourceID    string `json:"source_id" gorm:"primaryKey;type:char(36)"`
	ScopeType   string `json:"scope_type" gorm:"primaryKey;type:varchar(16)"`
	ScopeKey    string `json:"scope_key" gorm:"primaryKey;type:char(36)"`
	ActiveCount int64  `json:"active_count" gorm:"type:bigint;not null;default:0"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint;index"`
}

func (EntitlementConcurrencyCounter) TableName() string {
	return EntitlementConcurrencyCountersTableName
}

type EntitlementConcurrencyReservation struct {
	SourceType    string `json:"source_type"`
	SourceID      string `json:"source_id"`
	SourceName    string `json:"source_name"`
	UserID        string `json:"user_id"`
	ReservedCount int64  `json:"reserved_count"`
}

func (reservation EntitlementConcurrencyReservation) Active() bool {
	return strings.TrimSpace(reservation.SourceType) != "" &&
		strings.TrimSpace(reservation.SourceID) != "" &&
		strings.TrimSpace(reservation.UserID) != "" &&
		reservation.ReservedCount > 0
}

type EntitlementConcurrencyReserveInput struct {
	SourceType               string
	SourceID                 string
	SourceName               string
	UserID                   string
	RequestCount             int64
	MaxConcurrencyPerUser    int
	MaxConcurrencyPerPackage int
}

type EntitlementConcurrencyReserveResult struct {
	Allowed     bool
	Reason      string
	Reservation EntitlementConcurrencyReservation
}

func normalizeEntitlementConcurrencySourceType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case EntitlementConcurrencySourceServicePackage:
		return EntitlementConcurrencySourceServicePackage
	case EntitlementConcurrencySourceTopupPlan:
		return EntitlementConcurrencySourceTopupPlan
	default:
		return ""
	}
}

func normalizeEntitlementConcurrencyReserveInput(input EntitlementConcurrencyReserveInput) EntitlementConcurrencyReserveInput {
	input.SourceType = normalizeEntitlementConcurrencySourceType(input.SourceType)
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.SourceName = strings.TrimSpace(input.SourceName)
	input.UserID = strings.TrimSpace(input.UserID)
	if input.RequestCount <= 0 {
		input.RequestCount = 1
	}
	if input.MaxConcurrencyPerUser < 0 {
		input.MaxConcurrencyPerUser = 0
	}
	if input.MaxConcurrencyPerPackage < 0 {
		input.MaxConcurrencyPerPackage = 0
	}
	return input
}

func ensureEntitlementConcurrencyCounterWithDB(tx *gorm.DB, sourceType string, sourceID string, scopeType string, scopeKey string) (EntitlementConcurrencyCounter, error) {
	counter := EntitlementConcurrencyCounter{
		SourceType: strings.TrimSpace(sourceType),
		SourceID:   strings.TrimSpace(sourceID),
		ScopeType:  strings.TrimSpace(scopeType),
		ScopeKey:   strings.TrimSpace(scopeKey),
		UpdatedAt:  helper.GetTimestamp(),
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&counter).Error; err != nil {
		return EntitlementConcurrencyCounter{}, err
	}
	row := EntitlementConcurrencyCounter{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("source_type = ? AND source_id = ? AND scope_type = ? AND scope_key = ?",
			counter.SourceType,
			counter.SourceID,
			counter.ScopeType,
			counter.ScopeKey,
		).
		Take(&row).Error
	return row, err
}

func ReserveEntitlementConcurrencyWithDB(db *gorm.DB, input EntitlementConcurrencyReserveInput) (EntitlementConcurrencyReserveResult, error) {
	req := normalizeEntitlementConcurrencyReserveInput(input)
	if db == nil {
		return EntitlementConcurrencyReserveResult{}, fmt.Errorf("database handle is nil")
	}
	if req.SourceType == "" || req.SourceID == "" || req.UserID == "" {
		return EntitlementConcurrencyReserveResult{Allowed: true}, nil
	}
	if req.MaxConcurrencyPerUser <= 0 && req.MaxConcurrencyPerPackage <= 0 {
		return EntitlementConcurrencyReserveResult{Allowed: true}, nil
	}
	result := EntitlementConcurrencyReserveResult{}
	err := db.Transaction(func(tx *gorm.DB) error {
		if req.MaxConcurrencyPerUser > 0 {
			userCounter, err := ensureEntitlementConcurrencyCounterWithDB(
				tx,
				req.SourceType,
				req.SourceID,
				EntitlementConcurrencyScopeUser,
				req.UserID,
			)
			if err != nil {
				return err
			}
			if userCounter.ActiveCount+req.RequestCount > int64(req.MaxConcurrencyPerUser) {
				result.Reason = EntitlementConcurrencyReasonPerUserExceeded
				return nil
			}
		}
		if req.MaxConcurrencyPerPackage > 0 {
			sourceCounter, err := ensureEntitlementConcurrencyCounterWithDB(
				tx,
				req.SourceType,
				req.SourceID,
				EntitlementConcurrencyScopeSource,
				req.SourceID,
			)
			if err != nil {
				return err
			}
			if sourceCounter.ActiveCount+req.RequestCount > int64(req.MaxConcurrencyPerPackage) {
				result.Reason = EntitlementConcurrencyReasonPerSourceExceeded
				return nil
			}
		}
		updatedAt := helper.GetTimestamp()
		if req.MaxConcurrencyPerUser > 0 {
			if err := tx.Model(&EntitlementConcurrencyCounter{}).
				Where("source_type = ? AND source_id = ? AND scope_type = ? AND scope_key = ?",
					req.SourceType,
					req.SourceID,
					EntitlementConcurrencyScopeUser,
					req.UserID,
				).
				Updates(map[string]any{
					"active_count": gorm.Expr("active_count + ?", req.RequestCount),
					"updated_at":   updatedAt,
				}).Error; err != nil {
				return err
			}
		}
		if req.MaxConcurrencyPerPackage > 0 {
			if err := tx.Model(&EntitlementConcurrencyCounter{}).
				Where("source_type = ? AND source_id = ? AND scope_type = ? AND scope_key = ?",
					req.SourceType,
					req.SourceID,
					EntitlementConcurrencyScopeSource,
					req.SourceID,
				).
				Updates(map[string]any{
					"active_count": gorm.Expr("active_count + ?", req.RequestCount),
					"updated_at":   updatedAt,
				}).Error; err != nil {
				return err
			}
		}
		result.Allowed = true
		result.Reason = ""
		result.Reservation = EntitlementConcurrencyReservation{
			SourceType:    req.SourceType,
			SourceID:      req.SourceID,
			SourceName:    req.SourceName,
			UserID:        req.UserID,
			ReservedCount: req.RequestCount,
		}
		return nil
	})
	return result, err
}

func ReserveEntitlementConcurrency(input EntitlementConcurrencyReserveInput) (EntitlementConcurrencyReserveResult, error) {
	return ReserveEntitlementConcurrencyWithDB(DB, input)
}

func releaseEntitlementConcurrencyReservationWithDB(db *gorm.DB, reservation EntitlementConcurrencyReservation) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !reservation.Active() {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		updatedAt := helper.GetTimestamp()
		if err := tx.Model(&EntitlementConcurrencyCounter{}).
			Where("source_type = ? AND source_id = ? AND scope_type = ? AND scope_key = ?",
				strings.TrimSpace(reservation.SourceType),
				strings.TrimSpace(reservation.SourceID),
				EntitlementConcurrencyScopeUser,
				strings.TrimSpace(reservation.UserID),
			).
			Updates(map[string]any{
				"active_count": gorm.Expr("CASE WHEN active_count >= ? THEN active_count - ? ELSE 0 END", reservation.ReservedCount, reservation.ReservedCount),
				"updated_at":   updatedAt,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&EntitlementConcurrencyCounter{}).
			Where("source_type = ? AND source_id = ? AND scope_type = ? AND scope_key = ?",
				strings.TrimSpace(reservation.SourceType),
				strings.TrimSpace(reservation.SourceID),
				EntitlementConcurrencyScopeSource,
				strings.TrimSpace(reservation.SourceID),
			).
			Updates(map[string]any{
				"active_count": gorm.Expr("CASE WHEN active_count >= ? THEN active_count - ? ELSE 0 END", reservation.ReservedCount, reservation.ReservedCount),
				"updated_at":   updatedAt,
			}).Error
	})
}

func ReleaseEntitlementConcurrencyReservationWithDB(db *gorm.DB, reservation EntitlementConcurrencyReservation) error {
	return releaseEntitlementConcurrencyReservationWithDB(db, reservation)
}

func ReleaseEntitlementConcurrencyReservation(reservation EntitlementConcurrencyReservation) error {
	return releaseEntitlementConcurrencyReservationWithDB(DB, reservation)
}

type ActiveTopupConcurrencyEntitlement struct {
	TopupOrderID             string `json:"topup_order_id"`
	TopupPlanID              string `json:"topup_plan_id"`
	GroupID                  string `json:"group_id"`
	Title                    string `json:"title"`
	MaxConcurrencyPerUser    int    `json:"max_concurrency_per_user"`
	MaxConcurrencyPerPackage int    `json:"max_concurrency_per_package"`
}

func GetActiveTopupConcurrencyEntitlementForGroupWithDB(db *gorm.DB, userID string, groupID string) (ActiveTopupConcurrencyEntitlement, error) {
	if db == nil {
		return ActiveTopupConcurrencyEntitlement{}, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return ActiveTopupConcurrencyEntitlement{}, fmt.Errorf("用户 ID 不能为空")
	}
	normalizedGroupID := strings.TrimSpace(groupID)
	now := helper.GetTimestamp()
	row := ActiveTopupConcurrencyEntitlement{}
	query := db.Table(UserBalanceLotsTableName+" AS l").
		Select(
			"o.id AS topup_order_id",
			"o.topup_plan_id",
			"o.group_id",
			"o.title",
			"o.max_concurrency_per_user",
			"o.max_concurrency_per_package",
		).
		Joins("JOIN "+TopupOrdersTableName+" AS o ON o.id = l.source_id").
		Where("l.user_id = ? AND l.source_type = ? AND l.status = ? AND l.remaining_amount > 0 AND (l.expires_at = 0 OR l.expires_at > ?)",
			normalizedUserID,
			UserBalanceLotSourceTopup,
			UserBalanceLotStatusActive,
			now,
		).
		Where("o.business_type = ?", TopupOrderBusinessBalance)
	if normalizedGroupID != "" {
		query = query.Where("COALESCE(o.group_id, '') = ?", normalizedGroupID)
	}
	err := query.Order("l.granted_at desc, l.created_at desc, l.id desc").Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ActiveTopupConcurrencyEntitlement{}, gorm.ErrRecordNotFound
		}
		return ActiveTopupConcurrencyEntitlement{}, err
	}
	row.TopupOrderID = strings.TrimSpace(row.TopupOrderID)
	row.TopupPlanID = strings.TrimSpace(row.TopupPlanID)
	row.GroupID = strings.TrimSpace(row.GroupID)
	row.Title = strings.TrimSpace(row.Title)
	if row.MaxConcurrencyPerUser < 0 {
		row.MaxConcurrencyPerUser = 0
	}
	if row.MaxConcurrencyPerPackage < 0 {
		row.MaxConcurrencyPerPackage = 0
	}
	return row, nil
}

func GetActiveTopupConcurrencyEntitlementForGroup(userID string, groupID string) (ActiveTopupConcurrencyEntitlement, error) {
	return GetActiveTopupConcurrencyEntitlementForGroupWithDB(DB, userID, groupID)
}
