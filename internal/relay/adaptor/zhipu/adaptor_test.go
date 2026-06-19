package zhipu

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/internal/relay/meta"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

func TestGetRequestURLMessagesUsesAnthropicCompatibleEndpoint(t *testing.T) {
	adaptor := &Adaptor{}
	requestMeta := &meta.Meta{
		BaseURL:      "https://open.bigmodel.cn",
		Mode:         relaymode.Messages,
		UpstreamMode: relaymode.Messages,
	}

	got, err := adaptor.GetRequestURL(requestMeta)
	if err != nil {
		t.Fatalf("GetRequestURL() error = %v", err)
	}
	if want := "https://open.bigmodel.cn/api/anthropic/v1/messages"; got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}

func TestSetupRequestHeaderMessagesUsesXAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	adaptor := &Adaptor{}
	req := httptest.NewRequest(http.MethodPost, "https://open.bigmodel.cn/api/anthropic/v1/messages", nil)
	requestMeta := &meta.Meta{
		APIKey:       "test-key",
		IsStream:     true,
		Mode:         relaymode.Messages,
		UpstreamMode: relaymode.Messages,
	}

	if err := adaptor.SetupRequestHeader(ctx, req, requestMeta); err != nil {
		t.Fatalf("SetupRequestHeader() error = %v", err)
	}
	if got := req.Header.Get("x-api-key"); got != "test-key" {
		t.Fatalf("x-api-key = %q, want %q", got, "test-key")
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want empty", got)
	}
	if got := req.Header.Get("anthropic-version"); got != "2023-06-01" {
		t.Fatalf("anthropic-version = %q, want %q", got, "2023-06-01")
	}
	if got := req.Header.Get("Accept"); got != "text/event-stream" {
		t.Fatalf("Accept = %q, want %q", got, "text/event-stream")
	}
}
