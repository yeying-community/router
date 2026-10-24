package model

import (
	"math"
	"testing"

	"github.com/yeying-community/router/internal/admin/healthtrend"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLoadChannelRuntimeMetricsUsesRealTrafficAndIsolatesPersonalOwner(t *testing.T) {
	previousLogDB := LOG_DB
	t.Cleanup(func() { LOG_DB = previousLogDB })
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	LOG_DB = db
	if err := db.AutoMigrate(&Log{}); err != nil {
		t.Fatalf("migrate logs: %v", err)
	}
	rows := []Log{
		{Id: "managed-success", UserId: "user-a", CreatedAt: 110, Type: LogTypeConsume, ChannelId: "managed-1", ElapsedTime: 100, PromptTokens: 10, CompletionTokens: 5, Quota: 20},
		{Id: "managed-failure", UserId: "user-a", CreatedAt: 120, Type: LogTypeRelayFailure, ChannelId: "managed-1", ElapsedTime: 300},
		{Id: "managed-abort", UserId: "user-a", CreatedAt: 130, Type: LogTypeRelayFailure, ChannelId: "managed-1", ElapsedTime: 900, RelayErrorType: "client_abort"},
		{Id: "personal-owner", UserId: "user-a", CreatedAt: 140, Type: LogTypeConsume, ChannelId: "personal:personal-1", PersonalProviderId: "personal-1", ElapsedTime: 80},
		{Id: "personal-other-user", UserId: "user-b", CreatedAt: 150, Type: LogTypeConsume, ChannelId: "personal:personal-1", PersonalProviderId: "personal-1", ElapsedTime: 40},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed logs: %v", err)
	}

	metrics, err := LoadChannelRuntimeMetrics(ChannelRuntimeMetricQuery{
		Window:     ChannelMetricWindow{Period: ChannelMetricPeriodLastHour, StartAt: 100, EndAt: 200, IntervalSeconds: 50},
		ChannelIDs: []string{"managed-1"}, PersonalProviderIDs: []string{"personal-1"}, UserID: "user-a",
	})
	if err != nil {
		t.Fatalf("LoadChannelRuntimeMetrics: %v", err)
	}
	managed := metrics["managed-1"]
	if managed.SuccessCount != 1 || managed.FailureCount != 1 || managed.RequestCount != 2 {
		t.Fatalf("managed counts = %d/%d/%d, want 1/1/2", managed.SuccessCount, managed.FailureCount, managed.RequestCount)
	}
	if math.Abs(managed.SuccessRate-0.5) > 0.0001 || managed.AvgLatencyMs != 200 {
		t.Fatalf("managed rate/latency = %f/%d, want 0.5/200", managed.SuccessRate, managed.AvgLatencyMs)
	}
	if managed.LastObservedAt != 120 {
		t.Fatalf("managed last observed = %d, want 120 with client abort excluded", managed.LastObservedAt)
	}
	if managed.PromptTokens != 10 || managed.OutputTokens != 5 || managed.TotalTokens != 15 || managed.SpendQuota != 20 {
		t.Fatalf("managed token/quota metrics = %#v", managed)
	}
	personal := metrics[PersonalProviderChannelPrefix+"personal-1"]
	if personal.SuccessCount != 1 || personal.RequestCount != 1 || personal.AvgLatencyMs != 80 {
		t.Fatalf("personal metrics = %#v, want only user-a traffic", personal)
	}
}

func TestMergeChannelRuntimeMetricsUsesWeightedLatencyAndMergedBuckets(t *testing.T) {
	window := ChannelMetricWindow{Period: ChannelMetricPeriodLastHour, StartAt: 100, EndAt: 199, IntervalSeconds: 50}
	merged := MergeChannelRuntimeMetrics(window,
		ChannelRuntimeMetrics{
			PromptTokens: 10, OutputTokens: 5, SpendQuota: 20,
			Points: []healthtrend.Point{{BucketStart: 100, SuccessCount: 2, TotalCount: 2, AvgLatencyMs: 100, LatencyCount: 2, LastObservedAt: 120}},
		},
		ChannelRuntimeMetrics{
			PromptTokens: 4, OutputTokens: 6, SpendQuota: 30,
			Points: []healthtrend.Point{
				{BucketStart: 100, FailureCount: 1, TotalCount: 1, AvgLatencyMs: 400, LatencyCount: 1, LastObservedAt: 130},
				{BucketStart: 150, SuccessCount: 1, TotalCount: 1, AvgLatencyMs: 200, LatencyCount: 1, LastObservedAt: 170},
			},
		},
	)

	if merged.SuccessCount != 3 || merged.FailureCount != 1 || merged.RequestCount != 4 {
		t.Fatalf("merged counts = %d/%d/%d, want 3/1/4", merged.SuccessCount, merged.FailureCount, merged.RequestCount)
	}
	if math.Abs(merged.SuccessRate-0.75) > 0.0001 {
		t.Fatalf("merged success rate = %f, want 0.75", merged.SuccessRate)
	}
	if merged.AvgLatencyMs != 200 {
		t.Fatalf("merged avg latency = %d, want weighted average 200", merged.AvgLatencyMs)
	}
	if merged.LastObservedAt != 170 {
		t.Fatalf("merged last observed = %d, want 170", merged.LastObservedAt)
	}
	if merged.PromptTokens != 14 || merged.OutputTokens != 11 || merged.TotalTokens != 25 || merged.SpendQuota != 50 {
		t.Fatalf("merged token/quota metrics = %#v", merged)
	}
	if len(merged.Points) != 2 || merged.Points[0].SuccessCount != 2 || merged.Points[0].FailureCount != 1 || merged.Points[0].AvgLatencyMs != 200 {
		t.Fatalf("merged first point = %#v, want combined weighted bucket", merged.Points)
	}
}
