package presenter

import (
	"testing"

	"github.com/yeying-community/router/internal/admin/model"
)

func TestNewUserAddsAmountFields(t *testing.T) {
	row := &model.User{
		Id:        "u1",
		UsedQuota: 3400,
	}
	view := NewUser(row)
	if view == nil {
		t.Fatal("NewUser returned nil")
	}
	if view.BalanceAmount != 0 {
		t.Fatalf("balance_amount=%d, want 0", view.BalanceAmount)
	}
	if view.UsedAmount != row.UsedQuota {
		t.Fatalf("used_amount=%d, want %d", view.UsedAmount, row.UsedQuota)
	}
}

func TestNewTokenAddsAmountFields(t *testing.T) {
	row := &model.Token{
		Id:          "t1",
		Key:         "secret-token-1234",
		RemainQuota: 900,
		UsedQuota:   100,
	}
	view := NewToken(row)
	if view == nil {
		t.Fatal("NewToken returned nil")
	}
	if view.RemainingAmount != row.RemainQuota {
		t.Fatalf("remaining_amount=%d, want %d", view.RemainingAmount, row.RemainQuota)
	}
	if view.UsedAmount != row.UsedQuota {
		t.Fatalf("used_amount=%d, want %d", view.UsedAmount, row.UsedQuota)
	}
	if view.Key != row.Key {
		t.Fatalf("key=%q, want %q", view.Key, row.Key)
	}
}

func TestNewCreatedTokenKeepsKey(t *testing.T) {
	row := &model.Token{
		Id:          "t1",
		Key:         "secret",
		RemainQuota: 900,
		UsedQuota:   100,
	}
	view := NewCreatedToken(row)
	if view == nil {
		t.Fatal("NewCreatedToken returned nil")
	}
	if view.Key != row.Key {
		t.Fatalf("key=%q, want %q", view.Key, row.Key)
	}
}

func TestNewRedemptionAddsAmountField(t *testing.T) {
	row := &model.Redemption{
		Id:                  "r1",
		QuotaAmountSnapshot: 2048,
	}
	view := NewRedemption(row)
	if view == nil {
		t.Fatal("NewRedemption returned nil")
	}
	if view.CreditAmount != int64(row.QuotaAmountSnapshot) {
		t.Fatalf("credit_amount=%d, want %d", view.CreditAmount, int64(row.QuotaAmountSnapshot))
	}
}

func TestNewUserQuotaSummaryAddsAmountFields(t *testing.T) {
	summary := model.UserQuotaSummary{
		UserID: "u1",
		Daily: model.UserDailyQuotaSnapshot{
			Limit:          100,
			ConsumedQuota:  20,
			ReservedQuota:  10,
			RemainingQuota: 70,
		},
		PackageEmergency: model.UserPackageEmergencyQuotaSnapshot{
			Limit:          200,
			ConsumedQuota:  30,
			ReservedQuota:  20,
			RemainingQuota: 150,
		},
	}
	view := NewUserQuotaSummary(summary)
	if view.Daily.LimitAmount != summary.Daily.Limit || view.Daily.ConsumedAmount != summary.Daily.ConsumedQuota || view.Daily.ReservedAmount != summary.Daily.ReservedQuota || view.Daily.RemainingAmount != summary.Daily.RemainingQuota {
		t.Fatalf("daily amount fields mismatch: %+v", view.Daily)
	}
	if view.PackageEmergency.LimitAmount != summary.PackageEmergency.Limit || view.PackageEmergency.ConsumedAmount != summary.PackageEmergency.ConsumedQuota || view.PackageEmergency.ReservedAmount != summary.PackageEmergency.ReservedQuota || view.PackageEmergency.RemainingAmount != summary.PackageEmergency.RemainingQuota {
		t.Fatalf("package emergency amount fields mismatch: %+v", view.PackageEmergency)
	}
}

func TestNewLogStatisticAddsAmountField(t *testing.T) {
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
	if view.ChargeAmount != row.Quota {
		t.Fatalf("charge_amount=%d, want %d", view.ChargeAmount, row.Quota)
	}
}

func TestNewLogAddsAmountFields(t *testing.T) {
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
	if view.ChargeAmount != row.Quota {
		t.Fatalf("charge_amount=%d, want %d", view.ChargeAmount, row.Quota)
	}
	if view.UserDailyChargeAmount != row.UserDailyQuota {
		t.Fatalf("user_daily_charge_amount=%d, want %d", view.UserDailyChargeAmount, row.UserDailyQuota)
	}
	if view.UserEmergencyChargeAmount != row.UserEmergencyQuota {
		t.Fatalf("user_emergency_charge_amount=%d, want %d", view.UserEmergencyChargeAmount, row.UserEmergencyQuota)
	}
}

func TestNewChannelAddsAmountField(t *testing.T) {
	row := &model.Channel{
		Id:        "c1",
		UsedQuota: 2048,
	}
	view := NewChannel(row)
	if view == nil {
		t.Fatal("NewChannel returned nil")
	}
	if view.UsedAmount != row.UsedQuota {
		t.Fatalf("used_amount=%d, want %d", view.UsedAmount, row.UsedQuota)
	}
}

func TestNewGroupRemovesQuotaFields(t *testing.T) {
	row := &model.GroupCatalog{
		Id:           "g1",
		Name:         "enterprise",
		Description:  "desc",
		Source:       "manual",
		BillingRatio: 1.5,
		Enabled:      true,
		SortOrder:    3,
		UpdatedAt:    123,
	}
	view := NewGroup(row)
	if view == nil {
		t.Fatal("NewGroup returned nil")
	}
	if view.Id != row.Id || view.Name != row.Name || view.BillingRatio != row.BillingRatio {
		t.Fatalf("group fields mismatch: %+v", view)
	}
}
