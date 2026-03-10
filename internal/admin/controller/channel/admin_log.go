package channel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/logger"
)

func logChannelAdminInfo(c *gin.Context, action string, fields ...string) {
	logChannelAdmin(c, true, action, fields...)
}

func logChannelAdminWarn(c *gin.Context, action string, fields ...string) {
	logChannelAdmin(c, false, action, fields...)
}

func logChannelAdmin(c *gin.Context, info bool, action string, fields ...string) {
	if c == nil {
		return
	}
	parts := make([]string, 0, len(fields)+3)
	parts = append(parts, "[channel-admin]")
	if normalizedAction := strings.TrimSpace(action); normalizedAction != "" {
		parts = append(parts, "action="+normalizedAction)
	}
	if operator := strings.TrimSpace(c.GetString(ctxkey.Id)); operator != "" {
		parts = append(parts, "operator="+operator)
	}
	for _, field := range fields {
		normalized := strings.TrimSpace(field)
		if normalized == "" {
			continue
		}
		parts = append(parts, normalized)
	}
	message := strings.Join(parts, " ")
	if info {
		logger.Info(c.Request.Context(), message)
		return
	}
	logger.Warn(c.Request.Context(), message)
}

func stringField(key string, value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return ""
	}
	return fmt.Sprintf("%s=%s", key, normalized)
}

func quotedField(key string, value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return ""
	}
	return fmt.Sprintf("%s=%q", key, normalized)
}

func structuredPayloadField(key string, value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return ""
	}
	pretty := normalized
	if (strings.HasPrefix(normalized, "{") || strings.HasPrefix(normalized, "[")) && json.Valid([]byte(normalized)) {
		buffer := bytes.NewBuffer(nil)
		if err := json.Indent(buffer, []byte(normalized), "", "  "); err == nil {
			pretty = buffer.String()
		}
	}
	return fmt.Sprintf("%s=\n%s", key, pretty)
}

func intField(key string, value int) string {
	return fmt.Sprintf("%s=%d", key, value)
}

func int64Field(key string, value int64) string {
	return fmt.Sprintf("%s=%d", key, value)
}

func floatField(key string, value float64) string {
	return fmt.Sprintf("%s=%.6f", key, value)
}
