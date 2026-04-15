package channel

import (
	"encoding/json"
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
