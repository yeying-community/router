package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSyncEntitlementProductsFromLegacyBackfillsTopupPlansAndServicePackages(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&User{},
		&GroupCatalog{},
		&GroupModel{},
		&TopupPlan{},
		&TopupPlanVisibleUser{},
		&ServicePackage{},
		&ServicePackageVisibleUser{},
		&UserPackageSubscription{},
		&EntitlementProduct{},
		&EntitlementProductVisibleUser{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Create(&GroupCatalog{Id: "group-1", Name: "default", Enabled: true}).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if err := db.Create(&TopupPlan{
		Id:             "topup-1",
		Name:           "100 CNY",
		GroupID:        "group-1",
		Amount:         100,
		AmountCurrency: BillingCurrencyCodeCNY,
		QuotaAmount:    2600,
		QuotaCurrency:  BillingCurrencyCodeYYC,
		ValidityDays:   90,
		Enabled:        true,
		PublicVisible:  true,
		SortOrder:      2,
		CreatedAt:      10,
		UpdatedAt:      20,
	}).Error; err != nil {
		t.Fatalf("seed topup plan: %v", err)
	}
	if err := db.Create(&ServicePackage{
		Id:                         "package-1",
		Name:                       "starter",
		Description:                "starter package",
		GroupID:                    "group-1",
		PackageType:                ServicePackageTypeYYCQuota,
		QuotaMetric:                ServicePackageQuotaMetricYYC,
		PeriodType:                 ServicePackagePeriodDaily,
		PeriodLimit:                1000,
		SalePrice:                  19.9,
		SaleCurrency:               BillingCurrencyCodeCNY,
		DurationDays:               30,
		VisibilityScope:            ServicePackageVisibilityScopeUser,
		PackageEmergencyQuotaLimit: 200,
		AllowBalanceFallback:       true,
		Enabled:                    true,
		SortOrder:                  3,
		CreatedAt:                  30,
		UpdatedAt:                  40,
	}).Error; err != nil {
		t.Fatalf("seed service package: %v", err)
	}
	if err := db.Create(&ServicePackageVisibleUser{
		PackageID: "package-1",
		UserID:    "user-1",
		CreatedAt: 50,
	}).Error; err != nil {
		t.Fatalf("seed visible user: %v", err)
	}

	if err := syncEntitlementProductsFromLegacyWithDB(db); err != nil {
		t.Fatalf("syncEntitlementProductsFromLegacyWithDB: %v", err)
	}

	products := []EntitlementProduct{}
	if err := db.Order("kind asc").Find(&products).Error; err != nil {
		t.Fatalf("list products: %v", err)
	}
	if len(products) != 2 {
		t.Fatalf("len(products)=%d, want 2", len(products))
	}
	byID := map[string]EntitlementProduct{}
	for _, product := range products {
		byID[product.Id] = product
	}
	balance := byID["topup-1"]
	if balance.Kind != EntitlementProductKindBalance ||
		balance.Id != "topup-1" ||
		balance.SalePrice != 100 ||
		balance.QuotaAmount != 2600 ||
		balance.DurationDays != 90 ||
		!balance.PublicVisible {
		t.Fatalf("balance product=%#v", balance)
	}
	subscription := byID["package-1"]
	if subscription.Kind != EntitlementProductKindSubscription ||
		subscription.Id != "package-1" ||
		subscription.SalePrice != 19.9 ||
		subscription.PeriodType != ServicePackagePeriodDaily ||
		subscription.PeriodLimit != 1000 ||
		subscription.VisibilityScope != ServicePackageVisibilityScopeUser ||
		subscription.PublicVisible {
		t.Fatalf("subscription product=%#v", subscription)
	}
	visibleCount := int64(0)
	if err := db.Model(&EntitlementProductVisibleUser{}).
		Where("product_id = ? AND user_id = ?", subscription.Id, "user-1").
		Count(&visibleCount).Error; err != nil {
		t.Fatalf("count product visible user: %v", err)
	}
	if visibleCount != 1 {
		t.Fatalf("visibleCount=%d, want 1", visibleCount)
	}

	if err := syncEntitlementProductsFromLegacyWithDB(db); err != nil {
		t.Fatalf("second syncEntitlementProductsFromLegacyWithDB: %v", err)
	}
	productCount := int64(0)
	if err := db.Model(&EntitlementProduct{}).Count(&productCount).Error; err != nil {
		t.Fatalf("count products: %v", err)
	}
	if productCount != 2 {
		t.Fatalf("productCount=%d, want 2 after idempotent sync", productCount)
	}
}

func TestLegacyTopupPlanCRUDSyncsEntitlementProduct(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&User{},
		&GroupCatalog{},
		&GroupModel{},
		&TopupPlan{},
		&TopupPlanVisibleUser{},
		&EntitlementProduct{},
		&EntitlementProductVisibleUser{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Create(&GroupCatalog{Id: "group-1", Name: "default", Enabled: true}).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}

	plan, err := createTopupPlanWithDB(db, TopupPlan{
		Name:           "10 CNY",
		GroupID:        "group-1",
		Amount:         10,
		AmountCurrency: BillingCurrencyCodeCNY,
		QuotaAmount:    220,
		QuotaCurrency:  BillingCurrencyCodeYYC,
		Enabled:        true,
		PublicVisible:  true,
	})
	if err != nil {
		t.Fatalf("createTopupPlanWithDB: %v", err)
	}
	product := EntitlementProduct{}
	if err := db.First(&product, "id = ?", plan.Id).Error; err != nil {
		t.Fatalf("load product after create: %v", err)
	}
	if product.Id != plan.Id || product.Kind != EntitlementProductKindBalance || product.SalePrice != 10 || product.QuotaAmount != 220 {
		t.Fatalf("created product=%#v", product)
	}

	plan.Amount = 20
	plan.QuotaAmount = 500
	updated, err := updateTopupPlanWithDB(db, plan)
	if err != nil {
		t.Fatalf("updateTopupPlanWithDB: %v", err)
	}
	if err := db.First(&product, "id = ?", updated.Id).Error; err != nil {
		t.Fatalf("load product after update: %v", err)
	}
	if product.SalePrice != 20 || product.QuotaAmount != 500 {
		t.Fatalf("updated product=%#v", product)
	}

	if err := deleteTopupPlanWithDB(db, updated.Id); err != nil {
		t.Fatalf("deleteTopupPlanWithDB: %v", err)
	}
	count := int64(0)
	if err := db.Model(&EntitlementProduct{}).
		Where("id = ?", updated.Id).
		Count(&count).Error; err != nil {
		t.Fatalf("count product after delete: %v", err)
	}
	if count != 0 {
		t.Fatalf("product count after delete=%d, want 0", count)
	}
}

func TestLegacyServicePackageCRUDSyncsEntitlementProduct(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&User{},
		&GroupCatalog{},
		&GroupModel{},
		&ServicePackage{},
		&ServicePackageVisibleUser{},
		&UserPackageSubscription{},
		&EntitlementProduct{},
		&EntitlementProductVisibleUser{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Create(&GroupCatalog{Id: "group-1", Name: "default", Enabled: true}).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if err := db.Create(&User{Id: "user-1", Username: "alice", Status: UserStatusEnabled}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	pkg, err := createServicePackageWithDB(db, ServicePackage{
		Name:            "starter",
		GroupID:         "group-1",
		PeriodType:      ServicePackagePeriodDaily,
		PeriodLimit:     1000,
		SalePrice:       19.9,
		SaleCurrency:    BillingCurrencyCodeCNY,
		DurationDays:    30,
		VisibilityScope: ServicePackageVisibilityScopeUser,
		VisibleUserIDs:  []string{"user-1"},
		Enabled:         true,
	})
	if err != nil {
		t.Fatalf("createServicePackageWithDB: %v", err)
	}
	product := EntitlementProduct{}
	if err := db.First(&product, "id = ?", pkg.Id).Error; err != nil {
		t.Fatalf("load product after create: %v", err)
	}
	if product.Id != pkg.Id || product.Kind != EntitlementProductKindSubscription || product.SalePrice != 19.9 || product.PeriodLimit != 1000 || product.PublicVisible {
		t.Fatalf("created product=%#v", product)
	}
	visibleCount := int64(0)
	if err := db.Model(&EntitlementProductVisibleUser{}).Where("product_id = ? AND user_id = ?", product.Id, "user-1").Count(&visibleCount).Error; err != nil {
		t.Fatalf("count visible user after create: %v", err)
	}
	if visibleCount != 1 {
		t.Fatalf("visibleCount=%d, want 1", visibleCount)
	}

	pkg.PeriodLimit = 2000
	pkg.SalePrice = 29.9
	pkg.VisibilityScope = ServicePackageVisibilityScopeAll
	pkg.VisibleUserIDs = nil
	updated, err := updateServicePackageWithDB(db, pkg)
	if err != nil {
		t.Fatalf("updateServicePackageWithDB: %v", err)
	}
	if err := db.First(&product, "id = ?", updated.Id).Error; err != nil {
		t.Fatalf("load product after update: %v", err)
	}
	if product.SalePrice != 29.9 || product.PeriodLimit != 2000 || !product.PublicVisible {
		t.Fatalf("updated product=%#v", product)
	}
	if err := db.Model(&EntitlementProductVisibleUser{}).Where("product_id = ?", product.Id).Count(&visibleCount).Error; err != nil {
		t.Fatalf("count visible user after update: %v", err)
	}
	if visibleCount != 0 {
		t.Fatalf("visibleCount after update=%d, want 0", visibleCount)
	}

	if err := deleteServicePackageWithDB(db, updated.Id); err != nil {
		t.Fatalf("deleteServicePackageWithDB: %v", err)
	}
	productCount := int64(0)
	if err := db.Model(&EntitlementProduct{}).
		Where("id = ?", updated.Id).
		Count(&productCount).Error; err != nil {
		t.Fatalf("count product after delete: %v", err)
	}
	if productCount != 0 {
		t.Fatalf("productCount after delete=%d, want 0", productCount)
	}
}

func TestEntitlementProductCRUDDelegatesToLegacyTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&User{},
		&GroupCatalog{},
		&GroupModel{},
		&TopupPlan{},
		&TopupPlanVisibleUser{},
		&ServicePackage{},
		&ServicePackageVisibleUser{},
		&UserPackageSubscription{},
		&EntitlementProduct{},
		&EntitlementProductVisibleUser{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Create(&GroupCatalog{Id: "group-1", Name: "default", Enabled: true}).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}

	balance, err := CreateEntitlementProductWithDB(db, EntitlementProduct{
		Kind:          EntitlementProductKindBalance,
		Name:          "balance product",
		GroupID:       "group-1",
		SalePrice:     10,
		SaleCurrency:  BillingCurrencyCodeCNY,
		QuotaAmount:   200,
		QuotaCurrency: BillingCurrencyCodeYYC,
		ValidityDays:  7,
		PublicVisible: true,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateEntitlementProductWithDB balance: %v", err)
	}
	if balance.Kind != EntitlementProductKindBalance || balance.Id == "" {
		t.Fatalf("balance product=%#v", balance)
	}
	legacyTopup := TopupPlan{}
	if err := db.First(&legacyTopup, "id = ?", balance.Id).Error; err != nil {
		t.Fatalf("load legacy topup: %v", err)
	}
	if legacyTopup.Id != balance.Id {
		t.Fatalf("legacy topup id=%q, want product id %q", legacyTopup.Id, balance.Id)
	}
	if legacyTopup.Amount != 10 || legacyTopup.QuotaAmount != 200 || legacyTopup.ValidityDays != 7 {
		t.Fatalf("legacyTopup=%#v", legacyTopup)
	}

	balance.Name = "balance product updated"
	balance.SalePrice = 20
	balance.QuotaAmount = 500
	updatedBalance, err := UpdateEntitlementProductWithDB(db, balance)
	if err != nil {
		t.Fatalf("UpdateEntitlementProductWithDB balance: %v", err)
	}
	if updatedBalance.SalePrice != 20 || updatedBalance.QuotaAmount != 500 {
		t.Fatalf("updatedBalance=%#v", updatedBalance)
	}
	if err := db.First(&legacyTopup, "id = ?", balance.Id).Error; err != nil {
		t.Fatalf("reload legacy topup: %v", err)
	}
	if legacyTopup.Amount != 20 || legacyTopup.QuotaAmount != 500 {
		t.Fatalf("updated legacyTopup=%#v", legacyTopup)
	}
	if err := DeleteEntitlementProductWithDB(db, balance.Id); err != nil {
		t.Fatalf("DeleteEntitlementProductWithDB balance: %v", err)
	}
	topupCount := int64(0)
	if err := db.Model(&TopupPlan{}).Where("id = ?", balance.Id).Count(&topupCount).Error; err != nil {
		t.Fatalf("count topup: %v", err)
	}
	if topupCount != 0 {
		t.Fatalf("topupCount=%d, want 0", topupCount)
	}

	subscription, err := CreateEntitlementProductWithDB(db, EntitlementProduct{
		Kind:          EntitlementProductKindSubscription,
		Name:          "subscription product",
		GroupID:       "group-1",
		SalePrice:     29.9,
		SaleCurrency:  BillingCurrencyCodeCNY,
		QuotaMetric:   ServicePackageQuotaMetricYYC,
		QuotaAmount:   1000,
		QuotaCurrency: BillingCurrencyCodeYYC,
		PeriodType:    ServicePackagePeriodMonthly,
		PeriodLimit:   1000,
		DurationDays:  30,
		PublicVisible: true,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateEntitlementProductWithDB subscription: %v", err)
	}
	if subscription.Kind != EntitlementProductKindSubscription || subscription.Id == "" {
		t.Fatalf("subscription product=%#v", subscription)
	}
	legacyPackage := ServicePackage{}
	if err := db.First(&legacyPackage, "id = ?", subscription.Id).Error; err != nil {
		t.Fatalf("load legacy package: %v", err)
	}
	if legacyPackage.Id != subscription.Id {
		t.Fatalf("legacy package id=%q, want product id %q", legacyPackage.Id, subscription.Id)
	}
	if legacyPackage.SalePrice != 29.9 || legacyPackage.PeriodLimit != 1000 || legacyPackage.DurationDays != 30 {
		t.Fatalf("legacyPackage=%#v", legacyPackage)
	}

	subscription.PeriodLimit = 2000
	subscription.QuotaAmount = 2000
	subscription.SalePrice = 39.9
	updatedSubscription, err := UpdateEntitlementProductWithDB(db, subscription)
	if err != nil {
		t.Fatalf("UpdateEntitlementProductWithDB subscription: %v", err)
	}
	if updatedSubscription.SalePrice != 39.9 || updatedSubscription.PeriodLimit != 2000 {
		t.Fatalf("updatedSubscription=%#v", updatedSubscription)
	}
	if err := DeleteEntitlementProductWithDB(db, subscription.Id); err != nil {
		t.Fatalf("DeleteEntitlementProductWithDB subscription: %v", err)
	}
	packageCount := int64(0)
	if err := db.Model(&ServicePackage{}).Where("id = ?", subscription.Id).Count(&packageCount).Error; err != nil {
		t.Fatalf("count package: %v", err)
	}
	if packageCount != 0 {
		t.Fatalf("packageCount=%d, want 0", packageCount)
	}
}
