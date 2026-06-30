package model

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newUserEntitlementModelsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&User{},
		&GroupCatalog{},
		&GroupModel{},
		&UserPackageSubscription{},
		&TopupOrder{},
		&TopupPlan{},
		&Redemption{},
		&UserBalanceLot{},
		&UserBalanceLotTransaction{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return db
}

func seedEntitlementGroup(t *testing.T, db *gorm.DB, id string, name string, models ...string) {
	t.Helper()
	if err := db.Create(&GroupCatalog{Id: id, Name: name, Enabled: true}).Error; err != nil {
		t.Fatalf("create group %s: %v", id, err)
	}
	for _, modelName := range models {
		if err := db.Create(&GroupModel{
			Group:   id,
			Model:   modelName,
			Enabled: true,
		}).Error; err != nil {
			t.Fatalf("create group model %s/%s: %v", id, modelName, err)
		}
	}
}

func TestBuildUserEntitlementModelsMergesPackageTopupRedemptionAndLegacy(t *testing.T) {
	db := newUserEntitlementModelsTestDB(t)
	seedEntitlementGroup(t, db, "group-package", "Package Group", "glm-5.2", "shared-model")
	seedEntitlementGroup(t, db, "group-topup", "Topup Group", "qwen-plus", "shared-model")
	seedEntitlementGroup(t, db, "group-redeem", "Redeem Group", "deepseek-chat")
	seedEntitlementGroup(t, db, "group-legacy", "Legacy Group", "legacy-model")
	now := int64(1000)
	if err := db.Create(&User{
		Id:       "user-1",
		Username: "user-1",
		Status:   UserStatusEnabled,
		Group:    "group-legacy",
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&UserPackageSubscription{
		Id:          "subscription-1",
		UserID:      "user-1",
		PackageID:   "package-1",
		PackageName: "Package One",
		GroupID:     "group-package",
		PackageType: ServicePackageTypeYYCQuota,
		QuotaMetric: ServicePackageQuotaMetricYYC,
		StartedAt:   1,
		ExpiresAt:   0,
		Status:      UserPackageSubscriptionStatusActive,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("create package subscription: %v", err)
	}
	if err := db.Create(&TopupOrder{
		Id:            "topup-order-1",
		UserID:        "user-1",
		Status:        TopupOrderStatusFulfilled,
		BusinessType:  TopupOrderBusinessBalance,
		Title:         "Topup One",
		TransactionID: "txn-topup-1",
		TopupPlanID:   "topup-plan-1",
		GroupID:       "group-topup",
		CreatedAt:     now,
	}).Error; err != nil {
		t.Fatalf("create topup order: %v", err)
	}
	if _, _, err := CreditUserBalanceLotWithDB(db, UserBalanceLotCreditInput{
		UserID:      "user-1",
		SourceType:  UserBalanceLotSourceTopup,
		SourceID:    "topup-order-1",
		TotalAmount: 100,
		GrantedAt:   now,
	}); err != nil {
		t.Fatalf("credit topup lot: %v", err)
	}
	if err := db.Create(&Redemption{
		Id:               "redemption-1",
		RedeemedByUserId: "user-1",
		Status:           RedemptionCodeStatusUsed,
		Name:             "Redeem One",
		GroupID:          "group-redeem",
		RedeemedTime:     now,
	}).Error; err != nil {
		t.Fatalf("create redemption: %v", err)
	}
	if _, _, err := CreditUserBalanceLotWithDB(db, UserBalanceLotCreditInput{
		UserID:      "user-1",
		SourceType:  UserBalanceLotSourceRedeem,
		SourceID:    "redemption-1",
		TotalAmount: 100,
		GrantedAt:   now,
	}); err != nil {
		t.Fatalf("credit redemption lot: %v", err)
	}

	payload, err := BuildUserEntitlementModelsWithDB(context.Background(), db, "user-1")
	if err != nil {
		t.Fatalf("BuildUserEntitlementModelsWithDB: %v", err)
	}
	wantModels := []string{"deepseek-chat", "glm-5.2", "legacy-model", "qwen-plus", "shared-model"}
	if len(payload.Models) != len(wantModels) {
		t.Fatalf("models=%#v, want %#v", payload.Models, wantModels)
	}
	for i := range wantModels {
		if payload.Models[i] != wantModels[i] {
			t.Fatalf("models=%#v, want %#v", payload.Models, wantModels)
		}
	}
	sharedSources := payload.ByModel["shared-model"]
	if len(sharedSources) != 2 {
		t.Fatalf("shared-model sources=%#v, want 2", sharedSources)
	}
	if sharedSources[0].SourceType != UserEntitlementSourcePackage || sharedSources[0].GroupID != "group-package" {
		t.Fatalf("first shared source=%#v, want package group", sharedSources[0])
	}
	if sharedSources[1].SourceType != UserEntitlementSourceTopup || sharedSources[1].GroupID != "group-topup" {
		t.Fatalf("second shared source=%#v, want topup group", sharedSources[1])
	}

	groupID, source, err := ResolveUserEntitlementGroupForModelWithDB(context.Background(), db, "user-1", "qwen-plus")
	if err != nil {
		t.Fatalf("ResolveUserEntitlementGroupForModelWithDB: %v", err)
	}
	if groupID != "group-topup" || source == nil || source.SourceType != UserEntitlementSourceTopup {
		t.Fatalf("resolved group=%q source=%#v, want topup group", groupID, source)
	}
}

func TestConsumeUserBalanceLotsForGroupOnlyConsumesMatchingGroup(t *testing.T) {
	db := newUserEntitlementModelsTestDB(t)
	seedEntitlementGroup(t, db, "group-a", "Group A")
	seedEntitlementGroup(t, db, "group-b", "Group B")
	now := int64(1000)
	orders := []TopupOrder{
		{
			Id:            "order-a",
			UserID:        "user-1",
			Status:        TopupOrderStatusFulfilled,
			BusinessType:  TopupOrderBusinessBalance,
			Title:         "A",
			TransactionID: "txn-order-a",
			GroupID:       "group-a",
			CreatedAt:     now,
		},
		{
			Id:            "order-b",
			UserID:        "user-1",
			Status:        TopupOrderStatusFulfilled,
			BusinessType:  TopupOrderBusinessBalance,
			Title:         "B",
			TransactionID: "txn-order-b",
			GroupID:       "group-b",
			CreatedAt:     now,
		},
	}
	if err := db.Create(&orders).Error; err != nil {
		t.Fatalf("create orders: %v", err)
	}
	for _, order := range orders {
		if _, _, err := CreditUserBalanceLotWithDB(db, UserBalanceLotCreditInput{
			UserID:      "user-1",
			SourceType:  UserBalanceLotSourceTopup,
			SourceID:    order.Id,
			TotalAmount: 100,
			GrantedAt:   now,
		}); err != nil {
			t.Fatalf("credit lot %s: %v", order.Id, err)
		}
	}

	consumed, err := ConsumeUserBalanceLotsForGroupWithDB(db, "user-1", "group-b", 60, now)
	if err != nil {
		t.Fatalf("ConsumeUserBalanceLotsForGroupWithDB: %v", err)
	}
	if consumed != 60 {
		t.Fatalf("consumed=%d, want 60", consumed)
	}
	lotA := UserBalanceLot{}
	if err := db.Where("source_id = ?", "order-a").Take(&lotA).Error; err != nil {
		t.Fatalf("load lot A: %v", err)
	}
	lotB := UserBalanceLot{}
	if err := db.Where("source_id = ?", "order-b").Take(&lotB).Error; err != nil {
		t.Fatalf("load lot B: %v", err)
	}
	if lotA.RemainingAmount != 100 {
		t.Fatalf("lotA remaining=%d, want 100", lotA.RemainingAmount)
	}
	if lotB.RemainingAmount != 40 {
		t.Fatalf("lotB remaining=%d, want 40", lotB.RemainingAmount)
	}
}
