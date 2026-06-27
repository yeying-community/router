package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newEntitlementConcurrencyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&EntitlementConcurrencyCounter{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return db
}

func TestReserveEntitlementConcurrencyEnforcesUserLimit(t *testing.T) {
	db := newEntitlementConcurrencyTestDB(t)

	first, err := ReserveEntitlementConcurrencyWithDB(db, EntitlementConcurrencyReserveInput{
		SourceType:               EntitlementConcurrencySourceServicePackage,
		SourceID:                 "package-1",
		SourceName:               "套餐 A",
		UserID:                   "user-1",
		RequestCount:             1,
		MaxConcurrencyPerUser:    1,
		MaxConcurrencyPerPackage: 0,
	})
	if err != nil {
		t.Fatalf("first reserve: %v", err)
	}
	if !first.Allowed || !first.Reservation.Active() {
		t.Fatalf("first reserve=%+v, want allowed active reservation", first)
	}

	blocked, err := ReserveEntitlementConcurrencyWithDB(db, EntitlementConcurrencyReserveInput{
		SourceType:               EntitlementConcurrencySourceServicePackage,
		SourceID:                 "package-1",
		SourceName:               "套餐 A",
		UserID:                   "user-1",
		RequestCount:             1,
		MaxConcurrencyPerUser:    1,
		MaxConcurrencyPerPackage: 0,
	})
	if err != nil {
		t.Fatalf("second reserve: %v", err)
	}
	if blocked.Allowed || blocked.Reason != EntitlementConcurrencyReasonPerUserExceeded {
		t.Fatalf("blocked=%+v, want per-user exceeded", blocked)
	}

	if err := ReleaseEntitlementConcurrencyReservationWithDB(db, first.Reservation); err != nil {
		t.Fatalf("release reservation: %v", err)
	}

	afterRelease, err := ReserveEntitlementConcurrencyWithDB(db, EntitlementConcurrencyReserveInput{
		SourceType:               EntitlementConcurrencySourceServicePackage,
		SourceID:                 "package-1",
		SourceName:               "套餐 A",
		UserID:                   "user-1",
		RequestCount:             1,
		MaxConcurrencyPerUser:    1,
		MaxConcurrencyPerPackage: 0,
	})
	if err != nil {
		t.Fatalf("reserve after release: %v", err)
	}
	if !afterRelease.Allowed {
		t.Fatalf("after release=%+v, want allowed", afterRelease)
	}
}

func TestReserveEntitlementConcurrencyEnforcesPackageLimitAcrossUsers(t *testing.T) {
	db := newEntitlementConcurrencyTestDB(t)

	first, err := ReserveEntitlementConcurrencyWithDB(db, EntitlementConcurrencyReserveInput{
		SourceType:               EntitlementConcurrencySourceTopupPlan,
		SourceID:                 "plan-1",
		SourceName:               "充值方案 A",
		UserID:                   "user-1",
		RequestCount:             1,
		MaxConcurrencyPerUser:    0,
		MaxConcurrencyPerPackage: 2,
	})
	if err != nil {
		t.Fatalf("first reserve: %v", err)
	}
	second, err := ReserveEntitlementConcurrencyWithDB(db, EntitlementConcurrencyReserveInput{
		SourceType:               EntitlementConcurrencySourceTopupPlan,
		SourceID:                 "plan-1",
		SourceName:               "充值方案 A",
		UserID:                   "user-2",
		RequestCount:             1,
		MaxConcurrencyPerUser:    0,
		MaxConcurrencyPerPackage: 2,
	})
	if err != nil {
		t.Fatalf("second reserve: %v", err)
	}
	blocked, err := ReserveEntitlementConcurrencyWithDB(db, EntitlementConcurrencyReserveInput{
		SourceType:               EntitlementConcurrencySourceTopupPlan,
		SourceID:                 "plan-1",
		SourceName:               "充值方案 A",
		UserID:                   "user-3",
		RequestCount:             1,
		MaxConcurrencyPerUser:    0,
		MaxConcurrencyPerPackage: 2,
	})
	if err != nil {
		t.Fatalf("third reserve: %v", err)
	}
	if blocked.Allowed || blocked.Reason != EntitlementConcurrencyReasonPerSourceExceeded {
		t.Fatalf("blocked=%+v, want per-source exceeded", blocked)
	}

	if err := ReleaseEntitlementConcurrencyReservationWithDB(db, first.Reservation); err != nil {
		t.Fatalf("release first: %v", err)
	}
	if err := ReleaseEntitlementConcurrencyReservationWithDB(db, second.Reservation); err != nil {
		t.Fatalf("release second: %v", err)
	}
}
