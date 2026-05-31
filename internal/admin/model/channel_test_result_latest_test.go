package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newChannelTestResultLatestTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ChannelTest{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return db
}

func TestGetLatestChannelTestByModelEndpointNormalizesEndpoint(t *testing.T) {
	db := newChannelTestResultLatestTestDB(t)
	if err := db.Create(&ChannelTest{
		ChannelId:    "channel-1",
		Model:        "qwen-image-2.0",
		Round:        1,
		Type:         ProviderModelTypeImage,
		Endpoint:     ChannelModelEndpointImageEdit,
		Status:       ChannelTestStatusSupported,
		Supported:    true,
		ArtifactPath: "/tmp/router-test-artifact.png",
		ArtifactName: "router-test-artifact.png",
		ArtifactSize: 1,
	}).Error; err != nil {
		t.Fatalf("create channel test: %v", err)
	}

	row, err := GetLatestChannelTestByModelEndpointWithDB(db, "channel-1", "qwen-image-2.0", "/v1/images/edits?download=1")
	if err != nil {
		t.Fatalf("GetLatestChannelTestByModelEndpointWithDB returned error: %v", err)
	}
	if row.Endpoint != ChannelModelEndpointImageEdit {
		t.Fatalf("row.Endpoint=%q, want %q", row.Endpoint, ChannelModelEndpointImageEdit)
	}
}
