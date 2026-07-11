package channel

import (
	"strings"
	"testing"

	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMergeMissingProviderDetailsAsDeleted(t *testing.T) {
	current := []model.ProviderModelDetail{
		{
			Model:       "gpt-5.5",
			Type:        model.ProviderModelTypeText,
			Description: "active",
		},
	}
	existing := []model.ProviderModelDetail{
		{
			Model:       "gpt-5.5",
			Type:        model.ProviderModelTypeText,
			Description: "active",
		},
		{
			Model:       "gpt-4.1",
			Type:        model.ProviderModelTypeText,
			Description: "legacy",
		},
	}

	merged := mergeMissingProviderDetailsAsDeleted(current, existing)
	if len(merged) != 2 {
		t.Fatalf("mergeMissingProviderDetailsAsDeleted len=%d, want 2", len(merged))
	}

	byModel := make(map[string]model.ProviderModelDetail, len(merged))
	for _, detail := range merged {
		byModel[detail.Model] = detail
	}
	if byModel["gpt-5.5"].IsDeleted {
		t.Fatalf("gpt-5.5 should remain active")
	}
	if !byModel["gpt-4.1"].IsDeleted {
		t.Fatalf("gpt-4.1 should be soft deleted")
	}
	if byModel["gpt-4.1"].Description != "legacy" {
		t.Fatalf("soft deleted detail should preserve original fields, got description=%q", byModel["gpt-4.1"].Description)
	}
}

func TestCollectProviderModelUsageIncludesChannelNames(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Channel{},
		&model.ChannelModel{},
		&model.GroupModel{},
		&model.GroupModelChannel{},
	); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}

	const channelID = "f4dd52e7117948d58bfd5cdee400e571"
	if err := db.Create(&model.Channel{
		Id:       channelID,
		Name:     "primary-openai",
		Protocol: "openai",
		Status:   model.ChannelStatusEnabled,
	}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&model.ChannelModel{
		ChannelId:      channelID,
		Provider:       "openai",
		Model:          "openai/codex-mini-latest",
		Selected:       true,
		PublishEnabled: true,
	}).Error; err != nil {
		t.Fatalf("create channel model: %v", err)
	}

	usage, err := collectProviderModelUsageWithDB(db, "openai", "openai/codex-mini-latest")
	if err != nil {
		t.Fatalf("collect usage: %v", err)
	}

	message := usage.Error("openai", "openai/codex-mini-latest").Error()
	want := "channels=primary-openai (" + channelID + ")"
	if !strings.Contains(message, want) {
		t.Fatalf("usage error=%q, want to contain %q", message, want)
	}
}

func TestCollectProviderModelUsageIgnoresUnpublishedChannelModels(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Channel{},
		&model.ChannelModel{},
		&model.GroupModel{},
		&model.GroupModelChannel{},
	); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}

	const channelID = "f4dd52e7117948d58bfd5cdee400e571"
	if err := db.Create(&model.Channel{
		Id:       channelID,
		Name:     "qingyuntop-2",
		Protocol: "openai",
		Status:   model.ChannelStatusEnabled,
	}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&model.ChannelModel{
		ChannelId:      channelID,
		Provider:       "openai",
		Model:          "dall-e-3",
		Selected:       true,
		PublishEnabled: false,
	}).Error; err != nil {
		t.Fatalf("create channel model: %v", err)
	}

	usage, err := collectProviderModelUsageWithDB(db, "openai", "dall-e-3")
	if err != nil {
		t.Fatalf("collect usage: %v", err)
	}
	if len(usage.ChannelModels) != 0 {
		t.Fatalf("ChannelModels=%v, want empty for unpublished model", usage.ChannelModels)
	}
	if usage.InUse() {
		t.Fatalf("usage.InUse()=true, want false")
	}
}
