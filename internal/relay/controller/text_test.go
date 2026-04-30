package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/internal/relay/meta"
)

func TestLogTextStreamAcceptConflictIgnoresAlignedStreamAndAccept(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	ctx.Request.Header.Set("Accept", "text/event-stream")

	logTextStreamAcceptConflict(ctx, &meta.Meta{IsStream: true})
}

func TestLogTextStreamAcceptConflictIgnoresEmptyAccept(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)

	logTextStreamAcceptConflict(ctx, &meta.Meta{IsStream: false})
}
