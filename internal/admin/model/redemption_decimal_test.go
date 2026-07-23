package model

import (
	"math"
	"testing"

	"github.com/shopspring/decimal"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRedemptionQuotaAmountInt64PreservesLargeExactAmount(t *testing.T) {
	amount := decimal.RequireFromString("9007199254740993")
	actual, err := RedemptionQuotaAmountInt64(amount)
	if err != nil {
		t.Fatalf("RedemptionQuotaAmountInt64: %v", err)
	}
	if actual != 9007199254740993 {
		t.Fatalf("amount=%d, want 9007199254740993", actual)
	}
}

func TestRedemptionQuotaAmountInt64AcceptsMaxInt64(t *testing.T) {
	actual, err := RedemptionQuotaAmountInt64(decimal.NewFromInt(math.MaxInt64))
	if err != nil {
		t.Fatalf("RedemptionQuotaAmountInt64: %v", err)
	}
	if actual != math.MaxInt64 {
		t.Fatalf("amount=%d, want %d", actual, int64(math.MaxInt64))
	}
}

func TestRedemptionQuotaAmountInt64RejectsFractionAndOverflow(t *testing.T) {
	for _, amount := range []decimal.Decimal{
		decimal.RequireFromString("1.5"),
		decimal.RequireFromString("9223372036854775808"),
	} {
		if _, err := RedemptionQuotaAmountInt64(amount); err == nil {
			t.Fatalf("RedemptionQuotaAmountInt64(%s) unexpectedly succeeded", amount)
		}
	}
}

func TestRedemptionQuotaAmountSnapshotRoundTripsWithoutFloatPrecisionLoss(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Redemption{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	want := decimal.RequireFromString("9007199254740993")
	if err := db.Create(&Redemption{Id: "redemption-decimal", Code: "decimal-code", QuotaAmountSnapshot: want}).Error; err != nil {
		t.Fatalf("create redemption: %v", err)
	}
	var actual Redemption
	if err := db.First(&actual, "id = ?", "redemption-decimal").Error; err != nil {
		t.Fatalf("load redemption: %v", err)
	}
	if !actual.QuotaAmountSnapshot.Equal(want) {
		t.Fatalf("snapshot=%s, want %s", actual.QuotaAmountSnapshot, want)
	}
}
