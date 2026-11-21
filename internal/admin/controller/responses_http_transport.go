package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/logger"
)

// ResponsesHTTPTransport rejects the unsupported WebSocket handshake explicitly
// so compatible clients can use the authenticated POST Responses endpoint.
// It never upgrades a connection, forwards credentials or selects a channel.
func ResponsesHTTPTransport(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Allow", http.MethodPost)
	upgrade := false
	for _, token := range strings.Split(c.GetHeader("Connection"), ",") {
		upgrade = upgrade || strings.EqualFold(strings.TrimSpace(token), "upgrade")
	}
	if upgrade && strings.EqualFold(strings.TrimSpace(c.GetHeader("Upgrade")), "websocket") {
		logger.RelayInfof(c.Request.Context(), "TRANSPORT decision=use_http reason=responses_websocket_disabled")
		c.JSON(http.StatusUpgradeRequired, gin.H{"error": gin.H{
			"type":    "invalid_request_error",
			"code":    "websocket_not_supported",
			"message": "Use HTTP POST /v1/responses for this endpoint.",
		}})
		return
	}
	c.JSON(http.StatusMethodNotAllowed, gin.H{"error": gin.H{
		"type":    "invalid_request_error",
		"code":    "method_not_allowed",
		"message": "Use HTTP POST /v1/responses for this endpoint.",
	}})
}
