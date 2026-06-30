package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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

func TestListTopupPlansIncludesSupportedModels(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&GroupCatalog{}, &GroupModel{}, &TopupPlan{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Create(&GroupCatalog{Id: "group-1", Name: "default", Enabled: true}).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if err := db.Create(&[]GroupModel{
		{Group: "group-1", Model: "qwen-plus", Enabled: true},
		{Group: "group-1", Model: "qwen-max", Enabled: true},
		{Group: "group-1", Model: "qwen-disabled", Enabled: false},
	}).Error; err != nil {
		t.Fatalf("seed group models: %v", err)
	}
	if _, err := createTopupPlanWithDB(db, TopupPlan{
		Name:           "10 CNY",
		GroupID:        "group-1",
		Amount:         10,
		AmountCurrency: BillingCurrencyCodeCNY,
		QuotaAmount:    20,
		QuotaCurrency:  BillingCurrencyCodeUSD,
		Enabled:        true,
		PublicVisible:  true,
	}); err != nil {
		t.Fatalf("createTopupPlanWithDB returned error: %v", err)
	}

	rows, err := listTopupPlansWithDB(db, true)
	if err != nil {
		t.Fatalf("listTopupPlansWithDB returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows)=%d, want 1", len(rows))
	}
	want := []string{"qwen-max", "qwen-plus"}
	if len(rows[0].SupportedModels) != len(want) {
		t.Fatalf("supported_models=%#v, want %#v", rows[0].SupportedModels, want)
	}
	for i := range want {
		if rows[0].SupportedModels[i] != want[i] {
			t.Fatalf("supported_models=%#v, want %#v", rows[0].SupportedModels, want)
		}
	}
}
