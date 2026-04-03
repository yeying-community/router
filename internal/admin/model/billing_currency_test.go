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

	if got := byCode[BillingCurrencyCodeYYC].YYCPerUnit; got != defaultYYCYYCPerUnit {
		t.Fatalf("YYC yyc_per_unit=%v, want %v", got, defaultYYCYYCPerUnit)
	}
	if got := byCode[BillingCurrencyCodeYYC].MinorUnit; got != defaultYYCMinorUnit {
		t.Fatalf("YYC minor_unit=%v, want %v", got, defaultYYCMinorUnit)
	}
	if got := byCode[BillingCurrencyCodeUSD].YYCPerUnit; got != 600000 {
		t.Fatalf("USD yyc_per_unit=%v, want 600000", got)
	}
	if got := byCode[BillingCurrencyCodeUSD].MinorUnit; got != defaultFiatMinorUnit {
		t.Fatalf("USD minor_unit=%v, want %v", got, defaultFiatMinorUnit)
	}
	if got := byCode[BillingCurrencyCodeCNY].YYCPerUnit; got != defaultCNYYYCPerUnit {
		t.Fatalf("CNY yyc_per_unit=%v, want %v", got, defaultCNYYYCPerUnit)
	}
	if got := byCode[BillingCurrencyCodeCNY].MinorUnit; got != defaultFiatMinorUnit {
		t.Fatalf("CNY minor_unit=%v, want %v", got, defaultFiatMinorUnit)
	}
}
