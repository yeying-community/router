package channel

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/yeying-community/router/common/config"
	adminmodel "github.com/yeying-community/router/internal/admin/model"
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
