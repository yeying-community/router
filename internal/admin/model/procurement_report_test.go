package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// procurementReportCNYChargeRate must return the catalog's YYC-per-CNY rate so
// CNY base cost/profit can be expressed in the YYC accounting unit; it returns 0
// (callers fall back to CNY) when the rate is unavailable.
func TestProcurementReportCNYChargeRate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&BillingCurrency{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := syncDefaultBillingCurrenciesWithDB(db); err != nil {
		t.Fatalf("seed currencies: %v", err)
	}
	if err := SyncBillingCurrencyCatalogWithDB(db); err != nil {
		t.Fatalf("sync catalog cache: %v", err)
	}

	want, err := GetBillingCurrencyChargeRate(BillingCurrencyCodeCNY)
	if err != nil {
		t.Fatalf("get CNY charge rate: %v", err)
	}
	if got := procurementReportCNYChargeRate(); got != want || got <= 0 {
		t.Fatalf("procurementReportCNYChargeRate() = %v, want %v (>0)", got, want)
	}

	// A CNY base amount expressed in YYC round-trips back through the rate.
	const costBaseCNY = 1.5
	costYYC := costBaseCNY * procurementReportCNYChargeRate()
	if costYYC <= 0 {
		t.Fatalf("cost in YYC = %v, want > 0", costYYC)
	}
}
