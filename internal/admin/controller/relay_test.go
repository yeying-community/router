package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/routeobs"
)

func TestRelayNotFoundDisablesCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/group/channel-options", nil)
	c.Request = req

	RelayNotFound(c)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unexpected status code: got %d want %d", recorder.Code, http.StatusNotFound)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate" {
		t.Fatalf("unexpected Cache-Control header: got %q", got)
	}
	if got := recorder.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("unexpected Pragma header: got %q", got)
	}
	if got := recorder.Header().Get("Expires"); got != "0" {
		t.Fatalf("unexpected Expires header: got %q", got)
	}
}

func TestShouldRetrySkipsStatefulResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request = req
	c.Set(ctxkey.ResponsesStatefulRequest, true)
	c.Set(ctxkey.ResponsesPreviousResponseID, "resp_existing")

	err := &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusTooManyRequests,
	}
	if shouldRetry(c, err) {
		t.Fatal("shouldRetry returned true for stateful responses request, want false")
	}
}

func TestShouldRetryInitialResponsesOnUpstreamQuota(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(ctxkey.ResponsesStatefulRequest, true)
	err := &relaymodel.ErrorWithStatusCode{StatusCode: http.StatusForbidden}
	err.Error.Code = "insufficient_user_quota"
	err.Error.Type = "new_api_error"
	err.Error.Message = "用户额度不足"
	if !shouldRetry(c, err) {
		t.Fatal("initial responses request should retry provider quota failures")
	}
}

func TestShouldRetryRemainingCandidatesForUpstreamQuotaWithoutGeneralRetries(t *testing.T) {
	originalRetryTimes := config.RetryTimes
	config.RetryTimes = 0
	t.Cleanup(func() { config.RetryTimes = originalRetryTimes })

	err := &relaymodel.ErrorWithStatusCode{StatusCode: http.StatusForbidden}
	err.Error.Type = "new_api_error"
	err.Error.Code = "insufficient_user_quota"
	err.Error.Message = "用户额度不足, 剩余额度: $-0.095574"
	if !shouldRetryRemainingCandidates(err) {
		t.Fatal("upstream user quota failure should switch channels when RetryTimes=0")
	}
}

func TestShouldRetryRemainingCandidatesSkipsLocalQuotaWithoutGeneralRetries(t *testing.T) {
	originalRetryTimes := config.RetryTimes
	config.RetryTimes = 0
	t.Cleanup(func() { config.RetryTimes = originalRetryTimes })

	err := &relaymodel.ErrorWithStatusCode{StatusCode: http.StatusForbidden}
	err.Error.Type = "one_api_error"
	err.Error.Code = "insufficient_user_balance"
	err.Error.Message = "用户余额不足"
	if shouldRetryRemainingCandidates(err) {
		t.Fatal("local user quota failure should not switch channels when RetryTimes=0")
	}
}

func TestShouldRetryRemainingCandidatesUsesRetryTimesOnlyForTransportErrors(t *testing.T) {
	originalRetryTimes := config.RetryTimes
	config.RetryTimes = 0
	t.Cleanup(func() { config.RetryTimes = originalRetryTimes })

	for _, testCase := range []struct {
		name string
		err  *relaymodel.ErrorWithStatusCode
		want bool
	}{
		{
			name: "provider rate limit fails over without network retries",
			err:  &relaymodel.ErrorWithStatusCode{StatusCode: http.StatusTooManyRequests},
			want: true,
		},
		{
			name: "provider server response fails over without network retries",
			err:  &relaymodel.ErrorWithStatusCode{StatusCode: http.StatusBadGateway},
			want: true,
		},
		{
			name: "transport failure is disabled",
			err: &relaymodel.ErrorWithStatusCode{
				StatusCode: http.StatusInternalServerError,
				Error:      relaymodel.Error{Code: "do_request_failed"},
			},
			want: false,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := shouldRetryRemainingCandidates(testCase.err); got != testCase.want {
				t.Fatalf("shouldRetryRemainingCandidates() = %t, want %t", got, testCase.want)
			}
		})
	}

	config.RetryTimes = 1
	for _, code := range []string{"do_request_failed", "read_response_body_failed", "upstream_transport_goaway"} {
		err := &relaymodel.ErrorWithStatusCode{
			StatusCode: http.StatusInternalServerError,
			Error:      relaymodel.Error{Code: code},
		}
		if !shouldRetryRemainingCandidates(err) {
			t.Errorf("shouldRetryRemainingCandidates(%q) = false, want true", code)
		}
	}
}

func TestShouldRetrySkipsAfterResponseHasStarted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	responseRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(responseRecorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	_, _ = c.Writer.Write([]byte("partial response"))

	err := &relaymodel.ErrorWithStatusCode{StatusCode: http.StatusBadGateway}
	if shouldRetry(c, err) {
		t.Fatal("shouldRetry returned true after response output began")
	}
}

func TestShouldRetrySkipsLocalUserBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	err := &relaymodel.ErrorWithStatusCode{StatusCode: http.StatusForbidden}
	err.Error.Code = "insufficient_user_balance"
	if shouldRetry(c, err) {
		t.Fatal("local user balance failures must not retry another provider")
	}
}

func TestShouldRetrySkipsUserInputAndContentPolicyResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, testCase := range []struct {
		name string
		err  *relaymodel.ErrorWithStatusCode
	}{
		{
			name: "unprocessable user input",
			err:  &relaymodel.ErrorWithStatusCode{StatusCode: http.StatusUnprocessableEntity},
		},
		{
			name: "content policy refusal",
			err: &relaymodel.ErrorWithStatusCode{
				StatusCode: http.StatusForbidden,
				Error:      relaymodel.Error{Code: "content_policy_violation"},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			responseRecorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(responseRecorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			if shouldRetry(c, testCase.err) {
				t.Fatal("shouldRetry returned true for a user-input or content-policy error")
			}
		})
	}
}

func TestBuildRelayFailureLogCapturesRouteFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set(ctxkey.Id, "user-1")
	c.Set(ctxkey.Group, "default")
	c.Set(ctxkey.ChannelId, "channel-1")
	c.Set(ctxkey.ChannelName, "primary")
	c.Set(ctxkey.OriginalModel, "gpt-5")
	c.Set(ctxkey.TokenName, "prod-token")
	c.Set(ctxkey.RelayFallbackAttempts, `[{"attempt":1,"channel_id":"channel-1"}]`)
	routeobs.SetRouteDecision(c, routeobs.RouteDecision{
		Source:              "automatic",
		GroupID:             "default",
		Model:               "gpt-5",
		Endpoint:            "/v1/chat/completions",
		CandidateChannelIDs: []string{"channel-1", "channel-2"},
		SelectedPriority:    10,
		SelectionMode:       "priority_random",
		InitialChannelID:    "channel-1",
	})

	got := buildRelayFailureLog(c, &relaymodel.ErrorWithStatusCode{
		StatusCode: http.StatusServiceUnavailable,
		Error: relaymodel.Error{
			Message: "upstream unavailable",
			Type:    "one_api_error",
			Code:    "upstream_unavailable",
		},
	}, 2)

	if got == nil {
		t.Fatalf("relay failure log was not built")
	}
	if got.UserId != "user-1" || got.GroupId != "default" || got.ChannelId != "channel-1" {
		t.Fatalf("unexpected identity fields: %+v", got)
	}
	if got.ModelName != "gpt-5" || got.RequestModelName != "gpt-5" || got.ActualModelName != "gpt-5" {
		t.Fatalf("unexpected model fields: %+v", got)
	}
	if got.UpstreamEndpoint != "/v1/chat/completions" {
		t.Fatalf("UpstreamEndpoint=%q, want /v1/chat/completions", got.UpstreamEndpoint)
	}
	if got.FallbackCount != 2 {
		t.Fatalf("FallbackCount=%d, want 2", got.FallbackCount)
	}
	if got.FallbackAttempts == "" {
		t.Fatalf("FallbackAttempts empty")
	}
	var decision routeobs.RouteDecision
	if err := json.Unmarshal([]byte(got.RouteDecision), &decision); err != nil {
		t.Fatalf("unmarshal RouteDecision: %v", err)
	}
	if decision.InitialChannelID != "channel-1" || decision.FinalChannelID != "channel-1" || decision.CandidateCount != 2 {
		t.Fatalf("unexpected RouteDecision: %+v", decision)
	}
	if got.RelayErrorCode != "upstream_unavailable" || got.RelayErrorMessage != "upstream unavailable" {
		t.Fatalf("unexpected relay error fields: %+v", got)
	}
}
