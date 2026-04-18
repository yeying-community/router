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

func TestRelayResponsesAsChatResponse_ExtractsMessageOutputTextFromOutput(t *testing.T) {
	ctx, recorder := newBridgeTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"resp_456",
			"object":"response",
			"model":"gpt-5-2025-08-07",
			"created_at":1756315696,
			"output":[
				{"id":"rs_x","type":"reasoning","content":[{"type":"output_text","text":"SHOULD_NOT_APPEAR"}]},
				{"id":"msg_x","type":"message","role":"assistant","content":[{"type":"output_text","text":"hello from message"}]}
			],
			"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`)),
	}

	usage, relayErr := relayResponsesAsChatResponse(ctx, resp, "gpt-5-2025-08-07", 10)
	if relayErr != nil {
		t.Fatalf("relayResponsesAsChatResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.TotalTokens != 15 || usage.PromptTokens != 10 || usage.CompletionTokens != 5 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"content":"hello from message"`) {
		t.Fatalf("expected message output text to be bridged, got body: %s", body)
	}
	if strings.Contains(body, "SHOULD_NOT_APPEAR") {
		t.Fatalf("unexpected non-message output leaked into chat response: %s", body)
	}
}

func TestRelayResponsesAsChatResponse_ExtractsMessageTextTypeFromOutput(t *testing.T) {
	ctx, recorder := newBridgeTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"resp_789",
			"object":"response",
			"model":"gpt-5.4",
			"created_at":1756315696,
			"output":[
				{"id":"msg_x","type":"message","role":"assistant","content":[{"type":"text","text":"hello from text type"}]}
			],
			"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`)),
	}

	usage, relayErr := relayResponsesAsChatResponse(ctx, resp, "gpt-5.4", 10)
	if relayErr != nil {
		t.Fatalf("relayResponsesAsChatResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.TotalTokens != 15 || usage.PromptTokens != 10 || usage.CompletionTokens != 5 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"content":"hello from text type"`) {
		t.Fatalf("expected text-type output to be bridged, got body: %s", body)
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

func TestRelayResponsesResponseSkipsUpstreamCORSHeaders(t *testing.T) {
	ctx, recorder := newBridgeTestContext()
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
			"id":"resp_123",
			"object":"response",
			"model":"gpt-5.4",
			"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`)),
	}

	usage, relayErr := relayResponsesResponse(ctx, resp)
	if relayErr != nil {
		t.Fatalf("relayResponsesResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	allowOriginValues := recorder.Header().Values("Access-Control-Allow-Origin")
	if len(allowOriginValues) != 1 || allowOriginValues[0] != "http://localhost:3020" {
		t.Fatalf("expected CORS allow-origin header to remain middleware value, got %#v", allowOriginValues)
	}
	if recorder.Header().Get("X-Upstream") != "ok" {
		t.Fatalf("expected non-CORS upstream headers to remain copied, got %q", recorder.Header().Get("X-Upstream"))
	}
}

func TestRelayMessagesResponseSkipsUpstreamCORSHeaders(t *testing.T) {
	ctx, recorder := newBridgeTestContext()
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
			"content":[{"type":"text","text":"hello"}],
			"usage":{"input_tokens":10,"output_tokens":5}
		}`)),
	}

	usage, relayErr := relayMessagesResponse(ctx, resp, 10, "claude-sonnet-4-6")
	if relayErr != nil {
		t.Fatalf("relayMessagesResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	allowOriginValues := recorder.Header().Values("Access-Control-Allow-Origin")
	if len(allowOriginValues) != 1 || allowOriginValues[0] != "http://localhost:3020" {
		t.Fatalf("expected CORS allow-origin header to remain middleware value, got %#v", allowOriginValues)
	}
	if recorder.Header().Get("X-Upstream") != "ok" {
		t.Fatalf("expected non-CORS upstream headers to remain copied, got %q", recorder.Header().Get("X-Upstream"))
	}
}

func TestStreamResponsesAsChatHandlerNoDuplicateContentFromCompleted(t *testing.T) {
	ctx, recorder := newBridgeTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(
			"event: response.output_text.delta\n" +
				"data: {\"type\":\"response.output_text.delta\",\"delta\":\"OK\"}\n\n" +
				"event: response.output_text.done\n" +
				"data: {\"type\":\"response.output_text.done\",\"text\":\"OK\"}\n\n" +
				"event: response.completed\n" +
				"data: {\"type\":\"response.completed\",\"output_text\":\"OK\",\"response\":{\"usage\":{\"input_tokens\":20,\"output_tokens\":5,\"total_tokens\":25}}}\n\n" +
				"data: [DONE]\n",
		)),
	}

	relayErr, usage := StreamResponsesAsChatHandler(ctx, resp, "gpt-5.4", 20)
	if relayErr != nil {
		t.Fatalf("StreamResponsesAsChatHandler returned error: %+v", relayErr)
	}
	if usage == nil || usage.PromptTokens != 20 || usage.CompletionTokens != 5 || usage.TotalTokens != 25 {
		t.Fatalf("unexpected usage: %#v", usage)
	}

	body := recorder.Body.String()
	if strings.Count(body, `"content":"OK"`) != 1 {
		t.Fatalf("expected exactly one content chunk, got body: %s", body)
	}
	if !strings.Contains(body, `"finish_reason":"stop"`) {
		t.Fatalf("expected finish chunk, got body: %s", body)
	}
}

func TestStreamResponsesAsChatHandlerUsesCompletedTextWhenNoDelta(t *testing.T) {
	ctx, recorder := newBridgeTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(
			"event: response.completed\n" +
				"data: {\"type\":\"response.completed\",\"output_text\":\"ONLY_COMPLETED\",\"response\":{\"usage\":{\"input_tokens\":10,\"output_tokens\":4,\"total_tokens\":14}}}\n\n" +
				"data: [DONE]\n",
		)),
	}

	relayErr, usage := StreamResponsesAsChatHandler(ctx, resp, "gpt-5.4", 10)
	if relayErr != nil {
		t.Fatalf("StreamResponsesAsChatHandler returned error: %+v", relayErr)
	}
	if usage == nil || usage.PromptTokens != 10 || usage.CompletionTokens != 4 || usage.TotalTokens != 14 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"content":"ONLY_COMPLETED"`) {
		t.Fatalf("expected completed text to be forwarded, got body: %s", body)
	}
}

func TestStreamResponsesAsChatHandlerUsesCompletedOutputWhenNoDelta(t *testing.T) {
	ctx, recorder := newBridgeTestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(
			"event: response.completed\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_stream_completed\",\"model\":\"gpt-5.4\",\"created_at\":1710000000,\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"hello from stream completed\"}]}],\"usage\":{\"input_tokens\":10,\"output_tokens\":6,\"total_tokens\":16}}}\n\n" +
				"data: [DONE]\n",
		)),
	}

	relayErr, usage := StreamResponsesAsChatHandler(ctx, resp, "gpt-5.4", 10)
	if relayErr != nil {
		t.Fatalf("StreamResponsesAsChatHandler returned error: %+v", relayErr)
	}
	if usage == nil || usage.PromptTokens != 10 || usage.CompletionTokens != 6 || usage.TotalTokens != 16 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"content":"hello from stream completed"`) {
		t.Fatalf("expected completed output text to be forwarded, got body: %s", body)
	}
	if !strings.Contains(body, `"finish_reason":"stop"`) {
		t.Fatalf("expected finish chunk, got body: %s", body)
	}
}
