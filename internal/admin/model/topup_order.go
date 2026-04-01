package model

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"gorm.io/gorm"
)

const (
	TopupOrdersTableName      = "topup_orders"
	TopupOrderStatusCreated   = "created"
	TopupOrderStatusPending   = "pending"
	TopupOrderStatusPaid      = "paid"
	TopupOrderStatusFulfilled = "fulfilled"
	TopupOrderStatusFailed    = "failed"
	TopupOrderStatusCanceled  = "canceled"
	TopupOrderSourceTopUp     = "top_up_link"
)

type TopupOrder struct {
	Id              string `json:"id" gorm:"type:char(36);primaryKey"`
	UserID          string `json:"user_id" gorm:"type:char(36);index"`
	Username        string `json:"username" gorm:"type:varchar(255);default:'';index"`
	Status          string `json:"status" gorm:"type:varchar(32);default:'created';index"`
	Source          string `json:"source" gorm:"type:varchar(64);default:'top_up_link';index"`
	ProviderName    string `json:"provider_name" gorm:"type:varchar(128);default:''"`
	ProviderOrderID string `json:"provider_order_id" gorm:"type:varchar(255);default:'';index"`
	RedemptionID    string `json:"redemption_id" gorm:"type:char(36);index"`
	TransactionID   string `json:"transaction_id" gorm:"type:varchar(64);uniqueIndex"`
	StatusMessage   string `json:"status_message" gorm:"type:text;default:''"`
	RedirectURL     string `json:"redirect_url" gorm:"type:text;default:''"`
	PaidAt          int64  `json:"paid_at" gorm:"bigint;index"`
	RedeemedAt      int64  `json:"redeemed_at" gorm:"bigint;index"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint;index"`
}

type TopupOrderCallbackInput struct {
	OrderID         string
	TransactionID   string
	ProviderOrderID string
	Status          string
	ProviderName    string
	RedemptionID    string
	RedemptionCode  string
	StatusMessage   string
	PaidAt          int64
	RedeemedAt      int64
}

func (TopupOrder) TableName() string {
	return TopupOrdersTableName
}

func normalizeTopupOrderRow(row *TopupOrder) {
	if row == nil {
		return
	}
	row.Id = strings.TrimSpace(row.Id)
	row.UserID = strings.TrimSpace(row.UserID)
	row.Username = strings.TrimSpace(row.Username)
	row.Status = strings.TrimSpace(strings.ToLower(row.Status))
	if row.Status == "" {
		row.Status = TopupOrderStatusCreated
	}
	row.Source = strings.TrimSpace(strings.ToLower(row.Source))
	if row.Source == "" {
		row.Source = TopupOrderSourceTopUp
	}
	row.ProviderName = strings.TrimSpace(row.ProviderName)
	row.ProviderOrderID = strings.TrimSpace(row.ProviderOrderID)
	row.RedemptionID = strings.TrimSpace(row.RedemptionID)
	row.TransactionID = strings.TrimSpace(row.TransactionID)
	row.StatusMessage = strings.TrimSpace(row.StatusMessage)
	row.RedirectURL = strings.TrimSpace(row.RedirectURL)
}

func NormalizeTopupOrderStatus(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case TopupOrderStatusCreated:
		return TopupOrderStatusCreated
	case TopupOrderStatusPending:
		return TopupOrderStatusPending
	case TopupOrderStatusPaid:
		return TopupOrderStatusPaid
	case TopupOrderStatusFulfilled:
		return TopupOrderStatusFulfilled
	case TopupOrderStatusFailed:
		return TopupOrderStatusFailed
	case TopupOrderStatusCanceled:
		return TopupOrderStatusCanceled
	default:
		return ""
	}
}

func buildTopupOrderRedirectURL(baseLink string, orderID string, userID string, username string, transactionID string) (string, error) {
	trimmedBaseLink := strings.TrimSpace(baseLink)
	if trimmedBaseLink == "" {
		return "", fmt.Errorf("超级管理员未设置充值链接")
	}
	parsed, err := url.Parse(trimmedBaseLink)
	if err != nil {
		return "", fmt.Errorf("充值链接配置无效")
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("充值链接配置无效")
	}
	query := parsed.Query()
	query.Set("user_id", strings.TrimSpace(userID))
	query.Set("transaction_id", strings.TrimSpace(transactionID))
	query.Set("order_id", strings.TrimSpace(orderID))
	if normalizedUsername := strings.TrimSpace(username); normalizedUsername != "" {
		query.Set("username", normalizedUsername)
	} else {
		query.Del("username")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func CreateTopupOrderWithDB(db *gorm.DB, userID string, username string) (TopupOrder, error) {
	if db == nil {
		return TopupOrder{}, fmt.Errorf("database handle is nil")
	}
	order := TopupOrder{
		Id:            random.GetUUID(),
		UserID:        strings.TrimSpace(userID),
		Username:      strings.TrimSpace(username),
		Status:        TopupOrderStatusCreated,
		Source:        TopupOrderSourceTopUp,
		TransactionID: random.GetUUID(),
	}
	if order.UserID == "" {
		return TopupOrder{}, fmt.Errorf("无效的 user id")
	}
	redirectURL, err := buildTopupOrderRedirectURL(
		config.TopUpLink,
		order.Id,
		order.UserID,
		order.Username,
		order.TransactionID,
	)
	if err != nil {
		return TopupOrder{}, err
	}
	order.RedirectURL = redirectURL
	now := helper.GetTimestamp()
	order.CreatedAt = now
	order.UpdatedAt = now
	normalizeTopupOrderRow(&order)
	if err := db.Create(&order).Error; err != nil {
		return TopupOrder{}, err
	}
	return order, nil
}

func GetTopupOrderByIDWithDB(db *gorm.DB, orderID string, userID string) (TopupOrder, error) {
	if db == nil {
		return TopupOrder{}, fmt.Errorf("database handle is nil")
	}
	normalizedOrderID := strings.TrimSpace(orderID)
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedOrderID == "" || normalizedUserID == "" {
		return TopupOrder{}, gorm.ErrRecordNotFound
	}
	row := TopupOrder{}
	if err := db.Where("id = ? AND user_id = ?", normalizedOrderID, normalizedUserID).First(&row).Error; err != nil {
		return TopupOrder{}, err
	}
	normalizeTopupOrderRow(&row)
	return row, nil
}

func ListTopupOrdersPageWithDB(db *gorm.DB, userID string, page int, pageSize int) ([]TopupOrder, int64, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return nil, 0, fmt.Errorf("无效的 user id")
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	query := db.Model(&TopupOrder{}).Where("user_id = ?", normalizedUserID)
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]TopupOrder, 0, pageSize)
	if err := query.Order("created_at desc, id desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	for i := range rows {
		normalizeTopupOrderRow(&rows[i])
	}
	return rows, total, nil
}

func ApplyTopupOrderCallbackWithDB(db *gorm.DB, input TopupOrderCallbackInput) (TopupOrder, error) {
	if db == nil {
		return TopupOrder{}, fmt.Errorf("database handle is nil")
	}
	normalizedStatus := NormalizeTopupOrderStatus(input.Status)
	if normalizedStatus == "" {
		return TopupOrder{}, fmt.Errorf("无效的订单状态")
	}
	normalizedOrderID := strings.TrimSpace(input.OrderID)
	normalizedTransactionID := strings.TrimSpace(input.TransactionID)
	normalizedProviderOrderID := strings.TrimSpace(input.ProviderOrderID)
	if normalizedOrderID == "" && normalizedTransactionID == "" && normalizedProviderOrderID == "" {
		return TopupOrder{}, fmt.Errorf("order_id、transaction_id、provider_order_id 不能同时为空")
	}
	normalizedProviderName := strings.TrimSpace(input.ProviderName)
	normalizedStatusMessage := strings.TrimSpace(input.StatusMessage)
	normalizedRedemptionID := strings.TrimSpace(input.RedemptionID)
	normalizedRedemptionCode := strings.TrimSpace(input.RedemptionCode)

	result := TopupOrder{}
	err := db.Transaction(func(tx *gorm.DB) error {
		order, err := selectTopupOrderForCallbackWithDB(tx, normalizedOrderID, normalizedTransactionID, normalizedProviderOrderID)
		if err != nil {
			return err
		}
		if normalizedProviderName != "" {
			order.ProviderName = normalizedProviderName
		}
		if normalizedProviderOrderID != "" {
			order.ProviderOrderID = normalizedProviderOrderID
		}
		if normalizedStatusMessage != "" {
			order.StatusMessage = normalizedStatusMessage
		}
		if normalizedRedemptionID != "" || normalizedRedemptionCode != "" {
			redemption, err := selectRedemptionForTopupOrderWithDB(tx, normalizedRedemptionID, normalizedRedemptionCode)
			if err != nil {
				return err
			}
			if strings.TrimSpace(redemption.TopupOrderID) != "" && strings.TrimSpace(redemption.TopupOrderID) != order.Id {
				return fmt.Errorf("兑换码已关联其他订单")
			}
			if strings.TrimSpace(order.RedemptionID) != "" && strings.TrimSpace(order.RedemptionID) != redemption.Id {
				return fmt.Errorf("订单已关联其他兑换码")
			}
			order.RedemptionID = strings.TrimSpace(redemption.Id)
			if err := tx.Model(&Redemption{}).
				Where("id = ?", redemption.Id).
				Update("topup_order_id", order.Id).Error; err != nil {
				return err
			}
		}
		now := helper.GetTimestamp()
		order.Status = normalizedStatus
		switch normalizedStatus {
		case TopupOrderStatusPaid:
			if input.PaidAt > 0 {
				order.PaidAt = input.PaidAt
			} else if order.PaidAt == 0 {
				order.PaidAt = now
			}
		case TopupOrderStatusFulfilled:
			if input.PaidAt > 0 {
				order.PaidAt = input.PaidAt
			} else if order.PaidAt == 0 {
				order.PaidAt = now
			}
			if input.RedeemedAt > 0 {
				order.RedeemedAt = input.RedeemedAt
			} else if order.RedeemedAt == 0 {
				order.RedeemedAt = now
			}
		}
		order.UpdatedAt = now
		normalizeTopupOrderRow(&order)
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		result = order
		return nil
	})
	if err != nil {
		return TopupOrder{}, err
	}
	return result, nil
}

func MarkTopupOrderRedeemedWithDB(tx *gorm.DB, orderID string, redemptionID string, redeemedAt int64) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedOrderID := strings.TrimSpace(orderID)
	if normalizedOrderID == "" {
		return nil
	}
	order := TopupOrder{}
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", normalizedOrderID).First(&order).Error; err != nil {
		return err
	}
	now := redeemedAt
	if now <= 0 {
		now = helper.GetTimestamp()
	}
	order.Status = TopupOrderStatusFulfilled
	order.RedemptionID = strings.TrimSpace(redemptionID)
	if order.PaidAt == 0 {
		order.PaidAt = now
	}
	order.RedeemedAt = now
	order.UpdatedAt = helper.GetTimestamp()
	normalizeTopupOrderRow(&order)
	return tx.Save(&order).Error
}

func selectTopupOrderForCallbackWithDB(tx *gorm.DB, orderID string, transactionID string, providerOrderID string) (TopupOrder, error) {
	if tx == nil {
		return TopupOrder{}, fmt.Errorf("database handle is nil")
	}
	row := TopupOrder{}
	query := tx.Set("gorm:query_option", "FOR UPDATE")
	switch {
	case strings.TrimSpace(orderID) != "":
		if err := query.Where("id = ?", strings.TrimSpace(orderID)).First(&row).Error; err != nil {
			return TopupOrder{}, err
		}
	case strings.TrimSpace(transactionID) != "":
		if err := query.Where("transaction_id = ?", strings.TrimSpace(transactionID)).First(&row).Error; err != nil {
			return TopupOrder{}, err
		}
	case strings.TrimSpace(providerOrderID) != "":
		if err := query.Where("provider_order_id = ?", strings.TrimSpace(providerOrderID)).First(&row).Error; err != nil {
			return TopupOrder{}, err
		}
	default:
		return TopupOrder{}, gorm.ErrRecordNotFound
	}
	normalizeTopupOrderRow(&row)
	return row, nil
}

func selectRedemptionForTopupOrderWithDB(tx *gorm.DB, redemptionID string, redemptionCode string) (Redemption, error) {
	if tx == nil {
		return Redemption{}, fmt.Errorf("database handle is nil")
	}
	row := Redemption{}
	query := tx.Set("gorm:query_option", "FOR UPDATE")
	switch {
	case strings.TrimSpace(redemptionID) != "":
		if err := query.Where("id = ?", strings.TrimSpace(redemptionID)).First(&row).Error; err != nil {
			return Redemption{}, err
		}
	case strings.TrimSpace(redemptionCode) != "":
		if err := query.Where(`"code" = ?`, strings.TrimSpace(redemptionCode)).First(&row).Error; err != nil {
			return Redemption{}, err
		}
	default:
		return Redemption{}, gorm.ErrRecordNotFound
	}
	return row, nil
}
