package responsestate

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

const defaultResponseRouteTTL = 6 * time.Hour

type routeEntry struct {
	ChannelID string
	ExpireAt  time.Time
}

var (
	routeMu       sync.Mutex
	routeStore    = make(map[string]routeEntry)
	routeTTL      = defaultResponseRouteTTL
	routeNow      = time.Now
	lastPrunedAt  time.Time
	pruneInterval = 10 * time.Minute
)

type RequestState struct {
	PreviousResponseID string
	HasToolOutput      bool
}

func AnalyzeRequestBody(raw []byte) RequestState {
	state := RequestState{}
	if len(raw) == 0 {
		return state
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return state
	}
	if root, ok := payload.(map[string]any); ok {
		state.PreviousResponseID = strings.TrimSpace(asString(root["previous_response_id"]))
	}
	state.HasToolOutput = containsFunctionCallOutput(payload)
	return state
}

func IsStatefulRequestBody(raw []byte) bool {
	state := AnalyzeRequestBody(raw)
	return state.PreviousResponseID != "" || state.HasToolOutput
}

func ExtractResponseID(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(asString(payload["id"]))
}

func StoreRoute(responseID string, channelID string) {
	normalizedResponseID := strings.TrimSpace(responseID)
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedResponseID == "" || normalizedChannelID == "" {
		return
	}
	now := routeNow()
	routeMu.Lock()
	defer routeMu.Unlock()
	if now.Sub(lastPrunedAt) >= pruneInterval {
		pruneExpiredLocked(now)
		lastPrunedAt = now
	}
	routeStore[normalizedResponseID] = routeEntry{
		ChannelID: normalizedChannelID,
		ExpireAt:  now.Add(routeTTL),
	}
}

func LookupRoute(responseID string) (string, bool) {
	normalizedResponseID := strings.TrimSpace(responseID)
	if normalizedResponseID == "" {
		return "", false
	}
	now := routeNow()
	routeMu.Lock()
	defer routeMu.Unlock()
	entry, ok := routeStore[normalizedResponseID]
	if !ok {
		return "", false
	}
	if !entry.ExpireAt.IsZero() && now.After(entry.ExpireAt) {
		delete(routeStore, normalizedResponseID)
		return "", false
	}
	return entry.ChannelID, true
}

func pruneExpiredLocked(now time.Time) {
	for responseID, entry := range routeStore {
		if !entry.ExpireAt.IsZero() && now.After(entry.ExpireAt) {
			delete(routeStore, responseID)
		}
	}
}

func ResetForTest() {
	routeMu.Lock()
	defer routeMu.Unlock()
	routeStore = make(map[string]routeEntry)
	routeTTL = defaultResponseRouteTTL
	routeNow = time.Now
	lastPrunedAt = time.Time{}
}

func asString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func containsFunctionCallOutput(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		if strings.EqualFold(strings.TrimSpace(asString(typed["type"])), "function_call_output") {
			return true
		}
		for _, child := range typed {
			if containsFunctionCallOutput(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsFunctionCallOutput(child) {
				return true
			}
		}
	}
	return false
}
