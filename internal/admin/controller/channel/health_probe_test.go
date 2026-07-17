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

func TestPersistAutomaticProbeDoesNotUpdateEndpointConfiguration(t *testing.T) {
	db := newChannelHealthProbeTestDB(t)
	if err := db.AutoMigrate(&model.Channel{}, &model.ChannelModelPriceComponent{}, &model.ChannelModelEndpointTestResult{}); err != nil {
		t.Fatalf("migrate endpoint test results: %v", err)
	}
	if err := db.Create(&model.Channel{Id: "image-channel", Name: "image-channel"}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	result := model.ChannelTest{
		ChannelId: "image-channel",
		Model:     "gpt-5.5",
		Endpoint:  model.ChannelModelEndpointResponses,
		Source:    "automatic_probe",
		Status:    model.ChannelTestStatusUnsupported,
		Supported: false,
		TestedAt:  1_800_000_000,
		Message:   "upstream model unavailable",
	}
	if err := persistChannelModelTests("image-channel", "probe-task", []model.ChannelTest{result}); err != nil {
		t.Fatalf("persist automatic probe: %v", err)
	}
	var endpointResult model.ChannelModelEndpointTestResult
	if err := db.First(&endpointResult).Error; err == nil {
		t.Fatal("automatic probe must not create endpoint configuration test result")
	}
	var signal model.ChannelTest
	if err := db.First(&signal, "channel_id = ? AND model = ?", "image-channel", "gpt-5.5").Error; err != nil {
		t.Fatalf("load health signal: %v", err)
	}
	if signal.Source != "automatic_probe" {
		t.Fatalf("signal source=%q, want automatic_probe", signal.Source)
	}
}
