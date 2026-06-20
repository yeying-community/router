package model

import (
	"testing"

	"github.com/yeying-community/router/common/config"
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
