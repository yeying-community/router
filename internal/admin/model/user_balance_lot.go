package model

import (
	"fmt"
	"strings"

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
	UserBalanceLotMaxValidityDay = 3650

	UserBalanceLotQuotaCardKindTopup      = "topup"
	UserBalanceLotQuotaCardKindRedemption = "redemption"
	UserBalanceLotQuotaCardKindGift       = "gift"
)

type UserBalanceLot struct {
	Id              string `json:"id" gorm:"primaryKey;type:char(36)"`
	UserID          string `json:"user_id" gorm:"type:char(36);not null;index:idx_balance_lot_user_status_expire,priority:1"`
	SourceType      string `json:"source_type" gorm:"type:varchar(32);not null;index:idx_balance_lot_source,unique,priority:1"`
	SourceID        string `json:"source_id" gorm:"type:char(36);not null;index:idx_balance_lot_source,unique,priority:2"`
	TotalAmount     int64  `json:"total_amount" gorm:"type:bigint;not null;default:0"`
	UsedAmount      int64  `json:"used_amount" gorm:"type:bigint;not null;default:0"`
	RemainingAmount int64  `json:"remaining_amount" gorm:"type:bigint;not null;default:0;index:idx_balance_lot_user_status_expire,priority:2"`
	Status          string `json:"status" gorm:"type:varchar(16);not null;default:'active';index:idx_balance_lot_user_status_expire,priority:3"`
	GrantedAt       int64  `json:"granted_at" gorm:"bigint;not null;default:0;index"`
	ExpiresAt       int64  `json:"expires_at" gorm:"bigint;not null;default:0;index:idx_balance_lot_user_status_expire,priority:4"`
	ExpiredAt       int64  `json:"expired_at" gorm:"bigint;not null;default:0;index"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint;index"`
}

type UserBalanceLotCreditInput struct {
	UserID      string
	SourceType  string
	SourceID    string
	TotalAmount int64
	GrantedAt   int64
	ExpiresAt   int64
}

type UserBalanceLotConsumeResult struct {
	ConsumedAmount int64
	Source         LogBillingSourceSnapshot
}

func (result UserBalanceLotConsumeResult) LogBillingSourceSnapshot() LogBillingSourceSnapshot {
	return result.Source
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
	// Validity is a precise rolling window: granted_at + N * 24h.
	// Example: 2026-04-13 15:12:22 + 3 days => 2026-04-16 15:12:22.
	return grantedAt + int64(normalizedDays)*86400
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
	default:
		return strings.TrimSpace(strings.ToLower(value))
	}
}

func normalizeUserBalanceLotSourceFilter(value string) string {
	switch normalizeUserBalanceLotSourceType(value) {
	case UserBalanceLotSourceTopup, UserBalanceLotSourceRedeem:
		return normalizeUserBalanceLotSourceType(value)
	default:
		return ""
	}
}

func normalizeUserBalanceLotStatusFilter(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case UserBalanceLotStatusActive, UserBalanceLotStatusExhaust, UserBalanceLotStatusExpired:
		return strings.TrimSpace(strings.ToLower(value))
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
	if row.TotalAmount < 0 {
		row.TotalAmount = 0
	}
	if row.UsedAmount < 0 {
		row.UsedAmount = 0
	}
	if row.RemainingAmount < 0 {
		row.RemainingAmount = 0
	}
	if row.RemainingAmount > row.TotalAmount {
		row.RemainingAmount = row.TotalAmount
	}
	if row.UsedAmount+row.RemainingAmount > row.TotalAmount {
		row.UsedAmount = row.TotalAmount - row.RemainingAmount
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

func ListRecentUserBalanceLotsWithDB(db *gorm.DB, userID string, activeOnly bool, limit int) ([]UserBalanceLot, int64, error) {
	return ListRecentUserBalanceLotsByQuotaKindWithDB(db, userID, activeOnly, "", limit)
}

func ListRecentUserBalanceLotsByQuotaKindWithDB(db *gorm.DB, userID string, activeOnly bool, kind string, limit int) ([]UserBalanceLot, int64, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return nil, 0, fmt.Errorf("用户 ID 不能为空")
	}
	lotTable := UserBalanceLotsTableName
	query := db.Model(&UserBalanceLot{}).Where(lotTable+".user_id = ?", normalizedUserID)
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case "":
	case UserBalanceLotQuotaCardKindTopup:
		query = query.
			Joins("LEFT JOIN "+TopupOrdersTableName+" AS quota_card_orders ON quota_card_orders.id = "+lotTable+".source_id").
			Where(lotTable+".source_type = ?", UserBalanceLotSourceTopup).
			Where(
				"(quota_card_orders.id IS NULL OR COALESCE(quota_card_orders.credit_origin, '') NOT IN ?)",
				TopupOrderGiftCreditOriginValues(),
			)
	case UserBalanceLotQuotaCardKindRedemption:
		query = query.Where(lotTable+".source_type = ?", UserBalanceLotSourceRedeem)
	case UserBalanceLotQuotaCardKindGift:
		query = query.
			Joins("JOIN "+TopupOrdersTableName+" AS quota_card_orders ON quota_card_orders.id = "+lotTable+".source_id").
			Where(lotTable+".source_type = ?", UserBalanceLotSourceTopup).
			Where("quota_card_orders.credit_origin IN ?", TopupOrderGiftCreditOriginValues())
	default:
		return nil, 0, fmt.Errorf("无效的额度卡片类型")
	}
	if activeOnly {
		now := helper.GetTimestamp()
		query = query.
			Where(lotTable+".status = ? AND "+lotTable+".remaining_amount > 0", UserBalanceLotStatusActive).
			Where("("+lotTable+".expires_at = 0 OR "+lotTable+".expires_at > ?)", now)
	}
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 5000 {
		limit = 5000
	}
	rows := make([]UserBalanceLot, 0, limit)
	if err := query.
		Order(lotTable + ".granted_at desc, " + lotTable + ".created_at desc, " + lotTable + ".id desc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	for i := range rows {
		normalizeUserBalanceLotRow(&rows[i])
	}
	return rows, total, nil
}

func GetUserBalanceLotByIDWithDB(db *gorm.DB, userID string, lotID string) (UserBalanceLot, error) {
	if db == nil {
		return UserBalanceLot{}, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	normalizedID := strings.TrimSpace(lotID)
	if normalizedUserID == "" || normalizedID == "" {
		return UserBalanceLot{}, gorm.ErrRecordNotFound
	}
	row := UserBalanceLot{}
	if err := db.Where("id = ? AND user_id = ?", normalizedID, normalizedUserID).First(&row).Error; err != nil {
		return UserBalanceLot{}, err
	}
	normalizeUserBalanceLotRow(&row)
	return row, nil
}

func CreditUserBalanceLotWithDB(db *gorm.DB, input UserBalanceLotCreditInput) (UserBalanceLot, bool, error) {
	if db == nil {
		return UserBalanceLot{}, false, fmt.Errorf("database handle is nil")
	}
	row := UserBalanceLot{
		Id:              random.GetUUID(),
		UserID:          strings.TrimSpace(input.UserID),
		SourceType:      normalizeUserBalanceLotSourceType(input.SourceType),
		SourceID:        strings.TrimSpace(input.SourceID),
		TotalAmount:     input.TotalAmount,
		UsedAmount:      0,
		RemainingAmount: input.TotalAmount,
		GrantedAt:       input.GrantedAt,
		ExpiresAt:       input.ExpiresAt,
		Status:          UserBalanceLotStatusActive,
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
	if row.TotalAmount <= 0 {
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
			DeltaAmount:        row.TotalAmount,
			LotRemainingBefore: 0,
			LotRemainingAfter:  row.RemainingAmount,
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
		Where("user_id = ? AND status = ? AND remaining_amount > 0 AND expires_at > 0 AND expires_at <= ?", normalizedUserID, UserBalanceLotStatusActive, effectiveNow).
		Order("expires_at asc, created_at asc, id asc").
		Find(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	expiredTotal := int64(0)
	for _, row := range rows {
		if row.RemainingAmount <= 0 {
			continue
		}
		expiredAmount := row.RemainingAmount
		updated := map[string]any{
			"used_amount":      row.UsedAmount + expiredAmount,
			"remaining_amount": 0,
			"status":           UserBalanceLotStatusExpired,
			"expired_at":       effectiveNow,
			"updated_at":       effectiveNow,
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
			DeltaAmount:        -expiredAmount,
			LotRemainingBefore: row.RemainingAmount,
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
	if expiredTotal > 0 {
		RefreshUserGroupCaches(userID)
	}
	return expiredTotal, nil
}

func ExpireUserBalanceLots(userID string) (int64, error) {
	return ExpireUserBalanceLotsWithDB(DB, userID, helper.GetTimestamp())
}

func GetEffectiveUserBalanceAmountForGroupWithDB(db *gorm.DB, userID string, groupID string, now int64) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return 0, nil
	}
	normalizedGroupID := strings.TrimSpace(groupID)
	balanceAmount := int64(0)
	err := db.Transaction(func(tx *gorm.DB) error {
		effectiveNow := now
		if effectiveNow <= 0 {
			effectiveNow = helper.GetTimestamp()
		}
		if _, err := expireUserBalanceLotsInTx(tx, normalizedUserID, effectiveNow); err != nil {
			return err
		}
		query := tx.Model(&UserBalanceLot{}).
			Select("COALESCE(SUM(remaining_amount), 0)").
			Where("user_id = ? AND status = ? AND remaining_amount > 0 AND (expires_at = 0 OR expires_at > ?)", normalizedUserID, UserBalanceLotStatusActive, effectiveNow)
		if normalizedGroupID != "" {
			query = query.Where(`
				(
					source_type = ? AND EXISTS (
						SELECT 1 FROM topup_orders o
						WHERE o.id = user_balance_lots.source_id
						  AND o.business_type = ?
						  AND COALESCE(o.group_id, '') = ?
					)
				)
				OR
				(
					source_type = ? AND EXISTS (
						SELECT 1 FROM redemptions r
						WHERE r.id = user_balance_lots.source_id
						  AND COALESCE(r.group_id, '') = ?
					)
				)
			`, UserBalanceLotSourceTopup, TopupOrderBusinessBalance, normalizedGroupID, UserBalanceLotSourceRedeem, normalizedGroupID)
		}
		return query.Scan(&balanceAmount).Error
	})
	if err != nil {
		return 0, err
	}
	return balanceAmount, nil
}

func GetEffectiveUserBalanceAmountWithDB(db *gorm.DB, userID string, now int64) (int64, error) {
	return GetEffectiveUserBalanceAmountForGroupWithDB(db, userID, "", now)
}

func GetEffectiveUserBalanceAmountForGroup(userID string, groupID string) (int64, error) {
	return GetEffectiveUserBalanceAmountForGroupWithDB(DB, userID, groupID, helper.GetTimestamp())
}

func GetEffectiveUserBalanceAmount(userID string) (int64, error) {
	return GetEffectiveUserBalanceAmountWithDB(DB, userID, helper.GetTimestamp())
}

func ConsumeUserBalanceLotsForGroupDetailedWithDB(db *gorm.DB, userID string, groupID string, quota int64, now int64) (UserBalanceLotConsumeResult, error) {
	result := UserBalanceLotConsumeResult{}
	if db == nil {
		return result, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" || quota <= 0 {
		return result, nil
	}
	normalizedGroupID := strings.TrimSpace(groupID)
	err := db.Transaction(func(tx *gorm.DB) error {
		effectiveNow := now
		if effectiveNow <= 0 {
			effectiveNow = helper.GetTimestamp()
		}
		if _, err := expireUserBalanceLotsInTx(tx, normalizedUserID, effectiveNow); err != nil {
			return err
		}
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND status = ? AND remaining_amount > 0 AND (expires_at = 0 OR expires_at > ?)", normalizedUserID, UserBalanceLotStatusActive, effectiveNow).
			Order("CASE WHEN expires_at = 0 THEN 1 ELSE 0 END asc, expires_at asc, created_at asc, id asc")
		if normalizedGroupID != "" {
			query = query.Where(`
				(
					source_type = ? AND EXISTS (
						SELECT 1 FROM topup_orders o
						WHERE o.id = user_balance_lots.source_id
						  AND o.business_type = ?
						  AND COALESCE(o.group_id, '') = ?
					)
				)
				OR
				(
					source_type = ? AND EXISTS (
						SELECT 1 FROM redemptions r
						WHERE r.id = user_balance_lots.source_id
						  AND COALESCE(r.group_id, '') = ?
					)
				)
			`, UserBalanceLotSourceTopup, TopupOrderBusinessBalance, normalizedGroupID, UserBalanceLotSourceRedeem, normalizedGroupID)
		}
		rows := make([]UserBalanceLot, 0)
		if err := query.Find(&rows).Error; err != nil {
			return err
		}
		remainingToConsume := quota
		for _, row := range rows {
			if remainingToConsume <= 0 {
				break
			}
			available := row.RemainingAmount
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
				"used_amount":      row.UsedAmount + delta,
				"remaining_amount": nextRemaining,
				"status":           nextStatus,
				"updated_at":       effectiveNow,
			}).Error; err != nil {
				return err
			}
			if _, err := CreateUserBalanceLotTransactionWithDB(tx, UserBalanceLotTransactionInput{
				UserID:             row.UserID,
				LotID:              row.Id,
				SourceType:         row.SourceType,
				SourceID:           row.SourceID,
				TxType:             UserBalanceLotTxTypeConsume,
				DeltaAmount:        -delta,
				LotRemainingBefore: available,
				LotRemainingAfter:  nextRemaining,
				OccurredAt:         effectiveNow,
			}); err != nil {
				return err
			}
			if strings.TrimSpace(result.Source.ID) == "" {
				result.Source = LogBillingSourceSnapshot{
					ID:     strings.TrimSpace(row.SourceID),
					Name:   strings.TrimSpace(row.SourceID),
					Detail: strings.TrimSpace(row.SourceType),
				}
			}
			remainingToConsume -= delta
			result.ConsumedAmount += delta
		}
		return nil
	})
	if err != nil {
		return UserBalanceLotConsumeResult{}, err
	}
	if result.ConsumedAmount > 0 {
		RefreshUserGroupCaches(normalizedUserID)
	}
	result.Source = resolveBalanceLogBillingSourceSnapshotWithDB(db, result.Source)
	return result, nil
}

func resolveBalanceLogBillingSourceSnapshotWithDB(db *gorm.DB, source LogBillingSourceSnapshot) LogBillingSourceSnapshot {
	source.ID = strings.TrimSpace(source.ID)
	source.Name = strings.TrimSpace(source.Name)
	source.Detail = strings.TrimSpace(source.Detail)
	if db == nil || source.ID == "" {
		return source
	}
	switch source.Detail {
	case UserBalanceLotSourceTopup:
		order := TopupOrder{}
		if err := db.Select("id", "title", "package_name", "transaction_id").Where("id = ?", source.ID).First(&order).Error; err == nil {
			switch {
			case strings.TrimSpace(order.Title) != "":
				source.Name = strings.TrimSpace(order.Title)
			case strings.TrimSpace(order.PackageName) != "":
				source.Name = strings.TrimSpace(order.PackageName)
			case strings.TrimSpace(order.TransactionID) != "":
				source.Name = strings.TrimSpace(order.TransactionID)
			}
		}
	case UserBalanceLotSourceRedeem:
		redemption := Redemption{}
		if err := db.Select("id", "name", "code").Where("id = ?", source.ID).First(&redemption).Error; err == nil {
			switch {
			case strings.TrimSpace(redemption.Name) != "":
				source.Name = strings.TrimSpace(redemption.Name)
			case strings.TrimSpace(redemption.Code) != "":
				source.Name = strings.TrimSpace(redemption.Code)
			}
		}
	}
	return source
}

func ConsumeUserBalanceLotsForGroupWithDB(db *gorm.DB, userID string, groupID string, quota int64, now int64) (int64, error) {
	result, err := ConsumeUserBalanceLotsForGroupDetailedWithDB(db, userID, groupID, quota, now)
	if err != nil {
		return 0, err
	}
	return result.ConsumedAmount, nil
}

func ConsumeUserBalanceLotsWithDB(db *gorm.DB, userID string, quota int64, now int64) (int64, error) {
	return ConsumeUserBalanceLotsForGroupWithDB(db, userID, "", quota, now)
}

func ConsumeUserBalanceLotsForGroupDetailed(userID string, groupID string, quota int64) (UserBalanceLotConsumeResult, error) {
	return ConsumeUserBalanceLotsForGroupDetailedWithDB(DB, userID, groupID, quota, helper.GetTimestamp())
}

func ConsumeUserBalanceLotsForGroup(userID string, groupID string, quota int64) (int64, error) {
	return ConsumeUserBalanceLotsForGroupWithDB(DB, userID, groupID, quota, helper.GetTimestamp())
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
		query = query.Where("remaining_amount > 0")
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
