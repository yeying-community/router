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
	adminchannel "github.com/yeying-community/router/internal/admin/controller/channel"
	dbmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/monitor"
	"github.com/yeying-community/router/internal/relay/controller"
	relaylogging "github.com/yeying-community/router/internal/relay/logging"
	"github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
	"github.com/yeying-community/router/internal/transport/http/middleware"
)

func relayHelper(c *gin.Context, relayMode int) *model.ErrorWithStatusCode {
	var err *model.ErrorWithStatusCode
	switch relayMode {
	case relaymode.ImagesGenerations:
		err = controller.RelayImageHelper(c, relayMode)
	case relaymode.ImagesEdits:
		err = controller.RelayImageHelper(c, relayMode)
	case relaymode.AudioSpeech:
		fallthrough
	case relaymode.AudioTranslation:
		fallthrough
	case relaymode.AudioTranscription:
		err = controller.RelayAudioHelper(c, relayMode)
	case relaymode.Realtime:
		err = controller.RelayRealtimeHelper(c)
	case relaymode.Videos:
		err = controller.RelayVideoHelper(c, relayMode)
	case relaymode.Proxy:
		err = controller.RelayProxyHelper(c, relayMode)
	default:
		err = controller.RelayTextHelper(c)
	}
	return err
}

func Relay(c *gin.Context) {
	ctx := c.Request.Context()
	c.Set(ctxkey.RelayRetryCount, 0)
	c.Set(ctxkey.RelayError, "")
	c.Set(ctxkey.RelayErrorType, "")
	c.Set(ctxkey.RelayErrorCode, "")
	c.Set(ctxkey.RelayTermination, "")
	relayMode := getEffectiveRelayMode(c)
	if config.DebugEnabled {
		requestBody, _ := common.GetRequestBody(c)
		logger.Debugf(
			ctx,
			"request body summary: %s",
			common.MarshalPayloadLogFields(requestBody, c.ContentType()),
		)
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
	if markClientAbortIfNeeded(c, bizErr) {
		return
	}
	failedChannelIDs := map[string]struct{}{}
	if trimmedChannelID := strings.TrimSpace(channelId); trimmedChannelID != "" {
		failedChannelIDs[trimmedChannelID] = struct{}{}
	}
	go processChannelRelayError(ctx, userId, group, channelId, channelName, originalModel, requestPath, *bizErr)
	traceID := c.GetString(helper.TraceIDKey)
	retryAllRemainingCandidates := config.RetryTimes > 0
	retryCount := 0
	retryable := shouldRetry(c, bizErr)
	if !retryable {
		logger.RelayWarnf(ctx, relaylogging.NewFields("RETRY").
			String("decision", "skip").
			Int("status", bizErr.StatusCode).
			String("channel_id", channelId).
			String("channel_name", channelName).
			String("user_id", userId).
			String("group", group).
			String("model", originalModel).
			String("endpoint", requestPath).
			String("reason", "status_not_retryable").
			Build())
		retryAllRemainingCandidates = false
	}
	for retryAllRemainingCandidates {
		channel, selectionStats, err := dbmodel.CacheSelectRandomSatisfiedChannelForRequestExcluding(group, originalModel, requestPath, false, failedChannelIDs)
		if err != nil {
			fields := relaylogging.NewFields("RETRY").
				String("decision", "select_failed").
				String("user_id", userId).
				String("group", group).
				String("model", originalModel).
				String("endpoint", requestPath).
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
			String("user_id", userId).
			String("group", group).
			String("model", originalModel).
			String("endpoint", requestPath).
			String("from_channel_id", lastFailedChannelId).
			String("to_channel_id", channel.Id).
			String("to_channel_name", channel.DisplayName()).
			String("selection_scope", selectionStats.SelectionScope).
			Int("selected_priority", priorityToInt(selectionStats.SelectedPriority)).
			Int("tier_candidates", selectionStats.SelectedTierCandidates).
			Int("remaining_candidates", selectionStats.RemainingCandidates).
			Int("total_candidates", selectionStats.TotalCandidates).
			Int("failed_channels", len(failedChannelIDs)).
			Build())
		middleware.SetupContextForSelectedChannel(c, channel, originalModel)
		requestBody, err := common.GetRequestBody(c)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		relayMode = getEffectiveRelayMode(c)
		bizErr = relayHelper(c, relayMode)
		if bizErr == nil {
			return
		}
		if markClientAbortIfNeeded(c, bizErr) {
			return
		}
		channelId := c.GetString(ctxkey.ChannelId)
		lastFailedChannelId = channelId
		channelName = c.GetString(ctxkey.ChannelName)
		if trimmedChannelID := strings.TrimSpace(channelId); trimmedChannelID != "" {
			failedChannelIDs[trimmedChannelID] = struct{}{}
		}
		go processChannelRelayError(ctx, userId, group, channelId, channelName, originalModel, requestPath, *bizErr)
	}
	if bizErr != nil {
		normalizeFinalRelayError(bizErr)
		c.Set(ctxkey.RelayError, bizErr.Error.Message)
		c.Set(ctxkey.RelayErrorType, bizErr.Error.Type)
		c.Set(ctxkey.RelayErrorCode, errorCodeString(bizErr.Error.Code))
		c.Set(ctxkey.ChannelId, lastFailedChannelId)
		c.Set(ctxkey.ChannelName, channelName)

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
	switch getEffectiveRelayMode(c) {
	case relaymode.ImagesGenerations, relaymode.ImagesEdits:
		return false
	}
	if _, ok := c.Get(ctxkey.SpecificChannelId); ok {
		return false
	}
	if controller.IsGroupDailyQuotaExceededError(bizErr) {
		return false
	}
	if isRelayCapabilityError(bizErr) {
		return true
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

func markClientAbortIfNeeded(c *gin.Context, bizErr *model.ErrorWithStatusCode) bool {
	if c == nil || bizErr == nil || !isClientAbortRelayError(bizErr) {
		return false
	}
	c.Set(ctxkey.RelayTermination, "client_aborted")
	c.Set(ctxkey.RelayErrorType, "client_abort")
	c.Set(ctxkey.RelayErrorCode, errorCodeString(bizErr.Error.Code))
	c.Set(ctxkey.RelayError, bizErr.Error.Message)
	return true
}

func isClientAbortRelayError(err *model.ErrorWithStatusCode) bool {
	if err == nil {
		return false
	}
	code := strings.ToLower(strings.TrimSpace(errorCodeString(err.Code)))
	message := strings.ToLower(strings.TrimSpace(err.Message))
	if message == "" {
		return false
	}
	if strings.Contains(message, "context canceled") {
		switch code {
		case "do_request_failed", "read_response_body_failed", "copy_response_body_failed", "close_response_body_failed":
			return true
		}
	}
	if code == "copy_response_body_failed" {
		return strings.Contains(message, "broken pipe") ||
			strings.Contains(message, "use of closed network connection") ||
			strings.Contains(message, "connection reset by peer")
	}
	return false
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
	if isRelayCapabilityError(err) {
		err.StatusCode = http.StatusServiceUnavailable
		err.Error.Message = "当前分组可用上游能力不匹配，请稍后再试"
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
	if lowerCode == "insufficient_quota" || lowerCode == "billing_hard_limit_reached" || lowerCode == "1113" {
		return true
	}
	lowerMessage := strings.ToLower(strings.TrimSpace(err.Message))
	return strings.Contains(lowerMessage, "额度") ||
		strings.Contains(lowerMessage, "余额不足") ||
		strings.Contains(lowerMessage, "资源包") ||
		strings.Contains(lowerMessage, "欠费") ||
		strings.Contains(lowerMessage, "quota") ||
		strings.Contains(lowerMessage, "credit") ||
		strings.Contains(lowerMessage, "balance") ||
		strings.Contains(lowerMessage, "daily limit")
}

func isRelayCapabilityError(err *model.ErrorWithStatusCode) bool {
	return shouldDisableChannelModelRequestEndpointCapability(err) ||
		shouldDisableChannelModelCapability(err)
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
		return isTransientUpstreamTransportErrorCode(errorCodeString(err.Code))
	}
	return false
}

func isTransientUpstreamTransportErrorCode(code string) bool {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "do_request_failed", "upstream_transport_goaway":
		return true
	default:
		return false
	}
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

func processChannelRelayError(ctx context.Context, userId string, groupID string, channelId string, channelName string, requestModel string, requestPath string, err model.ErrorWithStatusCode) {
	msg := relaylogging.NewFields("UPSTREAM_ERR").
		String("channel_id", channelId).
		String("channel_name", channelName).
		String("group", groupID).
		String("model", requestModel).
		String("endpoint", requestPath).
		String("user_id", userId).
		Int("status", err.StatusCode).
		String("error_type", err.Type).
		String("error_code", errorCodeString(err.Code)).
		String("error", err.Message).
		Build()
	if isRelayContextCanceledError(&err) {
		logger.RelayWarnf(ctx, msg)
	} else if err.StatusCode >= http.StatusInternalServerError {
		logger.RelayErrorf(ctx, msg)
	} else {
		logger.RelayWarnf(ctx, msg)
	}
	if shouldDisableChannelModelRequestEndpointCapability(&err) {
		disabled, disableErr := dbmodel.DisableChannelModelRequestEndpointCapabilityWithReason(channelId, requestModel, requestPath, err.Message, "runtime")
		logChannelModelRequestEndpointDisableResult(ctx, channelId, channelName, requestModel, requestPath, err, disabled, disableErr)
		if disableErr != nil {
			monitor.Emit(channelId, false)
		} else if disabled {
			monitor.NotifyChannelModelEndpointCapabilityDisabled(channelId, channelName, requestModel, dbmodel.NormalizeRequestedChannelModelEndpoint(requestPath), err.Message)
			enqueueChannelModelCapabilityRecoveryTest(ctx, channelId, requestModel, requestPath)
		}
		return
	}
	if shouldDisableChannelModelCapability(&err) {
		disabled, disableErr := dbmodel.DisableChannelModelCapabilityWithReason(channelId, requestModel, err.Message, "runtime")
		logChannelModelCapabilityDisableResult(ctx, channelId, channelName, requestModel, err, disabled, disableErr)
		if disableErr != nil {
			monitor.Emit(channelId, false)
		} else if disabled {
			monitor.NotifyChannelModelCapabilityDisabled(channelId, channelName, requestModel, err.Message)
			enqueueChannelModelCapabilityRecoveryTest(ctx, channelId, requestModel, requestPath)
		}
		return
	}
	if monitor.ShouldDisableChannel(&err.Error, err.StatusCode) {
		monitor.DisableChannel(channelId, channelName, err.Message)
	} else {
		monitor.Emit(channelId, false)
	}
}

func enqueueChannelModelCapabilityRecoveryTest(ctx context.Context, channelID string, modelName string, endpoint string) {
	created, err := adminchannel.EnqueueChannelModelEndpointRecoveryTest(channelID, modelName, endpoint, helper.GetTraceID(ctx))
	if err != nil {
		logger.RelayWarnf(ctx, relaylogging.NewFields("RECOVERY_TEST_ENQUEUE_FAILED").
			String("channel_id", channelID).
			String("model", modelName).
			String("endpoint", endpoint).
			String("error", err.Error()).
			Build())
		return
	}
	if created {
		logger.RelayWarnf(ctx, relaylogging.NewFields("RECOVERY_TEST_ENQUEUED").
			String("channel_id", channelID).
			String("model", modelName).
			String("endpoint", endpoint).
			Build())
	}
}

func isRelayContextCanceledError(err *model.ErrorWithStatusCode) bool {
	if err == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(errorCodeString(err.Code)), "do_request_failed") {
		return false
	}
	lowerMessage := strings.ToLower(strings.TrimSpace(err.Message))
	return strings.Contains(lowerMessage, "context canceled")
}

func shouldDisableChannelModelCapability(err *model.ErrorWithStatusCode) bool {
	if err == nil {
		return false
	}
	code := errorCodeString(err.Code)
	if code == "unsupported_channel_endpoint" {
		return false
	}
	lowerMessage := strings.ToLower(strings.TrimSpace(err.Message))
	if isTransientUpstreamMessage(lowerMessage) {
		return false
	}
	if code == "model_not_found" {
		return true
	}

	lowerType := strings.ToLower(strings.TrimSpace(err.Type))
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

func isTransientUpstreamMessage(message string) bool {
	if message == "" {
		return false
	}
	return strings.Contains(message, "瞬时") ||
		strings.Contains(message, "稍候重试") ||
		strings.Contains(message, "稍后重试") ||
		strings.Contains(message, "暂时不可用") ||
		strings.Contains(message, "线路出现") ||
		strings.Contains(message, "重试全部源头链路") ||
		strings.Contains(message, "try again later") ||
		strings.Contains(message, "temporarily unavailable") ||
		strings.Contains(message, "temporary")
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
