package presenter

import (
	"testing"

	"github.com/yeying-community/router/internal/admin/model"
)

func TestNewUserAddsYYCAliases(t *testing.T) {
	row := &model.User{
		Id:                         "u1",
		Quota:                      1200,
		UsedQuota:                  3400,
		DailyQuotaLimit:            5600,
		MonthlyEmergencyQuotaLimit: 7800,
	}
	view := NewUser(row)
	if view == nil {
		t.Fatal("NewUser returned nil")
	}
	if view.YYCBalance != row.Quota {
		t.Fatalf("yyc_balance=%d, want %d", view.YYCBalance, row.Quota)
	}
	if view.YYCUsed != row.UsedQuota {
		t.Fatalf("yyc_used=%d, want %d", view.YYCUsed, row.UsedQuota)
	}
	if view.YYCDailyLimit != row.DailyQuotaLimit {
		t.Fatalf("yyc_daily_limit=%d, want %d", view.YYCDailyLimit, row.DailyQuotaLimit)
	}
	if view.YYCMonthlyEmergencyLimit != row.MonthlyEmergencyQuotaLimit {
		t.Fatalf("yyc_monthly_emergency_limit=%d, want %d", view.YYCMonthlyEmergencyLimit, row.MonthlyEmergencyQuotaLimit)
	}
}

func TestNewTokenAddsYYCAliases(t *testing.T) {
	row := &model.Token{
		Id:          "t1",
		RemainQuota: 900,
		UsedQuota:   100,
	}
	view := NewToken(row)
	if view == nil {
		t.Fatal("NewToken returned nil")
	}
	if view.YYCRemain != row.RemainQuota {
		t.Fatalf("yyc_remain=%d, want %d", view.YYCRemain, row.RemainQuota)
	}
	if view.YYCUsed != row.UsedQuota {
		t.Fatalf("yyc_used=%d, want %d", view.YYCUsed, row.UsedQuota)
	}
}

func TestNewRedemptionAddsYYCAlias(t *testing.T) {
	row := &model.Redemption{
		Id:    "r1",
		Quota: 2048,
	}
	view := NewRedemption(row)
	if view == nil {
		t.Fatal("NewRedemption returned nil")
	}
	if view.YYCValue != row.Quota {
		t.Fatalf("yyc_value=%d, want %d", view.YYCValue, row.Quota)
	}
}

func TestNewUserQuotaSummaryAddsYYCAliases(t *testing.T) {
	summary := model.UserQuotaSummary{
		UserID: "u1",
		Daily: model.UserDailyQuotaSnapshot{
			Limit:          100,
			ConsumedQuota:  20,
			ReservedQuota:  10,
			RemainingQuota: 70,
		},
		MonthlyEmergency: model.UserMonthlyEmergencyQuotaSnapshot{
			Limit:          200,
			ConsumedQuota:  30,
			ReservedQuota:  20,
			RemainingQuota: 150,
		},
	}
	view := NewUserQuotaSummary(summary)
	if view.Daily.YYCLimit != summary.Daily.Limit || view.Daily.YYCConsumed != summary.Daily.ConsumedQuota || view.Daily.YYCReserved != summary.Daily.ReservedQuota || view.Daily.YYCRemaining != summary.Daily.RemainingQuota {
		t.Fatalf("daily yyc aliases mismatch: %+v", view.Daily)
	}
	if view.MonthlyEmergency.YYCLimit != summary.MonthlyEmergency.Limit || view.MonthlyEmergency.YYCConsumed != summary.MonthlyEmergency.ConsumedQuota || view.MonthlyEmergency.YYCReserved != summary.MonthlyEmergency.ReservedQuota || view.MonthlyEmergency.YYCRemaining != summary.MonthlyEmergency.RemainingQuota {
		t.Fatalf("monthly emergency yyc aliases mismatch: %+v", view.MonthlyEmergency)
	}
}

func TestNewLogStatisticAddsYYCAlias(t *testing.T) {
	row := &model.LogStatistic{
		Day:              "2026-03-31",
		ModelName:        "gpt-5.4",
		RequestCount:     3,
		Quota:            2048,
		PromptTokens:     100,
		CompletionTokens: 200,
	}
	view := NewLogStatistic(row)
	if view == nil {
		t.Fatal("NewLogStatistic returned nil")
	}
	if view.YYCAmount != row.Quota {
		t.Fatalf("yyc_amount=%d, want %d", view.YYCAmount, row.Quota)
	}
}

func TestNewLogAddsYYCAliases(t *testing.T) {
	row := &model.Log{
		Id:                 "l1",
		Quota:              123,
		UserDailyQuota:     45,
		UserEmergencyQuota: 67,
	}
	view := NewLog(row)
	if view == nil {
		t.Fatal("NewLog returned nil")
	}
	if view.YYCAmount != row.Quota {
		t.Fatalf("yyc_amount=%d, want %d", view.YYCAmount, row.Quota)
	}
	if view.YYCUserDaily != row.UserDailyQuota {
		t.Fatalf("yyc_user_daily=%d, want %d", view.YYCUserDaily, row.UserDailyQuota)
	}
	if view.YYCUserEmergency != row.UserEmergencyQuota {
		t.Fatalf("yyc_user_emergency=%d, want %d", view.YYCUserEmergency, row.UserEmergencyQuota)
	}
}

func TestNewChannelAddsYYCAlias(t *testing.T) {
	row := &model.Channel{
		Id:        "c1",
		UsedQuota: 2048,
	}
	view := NewChannel(row)
	if view == nil {
		t.Fatal("NewChannel returned nil")
	}
	if view.YYCUsed != row.UsedQuota {
		t.Fatalf("yyc_used=%d, want %d", view.YYCUsed, row.UsedQuota)
	}
}

func TestNewGroupAddsYYCAlias(t *testing.T) {
	row := &model.GroupCatalog{
		Id:              "g1",
		DailyQuotaLimit: 4096,
	}
	view := NewGroup(row)
	if view == nil {
		t.Fatal("NewGroup returned nil")
	}
	if view.YYCDailyLimit != row.DailyQuotaLimit {
		t.Fatalf("yyc_daily_limit=%d, want %d", view.YYCDailyLimit, row.DailyQuotaLimit)
	}
}
