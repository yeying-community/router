package user

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTopupBalanceLotQueryContext(rawQuery string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodGet, "/?"+rawQuery, nil)
	context.Request = request
	return context
}

func TestParseTopupBalanceLotPageParamsDefaultsToHistoricalLots(t *testing.T) {
	_, _, _, _, positiveOnly, err := parseTopupBalanceLotPageParams(newTopupBalanceLotQueryContext(""))
	if err != nil {
		t.Fatalf("parse default params: %v", err)
	}
	if positiveOnly {
		t.Fatalf("positiveOnly default = true, want false")
	}
}

func TestParseTopupBalanceLotPageParamsAcceptsExplicitFilters(t *testing.T) {
	page, pageSize, sourceType, status, positiveOnly, err := parseTopupBalanceLotPageParams(newTopupBalanceLotQueryContext("page=2&page_size=50&source_type=redemption&status=expired&positive_only=true"))
	if err != nil {
		t.Fatalf("parse explicit params: %v", err)
	}
	if page != 2 || pageSize != 50 {
		t.Fatalf("page/pageSize=%d/%d, want 2/50", page, pageSize)
	}
	if sourceType != model.UserBalanceLotSourceRedeem {
		t.Fatalf("sourceType=%q, want %q", sourceType, model.UserBalanceLotSourceRedeem)
	}
	if status != model.UserBalanceLotStatusExpired {
		t.Fatalf("status=%q, want %q", status, model.UserBalanceLotStatusExpired)
	}
	if !positiveOnly {
		t.Fatalf("positiveOnly explicit true = false, want true")
	}
}

func TestBuildTopUpBalanceSummaryUsesEffectiveLotBucketsOnly(t *testing.T) {
	summary := buildTopUpBalanceSummary(100, 30, 20)
	if summary.TopupBalanceAmount != 100 {
		t.Fatalf("topup_balance_amount=%d, want 100", summary.TopupBalanceAmount)
	}
	if summary.RedeemBalanceAmount != 30 {
		t.Fatalf("redeem_balance_amount=%d, want 30", summary.RedeemBalanceAmount)
	}
	if summary.GiftBalanceAmount != 20 {
		t.Fatalf("gift_balance_amount=%d, want 20", summary.GiftBalanceAmount)
	}
	if summary.TotalBalanceAmount != 150 {
		t.Fatalf("total_balance_amount=%d, want 150", summary.TotalBalanceAmount)
	}
}

func TestBuildTopUpBalanceSummaryDoesNotAttributeUnknownResidual(t *testing.T) {
	summary := buildTopUpBalanceSummary(10, 5, 0)
	if summary.TotalBalanceAmount != 15 {
		t.Fatalf("total_balance_amount=%d, want effective lot sum 15", summary.TotalBalanceAmount)
	}
	if summary.TopupBalanceAmount != 10 {
		t.Fatalf("topup_balance_amount=%d, want 10", summary.TopupBalanceAmount)
	}
}

func TestParseTopupOrderPageParamsAcceptsCreditOriginFilter(t *testing.T) {
	page, pageSize, businessType, creditFilter, err := parseTopupOrderPageParams(
		newTopupBalanceLotQueryContext("page=2&page_size=30&business_type=balance_topup&credit_origin=gift"),
	)
	if err != nil {
		t.Fatalf("parse order params: %v", err)
	}
	if page != 2 || pageSize != 30 {
		t.Fatalf("page/pageSize=%d/%d, want 2/30", page, pageSize)
	}
	if businessType != model.TopupOrderBusinessBalance {
		t.Fatalf("businessType=%q, want %q", businessType, model.TopupOrderBusinessBalance)
	}
	if creditFilter != model.TopupOrderCreditFilterGift {
		t.Fatalf("creditFilter=%q, want %q", creditFilter, model.TopupOrderCreditFilterGift)
	}
}

func TestBuildUserQuotaOverviewCombinesPackageAndBalanceAmounts(t *testing.T) {
	overview := buildUserQuotaOverview(
		model.UserQuotaSummary{
			Daily: model.UserDailyQuotaSnapshot{
				BizDate:        "2026-07-10",
				Timezone:       "Asia/Shanghai",
				Limit:          100,
				ConsumedQuota:  25,
				ReservedQuota:  5,
				RemainingQuota: 70,
			},
		},
		buildTopUpBalanceSummary(20, 10, 5),
		15,
	)
	if overview.TotalAmount != 150 {
		t.Fatalf("total_amount=%d, want 150", overview.TotalAmount)
	}
	if overview.UsedAmount != 45 {
		t.Fatalf("used_amount=%d, want 45", overview.UsedAmount)
	}
	if overview.RemainingAmount != 105 {
		t.Fatalf("remaining_amount=%d, want 105", overview.RemainingAmount)
	}
	if overview.Balance.AvailableTodayAmount != 50 {
		t.Fatalf("available_today_amount=%d, want 50", overview.Balance.AvailableTodayAmount)
	}
}

func TestBuildAdminTopUpBalanceLotListItemsWithSources(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.UserBalanceLot{}, &model.TopupOrder{}, &model.Redemption{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Create(&model.TopupOrder{
		Id:         "order-1",
		Status:     model.TopupOrderStatusFulfilled,
		Title:      "Starter credits",
		Amount:     12.5,
		Currency:   "CNY",
		Quota:      1250,
		RedeemedAt: 100,
		CreatedAt:  90,
	}).Error; err != nil {
		t.Fatalf("create topup order: %v", err)
	}
	if err := db.Create(&model.Redemption{
		Id:              "redemption-1",
		Status:          model.RedemptionCodeStatusUsed,
		Name:            "Gift code",
		FaceValueAmount: 8,
		FaceValueUnit:   model.RedemptionFaceValueUnitYYC,
		Quota:           800,
		RedeemedTime:    110,
		CreatedTime:     95,
	}).Error; err != nil {
		t.Fatalf("create redemption: %v", err)
	}
	lots := []model.UserBalanceLot{
		{
			Id:         "lot-topup",
			SourceType: model.UserBalanceLotSourceTopup,
			SourceID:   "order-1",
		},
		{
			Id:         "lot-redemption",
			SourceType: model.UserBalanceLotSourceRedeem,
			SourceID:   "redemption-1",
		},
	}
	items, err := buildAdminTopUpBalanceLotListItemsWithSources(db, lots)
	if err != nil {
		t.Fatalf("build list items: %v", err)
	}
	if len(items) != len(lots) {
		t.Fatalf("items len=%d, want %d", len(items), len(lots))
	}
	if items[0].SourceDetail == nil {
		t.Fatalf("topup source detail missing")
	}
	if items[0].SourceDetail.DetailPath != "/admin/entitlement/topup/records/order-1" {
		t.Fatalf("topup detail path=%q", items[0].SourceDetail.DetailPath)
	}
	if items[0].SourceDetail.Title != "Starter credits" {
		t.Fatalf("topup title=%q", items[0].SourceDetail.Title)
	}
	if items[1].SourceDetail == nil {
		t.Fatalf("redemption source detail missing")
	}
	if items[1].SourceDetail.DetailPath != "/admin/redemption/records/redemption-1" {
		t.Fatalf("redemption detail path=%q", items[1].SourceDetail.DetailPath)
	}
	if items[1].SourceDetail.Title != "Gift code" {
		t.Fatalf("redemption title=%q", items[1].SourceDetail.Title)
	}
}

func TestNormalizeBatchGrantUserIDsDedupeAndTrim(t *testing.T) {
	got, err := normalizeBatchGrantUserIDs([]string{" user-1 ", "", "user-2", "user-1"}, 10)
	if err != nil {
		t.Fatalf("normalize user ids: %v", err)
	}
	want := []string{"user-1", "user-2"}
	if len(got) != len(want) {
		t.Fatalf("ids len=%d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeBatchGrantUserIDsRejectsEmpty(t *testing.T) {
	if _, err := normalizeBatchGrantUserIDs([]string{" ", ""}, 10); err == nil {
		t.Fatalf("empty ids accepted, want error")
	}
}

func TestNormalizeBatchGrantUserIDsRejectsOverLimit(t *testing.T) {
	if _, err := normalizeBatchGrantUserIDs([]string{"user-1", "user-2", "user-3"}, 2); err == nil {
		t.Fatalf("over limit ids accepted, want error")
	}
}

func TestBuildActiveUserPackageSubscriptionViewIncludesSupportedModels(t *testing.T) {
	originalCacheGetGroupModels := cacheGetGroupModelsFn
	originalGetGroupCatalogByID := getGroupCatalogByIDFn
	cacheGetGroupModelsFn = func(_ context.Context, groupID string) ([]string, error) {
		if groupID != "group-1" {
			t.Fatalf("groupID=%q, want group-1", groupID)
		}
		return []string{"glm-5.2", "glm-image"}, nil
	}
	getGroupCatalogByIDFn = func(groupID string) (model.GroupCatalog, error) {
		return model.GroupCatalog{Id: groupID, Name: "Group 1"}, nil
	}
	t.Cleanup(func() {
		cacheGetGroupModelsFn = originalCacheGetGroupModels
		getGroupCatalogByIDFn = originalGetGroupCatalogByID
	})

	view, err := buildActiveUserPackageSubscriptionView(model.UserPackageSubscription{
		Id:          "subscription-1",
		UserID:      "user-1",
		PackageID:   "package-1",
		PackageName: "Starter",
		GroupID:     "group-1",
		Status:      model.UserPackageSubscriptionStatusActive,
	})
	if err != nil {
		t.Fatalf("buildActiveUserPackageSubscriptionView: %v", err)
	}
	if len(view.SupportedModels) != 2 {
		t.Fatalf("supported_models len=%d, want 2", len(view.SupportedModels))
	}
	if view.SupportedModels[0] != "glm-5.2" || view.SupportedModels[1] != "glm-image" {
		t.Fatalf("supported_models=%#v, want [glm-5.2 glm-image]", view.SupportedModels)
	}
}

func TestBuildUserBalanceQuotaCardsClassifiesCreditSources(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.UserBalanceLot{}, &model.TopupOrder{}, &model.Redemption{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Create([]model.TopupOrder{
		{
			Id:            "order-paid",
			TransactionID: "transaction-paid",
			Title:         "充值 100",
			Status:        model.TopupOrderStatusFulfilled,
			CreditOrigin:  model.TopupOrderCreditOriginPaid,
		},
		{
			Id:            "order-gift",
			TransactionID: "transaction-gift",
			Title:         "新用户奖励",
			Status:        model.TopupOrderStatusFulfilled,
			CreditOrigin:  model.TopupOrderCreditOriginNewUser,
		},
	}).Error; err != nil {
		t.Fatalf("create topup orders: %v", err)
	}
	if err := db.Create(&model.Redemption{
		Id:           "redemption-1",
		Name:         "兑换码奖励",
		Status:       model.RedemptionCodeStatusUsed,
		RedeemedTime: 100,
	}).Error; err != nil {
		t.Fatalf("create redemption: %v", err)
	}

	cards, err := buildUserBalanceQuotaCards(db, []model.UserBalanceLot{
		{
			Id:              "lot-paid",
			SourceType:      model.UserBalanceLotSourceTopup,
			SourceID:        "order-paid",
			TotalAmount:     100,
			RemainingAmount: 80,
			Status:          model.UserBalanceLotStatusActive,
		},
		{
			Id:              "lot-gift",
			SourceType:      model.UserBalanceLotSourceTopup,
			SourceID:        "order-gift",
			TotalAmount:     50,
			RemainingAmount: 50,
			Status:          model.UserBalanceLotStatusActive,
		},
		{
			Id:              "lot-redemption",
			SourceType:      model.UserBalanceLotSourceRedeem,
			SourceID:        "redemption-1",
			TotalAmount:     30,
			RemainingAmount: 30,
			Status:          model.UserBalanceLotStatusActive,
		},
	})
	if err != nil {
		t.Fatalf("buildUserBalanceQuotaCards: %v", err)
	}
	if len(cards) != 3 {
		t.Fatalf("cards len=%d, want 3", len(cards))
	}
	if cards[0].Kind != userQuotaCardKindTopup || cards[0].Name != "充值 100" {
		t.Fatalf("paid card=%#v, want topup/充值 100", cards[0])
	}
	if cards[1].Kind != userQuotaCardKindGift || cards[1].Name != "新用户奖励" {
		t.Fatalf("gift card=%#v, want gift/新用户奖励", cards[1])
	}
	if cards[2].Kind != userQuotaCardKindRedemption || cards[2].Name != "兑换码奖励" {
		t.Fatalf("redemption card=%#v, want redemption/兑换码奖励", cards[2])
	}
	for _, card := range cards {
		if card.BalanceLot == nil || card.BalanceLot.SourceDetail == nil {
			t.Fatalf("card %q source detail missing", card.ID)
		}
		if card.BalanceLot.SourceDetail.DetailPath != "" {
			t.Fatalf("card %q leaked admin detail path %q", card.ID, card.BalanceLot.SourceDetail.DetailPath)
		}
	}
}

func TestLoadUserQuotaCardsFiltersSortsPaginatesAndChecksOwnership(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.UserPackageSubscription{},
		&model.UserBalanceLot{},
		&model.TopupOrder{},
		&model.Redemption{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
	})

	now := helper.GetTimestamp()
	if err := db.Create([]model.UserPackageSubscription{
		{
			Id:              "package-active",
			UserID:          "user-1",
			PackageName:     "有效套餐",
			QuotaMetric:     model.ServicePackageQuotaMetricYYC,
			DailyQuotaLimit: 400,
			StartedAt:       now - 100,
			ExpiresAt:       now + 3600,
			Status:          model.UserPackageSubscriptionStatusActive,
			UpdatedAt:       now - 100,
		},
		{
			Id:              "package-expired",
			UserID:          "user-1",
			PackageName:     "历史套餐",
			QuotaMetric:     model.ServicePackageQuotaMetricYYC,
			DailyQuotaLimit: 100,
			StartedAt:       now - 400,
			ExpiresAt:       now - 10,
			Status:          model.UserPackageSubscriptionStatusExpired,
			UpdatedAt:       now - 10,
		},
		{
			Id:              "package-canceled",
			UserID:          "user-1",
			PackageName:     "已取消套餐",
			QuotaMetric:     model.ServicePackageQuotaMetricYYC,
			DailyQuotaLimit: 500,
			StartedAt:       now - 50,
			ExpiresAt:       now + 3600,
			Status:          model.UserPackageSubscriptionStatusCanceled,
			UpdatedAt:       now - 20,
		},
		{
			Id:              "package-other",
			UserID:          "user-2",
			PackageName:     "其他用户套餐",
			QuotaMetric:     model.ServicePackageQuotaMetricYYC,
			DailyQuotaLimit: 999,
			StartedAt:       now - 50,
			ExpiresAt:       now + 3600,
			Status:          model.UserPackageSubscriptionStatusActive,
			UpdatedAt:       now - 50,
		},
	}).Error; err != nil {
		t.Fatalf("create package subscriptions: %v", err)
	}
	if err := db.Create([]model.TopupOrder{
		{
			Id:            "order-active",
			TransactionID: "transaction-active",
			Title:         "有效充值",
			Status:        model.TopupOrderStatusFulfilled,
			CreditOrigin:  model.TopupOrderCreditOriginPaid,
		},
		{
			Id:            "order-other",
			TransactionID: "transaction-other",
			Title:         "其他用户充值",
			Status:        model.TopupOrderStatusFulfilled,
			CreditOrigin:  model.TopupOrderCreditOriginPaid,
		},
		{
			Id:            "order-gift",
			TransactionID: "transaction-gift-history",
			Title:         "注册奖励",
			Status:        model.TopupOrderStatusFulfilled,
			CreditOrigin:  model.TopupOrderCreditOriginNewUser,
		},
	}).Error; err != nil {
		t.Fatalf("create topup orders: %v", err)
	}
	if err := db.Create(&model.Redemption{
		Id:           "redemption-expired",
		Name:         "历史兑换",
		Status:       model.RedemptionCodeStatusUsed,
		RedeemedTime: now - 300,
	}).Error; err != nil {
		t.Fatalf("create redemption: %v", err)
	}
	if err := db.Create([]model.UserBalanceLot{
		{
			Id:              "lot-active",
			UserID:          "user-1",
			SourceType:      model.UserBalanceLotSourceTopup,
			SourceID:        "order-active",
			TotalAmount:     300,
			RemainingAmount: 250,
			UsedAmount:      50,
			Status:          model.UserBalanceLotStatusActive,
			GrantedAt:       now - 200,
			ExpiresAt:       now + 3600,
			CreatedAt:       now - 200,
			UpdatedAt:       now - 200,
		},
		{
			Id:              "lot-expired",
			UserID:          "user-1",
			SourceType:      model.UserBalanceLotSourceRedeem,
			SourceID:        "redemption-expired",
			TotalAmount:     200,
			RemainingAmount: 0,
			UsedAmount:      200,
			Status:          model.UserBalanceLotStatusExpired,
			GrantedAt:       now - 300,
			ExpiresAt:       now - 20,
			ExpiredAt:       now - 20,
			CreatedAt:       now - 300,
			UpdatedAt:       now - 20,
		},
		{
			Id:              "lot-gift",
			UserID:          "user-1",
			SourceType:      model.UserBalanceLotSourceTopup,
			SourceID:        "order-gift",
			TotalAmount:     100,
			RemainingAmount: 100,
			Status:          model.UserBalanceLotStatusActive,
			GrantedAt:       now - 250,
			ExpiresAt:       now + 3600,
			CreatedAt:       now - 250,
			UpdatedAt:       now - 250,
		},
		{
			Id:              "lot-other",
			UserID:          "user-2",
			SourceType:      model.UserBalanceLotSourceTopup,
			SourceID:        "order-other",
			TotalAmount:     999,
			RemainingAmount: 999,
			Status:          model.UserBalanceLotStatusActive,
			GrantedAt:       now - 50,
			ExpiresAt:       now + 3600,
			CreatedAt:       now - 50,
			UpdatedAt:       now - 50,
		},
	}).Error; err != nil {
		t.Fatalf("create balance lots: %v", err)
	}

	active, err := loadUserQuotaCards("user-1", true, "all", 1, 20)
	if err != nil {
		t.Fatalf("load active cards: %v", err)
	}
	if active.Total != 3 || len(active.Items) != 3 {
		t.Fatalf("active total/items=%d/%d, want 3/3", active.Total, len(active.Items))
	}
	if active.Items[0].ID != "package-active" || active.Items[1].ID != "lot-active" || active.Items[2].ID != "lot-gift" {
		t.Fatalf(
			"active order=%q,%q,%q, want package-active,lot-active,lot-gift",
			active.Items[0].ID,
			active.Items[1].ID,
			active.Items[2].ID,
		)
	}

	firstPage, err := loadUserQuotaCards("user-1", false, "all", 1, 2)
	if err != nil {
		t.Fatalf("load history first page: %v", err)
	}
	if firstPage.Total != 5 || len(firstPage.Items) != 2 {
		t.Fatalf("history first total/items=%d/%d, want 5/2", firstPage.Total, len(firstPage.Items))
	}
	if firstPage.Items[0].ID != "package-active" || firstPage.Items[1].ID != "lot-active" {
		t.Fatalf("history first order=%q,%q, want package-active,lot-active", firstPage.Items[0].ID, firstPage.Items[1].ID)
	}

	secondPage, err := loadUserQuotaCards("user-1", false, "all", 2, 2)
	if err != nil {
		t.Fatalf("load history second page: %v", err)
	}
	if len(secondPage.Items) != 2 {
		t.Fatalf("history second items=%d, want 2", len(secondPage.Items))
	}
	if secondPage.Items[0].ID != "lot-gift" || secondPage.Items[1].ID != "lot-expired" {
		t.Fatalf("history second order=%q,%q, want lot-gift,lot-expired", secondPage.Items[0].ID, secondPage.Items[1].ID)
	}

	thirdPage, err := loadUserQuotaCards("user-1", false, "all", 3, 2)
	if err != nil {
		t.Fatalf("load history third page: %v", err)
	}
	if len(thirdPage.Items) != 1 || thirdPage.Items[0].ID != "package-expired" {
		t.Fatalf("history third items=%#v, want package-expired", thirdPage.Items)
	}

	for kind, wantID := range map[string]string{
		userQuotaCardKindTopup:      "lot-active",
		userQuotaCardKindRedemption: "lot-expired",
		userQuotaCardKindGift:       "lot-gift",
	} {
		filtered, err := loadUserQuotaCards("user-1", false, kind, 1, 20)
		if err != nil {
			t.Fatalf("load %s cards: %v", kind, err)
		}
		if filtered.Total != 1 || len(filtered.Items) != 1 || filtered.Items[0].ID != wantID {
			t.Fatalf("%s cards total/items=%d/%#v, want 1/%s", kind, filtered.Total, filtered.Items, wantID)
		}
	}

	packages, err := loadUserQuotaCards("user-1", false, userQuotaCardKindPackage, 1, 20)
	if err != nil {
		t.Fatalf("load package cards: %v", err)
	}
	if packages.Total != 2 || len(packages.Items) != 2 {
		t.Fatalf("package cards total/items=%d/%d, want 2/2", packages.Total, len(packages.Items))
	}
	for _, card := range packages.Items {
		if card.ID == "package-canceled" {
			t.Fatalf("canceled package card should not be listed")
		}
	}

	if _, err := loadUserQuotaCard("user-1", userQuotaCardKindTopup, "lot-other"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("other user's lot error=%v, want record not found", err)
	}
	if _, err := loadUserQuotaCard("user-1", userQuotaCardKindPackage, "package-other"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("other user's package error=%v, want record not found", err)
	}
	if _, err := loadUserQuotaCard("user-1", userQuotaCardKindGift, "lot-active"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("mismatched lot kind error=%v, want record not found", err)
	}
}
