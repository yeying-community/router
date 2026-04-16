package openai

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func shouldSkipUpstreamResponseHeader(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return true
	}
	if strings.HasPrefix(normalized, "access-control-") {
		return true
	}
	switch normalized {
	case "connection", "content-length", "transfer-encoding":
		return true
	default:
		return false
	}
}

func copyUpstreamResponseHeaders(c *gin.Context, source http.Header, skipContentType bool) {
	for key, values := range source {
		if len(values) == 0 {
			continue
		}
		if skipContentType && strings.EqualFold(key, "Content-Type") {
			continue
		}
		if shouldSkipUpstreamResponseHeader(key) {
			continue
		}
		c.Writer.Header().Set(key, values[0])
	}
}
