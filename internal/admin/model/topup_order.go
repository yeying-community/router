package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
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
	TopupOrderSourceTopUpAPI  = "top_up_api"
	TopupOrderBusinessBalance = "balance_topup"
	TopupOrderBusinessPackage = "package_purchase"
	TopupOrderCurrencyCNY     = "CNY"
)

type TopupOrder struct {
	Id              string  `json:"id" gorm:"type:char(36);primaryKey"`
	UserID          string  `json:"user_id" gorm:"type:char(36);index"`
	Username        string  `json:"username" gorm:"type:varchar(255);default:'';index"`
	Status          string  `json:"status" gorm:"type:varchar(32);default:'created';index"`
	Source          string  `json:"source" gorm:"type:varchar(64);default:'top_up_link';index"`
	ProviderName    string  `json:"provider_name" gorm:"type:varchar(128);default:''"`
	ProviderOrderID string  `json:"provider_order_id" gorm:"type:varchar(255);default:'';index"`
	RedemptionID    string  `json:"redemption_id" gorm:"type:char(36);index"`
	TransactionID   string  `json:"transaction_id" gorm:"type:varchar(64);uniqueIndex"`
	BusinessType    string  `json:"business_type" gorm:"type:varchar(32);default:'balance_topup';index"`
	Title           string  `json:"title" gorm:"type:varchar(255);default:''"`
	Amount          float64 `json:"amount" gorm:"type:decimal(10,2);default:0"`
	Currency        string  `json:"currency" gorm:"type:varchar(16);default:'CNY'"`
	Quota           int64   `json:"quota" gorm:"type:bigint;default:0"`
	PackageID       string  `json:"package_id" gorm:"type:char(36);default:'';index"`
	PackageName     string  `json:"package_name" gorm:"type:varchar(255);default:''"`
	CallbackURL     string  `json:"callback_url" gorm:"type:text;default:''"`
	ReturnURL       string  `json:"return_url" gorm:"type:text;default:''"`
	StatusMessage   string  `json:"status_message" gorm:"type:text;default:''"`
	RedirectURL     string  `json:"redirect_url" gorm:"type:text;default:''"`
	PaidAt          int64   `json:"paid_at" gorm:"bigint;index"`
	RedeemedAt      int64   `json:"redeemed_at" gorm:"bigint;index"`
	CreatedAt       int64   `json:"created_at" gorm:"bigint;index"`
	UpdatedAt       int64   `json:"updated_at" gorm:"bigint;index"`
}

type CreateTopupOrderInput struct {
	BusinessType string
	ClientType   string
	Title        string
	Amount       float64
	Currency     string
	Quota        int64
	PackageID    string
	ReturnURL    string
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
	row.BusinessType = resolveTopupOrderBusinessType(row.BusinessType, row.PackageID)
	row.Title = strings.TrimSpace(row.Title)
	row.Amount = normalizeTopupOrderAmount(row.Amount)
	row.Currency = normalizeTopupOrderCurrency(row.Currency)
	row.Quota = normalizeTopupOrderQuota(row.Quota)
	row.PackageID = strings.TrimSpace(row.PackageID)
	row.PackageName = strings.TrimSpace(row.PackageName)
	row.CallbackURL = strings.TrimSpace(row.CallbackURL)
	row.ReturnURL = strings.TrimSpace(row.ReturnURL)
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

func normalizeTopupOrderBusinessType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case TopupOrderBusinessBalance:
		return TopupOrderBusinessBalance
	case TopupOrderBusinessPackage:
		return TopupOrderBusinessPackage
	case "":
		return ""
	default:
		return ""
	}
}

func resolveTopupOrderBusinessType(value string, packageID string) string {
	if normalized := normalizeTopupOrderBusinessType(value); normalized != "" {
		return normalized
	}
	if strings.TrimSpace(packageID) != "" {
		return TopupOrderBusinessPackage
	}
	return TopupOrderBusinessBalance
}

func normalizeTopupOrderCurrency(value string) string {
	if strings.TrimSpace(strings.ToUpper(value)) == "" {
		return TopupOrderCurrencyCNY
	}
	return strings.TrimSpace(strings.ToUpper(value))
}

func normalizeTopupOrderAmount(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return math.Round(value*100) / 100
}

func normalizeTopupOrderQuota(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func topupOrderCallbackURL() string {
	baseURL := strings.TrimSpace(config.ServerAddress)
	if baseURL == "" {
		return ""
	}
	return strings.TrimRight(baseURL, "/") + "/api/v1/public/topup/callback"
}

func topupOrderSigningParts(payload map[string]string) []string {
	keys := make([]string, 0, len(payload))
	for key, value := range payload {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" || strings.EqualFold(strings.TrimSpace(key), "sign") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, strings.TrimSpace(payload[key])))
	}
	return parts
}

func topupOrderSigningBaseString(payload map[string]string) string {
	return strings.Join(topupOrderSigningParts(payload), "&")
}

func topupOrderSigningString(payload map[string]string, secret string) string {
	parts := append(topupOrderSigningParts(payload), "secret="+strings.TrimSpace(secret))
	return strings.Join(parts, "&")
}

func signTopupOrderPayload(payload map[string]string, secret string) string {
	sum := sha256.Sum256([]byte(topupOrderSigningString(payload, secret)))
	return hex.EncodeToString(sum[:])
}

func buildTopupOrderRedirectURL(baseLink string, order TopupOrder) (string, error) {
	trimmedBaseLink := strings.TrimSpace(baseLink)
	if trimmedBaseLink == "" {
		return "", fmt.Errorf("超级管理员未设置充值链接")
	}
	signSecret := strings.TrimSpace(config.TopUpSignSecret)
	if signSecret == "" {
		return "", fmt.Errorf("超级管理员未设置支付签名密钥")
	}
	parsed, err := url.Parse(trimmedBaseLink)
	if err != nil {
		return "", fmt.Errorf("充值链接配置无效")
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("充值链接配置无效")
	}
	payload := map[string]string{
		"merchant_app":   "router",
		"order_id":       strings.TrimSpace(order.Id),
		"transaction_id": strings.TrimSpace(order.TransactionID),
		"user_id":        strings.TrimSpace(order.UserID),
		"username":       strings.TrimSpace(order.Username),
		"business_type":  strings.TrimSpace(order.BusinessType),
		"title":          strings.TrimSpace(order.Title),
		"amount":         fmt.Sprintf("%.2f", order.Amount),
		"currency":       strings.TrimSpace(order.Currency),
		"callback_url":   strings.TrimSpace(order.CallbackURL),
		"return_url":     strings.TrimSpace(order.ReturnURL),
		"timestamp":      strconv.FormatInt(helper.GetTimestamp(), 10),
		"nonce":          random.GetUUID(),
	}
	if order.Quota > 0 {
		payload["quota"] = strconv.FormatInt(order.Quota, 10)
	}
	if strings.TrimSpace(order.PackageID) != "" {
		payload["package_id"] = strings.TrimSpace(order.PackageID)
	}
	if strings.TrimSpace(order.PackageName) != "" {
		payload["package_name"] = strings.TrimSpace(order.PackageName)
	}
	payload["sign"] = signTopupOrderPayload(payload, signSecret)
	query := parsed.Query()
	for key, value := range payload {
		if strings.TrimSpace(value) == "" {
			continue
		}
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func buildLegacyTopupOrderRedirectURL(baseLink string, order TopupOrder) (string, error) {
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
	query.Set("user_id", strings.TrimSpace(order.UserID))
	query.Set("transaction_id", strings.TrimSpace(order.TransactionID))
	query.Set("order_id", strings.TrimSpace(order.Id))
	if strings.TrimSpace(order.Username) != "" {
		query.Set("username", strings.TrimSpace(order.Username))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func CreateTopupOrderWithDB(db *gorm.DB, userID string, username string, input CreateTopupOrderInput) (TopupOrder, error) {
	if db == nil {
		return TopupOrder{}, fmt.Errorf("database handle is nil")
	}
	isLegacyMode := strings.TrimSpace(input.BusinessType) == "" &&
		input.Amount <= 0 &&
		strings.TrimSpace(input.PackageID) == "" &&
		input.Quota <= 0
	businessType := normalizeTopupOrderBusinessType(input.BusinessType)
	if businessType == "" && !isLegacyMode {
		return TopupOrder{}, fmt.Errorf("无效的业务类型")
	}
	amount := normalizeTopupOrderAmount(input.Amount)
	if amount <= 0 && !isLegacyMode && businessType == TopupOrderBusinessBalance {
		return TopupOrder{}, fmt.Errorf("支付金额必须大于 0")
	}
	currency := normalizeTopupOrderCurrency(input.Currency)
	order := TopupOrder{
		Id:            random.GetUUID(),
		UserID:        strings.TrimSpace(userID),
		Username:      strings.TrimSpace(username),
		Status:        TopupOrderStatusCreated,
		Source:        TopupOrderSourceTopUp,
		TransactionID: random.GetUUID(),
		BusinessType:  businessType,
		Amount:        amount,
		Currency:      currency,
		Quota:         normalizeTopupOrderQuota(input.Quota),
		PackageID:     strings.TrimSpace(input.PackageID),
		CallbackURL:   topupOrderCallbackURL(),
		ReturnURL:     strings.TrimSpace(input.ReturnURL),
	}
	if order.UserID == "" {
		return TopupOrder{}, fmt.Errorf("无效的 user id")
	}
	var redirectURL string
	var err error
	if isLegacyMode {
		order.BusinessType = ""
		redirectURL, err = buildLegacyTopupOrderRedirectURL(config.TopUpLink, order)
		if err != nil {
			return TopupOrder{}, err
		}
	} else {
		switch order.BusinessType {
		case TopupOrderBusinessBalance:
			order.Currency = BillingCurrencyCodeCNY
			if order.Amount <= 0 {
				return TopupOrder{}, fmt.Errorf("充值金额必须大于 0")
			}
			if order.Quota <= 0 {
				yycPerUnit, err := GetBillingCurrencyYYCPerUnit(order.Currency)
				if err != nil {
					return TopupOrder{}, err
				}
				order.Quota = normalizeTopupOrderQuota(int64(math.Round(order.Amount * yycPerUnit)))
			}
			if order.Quota <= 0 {
				return TopupOrder{}, fmt.Errorf("充值额度不能为空")
			}
			if strings.TrimSpace(input.Title) != "" {
				order.Title = strings.TrimSpace(input.Title)
			} else {
				order.Title = "账户充值"
			}
		case TopupOrderBusinessPackage:
			if strings.TrimSpace(order.PackageID) == "" {
				return TopupOrder{}, fmt.Errorf("套餐 ID 不能为空")
			}
			servicePackage, err := getServicePackageByIDWithDB(db, order.PackageID)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					return TopupOrder{}, fmt.Errorf("套餐不存在")
				}
				return TopupOrder{}, err
			}
			if !servicePackage.Enabled {
				return TopupOrder{}, fmt.Errorf("套餐已禁用")
			}
			order.Amount = normalizeTopupOrderAmount(servicePackage.SalePrice)
			order.Currency = normalizeTopupOrderCurrency(servicePackage.SaleCurrency)
			if order.Amount <= 0 {
				return TopupOrder{}, fmt.Errorf("套餐售价未配置")
			}
			order.PackageName = strings.TrimSpace(servicePackage.Name)
			if strings.TrimSpace(input.Title) != "" {
				order.Title = strings.TrimSpace(input.Title)
			} else if order.PackageName != "" {
				order.Title = "购买套餐：" + order.PackageName
			} else {
				order.Title = "购买套餐"
			}
		default:
			return TopupOrder{}, fmt.Errorf("无效的业务类型")
		}
		if order.CallbackURL == "" {
			return TopupOrder{}, fmt.Errorf("回调地址未配置")
		}
		if config.EffectiveTopUpMode() == config.TopUpModeAPI {
			order.Source = TopupOrderSourceTopUpAPI
		} else {
			redirectURL, err = buildTopupOrderRedirectURL(config.TopUpLink, order)
			if err != nil {
				return TopupOrder{}, err
			}
		}
	}
	now := helper.GetTimestamp()
	order.CreatedAt = now
	order.UpdatedAt = now
	normalizeTopupOrderRow(&order)

	if order.Source == TopupOrderSourceTopUpAPI {
		if err := db.Create(&order).Error; err != nil {
			return TopupOrder{}, err
		}
		createResult, err := createTopupOrderByExternalPayAPI(order, input.ClientType)
		if err != nil {
			updateTime := helper.GetTimestamp()
			updateErr := db.Model(&TopupOrder{}).
				Where("id = ?", order.Id).
				Updates(map[string]any{
					"status":         TopupOrderStatusFailed,
					"status_message": err.Error(),
					"updated_at":     updateTime,
				}).Error
			if updateErr != nil {
				return TopupOrder{}, fmt.Errorf("%s; update local top-up order failed: %w", err.Error(), updateErr)
			}
			return TopupOrder{}, err
		}
		order.ProviderName = createResult.ProviderName
		order.ProviderOrderID = createResult.ProviderOrderID
		order.RedirectURL = createResult.RedirectURL
		order.UpdatedAt = helper.GetTimestamp()
		normalizeTopupOrderRow(&order)
		if err := db.Model(&TopupOrder{}).
			Where("id = ?", order.Id).
			Updates(map[string]any{
				"provider_name":     order.ProviderName,
				"provider_order_id": order.ProviderOrderID,
				"redirect_url":      order.RedirectURL,
				"updated_at":        order.UpdatedAt,
			}).Error; err != nil {
			return TopupOrder{}, err
		}
		return order, nil
	}

	order.RedirectURL = redirectURL
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

func ListTopupOrdersPageWithDB(db *gorm.DB, userID string, businessType string, page int, pageSize int) ([]TopupOrder, int64, error) {
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
	if normalizedBusinessType := normalizeTopupOrderBusinessType(businessType); normalizedBusinessType != "" {
		query = applyTopupOrderBusinessTypeFilter(query, db, normalizedBusinessType)
	}
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

func applyTopupOrderBusinessTypeFilter(query *gorm.DB, db *gorm.DB, businessType string) *gorm.DB {
	if query == nil || db == nil {
		return query
	}
	normalizedBusinessType := normalizeTopupOrderBusinessType(businessType)
	if normalizedBusinessType == "" {
		return query
	}
	if db.Migrator().HasColumn(&TopupOrder{}, "business_type") {
		return query.Where("business_type = ?", normalizedBusinessType)
	}
	if db.Migrator().HasColumn(&TopupOrder{}, "package_id") {
		if normalizedBusinessType == TopupOrderBusinessPackage {
			return query.Where("COALESCE(TRIM(package_id), '') <> ''")
		}
		return query.Where("COALESCE(TRIM(package_id), '') = ''")
	}
	if normalizedBusinessType == TopupOrderBusinessPackage {
		return query.Where("1 = 0")
	}
	return query
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

func FulfillTopupOrderWithDB(db *gorm.DB, orderID string) (TopupOrder, bool, error) {
	if db == nil {
		return TopupOrder{}, false, fmt.Errorf("database handle is nil")
	}
	normalizedOrderID := strings.TrimSpace(orderID)
	if normalizedOrderID == "" {
		return TopupOrder{}, false, fmt.Errorf("订单 ID 不能为空")
	}
	result := TopupOrder{}
	fulfilledNow := false
	err := db.Transaction(func(tx *gorm.DB) error {
		order := TopupOrder{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", normalizedOrderID).First(&order).Error; err != nil {
			return err
		}
		normalizeTopupOrderRow(&order)
		if order.Status == TopupOrderStatusFulfilled {
			result = order
			return nil
		}
		if order.Status != TopupOrderStatusPaid {
			return fmt.Errorf("订单未支付")
		}
		switch order.BusinessType {
		case TopupOrderBusinessBalance:
			if order.Quota <= 0 {
				return fmt.Errorf("充值额度不能为空")
			}
			if err := tx.Model(&User{}).
				Where("id = ?", order.UserID).
				Update("quota", gorm.Expr("quota + ?", order.Quota)).Error; err != nil {
				return err
			}
		case TopupOrderBusinessPackage:
			if strings.TrimSpace(order.PackageID) == "" {
				return fmt.Errorf("套餐 ID 不能为空")
			}
			if _, err := AssignServicePackageToUserWithDB(tx, order.PackageID, order.UserID, helper.GetTimestamp()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("无效的业务类型")
		}
		now := helper.GetTimestamp()
		order.Status = TopupOrderStatusFulfilled
		if order.PaidAt == 0 {
			order.PaidAt = now
		}
		if order.RedeemedAt == 0 {
			order.RedeemedAt = now
		}
		order.UpdatedAt = now
		normalizeTopupOrderRow(&order)
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		result = order
		fulfilledNow = true
		return nil
	})
	if err != nil {
		return TopupOrder{}, false, err
	}
	return result, fulfilledNow, nil
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
