package channel

import (
	"context"
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
)

func shouldProbeInsufficientBalanceRecovery(channel *model.Channel, state model.ChannelCircuitBreakerState) bool {
	if channel == nil || channel.Status != model.ChannelStatusAutoDisabled {
		return false
	}
	return model.IsInsufficientBalanceCircuitBreakerState(state)
}

func EnqueueInsufficientBalanceChannelRecoveryTests(limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	channels, err := channelsvc.GetAllBasic(0, 0, "all", true)
	if err != nil {
		return 0, err
	}
	channelByID := make(map[string]*model.Channel)
	channelIDs := make([]string, 0)
	for _, channelRow := range channels {
		if channelRow == nil || channelRow.Status != model.ChannelStatusAutoDisabled {
			continue
		}
		channelID := strings.TrimSpace(channelRow.Id)
		if channelID == "" {
			continue
		}
		channelByID[channelID] = channelRow
		channelIDs = append(channelIDs, channelID)
	}
	if len(channelIDs) == 0 {
		return 0, nil
	}
	logInsufficientBalanceRecoveryProbe("scan_candidates", "", "", "", fmt.Sprintf("candidate_count=%d", len(channelIDs)))
	states, err := model.ListChannelCircuitBreakerStatesByChannelIDsWithDB(model.DB, channelIDs)
	if err != nil {
		return 0, err
	}
	createdCount := 0
	for _, state := range states {
		channelID := strings.TrimSpace(state.ChannelId)
		channelRow := channelByID[channelID]
		if !shouldProbeInsufficientBalanceRecovery(channelRow, state) {
			if model.IsInsufficientBalanceCircuitBreakerState(state) {
				status := "channel_missing"
				if channelRow != nil {
					status = fmt.Sprintf("channel_status=%d", channelRow.Status)
				}
				logInsufficientBalanceRecoveryProbe("skip_state_mismatch", channelID, "", "state/channel status does not require recovery probe", status)
			}
			continue
		}
		logInsufficientBalanceRecoveryProbe("probe_candidate", channelID, "", state.Reason, fmt.Sprintf("recover_after=%d", state.RecoverAfter))
		created, err := enqueueInsufficientBalanceRecoveryTest(channelRow, "")
		if err != nil {
			logInsufficientBalanceRecoveryProbe("probe_failed", channelID, "", err.Error(), "")
			return createdCount, fmt.Errorf("enqueue recovery test for channel %s: %w", channelID, err)
		}
		if created {
			createdCount++
			logInsufficientBalanceRecoveryProbe("probe_enqueued", channelID, "", "", "")
		} else {
			logInsufficientBalanceRecoveryProbe("probe_skipped", channelID, "", "task not created", "")
		}
		if createdCount >= limit {
			break
		}
	}
	return createdCount, nil
}

func logInsufficientBalanceRecoveryProbe(action string, channelID string, modelID string, reason string, detail string) {
	fields := []string{
		"[channel-recovery]",
		stringField("action", action),
		stringField("channel_id", channelID),
		stringField("model", modelID),
		stringField("reason", reason),
		stringField("detail", detail),
	}
	logger.Info(context.Background(), strings.Join(compactLogFields(fields), " "))
}
