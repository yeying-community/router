package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newChannelCircuitBreakerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ChannelCircuitBreakerState{}, &ChannelCircuitBreakerEvent{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return db
}

func TestRecordChannelCircuitBreakerWritesEvents(t *testing.T) {
	db := newChannelCircuitBreakerTestDB(t)

	if err := recordChannelCircuitBreakerOpenWithDB(db, "channel-1", "low_success_rate", 0.25, 12345); err != nil {
		t.Fatalf("record open: %v", err)
	}
	if err := updateChannelCircuitBreakerStateWithDB(db, "channel-1", ChannelCircuitBreakerStateHalfOpen, ""); err != nil {
		t.Fatalf("record half-open: %v", err)
	}
	if err := updateChannelCircuitBreakerStateWithDB(db, "channel-1", ChannelCircuitBreakerStateRecovered, ""); err != nil {
		t.Fatalf("record recovered: %v", err)
	}

	rows, err := ListChannelCircuitBreakerEventsWithDB(db, "channel-1", 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("event count = %d, want 3: %+v", len(rows), rows)
	}
	if rows[2].Event != ChannelCircuitBreakerStateOpen || rows[2].SuccessRate != 0.25 || rows[2].RecoverAfter != 12345 {
		t.Fatalf("open event = %+v", rows[2])
	}
	if rows[0].Event != ChannelCircuitBreakerStateRecovered {
		t.Fatalf("latest event = %+v, want recovered", rows[0])
	}
}

func TestRecordChannelCircuitBreakerOpenAndRecovered(t *testing.T) {
	db := newChannelCircuitBreakerTestDB(t)

	if err := recordChannelCircuitBreakerOpenWithDB(db, "channel-1", "low_success_rate", 0.25, 12345); err != nil {
		t.Fatalf("record open: %v", err)
	}
	row, err := getChannelCircuitBreakerStateWithDB(db, "channel-1")
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if row.State != ChannelCircuitBreakerStateOpen || row.Reason != "low_success_rate" || row.SuccessRate != 0.25 || row.RecoverAfter != 12345 {
		t.Fatalf("open state = %+v, want low success rate open row", row)
	}

	if err := updateChannelCircuitBreakerStateWithDB(db, "channel-1", ChannelCircuitBreakerStateRecovered, ""); err != nil {
		t.Fatalf("record recovered: %v", err)
	}
	row, err = getChannelCircuitBreakerStateWithDB(db, "channel-1")
	if err != nil {
		t.Fatalf("get recovered state: %v", err)
	}
	if row.State != ChannelCircuitBreakerStateRecovered || row.RecoveredAt == 0 {
		t.Fatalf("recovered state = %+v, want recovered with recovered_at", row)
	}
}

func TestRecordChannelCircuitBreakerCanceledOnlyUpdatesOpenState(t *testing.T) {
	db := newChannelCircuitBreakerTestDB(t)

	if err := recordChannelCircuitBreakerOpenWithDB(db, "channel-1", "low_success_rate", 0.25, 12345); err != nil {
		t.Fatalf("record open: %v", err)
	}
	if err := updateChannelCircuitBreakerStateWithDB(db, "channel-1", ChannelCircuitBreakerStateCanceled, "insufficient balance"); err != nil {
		t.Fatalf("record canceled: %v", err)
	}
	if err := updateChannelCircuitBreakerStateWithDB(db, "channel-1", ChannelCircuitBreakerStateRecovered, ""); err != nil {
		t.Fatalf("record recovered after canceled: %v", err)
	}
	row, err := getChannelCircuitBreakerStateWithDB(db, "channel-1")
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if row.State != ChannelCircuitBreakerStateCanceled || row.Reason != "insufficient balance" {
		t.Fatalf("state = %+v, want canceled and not overwritten by recovery", row)
	}
}

func TestRecordChannelCircuitBreakerRecoveredUpdatesHalfOpenState(t *testing.T) {
	db := newChannelCircuitBreakerTestDB(t)

	if err := recordChannelCircuitBreakerOpenWithDB(db, "channel-1", "low_success_rate", 0.25, 12345); err != nil {
		t.Fatalf("record open: %v", err)
	}
	if err := updateChannelCircuitBreakerStateWithDB(db, "channel-1", ChannelCircuitBreakerStateHalfOpen, ""); err != nil {
		t.Fatalf("record half-open: %v", err)
	}
	row, err := getChannelCircuitBreakerStateWithDB(db, "channel-1")
	if err != nil {
		t.Fatalf("get half-open state: %v", err)
	}
	if row.State != ChannelCircuitBreakerStateHalfOpen || row.RecoveredAt != 0 {
		t.Fatalf("half-open state = %+v, want half-open without recovered_at", row)
	}
	if err := updateChannelCircuitBreakerStateWithDB(db, "channel-1", ChannelCircuitBreakerStateRecovered, ""); err != nil {
		t.Fatalf("record recovered: %v", err)
	}

	row, err = getChannelCircuitBreakerStateWithDB(db, "channel-1")
	if err != nil {
		t.Fatalf("get recovered state: %v", err)
	}
	if row.State != ChannelCircuitBreakerStateRecovered || row.RecoveredAt == 0 {
		t.Fatalf("recovered state = %+v, want recovered with recovered_at", row)
	}
}

func TestListOpenChannelCircuitBreakerStates(t *testing.T) {
	db := newChannelCircuitBreakerTestDB(t)

	if err := recordChannelCircuitBreakerOpenWithDB(db, "channel-1", "low_success_rate", 0.25, 12345); err != nil {
		t.Fatalf("record open: %v", err)
	}
	if err := recordChannelCircuitBreakerOpenWithDB(db, "channel-2", "low_success_rate", 0.75, 12346); err != nil {
		t.Fatalf("record open: %v", err)
	}
	if err := updateChannelCircuitBreakerStateWithDB(db, "channel-2", ChannelCircuitBreakerStateRecovered, ""); err != nil {
		t.Fatalf("record recovered: %v", err)
	}

	rows, err := listOpenChannelCircuitBreakerStatesWithDB(db)
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	if len(rows) != 1 || rows[0].ChannelId != "channel-1" {
		t.Fatalf("open rows = %+v, want channel-1 only", rows)
	}
}

func TestListHalfOpenChannelCircuitBreakerStates(t *testing.T) {
	db := newChannelCircuitBreakerTestDB(t)

	if err := recordChannelCircuitBreakerOpenWithDB(db, "channel-1", "low_success_rate", 0.25, 12345); err != nil {
		t.Fatalf("record open: %v", err)
	}
	if err := recordChannelCircuitBreakerOpenWithDB(db, "channel-2", "low_success_rate", 0.75, 12346); err != nil {
		t.Fatalf("record open: %v", err)
	}
	if err := updateChannelCircuitBreakerStateWithDB(db, "channel-2", ChannelCircuitBreakerStateHalfOpen, ""); err != nil {
		t.Fatalf("record half-open: %v", err)
	}

	rows, err := listHalfOpenChannelCircuitBreakerStatesWithDB(db)
	if err != nil {
		t.Fatalf("list half-open: %v", err)
	}
	if len(rows) != 1 || rows[0].ChannelId != "channel-2" {
		t.Fatalf("half-open rows = %+v, want channel-2 only", rows)
	}
}

func TestListChannelCircuitBreakerStatesByChannelIDs(t *testing.T) {
	db := newChannelCircuitBreakerTestDB(t)

	if err := recordChannelCircuitBreakerOpenWithDB(db, "channel-1", "low_success_rate", 0.25, 12345); err != nil {
		t.Fatalf("record open: %v", err)
	}
	if err := recordChannelCircuitBreakerOpenWithDB(db, "channel-2", "low_success_rate", 0.75, 12346); err != nil {
		t.Fatalf("record open: %v", err)
	}

	rows, err := ListChannelCircuitBreakerStatesByChannelIDsWithDB(db, []string{"channel-2", "channel-2", "missing", ""})
	if err != nil {
		t.Fatalf("list by ids: %v", err)
	}
	if len(rows) != 1 || rows[0].ChannelId != "channel-2" {
		t.Fatalf("rows = %+v, want channel-2 only", rows)
	}
}
