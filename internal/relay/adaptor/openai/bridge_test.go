package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newBridgeTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	return ctx, recorder
}

func TestRelayResponsesAsChatResponse(t *testing.T) {
	ctx, recorder := newBridgeTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"X-Upstream": []string{"ok"},
		},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"resp_123",
			"object":"response",
			"model":"gpt-4.1",
			"created_at":1710000000,
			"output_text":"hello world",
			"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`)),
	}

	usage, relayErr := relayResponsesAsChatResponse(ctx, resp, "gpt-4.1", 10)
	if relayErr != nil {
		t.Fatalf("relayResponsesAsChatResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.TotalTokens != 15 || usage.PromptTokens != 10 || usage.CompletionTokens != 5 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"object":"chat.completion"`) || !strings.Contains(body, `"content":"hello world"`) {
		t.Fatalf("unexpected bridged chat payload: %s", body)
	}
	if recorder.Header().Get("X-Upstream") != "ok" {
		t.Fatalf("expected upstream header to be copied, got %q", recorder.Header().Get("X-Upstream"))
	}
}

func TestRelayChatAsResponsesResponse(t *testing.T) {
	ctx, recorder := newBridgeTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"chatcmpl_123",
			"object":"chat.completion",
			"model":"gpt-4.1",
			"created":1710000000,
			"choices":[{"index":0,"message":{"role":"assistant","content":"hello back"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}
		}`)),
	}

	usage, relayErr := relayChatAsResponsesResponse(ctx, resp, "gpt-4.1", 11)
	if relayErr != nil {
		t.Fatalf("relayChatAsResponsesResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.TotalTokens != 18 || usage.PromptTokens != 11 || usage.CompletionTokens != 7 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"object":"response"`) || !strings.Contains(body, `"output_text":"hello back"`) {
		t.Fatalf("unexpected bridged responses payload: %s", body)
	}
	if !strings.Contains(body, `"input_tokens":11`) || !strings.Contains(body, `"output_tokens":7`) {
		t.Fatalf("expected usage to be converted into responses shape: %s", body)
	}
}

func TestRelayMessagesResponse(t *testing.T) {
	ctx, recorder := newBridgeTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"X-Upstream": []string{"ok"},
		},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"msg_123",
			"type":"message",
			"role":"assistant",
			"content":[{"type":"text","text":"hello from claude"}],
			"usage":{"input_tokens":10,"output_tokens":5}
		}`)),
	}

	usage, relayErr := relayMessagesResponse(ctx, resp, 10, "claude-sonnet-4-6")
	if relayErr != nil {
		t.Fatalf("relayMessagesResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.PromptTokens != 10 || usage.CompletionTokens != 5 || usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"type":"message"`) || !strings.Contains(body, `"hello from claude"`) {
		t.Fatalf("unexpected relayed messages payload: %s", body)
	}
	if recorder.Header().Get("X-Upstream") != "ok" {
		t.Fatalf("expected upstream header to be copied, got %q", recorder.Header().Get("X-Upstream"))
	}
}

func TestRelayMessagesStreamResponse(t *testing.T) {
	ctx, recorder := newBridgeTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(
			"event: message_start\n" +
				"data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":11}}}\n\n" +
				"event: content_block_delta\n" +
				"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
				"event: message_delta\n" +
				"data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":7}}\n\n" +
				"data: [DONE]\n",
		)),
	}

	usage, relayErr := relayMessagesStreamResponse(ctx, resp, 11, "claude-sonnet-4-6")
	if relayErr != nil {
		t.Fatalf("relayMessagesStreamResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.PromptTokens != 11 || usage.CompletionTokens != 7 || usage.TotalTokens != 18 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "\"message_delta\"") || !strings.Contains(body, "[DONE]") {
		t.Fatalf("expected stream payload to be relayed as-is: %s", body)
	}
}
