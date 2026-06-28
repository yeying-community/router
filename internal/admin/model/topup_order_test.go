package model

import (
	"net/url"
	"testing"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTopupOrderTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&TopupOrder{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return db
}

func TestBuildTopupOrderRedirectURL(t *testing.T) {
	previousSecret := config.TopUpSignSecret
	previousMerchantApp := config.TopUpMerchantApp
	config.TopUpSignSecret = "test-sign-secret"
	config.TopUpMerchantApp = ""
	t.Cleanup(func() {
		config.TopUpSignSecret = previousSecret
		config.TopUpMerchantApp = previousMerchantApp
	})
	redirectURL, err := buildTopupOrderRedirectURL(
		"https://pay.example.com/checkout?source=router",
		TopupOrder{
			Id:            "order_1",
			UserID:        "user_1",
			Username:      "alice",
			TransactionID: "txn_1",
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parsed, err := url.Parse(redirectURL)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	query := parsed.Query()
	if got := query.Get("source"); got != "router" {
		t.Fatalf("expected source=router, got %q", got)
	}
	if got := query.Get("order_id"); got != "order_1" {
		t.Fatalf("expected order_id=order_1, got %q", got)
	}
	if got := query.Get("user_id"); got != "user_1" {
		t.Fatalf("expected user_id=user_1, got %q", got)
	}
	if got := query.Get("username"); got != "alice" {
		t.Fatalf("expected username=alice, got %q", got)
	}
	if got := query.Get("transaction_id"); got != "txn_1" {
		t.Fatalf("expected transaction_id=txn_1, got %q", got)
	}
	if got := query.Get("merchant_app"); got != "router" {
		t.Fatalf("expected merchant_app=router, got %q", got)
	}
	if got := query.Get("operation_type"); got != TopupOrderOperationTopup {
		t.Fatalf("expected operation_type=%s, got %q", TopupOrderOperationTopup, got)
	}
	if got := query.Get("sign"); got == "" {
		t.Fatal("expected sign to be set")
	}
}

func TestBuildTopupOrderRedirectURLUsesConfiguredMerchantAppAndClientType(t *testing.T) {
	previousSecret := config.TopUpSignSecret
	previousMerchantApp := config.TopUpMerchantApp
	config.TopUpSignSecret = "test-sign-secret"
	config.TopUpMerchantApp = "router-pay"
	t.Cleanup(func() {
		config.TopUpSignSecret = previousSecret
		config.TopUpMerchantApp = previousMerchantApp
	})
	redirectURL, err := buildTopupOrderRedirectURL(
		"https://pay.example.com/checkout",
		TopupOrder{
			Id:            "order_1",
			UserID:        "user_1",
			Username:      "alice",
			TransactionID: "txn_1",
			ClientType:    "mobile",
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parsed, err := url.Parse(redirectURL)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	query := parsed.Query()
	if got := query.Get("merchant_app"); got != "router-pay" {
		t.Fatalf("expected merchant_app=router-pay, got %q", got)
	}
	if got := query.Get("client_type"); got != "mobile" {
		t.Fatalf("expected client_type=mobile, got %q", got)
	}
}

func TestBuildTopupOrderRedirectURLRejectsInvalidBaseLink(t *testing.T) {
	previousSecret := config.TopUpSignSecret
	previousMerchantApp := config.TopUpMerchantApp
	config.TopUpSignSecret = "test-sign-secret"
	config.TopUpMerchantApp = ""
	t.Cleanup(func() {
		config.TopUpSignSecret = previousSecret
		config.TopUpMerchantApp = previousMerchantApp
	})
	if _, err := buildTopupOrderRedirectURL("://broken", TopupOrder{
		Id:            "order_1",
		UserID:        "user_1",
		Username:      "alice",
		TransactionID: "txn_1",
	}); err == nil {
		t.Fatal("expected error for invalid base link")
	}
}

func TestResolveTopupOrderBusinessType(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		packageID  string
		wantResult string
	}{
		{
			name:       "explicit balance",
			value:      TopupOrderBusinessBalance,
			wantResult: TopupOrderBusinessBalance,
		},
		{
			name:       "explicit package",
			value:      TopupOrderBusinessPackage,
			wantResult: TopupOrderBusinessPackage,
		},
		{
			name:       "infer package from package id",
			packageID:  "pkg_1",
			wantResult: TopupOrderBusinessPackage,
		},
		{
			name:       "fallback balance for legacy empty type",
			wantResult: TopupOrderBusinessBalance,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveTopupOrderBusinessType(tt.value, tt.packageID); got != tt.wantResult {
				t.Fatalf("expected %q, got %q", tt.wantResult, got)
			}
		})
	}
}

func TestResolveTopupOrderOperationType(t *testing.T) {
	tests := []struct {
		name         string
		businessType string
		value        string
		wantResult   string
	}{
		{
			name:         "balance enforces topup operation",
			businessType: TopupOrderBusinessBalance,
			value:        TopupOrderOperationUpgrade,
			wantResult:   TopupOrderOperationTopup,
		},
		{
			name:         "explicit renew",
			businessType: TopupOrderBusinessPackage,
			value:        TopupOrderOperationRenew,
			wantResult:   TopupOrderOperationRenew,
		},
		{
			name:         "explicit upgrade",
			businessType: TopupOrderBusinessPackage,
			value:        TopupOrderOperationUpgrade,
			wantResult:   TopupOrderOperationUpgrade,
		},
		{
			name:         "explicit downgrade",
			businessType: TopupOrderBusinessPackage,
			value:        TopupOrderOperationDowngrade,
			wantResult:   TopupOrderOperationDowngrade,
		},
		{
			name:         "explicit convert",
			businessType: TopupOrderBusinessPackage,
			value:        TopupOrderOperationConvert,
			wantResult:   TopupOrderOperationConvert,
		},
		{
			name:         "fallback to new for package",
			businessType: TopupOrderBusinessPackage,
			value:        "",
			wantResult:   TopupOrderOperationNew,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveTopupOrderOperationType(tt.businessType, tt.value); got != tt.wantResult {
				t.Fatalf("expected %q, got %q", tt.wantResult, got)
			}
		})
	}
}

func TestBuildTopupOrderPlanTitle(t *testing.T) {
	plan := ResolvedTopupPlan{
		TopupPlan: TopupPlan{
			Name:           "基础版",
			GroupName:      "enterprise",
			Amount:         1,
			AmountCurrency: BillingCurrencyCodeCNY,
			QuotaAmount:    20,
			QuotaCurrency:  BillingCurrencyCodeUSD,
		},
	}
	if got, want := buildTopupOrderPlanTitle(plan), "1 元 / 20 USD"; got != want {
		t.Fatalf("unexpected title: got %q want %q", got, want)
	}
}

func TestTopupOrderSigningStringHelpers(t *testing.T) {
	payload := map[string]string{
		"b":    "2",
		"a":    "1",
		"skip": "",
		"sign": "ignored",
	}
	if got, want := topupOrderSigningBaseString(payload), "a=1&b=2"; got != want {
		t.Fatalf("unexpected signing base string: got %q want %q", got, want)
	}
	if got, want := topupOrderSigningString(payload, "secret-value"), "a=1&b=2&secret=secret-value"; got != want {
		t.Fatalf("unexpected signing string: got %q want %q", got, want)
	}
}

func TestApplyTopupOrderCallbackMapsProviderFulfilledToPaid(t *testing.T) {
	db := newTopupOrderTestDB(t)
	order := TopupOrder{
		Id:            "order-1",
		UserID:        "user-1",
		Status:        TopupOrderStatusCreated,
		TransactionID: "txn-1",
		BusinessType:  TopupOrderBusinessBalance,
		OperationType: TopupOrderOperationTopup,
		Amount:        1,
		Currency:      TopupOrderCurrencyCNY,
		Quota:         100,
		CreatedAt:     1000,
		UpdatedAt:     1000,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}

	got, err := ApplyTopupOrderCallbackWithDB(db, TopupOrderCallbackInput{
		OrderID: order.Id,
		Status:  TopupOrderStatusFulfilled,
		PaidAt:  1234,
	})
	if err != nil {
		t.Fatalf("apply callback: %v", err)
	}
	if got.Status != TopupOrderStatusPaid {
		t.Fatalf("status = %q, want paid before local fulfillment", got.Status)
	}
	if got.PaidAt != 1234 {
		t.Fatalf("paid_at = %d, want 1234", got.PaidAt)
	}
	if got.RedeemedAt != 0 {
		t.Fatalf("redeemed_at = %d, want 0 before local fulfillment", got.RedeemedAt)
	}
}

func TestApplyTopupOrderCallbackDoesNotDowngradeFulfilledOrder(t *testing.T) {
	db := newTopupOrderTestDB(t)
	order := TopupOrder{
		Id:            "order-1",
		UserID:        "user-1",
		Status:        TopupOrderStatusFulfilled,
		TransactionID: "txn-1",
		BusinessType:  TopupOrderBusinessBalance,
		OperationType: TopupOrderOperationTopup,
		Amount:        1,
		Currency:      TopupOrderCurrencyCNY,
		Quota:         100,
		PaidAt:        1234,
		RedeemedAt:    1240,
		CreatedAt:     1000,
		UpdatedAt:     1240,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}

	got, err := ApplyTopupOrderCallbackWithDB(db, TopupOrderCallbackInput{
		OrderID:       order.Id,
		Status:        TopupOrderStatusFailed,
		StatusMessage: "late failed callback",
	})
	if err != nil {
		t.Fatalf("apply callback: %v", err)
	}
	if got.Status != TopupOrderStatusFulfilled {
		t.Fatalf("status = %q, want fulfilled", got.Status)
	}
	if got.StatusMessage != "" {
		t.Fatalf("status_message = %q, want unchanged", got.StatusMessage)
	}
}

func TestPreviewPackagePurchaseUsesCurrentActiveSubscriptionAcrossGroups(t *testing.T) {
	db := newServicePackageScopeTestDB(t)
	now := helper.GetTimestamp()
	if err := db.Create(&GroupCatalog{
		Id:      "group-2",
		Name:    "second",
		Enabled: true,
	}).Error; err != nil {
		t.Fatalf("seed second group: %v", err)
	}
	currentPackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:         "glm monthly",
		GroupID:      "group-1",
		PackageType:  ServicePackageTypeRequestQuota,
		QuotaMetric:  ServicePackageQuotaMetricRequestCount,
		PeriodLimit:  100,
		SalePrice:    100,
		SaleCurrency: "CNY",
		DurationDays: 30,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create current package: %v", err)
	}
	targetPackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:         "qwen monthly plus",
		GroupID:      "group-2",
		PackageType:  ServicePackageTypeRequestQuota,
		QuotaMetric:  ServicePackageQuotaMetricRequestCount,
		PeriodLimit:  200,
		SalePrice:    150,
		SaleCurrency: "CNY",
		DurationDays: 30,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create target package: %v", err)
	}
	subscription, err := AssignServicePackageToUserWithDB(db, currentPackage.Id, "user-1", now)
	if err != nil {
		t.Fatalf("assign current package: %v", err)
	}

	preview, err := PreviewPackagePurchaseWithDB(db, "user-1", targetPackage.Id, "", now+60)
	if err != nil {
		t.Fatalf("preview package purchase: %v", err)
	}
	if preview.OperationType != TopupOrderOperationUpgrade {
		t.Fatalf("operation_type=%q, want %q", preview.OperationType, TopupOrderOperationUpgrade)
	}
	if preview.CurrentPackageID != currentPackage.Id || preview.CurrentPackageName != currentPackage.Name {
		t.Fatalf("current package=%q/%q, want %q/%q", preview.CurrentPackageID, preview.CurrentPackageName, currentPackage.Id, currentPackage.Name)
	}
	if preview.CurrentExpiresAt != subscription.ExpiresAt {
		t.Fatalf("current_expires_at=%d, want %d", preview.CurrentExpiresAt, subscription.ExpiresAt)
	}
	if preview.TargetPackageAmount != normalizeTopupOrderAmount(targetPackage.SalePrice) {
		t.Fatalf("target_package_amount=%.2f, want %.2f", preview.TargetPackageAmount, normalizeTopupOrderAmount(targetPackage.SalePrice))
	}
	if preview.PayableAmount <= 0 || preview.PayableAmount >= preview.TargetPackageAmount {
		t.Fatalf("payable_amount=%.2f, want between 0 and target price %.2f", preview.PayableAmount, preview.TargetPackageAmount)
	}
	if preview.CurrentPackageCreditAmount <= 0 {
		t.Fatalf("current_package_credit_amount=%.2f, want > 0", preview.CurrentPackageCreditAmount)
	}
	if normalizeTopupOrderAmount(preview.PayableAmount+preview.CurrentPackageCreditAmount) != preview.TargetPackageAmount {
		t.Fatalf("payable + credit = %.2f, want target %.2f", normalizeTopupOrderAmount(preview.PayableAmount+preview.CurrentPackageCreditAmount), preview.TargetPackageAmount)
	}
}

func TestPreviewPackagePurchaseRenewRequiresCurrentActivePackage(t *testing.T) {
	db := newServicePackageScopeTestDB(t)
	now := helper.GetTimestamp()
	if err := db.Create(&GroupCatalog{
		Id:      "group-2",
		Name:    "second",
		Enabled: true,
	}).Error; err != nil {
		t.Fatalf("seed second group: %v", err)
	}
	currentPackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:         "glm monthly",
		GroupID:      "group-1",
		PackageType:  ServicePackageTypeRequestQuota,
		QuotaMetric:  ServicePackageQuotaMetricRequestCount,
		PeriodLimit:  100,
		SalePrice:    100,
		SaleCurrency: "CNY",
		DurationDays: 30,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create current package: %v", err)
	}
	otherPackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:         "qwen monthly",
		GroupID:      "group-2",
		PackageType:  ServicePackageTypeRequestQuota,
		QuotaMetric:  ServicePackageQuotaMetricRequestCount,
		PeriodLimit:  200,
		SalePrice:    80,
		SaleCurrency: "CNY",
		DurationDays: 30,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create other package: %v", err)
	}
	if _, err := AssignServicePackageToUserWithDB(db, currentPackage.Id, "user-1", now); err != nil {
		t.Fatalf("assign current package: %v", err)
	}

	if _, err := PreviewPackagePurchaseWithDB(db, "user-1", otherPackage.Id, TopupOrderOperationRenew, now+60); err == nil {
		t.Fatal("expected renew preview to fail for non-current package")
	}
}

func TestPreviewPackagePurchaseDowngradeSchedulesNextPackage(t *testing.T) {
	db := newServicePackageScopeTestDB(t)
	now := helper.GetTimestamp()
	currentPackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:         "pro monthly",
		GroupID:      "group-1",
		PackageType:  ServicePackageTypeRequestQuota,
		QuotaMetric:  ServicePackageQuotaMetricRequestCount,
		PeriodLimit:  1000,
		SalePrice:    200,
		SaleCurrency: "CNY",
		DurationDays: 30,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create current package: %v", err)
	}
	targetPackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:         "basic monthly",
		GroupID:      "group-1",
		PackageType:  ServicePackageTypeRequestQuota,
		QuotaMetric:  ServicePackageQuotaMetricRequestCount,
		PeriodLimit:  100,
		SalePrice:    80,
		SaleCurrency: "CNY",
		DurationDays: 30,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create target package: %v", err)
	}
	subscription, err := AssignServicePackageToUserWithDB(db, currentPackage.Id, "user-1", now)
	if err != nil {
		t.Fatalf("assign current package: %v", err)
	}
	preview, err := PreviewPackagePurchaseWithDB(db, "user-1", targetPackage.Id, TopupOrderOperationDowngrade, now+60)
	if err != nil {
		t.Fatalf("preview downgrade: %v", err)
	}
	if preview.OperationType != TopupOrderOperationDowngrade {
		t.Fatalf("operation_type=%q, want %q", preview.OperationType, TopupOrderOperationDowngrade)
	}
	if preview.StartAt != subscription.ExpiresAt {
		t.Fatalf("start_at=%d, want %d", preview.StartAt, subscription.ExpiresAt)
	}
	if preview.PayableAmount != normalizeTopupOrderAmount(targetPackage.SalePrice) {
		t.Fatalf("payable_amount=%.2f, want %.2f", preview.PayableAmount, normalizeTopupOrderAmount(targetPackage.SalePrice))
	}
}

func TestPreviewPackagePurchaseConvertRequiresDifferentType(t *testing.T) {
	db := newServicePackageScopeTestDB(t)
	now := helper.GetTimestamp()
	currentPackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:            "yyc package",
		GroupID:         "group-1",
		PackageType:     ServicePackageTypeYYCQuota,
		QuotaMetric:     ServicePackageQuotaMetricYYC,
		DailyQuotaLimit: 1000,
		SalePrice:       50,
		SaleCurrency:    "CNY",
		DurationDays:    30,
		Enabled:         true,
	})
	if err != nil {
		t.Fatalf("create current package: %v", err)
	}
	targetPackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:         "request package",
		GroupID:      "group-1",
		PackageType:  ServicePackageTypeRequestQuota,
		QuotaMetric:  ServicePackageQuotaMetricRequestCount,
		PeriodLimit:  100,
		SalePrice:    80,
		SaleCurrency: "CNY",
		DurationDays: 30,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create target package: %v", err)
	}
	if _, err := AssignServicePackageToUserWithDB(db, currentPackage.Id, "user-1", now); err != nil {
		t.Fatalf("assign current package: %v", err)
	}
	preview, err := PreviewPackagePurchaseWithDB(db, "user-1", targetPackage.Id, TopupOrderOperationConvert, now+60)
	if err != nil {
		t.Fatalf("preview convert: %v", err)
	}
	if preview.OperationType != TopupOrderOperationConvert {
		t.Fatalf("operation_type=%q, want %q", preview.OperationType, TopupOrderOperationConvert)
	}
}
