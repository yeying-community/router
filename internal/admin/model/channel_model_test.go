package model

import "testing"

func float64Ptr(value float64) *float64 {
	return &value
}

func TestBuildFetchedChannelModelConfigsPreservesExistingSelectionsAndMarksMissingRowsInactive(t *testing.T) {
	existingRows := []ChannelModel{
		{
			Model:         "alias-gpt-4.1",
			UpstreamModel: "gpt-4.1",
			Type:          ProviderModelTypeText,
			Selected:      false,
			InputPrice:    float64Ptr(1.2),
			OutputPrice:   float64Ptr(2.4),
			PriceUnit:     "per_1k_tokens",
			Currency:      "USD",
		},
		{
			Model:         "legacy-removed",
			UpstreamModel: "legacy-removed",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
	}

	fetchedRows := []ChannelModel{
		{
			Model:         "gpt-4.1",
			UpstreamModel: "gpt-4.1",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
		{
			Model:         "gpt-image-1",
			UpstreamModel: "gpt-image-1",
			Type:          ProviderModelTypeImage,
			Selected:      true,
		},
	}

	rows := BuildFetchedChannelModelConfigs(existingRows, fetchedRows, 0, false)
	if len(rows) != 3 {
		t.Fatalf("BuildFetchedChannelModelConfigs returned %d rows, want 3", len(rows))
	}

	if rows[0].Model != "alias-gpt-4.1" {
		t.Fatalf("rows[0].Model = %q, want alias-gpt-4.1", rows[0].Model)
	}
	if rows[0].UpstreamModel != "gpt-4.1" {
		t.Fatalf("rows[0].UpstreamModel = %q, want gpt-4.1", rows[0].UpstreamModel)
	}
	if rows[0].Selected {
		t.Fatalf("rows[0].Selected = true, want false")
	}
	if rows[0].InputPrice == nil || *rows[0].InputPrice != 1.2 {
		t.Fatalf("rows[0].InputPrice = %#v, want 1.2", rows[0].InputPrice)
	}
	if rows[0].OutputPrice == nil || *rows[0].OutputPrice != 2.4 {
		t.Fatalf("rows[0].OutputPrice = %#v, want 2.4", rows[0].OutputPrice)
	}
	if rows[0].Inactive {
		t.Fatalf("rows[0].Inactive = true, want false")
	}

	if rows[1].Model != "gpt-image-1" {
		t.Fatalf("rows[1].Model = %q, want gpt-image-1", rows[1].Model)
	}
	if rows[1].Selected {
		t.Fatalf("rows[1].Selected = true, want false")
	}
	if rows[1].Inactive {
		t.Fatalf("rows[1].Inactive = true, want false")
	}

	if rows[2].Model != "legacy-removed" {
		t.Fatalf("rows[2].Model = %q, want legacy-removed", rows[2].Model)
	}
	if rows[2].Selected {
		t.Fatalf("rows[2].Selected = true, want false")
	}
	if !rows[2].Inactive {
		t.Fatalf("rows[2].Inactive = false, want true")
	}
}

func TestApplyChannelTestResultsToModelConfigsAppliesSupportDecision(t *testing.T) {
	rows := []ChannelModel{
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.1",
			UpstreamModel: "gpt-5.1",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
	}
	results := []ChannelTest{
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.1",
			UpstreamModel: "gpt-5.1",
			Type:          ProviderModelTypeText,
			Endpoint:      ChannelModelEndpointResponses,
			Status:        ChannelTestStatusSupported,
			Supported:     true,
		},
	}

	updated := ApplyChannelTestResultsToModelConfigs(rows, results)
	if len(updated) != 1 {
		t.Fatalf("ApplyChannelTestResultsToModelConfigs returned %d rows, want 1", len(updated))
	}
	if !updated[0].Selected {
		t.Fatalf("updated[0].Selected = false, want true")
	}
}
