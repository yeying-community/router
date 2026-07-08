package dashboard

import (
	"math"
	"testing"
	"time"

	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSummarizeChannelHealthItemsUsesAllItems(t *testing.T) {
	items := []channelHealthItem{
		{
			SelectedModelCount: 2,
			TestedModelCount:   2,
			PassRate:           0.5,
			AvgLatencyMs:       9000,
			HasTestData:        true,
			HealthLevel:        channelHealthLevelCritical,
			CircuitBreaker: &channelCircuitBreakerDashboardItem{
				State: model.ChannelCircuitBreakerStateOpen,
			},
		},
		{
			SelectedModelCount: 1,
			TestedModelCount:   0,
			HasTestData:        false,
			HealthLevel:        channelHealthLevelWarning,
		},
		{
			SelectedModelCount: 0,
			TestedModelCount:   0,
			HasTestData:        false,
			HealthLevel:        channelHealthLevelHealthy,
		},
	}

	summary := summarizeChannelHealthItems(items)
	if summary.WithTests != 1 {
		t.Fatalf("WithTests=%d, want 1", summary.WithTests)
	}
	if summary.WithoutTests != 2 {
		t.Fatalf("WithoutTests=%d, want 2", summary.WithoutTests)
	}
	if summary.NeedsRetest != 2 {
		t.Fatalf("NeedsRetest=%d, want 2", summary.NeedsRetest)
	}
	if summary.RiskCount != 1 {
		t.Fatalf("RiskCount=%d, want 1", summary.RiskCount)
	}
	if summary.ActiveCircuitBreakerCount != 1 {
		t.Fatalf("ActiveCircuitBreakerCount=%d, want 1", summary.ActiveCircuitBreakerCount)
	}
	if summary.HighLatencyCount != 1 {
		t.Fatalf("HighLatencyCount=%d, want 1", summary.HighLatencyCount)
	}
	if math.Abs(summary.AvgPassRate-0.5) > 0.0001 {
		t.Fatalf("AvgPassRate=%f, want 0.5", summary.AvgPassRate)
	}
	if math.Abs(summary.AvgCoverageRate-(2.0/3.0)) > 0.0001 {
		t.Fatalf("AvgCoverageRate=%f, want %f", summary.AvgCoverageRate, 2.0/3.0)
	}
	if summary.AvgLatencyMs != 9000 {
		t.Fatalf("AvgLatencyMs=%d, want 9000", summary.AvgLatencyMs)
	}
}

func TestBuildUserGrowthDashboardWithDBCountsWeeklyGrowth(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Log{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	now := time.Date(2026, time.June, 17, 12, 0, 0, 0, time.UTC)
	currentStart := dashboardStartOfWeek(now)
	previousStart, _ := previousUserGrowthPeriod(currentStart, now, userGrowthGranularityWeek)
	oldStart := currentStart.AddDate(0, 0, -30)
	users := []model.User{
		{Id: "user-current", Username: "current", Password: "password123", AccessToken: "token-current", AffCode: "aff-current", Status: model.UserStatusEnabled, CreatedAt: currentStart.Add(time.Hour).Unix()},
		{Id: "user-existing", Username: "existing", Password: "password123", AccessToken: "token-existing", AffCode: "aff-existing", Status: model.UserStatusEnabled, CreatedAt: oldStart.Unix()},
		{Id: "user-previous", Username: "previous", Password: "password123", AccessToken: "token-previous", AffCode: "aff-previous", Status: model.UserStatusEnabled, CreatedAt: previousStart.Add(time.Hour).Unix()},
		{Id: "user-deleted", Username: "deleted", Password: "password123", AccessToken: "token-deleted", AffCode: "aff-deleted", Status: model.UserStatusDeleted, CreatedAt: currentStart.Add(2 * time.Hour).Unix()},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	logs := []model.Log{
		{Id: "log-current-1", UserId: "user-current", Type: model.LogTypeConsume, CreatedAt: currentStart.Add(2 * time.Hour).Unix()},
		{Id: "log-current-2", UserId: "user-current", Type: model.LogTypeConsume, CreatedAt: currentStart.Add(3 * time.Hour).Unix()},
		{Id: "log-current-3", UserId: "user-existing", Type: model.LogTypeConsume, CreatedAt: currentStart.Add(4 * time.Hour).Unix()},
		{Id: "topup-current-1", UserId: "user-current", Type: model.LogTypeTopup, CreatedAt: currentStart.Add(5 * time.Hour).Unix()},
		{Id: "topup-current-2", UserId: "user-existing", Type: model.LogTypeTopup, CreatedAt: currentStart.Add(6 * time.Hour).Unix()},
		{Id: "log-previous-1", UserId: "user-previous", Type: model.LogTypeConsume, CreatedAt: previousStart.Add(2 * time.Hour).Unix()},
		{Id: "topup-previous-1", UserId: "user-previous", Type: model.LogTypeTopup, CreatedAt: previousStart.Add(3 * time.Hour).Unix()},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("create logs: %v", err)
	}

	summary, trend, err := buildUserGrowthDashboardWithDB(db, db, "weekly", now)
	if err != nil {
		t.Fatalf("build user growth: %v", err)
	}
	if summary.Granularity != userGrowthGranularityWeek {
		t.Fatalf("Granularity=%q, want %q", summary.Granularity, userGrowthGranularityWeek)
	}
	if len(trend) != userGrowthTrendWeeks {
		t.Fatalf("trend len=%d, want %d", len(trend), userGrowthTrendWeeks)
	}
	if summary.Current.NewUserCount != 1 || summary.Previous.NewUserCount != 1 {
		t.Fatalf("new users current/previous=%d/%d, want 1/1", summary.Current.NewUserCount, summary.Previous.NewUserCount)
	}
	if summary.Current.ActiveUserCount != 2 || summary.Previous.ActiveUserCount != 1 {
		t.Fatalf("active users current/previous=%d/%d, want 2/1", summary.Current.ActiveUserCount, summary.Previous.ActiveUserCount)
	}
	if summary.Current.TopupUserCount != 2 || summary.Previous.TopupUserCount != 1 {
		t.Fatalf("topup users current/previous=%d/%d, want 2/1", summary.Current.TopupUserCount, summary.Previous.TopupUserCount)
	}
	if summary.Current.RequestCount != 3 {
		t.Fatalf("request count=%d, want 3", summary.Current.RequestCount)
	}
	if summary.ActiveUsers.Delta != 1 || !summary.ActiveUsers.HasBaseline {
		t.Fatalf("active comparison=%+v, want delta 1 with baseline", summary.ActiveUsers)
	}
	if math.Abs(summary.ActiveUsers.GrowthRate-1.0) > 0.0001 {
		t.Fatalf("active growth rate=%f, want 1.0", summary.ActiveUsers.GrowthRate)
	}
	last := trend[len(trend)-1]
	if last.NewUserCount != 1 || last.ActiveUserCount != 2 || last.TopupUserCount != 2 {
		t.Fatalf("last trend point=%+v, want current-period counts", last)
	}
}

func TestBuildUsageRankingIncludesUserBalance(t *testing.T) {
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	defer func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
	}()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Log{}, &model.UserBalanceLot{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	startAt := int64(100)
	endAt := int64(200)
	users := []model.User{
		{Id: "user-1", Username: "alice", Password: "password123", AccessToken: "token-1", AffCode: "aff-1", Status: model.UserStatusEnabled, CreatedAt: 10},
		{Id: "user-2", Username: "bob", Password: "password123", AccessToken: "token-2", AffCode: "aff-2", Status: model.UserStatusEnabled, CreatedAt: 11},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	lots := []model.UserBalanceLot{
		{Id: "lot-1", UserID: "user-1", SourceType: model.UserBalanceLotSourceTopup, SourceID: "topup-1", TotalAmount: 120, RemainingAmount: 120, Status: model.UserBalanceLotStatusActive, GrantedAt: 10, CreatedAt: 10, UpdatedAt: 10},
		{Id: "lot-2", UserID: "user-2", SourceType: model.UserBalanceLotSourceTopup, SourceID: "topup-2", TotalAmount: 50, RemainingAmount: 50, Status: model.UserBalanceLotStatusActive, GrantedAt: 11, CreatedAt: 11, UpdatedAt: 11},
	}
	if err := db.Create(&lots).Error; err != nil {
		t.Fatalf("create balance lots: %v", err)
	}
	logs := []model.Log{
		{Id: "log-1", UserId: "user-1", Type: model.LogTypeConsume, Quota: 30, CreatedAt: 120},
		{Id: "log-2", UserId: "user-2", Type: model.LogTypeConsume, Quota: 70, CreatedAt: 130},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("create logs: %v", err)
	}

	items, err := buildUsageRankingWithKeyword(startAt, endAt, 100, 0, "")
	if err != nil {
		t.Fatalf("build usage ranking: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len=%d, want 2: %+v", len(items), items)
	}
	if items[0].UserID != "user-2" || items[0].SpendAmount != 70 || items[0].BalanceAmount != 50 {
		t.Fatalf("first item=%+v, want user-2 with spend 70 and balance 50", items[0])
	}
	if items[1].UserID != "user-1" || items[1].SpendAmount != 30 || items[1].BalanceAmount != 120 {
		t.Fatalf("second item=%+v, want user-1 with spend 30 and balance 120", items[1])
	}
}
