package model

import "testing"

func TestResolveChannelModelCapabilityEndpointsForResponsesTextIncludesResponsesOnly(t *testing.T) {
	row := ChannelModel{
		Model:         "gpt-5.4",
		UpstreamModel: "gpt-5.4",
		Type:          ProviderModelTypeText,
		Selected:      true,
		Endpoint:      ChannelModelEndpointResponses,
	}

	got := ResolveChannelModelCapabilityEndpoints(row)
	if len(got) != 1 {
		t.Fatalf("ResolveChannelModelCapabilityEndpoints len = %d, want 1", len(got))
	}
	if got[0] != ChannelModelEndpointResponses {
		t.Fatalf("got[0] = %q, want %q", got[0], ChannelModelEndpointResponses)
	}
}

func TestResolveChannelModelCapabilityEndpointsForAudioUsesGenericAudioCapability(t *testing.T) {
	row := ChannelModel{
		Model:         "gpt-4o-mini-transcribe",
		UpstreamModel: "gpt-4o-mini-transcribe",
		Type:          ProviderModelTypeAudio,
		Selected:      true,
		Endpoint:      "/v1/audio/transcriptions",
	}

	got := ResolveChannelModelCapabilityEndpoints(row)
	if len(got) != 1 {
		t.Fatalf("ResolveChannelModelCapabilityEndpoints len = %d, want 1", len(got))
	}
	if got[0] != ChannelModelEndpointAudio {
		t.Fatalf("got[0] = %q, want %q", got[0], ChannelModelEndpointAudio)
	}
}

func TestResolveChannelModelCapabilityEndpointsForMessagesTextIncludesMessagesOnly(t *testing.T) {
	row := ChannelModel{
		Model:         "claude-sonnet-4-6",
		UpstreamModel: "claude-sonnet-4-6",
		Type:          ProviderModelTypeText,
		Selected:      true,
		Endpoint:      ChannelModelEndpointMessages,
	}

	got := ResolveChannelModelCapabilityEndpoints(row)
	if len(got) != 1 {
		t.Fatalf("ResolveChannelModelCapabilityEndpoints len = %d, want 1", len(got))
	}
	if got[0] != ChannelModelEndpointMessages {
		t.Fatalf("got[0] = %q, want %q", got[0], ChannelModelEndpointMessages)
	}
}

func TestBuildChannelModelEndpointRowsPreservesExistingDisabledEndpointState(t *testing.T) {
	existing := []ChannelModelEndpoint{
		{ChannelId: "channel-1", Model: "gpt-5.4", Endpoint: ChannelModelEndpointResponses, Enabled: false},
	}
	rows := []ChannelModel{
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Type:          ProviderModelTypeText,
			Selected:      true,
			Endpoint:      ChannelModelEndpointResponses,
		},
	}

	got := BuildChannelModelEndpointRows(existing, rows)
	if len(got) != 1 {
		t.Fatalf("BuildChannelModelEndpointRows len = %d, want 1", len(got))
	}
	if got[0].Endpoint != ChannelModelEndpointResponses || got[0].Enabled {
		t.Fatalf("responses endpoint = (%q, %t), want (%q, false)", got[0].Endpoint, got[0].Enabled, ChannelModelEndpointResponses)
	}
}

func TestBuildChannelModelEndpointRowsUsesExplicitDirectEndpoints(t *testing.T) {
	rows := []ChannelModel{
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Type:          ProviderModelTypeText,
			Selected:      true,
			Endpoint:      ChannelModelEndpointResponses,
			Endpoints: []string{
				ChannelModelEndpointChat,
				ChannelModelEndpointResponses,
			},
		},
	}
	got := BuildChannelModelEndpointRows(nil, rows)
	if len(got) != 2 {
		t.Fatalf("len(got)=%d, want 2", len(got))
	}
	if got[0].Endpoint != ChannelModelEndpointChat {
		t.Fatalf("got[0].Endpoint=%q, want %q", got[0].Endpoint, ChannelModelEndpointChat)
	}
	if got[1].Endpoint != ChannelModelEndpointResponses {
		t.Fatalf("got[1].Endpoint=%q, want %q", got[1].Endpoint, ChannelModelEndpointResponses)
	}
}

func TestIsChannelModelRequestEndpointSupportedByConfigs(t *testing.T) {
	rows := []ChannelModel{
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Type:          ProviderModelTypeText,
			Selected:      true,
			Endpoint:      ChannelModelEndpointResponses,
		},
	}

	if IsChannelModelRequestEndpointSupportedByConfigs(rows, "gpt-5.4", ChannelModelEndpointChat) {
		t.Fatalf("chat endpoint support = true, want false")
	}
	if !IsChannelModelRequestEndpointSupportedByConfigs(rows, "gpt-5.4", ChannelModelEndpointResponses) {
		t.Fatalf("responses endpoint support = false, want true")
	}
	if IsChannelModelRequestEndpointSupportedByConfigs(rows, "gpt-5.4", ChannelModelEndpointImages) {
		t.Fatalf("images endpoint support = true, want false")
	}
}

func TestBuildDisabledChannelModelEndpointRowsMarksOnlyTargetEndpoint(t *testing.T) {
	rows := []ChannelModelEndpoint{
		{ChannelId: "channel-1", Model: "gpt-5.4", Endpoint: ChannelModelEndpointChat, Enabled: true},
		{ChannelId: "channel-1", Model: "gpt-5.4", Endpoint: ChannelModelEndpointResponses, Enabled: true},
	}

	got, changed := buildDisabledChannelModelEndpointRows(rows, "channel-1", "gpt-5.4", ChannelModelEndpointResponses)
	if !changed {
		t.Fatalf("changed = false, want true")
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if !got[0].Enabled {
		t.Fatalf("chat endpoint enabled = false, want true")
	}
	if got[1].Enabled {
		t.Fatalf("responses endpoint enabled = true, want false")
	}
}

func TestBuildChannelModelEndpointRowsByTestsUpdatesEndpointSupport(t *testing.T) {
	rows := []ChannelModelEndpoint{
		{ChannelId: "channel-1", Model: "gpt-5.4", Endpoint: ChannelModelEndpointChat, Enabled: true},
		{ChannelId: "channel-1", Model: "gpt-5.4", Endpoint: ChannelModelEndpointResponses, Enabled: true},
	}
	tests := []ChannelTest{
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.4",
			Endpoint:      ChannelModelEndpointResponses,
			Status:        ChannelTestStatusUnsupported,
			Supported:     false,
			UpstreamModel: "gpt-5.4",
		},
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.4",
			Endpoint:      ChannelModelEndpointMessages,
			Status:        ChannelTestStatusSupported,
			Supported:     true,
			UpstreamModel: "gpt-5.4",
		},
	}

	got, changed := buildChannelModelEndpointRowsByTests(rows, "channel-1", tests)
	if !changed {
		t.Fatalf("changed = false, want true")
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	gotMap := map[string]bool{}
	for _, row := range got {
		gotMap[row.Model+"::"+row.Endpoint] = row.Enabled
	}
	if enabled := gotMap["gpt-5.4::"+ChannelModelEndpointChat]; !enabled {
		t.Fatalf("chat endpoint enabled = false, want true")
	}
	if enabled := gotMap["gpt-5.4::"+ChannelModelEndpointResponses]; enabled {
		t.Fatalf("responses endpoint enabled = true, want false")
	}
	if enabled := gotMap["gpt-5.4::"+ChannelModelEndpointMessages]; !enabled {
		t.Fatalf("messages endpoint enabled = false, want true")
	}
}

func TestNormalizeRequestedChannelModelEndpointMessagesMapsToMessages(t *testing.T) {
	if got := NormalizeRequestedChannelModelEndpoint("/v1/messages"); got != ChannelModelEndpointMessages {
		t.Fatalf("NormalizeRequestedChannelModelEndpoint(/v1/messages)=%q, want %q", got, ChannelModelEndpointMessages)
	}
	if got := NormalizeRequestedChannelModelEndpoint("/api/v1/public/messages"); got != ChannelModelEndpointMessages {
		t.Fatalf("NormalizeRequestedChannelModelEndpoint(/api/v1/public/messages)=%q, want %q", got, ChannelModelEndpointMessages)
	}
}

func TestIsChannelModelRequestEndpointSupportedByEndpointMapMessagesNoLongerBackCompat(t *testing.T) {
	endpointMap := map[string]bool{
		ChannelModelEndpointChat: true,
	}
	supported, explicit := IsChannelModelRequestEndpointSupportedByEndpointMap(endpointMap, ChannelModelEndpointMessages)
	if !explicit || supported {
		t.Fatalf("messages support from legacy chat endpoint = (%t, %t), want (false, true)", supported, explicit)
	}
}

func TestIsChannelModelRequestEndpointSupportedByEndpointMapNoBridgeCompatibility(t *testing.T) {
	endpointMap := map[string]bool{
		ChannelModelEndpointResponses: true,
	}
	if supported, explicit := IsChannelModelRequestEndpointSupportedByEndpointMap(endpointMap, ChannelModelEndpointChat); !explicit || supported {
		t.Fatalf("chat request support via responses endpoint = (%t, %t), want (false, true)", supported, explicit)
	}
	if supported, explicit := IsChannelModelRequestEndpointSupportedByEndpointMap(endpointMap, ChannelModelEndpointMessages); !explicit || supported {
		t.Fatalf("messages request support via responses endpoint = (%t, %t), want (false, true)", supported, explicit)
	}
}
