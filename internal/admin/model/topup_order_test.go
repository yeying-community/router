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

func TestListTopupOrdersPageFilteredByCreditOrigin(t *testing.T) {
	db := newTopupOrderTestDB(t)
	orders := []TopupOrder{
		{
			Id:            "paid-order",
			UserID:        "user-1",
			TransactionID: "paid-transaction",
			BusinessType:  TopupOrderBusinessBalance,
			CreditOrigin:  TopupOrderCreditOriginPaid,
			CreatedAt:     30,
		},
		{
			Id:            "gift-order",
			UserID:        "user-1",
			TransactionID: "gift-transaction",
			BusinessType:  TopupOrderBusinessBalance,
			CreditOrigin:  TopupOrderCreditOriginAdmin,
			CreatedAt:     20,
		},
		{
			Id:            "package-order",
			UserID:        "user-1",
			TransactionID: "package-transaction",
			BusinessType:  TopupOrderBusinessPackage,
			CreditOrigin:  TopupOrderCreditOriginPaid,
			CreatedAt:     10,
		},
		{
			Id:            "unknown-origin-order",
			UserID:        "user-1",
			TransactionID: "unknown-origin-transaction",
			BusinessType:  TopupOrderBusinessBalance,
			CreditOrigin:  "legacy_unknown",
			CreatedAt:     5,
		},
	}
	if err := db.Create(&orders).Error; err != nil {
		t.Fatalf("create orders: %v", err)
	}

	paid, paidTotal, err := ListTopupOrdersPageFilteredWithDB(
		db,
		"user-1",
		TopupOrderBusinessBalance,
		TopupOrderCreditFilterPaid,
		1,
		20,
	)
	if err != nil {
		t.Fatalf("list paid orders: %v", err)
	}
	if paidTotal != 1 || len(paid) != 1 || paid[0].Id != "paid-order" {
		t.Fatalf("paid orders=%+v total=%d, want paid-order only", paid, paidTotal)
	}

	gifts, giftTotal, err := ListTopupOrdersPageFilteredWithDB(
		db,
		"user-1",
		TopupOrderBusinessBalance,
		TopupOrderCreditFilterGift,
		1,
		20,
	)
	if err != nil {
		t.Fatalf("list gift orders: %v", err)
	}
	if giftTotal != 1 || len(gifts) != 1 || gifts[0].Id != "gift-order" {
		t.Fatalf("gift orders=%+v total=%d, want gift-order only", gifts, giftTotal)
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

func TestGrantBalanceAmountToUserCreatesOrderLotAndQuota(t *testing.T) {
	db := newTopupOrderTestDB(t)
	if err := db.AutoMigrate(&User{}, &UserBalanceLot{}, &UserBalanceLotTransaction{}); err != nil {
		t.Fatalf("AutoMigrate grant balance dependencies: %v", err)
	}
	user := User{
		Id:       "user-1",
		Username: "alice",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	order, err := GrantBalanceAmountToUserWithDB(db, user.Id, user.Username, 250, "管理员补偿充值", "admin-1")
	if err != nil {
		t.Fatalf("GrantBalanceAmountToUserWithDB: %v", err)
	}
	if order.Status != TopupOrderStatusFulfilled {
		t.Fatalf("order status=%q, want %q", order.Status, TopupOrderStatusFulfilled)
	}
	if order.Source != TopupOrderSourceTopUpAPI {
		t.Fatalf("order source=%q, want %q", order.Source, TopupOrderSourceTopUpAPI)
	}
	if order.BusinessType != TopupOrderBusinessBalance || order.OperationType != TopupOrderOperationTopup {
		t.Fatalf("unexpected order type: business=%q operation=%q", order.BusinessType, order.OperationType)
	}
	if order.Quota != 250 {
		t.Fatalf("order quota=%d, want 250", order.Quota)
	}
	if order.CreditOrigin != TopupOrderCreditOriginAdmin {
		t.Fatalf("order credit_origin=%q, want %q", order.CreditOrigin, TopupOrderCreditOriginAdmin)
	}

	gotUser := User{}
	if err := db.First(&gotUser, "id = ?", user.Id).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	effectiveBalance, err := GetEffectiveUserBalanceAmountWithDB(db, user.Id, helper.GetTimestamp())
	if err != nil {
		t.Fatalf("load effective balance: %v", err)
	}
	if effectiveBalance != 250 {
		t.Fatalf("effective balance=%d, want 250", effectiveBalance)
	}

	lot := UserBalanceLot{}
	if err := db.First(&lot, "source_type = ? AND source_id = ?", UserBalanceLotSourceTopup, order.Id).Error; err != nil {
		t.Fatalf("load balance lot: %v", err)
	}
	if lot.UserID != user.Id || lot.TotalAmount != 250 || lot.RemainingAmount != 250 || lot.Status != UserBalanceLotStatusActive {
		t.Fatalf("unexpected lot: user=%q total=%d remaining=%d status=%q", lot.UserID, lot.TotalAmount, lot.RemainingAmount, lot.Status)
	}

	tx := UserBalanceLotTransaction{}
	if err := db.First(&tx, "lot_id = ? AND tx_type = ?", lot.Id, UserBalanceLotTxTypeCredit).Error; err != nil {
		t.Fatalf("load balance lot transaction: %v", err)
	}
	if tx.SourceType != UserBalanceLotSourceTopup || tx.SourceID != order.Id || tx.DeltaAmount != 250 {
		t.Fatalf("unexpected lot tx: source=%q/%q delta=%d", tx.SourceType, tx.SourceID, tx.DeltaAmount)
	}
}

func TestPreviewPackagePurchaseTreatsDifferentGroupAsPurchase(t *testing.T) {
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
	if _, err := AssignServicePackageToUserWithDB(db, currentPackage.Id, "user-1", now); err != nil {
		t.Fatalf("assign current package: %v", err)
	}

	preview, err := PreviewPackagePurchaseWithDB(db, "user-1", targetPackage.Id, "", now+60)
	if err != nil {
		t.Fatalf("preview package purchase: %v", err)
	}
	if preview.OperationType != TopupOrderOperationNew {
		t.Fatalf("operation_type=%q, want %q", preview.OperationType, TopupOrderOperationNew)
	}
	if preview.SlotActivePackageID != "" || preview.SlotActivePackageName != "" || preview.SlotActivePackageExpiresAt != 0 {
		t.Fatalf("slot active package should be empty for different slot purchase: %+v", preview)
	}
	if preview.TargetPackageAmount != normalizeTopupOrderAmount(targetPackage.SalePrice) {
		t.Fatalf("target_package_amount=%.2f, want %.2f", preview.TargetPackageAmount, normalizeTopupOrderAmount(targetPackage.SalePrice))
	}
	if preview.PayableAmount != preview.TargetPackageAmount {
		t.Fatalf("payable_amount=%.2f, want target price %.2f", preview.PayableAmount, preview.TargetPackageAmount)
	}
}

func TestPreviewPackagePurchaseUpgradesSamePackageSlot(t *testing.T) {
	db := newServicePackageScopeTestDB(t)
	now := helper.GetTimestamp()
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
		Name:         "glm monthly plus",
		GroupID:      "group-1",
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
	if preview.SlotActivePackageID != currentPackage.Id || preview.SlotActivePackageName != currentPackage.Name {
		t.Fatalf("slot active package=%q/%q, want %q/%q", preview.SlotActivePackageID, preview.SlotActivePackageName, currentPackage.Id, currentPackage.Name)
	}
	if preview.SlotActivePackageExpiresAt != subscription.ExpiresAt {
		t.Fatalf("slot_active_package_expires_at=%d, want %d", preview.SlotActivePackageExpiresAt, subscription.ExpiresAt)
	}
	if preview.PayableAmount <= 0 || preview.PayableAmount >= preview.TargetPackageAmount {
		t.Fatalf("payable_amount=%.2f, want between 0 and target price %.2f", preview.PayableAmount, preview.TargetPackageAmount)
	}
	if preview.SlotActivePackageCreditAmount <= 0 {
		t.Fatalf("slot_active_package_credit_amount=%.2f, want > 0", preview.SlotActivePackageCreditAmount)
	}
	if normalizeTopupOrderAmount(preview.PayableAmount+preview.SlotActivePackageCreditAmount) != preview.TargetPackageAmount {
		t.Fatalf("payable + credit = %.2f, want target %.2f", normalizeTopupOrderAmount(preview.PayableAmount+preview.SlotActivePackageCreditAmount), preview.TargetPackageAmount)
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
	if _, err := AssignServicePackageToUserWithDB(db, currentPackage.Id, "user-1", now); err != nil {
		t.Fatalf("assign current package: %v", err)
	}
	preview, err := PreviewPackagePurchaseWithDB(db, "user-1", targetPackage.Id, TopupOrderOperationDowngrade, now+60)
	if err != nil {
		t.Fatalf("preview downgrade: %v", err)
	}
	if preview.OperationType != TopupOrderOperationDowngrade {
		t.Fatalf("operation_type=%q, want %q", preview.OperationType, TopupOrderOperationDowngrade)
	}
	if preview.StartAt != now+60 {
		t.Fatalf("start_at=%d, want %d", preview.StartAt, now+60)
	}
	if preview.PayableAmount != normalizeTopupOrderAmount(targetPackage.SalePrice) {
		t.Fatalf("payable_amount=%.2f, want %.2f", preview.PayableAmount, normalizeTopupOrderAmount(targetPackage.SalePrice))
	}
}

func TestPreviewPackagePurchaseConvertDifferentSlotFallsBackToPurchase(t *testing.T) {
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
	if preview.OperationType != TopupOrderOperationNew {
		t.Fatalf("operation_type=%q, want %q", preview.OperationType, TopupOrderOperationNew)
	}
	if preview.SlotActivePackageID != "" {
		t.Fatalf("slot_active_package_id=%q, want empty for different slot", preview.SlotActivePackageID)
	}
}
