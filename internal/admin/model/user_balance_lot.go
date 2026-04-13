package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	UserBalanceLotsTableName = "user_balance_lots"

	UserBalanceLotStatusActive   = "active"
	UserBalanceLotStatusExhaust  = "exhausted"
	UserBalanceLotStatusExpired  = "expired"
	UserBalanceLotSourceTopup    = "topup_order"
	UserBalanceLotSourceRedeem   = "redemption"
	UserBalanceLotSourceLegacy   = "legacy_migration"
	UserBalanceLotMaxValidityDay = 3650
)

type UserBalanceLot struct {
	Id           string `json:"id" gorm:"primaryKey;type:char(36)"`
	UserID       string `json:"user_id" gorm:"type:char(36);not null;index:idx_balance_lot_user_status_expire,priority:1"`
	SourceType   string `json:"source_type" gorm:"type:varchar(32);not null;index:idx_balance_lot_source,unique,priority:1"`
	SourceID     string `json:"source_id" gorm:"type:char(36);not null;index:idx_balance_lot_source,unique,priority:2"`
	TotalYYC     int64  `json:"total_yyc" gorm:"type:bigint;not null;default:0"`
	UsedYYC      int64  `json:"used_yyc" gorm:"type:bigint;not null;default:0"`
	RemainingYYC int64  `json:"remaining_yyc" gorm:"type:bigint;not null;default:0;index:idx_balance_lot_user_status_expire,priority:2"`
	Status       string `json:"status" gorm:"type:varchar(16);not null;default:'active';index:idx_balance_lot_user_status_expire,priority:3"`
	GrantedAt    int64  `json:"granted_at" gorm:"bigint;not null;default:0;index"`
	ExpiresAt    int64  `json:"expires_at" gorm:"bigint;not null;default:0;index:idx_balance_lot_user_status_expire,priority:4"`
	ExpiredAt    int64  `json:"expired_at" gorm:"bigint;not null;default:0;index"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint;index"`
}

type UserBalanceLotCreditInput struct {
	UserID     string
	SourceType string
	SourceID   string
	TotalYYC   int64
	GrantedAt  int64
	ExpiresAt  int64
}

func (UserBalanceLot) TableName() string {
	return UserBalanceLotsTableName
}

func normalizeTopupPlanValidityDays(value int) int {
	switch {
	case value < 0:
		return 0
	case value > UserBalanceLotMaxValidityDay:
		return UserBalanceLotMaxValidityDay
	default:
		return value
	}
}

func resolveBalanceCreditExpiresAt(grantedAt int64, validityDays int) int64 {
	normalizedDays := normalizeTopupPlanValidityDays(validityDays)
	if normalizedDays <= 0 {
		return 0
	}
	if grantedAt <= 0 {
		grantedAt = helper.GetTimestamp()
	}
	locationName := normalizeGroupQuotaResetTimezone(DefaultGroupQuotaResetTimezone)
	location, err := time.LoadLocation(locationName)
	if err != nil {
		location = time.FixedZone(DefaultGroupQuotaResetTimezone, 8*3600)
	}
	bizTime := time.Unix(grantedAt, 0).In(location)
	dayStart := time.Date(bizTime.Year(), bizTime.Month(), bizTime.Day(), 0, 0, 0, 0, location)
	return dayStart.Add(time.Duration(normalizedDays)*24*time.Hour - time.Second).Unix()
}

func ResolveBalanceCreditExpiresAt(grantedAt int64, validityDays int) int64 {
	return resolveBalanceCreditExpiresAt(grantedAt, validityDays)
}

func normalizeUserBalanceLotStatus(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case UserBalanceLotStatusActive:
		return UserBalanceLotStatusActive
	case UserBalanceLotStatusExhaust:
		return UserBalanceLotStatusExhaust
	case UserBalanceLotStatusExpired:
		return UserBalanceLotStatusExpired
	default:
		return UserBalanceLotStatusActive
	}
}

func normalizeUserBalanceLotSourceType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case UserBalanceLotSourceTopup:
		return UserBalanceLotSourceTopup
	case UserBalanceLotSourceRedeem:
		return UserBalanceLotSourceRedeem
	case UserBalanceLotSourceLegacy:
		return UserBalanceLotSourceLegacy
	default:
		return strings.TrimSpace(strings.ToLower(value))
	}
}

func normalizeUserBalanceLotSourceFilter(value string) string {
	switch normalizeUserBalanceLotSourceType(value) {
	case UserBalanceLotSourceTopup, UserBalanceLotSourceRedeem, UserBalanceLotSourceLegacy:
		return normalizeUserBalanceLotSourceType(value)
	default:
		return ""
	}
}

func normalizeUserBalanceLotStatusFilter(value string) string {
	switch normalizeUserBalanceLotStatus(value) {
	case UserBalanceLotStatusActive, UserBalanceLotStatusExhaust, UserBalanceLotStatusExpired:
		return normalizeUserBalanceLotStatus(value)
	default:
		return ""
	}
}

func normalizeUserBalanceLotRow(row *UserBalanceLot) {
	if row == nil {
		return
	}
	row.Id = strings.TrimSpace(row.Id)
	if row.Id == "" {
		row.Id = random.GetUUID()
	}
	row.UserID = strings.TrimSpace(row.UserID)
	row.SourceType = normalizeUserBalanceLotSourceType(row.SourceType)
	row.SourceID = strings.TrimSpace(row.SourceID)
	if row.TotalYYC < 0 {
		row.TotalYYC = 0
	}
	if row.UsedYYC < 0 {
		row.UsedYYC = 0
	}
	if row.RemainingYYC < 0 {
		row.RemainingYYC = 0
	}
	if row.RemainingYYC > row.TotalYYC {
		row.RemainingYYC = row.TotalYYC
	}
	if row.UsedYYC+row.RemainingYYC > row.TotalYYC {
		row.UsedYYC = row.TotalYYC - row.RemainingYYC
	}
	if row.GrantedAt < 0 {
		row.GrantedAt = 0
	}
	if row.ExpiresAt < 0 {
		row.ExpiresAt = 0
	}
	if row.ExpiredAt < 0 {
		row.ExpiredAt = 0
	}
	row.Status = normalizeUserBalanceLotStatus(row.Status)
}

func CreditUserBalanceLotWithDB(db *gorm.DB, input UserBalanceLotCreditInput) (UserBalanceLot, bool, error) {
	if db == nil {
		return UserBalanceLot{}, false, fmt.Errorf("database handle is nil")
	}
	row := UserBalanceLot{
		Id:           random.GetUUID(),
		UserID:       strings.TrimSpace(input.UserID),
		SourceType:   normalizeUserBalanceLotSourceType(input.SourceType),
		SourceID:     strings.TrimSpace(input.SourceID),
		TotalYYC:     input.TotalYYC,
		UsedYYC:      0,
		RemainingYYC: input.TotalYYC,
		GrantedAt:    input.GrantedAt,
		ExpiresAt:    input.ExpiresAt,
		Status:       UserBalanceLotStatusActive,
	}
	normalizeUserBalanceLotRow(&row)
	if row.UserID == "" {
		return UserBalanceLot{}, false, fmt.Errorf("用户 ID 不能为空")
	}
	if row.SourceType == "" {
		return UserBalanceLot{}, false, fmt.Errorf("来源类型不能为空")
	}
	if row.SourceID == "" {
		return UserBalanceLot{}, false, fmt.Errorf("来源 ID 不能为空")
	}
	if row.TotalYYC <= 0 {
		return UserBalanceLot{}, false, fmt.Errorf("额度必须大于 0")
	}
	if row.GrantedAt <= 0 {
		row.GrantedAt = helper.GetTimestamp()
	}
	if row.ExpiresAt > 0 && row.ExpiresAt < row.GrantedAt {
		row.ExpiresAt = row.GrantedAt
	}
	now := helper.GetTimestamp()
	row.CreatedAt = now
	row.UpdatedAt = now
	createResult := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "source_type"}, {Name: "source_id"}},
		DoNothing: true,
	}).Create(&row)
	if createResult.Error != nil {
		return UserBalanceLot{}, false, createResult.Error
	}
	existing := UserBalanceLot{}
	if err := db.Where("source_type = ? AND source_id = ?", row.SourceType, row.SourceID).First(&existing).Error; err != nil {
		return UserBalanceLot{}, false, err
	}
	row = existing
	normalizeUserBalanceLotRow(&row)
	created := createResult.RowsAffected > 0
	if created {
		if _, err := CreateUserBalanceLotTransactionWithDB(db, UserBalanceLotTransactionInput{
			UserID:             row.UserID,
			LotID:              row.Id,
			SourceType:         row.SourceType,
			SourceID:           row.SourceID,
			TxType:             UserBalanceLotTxTypeCredit,
			DeltaYYC:           row.TotalYYC,
			LotRemainingBefore: 0,
			LotRemainingAfter:  row.RemainingYYC,
			OccurredAt:         row.GrantedAt,
		}); err != nil {
			return UserBalanceLot{}, false, err
		}
	}
	return row, created, nil
}

func expireUserBalanceLotsInTx(tx *gorm.DB, userID string, now int64) (int64, error) {
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return 0, nil
	}
	effectiveNow := now
	if effectiveNow <= 0 {
		effectiveNow = helper.GetTimestamp()
	}
	rows := make([]UserBalanceLot, 0)
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND status = ? AND remaining_yyc > 0 AND expires_at > 0 AND expires_at <= ?", normalizedUserID, UserBalanceLotStatusActive, effectiveNow).
		Order("expires_at asc, created_at asc, id asc").
		Find(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	expiredTotal := int64(0)
	for _, row := range rows {
		if row.RemainingYYC <= 0 {
			continue
		}
		expiredAmount := row.RemainingYYC
		updated := map[string]any{
			"used_yyc":      row.UsedYYC + expiredAmount,
			"remaining_yyc": 0,
			"status":        UserBalanceLotStatusExpired,
			"expired_at":    effectiveNow,
			"updated_at":    effectiveNow,
		}
		if err := tx.Model(&UserBalanceLot{}).Where("id = ?", row.Id).Updates(updated).Error; err != nil {
			return 0, err
		}
		if _, err := CreateUserBalanceLotTransactionWithDB(tx, UserBalanceLotTransactionInput{
			UserID:             row.UserID,
			LotID:              row.Id,
			SourceType:         row.SourceType,
			SourceID:           row.SourceID,
			TxType:             UserBalanceLotTxTypeExpire,
			DeltaYYC:           -expiredAmount,
			LotRemainingBefore: row.RemainingYYC,
			LotRemainingAfter:  0,
			OccurredAt:         effectiveNow,
		}); err != nil {
			return 0, err
		}
		expiredTotal += expiredAmount
	}
	if expiredTotal <= 0 {
		return 0, nil
	}
	user := User{}
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Select("id", "quota").Where("id = ?", normalizedUserID).First(&user).Error; err != nil {
		return 0, err
	}
	newQuota := user.Quota - expiredTotal
	if newQuota < 0 {
		newQuota = 0
	}
	if err := tx.Model(&User{}).Where("id = ?", normalizedUserID).Updates(map[string]any{
		"quota":      newQuota,
		"updated_at": effectiveNow,
	}).Error; err != nil {
		return 0, err
	}
	return expiredTotal, nil
}

func ExpireUserBalanceLotsWithDB(db *gorm.DB, userID string, now int64) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database handle is nil")
	}
	expiredTotal := int64(0)
	if err := db.Transaction(func(tx *gorm.DB) error {
		amount, err := expireUserBalanceLotsInTx(tx, userID, now)
		if err != nil {
			return err
		}
		expiredTotal = amount
		return nil
	}); err != nil {
		return 0, err
	}
	return expiredTotal, nil
}

func ExpireUserBalanceLots(userID string) (int64, error) {
	return ExpireUserBalanceLotsWithDB(DB, userID, helper.GetTimestamp())
}

func ConsumeUserBalanceLotsWithDB(db *gorm.DB, userID string, quota int64, now int64) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" || quota <= 0 {
		return 0, nil
	}
	consumed := int64(0)
	err := db.Transaction(func(tx *gorm.DB) error {
		effectiveNow := now
		if effectiveNow <= 0 {
			effectiveNow = helper.GetTimestamp()
		}
		if _, err := expireUserBalanceLotsInTx(tx, normalizedUserID, effectiveNow); err != nil {
			return err
		}
		rows := make([]UserBalanceLot, 0)
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND status = ? AND remaining_yyc > 0 AND (expires_at = 0 OR expires_at > ?)", normalizedUserID, UserBalanceLotStatusActive, effectiveNow).
			Order("CASE WHEN expires_at = 0 THEN 1 ELSE 0 END asc, expires_at asc, created_at asc, id asc").
			Find(&rows).Error; err != nil {
			return err
		}
		remainingToConsume := quota
		for _, row := range rows {
			if remainingToConsume <= 0 {
				break
			}
			available := row.RemainingYYC
			if available <= 0 {
				continue
			}
			delta := minInt64(available, remainingToConsume)
			nextRemaining := available - delta
			nextStatus := UserBalanceLotStatusActive
			if nextRemaining <= 0 {
				nextRemaining = 0
				nextStatus = UserBalanceLotStatusExhaust
			}
			if err := tx.Model(&UserBalanceLot{}).Where("id = ?", row.Id).Updates(map[string]any{
				"used_yyc":      row.UsedYYC + delta,
				"remaining_yyc": nextRemaining,
				"status":        nextStatus,
				"updated_at":    effectiveNow,
			}).Error; err != nil {
				return err
			}
			if _, err := CreateUserBalanceLotTransactionWithDB(tx, UserBalanceLotTransactionInput{
				UserID:             row.UserID,
				LotID:              row.Id,
				SourceType:         row.SourceType,
				SourceID:           row.SourceID,
				TxType:             UserBalanceLotTxTypeConsume,
				DeltaYYC:           -delta,
				LotRemainingBefore: available,
				LotRemainingAfter:  nextRemaining,
				OccurredAt:         effectiveNow,
			}); err != nil {
				return err
			}
			remainingToConsume -= delta
			consumed += delta
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return consumed, nil
}

func ConsumeUserBalanceLots(userID string, quota int64) (int64, error) {
	return ConsumeUserBalanceLotsWithDB(DB, userID, quota, helper.GetTimestamp())
}

func ListUserBalanceLotsPageWithDB(db *gorm.DB, userID string, sourceType string, status string, page int, pageSize int, positiveOnly bool) ([]UserBalanceLot, int64, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return nil, 0, fmt.Errorf("用户 ID 不能为空")
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	query := db.Model(&UserBalanceLot{}).Where("user_id = ?", normalizedUserID)
	if normalizedSource := normalizeUserBalanceLotSourceFilter(sourceType); normalizedSource != "" {
		query = query.Where("source_type = ?", normalizedSource)
	}
	if normalizedStatus := normalizeUserBalanceLotStatusFilter(status); normalizedStatus != "" {
		query = query.Where("status = ?", normalizedStatus)
	}
	if positiveOnly {
		query = query.Where("remaining_yyc > 0")
	}
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]UserBalanceLot, 0, pageSize)
	orderExpr := "CASE WHEN expires_at = 0 THEN 1 ELSE 0 END asc, expires_at asc, created_at desc, id desc"
	if err := query.Order(orderExpr).Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	for i := range rows {
		normalizeUserBalanceLotRow(&rows[i])
	}
	return rows, total, nil
}
