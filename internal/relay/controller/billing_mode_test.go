package controller

import (
	"context"
	"testing"

	adminmodel "github.com/yeying-community/router/internal/admin/model"
	relaymeta "github.com/yeying-community/router/internal/relay/meta"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBuildBalanceRelayBillingPlanWithoutPackage(t *testing.T) {
	plan := buildBalanceRelayBillingPlan(false)
	if plan.Source != relayBillingSourceBalance {
		t.Fatalf("plan.Source = %q, want %q", plan.Source, relayBillingSourceBalance)
	}
	if !plan.ChargeUserBalance() {
		t.Fatalf("plan.ChargeUserBalance() = false, want true")
	}
}

func TestBuildBalanceRelayBillingPlanWithPackageFallback(t *testing.T) {
	plan := buildBalanceRelayBillingPlan(true)
	if plan.Source != relayBillingSourcePackageFallbackBalance {
		t.Fatalf("plan.Source = %q, want %q", plan.Source, relayBillingSourcePackageFallbackBalance)
	}
	if !plan.ChargeUserBalance() {
		t.Fatalf("plan.ChargeUserBalance() = false, want true")
	}
}

func TestRelayBillingPlanWithRequestPackageDoesNotChargeTokenQuota(t *testing.T) {
	plan := relayBillingPlan{
		Source: relayBillingSourcePackage,
		RequestPackageReservation: adminmodel.RequestPackageReservation{
			CounterID:      "counter-1",
			SubscriptionID: "subscription-1",
			ReservedAmount: 1,
		},
	}
	if plan.ChargeUserBalance() {
		t.Fatalf("plan.ChargeUserBalance() = true, want false")
	}
	if !plan.UsesRequestPackage() {
		t.Fatalf("plan.UsesRequestPackage() = false, want true")
	}
	if plan.ChargeTokenQuota() {
		t.Fatalf("plan.ChargeTokenQuota() = true, want false")
	}
}

func TestTryBuildRequestPackageBillingPlanMatchesGroupBeforePricing(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&adminmodel.User{},
		&adminmodel.GroupCatalog{},
		&adminmodel.ServicePackage{},
		&adminmodel.ServicePackageVisibleUser{},
		&adminmodel.UserPackageSubscription{},
		&adminmodel.UserPackageUsageCounter{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	previousDB := adminmodel.DB
	adminmodel.DB = db
	t.Cleanup(func() {
		adminmodel.DB = previousDB
	})
	if err := db.Create(&adminmodel.GroupCatalog{Id: "group-1", Name: "request package group", Enabled: true}).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if err := db.Create(&adminmodel.User{Id: "user-1", Username: "user1", Password: "password123", Status: adminmodel.UserStatusEnabled}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	servicePackage, err := adminmodel.CreateServicePackage(adminmodel.ServicePackage{
		Name:        "request monthly",
		GroupID:     "group-1",
		PackageType: adminmodel.ServicePackageTypeRequestQuota,
		QuotaMetric: adminmodel.ServicePackageQuotaMetricRequestCount,
		PeriodLimit: 25000,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("CreateServicePackage returned error: %v", err)
	}
	if _, err := adminmodel.AssignServicePackageToUser(servicePackage.Id, "user-1", 0); err != nil {
		t.Fatalf("AssignServicePackageToUser returned error: %v", err)
	}

	plan, matched, relayErr := tryBuildRequestPackageBillingPlan(context.Background(), &relaymeta.Meta{
		UserId: "user-1",
		Group:  "group-1",
	})
	if relayErr != nil {
		t.Fatalf("tryBuildRequestPackageBillingPlan returned error: %+v", relayErr)
	}
	if !matched {
		t.Fatalf("matched=false, want true")
	}
	if !plan.UsesRequestPackage() || plan.ChargeTokenQuota() || plan.ChargeUserBalance() {
		t.Fatalf("request package plan mismatch: %+v", plan)
	}
	if !plan.RequestPackageReservation.Active() {
		t.Fatalf("request package reservation not active: %+v", plan.RequestPackageReservation)
	}
}

func TestTryBuildRequestPackageBillingPlanWithAmountReservesRequestedCount(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&adminmodel.User{},
		&adminmodel.GroupCatalog{},
		&adminmodel.ServicePackage{},
		&adminmodel.ServicePackageVisibleUser{},
		&adminmodel.UserPackageSubscription{},
		&adminmodel.UserPackageUsageCounter{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	previousDB := adminmodel.DB
	adminmodel.DB = db
	t.Cleanup(func() {
		adminmodel.DB = previousDB
	})
	if err := db.Create(&adminmodel.GroupCatalog{Id: "group-1", Name: "request package group", Enabled: true}).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if err := db.Create(&adminmodel.User{Id: "user-1", Username: "user1", Password: "password123", Status: adminmodel.UserStatusEnabled}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	servicePackage, err := adminmodel.CreateServicePackage(adminmodel.ServicePackage{
		Name:        "request monthly",
		GroupID:     "group-1",
		PackageType: adminmodel.ServicePackageTypeRequestQuota,
		QuotaMetric: adminmodel.ServicePackageQuotaMetricRequestCount,
		PeriodLimit: 25000,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("CreateServicePackage returned error: %v", err)
	}
	if _, err := adminmodel.AssignServicePackageToUser(servicePackage.Id, "user-1", 0); err != nil {
		t.Fatalf("AssignServicePackageToUser returned error: %v", err)
	}

	plan, matched, relayErr := tryBuildRequestPackageBillingPlanWithAmount(context.Background(), &relaymeta.Meta{
		UserId: "user-1",
		Group:  "group-1",
	}, 4)
	if relayErr != nil {
		t.Fatalf("tryBuildRequestPackageBillingPlanWithAmount returned error: %+v", relayErr)
	}
	if !matched {
		t.Fatalf("matched=false, want true")
	}
	if plan.RequestPackageReservation.ReservedAmount != 4 {
		t.Fatalf("reserved_amount=%d, want 4", plan.RequestPackageReservation.ReservedAmount)
	}
}
