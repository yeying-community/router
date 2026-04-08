package model

import "testing"

func TestDefaultTopupPlans(t *testing.T) {
	items := defaultTopupPlans("group-1")
	if len(items) != 5 {
		t.Fatalf("len(items) = %d, want 5", len(items))
	}
	if items[0].Amount != 1 || items[0].QuotaAmount != 20 {
		t.Fatalf("items[0] = %#v, want 1 CNY / 20 USD", items[0])
	}
}

func TestNormalizeTopupPlansFiltersInvalidAndNormalizesOrder(t *testing.T) {
	items := NormalizeTopupPlans([]TopupPlan{
		{Id: "", Name: "", Amount: 20, AmountCurrency: "cny", QuotaAmount: 500, QuotaCurrency: "usd", Enabled: true, SortOrder: 3},
		{Id: "", Name: "", Amount: 0, AmountCurrency: "cny", QuotaAmount: 1, QuotaCurrency: "usd", Enabled: true, SortOrder: 1},
		{Id: "", Name: "", Amount: 10, AmountCurrency: "cny", QuotaAmount: 220, QuotaCurrency: "usd", Enabled: true, SortOrder: 2},
	})

	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].SortOrder != 1 || items[1].SortOrder != 2 {
		t.Fatalf("sort orders = %#v, want sequential values", items)
	}
	if items[0].AmountCurrency != BillingCurrencyCodeCNY || items[0].QuotaCurrency != BillingCurrencyCodeUSD {
		t.Fatalf("normalized currencies = %#v", items[0])
	}
}
