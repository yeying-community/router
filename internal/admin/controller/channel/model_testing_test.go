package channel

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yeying-community/router/common/config"
	adminmodel "github.com/yeying-community/router/internal/admin/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBuildResponsesTextModelTestRequestBody_Stream_NoStreamOptions(t *testing.T) {
	body := buildResponsesTextModelTestRequestBody("gpt-5.4", true)
	if len(body) == 0 {
		t.Fatal("expected non-empty request body")
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal request body failed: %v", err)
	}

	if _, exists := payload["stream_options"]; exists {
		t.Fatalf("unexpected stream_options in request body: %v", payload["stream_options"])
	}
	if payload["model"] != "gpt-5.4" {
		t.Fatalf("unexpected model: %v", payload["model"])
	}
	stream, _ := payload["stream"].(bool)
	if !stream {
		t.Fatalf("expected stream=true, got %v", payload["stream"])
	}

	inputRows, ok := payload["input"].([]any)
	if !ok || len(inputRows) == 0 {
		t.Fatalf("expected non-empty input array, got %T", payload["input"])
	}
	firstRow, ok := inputRows[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first input row object, got %T", inputRows[0])
	}
	if firstRow["role"] != "user" {
		t.Fatalf("unexpected first row role: %v", firstRow["role"])
	}
	contentRows, ok := firstRow["content"].([]any)
	if !ok || len(contentRows) == 0 {
		t.Fatalf("expected non-empty content rows, got %T", firstRow["content"])
	}
	firstContent, ok := contentRows[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first content object, got %T", contentRows[0])
	}
	if firstContent["type"] != "input_text" {
		t.Fatalf("unexpected first content type: %v", firstContent["type"])
	}
	if firstContent["text"] != config.TestPrompt {
		t.Fatalf("unexpected first content text: %v", firstContent["text"])
	}
}

func TestReplaceModelNameInRawTextRequest_PreserveStreamFlag(t *testing.T) {
	body := []byte(`{"model":"old-model","stream":true,"input":"hello"}`)
	updated, stream, err := replaceModelNameInRawTextRequest(body, "new-model")
	if err != nil {
		t.Fatalf("replaceModelNameInRawTextRequest failed: %v", err)
	}
	if !stream {
		t.Fatalf("expected stream=true")
	}

	var payload map[string]any
	if err := json.Unmarshal(updated, &payload); err != nil {
		t.Fatalf("unmarshal updated body failed: %v", err)
	}
	if payload["model"] != "new-model" {
		t.Fatalf("unexpected model after replace: %v", payload["model"])
	}
}

func TestBuildChannelModelTestResult_PreserveIsStream(t *testing.T) {
	row := adminmodel.ChannelModel{
		Model:         "gpt-5.4",
		UpstreamModel: "gpt-5.4",
		Type:          adminmodel.ProviderModelTypeText,
		Endpoint:      adminmodel.ChannelModelEndpointResponses,
	}
	result := buildChannelModelTestResult(row, channelModelTestExecution{
		IsStream:  true,
		LatencyMs: 123,
		Message:   "ok",
	})
	if !result.IsStream {
		t.Fatalf("expected is_stream=true, got false")
	}
	if !result.Supported || result.Status != adminmodel.ChannelTestStatusSupported {
		t.Fatalf("expected supported result, got status=%q supported=%v", result.Status, result.Supported)
	}
}

func TestResolveChannelModelTestEndpoint_StrictRejectsEmpty(t *testing.T) {
	_, err := resolveChannelModelTestEndpoint(adminmodel.ProviderModelTypeText, "")
	if err == nil {
		t.Fatalf("expected error when endpoint is empty")
	}
}

func TestResolveChannelModelTestEndpoint_StrictRejectsMismatchedType(t *testing.T) {
	_, err := resolveChannelModelTestEndpoint(adminmodel.ProviderModelTypeText, adminmodel.ChannelModelEndpointImages)
	if err == nil {
		t.Fatalf("expected error when text model uses image endpoint")
	}
}

func TestResolveChannelModelTestEndpoint_AcceptsMessagesForText(t *testing.T) {
	endpoint, err := resolveChannelModelTestEndpoint(adminmodel.ProviderModelTypeText, adminmodel.ChannelModelEndpointMessages)
	if err != nil {
		t.Fatalf("resolveChannelModelTestEndpoint returned error: %v", err)
	}
	if endpoint != adminmodel.ChannelModelEndpointMessages {
		t.Fatalf("endpoint = %q, want %q", endpoint, adminmodel.ChannelModelEndpointMessages)
	}
}

func TestResolveChannelModelTestEndpointForRow_RejectsUnsupportedEndpointForModel(t *testing.T) {
	_, err := resolveChannelModelTestEndpointForRow(adminmodel.ChannelModel{
		Model:     "gpt-image-2",
		Type:      adminmodel.ProviderModelTypeImage,
		Endpoint:  adminmodel.ChannelModelEndpointImages,
		Endpoints: []string{adminmodel.ChannelModelEndpointResponses},
	})
	if err == nil {
		t.Fatal("expected error when endpoint is not declared by model")
	}
}

func TestResolveChannelModelTestEndpointForRow_AllowsDeclaredEndpointForModel(t *testing.T) {
	endpoint, err := resolveChannelModelTestEndpointForRow(adminmodel.ChannelModel{
		Model:     "gpt-image-2",
		Type:      adminmodel.ProviderModelTypeImage,
		Endpoint:  adminmodel.ChannelModelEndpointResponses,
		Endpoints: []string{adminmodel.ChannelModelEndpointResponses},
	})
	if err != nil {
		t.Fatalf("resolveChannelModelTestEndpointForRow returned error: %v", err)
	}
	if endpoint != adminmodel.ChannelModelEndpointResponses {
		t.Fatalf("endpoint = %q, want %q", endpoint, adminmodel.ChannelModelEndpointResponses)
	}
}

func TestResolveChannelModelTestEndpointForRow_AllowsRealtimeForAudioModel(t *testing.T) {
	endpoint, err := resolveChannelModelTestEndpointForRow(adminmodel.ChannelModel{
		Model:     "gpt-realtime-2",
		Type:      adminmodel.ProviderModelTypeAudio,
		Endpoint:  adminmodel.ChannelModelEndpointRealtime,
		Endpoints: []string{adminmodel.ChannelModelEndpointRealtime},
	})
	if err != nil {
		t.Fatalf("resolveChannelModelTestEndpointForRow returned error: %v", err)
	}
	if endpoint != adminmodel.ChannelModelEndpointRealtime {
		t.Fatalf("endpoint = %q, want %q", endpoint, adminmodel.ChannelModelEndpointRealtime)
	}
}

func TestValidateChannelModelTestEndpointAgainstProviderRejectsOutsideProviderRange(t *testing.T) {
	previousDB := adminmodel.DB
	t.Cleanup(func() {
		adminmodel.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	adminmodel.DB = db
	if err := db.AutoMigrate(&adminmodel.ProviderModel{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Create(&adminmodel.ProviderModel{
		Provider:           "qwen",
		Model:              "qwen3.7-max",
		Tags:               adminmodel.ProviderModelTypeText,
		Status:             adminmodel.ProviderModelStatusActive,
		SupportedEndpoints: adminmodel.ChannelModelEndpointChat,
	}).Error; err != nil {
		t.Fatalf("create provider model: %v", err)
	}

	err = validateChannelModelTestEndpointAgainstProvider(adminmodel.ChannelModel{
		Model:         "qwen3.7-max",
		UpstreamModel: "qwen3.7-max",
		Provider:      "qwen",
		Type:          adminmodel.ProviderModelTypeText,
		Endpoint:      adminmodel.ChannelModelEndpointResponses,
		Endpoints: []string{
			adminmodel.ChannelModelEndpointChat,
			adminmodel.ChannelModelEndpointResponses,
		},
	}, adminmodel.ChannelModelEndpointResponses)
	if err == nil || !strings.Contains(err.Error(), "供应商官方端点范围不包含") {
		t.Fatalf("validateChannelModelTestEndpointAgainstProvider error=%v, want provider endpoint rejection", err)
	}
}

func TestValidateChannelModelTestEndpointAgainstProviderAllowsProviderEndpoint(t *testing.T) {
	previousDB := adminmodel.DB
	t.Cleanup(func() {
		adminmodel.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	adminmodel.DB = db
	if err := db.AutoMigrate(&adminmodel.ProviderModel{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Create(&adminmodel.ProviderModel{
		Provider:           "qwen",
		Model:              "qwen3.7-max",
		Tags:               adminmodel.ProviderModelTypeText,
		Status:             adminmodel.ProviderModelStatusActive,
		SupportedEndpoints: adminmodel.ChannelModelEndpointChat,
	}).Error; err != nil {
		t.Fatalf("create provider model: %v", err)
	}

	if err := validateChannelModelTestEndpointAgainstProvider(adminmodel.ChannelModel{
		Model:         "qwen3.7-max",
		UpstreamModel: "qwen3.7-max",
		Provider:      "qwen",
		Type:          adminmodel.ProviderModelTypeText,
		Endpoint:      adminmodel.ChannelModelEndpointChat,
		Endpoints: []string{
			adminmodel.ChannelModelEndpointChat,
			adminmodel.ChannelModelEndpointResponses,
		},
	}, adminmodel.ChannelModelEndpointChat); err != nil {
		t.Fatalf("validateChannelModelTestEndpointAgainstProvider returned error: %v", err)
	}
}

func TestRestoreRuntimeDisabledCapabilitiesAfterSuccessfulTests(t *testing.T) {
	previousDB := adminmodel.DB
	t.Cleanup(func() {
		adminmodel.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	adminmodel.DB = db
	if err := db.AutoMigrate(
		&adminmodel.Channel{},
		&adminmodel.ChannelModel{},
		&adminmodel.ChannelModelPriceComponent{},
		&adminmodel.ChannelModelEndpoint{},
		&adminmodel.ProviderModel{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Create(&adminmodel.Channel{
		Id:       "channel-1",
		Name:     "channel-1",
		Protocol: "openai",
		Status:   adminmodel.ChannelStatusEnabled,
	}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&adminmodel.ProviderModel{
		Provider:           "qwen",
		Model:              "qwen3.7-max",
		Tags:               adminmodel.ProviderModelTypeText,
		Status:             adminmodel.ProviderModelStatusActive,
		SupportedEndpoints: adminmodel.ChannelModelEndpointChat,
	}).Error; err != nil {
		t.Fatalf("create provider model: %v", err)
	}
	if err := db.Create(&adminmodel.ChannelModel{
		ChannelId:      "channel-1",
		Model:          "qwen3.7-max",
		UpstreamModel:  "qwen3.7-max",
		Provider:       "qwen",
		Type:           adminmodel.ProviderModelTypeText,
		Endpoint:       adminmodel.ChannelModelEndpointChat,
		Endpoints:      []string{adminmodel.ChannelModelEndpointChat},
		Inactive:       true,
		Selected:       false,
		DisabledReason: "model not found",
		DisabledAt:     123,
		DisabledBy:     "runtime",
	}).Error; err != nil {
		t.Fatalf("create channel model: %v", err)
	}
	if err := db.Create(&adminmodel.ChannelModelEndpoint{
		ChannelId:      "channel-1",
		Model:          "qwen3.7-max",
		Endpoint:       adminmodel.ChannelModelEndpointChat,
		Enabled:        false,
		DisabledReason: "unsupported endpoint",
		DisabledAt:     123,
		DisabledBy:     "runtime",
	}).Error; err != nil {
		t.Fatalf("create channel endpoint: %v", err)
	}

	var restoredModels []string
	var restoredEndpoints []channelModelEndpointRestore
	if err := db.Transaction(func(tx *gorm.DB) error {
		models, endpoints, err := restoreRuntimeDisabledCapabilitiesAfterSuccessfulTests(tx, "channel-1", []adminmodel.ChannelTest{
			{
				ChannelId: "channel-1",
				Model:     "qwen3.7-max",
				Type:      adminmodel.ProviderModelTypeText,
				Endpoint:  adminmodel.ChannelModelEndpointChat,
				Status:    adminmodel.ChannelTestStatusSupported,
				Supported: true,
			},
		})
		restoredModels = models
		restoredEndpoints = endpoints
		return err
	}); err != nil {
		t.Fatalf("restoreRuntimeDisabledCapabilitiesAfterSuccessfulTests: %v", err)
	}
	if len(restoredModels) != 1 || restoredModels[0] != "qwen3.7-max" {
		t.Fatalf("restoredModels=%v, want qwen3.7-max", restoredModels)
	}
	if len(restoredEndpoints) != 1 || restoredEndpoints[0].Model != "qwen3.7-max" || restoredEndpoints[0].Endpoint != adminmodel.ChannelModelEndpointChat {
		t.Fatalf("restoredEndpoints=%v, want qwen3.7-max chat", restoredEndpoints)
	}
	modelRow := adminmodel.ChannelModel{}
	if err := db.First(&modelRow, "channel_id = ? AND model = ?", "channel-1", "qwen3.7-max").Error; err != nil {
		t.Fatalf("load channel model: %v", err)
	}
	if modelRow.Inactive || !modelRow.Selected || modelRow.DisabledReason != "" || modelRow.DisabledAt != 0 || modelRow.DisabledBy != "" {
		t.Fatalf("model row after restore = %+v, want selected active without runtime metadata", modelRow)
	}
	endpointRow := adminmodel.ChannelModelEndpoint{}
	if err := db.First(&endpointRow, "channel_id = ? AND model = ? AND endpoint = ?", "channel-1", "qwen3.7-max", adminmodel.ChannelModelEndpointChat).Error; err != nil {
		t.Fatalf("load endpoint row: %v", err)
	}
	if !endpointRow.Enabled || endpointRow.DisabledReason != "" || endpointRow.DisabledAt != 0 || endpointRow.DisabledBy != "" {
		t.Fatalf("endpoint row after restore = %+v, want enabled without runtime metadata", endpointRow)
	}
}

func TestResolveChannelModelTestKind_UsesEndpointBeforeQwenModelType(t *testing.T) {
	if got := resolveChannelModelTestKind(adminmodel.ProviderModelTypeImage, adminmodel.ChannelModelEndpointChat); got != channelModelTestKindText {
		t.Fatalf("image+chat test kind = %q, want %q", got, channelModelTestKindText)
	}
	if got := resolveChannelModelTestKind(adminmodel.ProviderModelTypeAudio, adminmodel.ChannelModelEndpointChat); got != channelModelTestKindText {
		t.Fatalf("audio+chat test kind = %q, want %q", got, channelModelTestKindText)
	}
	if got := resolveChannelModelTestKind(adminmodel.ProviderModelTypeImage, adminmodel.ChannelModelEndpointResponses); got != channelModelTestKindImageResponses {
		t.Fatalf("image+responses test kind = %q, want %q", got, channelModelTestKindImageResponses)
	}
}

func TestWrapRealtimeHandshakeError(t *testing.T) {
	err := wrapRealtimeHandshakeError(errors.New("http status code: 401, error message: invalid token"))
	if err == nil {
		t.Fatal("expected wrapped error")
	}
	if got := err.Error(); got != "WebSocket 握手失败: http status code: 401, error message: invalid token" {
		t.Fatalf("unexpected wrapped error: %q", got)
	}
}

func TestWrapRealtimeSessionError(t *testing.T) {
	err := wrapRealtimeSessionError(errors.New("realtime server error: model price not configured"))
	if err == nil {
		t.Fatal("expected wrapped error")
	}
	if got := err.Error(); got != "WebSocket 握手成功，但会话失败: realtime server error: model price not configured" {
		t.Fatalf("unexpected wrapped error: %q", got)
	}
}

func TestBuildRealtimeSessionSuccessMessage(t *testing.T) {
	withText := buildRealtimeSessionSuccessMessage("realtime", "OK")
	if withText != "WebSocket 会话成功（subprotocol=realtime），返回文本：OK" {
		t.Fatalf("unexpected success message with text: %q", withText)
	}

	withoutText := buildRealtimeSessionSuccessMessage("", "")
	if withoutText != "WebSocket 会话成功，未返回文本" {
		t.Fatalf("unexpected success message without text: %q", withoutText)
	}
}
