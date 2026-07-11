package billing

import (
	"testing"

	adminmodel "github.com/yeying-community/router/internal/admin/model"
)

func TestBuildPostConsumeLogEntryUsesBillingQuantity(t *testing.T) {
	snapshot := BillingSnapshot{
		InputQuantity: 123,
		ChargeAmount:  98765,
	}
	entry := buildPostConsumeLogEntry(
		"user-1",
		"group-1",
		"channel-1",
		"whisper-1",
		"token",
		98765,
		true,
		0,
		0,
		adminmodel.ResolvedModelPricing{
			Model:     "whisper-1",
			PriceUnit: adminmodel.ProviderPriceUnitPer1KTokens,
			Currency:  adminmodel.ProviderPriceCurrencyUSD,
		},
		1,
		snapshot,
		adminmodel.LogBillingSourceSnapshot{},
		adminmodel.LogBillingSourceSnapshot{},
	)
	if entry.PromptTokens != 123 {
		t.Fatalf("PromptTokens = %d, want billing quantity 123", entry.PromptTokens)
	}
	if entry.Quota != 98765 {
		t.Fatalf("Quota = %d, want YYC quota 98765", entry.Quota)
	}
}
