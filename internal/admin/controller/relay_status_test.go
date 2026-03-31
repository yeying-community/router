package controller

import (
	"net/http"
	"testing"

	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

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
			Message: "当前用户今日额度及本月应急额度已达上限",
			Type:    "one_api_error",
			Code:    "user_quota_limit_exceeded",
		},
	}

	normalizeFinalRelayError(err)

	if err.StatusCode != http.StatusForbidden {
		t.Fatalf("unexpected status code: got %d want %d", err.StatusCode, http.StatusForbidden)
	}
	if err.Message != "当前用户今日额度及本月应急额度已达上限" {
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
