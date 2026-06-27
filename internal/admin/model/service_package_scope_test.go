package model

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newServicePackageScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&User{},
		&GroupCatalog{},
		&ServicePackage{},
		&ServicePackageVisibleUser{},
		&UserPackageSubscription{},
		&UserPackageUsageCounter{},
		&EntitlementConcurrencyCounter{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Create(&GroupCatalog{
		Id:      "group-1",
		Name:    "default",
		Enabled: true,
	}).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if err := db.Create(&User{
		Id:       "user-1",
		Username: "user1",
		Password: "password123",
		Status:   UserStatusEnabled,
	}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return db
}

func TestCreateServicePackageDefaultsLegacyYYCScopeFields(t *testing.T) {
	db := newServicePackageScopeTestDB(t)

	row, err := createServicePackageWithDB(db, ServicePackage{
		Name:            "legacy yyc",
		GroupID:         "group-1",
		DailyQuotaLimit: 1000,
		Enabled:         true,
	})
	if err != nil {
		t.Fatalf("createServicePackageWithDB returned error: %v", err)
	}

	if row.PackageType != ServicePackageTypeYYCQuota {
		t.Fatalf("package_type=%q, want %q", row.PackageType, ServicePackageTypeYYCQuota)
	}
	if row.ScopeType != ServicePackageScopeAll {
		t.Fatalf("scope_type=%q, want %q", row.ScopeType, ServicePackageScopeAll)
	}
	if row.QuotaMetric != ServicePackageQuotaMetricYYC {
		t.Fatalf("quota_metric=%q, want %q", row.QuotaMetric, ServicePackageQuotaMetricYYC)
	}
	if row.PeriodType != ServicePackagePeriodDaily {
		t.Fatalf("period_type=%q, want %q", row.PeriodType, ServicePackagePeriodDaily)
	}
	if row.PeriodLimit != 1000 {
		t.Fatalf("period_limit=%d, want 1000", row.PeriodLimit)
	}
	if !row.AllowBalanceFallback {
		t.Fatalf("allow_balance_fallback=false, want true for legacy YYC package")
	}
}

func TestCreateServicePackageUsesGroupMonthlyRequestQuota(t *testing.T) {
	db := newServicePackageScopeTestDB(t)

	row, err := createServicePackageWithDB(db, ServicePackage{
		Name:                     "glm monthly",
		GroupID:                  "group-1",
		PackageType:              ServicePackageTypeRequestQuota,
		ScopeProvider:            " ZHIPU ",
		ScopeModel:               "glm-5.2",
		QuotaMetric:              ServicePackageQuotaMetricRequestCount,
		PeriodLimit:              25000,
		MaxConcurrencyPerUser:    3,
		MaxConcurrencyPerPackage: 100,
		Enabled:                  true,
	})
	if err != nil {
		t.Fatalf("createServicePackageWithDB returned error: %v", err)
	}

	if row.ScopeType != ServicePackageScopeAll {
		t.Fatalf("scope_type=%q, want %q", row.ScopeType, ServicePackageScopeAll)
	}
	if row.ScopeProvider != "" || row.ScopeModel != "" || row.ScopeEndpoint != "" {
		t.Fatalf("scope fields=%q/%q/%q, want empty because package capabilities are defined by group", row.ScopeProvider, row.ScopeModel, row.ScopeEndpoint)
	}
	if row.PeriodType != ServicePackagePeriodMonthly {
		t.Fatalf("period_type=%q, want %q", row.PeriodType, ServicePackagePeriodMonthly)
	}
	if row.PeriodLimit != 25000 {
		t.Fatalf("period_limit=%d, want 25000", row.PeriodLimit)
	}
	if row.AllowBalanceFallback {
		t.Fatalf("allow_balance_fallback=true, want false for request quota unless explicit")
	}
}

func TestAssignServicePackageSnapshotsGroupQuotaFields(t *testing.T) {
	db := newServicePackageScopeTestDB(t)
	servicePackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:                     "glm monthly",
		GroupID:                  "group-1",
		PackageType:              ServicePackageTypeRequestQuota,
		ScopeProvider:            "zhipu",
		ScopeModel:               "glm-5.2",
		QuotaMetric:              ServicePackageQuotaMetricRequestCount,
		PeriodLimit:              25000,
		MaxConcurrencyPerUser:    3,
		MaxConcurrencyPerPackage: 100,
		Enabled:                  true,
	})
	if err != nil {
		t.Fatalf("createServicePackageWithDB returned error: %v", err)
	}

	subscription, err := AssignServicePackageToUserWithDB(db, servicePackage.Id, "user-1", 100)
	if err != nil {
		t.Fatalf("AssignServicePackageToUserWithDB returned error: %v", err)
	}

	if subscription.PackageType != ServicePackageTypeRequestQuota {
		t.Fatalf("subscription package_type=%q, want %q", subscription.PackageType, ServicePackageTypeRequestQuota)
	}
	if subscription.ScopeType != ServicePackageScopeAll {
		t.Fatalf("subscription scope_type=%q, want %q", subscription.ScopeType, ServicePackageScopeAll)
	}
	if subscription.ScopeProvider != "" || subscription.ScopeModel != "" || subscription.ScopeEndpoint != "" {
		t.Fatalf("subscription scope=%q/%q/%q, want empty because package capabilities are defined by group", subscription.ScopeProvider, subscription.ScopeModel, subscription.ScopeEndpoint)
	}
	if subscription.QuotaMetric != ServicePackageQuotaMetricRequestCount {
		t.Fatalf("subscription quota_metric=%q, want %q", subscription.QuotaMetric, ServicePackageQuotaMetricRequestCount)
	}
	if subscription.PeriodType != ServicePackagePeriodMonthly || subscription.PeriodLimit != 25000 {
		t.Fatalf("subscription period=%q/%d, want monthly/25000", subscription.PeriodType, subscription.PeriodLimit)
	}
	if subscription.MaxConcurrencyPerUser != 3 || subscription.MaxConcurrencyPerPackage != 100 {
		t.Fatalf("subscription concurrency=%d/%d, want 3/100", subscription.MaxConcurrencyPerUser, subscription.MaxConcurrencyPerPackage)
	}
}

func TestMigrateServicePackageScopeAndUsageCountersBackfillsLegacyRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ServicePackage{}, &UserPackageSubscription{}); err != nil {
		t.Fatalf("AutoMigrate legacy tables: %v", err)
	}
	if err := db.Exec(`
		UPDATE service_packages
		SET package_type = '', scope_type = '', quota_metric = '', period_type = '', period_limit = 0, allow_balance_fallback = FALSE
	`).Error; err != nil {
		t.Fatalf("prepare service package defaults: %v", err)
	}
	if err := db.Create(&ServicePackage{
		Id:              "package-1",
		Name:            "legacy",
		GroupID:         "group-1",
		DailyQuotaLimit: 1234,
	}).Error; err != nil {
		t.Fatalf("seed service package: %v", err)
	}
	if err := db.Exec(`
		UPDATE service_packages
		SET package_type = '', scope_type = '', quota_metric = '', period_type = '', period_limit = 0, allow_balance_fallback = FALSE
		WHERE id = 'package-1'
	`).Error; err != nil {
		t.Fatalf("blank service package scope fields: %v", err)
	}
	if err := db.Create(&UserPackageSubscription{
		Id:              "subscription-1",
		UserID:          "user-1",
		PackageID:       "package-1",
		PackageName:     "legacy",
		GroupID:         "group-1",
		DailyQuotaLimit: 1234,
		Status:          UserPackageSubscriptionStatusActive,
	}).Error; err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	if err := db.Exec(`
		UPDATE user_package_subscriptions
		SET package_type = '', scope_type = '', quota_metric = '', period_type = '', period_limit = 0, allow_balance_fallback = FALSE
		WHERE id = 'subscription-1'
	`).Error; err != nil {
		t.Fatalf("blank subscription scope fields: %v", err)
	}

	if err := migrateServicePackageScopeAndUsageCountersWithDB(db); err != nil {
		t.Fatalf("migrateServicePackageScopeAndUsageCountersWithDB returned error: %v", err)
	}

	servicePackage := ServicePackage{}
	if err := db.First(&servicePackage, "id = ?", "package-1").Error; err != nil {
		t.Fatalf("load service package: %v", err)
	}
	if servicePackage.PackageType != ServicePackageTypeYYCQuota ||
		servicePackage.ScopeType != ServicePackageScopeAll ||
		servicePackage.QuotaMetric != ServicePackageQuotaMetricYYC ||
		servicePackage.PeriodType != ServicePackagePeriodDaily ||
		servicePackage.PeriodLimit != 1234 ||
		!servicePackage.AllowBalanceFallback {
		t.Fatalf("service package backfill mismatch: %+v", servicePackage)
	}

	subscription := UserPackageSubscription{}
	if err := db.First(&subscription, "id = ?", "subscription-1").Error; err != nil {
		t.Fatalf("load subscription: %v", err)
	}
	if subscription.PackageType != ServicePackageTypeYYCQuota ||
		subscription.ScopeType != ServicePackageScopeAll ||
		subscription.QuotaMetric != ServicePackageQuotaMetricYYC ||
		subscription.PeriodType != ServicePackagePeriodDaily ||
		subscription.PeriodLimit != 1234 ||
		!subscription.AllowBalanceFallback {
		t.Fatalf("subscription backfill mismatch: %+v", subscription)
	}
	if !db.Migrator().HasTable(&UserPackageUsageCounter{}) {
		t.Fatalf("user package usage counter table was not created")
	}
}

func TestReserveRequestPackageMatchesGroupAndSettlesMonthlyCounter(t *testing.T) {
	db := newServicePackageScopeTestDB(t)
	servicePackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:                  "glm monthly",
		GroupID:               "group-1",
		PackageType:           ServicePackageTypeRequestQuota,
		QuotaMetric:           ServicePackageQuotaMetricRequestCount,
		PeriodLimit:           2,
		AllowBalanceFallback:  false,
		MaxConcurrencyPerUser: 1,
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("createServicePackageWithDB returned error: %v", err)
	}
	if _, err := AssignServicePackageToUserWithDB(db, servicePackage.Id, "user-1", 0); err != nil {
		t.Fatalf("AssignServicePackageToUserWithDB returned error: %v", err)
	}
	now := time.Now()
	wantPeriodKey := businessMonthByTimezone(now, DefaultGroupQuotaResetTimezone)

	result, err := ReserveRequestPackageWithDB(db, PackageScopeRequest{
		UserID:  "user-1",
		GroupID: "group-1",
		Now:     now,
	})
	if err != nil {
		t.Fatalf("ReserveRequestPackageWithDB returned error: %v", err)
	}
	if !result.Matched || !result.Allowed || !result.Reservation.Active() {
		t.Fatalf("reserve result mismatch: %+v", result)
	}
	if result.Reservation.PeriodKey != wantPeriodKey {
		t.Fatalf("period_key=%q, want %s", result.Reservation.PeriodKey, wantPeriodKey)
	}

	blocked, err := ReserveRequestPackageWithDB(db, PackageScopeRequest{
		UserID:  "user-1",
		GroupID: "group-1",
		Now:     now,
	})
	if err != nil {
		t.Fatalf("second ReserveRequestPackageWithDB returned error: %v", err)
	}
	if !blocked.Matched || blocked.Allowed || blocked.Reason != "request_concurrency_per_user_exceeded" {
		t.Fatalf("second reserve result mismatch: %+v", blocked)
	}

	if err := ReleaseRequestPackageReservationWithDB(db, result.Reservation); err != nil {
		t.Fatalf("ReleaseRequestPackageReservationWithDB returned error: %v", err)
	}
	result, err = ReserveRequestPackageWithDB(db, PackageScopeRequest{
		UserID:  "user-1",
		GroupID: "group-1",
		Now:     now,
	})
	if err != nil {
		t.Fatalf("third ReserveRequestPackageWithDB returned error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("third reserve not allowed: %+v", result)
	}
	settled, err := SettleRequestPackageReservationWithDB(db, result.Reservation, 1)
	if err != nil {
		t.Fatalf("SettleRequestPackageReservationWithDB returned error: %v", err)
	}
	if settled != 1 {
		t.Fatalf("settled=%d, want 1", settled)
	}

	result, err = ReserveRequestPackageWithDB(db, PackageScopeRequest{
		UserID:  "user-1",
		GroupID: "group-1",
		Now:     now,
	})
	if err != nil {
		t.Fatalf("fourth ReserveRequestPackageWithDB returned error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("fourth reserve not allowed: %+v", result)
	}
	if _, err := SettleRequestPackageReservationWithDB(db, result.Reservation, 1); err != nil {
		t.Fatalf("second settle returned error: %v", err)
	}
	exhausted, err := ReserveRequestPackageWithDB(db, PackageScopeRequest{
		UserID:  "user-1",
		GroupID: "group-1",
		Now:     now,
	})
	if err != nil {
		t.Fatalf("exhausted ReserveRequestPackageWithDB returned error: %v", err)
	}
	if !exhausted.Matched || exhausted.Allowed || exhausted.Reason != "request_quota_exhausted" {
		t.Fatalf("exhausted result mismatch: %+v", exhausted)
	}
}

func TestReserveRequestPackageIgnoresDifferentGroup(t *testing.T) {
	db := newServicePackageScopeTestDB(t)
	servicePackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:        "glm monthly",
		GroupID:     "group-1",
		PackageType: ServicePackageTypeRequestQuota,
		QuotaMetric: ServicePackageQuotaMetricRequestCount,
		PeriodLimit: 2,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("createServicePackageWithDB returned error: %v", err)
	}
	if _, err := AssignServicePackageToUserWithDB(db, servicePackage.Id, "user-1", 0); err != nil {
		t.Fatalf("AssignServicePackageToUserWithDB returned error: %v", err)
	}

	result, err := ReserveRequestPackageWithDB(db, PackageScopeRequest{
		UserID:  "user-1",
		GroupID: "group-2",
	})
	if err != nil {
		t.Fatalf("ReserveRequestPackageWithDB returned error: %v", err)
	}
	if result.Matched || result.Allowed {
		t.Fatalf("different group package should not match: %+v", result)
	}
}

func TestAssignServicePackageReplacesPreviousActiveAcrossGroups(t *testing.T) {
	db := newServicePackageScopeTestDB(t)
	if err := db.Create(&GroupCatalog{
		Id:      "group-2",
		Name:    "qwen",
		Enabled: true,
	}).Error; err != nil {
		t.Fatalf("seed second group: %v", err)
	}
	firstPackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:        "glm monthly",
		GroupID:     "group-1",
		PackageType: ServicePackageTypeRequestQuota,
		QuotaMetric: ServicePackageQuotaMetricRequestCount,
		PeriodLimit: 25000,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create first package: %v", err)
	}
	secondPackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:        "qwen monthly",
		GroupID:     "group-2",
		PackageType: ServicePackageTypeRequestQuota,
		QuotaMetric: ServicePackageQuotaMetricRequestCount,
		PeriodLimit: 25000,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create second package: %v", err)
	}
	if _, err := AssignServicePackageToUserWithDB(db, firstPackage.Id, "user-1", 0); err != nil {
		t.Fatalf("assign first package: %v", err)
	}
	if _, err := AssignServicePackageToUserWithDB(db, secondPackage.Id, "user-1", 0); err != nil {
		t.Fatalf("assign second package: %v", err)
	}

	rows, err := listActiveUserPackageSubscriptionsWithDB(db, "user-1")
	if err != nil {
		t.Fatalf("list active subscriptions: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("active subscription count=%d, want 1: %+v", len(rows), rows)
	}
	if rows[0].PackageID != secondPackage.Id || rows[0].GroupID != "group-2" {
		t.Fatalf("active subscription=%+v, want second package in group-2", rows[0])
	}
	replaced := int64(0)
	if err := db.Model(&UserPackageSubscription{}).
		Where("package_id = ? AND status = ?", firstPackage.Id, UserPackageSubscriptionStatusReplaced).
		Count(&replaced).Error; err != nil {
		t.Fatalf("count replaced first package: %v", err)
	}
	if replaced != 1 {
		t.Fatalf("replaced first package count=%d, want 1", replaced)
	}
}

func TestAssignServicePackageReplacesSameGroupRequestQuota(t *testing.T) {
	db := newServicePackageScopeTestDB(t)
	firstPackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:        "glm monthly small",
		GroupID:     "group-1",
		PackageType: ServicePackageTypeRequestQuota,
		QuotaMetric: ServicePackageQuotaMetricRequestCount,
		PeriodLimit: 100,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create first package: %v", err)
	}
	secondPackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:        "glm monthly large",
		GroupID:     "group-1",
		PackageType: ServicePackageTypeRequestQuota,
		QuotaMetric: ServicePackageQuotaMetricRequestCount,
		PeriodLimit: 25000,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create second package: %v", err)
	}
	if _, err := AssignServicePackageToUserWithDB(db, firstPackage.Id, "user-1", 0); err != nil {
		t.Fatalf("assign first package: %v", err)
	}
	if _, err := AssignServicePackageToUserWithDB(db, secondPackage.Id, "user-1", 0); err != nil {
		t.Fatalf("assign second package: %v", err)
	}

	rows, err := listActiveUserPackageSubscriptionsWithDB(db, "user-1")
	if err != nil {
		t.Fatalf("list active subscriptions: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("active subscription count=%d, want 1: %+v", len(rows), rows)
	}
	if rows[0].PackageID != secondPackage.Id || rows[0].PeriodLimit != 25000 {
		t.Fatalf("active subscription=%+v, want second package with limit 25000", rows[0])
	}
	replaced := int64(0)
	if err := db.Model(&UserPackageSubscription{}).
		Where("package_id = ? AND status = ?", firstPackage.Id, UserPackageSubscriptionStatusReplaced).
		Count(&replaced).Error; err != nil {
		t.Fatalf("count replaced first package: %v", err)
	}
	if replaced != 1 {
		t.Fatalf("replaced first package count=%d, want 1", replaced)
	}
}

func TestRequestPackageUsageSnapshotReportsCurrentPeriodUsage(t *testing.T) {
	db := newServicePackageScopeTestDB(t)
	servicePackage, err := createServicePackageWithDB(db, ServicePackage{
		Name:        "glm monthly",
		GroupID:     "group-1",
		PackageType: ServicePackageTypeRequestQuota,
		QuotaMetric: ServicePackageQuotaMetricRequestCount,
		PeriodLimit: 2,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create package: %v", err)
	}
	subscription, err := AssignServicePackageToUserWithDB(db, servicePackage.Id, "user-1", 0)
	if err != nil {
		t.Fatalf("assign package: %v", err)
	}
	now := time.Now()
	initial, err := GetRequestPackageUsageSnapshotWithDB(db, subscription, now)
	if err != nil {
		t.Fatalf("initial snapshot: %v", err)
	}
	if initial.LimitAmount != 2 || initial.RemainingAmount != 2 || initial.ConsumedAmount != 0 || initial.ReservedAmount != 0 {
		t.Fatalf("initial snapshot=%+v, want limit/remaining 2 and no usage", initial)
	}

	reserved, err := ReserveRequestPackageWithDB(db, PackageScopeRequest{
		UserID:  "user-1",
		GroupID: "group-1",
		Now:     now,
	})
	if err != nil {
		t.Fatalf("reserve package: %v", err)
	}
	if !reserved.Allowed {
		t.Fatalf("reserve not allowed: %+v", reserved)
	}
	afterReserve, err := GetRequestPackageUsageSnapshotWithDB(db, subscription, now)
	if err != nil {
		t.Fatalf("snapshot after reserve: %v", err)
	}
	if afterReserve.ReservedAmount != 1 || afterReserve.ConsumedAmount != 0 || afterReserve.RemainingAmount != 1 {
		t.Fatalf("after reserve snapshot=%+v, want reserved=1 consumed=0 remaining=1", afterReserve)
	}
	if _, err := SettleRequestPackageReservationWithDB(db, reserved.Reservation, 1); err != nil {
		t.Fatalf("settle package: %v", err)
	}
	afterSettle, err := GetRequestPackageUsageSnapshotWithDB(db, subscription, now)
	if err != nil {
		t.Fatalf("snapshot after settle: %v", err)
	}
	if afterSettle.ReservedAmount != 0 || afterSettle.ConsumedAmount != 1 || afterSettle.RemainingAmount != 1 {
		t.Fatalf("after settle snapshot=%+v, want reserved=0 consumed=1 remaining=1", afterSettle)
	}
}
