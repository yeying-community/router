package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newOpenAITestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	return ctx, recorder
}

func TestRelayResponsesResponseSkipsUpstreamCORSHeaders(t *testing.T) {
	ctx, recorder := newOpenAITestContext()
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
	if usage.ImageGenerationCalls != 0 {
		t.Fatalf("unexpected image generation calls: %#v", usage)
	}
	allowOriginValues := recorder.Header().Values("Access-Control-Allow-Origin")
	if len(allowOriginValues) != 1 || allowOriginValues[0] != "http://localhost:3020" {
		t.Fatalf("expected CORS allow-origin header to remain middleware value, got %#v", allowOriginValues)
	}
	if recorder.Header().Get("X-Upstream") != "ok" {
		t.Fatalf("expected non-CORS upstream headers to remain copied, got %q", recorder.Header().Get("X-Upstream"))
	}
}

func TestRelayResponsesResponseCapturesImageGenerationCalls(t *testing.T) {
	ctx, _ := newOpenAITestContext()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"resp_img_2",
			"object":"response",
			"model":"gpt-5",
			"output":[{"type":"image_generation_call"}],
			"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}
		}`)),
	}

	usage, relayErr := relayResponsesResponse(ctx, resp)
	if relayErr != nil {
		t.Fatalf("relayResponsesResponse returned error: %+v", relayErr)
	}
	if usage == nil || usage.ImageGenerationCalls != 1 {
		t.Fatalf("unexpected image generation calls: %#v", usage)
	}
}

func TestRelayMessagesResponseSkipsUpstreamCORSHeaders(t *testing.T) {
	ctx, recorder := newOpenAITestContext()
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
