package model

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newCostRateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ChannelModelCostRate{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestReplaceAndResolveChannelModelCostRate(t *testing.T) {
	db := newCostRateTestDB(t)
	err := ReplaceChannelModelCostRatesWithDB(db, "ch-1", []ChannelModelCostRate{
		{Model: "GPT-6.1-Sol", CapacityUnit: "PER_1K_TOKENS", UnitCostYyc: 142.8, UnitCostOriginal: 0.002, Currency: "usd", Confidence: "actual", AsOf: 1000},
		{Model: "m-estimated", CapacityUnit: "per_1k_tokens", UnitCostYyc: 50, Confidence: "estimated", AsOf: 1000},
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	// Lookup is case-insensitive on unit; model trimmed, unit lowered.
	rate, ok := ResolveChannelModelCostRateWithDB(db, "ch-1", "GPT-6.1-Sol", []string{"per_1k_tokens", "usd_equivalent"})
	if !ok || rate.UnitCostYyc != 142.8 || rate.CapacityUnit != "per_1k_tokens" {
		t.Fatalf("resolve = %+v ok=%v", rate, ok)
	}
	// Unknown model / unit → miss.
	if _, ok := ResolveChannelModelCostRateWithDB(db, "ch-1", "missing", []string{"per_1k_tokens"}); ok {
		t.Fatal("expected miss for unknown model")
	}
	if _, ok := ResolveChannelModelCostRateWithDB(db, "ch-1", "GPT-6.1-Sol", []string{"per_image"}); ok {
		t.Fatal("expected miss for non-matching unit")
	}
	// Replace is a full refresh: empty set clears the channel.
	if err := ReplaceChannelModelCostRatesWithDB(db, "ch-1", nil); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, ok := ResolveChannelModelCostRateWithDB(db, "ch-1", "GPT-6.1-Sol", []string{"per_1k_tokens"}); ok {
		t.Fatal("expected miss after clear")
	}
}

func TestIsChannelModelCostRateFresh(t *testing.T) {
	now := time.Unix(10_000, 0)
	fresh := ChannelModelCostRate{UnitCostYyc: 1, Confidence: "actual", AsOf: 9_500}
	if !IsChannelModelCostRateFresh(fresh, now, 1000) {
		t.Fatal("expected fresh within TTL")
	}
	stale := ChannelModelCostRate{UnitCostYyc: 1, Confidence: "actual", AsOf: 8_000}
	if IsChannelModelCostRateFresh(stale, now, 1000) {
		t.Fatal("expected stale beyond TTL")
	}
	// Non-actual confidence never drives the floor.
	est := ChannelModelCostRate{UnitCostYyc: 1, Confidence: "estimated", AsOf: 9_999}
	if IsChannelModelCostRateFresh(est, now, 1000) {
		t.Fatal("estimated must not be fresh")
	}
	// TTL 0 disables.
	if IsChannelModelCostRateFresh(fresh, now, 0) {
		t.Fatal("ttl 0 must disable")
	}
	// Zero cost never fresh.
	zero := ChannelModelCostRate{UnitCostYyc: 0, Confidence: "actual", AsOf: 9_999}
	if IsChannelModelCostRateFresh(zero, now, 1000) {
		t.Fatal("zero cost must not be fresh")
	}
}
