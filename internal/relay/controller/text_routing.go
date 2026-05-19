package controller

import (
	"encoding/json"
	"fmt"
	"strings"

	adminmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/apitype"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

func normalizeMessagesRequestBody(raw []byte, modelName string) ([]byte, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if strings.TrimSpace(modelName) != "" {
		payload["model"] = strings.TrimSpace(modelName)
	}
	return json.Marshal(payload)
}

func cloneGeneralOpenAIRequest(req *relaymodel.GeneralOpenAIRequest) (*relaymodel.GeneralOpenAIRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	cloned := &relaymodel.GeneralOpenAIRequest{}
	if err := json.Unmarshal(payload, cloned); err != nil {
		return nil, err
	}
	return cloned, nil
}

func shouldTruncateFieldForRelayDebug(fieldKey string, value string) bool {
	key := strings.ToLower(strings.TrimSpace(fieldKey))
	if key == "image_url" || key == "url" || strings.Contains(key, "image") || strings.Contains(key, "b64") || strings.Contains(key, "base64") {
		return true
	}
	lowerValue := strings.ToLower(strings.TrimSpace(value))
	if strings.HasPrefix(lowerValue, "data:image/") {
		return true
	}
	return false
}

func truncateRelayDebugValue(value string) string {
	const (
		head = 48
		tail = 24
	)
	if len(value) <= head+tail+16 {
		return value
	}
	return fmt.Sprintf("%s...%s(len=%d)", value[:head], value[len(value)-tail:], len(value))
}

func sanitizePayloadForRelayDebugValue(value any, parentKey string) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[key] = sanitizePayloadForRelayDebugValue(child, key)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, child := range typed {
			result = append(result, sanitizePayloadForRelayDebugValue(child, parentKey))
		}
		return result
	case string:
		if shouldTruncateFieldForRelayDebug(parentKey, typed) {
			return truncateRelayDebugValue(typed)
		}
		return typed
	default:
		return value
	}
}

func sanitizePayloadForRelayDebug(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Sprintf("{\"_parse_error\":%q,\"_raw_len\":%d}", err.Error(), len(raw))
	}
	sanitized := sanitizePayloadForRelayDebugValue(payload, "").(map[string]any)
	encoded, err := json.Marshal(sanitized)
	if err != nil {
		return fmt.Sprintf("{\"_marshal_error\":%q,\"_raw_len\":%d}", err.Error(), len(raw))
	}
	return string(encoded)
}

func supportsMessagesUpstream(meta *meta.Meta) bool {
	if meta == nil {
		return false
	}
	return meta.APIType == apitype.Anthropic || meta.APIType == apitype.AwsClaude
}

func endpointByRelayMode(mode int) string {
	switch mode {
	case relaymode.Messages:
		return adminmodel.ChannelModelEndpointMessages
	case relaymode.Responses:
		return adminmodel.ChannelModelEndpointResponses
	default:
		return adminmodel.ChannelModelEndpointChat
	}
}

func resolveRequestedTextEndpoint(meta *meta.Meta) string {
	if meta == nil {
		return adminmodel.ChannelModelEndpointChat
	}
	if normalized := adminmodel.NormalizeRequestedChannelModelEndpoint(meta.RequestURLPath); normalized != "" {
		return normalized
	}
	return endpointByRelayMode(meta.Mode)
}

func resolveSelectedModelDirectTextEndpointSupport(meta *meta.Meta, row adminmodel.ChannelModel, originModelName string, actualModelName string) (supportsChat bool, supportsResponses bool, supportsMessages bool) {
	if meta == nil {
		return supportsChat, supportsResponses, supportsMessages
	}
	modelCandidates := []string{
		strings.TrimSpace(row.Model),
		strings.TrimSpace(row.UpstreamModel),
		strings.TrimSpace(actualModelName),
		strings.TrimSpace(originModelName),
	}
	endpointMap := adminmodel.CacheGetChannelModelEndpointSupport(meta.ChannelId, modelCandidates...)
	if len(endpointMap) == 0 {
		return supportsChat, supportsResponses, supportsMessages
	}
	if enabled, ok := endpointMap[adminmodel.ChannelModelEndpointChat]; ok && enabled {
		supportsChat = true
	}
	if enabled, ok := endpointMap[adminmodel.ChannelModelEndpointResponses]; ok && enabled {
		supportsResponses = true
	}
	if enabled, ok := endpointMap[adminmodel.ChannelModelEndpointMessages]; ok && enabled {
		supportsMessages = true
	}
	return supportsChat, supportsResponses, supportsMessages
}

func hasSelectedTextChannelModelConfigs(rows []adminmodel.ChannelModel) bool {
	for _, row := range adminmodel.NormalizeChannelModelsPreserveOrder(rows) {
		if !row.Selected || row.Inactive {
			continue
		}
		endpoint := adminmodel.NormalizeChannelModelEndpoint(row.Type, row.Endpoint)
		if endpoint == adminmodel.ChannelModelEndpointChat ||
			endpoint == adminmodel.ChannelModelEndpointResponses ||
			endpoint == adminmodel.ChannelModelEndpointMessages {
			return true
		}
	}
	return false
}

func resolveChannelTextUpstream(meta *meta.Meta, originModelName string, actualModelName string) (int, string, error) {
	if meta == nil {
		return relaymode.ChatCompletions, adminmodel.ChannelModelEndpointChat, nil
	}
	requestEndpoint := resolveRequestedTextEndpoint(meta)
	if row, ok := adminmodel.FindSelectedChannelModelConfig(meta.ChannelModelConfigs, originModelName, actualModelName); ok {
		supportsChat, supportsResponses, supportsMessagesDirect := resolveSelectedModelDirectTextEndpointSupport(meta, row, originModelName, actualModelName)
		supportsMessages := supportsMessagesDirect && supportsMessagesUpstream(meta)

		switch requestEndpoint {
		case adminmodel.ChannelModelEndpointChat:
			if supportsChat {
				return relaymode.ChatCompletions, adminmodel.ChannelModelEndpointChat, nil
			}
		case adminmodel.ChannelModelEndpointMessages:
			if supportsMessages {
				return relaymode.Messages, adminmodel.ChannelModelEndpointMessages, nil
			}
		case adminmodel.ChannelModelEndpointResponses:
			if supportsResponses {
				return relaymode.Responses, adminmodel.ChannelModelEndpointResponses, nil
			}
		}

		return 0, "", fmt.Errorf(
			"channel model %q does not support request endpoint %s",
			strings.TrimSpace(row.Model),
			requestEndpoint,
		)
	}
	if hasSelectedTextChannelModelConfigs(meta.ChannelModelConfigs) {
		requestModel := strings.TrimSpace(actualModelName)
		if requestModel == "" {
			requestModel = strings.TrimSpace(originModelName)
		}
		if requestModel == "" {
			requestModel = "(empty)"
		}
		return 0, "", fmt.Errorf("requested model %q is not selected for this channel", requestModel)
	}
	return 0, "", fmt.Errorf("channel does not have selected model endpoint config for %s", requestEndpoint)
}

func convertTextRequestForUpstream(req *relaymodel.GeneralOpenAIRequest, downstreamMode int, upstreamMode int) (*relaymodel.GeneralOpenAIRequest, error) {
	cloned, err := cloneGeneralOpenAIRequest(req)
	if err != nil {
		return nil, err
	}
	if downstreamMode != upstreamMode &&
		(downstreamMode == relaymode.ChatCompletions || downstreamMode == relaymode.Messages || downstreamMode == relaymode.Responses) &&
		(upstreamMode == relaymode.ChatCompletions || upstreamMode == relaymode.Messages || upstreamMode == relaymode.Responses) {
		return nil, fmt.Errorf(
			"text endpoint conversion is not allowed: downstream=%s upstream=%s",
			relayModeLabel(downstreamMode),
			relayModeLabel(upstreamMode),
		)
	}
	return cloned, nil
}
