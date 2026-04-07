package channel

import (
	"strings"
	"testing"
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
	if got != "Helloworld!" {
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
