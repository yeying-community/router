package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/i18n"
)

func TestTraceIDPrefersExplicitHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(TraceID())
	engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, helper.GetTraceID(c.Request.Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(helper.TraceIDKey, "trace-explicit")
	req.Header.Set(helper.TraceParentHeader, "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")
	req.Header.Set(helper.XRequestIDHeader, "trace-request-id")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get(helper.TraceIDKey); got != "trace-explicit" {
		t.Fatalf("response trace id = %q, want %q", got, "trace-explicit")
	}
	if got := recorder.Body.String(); got != "trace-explicit" {
		t.Fatalf("context trace id = %q, want %q", got, "trace-explicit")
	}
}

func TestTraceIDFallsBackToTraceParent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(TraceID())
	engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, helper.GetTraceID(c.Request.Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(helper.TraceParentHeader, "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if got := recorder.Body.String(); got != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("context trace id = %q, want traceparent id", got)
	}
}

func TestTraceIDFallsBackToXRequestIDWhenTraceParentInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(TraceID())
	engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, helper.GetTraceID(c.Request.Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(helper.TraceParentHeader, "00-short-0123456789abcdef-01")
	req.Header.Set(helper.XRequestIDHeader, "trace-request-id")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if got := recorder.Body.String(); got != "trace-request-id" {
		t.Fatalf("context trace id = %q, want %q", got, "trace-request-id")
	}
}

func TestLanguageNormalizesChineseHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(Language())
	engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, i18n.GetLang(c))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en;q=0.8")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if got := recorder.Body.String(); got != "zh-CN" {
		t.Fatalf("language = %q, want %q", got, "zh-CN")
	}
}

func TestLanguageDefaultsToEnglish(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(Language())
	engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, i18n.GetLang(c))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if got := recorder.Body.String(); got != "en" {
		t.Fatalf("language = %q, want %q", got, "en")
	}
}
