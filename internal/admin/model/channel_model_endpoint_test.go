package model

import "testing"

func TestNormalizeProviderModelSupportedEndpointsFiltersByModelType(t *testing.T) {
	got := NormalizeProviderModelSupportedEndpoints(ProviderModelTypeText, []string{
		ChannelModelEndpointResponses,
		ChannelModelEndpointImages,
		ChannelModelEndpointChat,
		ChannelModelEndpointResponses,
	})
	if len(got) != 2 || got[0] != ChannelModelEndpointChat || got[1] != ChannelModelEndpointResponses {
		t.Fatalf("NormalizeProviderModelSupportedEndpoints = %#v, want chat+responses", got)
	}
}

func TestDefaultProviderModelSupportedEndpointsByProvider(t *testing.T) {
	openai := DefaultProviderModelSupportedEndpoints("openai", ProviderModelTypeText, "gpt-5.4")
	if len(openai) != 2 || openai[0] != ChannelModelEndpointResponses || openai[1] != ChannelModelEndpointChat {
		t.Fatalf("openai default endpoints = %#v, want responses+chat", openai)
	}

	openAIRealtime := DefaultProviderModelSupportedEndpoints("openai", ProviderModelTypeAudio, "gpt-realtime-2")
	if len(openAIRealtime) != 1 || openAIRealtime[0] != ChannelModelEndpointRealtime {
		t.Fatalf("openai realtime default endpoints = %#v, want realtime", openAIRealtime)
	}

	regularAudio := DefaultProviderModelSupportedEndpoints("openai", ProviderModelTypeAudio, "tts-1")
	if len(regularAudio) != 1 || regularAudio[0] != ChannelModelEndpointAudio {
		t.Fatalf("openai regular audio default endpoints = %#v, want audio/speech", regularAudio)
	}

	anthropic := DefaultProviderModelSupportedEndpoints("anthropic", ProviderModelTypeText, "claude-opus-4-6")
	if len(anthropic) != 1 || anthropic[0] != ChannelModelEndpointMessages {
		t.Fatalf("anthropic default endpoints = %#v, want messages", anthropic)
	}
}

func TestOpenAITextProviderModelEndpointCandidatesBackfillsChat(t *testing.T) {
	got := openAITextProviderModelEndpointCandidates(ChannelModelEndpointResponses)
	want := ChannelModelEndpointChat + "," + ChannelModelEndpointResponses
	if got != want {
		t.Fatalf("openAITextProviderModelEndpointCandidates = %q, want %q", got, want)
	}
}

func TestBuildChannelModelEndpointRowsUsesProviderCatalogCandidates(t *testing.T) {
	rows := []ChannelModel{
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Provider:      "openai",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
	}
	providerEndpoints := map[string][]string{
		buildProviderModelEndpointKey("openai", "gpt-5.4"): {
			ChannelModelEndpointChat,
			ChannelModelEndpointResponses,
		},
	}

	got := BuildChannelModelEndpointRowsWithProviderEndpoints(nil, rows, providerEndpoints)
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

func TestBuildChannelModelEndpointRowsPreservesExistingDisabledEndpointState(t *testing.T) {
	existing := []ChannelModelEndpoint{
		{ChannelId: "channel-1", Model: "gpt-5.4", Endpoint: ChannelModelEndpointResponses, Enabled: false},
	}
	rows := []ChannelModel{
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Provider:      "openai",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
	}
	providerEndpoints := map[string][]string{
		buildProviderModelEndpointKey("openai", "gpt-5.4"): {ChannelModelEndpointResponses},
	}

	got := BuildChannelModelEndpointRowsWithProviderEndpoints(existing, rows, providerEndpoints)
	if len(got) != 1 {
		t.Fatalf("BuildChannelModelEndpointRows len = %d, want 1", len(got))
	}
	if got[0].Endpoint != ChannelModelEndpointResponses || got[0].Enabled {
		t.Fatalf("responses endpoint = (%q, %t), want (%q, false)", got[0].Endpoint, got[0].Enabled, ChannelModelEndpointResponses)
	}
}

func TestBuildChannelModelEndpointRowsSkipsInactiveOrUnselectedModels(t *testing.T) {
	rows := []ChannelModel{
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Provider:      "openai",
			Type:          ProviderModelTypeText,
			Selected:      false,
			Inactive:      true,
		},
	}
	providerEndpoints := map[string][]string{
		buildProviderModelEndpointKey("openai", "gpt-5.4"): {
			ChannelModelEndpointChat,
			ChannelModelEndpointResponses,
		},
	}

	got := BuildChannelModelEndpointRowsWithProviderEndpoints(nil, rows, providerEndpoints)
	if len(got) != 0 {
		t.Fatalf("len(got)=%d, want 0 for inactive/unselected model", len(got))
	}
}

func TestBuildChannelModelEndpointRowsPreservesExistingUpdatedAt(t *testing.T) {
	existing := []ChannelModelEndpoint{
		{ChannelId: "channel-1", Model: "gpt-5.4", Endpoint: ChannelModelEndpointResponses, Enabled: true, UpdatedAt: 123},
	}
	rows := []ChannelModel{
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Provider:      "openai",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
	}
	providerEndpoints := map[string][]string{
		buildProviderModelEndpointKey("openai", "gpt-5.4"): {ChannelModelEndpointResponses},
	}

	got := BuildChannelModelEndpointRowsWithProviderEndpoints(existing, rows, providerEndpoints)
	if len(got) != 1 {
		t.Fatalf("BuildChannelModelEndpointRows len = %d, want 1", len(got))
	}
	if got[0].UpdatedAt != 123 {
		t.Fatalf("responses updated_at = %d, want 123", got[0].UpdatedAt)
	}
}

func TestMergeChannelModelEndpointListRowsExplicitOverridesSnapshotState(t *testing.T) {
	snapshotRows := []ChannelModelEndpoint{
		{ChannelId: "channel-1", Model: "gpt-5.4", Endpoint: ChannelModelEndpointResponses, Enabled: false, UpdatedAt: 0},
	}
	explicitRows := []ChannelModelEndpoint{
		{ChannelId: "channel-1", Model: "gpt-5.4", Endpoint: ChannelModelEndpointResponses, Enabled: true, UpdatedAt: 456},
	}

	got := MergeChannelModelEndpointListRows(snapshotRows, explicitRows)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if !got[0].Enabled {
		t.Fatalf("merged enabled = false, want true")
	}
	if got[0].UpdatedAt != 456 {
		t.Fatalf("merged updated_at = %d, want 456", got[0].UpdatedAt)
	}
}

func TestMergeChannelModelEndpointListRowsKeepsExplicitOnlyRows(t *testing.T) {
	explicitRows := []ChannelModelEndpoint{
		{ChannelId: "channel-1", Model: "gpt-5.4", Endpoint: ChannelModelEndpointChat, Enabled: false, UpdatedAt: 789},
	}

	got := MergeChannelModelEndpointListRows(nil, explicitRows)
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0 for explicit-only orphan rows", len(got))
	}
}

func TestBuildChannelModelEndpointRowsDoesNotFallbackToChannelModelEndpoint(t *testing.T) {
	rows := []ChannelModel{
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Provider:      "openai",
			Type:          ProviderModelTypeText,
			Selected:      true,
			Endpoint:      ChannelModelEndpointResponses,
			Endpoints:     []string{ChannelModelEndpointResponses},
		},
	}

	got := BuildChannelModelEndpointRowsWithProviderEndpoints(nil, rows, nil)
	if len(got) != 0 {
		t.Fatalf("len(got)=%d, want 0 without provider catalog endpoint candidates", len(got))
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

func TestNormalizeRequestedChannelModelEndpointMessagesMapsToMessages(t *testing.T) {
	if got := NormalizeRequestedChannelModelEndpoint("/v1/messages"); got != ChannelModelEndpointMessages {
		t.Fatalf("NormalizeRequestedChannelModelEndpoint(/v1/messages)=%q, want %q", got, ChannelModelEndpointMessages)
	}
	if got := NormalizeRequestedChannelModelEndpoint("/api/v1/public/messages"); got != ChannelModelEndpointMessages {
		t.Fatalf("NormalizeRequestedChannelModelEndpoint(/api/v1/public/messages)=%q, want %q", got, ChannelModelEndpointMessages)
	}
}

func TestNormalizeRequestedChannelModelEndpointRealtimeMapsToRealtime(t *testing.T) {
	if got := NormalizeRequestedChannelModelEndpoint("/v1/realtime"); got != ChannelModelEndpointRealtime {
		t.Fatalf("NormalizeRequestedChannelModelEndpoint(/v1/realtime)=%q, want %q", got, ChannelModelEndpointRealtime)
	}
	if got := NormalizeRequestedChannelModelEndpoint("/api/v1/public/realtime"); got != ChannelModelEndpointRealtime {
		t.Fatalf("NormalizeRequestedChannelModelEndpoint(/api/v1/public/realtime)=%q, want %q", got, ChannelModelEndpointRealtime)
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
