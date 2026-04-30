package openai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/meta"
)

func TestSetupRequestHeaderSetsJSONAcceptForNonStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("Accept", "text/event-stream")
	ctx.Request.Header.Set("Content-Type", "application/json")

	req := httptest.NewRequest(http.MethodPost, "https://example.com/v1/chat/completions", nil)
	adaptor := &Adaptor{ChannelProtocol: relaychannel.OpenAI}
	meta := &meta.Meta{APIKey: "sk-test", ChannelProtocol: relaychannel.OpenAI}

	if err := adaptor.SetupRequestHeader(ctx, req, meta); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}
	if got := req.Header.Get("Accept"); got != "application/json" {
		t.Fatalf("Accept = %q, want %q", got, "application/json")
	}
}

func TestSetupRequestHeaderSetsSSEAcceptForStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("Accept", "application/json")
	ctx.Request.Header.Set("Content-Type", "application/json")

	req := httptest.NewRequest(http.MethodPost, "https://example.com/v1/chat/completions", nil)
	adaptor := &Adaptor{ChannelProtocol: relaychannel.OpenAI}
	meta := &meta.Meta{
		APIKey:          "sk-test",
		ChannelProtocol: relaychannel.OpenAI,
		IsStream:        true,
	}

	if err := adaptor.SetupRequestHeader(ctx, req, meta); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}
	if got := req.Header.Get("Accept"); got != "text/event-stream" {
		t.Fatalf("Accept = %q, want %q", got, "text/event-stream")
	}
}
