package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newChannelModelEndpointTestResultTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ChannelModelEndpointTestResult{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return db
}

func TestUpsertChannelModelEndpointTestResultsKeepsNewestResult(t *testing.T) {
	db := newChannelModelEndpointTestResultTestDB(t)
	newer := ChannelTest{
		ChannelId: "channel-1",
		Model:     "qwen3.7-max",
		Type:      ProviderModelTypeText,
		Endpoint:  ChannelModelEndpointChat,
		Status:    ChannelTestStatusSupported,
		Supported: true,
		TestedAt:  200,
	}
	older := ChannelTest{
		ChannelId: "channel-1",
		Model:     "qwen3.7-max",
		Type:      ProviderModelTypeText,
		Endpoint:  ChannelModelEndpointChat,
		Status:    ChannelTestStatusUnsupported,
		Supported: false,
		Message:   "older failure",
		TestedAt:  100,
	}
	if err := UpsertChannelModelEndpointTestResultsWithDB(db, "channel-1", "task-newer", []ChannelTest{newer}); err != nil {
		t.Fatalf("upsert newer: %v", err)
	}
	if err := UpsertChannelModelEndpointTestResultsWithDB(db, "channel-1", "task-older", []ChannelTest{older}); err != nil {
		t.Fatalf("upsert older: %v", err)
	}

	row := ChannelModelEndpointTestResult{}
	if err := db.First(&row, "channel_id = ? AND model = ? AND endpoint = ?", "channel-1", "qwen3.7-max", ChannelModelEndpointChat).Error; err != nil {
		t.Fatalf("load row: %v", err)
	}
	if row.LastTestTaskId != "task-newer" || row.LastTestedAt != 200 || !row.LastSupported || row.LastTestStatus != ChannelModelEndpointTestStatusSuccess {
		t.Fatalf("row=%#v, want newer successful result preserved", row)
	}
}
