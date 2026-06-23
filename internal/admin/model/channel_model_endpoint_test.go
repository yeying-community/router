package model

import (
	"testing"

	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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

	deepSeek := NormalizeProviderModelSupportedEndpointsForModel(
		ProviderModelTypeText,
		"deepseek-v4-pro",
		[]string{ChannelModelEndpointChat, ChannelModelEndpointMessages},
	)
	if len(deepSeek) != 2 || deepSeek[0] != ChannelModelEndpointChat || deepSeek[1] != ChannelModelEndpointMessages {
		t.Fatalf("deepseek normalized endpoints = %#v, want chat+messages", deepSeek)
	}

	embedding := DefaultProviderModelSupportedEndpoints("volcengine", ProviderModelTypeEmbedding, "Seed1.6-Embedding")
	if len(embedding) != 1 || embedding[0] != ChannelModelEndpointEmbeddings {
		t.Fatalf("embedding default endpoints = %#v, want embeddings", embedding)
	}

	volcengineText := DefaultProviderModelSupportedEndpoints("volcengine", ProviderModelTypeText, "doubao-seed-2-0-pro-260215")
	if len(volcengineText) != 2 ||
		volcengineText[0] != ChannelModelEndpointChat ||
		volcengineText[1] != ChannelModelEndpointResponses {
		t.Fatalf("volcengine text default endpoints = %#v, want chat+responses", volcengineText)
	}
	if providerModelEndpointsContain(volcengineText, ChannelModelEndpointMessages) {
		t.Fatalf("volcengine text default endpoints = %#v, should not include messages", volcengineText)
	}

	qwenVL := DefaultProviderModelSupportedEndpoints("qwen", ProviderModelTypeImage, "qwen-vl-max")
	if len(qwenVL) != 1 || qwenVL[0] != ChannelModelEndpointChat {
		t.Fatalf("qwen vl default endpoints = %#v, want chat", qwenVL)
	}

	qwenOmni := DefaultProviderModelSupportedEndpoints("qwen", ProviderModelTypeAudio, "qwen3-omni-flash")
	if len(qwenOmni) != 1 || qwenOmni[0] != ChannelModelEndpointChat {
		t.Fatalf("qwen omni default endpoints = %#v, want chat", qwenOmni)
	}

	qwenRealtime := DefaultProviderModelSupportedEndpoints("qwen", ProviderModelTypeAudio, "qwen3-omni-flash-realtime")
	if len(qwenRealtime) != 1 || qwenRealtime[0] != ChannelModelEndpointRealtime {
		t.Fatalf("qwen realtime default endpoints = %#v, want realtime", qwenRealtime)
	}

	qwenRealtimeTTS := DefaultProviderModelSupportedEndpoints("qwen", ProviderModelTypeAudio, "qwen-tts-realtime")
	if len(qwenRealtimeTTS) != 1 || qwenRealtimeTTS[0] != ChannelModelEndpointRealtime {
		t.Fatalf("qwen realtime tts default endpoints = %#v, want realtime", qwenRealtimeTTS)
	}

	qwenTTS := DefaultProviderModelSupportedEndpoints("qwen", ProviderModelTypeAudio, "qwen-tts")
	if len(qwenTTS) != 0 {
		t.Fatalf("qwen tts default endpoints = %#v, want empty", qwenTTS)
	}

	zhipuRealtime := DefaultProviderModelSupportedEndpoints("zhipu", ProviderModelTypeAudio, "glm-realtime-flash")
	if len(zhipuRealtime) != 1 || zhipuRealtime[0] != ChannelModelEndpointRealtime {
		t.Fatalf("zhipu realtime default endpoints = %#v, want realtime", zhipuRealtime)
	}

	qwenImage := DefaultProviderModelSupportedEndpoints("qwen", ProviderModelTypeImage, "qwen-image-2.0")
	if len(qwenImage) != 2 || qwenImage[0] != ChannelModelEndpointImages || qwenImage[1] != ChannelModelEndpointImageEdit {
		t.Fatalf("qwen image default endpoints = %#v, want images+edits", qwenImage)
	}
}

func TestDefaultChannelModelEndpointWithProtocol_ZhipuTextUsesChat(t *testing.T) {
	got := DefaultChannelModelEndpointWithProtocol(ProviderModelTypeText, relaychannel.Zhipu)
	if got != ChannelModelEndpointChat {
		t.Fatalf("zhipu text default endpoint = %q, want %q", got, ChannelModelEndpointChat)
	}
}

func TestNormalizeProviderModelSupportedEndpointsForQwenSpecialModels(t *testing.T) {
	qwenVL := NormalizeProviderModelSupportedEndpointsForModel(
		ProviderModelTypeImage,
		"qwen-vl-max",
		[]string{ChannelModelEndpointResponses, ChannelModelEndpointChat},
	)
	if len(qwenVL) != 1 || qwenVL[0] != ChannelModelEndpointChat {
		t.Fatalf("qwen vl normalized endpoints = %#v, want chat", qwenVL)
	}

	qwenOmni := NormalizeProviderModelSupportedEndpointsForModel(
		ProviderModelTypeAudio,
		"qwen3-omni-flash",
		[]string{ChannelModelEndpointAudio, ChannelModelEndpointChat},
	)
	if len(qwenOmni) != 1 || qwenOmni[0] != ChannelModelEndpointChat {
		t.Fatalf("qwen omni normalized endpoints = %#v, want chat", qwenOmni)
	}

	qwenTTS := NormalizeProviderModelSupportedEndpointsForModel(
		ProviderModelTypeAudio,
		"qwen-tts",
		[]string{ChannelModelEndpointAudio, ChannelModelEndpointChat},
	)
	if len(qwenTTS) != 0 {
		t.Fatalf("qwen tts normalized endpoints = %#v, want empty", qwenTTS)
	}

	qwenImage, handled := qwenProviderSupportedEndpoints(
		ProviderModelTypeImage,
		"qwen-image-2.0",
		[]string{ChannelModelEndpointResponses, ChannelModelEndpointImages, ChannelModelEndpointImageEdit},
	)
	if !handled {
		t.Fatalf("qwen image endpoint rule handled=false, want true")
	}
	if len(qwenImage) != 2 || qwenImage[0] != ChannelModelEndpointImages || qwenImage[1] != ChannelModelEndpointImageEdit {
		t.Fatalf("qwen image normalized endpoints = %#v, want images+edits", qwenImage)
	}
}

func TestOpenAITextProviderModelEndpointCandidatesBackfillsChat(t *testing.T) {
	got := openAITextProviderModelEndpointCandidates(ChannelModelEndpointResponses)
	want := ChannelModelEndpointChat + "," + ChannelModelEndpointResponses
	if got != want {
		t.Fatalf("openAITextProviderModelEndpointCandidates = %q, want %q", got, want)
	}
}

func TestBuildChannelModelEndpointRowsUsesProviderModelCandidates(t *testing.T) {
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

func TestMergeChannelModelEndpointListRowsDropsExplicitOnlyRows(t *testing.T) {
	explicitRows := []ChannelModelEndpoint{
		{ChannelId: "channel-1", Model: "gpt-5.4", Endpoint: ChannelModelEndpointChat, Enabled: false, UpdatedAt: 789},
	}

	got := MergeChannelModelEndpointListRows(nil, explicitRows)
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0 for endpoint outside provider baseline", len(got))
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
		t.Fatalf("len(got)=%d, want 0 without provider model endpoint candidates", len(got))
	}
}

func TestBuildDisabledChannelModelEndpointRowsMarksOnlyTargetEndpoint(t *testing.T) {
	rows := []ChannelModelEndpoint{
		{ChannelId: "channel-1", Model: "gpt-5.4", Endpoint: ChannelModelEndpointChat, Enabled: true},
		{ChannelId: "channel-1", Model: "gpt-5.4", Endpoint: ChannelModelEndpointResponses, Enabled: true},
	}

	got, changed := buildDisabledChannelModelEndpointRows(rows, "channel-1", "gpt-5.4", ChannelModelEndpointResponses, "unsupported endpoint", "runtime")
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
	if got[1].DisabledReason != "unsupported endpoint" || got[1].DisabledBy != "runtime" || got[1].DisabledAt == 0 {
		t.Fatalf("responses endpoint disable metadata = reason:%q by:%q at:%d, want populated", got[1].DisabledReason, got[1].DisabledBy, got[1].DisabledAt)
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

func TestNormalizeRequestedChannelModelEndpointEmbeddingsMapsToEmbeddings(t *testing.T) {
	if got := NormalizeRequestedChannelModelEndpoint("/v1/embeddings"); got != ChannelModelEndpointEmbeddings {
		t.Fatalf("NormalizeRequestedChannelModelEndpoint(/v1/embeddings)=%q, want %q", got, ChannelModelEndpointEmbeddings)
	}
	if got := NormalizeRequestedChannelModelEndpoint("/api/v1/public/embeddings"); got != ChannelModelEndpointEmbeddings {
		t.Fatalf("NormalizeRequestedChannelModelEndpoint(/api/v1/public/embeddings)=%q, want %q", got, ChannelModelEndpointEmbeddings)
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

func TestReplaceChannelModelsWithDBSyncsEndpointsFromStoredRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(
		&Channel{},
		&ChannelModel{},
		&ChannelModelEndpoint{},
		&ChannelModelPriceComponent{},
		&ProviderModel{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&Channel{
		Id:       "channel-1",
		Protocol: "openai",
		Name:     "channel-1",
	}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}

	if err := db.Create(&ProviderModel{
		Provider:           "openai",
		Model:              "gpt-5.4",
		Tags:               ProviderModelTypeText,
		Status:             ProviderModelStatusActive,
		SupportedEndpoints: ChannelModelEndpointChat + "," + ChannelModelEndpointResponses,
	}).Error; err != nil {
		t.Fatalf("create provider model gpt-5.4: %v", err)
	}
	if err := db.Create(&ProviderModel{
		Provider:           "openai",
		Model:              "gpt-5.4-nano",
		Tags:               ProviderModelTypeText,
		Status:             ProviderModelStatusActive,
		SupportedEndpoints: ChannelModelEndpointChat + "," + ChannelModelEndpointResponses,
	}).Error; err != nil {
		t.Fatalf("create provider model gpt-5.4-nano: %v", err)
	}

	initialRows := []ChannelModel{
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Provider:      "openai",
			Type:          ProviderModelTypeText,
			Selected:      true,
			SortOrder:     1,
		},
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.4-nano",
			UpstreamModel: "gpt-5.4-nano",
			Provider:      "openai",
			Type:          ProviderModelTypeText,
			Selected:      true,
			SortOrder:     2,
		},
	}
	if err := db.Create(&initialRows).Error; err != nil {
		t.Fatalf("create initial channel models: %v", err)
	}
	if err := db.Create(&[]ChannelModelEndpoint{
		{
			ChannelId: "channel-1",
			Model:     "gpt-5.4",
			Endpoint:  ChannelModelEndpointChat,
			Enabled:   true,
		},
		{
			ChannelId: "channel-1",
			Model:     "gpt-5.4",
			Endpoint:  ChannelModelEndpointResponses,
		},
		{
			ChannelId: "channel-1",
			Model:     "gpt-5.4-nano",
			Endpoint:  ChannelModelEndpointChat,
			Enabled:   true,
		},
		{
			ChannelId: "channel-1",
			Model:     "gpt-5.4-nano",
			Endpoint:  ChannelModelEndpointResponses,
			Enabled:   true,
		},
	}).Error; err != nil {
		t.Fatalf("create initial channel endpoints: %v", err)
	}
	if err := db.Model(&ChannelModelEndpoint{}).
		Where("channel_id = ? AND model = ? AND endpoint = ?", "channel-1", "gpt-5.4", ChannelModelEndpointResponses).
		Update("enabled", false).Error; err != nil {
		t.Fatalf("set initial disabled endpoint: %v", err)
	}

	updatedRows := []ChannelModel{
		{
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Provider:      "openai",
			Type:          ProviderModelTypeText,
			Selected:      true,
			SortOrder:     1,
		},
		{
			Model:         "gpt-5.4-nano",
			UpstreamModel: "gpt-5.4-nano",
			Provider:      "openai",
			Type:          ProviderModelTypeText,
			Selected:      false,
			SortOrder:     2,
		},
	}
	if err := ReplaceChannelModelsWithDB(db, "channel-1", updatedRows); err != nil {
		t.Fatalf("ReplaceChannelModelsWithDB: %v", err)
	}

	endpointRows, err := listChannelModelEndpointRowsByChannelIDWithDB(db, "channel-1")
	if err != nil {
		t.Fatalf("listChannelModelEndpointRowsByChannelIDWithDB: %v", err)
	}
	if len(endpointRows) != 2 {
		t.Fatalf("len(endpointRows)=%d, want 2 endpoints preserved for selected model only", len(endpointRows))
	}

	got := map[string]bool{}
	for _, row := range endpointRows {
		if row.Model != "gpt-5.4" {
			t.Fatalf("unexpected endpoint row model=%q, want only gpt-5.4", row.Model)
		}
		got[row.Endpoint] = row.Enabled
	}
	if enabled, ok := got[ChannelModelEndpointChat]; !ok || !enabled {
		t.Fatalf("chat endpoint = (%t, %t), want (true, true)", enabled, ok)
	}
	if enabled, ok := got[ChannelModelEndpointResponses]; !ok || enabled {
		t.Fatalf("responses endpoint = (%t, %t), want (false, true)", enabled, ok)
	}
}
