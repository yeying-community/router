package billing

import (
	"testing"

	"github.com/yeying-community/router/internal/admin/model"
)

func TestShouldAutoRefreshChannelBillingIncludesInsufficientBalanceAutoDisabled(t *testing.T) {
	channel := &model.Channel{Id: "channel-1", Status: model.ChannelStatusAutoDisabled}
	states := map[string]model.ChannelCircuitBreakerState{
		"channel-1": {
			ChannelId: "channel-1",
			State:     model.ChannelCircuitBreakerStateCanceled,
			Reason:    model.ChannelCircuitBreakerReasonInsufficientBalance,
		},
	}

	if !shouldAutoRefreshChannelBilling(channel, states) {
		t.Fatalf("insufficient-balance auto-disabled channel should be auto-refreshed")
	}
}

func TestShouldAutoRefreshChannelBillingSkipsManualDisabled(t *testing.T) {
	channel := &model.Channel{Id: "channel-1", Status: model.ChannelStatusManuallyDisabled}
	states := map[string]model.ChannelCircuitBreakerState{
		"channel-1": {
			ChannelId: "channel-1",
			State:     model.ChannelCircuitBreakerStateCanceled,
			Reason:    model.ChannelCircuitBreakerReasonInsufficientBalance,
		},
	}

	if shouldAutoRefreshChannelBilling(channel, states) {
		t.Fatalf("manually disabled channel should not be auto-refreshed")
	}
}
