package router

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/config"
)

//go:embed web/dist/index.html
var testBuildFS embed.FS

func TestSetWebRouterServesSPAIndexForNonAPINoRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	restore := configureRouterTestState(t)
	defer restore()

	engine := gin.New()
	SetWebRouter(engine, testBuildFS, []byte("<html>router test index</html>"))

	req := httptest.NewRequest(http.MethodGet, "/workspaces/demo", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-cache")
	}
	if got := recorder.Body.String(); got != "<html>router test index</html>" {
		t.Fatalf("body = %q, want SPA index page", got)
	}
}

func TestSetWebRouterReturnsRelayNotFoundForAPINoRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	restore := configureRouterTestState(t)
	defer restore()

	engine := gin.New()
	SetWebRouter(engine, testBuildFS, []byte("<html>router test index</html>"))

	req := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate" {
		t.Fatalf("Cache-Control = %q, want relay no-cache header", got)
	}
}

func TestSetRouterRedirectsToFrontendBaseURLWhenConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)

	restore := configureRouterTestState(t)
	defer restore()

	common.FrontendBaseURL = "https://router.example.com/ui/"
	common.DisableOpenAICompat = true
	config.IsMasterNode = false

	engine := gin.New()
	SetRouter(engine, testBuildFS)

	req := httptest.NewRequest(http.MethodGet, "/admin/channels", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusMovedPermanently {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusMovedPermanently)
	}
	if got := recorder.Header().Get("Location"); got != "https://router.example.com/ui/admin/channels" {
		t.Fatalf("Location = %q, want %q", got, "https://router.example.com/ui/admin/channels")
	}
}

func TestSetRouterIgnoresFrontendBaseURLOnMasterNode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	restore := configureRouterTestState(t)
	defer restore()

	common.FrontendBaseURL = "https://router.example.com/ui/"
	common.DisableOpenAICompat = true
	config.IsMasterNode = true

	engine := gin.New()
	SetRouter(engine, testBuildFS)

	req := httptest.NewRequest(http.MethodGet, "/admin/channels", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Location"); got != "" {
		t.Fatalf("Location = %q, want empty", got)
	}
	if got := recorder.Body.String(); got != "<!doctype html><title>router test embed</title>" {
		t.Fatalf("body = %q, want embedded index page", got)
	}
}

func configureRouterTestState(t *testing.T) func() {
	t.Helper()

	previousFrontendBaseURL := common.FrontendBaseURL
	previousDisableOpenAICompat := common.DisableOpenAICompat
	previousIsMasterNode := config.IsMasterNode
	previousGlobalWebRateLimitNum := config.GlobalWebRateLimitNum
	previousGlobalWebRateLimitDuration := config.GlobalWebRateLimitDuration

	config.GlobalWebRateLimitNum = 0
	config.GlobalWebRateLimitDuration = 0

	return func() {
		common.FrontendBaseURL = previousFrontendBaseURL
		common.DisableOpenAICompat = previousDisableOpenAICompat
		config.IsMasterNode = previousIsMasterNode
		config.GlobalWebRateLimitNum = previousGlobalWebRateLimitNum
		config.GlobalWebRateLimitDuration = previousGlobalWebRateLimitDuration
	}
}
