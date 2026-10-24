package model

import (
	"fmt"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestResolveAsyncTaskBusinessOutcomeForSupportedChannelModelTest(t *testing.T) {
	status, message, ok := ResolveAsyncTaskBusinessOutcome(
		AsyncTaskTypeChannelModelTest,
		`{"status":"supported","supported":true,"message":"ok"}`,
	)
	if !ok {
		t.Fatalf("ResolveAsyncTaskBusinessOutcome ok = false, want true")
	}
	if status != AsyncTaskStatusSucceeded {
		t.Fatalf("ResolveAsyncTaskBusinessOutcome status = %q, want %q", status, AsyncTaskStatusSucceeded)
	}
	if message != "" {
		t.Fatalf("ResolveAsyncTaskBusinessOutcome message = %q, want empty", message)
	}
}

func TestResolveAsyncTaskBusinessOutcomeForUnsupportedChannelModelTest(t *testing.T) {
	status, message, ok := ResolveAsyncTaskBusinessOutcome(
		AsyncTaskTypeChannelModelTest,
		`{"status":"unsupported","supported":false,"message":"dial tcp timeout"}`,
	)
	if !ok {
		t.Fatalf("ResolveAsyncTaskBusinessOutcome ok = false, want true")
	}
	if status != AsyncTaskStatusSucceeded {
		t.Fatalf("ResolveAsyncTaskBusinessOutcome status = %q, want %q", status, AsyncTaskStatusSucceeded)
	}
	if message != "" {
		t.Fatalf("ResolveAsyncTaskBusinessOutcome message = %q, want empty", message)
	}
}

func TestResolveAsyncTaskBusinessOutcomeRejectsInvalidModelTestResult(t *testing.T) {
	for _, result := range []string{"", "not-json", `{}`, `{"status":"unknown"}`} {
		status, message, ok := ResolveAsyncTaskBusinessOutcome(AsyncTaskTypeChannelModelTest, result)
		if ok || status != "" || message != "" {
			t.Fatalf("ResolveAsyncTaskBusinessOutcome(%q) = %q/%q/%v, want empty/empty/false", result, status, message, ok)
		}
	}
}

func TestNormalizeAsyncTaskRowPreservesExecutionStatus(t *testing.T) {
	row := AsyncTask{
		Type:   AsyncTaskTypeChannelModelTest,
		Status: AsyncTaskStatusSucceeded,
		Result: `{"status":"unsupported","supported":false,"message":"request timeout"}`,
	}

	normalizeAsyncTaskRow(&row)

	if row.Status != AsyncTaskStatusSucceeded {
		t.Fatalf("normalizeAsyncTaskRow status = %q, want %q", row.Status, AsyncTaskStatusSucceeded)
	}
	if row.ErrorMessage != "" {
		t.Fatalf("normalizeAsyncTaskRow error_message = %q, want empty", row.ErrorMessage)
	}
}

func TestListDutyAttentionTasksWithDBIncludesAllMatchingTasks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&AsyncTask{}); err != nil {
		t.Fatalf("migrate async tasks: %v", err)
	}

	const now = int64(100_000)
	rows := make([]AsyncTask, 0, 211)
	for i := 0; i < 205; i++ {
		rows = append(rows, AsyncTask{
			Id: fmt.Sprintf("pending-%03d", i), Status: AsyncTaskStatusPending,
			CreatedAt: now - 60,
		})
	}
	rows = append(rows,
		AsyncTask{Id: "stale-pending", Status: AsyncTaskStatusPending, CreatedAt: now - 3_601},
		AsyncTask{Id: "stale-running", Status: AsyncTaskStatusRunning, CreatedAt: now - 5_000, StartedAt: now - 1_900},
		AsyncTask{Id: "recent-running", Status: AsyncTaskStatusRunning, CreatedAt: now - 5_000, StartedAt: now - 500},
		AsyncTask{Id: "recent-failed", Status: AsyncTaskStatusFailed, CreatedAt: now - 120},
		AsyncTask{Id: "old-failed", Status: AsyncTaskStatusFailed, CreatedAt: now - 3_601},
		AsyncTask{Id: "recent-succeeded", Status: AsyncTaskStatusSucceeded, CreatedAt: now - 60},
	)
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed async tasks: %v", err)
	}

	items, total, err := ListDutyAttentionTasksWithDB(db, now-3_600, now, now-1_800)
	if err != nil {
		t.Fatalf("list duty attention tasks: %v", err)
	}
	if total != 208 || len(items) != 208 {
		t.Fatalf("duty task result = %d items / %d total, want all 208 matching tasks", len(items), total)
	}
	for _, item := range items {
		if item.Id == "old-failed" || item.Id == "recent-running" || item.Id == "recent-succeeded" {
			t.Fatalf("out-of-scope task %q was returned", item.Id)
		}
	}
}
