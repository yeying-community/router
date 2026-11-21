package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResponsesHTTPTransport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name, connection, upgrade string
		status                    int
	}{
		{"websocket", "keep-alive, Upgrade", "WebSocket", http.StatusUpgradeRequired},
		{"plain GET", "", "", http.StatusMethodNotAllowed},
		{"incomplete handshake", "keep-alive", "websocket", http.StatusMethodNotAllowed},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
			c.Request.Header.Set("Connection", test.connection)
			c.Request.Header.Set("Upgrade", test.upgrade)
			c.Request.Header.Set("Authorization", "Bearer dummy-do-not-echo")
			ResponsesHTTPTransport(c)
			if recorder.Code != test.status || recorder.Header().Get("Allow") != http.MethodPost ||
				recorder.Header().Get("Cache-Control") != "no-store" ||
				strings.Contains(recorder.Body.String(), "dummy-do-not-echo") {
				t.Fatal("wrong transport negotiation or credential echoed")
			}
		})
	}
}
