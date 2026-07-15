package channel

import (
	"testing"
	"time"

	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newChannelHealthProbeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ChannelModel{}, &model.ChannelTest{}, &model.Log{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestEnqueueDueChannelHealthProbesUsesTrafficAndSilence(t *testing.T) {
	db := newChannelHealthProbeTestDB(t)
	now := int64(1_800_000_000)
	rows := []model.ChannelModel{
		{ChannelId: "recent", Model: "gpt-recent", Type: model.ProviderModelTypeText, Selected: true, PublishEnabled: true, PublishedAt: now - int64(24*time.Hour/time.Second)},
		{ChannelId: "stale", Model: "gpt-stale", Type: model.ProviderModelTypeText, Selected: true, PublishEnabled: true, PublishedAt: now - int64(24*time.Hour/time.Second)},
		{ChannelId: "new", Model: "gpt-new", Type: model.ProviderModelTypeText, Selected: true, PublishEnabled: true, PublishedAt: now - int64(time.Hour/time.Second)},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create channel models: %v", err)
	}
	if err := db.Create(&model.Log{Id: "recent-log", Type: model.LogTypeConsume, ChannelId: "recent", RequestModelName: "gpt-recent", CreatedAt: now - 60}).Error; err != nil {
		t.Fatalf("create recent log: %v", err)
	}
	createdTargets := make([]string, 0)
	created, err := enqueueDueChannelHealthProbes(db, db, now, 10, func(channelID string, modelID string) (bool, error) {
		createdTargets = append(createdTargets, channelID+":"+modelID)
		return true, nil
	})
	if err != nil {
		t.Fatalf("enqueue probes: %v", err)
	}
	if created != 1 || len(createdTargets) != 1 || createdTargets[0] != "stale:gpt-stale" {
		t.Fatalf("created=%d targets=%v, want stale target", created, createdTargets)
	}
}

func TestEnqueueDueChannelHealthProbesRetriesFailureAfterBackoff(t *testing.T) {
	db := newChannelHealthProbeTestDB(t)
	now := int64(1_800_000_000)
	row := model.ChannelModel{ChannelId: "failed", Model: "gpt-failed", Type: model.ProviderModelTypeText, Selected: true, PublishEnabled: true, PublishedAt: now - int64(24*time.Hour/time.Second)}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create channel model: %v", err)
	}
	if err := db.Create(&model.Log{Id: "failed-log", Type: model.LogTypeRelayFailure, ChannelId: "failed", RequestModelName: "gpt-failed", RelayErrorCode: "upstream_unavailable", CreatedAt: now - int64(20*time.Minute/time.Second)}).Error; err != nil {
		t.Fatalf("create failure log: %v", err)
	}
	created, err := enqueueDueChannelHealthProbes(db, db, now, 10, func(channelID string, modelID string) (bool, error) {
		return true, nil
	})
	if err != nil {
		t.Fatalf("enqueue probes: %v", err)
	}
	if created != 1 {
		t.Fatalf("created=%d, want 1", created)
	}
}
