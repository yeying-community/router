package model

import "testing"

func TestFilterActiveProviderModelDetails(t *testing.T) {
	details := []ProviderModelDetail{
		{Model: "gpt-5.5", Type: ProviderModelTypeText},
		{Model: "gpt-4.1", Type: ProviderModelTypeText, IsDeleted: true},
	}

	filtered := FilterActiveProviderModelDetails(details)
	if len(filtered) != 1 {
		t.Fatalf("FilterActiveProviderModelDetails len=%d, want 1", len(filtered))
	}
	if filtered[0].Model != "gpt-5.5" {
		t.Fatalf("FilterActiveProviderModelDetails model=%q, want gpt-5.5", filtered[0].Model)
	}
}
