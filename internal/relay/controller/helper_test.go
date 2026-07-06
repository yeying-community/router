package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

func TestGetAndValidateTextRequestMessagesPreservesMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{
		"model":"claude-opus-4-6",
		"system":[{"type":"text","text":"system prompt"}],
		"messages":[
			{"role":"user","content":"hello"},
			{"role":"assistant","content":[{"type":"text","text":"world"}]}
		],
		"max_tokens":128,
		"stream":true
	}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	request, rawBody, err := getAndValidateTextRequest(ctx, relaymode.Messages)
	if err != nil {
		t.Fatalf("getAndValidateTextRequest returned error: %v", err)
	}
	if len(rawBody) == 0 {
		t.Fatalf("rawBody is empty")
	}
	if request == nil {
		t.Fatalf("request is nil")
	}
	if request.Model != "claude-opus-4-6" {
		t.Fatalf("request.Model = %q, want %q", request.Model, "claude-opus-4-6")
	}
	if len(request.Messages) != 3 {
		t.Fatalf("len(request.Messages) = %d, want 3", len(request.Messages))
	}
	if request.Messages[0].Role != "system" {
		t.Fatalf("request.Messages[0].Role = %q, want system", request.Messages[0].Role)
	}
	if request.Messages[1].Role != "user" || request.Messages[1].StringContent() != "hello" {
		t.Fatalf("unexpected user message: %#v", request.Messages[1])
	}
	if request.Messages[2].Role != "assistant" || request.Messages[2].StringContent() != "world" {
		t.Fatalf("unexpected assistant message: %#v", request.Messages[2])
	}
}

func TestResolveTextMaxOutputTokensUsesLargestLimit(t *testing.T) {
	maxCompletionTokens := 256
	maxOutputTokens := 384
	got := resolveTextMaxOutputTokens(&relaymodel.GeneralOpenAIRequest{
		MaxTokens:           128,
		MaxCompletionTokens: &maxCompletionTokens,
		MaxOutputTokens:     &maxOutputTokens,
	})
	if got != 384 {
		t.Fatalf("resolveTextMaxOutputTokens() = %d, want 384", got)
	}
}
