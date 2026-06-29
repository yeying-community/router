package user

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
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
		{
			Id:         "lot-legacy",
			SourceType: model.UserBalanceLotSourceLegacy,
			SourceID:   "legacy-1",
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
	if items[0].SourceDetail.DetailPath != "/admin/flow/topup/order-1" {
		t.Fatalf("topup detail path=%q", items[0].SourceDetail.DetailPath)
	}
	if items[0].SourceDetail.Title != "Starter credits" {
		t.Fatalf("topup title=%q", items[0].SourceDetail.Title)
	}
	if items[1].SourceDetail == nil {
		t.Fatalf("redemption source detail missing")
	}
	if items[1].SourceDetail.DetailPath != "/admin/flow/redemption/redemption-1" {
		t.Fatalf("redemption detail path=%q", items[1].SourceDetail.DetailPath)
	}
	if items[1].SourceDetail.Title != "Gift code" {
		t.Fatalf("redemption title=%q", items[1].SourceDetail.Title)
	}
	if items[2].SourceDetail != nil {
		t.Fatalf("legacy source detail=%#v, want nil", items[2].SourceDetail)
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
