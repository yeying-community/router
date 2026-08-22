package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newUserNotificationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &UserBalanceLot{}, &UserNotificationEvent{}, &UserBalanceNotificationState{}); err != nil {
		t.Fatalf("migrate notification tables: %v", err)
	}
	return db
}

func TestCreateUserOrderNotificationEventIsIdempotent(t *testing.T) {
	db := newUserNotificationTestDB(t)
	if err := db.Create(&User{Id: "user-1", Username: "test", Email: "person@example.com"}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	order := TopupOrder{Id: "order-1", UserID: "user-1", BusinessType: TopupOrderBusinessBalance, CreditOrigin: TopupOrderCreditOriginPaid, Status: TopupOrderStatusFulfilled, Quota: 100}
	for range 2 {
		if err := CreateUserOrderNotificationEventWithDB(db, order); err != nil {
			t.Fatalf("create notification event: %v", err)
		}
	}
	var count int64
	if err := db.Model(&UserNotificationEvent{}).Count(&count).Error; err != nil {
		t.Fatalf("count notification events: %v", err)
	}
	if count != 1 {
		t.Fatalf("notification event count = %d, want 1", count)
	}
}

func TestRefreshUserBalanceLowNotificationEventsRequiresRecovery(t *testing.T) {
	db := newUserNotificationTestDB(t)
	if err := db.Create(&User{Id: "user-1", Username: "test", Email: "person@example.com"}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	lot := UserBalanceLot{Id: "lot-1", UserID: "user-1", SourceType: UserBalanceLotSourceTopup, SourceID: "order-1", TotalAmount: 50, RemainingAmount: 50, Status: UserBalanceLotStatusActive}
	if err := db.Create(&lot).Error; err != nil {
		t.Fatalf("create balance lot: %v", err)
	}
	if err := RefreshUserBalanceLowNotificationEventsWithDB(db, 100, 1000); err != nil {
		t.Fatalf("create first balance alert: %v", err)
	}
	if err := RefreshUserBalanceLowNotificationEventsWithDB(db, 100, 1001); err != nil {
		t.Fatalf("repeat low balance scan: %v", err)
	}
	if err := db.Model(&UserBalanceLot{}).Where("id = ?", lot.Id).Update("remaining_amount", 150).Error; err != nil {
		t.Fatalf("restore balance: %v", err)
	}
	if err := RefreshUserBalanceLowNotificationEventsWithDB(db, 100, 1002); err != nil {
		t.Fatalf("scan restored balance: %v", err)
	}
	if err := db.Model(&UserBalanceLot{}).Where("id = ?", lot.Id).Update("remaining_amount", 25).Error; err != nil {
		t.Fatalf("lower balance again: %v", err)
	}
	if err := RefreshUserBalanceLowNotificationEventsWithDB(db, 100, 1003); err != nil {
		t.Fatalf("create second balance alert: %v", err)
	}
	var count int64
	if err := db.Model(&UserNotificationEvent{}).Where("event_type = ?", UserNotificationEventTypeBalanceLow).Count(&count).Error; err != nil {
		t.Fatalf("count balance alerts: %v", err)
	}
	if count != 2 {
		t.Fatalf("balance alert count = %d, want 2", count)
	}
}
