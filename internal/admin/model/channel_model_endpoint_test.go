package model

import "testing"

func TestResolveChannelModelCapabilityEndpointsForResponsesTextIncludesChatAndResponses(t *testing.T) {
	row := ChannelModel{
		Model:         "gpt-5.4",
		UpstreamModel: "gpt-5.4",
		Type:          ProviderModelTypeText,
		Selected:      true,
		Endpoint:      ChannelModelEndpointResponses,
	}

	got := ResolveChannelModelCapabilityEndpoints(row)
	if len(got) != 2 {
		t.Fatalf("ResolveChannelModelCapabilityEndpoints len = %d, want 2", len(got))
	}
	if got[0] != ChannelModelEndpointChat {
		t.Fatalf("got[0] = %q, want %q", got[0], ChannelModelEndpointChat)
	}
	if got[1] != ChannelModelEndpointResponses {
		t.Fatalf("got[1] = %q, want %q", got[1], ChannelModelEndpointResponses)
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

func TestBuildChannelModelEndpointRowsPreservesExistingDisabledEndpointState(t *testing.T) {
	existing := []ChannelModelEndpoint{
		{ChannelId: "channel-1", Model: "gpt-5.4", Endpoint: ChannelModelEndpointChat, Enabled: false},
		{ChannelId: "channel-1", Model: "gpt-5.4", Endpoint: ChannelModelEndpointResponses, Enabled: true},
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
	if len(got) != 2 {
		t.Fatalf("BuildChannelModelEndpointRows len = %d, want 2", len(got))
	}
	if got[0].Endpoint != ChannelModelEndpointChat || got[0].Enabled {
		t.Fatalf("chat endpoint = (%q, %t), want (%q, false)", got[0].Endpoint, got[0].Enabled, ChannelModelEndpointChat)
	}
	if got[1].Endpoint != ChannelModelEndpointResponses || !got[1].Enabled {
		t.Fatalf("responses endpoint = (%q, %t), want (%q, true)", got[1].Endpoint, got[1].Enabled, ChannelModelEndpointResponses)
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

	if !IsChannelModelRequestEndpointSupportedByConfigs(rows, "gpt-5.4", ChannelModelEndpointChat) {
		t.Fatalf("chat endpoint support = false, want true")
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

func TestNormalizeRequestedChannelModelEndpointMessagesMapsToChat(t *testing.T) {
	if got := NormalizeRequestedChannelModelEndpoint("/v1/messages"); got != ChannelModelEndpointChat {
		t.Fatalf("NormalizeRequestedChannelModelEndpoint(/v1/messages)=%q, want %q", got, ChannelModelEndpointChat)
	}
	if got := NormalizeRequestedChannelModelEndpoint("/api/v1/public/messages"); got != ChannelModelEndpointChat {
		t.Fatalf("NormalizeRequestedChannelModelEndpoint(/api/v1/public/messages)=%q, want %q", got, ChannelModelEndpointChat)
	}
}
