package controller

import (
	"strings"
	"sync"
	"time"

	dbmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/model"
)

const (
	runtimeCapabilityFailureWindow    = time.Minute
	runtimeCapabilityFailureThreshold = 5
)

type runtimeCapabilityFailureState struct {
	firstSeen time.Time
	count     int
}

var runtimeCapabilityFailures = struct {
	sync.Mutex
	items map[string]runtimeCapabilityFailureState
}{
	items: make(map[string]runtimeCapabilityFailureState),
}

func runtimeCapabilityFailureKey(channelID string, modelName string, endpoint string) string {
	normalizedChannelID := strings.TrimSpace(channelID)
	normalizedModelName := strings.TrimSpace(modelName)
	normalizedEndpoint := dbmodel.NormalizeRequestedChannelModelEndpoint(endpoint)
	if normalizedChannelID == "" || normalizedModelName == "" || normalizedEndpoint == "" {
		return ""
	}
	return normalizedChannelID + "::" + normalizedModelName + "::" + normalizedEndpoint
}

func clearRuntimeCapabilityFailureWindow(channelID string, modelName string, endpoint string) {
	key := runtimeCapabilityFailureKey(channelID, modelName, endpoint)
	if key == "" {
		return
	}
	runtimeCapabilityFailures.Lock()
	delete(runtimeCapabilityFailures.items, key)
	runtimeCapabilityFailures.Unlock()
}

func recordRuntimeCapabilityFailureWindow(channelID string, modelName string, endpoint string, now time.Time) (int, bool) {
	key := runtimeCapabilityFailureKey(channelID, modelName, endpoint)
	if key == "" {
		return 0, false
	}
	if now.IsZero() {
		now = time.Now()
	}
	runtimeCapabilityFailures.Lock()
	defer runtimeCapabilityFailures.Unlock()
	state := runtimeCapabilityFailures.items[key]
	if state.firstSeen.IsZero() || now.Sub(state.firstSeen) > runtimeCapabilityFailureWindow {
		state = runtimeCapabilityFailureState{
			firstSeen: now,
			count:     1,
		}
	} else {
		state.count++
	}
	if state.count >= runtimeCapabilityFailureThreshold {
		delete(runtimeCapabilityFailures.items, key)
		return state.count, true
	}
	runtimeCapabilityFailures.items[key] = state
	return state.count, false
}

func shouldTrackRuntimeCapabilityFailure(err *model.ErrorWithStatusCode) bool {
	if err == nil {
		return false
	}
	if isRelayCapabilityError(err) || isUpstreamQuotaRelayError(err) || isLocalQuotaRelayError(err) {
		return false
	}
	return isTransientUpstreamRelayError(err)
}
