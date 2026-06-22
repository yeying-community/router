package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

func TestRelayNotFoundDisablesCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/group/channel-options", nil)
	c.Request = req

	RelayNotFound(c)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unexpected status code: got %d want %d", recorder.Code, http.StatusNotFound)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate" {
		t.Fatalf("unexpected Cache-Control header: got %q", got)
	}
	if got := recorder.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("unexpected Pragma header: got %q", got)
	}
	if got := recorder.Header().Get("Expires"); got != "0" {
		t.Fatalf("unexpected Expires header: got %q", got)
	}
}

func TestShouldRetrySkipsStatefulResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request = req
	c.Set(ctxkey.ResponsesStatefulRequest, true)

	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusTooManyRequests,
	}
	if shouldRetry(c, err) {
		t.Fatal("shouldRetry returned true for stateful responses request, want false")
	}
}
