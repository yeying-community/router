package model

import (
	"testing"

	relaychannel "github.com/yeying-community/router/internal/relay/channel"
)

func float64Ptr(value float64) *float64 {
	return &value
}

func TestBuildDefaultChannelModelConfigsWithProtocol_AnthropicPrefersMessages(t *testing.T) {
	rows := BuildDefaultChannelModelConfigsWithProtocol([]string{"claude-sonnet-4-6"}, relaychannel.Anthropic)
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].Endpoint != ChannelModelEndpointMessages {
		t.Fatalf("rows[0].Endpoint = %q, want %q", rows[0].Endpoint, ChannelModelEndpointMessages)
	}
	if len(rows[0].Endpoints) == 0 || rows[0].Endpoints[0] != ChannelModelEndpointMessages {
		t.Fatalf("rows[0].Endpoints = %#v, want first %q", rows[0].Endpoints, ChannelModelEndpointMessages)
	}
}

func TestNormalizeChannelModelRow_PreservesExplicitPrimaryEndpoint(t *testing.T) {
	row := ChannelModel{
		Model:         "gpt-5.1",
		UpstreamModel: "gpt-5.1",
		Type:          ProviderModelTypeText,
		Endpoint:      ChannelModelEndpointChat,
		Endpoints: []string{
			ChannelModelEndpointResponses,
			ChannelModelEndpointChat,
		},
	}
	normalizeChannelModelRow(&row)
	if row.Endpoint != ChannelModelEndpointChat {
		t.Fatalf("row.Endpoint = %q, want %q", row.Endpoint, ChannelModelEndpointChat)
	}
	if len(row.Endpoints) != 2 {
		t.Fatalf("len(row.Endpoints) = %d, want 2", len(row.Endpoints))
	}
}

func TestCompleteChannelModelRowDefaults_PreservesExplicitPrimaryEndpoint(t *testing.T) {
	row := ChannelModel{
		Model:         "gpt-5.1",
		UpstreamModel: "gpt-5.1",
		Type:          ProviderModelTypeText,
		Endpoint:      ChannelModelEndpointChat,
		Endpoints: []string{
			ChannelModelEndpointResponses,
			ChannelModelEndpointChat,
		},
	}
	completeChannelModelRowDefaults(&row, relaychannel.OpenAI)
	if row.Endpoint != ChannelModelEndpointChat {
		t.Fatalf("row.Endpoint = %q, want %q", row.Endpoint, ChannelModelEndpointChat)
	}
	if len(row.Endpoints) != 2 {
		t.Fatalf("len(row.Endpoints) = %d, want 2", len(row.Endpoints))
	}
}

func TestNormalizeChannelModelRow_UsesFirstEndpointWhenPrimaryMissing(t *testing.T) {
	row := ChannelModel{
		Model:         "gpt-5.1",
		UpstreamModel: "gpt-5.1",
		Type:          ProviderModelTypeText,
		Endpoint:      "",
		Endpoints: []string{
			ChannelModelEndpointChat,
			ChannelModelEndpointResponses,
		},
	}
	normalizeChannelModelRow(&row)
	if row.Endpoint != ChannelModelEndpointChat {
		t.Fatalf("row.Endpoint = %q, want %q", row.Endpoint, ChannelModelEndpointChat)
	}
}

func TestApplyChannelModelEndpointState_PreservesExplicitPrimaryEvenIfDisabled(t *testing.T) {
	row := ChannelModel{
		Model:         "gpt-5.1",
		UpstreamModel: "gpt-5.1",
		Type:          ProviderModelTypeText,
		Endpoint:      ChannelModelEndpointChat,
		Endpoints: []string{
			ChannelModelEndpointChat,
			ChannelModelEndpointResponses,
		},
	}
	state := channelModelEndpointState{
		Endpoints: []string{
			ChannelModelEndpointChat,
			ChannelModelEndpointResponses,
		},
		Enabled: map[string]bool{
			ChannelModelEndpointChat:      false,
			ChannelModelEndpointResponses: true,
		},
	}
	applyChannelModelEndpointState(&row, state)
	if row.Endpoint != ChannelModelEndpointChat {
		t.Fatalf("row.Endpoint = %q, want %q", row.Endpoint, ChannelModelEndpointChat)
	}
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

func TestBuildDisabledChannelModelConfigsMarksOnlyTargetModelInactive(t *testing.T) {
	rows := []ChannelModel{
		{
			Model:         "gpt-5.3-codex",
			UpstreamModel: "gpt-5.3-codex",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
		{
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
	}

	updated, changed := buildDisabledChannelModelConfigs(rows, "gpt-5.3-codex")
	if !changed {
		t.Fatalf("changed = false, want true")
	}
	if len(updated) != 2 {
		t.Fatalf("updated len = %d, want 2", len(updated))
	}
	if !updated[0].Inactive {
		t.Fatalf("updated[0].Inactive = false, want true")
	}
	if updated[0].Selected {
		t.Fatalf("updated[0].Selected = true, want false")
	}
	if updated[1].Inactive {
		t.Fatalf("updated[1].Inactive = true, want false")
	}
	if !updated[1].Selected {
		t.Fatalf("updated[1].Selected = false, want true")
	}
}

func TestBuildDisabledChannelModelConfigsNoopWhenTargetMissing(t *testing.T) {
	rows := []ChannelModel{
		{
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
	}

	updated, changed := buildDisabledChannelModelConfigs(rows, "gpt-5.3-codex")
	if changed {
		t.Fatalf("changed = true, want false")
	}
	if len(updated) != 1 {
		t.Fatalf("updated len = %d, want 1", len(updated))
	}
	if updated[0].Inactive {
		t.Fatalf("updated[0].Inactive = true, want false")
	}
	if !updated[0].Selected {
		t.Fatalf("updated[0].Selected = false, want true")
	}
}
