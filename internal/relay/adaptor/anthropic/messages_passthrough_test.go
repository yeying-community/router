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

func TestCalcClaudeTotalTokensUsesIterations(t *testing.T) {
	inputTotal, outputTotal, total := calcClaudeTotalTokens(&Usage{
		InputTokens:              999,
		OutputTokens:             888,
		CacheCreationInputTokens: 777,
		CacheReadInputTokens:     666,
		Iterations: []UsageIteration{
			{
				Type:                     "tool_result",
				Model:                    "claude-opus-4-6",
				InputTokens:              10,
				OutputTokens:             3,
				CacheCreationInputTokens: 2,
				CacheReadInputTokens:     1,
			},
			{
				Type:                     "message",
				Model:                    "claude-opus-4-6",
				InputTokens:              20,
				OutputTokens:             4,
				CacheCreationInputTokens: 5,
				CacheReadInputTokens:     6,
			},
		},
	})

	if inputTotal != 44 || outputTotal != 7 || total != 51 {
		t.Fatalf("unexpected totals: input=%d output=%d total=%d", inputTotal, outputTotal, total)
	}
}

func TestCalcClaudeTotalTokensFallsBackToTopLevelUsage(t *testing.T) {
	inputTotal, outputTotal, total := calcClaudeTotalTokens(&Usage{
		InputTokens:              10,
		OutputTokens:             4,
		CacheCreationInputTokens: 3,
		CacheReadInputTokens:     2,
	})

	if inputTotal != 15 || outputTotal != 4 || total != 19 {
		t.Fatalf("unexpected totals: input=%d output=%d total=%d", inputTotal, outputTotal, total)
	}
}

func TestRelayMessagesResponseUsesClaudeRealTokenTotals(t *testing.T) {
	ctx, _ := newAnthropicPassthroughTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"msg_123",
			"type":"message",
			"role":"assistant",
			"content":[{"type":"text","text":"hello from claude"}],
			"usage":{
				"input_tokens":10,
				"output_tokens":5,
				"cache_creation_input_tokens":3,
				"cache_read_input_tokens":2
			}
		}`)),
	}

	usage, relayErr := relayMessagesResponse(ctx, resp)
	if relayErr != nil {
		t.Fatalf("relayMessagesResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.PromptTokens != 15 || usage.CompletionTokens != 5 || usage.TotalTokens != 20 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
}

func TestRelayMessagesResponseUsesIterationsInsteadOfTopLevelUsage(t *testing.T) {
	ctx, _ := newAnthropicPassthroughTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"msg_123",
			"type":"message",
			"role":"assistant",
			"content":[{"type":"text","text":"hello from claude"}],
			"usage":{
				"input_tokens":100,
				"output_tokens":50,
				"cache_creation_input_tokens":30,
				"cache_read_input_tokens":20,
				"iterations":[
					{
						"type":"message",
						"model":"claude-opus-4-6",
						"input_tokens":10,
						"output_tokens":3,
						"cache_creation_input_tokens":2,
						"cache_read_input_tokens":1
					},
					{
						"type":"message",
						"model":"claude-opus-4-6",
						"input_tokens":20,
						"output_tokens":4,
						"cache_creation_input_tokens":5,
						"cache_read_input_tokens":6
					}
				]
			}
		}`)),
	}

	usage, relayErr := relayMessagesResponse(ctx, resp)
	if relayErr != nil {
		t.Fatalf("relayMessagesResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.PromptTokens != 44 || usage.CompletionTokens != 7 || usage.TotalTokens != 51 {
		t.Fatalf("unexpected usage: %#v", usage)
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

func TestRelayMessagesStreamResponseUsesLatestUsageSnapshot(t *testing.T) {
	ctx, _ := newAnthropicPassthroughTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(
			"event: message_start\n" +
				"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"usage\":{\"input_tokens\":11,\"output_tokens\":0}}}\n\n" +
				"event: message_delta\n" +
				"data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":7}}\n\n" +
				"event: message_delta\n" +
				"data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":9}}\n\n",
		)),
	}

	usage, relayErr := relayMessagesStreamResponse(ctx, resp)
	if relayErr != nil {
		t.Fatalf("relayMessagesStreamResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.PromptTokens != 11 || usage.CompletionTokens != 9 || usage.TotalTokens != 20 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
}

func TestRelayMessagesStreamResponseUsesIterationsForBillingTotals(t *testing.T) {
	ctx, _ := newAnthropicPassthroughTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(
			"event: message_start\n" +
				"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"usage\":{\"input_tokens\":11,\"output_tokens\":0}}}\n\n" +
				"event: message_delta\n" +
				"data: {\"type\":\"message_delta\",\"usage\":{\"input_tokens\":100,\"output_tokens\":50,\"cache_creation_input_tokens\":30,\"cache_read_input_tokens\":20,\"iterations\":[{\"type\":\"message\",\"model\":\"claude-opus-4-6\",\"input_tokens\":10,\"output_tokens\":3,\"cache_creation_input_tokens\":2,\"cache_read_input_tokens\":1},{\"type\":\"message\",\"model\":\"claude-opus-4-6\",\"input_tokens\":20,\"output_tokens\":4,\"cache_creation_input_tokens\":5,\"cache_read_input_tokens\":6}]}}\n\n",
		)),
	}

	usage, relayErr := relayMessagesStreamResponse(ctx, resp)
	if relayErr != nil {
		t.Fatalf("relayMessagesStreamResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.PromptTokens != 44 || usage.CompletionTokens != 7 || usage.TotalTokens != 51 {
		t.Fatalf("unexpected usage: %#v", usage)
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

func TestRelayMessagesStreamAsChatResponseUsesIterationsForBillingTotals(t *testing.T) {
	ctx, _ := newAnthropicPassthroughTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(
			"event: message_start\n" +
				"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"claude-opus-4-6\",\"usage\":{\"input_tokens\":11,\"output_tokens\":0}}}\n\n" +
				"event: content_block_delta\n" +
				"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
				"event: message_delta\n" +
				"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"input_tokens\":100,\"output_tokens\":50,\"cache_creation_input_tokens\":30,\"cache_read_input_tokens\":20,\"iterations\":[{\"type\":\"message\",\"model\":\"claude-opus-4-6\",\"input_tokens\":10,\"output_tokens\":3,\"cache_creation_input_tokens\":2,\"cache_read_input_tokens\":1},{\"type\":\"message\",\"model\":\"claude-opus-4-6\",\"input_tokens\":20,\"output_tokens\":4,\"cache_creation_input_tokens\":5,\"cache_read_input_tokens\":6}]}}\n\n" +
				"data: [DONE]\n",
		)),
	}

	usage, relayErr := relayMessagesStreamAsChatResponse(ctx, resp, 11, "claude-opus-4-6")
	if relayErr != nil {
		t.Fatalf("relayMessagesStreamAsChatResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.PromptTokens != 44 || usage.CompletionTokens != 7 || usage.TotalTokens != 51 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
}

func TestRelayMessagesResponseSkipsUpstreamCORSHeaders(t *testing.T) {
	ctx, recorder := newAnthropicPassthroughTestContext()
	recorder.Header().Set("Access-Control-Allow-Origin", "http://localhost:3020")
	recorder.Header().Set("Access-Control-Allow-Credentials", "true")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Access-Control-Allow-Origin":      []string{"*"},
			"Access-Control-Allow-Credentials": []string{"true"},
			"X-Upstream":                       []string{"ok"},
		},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"msg_123",
			"type":"message",
			"role":"assistant",
			"content":[{"type":"text","text":"hello from claude"}],
			"usage":{"input_tokens":10,"output_tokens":5}
		}`)),
	}

	_, relayErr := relayMessagesResponse(ctx, resp)
	if relayErr != nil {
		t.Fatalf("relayMessagesResponse returned error: %+v", relayErr)
	}
	allowOriginValues := recorder.Header().Values("Access-Control-Allow-Origin")
	if len(allowOriginValues) != 1 || allowOriginValues[0] != "http://localhost:3020" {
		t.Fatalf("expected CORS allow-origin header to remain middleware value, got %#v", allowOriginValues)
	}
	if recorder.Header().Get("X-Upstream") != "ok" {
		t.Fatalf("expected non-CORS upstream headers to remain copied, got %q", recorder.Header().Get("X-Upstream"))
	}
}

func TestRelayMessagesStreamAsChatResponseSkipsUpstreamCORSHeaders(t *testing.T) {
	ctx, recorder := newAnthropicPassthroughTestContext()
	recorder.Header().Set("Access-Control-Allow-Origin", "http://localhost:3020")
	recorder.Header().Set("Access-Control-Allow-Credentials", "true")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":                     []string{"text/event-stream"},
			"Access-Control-Allow-Origin":      []string{"*"},
			"Access-Control-Allow-Credentials": []string{"true"},
		},
		Body: io.NopCloser(strings.NewReader(
			"event: message_start\n" +
				"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"claude-opus-4-6\",\"usage\":{\"input_tokens\":11,\"output_tokens\":0}}}\n\n" +
				"event: content_block_delta\n" +
				"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
				"event: message_delta\n" +
				"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":7}}\n\n" +
				"data: [DONE]\n",
		)),
	}

	_, relayErr := relayMessagesStreamAsChatResponse(ctx, resp, 11, "claude-opus-4-6")
	if relayErr != nil {
		t.Fatalf("relayMessagesStreamAsChatResponse returned error: %+v", relayErr)
	}
	allowOriginValues := recorder.Header().Values("Access-Control-Allow-Origin")
	if len(allowOriginValues) != 1 || allowOriginValues[0] != "http://localhost:3020" {
		t.Fatalf("expected CORS allow-origin header to remain middleware value, got %#v", allowOriginValues)
	}
}
