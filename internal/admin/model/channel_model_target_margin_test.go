package model

import (
	"testing"

	"github.com/yeying-community/router/common/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestResolveChannelModelTargetMarginWithDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ChannelModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	originGlobal := config.BillingTargetMargin
	config.BillingTargetMargin = 0.2
	defer func() { config.BillingTargetMargin = originGlobal }()

	override := 0.4
	if err := db.Create(&ChannelModel{ChannelId: "c1", Model: "m-override", TargetMargin: &override}).Error; err != nil {
		t.Fatalf("create override model: %v", err)
	}
	if err := db.Create(&ChannelModel{ChannelId: "c1", Model: "m-global"}).Error; err != nil {
		t.Fatalf("create global model: %v", err)
	}
	// Out-of-range override is clamped by normalizeTargetMargin (<= 0.95).
	tooHigh := 5.0
	if err := db.Create(&ChannelModel{ChannelId: "c1", Model: "m-clamped", TargetMargin: &tooHigh}).Error; err != nil {
		t.Fatalf("create clamped model: %v", err)
	}

	if got := ResolveChannelModelTargetMarginWithDB(db, "c1", "m-override"); got != 0.4 {
		t.Fatalf("override margin = %v, want 0.4", got)
	}
	if got := ResolveChannelModelTargetMarginWithDB(db, "c1", "m-global"); got != 0.2 {
		t.Fatalf("global fallback margin = %v, want 0.2 (global)", got)
	}
	if got := ResolveChannelModelTargetMarginWithDB(db, "c1", "m-clamped"); got != 0.95 {
		t.Fatalf("clamped margin = %v, want 0.95", got)
	}
	// Unknown model and empty args fall back to the global policy.
	if got := ResolveChannelModelTargetMarginWithDB(db, "c1", "missing"); got != 0.2 {
		t.Fatalf("unknown model margin = %v, want 0.2 (global)", got)
	}
	if got := ResolveChannelModelTargetMarginWithDB(db, "", ""); got != 0.2 {
		t.Fatalf("empty args margin = %v, want 0.2 (global)", got)
	}
	if got := ResolveChannelModelTargetMarginWithDB(nil, "c1", "m-override"); got != 0.2 {
		t.Fatalf("nil db margin = %v, want 0.2 (global)", got)
	}
}
