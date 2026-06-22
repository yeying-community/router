package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

func resetRuntimeCapabilityFailuresForTest() {
	runtimeCapabilityFailures.Lock()
	runtimeCapabilityFailures.items = make(map[string]runtimeCapabilityFailureState)
	runtimeCapabilityFailures.Unlock()
}

func TestNormalizeFinalRelayErrorForTransientUpstream429(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusTooManyRequests,
		Error: relaymodel.Error{
			Message: "rate limit exceeded",
			Type:    "rate_limit_error",
			Code:    "rate_limit_exceeded",
		},
	}

	normalizeFinalRelayError(err)

	if err.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status code: got %d want %d", err.StatusCode, http.StatusServiceUnavailable)
	}
	if err.Message != "当前分组可用上游暂时不可用，请稍后再试" {
		t.Fatalf("unexpected message: got %q", err.Message)
	}
}

func TestNormalizeFinalRelayErrorForTransientDoRequestFailed(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusInternalServerError,
		Error: relaymodel.Error{
			Message: "do request failed: i/o timeout",
			Type:    "one_api_error",
			Code:    "do_request_failed",
		},
	}

	normalizeFinalRelayError(err)

	if err.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status code: got %d want %d", err.StatusCode, http.StatusServiceUnavailable)
	}
	if err.Message != "当前分组可用上游暂时不可用，请稍后再试" {
		t.Fatalf("unexpected message: got %q", err.Message)
	}
}

func TestNormalizeFinalRelayErrorForTransientUpstreamTransportGoaway(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusInternalServerError,
		Error: relaymodel.Error{
			Message: "do request failed: http2: server sent GOAWAY and closed the connection",
			Type:    "one_api_error",
			Code:    "upstream_transport_goaway",
		},
	}

	normalizeFinalRelayError(err)

	if err.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status code: got %d want %d", err.StatusCode, http.StatusServiceUnavailable)
	}
	if err.Message != "当前分组可用上游暂时不可用，请稍后再试" {
		t.Fatalf("unexpected message: got %q", err.Message)
	}
}

func TestRuntimeCapabilityFailureWindowPausesAfterThreshold(t *testing.T) {
	resetRuntimeCapabilityFailuresForTest()
	t.Cleanup(resetRuntimeCapabilityFailuresForTest)

	now := time.Unix(1000, 0)
	for idx := 1; idx < runtimeCapabilityFailureThreshold; idx++ {
		count, shouldPause := recordRuntimeCapabilityFailureWindow("channel-1", "gpt-image-2", "/v1/images/generations", now.Add(time.Duration(idx)*time.Second))
		if shouldPause {
			t.Fatalf("recordRuntimeCapabilityFailureWindow shouldPause=true at count %d, want false", count)
		}
		if count != idx {
			t.Fatalf("count=%d, want %d", count, idx)
		}
	}
	count, shouldPause := recordRuntimeCapabilityFailureWindow("channel-1", "gpt-image-2", "/v1/images/generations", now.Add(5*time.Second))
	if !shouldPause {
		t.Fatalf("recordRuntimeCapabilityFailureWindow shouldPause=false at count %d, want true", count)
	}
	if count != runtimeCapabilityFailureThreshold {
		t.Fatalf("count=%d, want %d", count, runtimeCapabilityFailureThreshold)
	}
}

func TestRuntimeCapabilityFailureWindowResetsAfterSuccess(t *testing.T) {
	resetRuntimeCapabilityFailuresForTest()
	t.Cleanup(resetRuntimeCapabilityFailuresForTest)

	now := time.Unix(1000, 0)
	count, shouldPause := recordRuntimeCapabilityFailureWindow("channel-1", "gpt-image-2", "/v1/images/generations", now)
	if count != 1 || shouldPause {
		t.Fatalf("first record count=%d shouldPause=%v, want count=1 shouldPause=false", count, shouldPause)
	}
	clearRuntimeCapabilityFailureWindow("channel-1", "gpt-image-2", "/v1/images/generations")
	count, shouldPause = recordRuntimeCapabilityFailureWindow("channel-1", "gpt-image-2", "/v1/images/generations", now.Add(2*time.Second))
	if count != 1 || shouldPause {
		t.Fatalf("record after clear count=%d shouldPause=%v, want count=1 shouldPause=false", count, shouldPause)
	}
}

func TestShouldTrackRuntimeCapabilityFailureOnlyTracksTransientErrors(t *testing.T) {
	if !shouldTrackRuntimeCapabilityFailure(&relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusInternalServerError,
		Error: relaymodel.Error{
			Message: "do request failed: http2: server sent GOAWAY",
			Type:    "one_api_error",
			Code:    "upstream_transport_goaway",
		},
	}) {
		t.Fatalf("shouldTrackRuntimeCapabilityFailure = false, want true for transient goaway")
	}
	if shouldTrackRuntimeCapabilityFailure(&relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusNotFound,
		Error: relaymodel.Error{
			Message: "model does not exist",
			Type:    "invalid_request_error",
			Code:    "model_not_found",
		},
	}) {
		t.Fatalf("shouldTrackRuntimeCapabilityFailure = true, want false for explicit capability error")
	}
	if shouldTrackRuntimeCapabilityFailure(&relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusPaymentRequired,
		Error: relaymodel.Error{
			Message: "insufficient quota",
			Type:    "insufficient_quota",
			Code:    "insufficient_quota",
		},
	}) {
		t.Fatalf("shouldTrackRuntimeCapabilityFailure = true, want false for quota error")
	}
}

func TestNormalizeFinalRelayErrorKeepsLocalServiceErrors(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusServiceUnavailable,
		Error: relaymodel.Error{
			Message: "pricing missing",
			Type:    "one_api_error",
			Code:    "model_pricing_not_configured",
		},
	}

	normalizeFinalRelayError(err)

	if err.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status code: got %d want %d", err.StatusCode, http.StatusServiceUnavailable)
	}
	if err.Message != "pricing missing" {
		t.Fatalf("unexpected message: got %q", err.Message)
	}
}

func TestNormalizeFinalRelayErrorForUpstreamQuotaExhausted(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusPaymentRequired,
		Error: relaymodel.Error{
			Message: "每日额度超限",
			Type:    "insufficient_quota",
			Code:    "insufficient_quota",
		},
	}

	normalizeFinalRelayError(err)

	if err.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status code: got %d want %d", err.StatusCode, http.StatusServiceUnavailable)
	}
	if err.Message != "当前分组可用上游额度不足，请稍后再试" {
		t.Fatalf("unexpected message: got %q", err.Message)
	}
}

func TestNormalizeFinalRelayErrorForZhipuInsufficientBalanceCode(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusTooManyRequests,
		Error: relaymodel.Error{
			Message: "余额不足或无可用资源包,请充值。",
			Type:    "",
			Code:    "1113",
		},
	}

	normalizeFinalRelayError(err)

	if err.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status code: got %d want %d", err.StatusCode, http.StatusServiceUnavailable)
	}
	if err.Message != "当前分组可用上游额度不足，请稍后再试" {
		t.Fatalf("unexpected message: got %q", err.Message)
	}
}

func TestNormalizeFinalRelayErrorKeepsGroupDailyQuotaExceeded(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusForbidden,
		Error: relaymodel.Error{
			Message: "当前分组套餐每日额度已达上限，请明日再试",
			Type:    "one_api_error",
			Code:    "group_daily_quota_exceeded",
		},
	}

	normalizeFinalRelayError(err)

	if err.StatusCode != http.StatusForbidden {
		t.Fatalf("unexpected status code: got %d want %d", err.StatusCode, http.StatusForbidden)
	}
	if err.Message != "当前分组套餐每日额度已达上限，请明日再试" {
		t.Fatalf("unexpected message: got %q", err.Message)
	}
}

func TestNormalizeFinalRelayErrorKeepsUserQuotaExceeded(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusForbidden,
		Error: relaymodel.Error{
			Message: "当前用户今日额度及套餐应急额度已达上限",
			Type:    "one_api_error",
			Code:    "user_quota_limit_exceeded",
		},
	}

	normalizeFinalRelayError(err)

	if err.StatusCode != http.StatusForbidden {
		t.Fatalf("unexpected status code: got %d want %d", err.StatusCode, http.StatusForbidden)
	}
	if err.Message != "当前用户今日额度及套餐应急额度已达上限" {
		t.Fatalf("unexpected message: got %q", err.Message)
	}
}

func TestNormalizeFinalRelayErrorKeepsInsufficientUserQuota(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusForbidden,
		Error: relaymodel.Error{
			Message: "user quota is not enough",
			Type:    "one_api_error",
			Code:    "insufficient_user_quota",
		},
	}

	normalizeFinalRelayError(err)

	if err.StatusCode != http.StatusForbidden {
		t.Fatalf("unexpected status code: got %d want %d", err.StatusCode, http.StatusForbidden)
	}
	if err.Message != "user quota is not enough" {
		t.Fatalf("unexpected message: got %q", err.Message)
	}
}

func TestNormalizeFinalRelayErrorKeepsTokenQuotaExceeded(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusForbidden,
		Error: relaymodel.Error{
			Message: "令牌额度不足",
			Type:    "one_api_error",
			Code:    "pre_consume_token_quota_failed",
		},
	}

	normalizeFinalRelayError(err)

	if err.StatusCode != http.StatusForbidden {
		t.Fatalf("unexpected status code: got %d want %d", err.StatusCode, http.StatusForbidden)
	}
	if err.Message != "令牌额度不足" {
		t.Fatalf("unexpected message: got %q", err.Message)
	}
}

func TestNormalizeFinalRelayErrorForCapabilityMismatch(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusBadRequest,
		Error: relaymodel.Error{
			Message: "channel model does not support /v1/responses",
			Type:    "one_api_error",
			Code:    "unsupported_channel_endpoint",
		},
	}

	normalizeFinalRelayError(err)

	if err.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status code: got %d want %d", err.StatusCode, http.StatusServiceUnavailable)
	}
	if err.Message != "当前分组可用上游能力不匹配，请稍后再试" {
		t.Fatalf("unexpected message: got %q", err.Message)
	}
}

func TestShouldDisableChannelModelCapabilityForModelNotFoundCode(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusNotFound,
		Error: relaymodel.Error{
			Message: "The model `gpt-x` does not exist",
			Type:    "invalid_request_error",
			Code:    "model_not_found",
		},
	}

	if !shouldDisableChannelModelCapability(err) {
		t.Fatalf("shouldDisableChannelModelCapability = false, want true")
	}
}

func TestShouldDisableChannelModelCapabilitySkipsTransientUpstreamMessage(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusNotFound,
		Error: relaymodel.Error{
			Message: "当前线路出现瞬时波动，系统已自动为您重试全部源头链路，请稍候重试",
			Type:    "invalid_request_error",
			Code:    "model_not_found",
		},
	}

	if shouldDisableChannelModelCapability(err) {
		t.Fatalf("shouldDisableChannelModelCapability = true, want false for transient upstream message")
	}
}

func TestShouldDisableChannelModelCapabilityForModelScopedPermission(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusForbidden,
		Error: relaymodel.Error{
			Message: "You do not have access to model gpt-5.4",
			Type:    "permission_error",
			Code:    "",
		},
	}

	if !shouldDisableChannelModelCapability(err) {
		t.Fatalf("shouldDisableChannelModelCapability = false, want true")
	}
}

func TestShouldDisableChannelModelCapabilitySkipsUnsupportedChannelEndpoint(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusBadRequest,
		Error: relaymodel.Error{
			Message: "channel model does not support /v1/responses",
			Type:    "one_api_error",
			Code:    "unsupported_channel_endpoint",
		},
	}

	if shouldDisableChannelModelCapability(err) {
		t.Fatalf("shouldDisableChannelModelCapability = true, want false")
	}
}

func TestShouldDisableChannelModelRequestEndpointCapabilityForUnsupportedEndpoint(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusBadRequest,
		Error: relaymodel.Error{
			Message: "channel model does not support /v1/responses",
			Type:    "one_api_error",
			Code:    "unsupported_channel_endpoint",
		},
	}

	if !shouldDisableChannelModelRequestEndpointCapability(err) {
		t.Fatalf("shouldDisableChannelModelRequestEndpointCapability = false, want true")
	}
}

func TestShouldRetryForUnsupportedEndpointCapabilityError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusBadRequest,
		Error: relaymodel.Error{
			Message: "channel model does not support /v1/responses",
			Type:    "one_api_error",
			Code:    "unsupported_channel_endpoint",
		},
	}

	if !shouldRetry(ctx, err) {
		t.Fatalf("shouldRetry() = false, want true for endpoint capability error")
	}
}

func TestShouldRetryForModelNotFoundCapabilityError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusBadRequest,
		Error: relaymodel.Error{
			Message: "The model `gpt-x` does not exist",
			Type:    "invalid_request_error",
			Code:    "model_not_found",
		},
	}

	if !shouldRetry(ctx, err) {
		t.Fatalf("shouldRetry() = false, want true for model capability error")
	}
}

func TestIsClientAbortRelayErrorForContextCanceled(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusInternalServerError,
		Error: relaymodel.Error{
			Message: "context canceled",
			Type:    "one_api_error",
			Code:    "read_response_body_failed",
		},
	}

	if !isClientAbortRelayError(err) {
		t.Fatalf("isClientAbortRelayError = false, want true")
	}
}

func TestIsClientAbortRelayErrorForDownstreamBrokenPipe(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusInternalServerError,
		Error: relaymodel.Error{
			Message: "write tcp 127.0.0.1:8080->127.0.0.1:12345: write: broken pipe",
			Type:    "one_api_error",
			Code:    "copy_response_body_failed",
		},
	}

	if !isClientAbortRelayError(err) {
		t.Fatalf("isClientAbortRelayError = false, want true")
	}
}

func TestIsClientAbortRelayErrorSkipsUpstreamConnectionReset(t *testing.T) {
	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusInternalServerError,
		Error: relaymodel.Error{
			Message: "read tcp 192.168.0.19:62148->104.21.26.232:443: read: connection reset by peer",
			Type:    "one_api_error",
			Code:    "read_response_body_failed",
		},
	}

	if isClientAbortRelayError(err) {
		t.Fatalf("isClientAbortRelayError = true, want false")
	}
}

func TestShouldRetrySkipsImageRelayModes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)

	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusInternalServerError,
		Error: relaymodel.Error{
			Message: "do request failed: http2: server sent GOAWAY and closed the connection",
			Type:    "one_api_error",
			Code:    "upstream_transport_goaway",
		},
	}

	if shouldRetry(ctx, err) {
		t.Fatalf("shouldRetry() = true, want false for image relay modes")
	}
}
