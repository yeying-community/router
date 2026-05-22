package channel

import (
	"strings"
	"testing"

	adminmodel "github.com/yeying-community/router/internal/admin/model"
)

func TestParseTextModelTestResponse_ChatJSON(t *testing.T) {
	resp := `{"choices":[{"message":{"content":"chat ok"}}]}`
	got, err := parseTextModelTestResponse(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "chat ok" {
		t.Fatalf("unexpected parsed text: %q", got)
	}
}

func TestParseTextModelTestResponse_ResponsesJSON(t *testing.T) {
	resp := `{"output":[{"content":[{"type":"output_text","text":"responses ok"}]}]}`
	got, err := parseTextModelTestResponse(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "responses ok" {
		t.Fatalf("unexpected parsed text: %q", got)
	}
}

func TestParseTextModelTestResponse_ResponsesJSONOutputText(t *testing.T) {
	resp := `{"status":"completed","output_text":"responses output_text ok","output":[]}`
	got, err := parseTextModelTestResponse(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "responses output_text ok" {
		t.Fatalf("unexpected parsed text: %q", got)
	}
}

func TestParseTextModelTestResponse_ResponsesJSONEmptyOutput(t *testing.T) {
	resp := `{"status":"completed","output":[]}`
	_, err := parseResponsesModelTestResponse(resp)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "output is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseTextModelTestResponse_ResponsesJSONEmptyOutputWithUsage(t *testing.T) {
	resp := `{"status":"completed","output":[],"usage":{"output_tokens":9,"total_tokens":37}}`
	got, err := parseResponsesModelTestResponse(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got == "" {
		t.Fatalf("expected non-empty success message, got %q", got)
	}
}

func TestParseTextModelTestResponseByEndpoint_ResponsesOnlyError(t *testing.T) {
	resp := `{"status":"completed","output":[]}`
	_, err := parseTextModelTestResponseByEndpoint(adminmodel.ChannelModelEndpointResponses, resp)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), "parse as chat failed") || strings.Contains(err.Error(), "parse as messages failed") {
		t.Fatalf("unexpected mixed parser error: %v", err)
	}
	if !strings.Contains(err.Error(), "parse as responses failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseTextModelTestResponse_MessagesJSON(t *testing.T) {
	resp := `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"我是claude模型，由Anthropic开发。"}]}`
	got, err := parseTextModelTestResponse(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "我是claude模型，由Anthropic开发。" {
		t.Fatalf("unexpected parsed text: %q", got)
	}
}

func TestParseTextModelTestResponse_ResponsesSSE(t *testing.T) {
	resp := strings.Join([]string{
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"Hello"}`,
		"",
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":" world"}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","output_text":"!"}`,
		"",
		"data: [DONE]",
	}, "\n")

	got, err := parseTextModelTestResponse(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "Hello world!" {
		t.Fatalf("unexpected parsed text: %q", got)
	}
}

func TestParseTextModelTestResponse_ResponsesSSEStartsWithID(t *testing.T) {
	resp := strings.Join([]string{
		"id: 1",
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"Hello"}`,
		"",
		"id: 2",
		"event: response.completed",
		`data: {"type":"response.completed","output_text":"Hello"}`,
		"",
		"data: [DONE]",
	}, "\n")

	got, err := parseTextModelTestResponseByEndpoint(adminmodel.ChannelModelEndpointResponses, resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "Hello" {
		t.Fatalf("unexpected parsed text: %q", got)
	}
}

func TestParseTextModelTestResponse_ResponsesSSECompletedContainsFullText(t *testing.T) {
	resp := strings.Join([]string{
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"OpenAI"}`,
		"",
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":" o4-mini"}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","output_text":"OpenAI o4-mini"}`,
		"",
		"data: [DONE]",
	}, "\n")

	got, err := parseTextModelTestResponse(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "OpenAI o4-mini" {
		t.Fatalf("unexpected parsed text: %q", got)
	}
}

func TestParseTextModelTestResponse_ResponsesSSEOutputTextDoneContainsFullText(t *testing.T) {
	resp := strings.Join([]string{
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"Open"}`,
		"",
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"AI"}`,
		"",
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":" o3"}`,
		"",
		"event: response.output_text.done",
		`data: {"type":"response.output_text.done","text":"OpenAI o3"}`,
		"",
		"data: [DONE]",
	}, "\n")

	got, err := parseTextModelTestResponse(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "OpenAI o3" {
		t.Fatalf("unexpected parsed text: %q", got)
	}
}

func TestParseTextModelTestResponse_ChatSSE(t *testing.T) {
	resp := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"你"}}]}`,
		`data: {"choices":[{"delta":{"content":"好"}}]}`,
		"data: [DONE]",
	}, "\n")

	got, err := parseTextModelTestResponse(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "你好" {
		t.Fatalf("unexpected parsed text: %q", got)
	}
}

func TestParseTextModelTestResponse_AnthropicMessagesSSE(t *testing.T) {
	resp := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"content":[]}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"claude-"}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"haiku-4-5-20251001"}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
	}, "\n")

	got, err := parseTextModelTestResponse(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "claude-haiku-4-5-20251001" {
		t.Fatalf("unexpected parsed text: %q", got)
	}
}

func TestParseTextModelTestResponseByEndpoint_MessagesSSE(t *testing.T) {
	resp := strings.Join([]string{
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"claude-opus-4-6"}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
	}, "\n")

	got, err := parseTextModelTestResponseByEndpoint(adminmodel.ChannelModelEndpointMessages, resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "claude-opus-4-6" {
		t.Fatalf("unexpected parsed text: %q", got)
	}
}

func TestParseTextModelTestResponse_SSEError(t *testing.T) {
	resp := strings.Join([]string{
		"event: error",
		`data: {"error":{"message":"rate limited"}}`,
	}, "\n")

	_, err := parseTextModelTestResponse(resp)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("unexpected error: %v", err)
	}
}
