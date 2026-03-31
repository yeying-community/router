package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	dbmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/monitor"
	"github.com/yeying-community/router/internal/relay/controller"
	relaylogging "github.com/yeying-community/router/internal/relay/logging"
	"github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
	"github.com/yeying-community/router/internal/transport/http/middleware"
)

// https://platform.openai.com/docs/api-reference/chat

func relayHelper(c *gin.Context, relayMode int) *model.ErrorWithStatusCode {
	var err *model.ErrorWithStatusCode
	switch relayMode {
	case relaymode.ImagesGenerations:
		err = controller.RelayImageHelper(c, relayMode)
	case relaymode.AudioSpeech:
		fallthrough
	case relaymode.AudioTranslation:
		fallthrough
	case relaymode.AudioTranscription:
		err = controller.RelayAudioHelper(c, relayMode)
	case relaymode.Videos:
		err = controller.RelayVideoHelper(c, relayMode)
	case relaymode.Proxy:
		err = controller.RelayProxyHelper(c, relayMode)
	default:
		err = controller.RelayTextHelper(c)
	}
	return err
}

// Relay godoc
// @Summary OpenAI-compatible relay
// @Tags public
// @Security BearerAuth
// @Accept json
// @Produce json
func Relay(c *gin.Context) {
	ctx := c.Request.Context()
	c.Set(ctxkey.RelayRetryCount, 0)
	c.Set(ctxkey.RelayError, "")
	relayMode := getEffectiveRelayMode(c)
	if config.DebugEnabled {
		requestBody, _ := common.GetRequestBody(c)
		logger.Debugf(ctx, "request body: %s", string(requestBody))
	}
	channelId := c.GetString(ctxkey.ChannelId)
	userId := c.GetString(ctxkey.Id)
	requestPath := c.Request.URL.Path
	bizErr := relayHelper(c, relayMode)
	if bizErr == nil {
		monitor.Emit(channelId, true)
		return
	}
	lastFailedChannelId := channelId
	channelName := c.GetString(ctxkey.ChannelName)
	group := c.GetString(ctxkey.Group)
	originalModel := c.GetString(ctxkey.OriginalModel)
	failedChannelIDs := map[string]struct{}{}
	if trimmedChannelID := strings.TrimSpace(channelId); trimmedChannelID != "" {
		failedChannelIDs[trimmedChannelID] = struct{}{}
	}
	go processChannelRelayError(ctx, userId, channelId, channelName, originalModel, requestPath, *bizErr)
	traceID := c.GetString(helper.TraceIDKey)
	retryTimes := config.RetryTimes
	retryCount := 0
	retryable := shouldRetry(c, bizErr)
	if !retryable {
		logger.RelayWarnf(ctx, relaylogging.NewFields("RETRY").
			String("decision", "skip").
			Int("status", bizErr.StatusCode).
			String("channel_id", channelId).
			String("channel_name", channelName).
			String("group", group).
			String("model", originalModel).
			String("reason", "status_not_retryable").
			Build())
		retryTimes = 0
	}
	if retryable && retryTimes <= 0 {
		logger.RelayWarnf(ctx, relaylogging.NewFields("RETRY").
			String("decision", "skip").
			Int("status", bizErr.StatusCode).
			String("channel_id", channelId).
			String("channel_name", channelName).
			String("group", group).
			String("model", originalModel).
			String("reason", "retry_disabled").
			Build())
	}
	for i := retryTimes; i > 0; i-- {
		channel, selectionStats, err := dbmodel.CacheSelectRandomSatisfiedChannelForRequestExcluding(group, originalModel, requestPath, false, failedChannelIDs)
		if err != nil {
			fields := relaylogging.NewFields("RETRY").
				String("decision", "select_failed").
				String("group", group).
				String("model", originalModel).
				String("reason", resolveRetrySelectionFailureReason(selectionStats)).
				String("selection_scope", selectionStats.SelectionScope).
				Int("total_candidates", selectionStats.TotalCandidates).
				Int("remaining_candidates", selectionStats.RemainingCandidates).
				Int("selected_priority", priorityToInt(selectionStats.SelectedPriority)).
				Int("tier_candidates", selectionStats.SelectedTierCandidates).
				Int("failed_channels", len(failedChannelIDs)).
				String("error", err.Error())
			if resolveRetrySelectionFailureReason(selectionStats) == "selector_error" {
				logger.RelayErrorf(ctx, fields.Build())
			} else {
				logger.RelayWarnf(ctx, fields.Build())
			}
			break
		}
		retryCount++
		c.Set(ctxkey.RelayRetryCount, retryCount)
		logger.RelayWarnf(ctx, relaylogging.NewFields("RETRY").
			String("decision", "switch").
			Int("attempt", retryCount).
			String("group", group).
			String("model", originalModel).
			String("from_channel_id", lastFailedChannelId).
			String("to_channel_id", channel.Id).
			String("to_channel_name", channel.DisplayName()).
			String("selection_scope", selectionStats.SelectionScope).
			Int("selected_priority", priorityToInt(selectionStats.SelectedPriority)).
			Int("tier_candidates", selectionStats.SelectedTierCandidates).
			Int("remaining_candidates", selectionStats.RemainingCandidates).
			Int("total_candidates", selectionStats.TotalCandidates).
			Int("failed_channels", len(failedChannelIDs)).
			Int("remaining", i-1).
			Build())
		middleware.SetupContextForSelectedChannel(c, channel, originalModel)
		requestBody, err := common.GetRequestBody(c)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		relayMode = getEffectiveRelayMode(c)
		bizErr = relayHelper(c, relayMode)
		if bizErr == nil {
			return
		}
		channelId := c.GetString(ctxkey.ChannelId)
		lastFailedChannelId = channelId
		channelName := c.GetString(ctxkey.ChannelName)
		if trimmedChannelID := strings.TrimSpace(channelId); trimmedChannelID != "" {
			failedChannelIDs[trimmedChannelID] = struct{}{}
		}
		go processChannelRelayError(ctx, userId, channelId, channelName, originalModel, requestPath, *bizErr)
	}
	if bizErr != nil {
		upstreamStatus := bizErr.StatusCode
		normalizeFinalRelayError(bizErr)
		c.Set(ctxkey.RelayError, bizErr.Error.Message)
		logger.RelayErrorf(ctx, relaylogging.NewFields("FAIL").
			Int("status", bizErr.StatusCode).
			Int("upstream_status", upstreamStatus).
			String("channel_id", lastFailedChannelId).
			String("channel_name", channelName).
			String("group", group).
			String("model", originalModel).
			Int("retry_count", retryCount).
			String("error", bizErr.Error.Message).
			Build())

		// BUG: bizErr is in race condition
		bizErr.Error.Message = helper.MessageWithTraceID(bizErr.Error.Message, traceID)
		c.JSON(bizErr.StatusCode, gin.H{
			"error": bizErr.Error,
		})
	}
}

func getEffectiveRelayMode(c *gin.Context) int {
	return relaymode.GetByPath(c.Request.URL.Path)
}

func shouldRetry(c *gin.Context, bizErr *model.ErrorWithStatusCode) bool {
	if bizErr == nil {
		return false
	}
	if _, ok := c.Get(ctxkey.SpecificChannelId); ok {
		return false
	}
	if controller.IsGroupDailyQuotaExceededError(bizErr) {
		return false
	}
	statusCode := bizErr.StatusCode
	if statusCode == http.StatusPaymentRequired {
		return true
	}
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	if statusCode/100 == 5 {
		return true
	}
	if statusCode == http.StatusBadRequest {
		return false
	}
	if statusCode/100 == 2 {
		return false
	}
	return true
}

func normalizeFinalRelayError(err *model.ErrorWithStatusCode) {
	if err == nil {
		return
	}
	if isLocalQuotaRelayError(err) {
		return
	}
	if isUpstreamQuotaRelayError(err) {
		err.StatusCode = http.StatusServiceUnavailable
		err.Error.Message = "当前分组可用上游额度不足，请稍后再试"
		return
	}
	if !isTransientUpstreamRelayError(err) {
		return
	}
	err.StatusCode = http.StatusServiceUnavailable
	err.Error.Message = "当前分组可用上游暂时不可用，请稍后再试"
}

func isLocalQuotaRelayError(err *model.ErrorWithStatusCode) bool {
	if err == nil {
		return false
	}
	code := strings.ToLower(errorCodeString(err.Code))
	return code == "group_daily_quota_exceeded" || code == "user_quota_limit_exceeded"
}

func isUpstreamQuotaRelayError(err *model.ErrorWithStatusCode) bool {
	if err == nil {
		return false
	}
	if err.StatusCode == http.StatusPaymentRequired {
		return true
	}
	lowerType := strings.ToLower(strings.TrimSpace(err.Type))
	if lowerType == "insufficient_quota" || lowerType == "billing_error" {
		return true
	}
	lowerCode := strings.ToLower(errorCodeString(err.Code))
	if lowerCode == "insufficient_quota" || lowerCode == "billing_hard_limit_reached" {
		return true
	}
	lowerMessage := strings.ToLower(strings.TrimSpace(err.Message))
	return strings.Contains(lowerMessage, "额度") ||
		strings.Contains(lowerMessage, "欠费") ||
		strings.Contains(lowerMessage, "quota") ||
		strings.Contains(lowerMessage, "credit") ||
		strings.Contains(lowerMessage, "balance") ||
		strings.Contains(lowerMessage, "daily limit")
}

func isTransientUpstreamRelayError(err *model.ErrorWithStatusCode) bool {
	if err == nil {
		return false
	}
	if err.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if err.StatusCode >= http.StatusInternalServerError {
		if err.Type != "one_api_error" {
			return true
		}
		return errorCodeString(err.Code) == "do_request_failed"
	}
	return false
}

func errorCodeString(code any) string {
	return strings.TrimSpace(fmt.Sprint(code))
}

func priorityToInt(priority int64) int {
	return int(priority)
}

func resolveRetrySelectionFailureReason(stats dbmodel.SatisfiedChannelSelectionStats) string {
	if stats.TotalCandidates == 0 {
		return "no_candidates"
	}
	if stats.RemainingCandidates == 0 {
		return "candidate_exhausted"
	}
	return "selector_error"
}

func processChannelRelayError(ctx context.Context, userId string, channelId string, channelName string, requestModel string, requestPath string, err model.ErrorWithStatusCode) {
	msg := relaylogging.NewFields("UPSTREAM_ERR").
		String("channel_id", channelId).
		String("channel_name", channelName).
		String("model", requestModel).
		String("user_id", userId).
		Int("status", err.StatusCode).
		String("error", err.Message).
		Build()
	if err.StatusCode >= http.StatusInternalServerError {
		logger.RelayErrorf(ctx, msg)
	} else {
		logger.RelayWarnf(ctx, msg)
	}
	if shouldDisableChannelModelRequestEndpointCapability(&err) {
		disabled, disableErr := dbmodel.DisableChannelModelRequestEndpointCapability(channelId, requestModel, requestPath)
		logChannelModelRequestEndpointDisableResult(ctx, channelId, channelName, requestModel, requestPath, err, disabled, disableErr)
		if disableErr != nil {
			monitor.Emit(channelId, false)
		}
		return
	}
	// https://platform.openai.com/docs/guides/error-codes/api-errors
	if shouldDisableChannelModelCapability(&err) {
		disabled, disableErr := dbmodel.DisableChannelModelCapability(channelId, requestModel)
		logChannelModelCapabilityDisableResult(ctx, channelId, channelName, requestModel, err, disabled, disableErr)
		if disableErr != nil {
			monitor.Emit(channelId, false)
		}
		return
	}
	if monitor.ShouldDisableChannel(&err.Error, err.StatusCode) {
		monitor.DisableChannel(channelId, channelName, err.Message)
	} else {
		monitor.Emit(channelId, false)
	}
}

func shouldDisableChannelModelCapability(err *model.ErrorWithStatusCode) bool {
	if err == nil {
		return false
	}
	code := errorCodeString(err.Code)
	if code == "unsupported_channel_endpoint" {
		return false
	}
	if code == "model_not_found" {
		return true
	}

	lowerType := strings.ToLower(strings.TrimSpace(err.Type))
	lowerMessage := strings.ToLower(strings.TrimSpace(err.Message))
	if lowerType == "permission_error" && isModelScopedPermissionMessage(lowerMessage) {
		return true
	}
	if lowerType == "invalid_request_error" && isModelNotFoundMessage(lowerMessage) {
		return true
	}
	return false
}

func shouldDisableChannelModelRequestEndpointCapability(err *model.ErrorWithStatusCode) bool {
	if err == nil {
		return false
	}
	return errorCodeString(err.Code) == "unsupported_channel_endpoint"
}

func isModelScopedPermissionMessage(message string) bool {
	if message == "" || !strings.Contains(message, "model") {
		return false
	}
	return strings.Contains(message, "access") ||
		strings.Contains(message, "permission") ||
		strings.Contains(message, "not allowed")
}

func isModelNotFoundMessage(message string) bool {
	if message == "" || !strings.Contains(message, "model") {
		return false
	}
	return strings.Contains(message, "not found") ||
		strings.Contains(message, "does not exist") ||
		strings.Contains(message, "unknown model") ||
		strings.Contains(message, "unsupported model")
}

func logChannelModelCapabilityDisableResult(ctx context.Context, channelId string, channelName string, requestModel string, relayErr model.ErrorWithStatusCode, disabled bool, disableErr error) {
	fields := relaylogging.NewFields("MODEL_CAPABILITY_DISABLE").
		String("channel_id", channelId).
		String("channel_name", channelName).
		String("model", requestModel).
		Int("status", relayErr.StatusCode).
		String("error_type", relayErr.Type).
		String("error_code", errorCodeString(relayErr.Code)).
		String("reason", relayErr.Message)
	if disableErr != nil {
		logger.RelayErrorf(ctx, fields.String("result", "failed").String("disable_error", disableErr.Error()).Build())
		return
	}
	if disabled {
		logger.RelayWarnf(ctx, fields.String("result", "disabled").Build())
		return
	}
	logger.RelayWarnf(ctx, fields.String("result", "noop").Build())
}

func logChannelModelRequestEndpointDisableResult(ctx context.Context, channelId string, channelName string, requestModel string, requestPath string, relayErr model.ErrorWithStatusCode, disabled bool, disableErr error) {
	fields := relaylogging.NewFields("MODEL_ENDPOINT_DISABLE").
		String("channel_id", channelId).
		String("channel_name", channelName).
		String("model", requestModel).
		String("endpoint", dbmodel.NormalizeRequestedChannelModelEndpoint(requestPath)).
		Int("status", relayErr.StatusCode).
		String("error_type", relayErr.Type).
		String("error_code", errorCodeString(relayErr.Code)).
		String("reason", relayErr.Message)
	if disableErr != nil {
		logger.RelayErrorf(ctx, fields.String("result", "failed").String("disable_error", disableErr.Error()).Build())
		return
	}
	if disabled {
		logger.RelayWarnf(ctx, fields.String("result", "disabled").Build())
		return
	}
	logger.RelayWarnf(ctx, fields.String("result", "noop").Build())
}

// RelayNotImplemented godoc
// @Summary OpenAI-compatible endpoint not implemented
// @Tags public
// @Security BearerAuth
// @Produce json
// @Success 501 {object} docs.OpenAIErrorResponse
// @Router /api/v1/public/images/edits [post]
// @Router /api/v1/public/images/variations [post]
// @Router /api/v1/public/files [get]
// @Router /api/v1/public/files [post]
// @Router /api/v1/public/files/{id} [delete]
// @Router /api/v1/public/files/{id} [get]
// @Router /api/v1/public/files/{id}/content [get]
// @Router /api/v1/public/fine_tuning/jobs [post]
// @Router /api/v1/public/fine_tuning/jobs [get]
// @Router /api/v1/public/fine_tuning/jobs/{id} [get]
// @Router /api/v1/public/fine_tuning/jobs/{id}/cancel [post]
// @Router /api/v1/public/fine_tuning/jobs/{id}/events [get]
// @Router /api/v1/public/models/{model} [delete]
// @Router /api/v1/public/assistants [post]
// @Router /api/v1/public/assistants [get]
// @Router /api/v1/public/assistants/{id} [get]
// @Router /api/v1/public/assistants/{id} [post]
// @Router /api/v1/public/assistants/{id} [delete]
// @Router /api/v1/public/assistants/{id}/files [post]
// @Router /api/v1/public/assistants/{id}/files [get]
// @Router /api/v1/public/assistants/{id}/files/{fileId} [get]
// @Router /api/v1/public/assistants/{id}/files/{fileId} [delete]
// @Router /api/v1/public/threads [post]
// @Router /api/v1/public/threads/{id} [get]
// @Router /api/v1/public/threads/{id} [post]
// @Router /api/v1/public/threads/{id} [delete]
// @Router /api/v1/public/threads/{id}/messages [post]
// @Router /api/v1/public/threads/{id}/messages/{messageId} [get]
// @Router /api/v1/public/threads/{id}/messages/{messageId} [post]
// @Router /api/v1/public/threads/{id}/messages/{messageId}/files/{filesId} [get]
// @Router /api/v1/public/threads/{id}/messages/{messageId}/files [get]
// @Router /api/v1/public/threads/{id}/runs [post]
// @Router /api/v1/public/threads/{id}/runs [get]
// @Router /api/v1/public/threads/{id}/runs/{runsId} [get]
// @Router /api/v1/public/threads/{id}/runs/{runsId} [post]
// @Router /api/v1/public/threads/{id}/runs/{runsId}/submit_tool_outputs [post]
// @Router /api/v1/public/threads/{id}/runs/{runsId}/cancel [post]
// @Router /api/v1/public/threads/{id}/runs/{runsId}/steps/{stepId} [get]
// @Router /api/v1/public/threads/{id}/runs/{runsId}/steps [get]
func RelayNotImplemented(c *gin.Context) {
	err := model.Error{
		Message: "API not implemented",
		Type:    "one_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	err := model.Error{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}
