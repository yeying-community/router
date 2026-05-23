package model

import (
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newChannelManualValidationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ProviderModel{}, &ChannelModelSyncResult{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return db
}

func TestValidateManualChannelModelChangesSkipsUnchangedDirtySelectedRows(t *testing.T) {
	db := newChannelManualValidationTestDB(t)
	if err := db.Create(&ProviderModel{
		Provider: "qwen",
		Model:    "qwen3.7-max",
		Type:     ProviderModelTypeText,
		Status:   ProviderModelStatusActive,
	}).Error; err != nil {
		t.Fatalf("create provider model: %v", err)
	}
	if err := db.Create(&ChannelModelSyncResult{
		ChannelId:     "channel-1",
		Model:         "qwen3.7-max",
		UpstreamModel: "qwen3.7-max",
		Returned:      true,
	}).Error; err != nil {
		t.Fatalf("create sync result: %v", err)
	}
	currentRows := []ChannelModel{
		{Model: "qwen-vl-max-latest", UpstreamModel: "qwen-vl-max-latest", Provider: "qwen", Type: ProviderModelTypeText, Selected: true},
		{Model: "qwen3.7-max", UpstreamModel: "qwen3.7-max", Provider: "qwen", Type: ProviderModelTypeText, Selected: false},
	}
	nextRows := []ChannelModel{
		{Model: "qwen-vl-max-latest", UpstreamModel: "qwen-vl-max-latest", Provider: "qwen", Type: ProviderModelTypeText, Selected: true},
		{Model: "qwen3.7-max", UpstreamModel: "qwen3.7-max", Provider: "qwen", Type: ProviderModelTypeText, Selected: true},
	}

	if err := ValidateManualChannelModelChangesWithDB(db, "channel-1", currentRows, nextRows); err != nil {
		t.Fatalf("ValidateManualChannelModelChangesWithDB returned error for unchanged dirty row: %v", err)
	}
}

func TestValidateManualChannelModelChangesValidatesNewlyEnabledRows(t *testing.T) {
	db := newChannelManualValidationTestDB(t)
	currentRows := []ChannelModel{
		{Model: "qwen-vl-max-latest", UpstreamModel: "qwen-vl-max-latest", Provider: "qwen", Type: ProviderModelTypeText, Selected: false},
	}
	nextRows := []ChannelModel{
		{Model: "qwen-vl-max-latest", UpstreamModel: "qwen-vl-max-latest", Provider: "qwen", Type: ProviderModelTypeText, Selected: true},
	}

	err := ValidateManualChannelModelChangesWithDB(db, "channel-1", currentRows, nextRows)
	if err == nil || !strings.Contains(err.Error(), "缺少供应商官方信息") {
		t.Fatalf("ValidateManualChannelModelChangesWithDB error=%v, want missing provider official info", err)
	}
}
