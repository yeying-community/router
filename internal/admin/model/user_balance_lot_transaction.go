package model

import (
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"gorm.io/gorm"
)

const (
	UserBalanceLotTransactionsTableName = "user_balance_lot_transactions"

	UserBalanceLotTxTypeCredit  = "credit"
	UserBalanceLotTxTypeConsume = "consume"
	UserBalanceLotTxTypeExpire  = "expire"
)

type UserBalanceLotTransaction struct {
	Id                 string `json:"id" gorm:"primaryKey;type:char(36)"`
	UserID             string `json:"user_id" gorm:"type:char(36);not null;index:idx_balance_lot_tx_user_time,priority:1"`
	LotID              string `json:"lot_id" gorm:"type:char(36);not null;index:idx_balance_lot_tx_lot_time,priority:1"`
	SourceType         string `json:"source_type" gorm:"type:varchar(32);not null;index:idx_balance_lot_tx_source,priority:1"`
	SourceID           string `json:"source_id" gorm:"type:char(36);not null;index:idx_balance_lot_tx_source,priority:2"`
	TxType             string `json:"tx_type" gorm:"type:varchar(16);not null;index:idx_balance_lot_tx_user_time,priority:2"`
	DeltaAmount        int64  `json:"delta_amount" gorm:"type:bigint;not null;default:0"`
	LotRemainingBefore int64  `json:"lot_remaining_before" gorm:"type:bigint;not null;default:0"`
	LotRemainingAfter  int64  `json:"lot_remaining_after" gorm:"type:bigint;not null;default:0"`
	OccurredAt         int64  `json:"occurred_at" gorm:"bigint;not null;default:0;index:idx_balance_lot_tx_user_time,priority:3;index:idx_balance_lot_tx_lot_time,priority:2"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint;index"`
}

type UserBalanceLotTransactionInput struct {
	UserID             string
	LotID              string
	SourceType         string
	SourceID           string
	TxType             string
	DeltaAmount        int64
	LotRemainingBefore int64
	LotRemainingAfter  int64
	OccurredAt         int64
}

func (UserBalanceLotTransaction) TableName() string {
	return UserBalanceLotTransactionsTableName
}

func normalizeUserBalanceLotTxType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case UserBalanceLotTxTypeCredit:
		return UserBalanceLotTxTypeCredit
	case UserBalanceLotTxTypeConsume:
		return UserBalanceLotTxTypeConsume
	case UserBalanceLotTxTypeExpire:
		return UserBalanceLotTxTypeExpire
	default:
		return ""
	}
}

func normalizeUserBalanceLotTxTypeFilter(value string) string {
	return normalizeUserBalanceLotTxType(value)
}

func normalizeUserBalanceLotTransactionRow(row *UserBalanceLotTransaction) {
	if row == nil {
		return
	}
	row.Id = strings.TrimSpace(row.Id)
	if row.Id == "" {
		row.Id = random.GetUUID()
	}
	row.UserID = strings.TrimSpace(row.UserID)
	row.LotID = strings.TrimSpace(row.LotID)
	row.SourceType = normalizeUserBalanceLotSourceType(row.SourceType)
	row.SourceID = strings.TrimSpace(row.SourceID)
	row.TxType = normalizeUserBalanceLotTxType(row.TxType)
	if row.LotRemainingBefore < 0 {
		row.LotRemainingBefore = 0
	}
	if row.LotRemainingAfter < 0 {
		row.LotRemainingAfter = 0
	}
	if row.OccurredAt <= 0 {
		row.OccurredAt = helper.GetTimestamp()
	}
}

func CreateUserBalanceLotTransactionWithDB(db *gorm.DB, input UserBalanceLotTransactionInput) (UserBalanceLotTransaction, error) {
	if db == nil {
		return UserBalanceLotTransaction{}, fmt.Errorf("database handle is nil")
	}
	row := UserBalanceLotTransaction{
		Id:                 random.GetUUID(),
		UserID:             strings.TrimSpace(input.UserID),
		LotID:              strings.TrimSpace(input.LotID),
		SourceType:         normalizeUserBalanceLotSourceType(input.SourceType),
		SourceID:           strings.TrimSpace(input.SourceID),
		TxType:             normalizeUserBalanceLotTxType(input.TxType),
		DeltaAmount:        input.DeltaAmount,
		LotRemainingBefore: input.LotRemainingBefore,
		LotRemainingAfter:  input.LotRemainingAfter,
		OccurredAt:         input.OccurredAt,
	}
	normalizeUserBalanceLotTransactionRow(&row)
	if row.UserID == "" {
		return UserBalanceLotTransaction{}, fmt.Errorf("用户 ID 不能为空")
	}
	if row.LotID == "" {
		return UserBalanceLotTransaction{}, fmt.Errorf("余额批次 ID 不能为空")
	}
	if row.SourceType == "" {
		return UserBalanceLotTransaction{}, fmt.Errorf("来源类型不能为空")
	}
	if row.SourceID == "" {
		return UserBalanceLotTransaction{}, fmt.Errorf("来源 ID 不能为空")
	}
	if row.TxType == "" {
		return UserBalanceLotTransaction{}, fmt.Errorf("交易类型不能为空")
	}
	if row.DeltaAmount == 0 {
		return UserBalanceLotTransaction{}, fmt.Errorf("交易变动额度不能为 0")
	}
	now := helper.GetTimestamp()
	row.CreatedAt = now
	row.UpdatedAt = now
	if err := db.Create(&row).Error; err != nil {
		return UserBalanceLotTransaction{}, err
	}
	return row, nil
}

func ListUserBalanceLotTransactionsPageWithDB(db *gorm.DB, userID string, sourceType string, txType string, page int, pageSize int) ([]UserBalanceLotTransaction, int64, error) {
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
	query := db.Model(&UserBalanceLotTransaction{}).Where("user_id = ?", normalizedUserID)
	if normalizedSource := normalizeUserBalanceLotSourceFilter(sourceType); normalizedSource != "" {
		query = query.Where("source_type = ?", normalizedSource)
	}
	if normalizedTxType := normalizeUserBalanceLotTxTypeFilter(txType); normalizedTxType != "" {
		query = query.Where("tx_type = ?", normalizedTxType)
	}
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]UserBalanceLotTransaction, 0, pageSize)
	if err := query.
		Order("occurred_at desc, id desc").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	for i := range rows {
		normalizeUserBalanceLotTransactionRow(&rows[i])
	}
	return rows, total, nil
}
