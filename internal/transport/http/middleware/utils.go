package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
)

func abortWithMessage(c *gin.Context, statusCode int, message string) {
	c.Set(ctxkey.RelayError, strings.TrimSpace(message))
	c.Set(ctxkey.RelayErrorType, "one_api_error")
	c.Set(ctxkey.RelayErrorCode, "request_aborted")
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": helper.MessageWithTraceID(message, c.GetString(helper.TraceIDKey)),
			"type":    "one_api_error",
		},
	})
	c.Abort()
	logger.Warnf(c.Request.Context(), "request aborted status=%d reason=%q path=%s", statusCode, strings.TrimSpace(message), c.Request.URL.Path)
}

func normalizeRelayPath(path string) string {
	if strings.HasPrefix(path, "/api/v1/public/") {
		return "/v1/" + strings.TrimPrefix(path, "/api/v1/public/")
	}
	return path
}

func getRequestModel(c *gin.Context) (string, error) {
	var modelRequest ModelRequest
	err := common.UnmarshalBodyReusable(c, &modelRequest)
	if err != nil {
		return "", fmt.Errorf("common.UnmarshalBodyReusable failed: %w", err)
	}
	path := normalizeRelayPath(c.Request.URL.Path)
	if strings.HasPrefix(path, "/v1/moderations") {
		if modelRequest.Model == "" {
			modelRequest.Model = "text-moderation-stable"
		}
	}
	if strings.HasSuffix(path, "embeddings") {
		if modelRequest.Model == "" {
			modelRequest.Model = c.Param("model")
		}
	}
	if strings.HasPrefix(path, "/v1/images/generations") {
		if modelRequest.Model == "" {
			modelRequest.Model = "dall-e-2"
		}
	}
	if strings.HasPrefix(path, "/v1/audio/transcriptions") || strings.HasPrefix(path, "/v1/audio/translations") {
		if modelRequest.Model == "" {
			modelRequest.Model = "whisper-1"
		}
	}
	if strings.HasPrefix(path, "/v1/videos") && modelRequest.Model == "" {
		if modelValue := strings.TrimSpace(c.Query("model")); modelValue != "" {
			modelRequest.Model = modelValue
		} else if modelValue := strings.TrimSpace(c.PostForm("model")); modelValue != "" {
			modelRequest.Model = modelValue
		}
	}
	return modelRequest.Model, nil
}

func isModelInList(modelName string, models string) bool {
	modelList := strings.Split(models, ",")
	for _, model := range modelList {
		if modelName == model {
			return true
		}
	}
	return false
}
