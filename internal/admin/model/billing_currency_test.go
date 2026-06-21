package model

import (
	"testing"

	"github.com/yeying-community/router/common/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNormalizeBillingCurrencyCode(t *testing.T) {
	tests := map[string]string{
		"":    BillingCurrencyCodeUSD,
		"usd": BillingCurrencyCodeUSD,
		"USD": BillingCurrencyCodeUSD,
		"rmb": BillingCurrencyCodeCNY,
		"cny": BillingCurrencyCodeCNY,
		"eur": "EUR",
	}
	for input, want := range tests {
		if got := normalizeBillingCurrencyCode(input); got != want {
			t.Fatalf("normalizeBillingCurrencyCode(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestDefaultBillingCurrenciesUsesQuotaPerUnit(t *testing.T) {
	origin := config.QuotaPerUnit
	config.QuotaPerUnit = 600000
	defer func() {
		config.QuotaPerUnit = origin
	}()

	rows := defaultBillingCurrencies()
	if len(rows) != 3 {
		t.Fatalf("defaultBillingCurrencies len=%d, want 3", len(rows))
	}

	byCode := make(map[string]BillingCurrency, len(rows))
	for _, row := range rows {
		byCode[row.Code] = row
	}

	if got := byCode[BillingCurrencyCodeYYC].ChargeRate; got != defaultYYCChargeRate {
		t.Fatalf("YYC charge_rate=%v, want %v", got, defaultYYCChargeRate)
	}
	if got := byCode[BillingCurrencyCodeYYC].MinorUnit; got != defaultYYCMinorUnit {
		t.Fatalf("YYC minor_unit=%v, want %v", got, defaultYYCMinorUnit)
	}
	if got := byCode[BillingCurrencyCodeUSD].ChargeRate; got != 600000 {
		t.Fatalf("USD charge_rate=%v, want 600000", got)
	}
	if got := byCode[BillingCurrencyCodeUSD].MinorUnit; got != defaultFiatMinorUnit {
		t.Fatalf("USD minor_unit=%v, want %v", got, defaultFiatMinorUnit)
	}
	if got := byCode[BillingCurrencyCodeCNY].ChargeRate; got != defaultCNYChargeRate {
		t.Fatalf("CNY charge_rate=%v, want %v", got, defaultCNYChargeRate)
	}
	if got := byCode[BillingCurrencyCodeCNY].MinorUnit; got != defaultFiatMinorUnit {
		t.Fatalf("CNY minor_unit=%v, want %v", got, defaultFiatMinorUnit)
	}
}

func TestMigrateBillingCurrencyChargeRateCopiesLegacyYYCPerUnit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE billing_currencies (
			code varchar(16) PRIMARY KEY,
			name varchar(64) NOT NULL DEFAULT '',
			symbol varchar(16) NOT NULL DEFAULT '',
			minor_unit bigint NOT NULL DEFAULT 6,
			yyc_per_unit double precision NOT NULL DEFAULT 0,
			status bigint NOT NULL DEFAULT 1,
			source varchar(64) NOT NULL DEFAULT 'system_default',
			created_at bigint NOT NULL DEFAULT 0,
			updated_at bigint NOT NULL DEFAULT 0
		)
	`).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO billing_currencies
			(code, name, symbol, minor_unit, yyc_per_unit, status, source, created_at, updated_at)
		VALUES
			('USD', 'US Dollar', '$', 6, 500000, 1, 'system_default', 1, 1),
			('CNY', 'Chinese Yuan', '¥', 6, 72000, 1, 'manual', 1, 1),
			('YYC', 'Yeying Coin', 'Ɏ', 0, 1, 1, 'system_default', 1, 1)
	`).Error; err != nil {
		t.Fatalf("insert legacy rows: %v", err)
	}

	if err := migrateBillingCurrencyChargeRateWithDB(db); err != nil {
		t.Fatalf("migrate charge rate: %v", err)
	}

	rows := make([]struct {
		Code       string
		ChargeRate float64
	}, 0)
	if err := db.Raw("SELECT code, charge_rate FROM billing_currencies ORDER BY code").Scan(&rows).Error; err != nil {
		t.Fatalf("load rows: %v", err)
	}
	byCode := make(map[string]float64, len(rows))
	for _, row := range rows {
		byCode[row.Code] = row.ChargeRate
	}
	if got := byCode[BillingCurrencyCodeUSD]; got != 500000 {
		t.Fatalf("USD charge_rate=%v, want 500000", got)
	}
	if got := byCode[BillingCurrencyCodeCNY]; got != 72000 {
		t.Fatalf("CNY charge_rate=%v, want 72000", got)
	}
	if got := byCode[BillingCurrencyCodeYYC]; got != 1 {
		t.Fatalf("YYC charge_rate=%v, want 1", got)
	}
}
