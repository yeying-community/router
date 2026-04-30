package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/common/random"
	"gorm.io/gorm"
)

const (
	TopupOrdersTableName       = "topup_orders"
	TopupOrderStatusCreated    = "created"
	TopupOrderStatusPending    = "pending"
	TopupOrderStatusPaid       = "paid"
	TopupOrderStatusFulfilled  = "fulfilled"
	TopupOrderStatusFailed     = "failed"
	TopupOrderStatusCanceled   = "canceled"
	TopupOrderSourceTopUp      = "top_up_link"
	TopupOrderSourceTopUpAPI   = "top_up_api"
	TopupOrderBusinessBalance  = "balance_topup"
	TopupOrderBusinessPackage  = "package_purchase"
	TopupOrderCurrencyCNY      = "CNY"
	TopupOrderOperationTopup   = "topup"
	TopupOrderOperationNew     = "purchase"
	TopupOrderOperationRenew   = "renew"
	TopupOrderOperationUpgrade = "upgrade"
)

type TopupOrder struct {
	Id              string  `json:"id" gorm:"type:char(36);primaryKey"`
	UserID          string  `json:"user_id" gorm:"type:char(36);index"`
	Username        string  `json:"username" gorm:"type:varchar(255);default:'';index"`
	Status          string  `json:"status" gorm:"type:varchar(32);default:'created';index"`
	Source          string  `json:"source" gorm:"type:varchar(64);default:'top_up_link';index"`
	ProviderName    string  `json:"provider_name" gorm:"type:varchar(128);default:''"`
	ProviderOrderID string  `json:"provider_order_id" gorm:"type:varchar(255);default:'';index"`
	TransactionID   string  `json:"transaction_id" gorm:"type:varchar(64);uniqueIndex"`
	BusinessType    string  `json:"business_type" gorm:"type:varchar(32);default:'balance_topup';index"`
	OperationType   string  `json:"operation_type" gorm:"type:varchar(32);default:'';index"`
	Title           string  `json:"title" gorm:"type:varchar(255);default:''"`
	Amount          float64 `json:"amount" gorm:"type:decimal(10,2);default:0"`
	Currency        string  `json:"currency" gorm:"type:varchar(16);default:'CNY'"`
	Quota           int64   `json:"quota" gorm:"type:bigint;default:0"`
	TopupPlanID     string  `json:"topup_plan_id" gorm:"type:char(36);default:'';index"`
	ValidityDays    int     `json:"validity_days" gorm:"type:int;not null;default:0"`
	CreditExpiresAt int64   `json:"credit_expires_at" gorm:"bigint;not null;default:0;index"`
	PackageID       string  `json:"package_id" gorm:"type:char(36);default:'';index"`
	PackageName     string  `json:"package_name" gorm:"type:varchar(255);default:''"`
	ClientType      string  `json:"client_type" gorm:"-"`
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
	BusinessType  string
	OperationType string
	ClientType    string
	Title         string
	Amount        float64
	Currency      string
	Quota         int64
	PlanID        string
	PackageID     string
	ReturnURL     string
}

type TopupOrderCallbackInput struct {
	OrderID         string
	TransactionID   string
	ProviderOrderID string
	Status          string
	ProviderName    string
	StatusMessage   string
	PaidAt          int64
	RedeemedAt      int64
}

type PackagePurchasePreview struct {
	OperationType      string  `json:"operation_type"`
	StartAt            int64   `json:"start_at"`
	ExpiresAt          int64   `json:"expires_at"`
	CurrentExpiresAt   int64   `json:"current_expires_at"`
	TargetPackageID    string  `json:"target_package_id"`
	TargetPackageName  string  `json:"target_package_name"`
	CurrentPackageID   string  `json:"current_package_id"`
	CurrentPackageName string  `json:"current_package_name"`
	PayableAmount      float64 `json:"payable_amount"`
	PayableCurrency    string  `json:"payable_currency"`
	PayableYYC         int64   `json:"payable_yyc"`
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
	row.TransactionID = strings.TrimSpace(row.TransactionID)
	row.BusinessType = resolveTopupOrderBusinessType(row.BusinessType, row.PackageID)
	row.OperationType = resolveTopupOrderOperationType(row.BusinessType, row.OperationType)
	row.Title = strings.TrimSpace(row.Title)
	row.Amount = normalizeTopupOrderAmount(row.Amount)
	row.Currency = normalizeTopupOrderCurrency(row.Currency)
	row.Quota = normalizeTopupOrderQuota(row.Quota)
	row.TopupPlanID = strings.TrimSpace(row.TopupPlanID)
	row.ValidityDays = normalizeTopupPlanValidityDays(row.ValidityDays)
	if row.CreditExpiresAt < 0 {
		row.CreditExpiresAt = 0
	}
	row.PackageID = strings.TrimSpace(row.PackageID)
	row.PackageName = strings.TrimSpace(row.PackageName)
	row.ClientType = strings.TrimSpace(strings.ToLower(row.ClientType))
	row.CallbackURL = strings.TrimSpace(row.CallbackURL)
	row.ReturnURL = sanitizeTopupReturnURL(row.ReturnURL)
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

func normalizeTopupOrderOperationType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case TopupOrderOperationNew:
		return TopupOrderOperationNew
	case "new_purchase":
		return TopupOrderOperationNew
	case TopupOrderOperationRenew:
		return TopupOrderOperationRenew
	case TopupOrderOperationUpgrade:
		return TopupOrderOperationUpgrade
	default:
		return ""
	}
}

func resolveTopupOrderOperationType(businessType string, value string) string {
	switch strings.TrimSpace(businessType) {
	case TopupOrderBusinessBalance:
		// Balance top-up always carries a fixed operation type so it can be
		// explicitly included in upstream signatures.
		return TopupOrderOperationTopup
	case TopupOrderBusinessPackage:
		if normalized := normalizeTopupOrderOperationType(value); normalized != "" {
			return normalized
		}
		return TopupOrderOperationNew
	default:
		return ""
	}
}

func sanitizeTopupReturnURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return trimmed
	}
	query := parsed.Query()
	for _, key := range []string{
		"pay_status",
		"trade_no",
		"provider_order_id",
		"merchant_app",
		"order_id",
		"transaction_id",
		"user_id",
		"username",
		"business_type",
		"operation_type",
		"title",
		"amount",
		"currency",
		"quota",
		"package_id",
		"package_name",
		"client_type",
		"callback_url",
		"timestamp",
		"nonce",
		"sign",
	} {
		query.Del(key)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func isDirtyTopupReturnURL(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	return sanitizeTopupReturnURL(trimmed) != trimmed
}

func previewTopupStatusReason(raw string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if len(trimmed) > 160 {
		return trimmed[:160] + "..."
	}
	return trimmed
}

func logTopupOrderLifecycle(event string, order TopupOrder, fromStatus string, reason string) {
	logger.SysLogf(
		"[topup.order] event=%s order_id=%q user_id=%q username=%q business_type=%q operation_type=%q source=%q from_status=%q to_status=%q amount=%.2f currency=%q quota=%d topup_plan_id=%q package_id=%q package_name=%q provider_name=%q provider_order_id=%q transaction_id=%q validity_days=%d credit_expires_at=%d reason=%q",
		strings.TrimSpace(event),
		strings.TrimSpace(order.Id),
		strings.TrimSpace(order.UserID),
		strings.TrimSpace(order.Username),
		strings.TrimSpace(order.BusinessType),
		strings.TrimSpace(order.OperationType),
		strings.TrimSpace(order.Source),
		strings.TrimSpace(fromStatus),
		strings.TrimSpace(order.Status),
		order.Amount,
		strings.TrimSpace(order.Currency),
		order.Quota,
		strings.TrimSpace(order.TopupPlanID),
		strings.TrimSpace(order.PackageID),
		strings.TrimSpace(order.PackageName),
		strings.TrimSpace(order.ProviderName),
		strings.TrimSpace(order.ProviderOrderID),
		strings.TrimSpace(order.TransactionID),
		order.ValidityDays,
		order.CreditExpiresAt,
		previewTopupStatusReason(reason),
	)
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

func topupOrderAmountCurrencyLabel(code string) string {
	normalized := normalizeBillingCurrencyCode(code)
	if normalized == BillingCurrencyCodeCNY {
		return "元"
	}
	return normalized
}

func buildTopupOrderPlanTitle(plan ResolvedTopupPlan) string {
	amountPart := strings.TrimSpace(
		formatTopupPlanNumber(plan.Amount) + " " + topupOrderAmountCurrencyLabel(plan.AmountCurrency),
	)
	quotaPart := strings.TrimSpace(
		formatTopupPlanNumber(plan.QuotaAmount) + " " + normalizeBillingCurrencyCode(plan.QuotaCurrency),
	)
	parts := make([]string, 0, 2)
	if amountPart != "" {
		parts = append(parts, amountPart)
	}
	if quotaPart != "" {
		parts = append(parts, quotaPart)
	}
	return strings.Join(parts, " / ")
}

func resolvePackagePurchaseOperationType(requestedOperationType string, activeSubscription *UserPackageSubscription, targetPackageID string) string {
	if normalized := normalizeTopupOrderOperationType(requestedOperationType); normalized != "" {
		return normalized
	}
	if activeSubscription == nil {
		return TopupOrderOperationNew
	}
	if strings.TrimSpace(activeSubscription.PackageID) == strings.TrimSpace(targetPackageID) {
		return TopupOrderOperationRenew
	}
	return TopupOrderOperationUpgrade
}

func calcPackagePriceYYC(amount float64, currency string) (int64, error) {
	if amount <= 0 {
		return 0, nil
	}
	yycPerUnit, err := GetBillingCurrencyYYCPerUnit(currency)
	if err != nil {
		return 0, err
	}
	return normalizeTopupOrderQuota(int64(math.Round(amount * yycPerUnit))), nil
}

func calcPayableAmountByYYC(payableYYC int64, currency string) (float64, error) {
	if payableYYC <= 0 {
		return 0, nil
	}
	yycPerUnit, err := GetBillingCurrencyYYCPerUnit(currency)
	if err != nil {
		return 0, err
	}
	if yycPerUnit <= 0 {
		return 0, fmt.Errorf("币种兑换率无效：%s", strings.TrimSpace(strings.ToUpper(currency)))
	}
	// Round up to cents to avoid rounding down an otherwise payable amount.
	amount := math.Ceil((float64(payableYYC)/yycPerUnit)*100) / 100
	if amount <= 0 {
		return 0.01, nil
	}
	return amount, nil
}

func calcUpgradePayableYYCWithDB(db *gorm.DB, activeSubscription UserPackageSubscription, targetPackage ServicePackage, now int64) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database handle is nil")
	}
	if activeSubscription.ExpiresAt <= 0 {
		targetYYC, err := calcPackagePriceYYC(targetPackage.SalePrice, targetPackage.SaleCurrency)
		if err != nil {
			return 0, err
		}
		return targetYYC, nil
	}
	currentPackage, err := getServicePackageByIDWithDB(db, activeSubscription.PackageID)
	if err != nil {
		// If historical template is missing, fallback to full target price.
		targetYYC, convertErr := calcPackagePriceYYC(targetPackage.SalePrice, targetPackage.SaleCurrency)
		if convertErr != nil {
			return 0, convertErr
		}
		return targetYYC, nil
	}
	currentYYC, err := calcPackagePriceYYC(currentPackage.SalePrice, currentPackage.SaleCurrency)
	if err != nil {
		return 0, err
	}
	targetYYC, err := calcPackagePriceYYC(targetPackage.SalePrice, targetPackage.SaleCurrency)
	if err != nil {
		return 0, err
	}
	diffYYC := targetYYC - currentYYC
	if diffYYC <= 0 {
		return 0, nil
	}

	periodSeconds := activeSubscription.ExpiresAt - activeSubscription.StartedAt
	if periodSeconds <= 0 {
		durationDays := normalizeServicePackageDurationDays(currentPackage.DurationDays)
		periodSeconds = int64(durationDays) * 86400
	}
	if periodSeconds <= 0 {
		return diffYYC, nil
	}
	remainingSeconds := activeSubscription.ExpiresAt - now
	if remainingSeconds <= 0 {
		return 0, nil
	}
	if remainingSeconds > periodSeconds {
		remainingSeconds = periodSeconds
	}
	return int64(math.Ceil(float64(diffYYC) * (float64(remainingSeconds) / float64(periodSeconds)))), nil
}

func PreviewPackagePurchaseWithDB(db *gorm.DB, userID string, packageID string, requestedOperationType string, now int64) (PackagePurchasePreview, error) {
	if db == nil {
		return PackagePurchasePreview{}, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return PackagePurchasePreview{}, fmt.Errorf("用户 ID 不能为空")
	}
	normalizedPackageID := strings.TrimSpace(packageID)
	if normalizedPackageID == "" {
		return PackagePurchasePreview{}, fmt.Errorf("套餐 ID 不能为空")
	}
	effectiveNow := now
	if effectiveNow <= 0 {
		effectiveNow = helper.GetTimestamp()
	}
	targetPackage, err := getServicePackageByIDWithDB(db, normalizedPackageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PackagePurchasePreview{}, fmt.Errorf("套餐不存在")
		}
		return PackagePurchasePreview{}, err
	}
	if !targetPackage.Enabled {
		return PackagePurchasePreview{}, fmt.Errorf("套餐已禁用")
	}

	if err := syncUserPackageSubscriptionsWithDB(db, normalizedUserID, effectiveNow); err != nil {
		return PackagePurchasePreview{}, err
	}
	var active *UserPackageSubscription
	activeSubscription, activeErr := getActiveUserPackageSubscriptionWithDB(db, normalizedUserID)
	if activeErr == nil {
		active = &activeSubscription
	} else if !errors.Is(activeErr, gorm.ErrRecordNotFound) {
		return PackagePurchasePreview{}, activeErr
	}

	operationType := resolvePackagePurchaseOperationType(requestedOperationType, active, normalizedPackageID)
	preview := PackagePurchasePreview{
		OperationType:     operationType,
		TargetPackageID:   strings.TrimSpace(targetPackage.Id),
		TargetPackageName: strings.TrimSpace(targetPackage.Name),
		PayableCurrency:   normalizeTopupOrderCurrency(targetPackage.SaleCurrency),
	}
	if active != nil {
		preview.CurrentPackageID = strings.TrimSpace(active.PackageID)
		preview.CurrentPackageName = strings.TrimSpace(active.PackageName)
		preview.CurrentExpiresAt = active.ExpiresAt
	}

	switch operationType {
	case TopupOrderOperationRenew:
		if active == nil {
			return PackagePurchasePreview{}, fmt.Errorf("当前无生效套餐，无法续费")
		}
		if strings.TrimSpace(active.PackageID) != normalizedPackageID {
			return PackagePurchasePreview{}, fmt.Errorf("当前生效套餐与续费套餐不一致")
		}
		tailEnd, hasUnlimitedTail, err := latestUserPackageSubscriptionTailWithDB(db, normalizedUserID)
		if err != nil {
			return PackagePurchasePreview{}, err
		}
		if hasUnlimitedTail {
			return PackagePurchasePreview{}, fmt.Errorf("当前套餐无到期时间，无法续费")
		}
		startAt := effectiveNow
		if tailEnd > effectiveNow {
			startAt = tailEnd
		}
		durationDays := normalizeServicePackageDurationDays(targetPackage.DurationDays)
		expiresAt := int64(0)
		if durationDays > 0 {
			expiresAt = startAt + int64(durationDays)*86400
		}
		preview.StartAt = startAt
		preview.ExpiresAt = expiresAt
		preview.PayableAmount = normalizeTopupOrderAmount(targetPackage.SalePrice)
		payableYYC, err := calcPackagePriceYYC(preview.PayableAmount, preview.PayableCurrency)
		if err != nil {
			return PackagePurchasePreview{}, err
		}
		preview.PayableYYC = payableYYC
	case TopupOrderOperationUpgrade:
		if active == nil {
			operationType = TopupOrderOperationNew
			preview.OperationType = operationType
		} else if strings.TrimSpace(active.PackageID) == normalizedPackageID {
			return PackagePurchasePreview{}, fmt.Errorf("目标套餐与当前套餐一致，请使用续费")
		}
		if preview.OperationType == TopupOrderOperationUpgrade {
			payableYYC, err := calcUpgradePayableYYCWithDB(db, *active, targetPackage, effectiveNow)
			if err != nil {
				return PackagePurchasePreview{}, err
			}
			if payableYYC <= 0 {
				return PackagePurchasePreview{}, fmt.Errorf("目标套餐无需补差价，请选择续费或待当前周期结束后更换")
			}
			payableAmount, err := calcPayableAmountByYYC(payableYYC, preview.PayableCurrency)
			if err != nil {
				return PackagePurchasePreview{}, err
			}
			preview.PayableYYC = payableYYC
			preview.PayableAmount = normalizeTopupOrderAmount(payableAmount)
			preview.StartAt = effectiveNow
			preview.ExpiresAt = active.ExpiresAt
			break
		}
		fallthrough
	case TopupOrderOperationNew:
		durationDays := normalizeServicePackageDurationDays(targetPackage.DurationDays)
		expiresAt := int64(0)
		if durationDays > 0 {
			expiresAt = effectiveNow + int64(durationDays)*86400
		}
		preview.StartAt = effectiveNow
		preview.ExpiresAt = expiresAt
		preview.PayableAmount = normalizeTopupOrderAmount(targetPackage.SalePrice)
		payableYYC, err := calcPackagePriceYYC(preview.PayableAmount, preview.PayableCurrency)
		if err != nil {
			return PackagePurchasePreview{}, err
		}
		preview.PayableYYC = payableYYC
	default:
		return PackagePurchasePreview{}, fmt.Errorf("无效的套餐操作类型")
	}

	if preview.PayableAmount <= 0 {
		return PackagePurchasePreview{}, fmt.Errorf("套餐应付金额必须大于 0")
	}
	if preview.PayableYYC <= 0 {
		payableYYC, err := calcPackagePriceYYC(preview.PayableAmount, preview.PayableCurrency)
		if err != nil {
			return PackagePurchasePreview{}, err
		}
		preview.PayableYYC = payableYYC
	}
	return preview, nil
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
	normalizedBusinessType := resolveTopupOrderBusinessType(order.BusinessType, order.PackageID)
	normalizedOperationType := resolveTopupOrderOperationType(normalizedBusinessType, order.OperationType)
	payload := map[string]string{
		"merchant_app":   config.TopUpMerchantAppValue(),
		"order_id":       strings.TrimSpace(order.Id),
		"transaction_id": strings.TrimSpace(order.TransactionID),
		"user_id":        strings.TrimSpace(order.UserID),
		"username":       strings.TrimSpace(order.Username),
		"business_type":  normalizedBusinessType,
		"operation_type": normalizedOperationType,
		"title":          strings.TrimSpace(order.Title),
		"amount":         fmt.Sprintf("%.2f", order.Amount),
		"currency":       strings.TrimSpace(order.Currency),
		"callback_url":   strings.TrimSpace(order.CallbackURL),
		"return_url":     strings.TrimSpace(order.ReturnURL),
		"timestamp":      strconv.FormatInt(helper.GetTimestamp(), 10),
		"nonce":          random.GetUUID(),
	}
	if normalizedClientType := normalizeTopupOrderClientType(order.ClientType); normalizedClientType != "" {
		payload["client_type"] = normalizedClientType
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

func CreateTopupOrderWithDB(db *gorm.DB, userID string, username string, input CreateTopupOrderInput) (TopupOrder, error) {
	if db == nil {
		return TopupOrder{}, fmt.Errorf("database handle is nil")
	}
	businessType := normalizeTopupOrderBusinessType(input.BusinessType)
	if businessType == "" {
		return TopupOrder{}, fmt.Errorf("无效的业务类型")
	}
	amount := normalizeTopupOrderAmount(input.Amount)
	normalizedPlanID := strings.TrimSpace(input.PlanID)
	if amount <= 0 && businessType == TopupOrderBusinessBalance && normalizedPlanID == "" {
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
		OperationType: resolveTopupOrderOperationType(businessType, input.OperationType),
		Amount:        amount,
		Currency:      currency,
		Quota:         normalizeTopupOrderQuota(input.Quota),
		TopupPlanID:   strings.TrimSpace(input.PlanID),
		PackageID:     strings.TrimSpace(input.PackageID),
		ClientType:    strings.TrimSpace(input.ClientType),
		CallbackURL:   topupOrderCallbackURL(),
		ReturnURL:     sanitizeTopupReturnURL(input.ReturnURL),
	}
	if order.UserID == "" {
		return TopupOrder{}, fmt.Errorf("无效的 user id")
	}
	var redirectURL string
	var err error
	switch order.BusinessType {
	case TopupOrderBusinessBalance:
		planID := normalizedPlanID
		if planID != "" {
			resolvedPlan, err := ResolveTopupPlan(planID)
			if err != nil {
				return TopupOrder{}, err
			}
			order.TopupPlanID = strings.TrimSpace(resolvedPlan.Id)
			order.Amount = normalizeTopupOrderAmount(resolvedPlan.Amount)
			order.Currency = normalizeTopupOrderCurrency(resolvedPlan.AmountCurrency)
			order.Quota = normalizeTopupOrderQuota(resolvedPlan.QuotaYYC)
			order.ValidityDays = normalizeTopupPlanValidityDays(resolvedPlan.ValidityDays)
			if strings.TrimSpace(input.Title) != "" {
				order.Title = strings.TrimSpace(input.Title)
			} else {
				order.Title = buildTopupOrderPlanTitle(resolvedPlan)
			}
		} else {
			order.TopupPlanID = ""
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
			if strings.TrimSpace(input.Title) != "" {
				order.Title = strings.TrimSpace(input.Title)
			} else {
				order.Title = "账户充值"
			}
			order.ValidityDays = 0
		}
		if order.Quota <= 0 {
			return TopupOrder{}, fmt.Errorf("充值额度不能为空")
		}
	case TopupOrderBusinessPackage:
		if strings.TrimSpace(order.PackageID) == "" {
			return TopupOrder{}, fmt.Errorf("套餐 ID 不能为空")
		}
		preview, err := PreviewPackagePurchaseWithDB(
			db,
			strings.TrimSpace(order.UserID),
			strings.TrimSpace(order.PackageID),
			order.OperationType,
			helper.GetTimestamp(),
		)
		if err != nil {
			return TopupOrder{}, err
		}
		order.OperationType = resolveTopupOrderOperationType(TopupOrderBusinessPackage, preview.OperationType)
		order.Amount = normalizeTopupOrderAmount(preview.PayableAmount)
		order.Currency = normalizeTopupOrderCurrency(preview.PayableCurrency)
		if order.Amount <= 0 {
			return TopupOrder{}, fmt.Errorf("套餐应付金额必须大于 0")
		}
		order.PackageName = strings.TrimSpace(preview.TargetPackageName)
		if strings.TrimSpace(input.Title) != "" {
			order.Title = strings.TrimSpace(input.Title)
		} else {
			switch order.OperationType {
			case TopupOrderOperationRenew:
				if order.PackageName != "" {
					order.Title = "续费套餐：" + order.PackageName
				} else {
					order.Title = "续费套餐"
				}
			case TopupOrderOperationUpgrade:
				if order.PackageName != "" {
					order.Title = "升级套餐：" + order.PackageName
				} else {
					order.Title = "升级套餐"
				}
			default:
				if order.PackageName != "" {
					order.Title = "购买套餐：" + order.PackageName
				} else {
					order.Title = "购买套餐"
				}
			}
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
	now := helper.GetTimestamp()
	order.CreatedAt = now
	order.UpdatedAt = now
	normalizeTopupOrderRow(&order)

	if reusedOrder, reused, err := findReusableTopupOrderWithDB(db, order); err != nil {
		return TopupOrder{}, err
	} else if reused {
		logTopupOrderLifecycle("reused", reusedOrder, order.Status, "reuse existing unfinished order")
		return reusedOrder, nil
	}

	if order.Source == TopupOrderSourceTopUpAPI {
		if err := db.Create(&order).Error; err != nil {
			return TopupOrder{}, err
		}
		logTopupOrderLifecycle("created", order, "", "created local order before calling external payment api")
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
			failedOrder := order
			failedOrder.Status = TopupOrderStatusFailed
			failedOrder.StatusMessage = err.Error()
			failedOrder.UpdatedAt = updateTime
			logTopupOrderLifecycle("external_pay_create_failed", failedOrder, TopupOrderStatusCreated, err.Error())
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
		logTopupOrderLifecycle("external_pay_created", order, TopupOrderStatusCreated, "external payment order created and redirect url generated")
		return order, nil
	}

	order.RedirectURL = redirectURL
	normalizeTopupOrderRow(&order)
	if err := db.Create(&order).Error; err != nil {
		return TopupOrder{}, err
	}
	logTopupOrderLifecycle("created", order, "", "created redirect mode order")
	return order, nil
}

func findReusableTopupOrderWithDB(db *gorm.DB, order TopupOrder) (TopupOrder, bool, error) {
	if db == nil {
		return TopupOrder{}, false, fmt.Errorf("database handle is nil")
	}
	query := db.Model(&TopupOrder{}).
		Where("user_id = ?", order.UserID).
		Where("status IN ?", []string{
			TopupOrderStatusCreated,
			TopupOrderStatusPending,
			TopupOrderStatusPaid,
		})

	switch order.BusinessType {
	case TopupOrderBusinessPackage:
		operationType := resolveTopupOrderOperationType(TopupOrderBusinessPackage, order.OperationType)
		query = query.Where("business_type = ?", TopupOrderBusinessPackage).
			Where("package_id = ?", order.PackageID)
		if operationType == TopupOrderOperationNew {
			query = query.Where("(COALESCE(TRIM(operation_type), '') = '' OR operation_type = ?)", operationType)
		} else {
			query = query.Where("operation_type = ?", operationType)
		}
	case TopupOrderBusinessBalance:
		query = query.Where("business_type = ?", TopupOrderBusinessBalance).
			Where("amount = ?", order.Amount).
			Where("currency = ?", order.Currency).
			Where("quota = ?", order.Quota)
		if strings.TrimSpace(order.TopupPlanID) != "" {
			query = query.Where("topup_plan_id = ?", strings.TrimSpace(order.TopupPlanID))
		} else {
			query = query.Where("COALESCE(TRIM(topup_plan_id), '') = ''")
		}
	default:
		return TopupOrder{}, false, nil
	}

	rows := make([]TopupOrder, 0, 5)
	if err := query.Order("created_at desc, id desc").Limit(5).Find(&rows).Error; err != nil {
		return TopupOrder{}, false, err
	}
	for i := range rows {
		candidate := rows[i]
		normalizeTopupOrderRow(&candidate)
		if isDirtyTopupReturnURL(candidate.ReturnURL) ||
			strings.Contains(candidate.RedirectURL, "pay_status=") ||
			strings.Contains(candidate.RedirectURL, "trade_no=") {
			continue
		}
		if candidate.Source == TopupOrderSourceTopUpAPI &&
			strings.TrimSpace(candidate.ProviderOrderID) != "" &&
			(candidate.Status == TopupOrderStatusCreated ||
				candidate.Status == TopupOrderStatusPending ||
				candidate.Status == TopupOrderStatusPaid) {
			refreshedOrder, err := RefreshTopupOrderStatusWithDB(db, candidate.Id, candidate.UserID)
			if err == nil {
				candidate = refreshedOrder
			}
		}
		switch candidate.Status {
		case TopupOrderStatusCreated, TopupOrderStatusPending:
			return candidate, true, nil
		case TopupOrderStatusPaid:
			fulfilledOrder, _, err := FulfillTopupOrderWithDB(db, candidate.Id)
			if err != nil {
				return TopupOrder{}, false, err
			}
			return fulfilledOrder, true, nil
		}
	}
	return TopupOrder{}, false, nil
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

func GetTopupOrderByIDForAdminWithDB(db *gorm.DB, orderID string) (TopupOrder, error) {
	if db == nil {
		return TopupOrder{}, fmt.Errorf("database handle is nil")
	}
	normalizedOrderID := strings.TrimSpace(orderID)
	if normalizedOrderID == "" {
		return TopupOrder{}, gorm.ErrRecordNotFound
	}
	row := TopupOrder{}
	if err := db.Where("id = ?", normalizedOrderID).First(&row).Error; err != nil {
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

func ListTopupOrderReconcileCandidatesWithDB(db *gorm.DB, limit int, maxUpdatedAt int64) ([]TopupOrder, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	if limit <= 0 {
		limit = 20
	}
	query := db.Model(&TopupOrder{}).
		Where("source = ?", TopupOrderSourceTopUpAPI).
		Where("status IN ?", []string{
			TopupOrderStatusCreated,
			TopupOrderStatusPending,
			TopupOrderStatusPaid,
		})
	if maxUpdatedAt > 0 {
		query = query.Where("updated_at <= ?", maxUpdatedAt)
	}
	rows := make([]TopupOrder, 0, limit)
	if err := query.Order("updated_at asc, created_at asc, id asc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		normalizeTopupOrderRow(&rows[i])
	}
	return rows, nil
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
	result := TopupOrder{}
	previousStatus := ""
	err := db.Transaction(func(tx *gorm.DB) error {
		order, err := selectTopupOrderForCallbackWithDB(tx, normalizedOrderID, normalizedTransactionID, normalizedProviderOrderID)
		if err != nil {
			return err
		}
		previousStatus = order.Status
		if normalizedProviderName != "" {
			order.ProviderName = normalizedProviderName
		}
		if normalizedProviderOrderID != "" {
			order.ProviderOrderID = normalizedProviderOrderID
		}
		if normalizedStatusMessage != "" {
			order.StatusMessage = normalizedStatusMessage
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
	logTopupOrderLifecycle("callback_applied", result, previousStatus, normalizedStatusMessage)
	return result, nil
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
	previousStatus := ""
	err := db.Transaction(func(tx *gorm.DB) error {
		order := TopupOrder{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", normalizedOrderID).First(&order).Error; err != nil {
			return err
		}
		normalizeTopupOrderRow(&order)
		previousStatus = order.Status
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
			effectiveGrantedAt := order.PaidAt
			if effectiveGrantedAt <= 0 {
				effectiveGrantedAt = helper.GetTimestamp()
			}
			if order.CreditExpiresAt <= 0 {
				order.CreditExpiresAt = resolveBalanceCreditExpiresAt(effectiveGrantedAt, order.ValidityDays)
			}
			lot, creditedNow, err := CreditUserBalanceLotWithDB(tx, UserBalanceLotCreditInput{
				UserID:     order.UserID,
				SourceType: UserBalanceLotSourceTopup,
				SourceID:   order.Id,
				TotalYYC:   order.Quota,
				GrantedAt:  effectiveGrantedAt,
				ExpiresAt:  order.CreditExpiresAt,
			})
			if err != nil {
				return err
			}
			if creditedNow {
				if err := tx.Model(&User{}).
					Where("id = ?", order.UserID).
					Update("quota", gorm.Expr("quota + ?", order.Quota)).Error; err != nil {
					return err
				}
			}
			if lot.ExpiresAt > 0 {
				order.CreditExpiresAt = lot.ExpiresAt
			}
		case TopupOrderBusinessPackage:
			if strings.TrimSpace(order.PackageID) == "" {
				return fmt.Errorf("套餐 ID 不能为空")
			}
			switch resolveTopupOrderOperationType(TopupOrderBusinessPackage, order.OperationType) {
			case TopupOrderOperationRenew:
				if _, err := RenewServicePackageForUserWithDB(tx, order.PackageID, order.UserID, helper.GetTimestamp()); err != nil {
					return err
				}
			case TopupOrderOperationUpgrade:
				if _, err := UpgradeServicePackageForUserWithDB(tx, order.PackageID, order.UserID, helper.GetTimestamp()); err != nil {
					return err
				}
			default:
				if _, err := AssignServicePackageToUserWithDB(tx, order.PackageID, order.UserID, helper.GetTimestamp()); err != nil {
					return err
				}
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
	if fulfilledNow {
		logTopupOrderLifecycle("fulfilled", result, previousStatus, result.StatusMessage)
	}
	return result, fulfilledNow, nil
}

func GrantTopupPlanToUserWithDB(db *gorm.DB, userID string, username string, planID string, grantedBy string) (TopupOrder, error) {
	if db == nil {
		return TopupOrder{}, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return TopupOrder{}, fmt.Errorf("用户 ID 不能为空")
	}
	normalizedPlanID := strings.TrimSpace(planID)
	if normalizedPlanID == "" {
		return TopupOrder{}, fmt.Errorf("充值额度不能为空")
	}
	normalizedUsername := strings.TrimSpace(username)
	resolvedPlan, err := ResolveTopupPlan(normalizedPlanID)
	if err != nil {
		return TopupOrder{}, err
	}
	now := helper.GetTimestamp()
	result := TopupOrder{}
	err = db.Transaction(func(tx *gorm.DB) error {
		user := User{}
		if err := tx.Select("id").First(&user, "id = ?", normalizedUserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("用户不存在")
			}
			return err
		}
		order := TopupOrder{
			Id:            random.GetUUID(),
			UserID:        normalizedUserID,
			Username:      normalizedUsername,
			Status:        TopupOrderStatusFulfilled,
			Source:        TopupOrderSourceTopUpAPI,
			ProviderName:  "admin",
			TransactionID: random.GetUUID(),
			BusinessType:  TopupOrderBusinessBalance,
			OperationType: TopupOrderOperationTopup,
			Title:         buildTopupOrderPlanTitle(resolvedPlan),
			Amount:        normalizeTopupOrderAmount(resolvedPlan.Amount),
			Currency:      normalizeTopupOrderCurrency(resolvedPlan.AmountCurrency),
			Quota:         normalizeTopupOrderQuota(resolvedPlan.QuotaYYC),
			TopupPlanID:   strings.TrimSpace(resolvedPlan.Id),
			ValidityDays:  normalizeTopupPlanValidityDays(resolvedPlan.ValidityDays),
			PaidAt:        now,
			RedeemedAt:    now,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if order.Quota <= 0 {
			return fmt.Errorf("充值额度不能为空")
		}
		if strings.TrimSpace(order.Title) == "" {
			order.Title = "管理员赠送充值额度"
		}
		normalizedGrantedBy := strings.TrimSpace(grantedBy)
		if normalizedGrantedBy != "" {
			order.StatusMessage = "管理员赠送，操作者：" + normalizedGrantedBy
		} else {
			order.StatusMessage = "管理员赠送"
		}
		if order.ValidityDays > 0 {
			order.CreditExpiresAt = resolveBalanceCreditExpiresAt(now, order.ValidityDays)
		}
		normalizeTopupOrderRow(&order)
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		lot, creditedNow, err := CreditUserBalanceLotWithDB(tx, UserBalanceLotCreditInput{
			UserID:     normalizedUserID,
			SourceType: UserBalanceLotSourceTopup,
			SourceID:   order.Id,
			TotalYYC:   order.Quota,
			GrantedAt:  now,
			ExpiresAt:  order.CreditExpiresAt,
		})
		if err != nil {
			return err
		}
		if creditedNow {
			if err := tx.Model(&User{}).
				Where("id = ?", normalizedUserID).
				Update("quota", gorm.Expr("quota + ?", order.Quota)).Error; err != nil {
				return err
			}
		}
		if lot.ExpiresAt > 0 && lot.ExpiresAt != order.CreditExpiresAt {
			order.CreditExpiresAt = lot.ExpiresAt
			if err := tx.Model(&TopupOrder{}).
				Where("id = ?", order.Id).
				Update("credit_expires_at", order.CreditExpiresAt).Error; err != nil {
				return err
			}
		}
		result = order
		return nil
	})
	if err != nil {
		return TopupOrder{}, err
	}
	logTopupOrderLifecycle("granted", result, "", result.StatusMessage)
	return result, nil
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
