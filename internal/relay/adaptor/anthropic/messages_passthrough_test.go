package anthropic

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newAnthropicPassthroughTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	return ctx, recorder
}

func TestRelayMessagesResponsePreservesAnthropicShape(t *testing.T) {
	ctx, recorder := newAnthropicPassthroughTestContext()
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

	usage, relayErr := relayMessagesResponse(ctx, resp)
	if relayErr != nil {
		t.Fatalf("relayMessagesResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.PromptTokens != 10 || usage.CompletionTokens != 5 || usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"type":"message"`) || !strings.Contains(body, `"hello from claude"`) {
		t.Fatalf("unexpected relayed anthropic payload: %s", body)
	}
	if strings.Contains(body, `"object":"chat.completion"`) {
		t.Fatalf("unexpected OpenAI payload leaked into anthropic passthrough: %s", body)
	}
	if recorder.Header().Get("X-Upstream") != "ok" {
		t.Fatalf("expected upstream header to be copied, got %q", recorder.Header().Get("X-Upstream"))
	}
}

func TestRelayMessagesStreamResponsePreservesAnthropicSSE(t *testing.T) {
	ctx, recorder := newAnthropicPassthroughTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(
			"event: message_start\n" +
				"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"usage\":{\"input_tokens\":11,\"output_tokens\":0}}}\n\n" +
				"event: content_block_delta\n" +
				"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
				"event: message_delta\n" +
				"data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":7}}\n\n",
		)),
	}

	usage, relayErr := relayMessagesStreamResponse(ctx, resp)
	if relayErr != nil {
		t.Fatalf("relayMessagesStreamResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.PromptTokens != 11 || usage.CompletionTokens != 7 || usage.TotalTokens != 18 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: message_start") || !strings.Contains(body, `"type":"message_delta"`) {
		t.Fatalf("expected anthropic stream payload to be relayed as-is: %s", body)
	}
	if strings.Contains(body, `"object":"chat.completion.chunk"`) {
		t.Fatalf("unexpected OpenAI stream payload leaked into anthropic passthrough: %s", body)
	}
}

func TestRelayMessagesStreamResponseSupportsLargeAnthropicDataLine(t *testing.T) {
	ctx, recorder := newAnthropicPassthroughTestContext()
	largeText := strings.Repeat("a", 70*1024)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(
			"event: message_start\n" +
				"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"usage\":{\"input_tokens\":11,\"output_tokens\":0}}}\n\n" +
				"event: content_block_delta\n" +
				"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"" + largeText + "\"}}\n\n" +
				"event: message_delta\n" +
				"data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":7}}\n\n",
		)),
	}

	usage, relayErr := relayMessagesStreamResponse(ctx, resp)
	if relayErr != nil {
		t.Fatalf("relayMessagesStreamResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.PromptTokens != 11 || usage.CompletionTokens != 7 || usage.TotalTokens != 18 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, largeText) {
		t.Fatalf("expected large anthropic stream payload to be relayed intact")
	}
}

func TestRelayMessagesStreamAsChatResponse(t *testing.T) {
	ctx, recorder := newAnthropicPassthroughTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
			"X-Upstream":   []string{"ok"},
		},
		Body: io.NopCloser(strings.NewReader(
			"event: message_start\n" +
				"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"claude-opus-4-6\",\"usage\":{\"input_tokens\":11,\"output_tokens\":0}}}\n\n" +
				"event: content_block_delta\n" +
				"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello \"}}\n\n" +
				"event: content_block_delta\n" +
				"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"world\"}}\n\n" +
				"event: message_delta\n" +
				"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":7}}\n\n" +
				"data: [DONE]\n",
		)),
	}

	usage, relayErr := relayMessagesStreamAsChatResponse(ctx, resp, 11, "claude-opus-4-6")
	if relayErr != nil {
		t.Fatalf("relayMessagesStreamAsChatResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.PromptTokens != 11 || usage.CompletionTokens != 7 || usage.TotalTokens != 18 {
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
