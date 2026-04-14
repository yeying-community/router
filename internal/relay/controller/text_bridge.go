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

func normalizeResponsesInputValue(input any) (any, bool) {
	if input == nil {
		return nil, false
	}
	switch value := input.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return input, false
		}
		return []relaymodel.Message{{
			Role:    "user",
			Content: value,
		}}, true
	case []string:
		if len(value) == 0 {
			return input, false
		}
		messages := make([]relaymodel.Message, 0, len(value))
		for _, item := range value {
			if strings.TrimSpace(item) == "" {
				continue
			}
			messages = append(messages, relaymodel.Message{
				Role:    "user",
				Content: item,
			})
		}
		if len(messages) == 0 {
			return input, false
		}
		return messages, true
	case []any:
		if len(value) == 0 {
			return input, false
		}
		messages := make([]relaymodel.Message, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				return input, false
			}
			if strings.TrimSpace(text) == "" {
				continue
			}
			messages = append(messages, relaymodel.Message{
				Role:    "user",
				Content: text,
			})
		}
		if len(messages) == 0 {
			return input, false
		}
		return messages, true
	}

	payload, err := json.Marshal(input)
	if err != nil {
		return input, false
	}

	var list []any
	if err := json.Unmarshal(payload, &list); err == nil {
		if len(list) == 0 {
			return input, false
		}
		return list, false
	}

	var single map[string]any
	if err := json.Unmarshal(payload, &single); err == nil && len(single) > 0 {
		return []any{single}, true
	}

	return input, false
}

func normalizeResponsesInput(req *relaymodel.GeneralOpenAIRequest) bool {
	if req == nil {
		return false
	}
	normalized, changed := normalizeResponsesInputValue(req.Input)
	if changed {
		req.Input = normalized
	}
	return changed
}

func normalizeResponsesRequestBody(raw []byte, modelName string) ([]byte, error) {
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
	if payload["input"] == nil {
		if parsed, ok := parseMessagesAnyForResponsesInput(payload["messages"]); ok {
			payload["input"] = parsed
			delete(payload, "messages")
		}
	}
	normalized, changed := normalizeResponsesInputValue(payload["input"])
	if changed {
		payload["input"] = normalized
	}
	if parsed, ok := parseMessagesAnyForResponsesInput(payload["input"]); ok {
		payload["input"] = parsed
	}
	return json.Marshal(payload)
}

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

func convertMessageContentForResponses(content any) any {
	switch value := content.(type) {
	case string:
		return []any{
			map[string]any{
				"type": "input_text",
				"text": value,
			},
		}
	case []any:
		converted := make([]any, 0, len(value))
		changed := false
		for _, item := range value {
			block, ok := item.(map[string]any)
			if !ok {
				converted = append(converted, item)
				continue
			}
			blockType := strings.TrimSpace(fmt.Sprint(block["type"]))
			switch blockType {
			case relaymodel.ContentTypeText:
				text, _ := block["text"].(string)
				converted = append(converted, map[string]any{
					"type": "input_text",
					"text": text,
				})
				changed = true
			case relaymodel.ContentTypeImageURL:
				url := ""
				detail := ""
				switch imageURL := block["image_url"].(type) {
				case map[string]any:
					url = strings.TrimSpace(fmt.Sprint(imageURL["url"]))
					detail = strings.TrimSpace(fmt.Sprint(imageURL["detail"]))
				case string:
					url = strings.TrimSpace(imageURL)
				default:
					converted = append(converted, item)
					continue
				}
				if url == "" {
					converted = append(converted, item)
					continue
				}
				imagePart := map[string]any{
					"type":      "input_image",
					"image_url": url,
				}
				if detail != "" {
					imagePart["detail"] = detail
				}
				converted = append(converted, imagePart)
				changed = true
			default:
				converted = append(converted, item)
			}
		}
		if changed {
			return converted
		}
	}
	return content
}

func convertMessagesForResponsesInput(messages []relaymodel.Message) []any {
	if len(messages) == 0 {
		return nil
	}
	input := make([]any, 0, len(messages))
	for _, message := range messages {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = "user"
		}
		item := map[string]any{"role": role}
		if message.Content != nil {
			item["content"] = convertMessageContentForResponses(message.Content)
		}
		if message.Name != nil {
			if name := strings.TrimSpace(*message.Name); name != "" {
				item["name"] = name
			}
		}
		if len(message.ToolCalls) > 0 {
			item["tool_calls"] = message.ToolCalls
		}
		if toolCallID := strings.TrimSpace(message.ToolCallId); toolCallID != "" {
			item["tool_call_id"] = toolCallID
		}
		input = append(input, item)
	}
	return input
}

func parseMessagesAnyForResponsesInput(value any) ([]any, bool) {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return nil, false
	}
	input := make([]any, 0, len(items))
	for _, item := range items {
		message, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		role := strings.TrimSpace(fmt.Sprint(message["role"]))
		if role == "" {
			role = "user"
		}
		converted := map[string]any{"role": role}
		if content, exists := message["content"]; exists {
			converted["content"] = convertMessageContentForResponses(content)
		}
		if name := strings.TrimSpace(fmt.Sprint(message["name"])); name != "" {
			converted["name"] = name
		}
		if toolCalls, exists := message["tool_calls"]; exists {
			converted["tool_calls"] = toolCalls
		}
		if toolCallID := strings.TrimSpace(fmt.Sprint(message["tool_call_id"])); toolCallID != "" {
			converted["tool_call_id"] = toolCallID
		}
		input = append(input, converted)
	}
	return input, true
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

func parseInputAsMessages(input any) []relaymodel.Message {
	if input == nil {
		return nil
	}
	switch value := input.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return []relaymodel.Message{{
			Role:    "user",
			Content: value,
		}}
	case []string:
		if len(value) == 0 {
			return nil
		}
		messages := make([]relaymodel.Message, 0, len(value))
		for _, item := range value {
			if strings.TrimSpace(item) == "" {
				continue
			}
			messages = append(messages, relaymodel.Message{Role: "user", Content: item})
		}
		return messages
	}

	payload, err := json.Marshal(input)
	if err != nil {
		return nil
	}

	messages := make([]relaymodel.Message, 0)
	if err := json.Unmarshal(payload, &messages); err == nil && len(messages) > 0 {
		return messages
	}

	stringList := make([]string, 0)
	if err := json.Unmarshal(payload, &stringList); err == nil && len(stringList) > 0 {
		messages := make([]relaymodel.Message, 0, len(stringList))
		for _, item := range stringList {
			if strings.TrimSpace(item) == "" {
				continue
			}
			messages = append(messages, relaymodel.Message{Role: "user", Content: item})
		}
		return messages
	}

	single := map[string]any{}
	if err := json.Unmarshal(payload, &single); err == nil {
		role := strings.TrimSpace(fmt.Sprintf("%v", single["role"]))
		if role == "" {
			role = "user"
		}
		return []relaymodel.Message{{
			Role:    role,
			Content: single["content"],
		}}
	}
	return nil
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

func rowSupportsTextEndpoint(row adminmodel.ChannelModel, endpoint string) bool {
	normalizedEndpoint := adminmodel.NormalizeRequestedChannelModelEndpoint(endpoint)
	if normalizedEndpoint == "" {
		return false
	}
	for _, candidate := range adminmodel.ResolveChannelModelCapabilityEndpoints(row) {
		if adminmodel.NormalizeRequestedChannelModelEndpoint(candidate) == normalizedEndpoint {
			return true
		}
	}
	return false
}

func resolveSelectedModelDirectTextEndpointSupport(meta *meta.Meta, row adminmodel.ChannelModel, originModelName string, actualModelName string) (supportsChat bool, supportsResponses bool, supportsMessages bool) {
	directEndpoints := adminmodel.ResolveChannelModelDirectEndpoints(row)
	for _, endpoint := range directEndpoints {
		switch adminmodel.NormalizeRequestedChannelModelEndpoint(endpoint) {
		case adminmodel.ChannelModelEndpointChat:
			supportsChat = true
		case adminmodel.ChannelModelEndpointResponses:
			supportsResponses = true
		case adminmodel.ChannelModelEndpointMessages:
			supportsMessages = true
		}
	}
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
	if enabled, ok := endpointMap[adminmodel.ChannelModelEndpointChat]; ok && supportsChat && !enabled {
		supportsChat = false
	}
	if enabled, ok := endpointMap[adminmodel.ChannelModelEndpointResponses]; ok && supportsResponses && !enabled {
		supportsResponses = false
	}
	if enabled, ok := endpointMap[adminmodel.ChannelModelEndpointMessages]; ok && supportsMessages && !enabled {
		supportsMessages = false
	}
	return supportsChat, supportsResponses, supportsMessages
}

func hasSelectedTextChannelModelConfigs(rows []adminmodel.ChannelModel) bool {
	for _, row := range adminmodel.NormalizeChannelModelConfigsPreserveOrder(rows) {
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
			if supportsMessages {
				return relaymode.Messages, adminmodel.ChannelModelEndpointMessages, nil
			}
			if supportsResponses {
				return relaymode.Responses, adminmodel.ChannelModelEndpointResponses, nil
			}
		case adminmodel.ChannelModelEndpointMessages:
			if supportsMessages {
				return relaymode.Messages, adminmodel.ChannelModelEndpointMessages, nil
			}
			if supportsChat {
				return relaymode.ChatCompletions, adminmodel.ChannelModelEndpointChat, nil
			}
			if supportsResponses {
				return relaymode.Responses, adminmodel.ChannelModelEndpointResponses, nil
			}
		case adminmodel.ChannelModelEndpointResponses:
			if supportsResponses {
				return relaymode.Responses, adminmodel.ChannelModelEndpointResponses, nil
			}
			if supportsChat {
				return relaymode.ChatCompletions, adminmodel.ChannelModelEndpointChat, nil
			}
		}

		if supportsMessagesUpstream(meta) {
			if supportsMessages {
				return relaymode.Messages, adminmodel.ChannelModelEndpointMessages, nil
			}
			if supportsChat {
				return relaymode.ChatCompletions, adminmodel.ChannelModelEndpointChat, nil
			}
			if supportsResponses {
				return relaymode.Responses, adminmodel.ChannelModelEndpointResponses, nil
			}
		} else {
			if supportsResponses {
				return relaymode.Responses, adminmodel.ChannelModelEndpointResponses, nil
			}
			if supportsChat {
				return relaymode.ChatCompletions, adminmodel.ChannelModelEndpointChat, nil
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
	if supportsMessagesUpstream(meta) {
		if requestEndpoint == adminmodel.ChannelModelEndpointResponses {
			return 0, "", fmt.Errorf("channel does not support %s without selected model endpoint config", adminmodel.ChannelModelEndpointResponses)
		}
		return relaymode.Messages, adminmodel.ChannelModelEndpointMessages, nil
	}
	return relaymode.Responses, adminmodel.ChannelModelEndpointResponses, nil
}

func convertTextRequestForUpstream(req *relaymodel.GeneralOpenAIRequest, downstreamMode int, upstreamMode int) (*relaymodel.GeneralOpenAIRequest, error) {
	cloned, err := cloneGeneralOpenAIRequest(req)
	if err != nil {
		return nil, err
	}

	if upstreamMode == relaymode.Responses {
		if cloned.Input == nil && len(cloned.Messages) > 0 {
			cloned.Input = convertMessagesForResponsesInput(cloned.Messages)
			cloned.Messages = nil
		}
		if cloned.MaxOutputTokens == nil && cloned.MaxTokens > 0 {
			value := cloned.MaxTokens
			cloned.MaxOutputTokens = &value
			cloned.MaxTokens = 0
		}
		return cloned, nil
	}

	if upstreamMode == relaymode.ChatCompletions {
		if len(cloned.Messages) == 0 {
			cloned.Messages = parseInputAsMessages(cloned.Input)
		}
		if len(cloned.Messages) == 0 {
			return nil, fmt.Errorf("field messages or input is required")
		}
		cloned.Input = nil
		if cloned.MaxTokens == 0 && cloned.MaxOutputTokens != nil && *cloned.MaxOutputTokens > 0 {
			cloned.MaxTokens = *cloned.MaxOutputTokens
		}
		return cloned, nil
	}

	if upstreamMode == relaymode.Messages {
		if len(cloned.Messages) == 0 {
			cloned.Messages = parseInputAsMessages(cloned.Input)
		}
		if len(cloned.Messages) == 0 {
			return nil, fmt.Errorf("field messages or input is required")
		}
		cloned.Input = nil
		if cloned.MaxTokens == 0 && cloned.MaxOutputTokens != nil && *cloned.MaxOutputTokens > 0 {
			cloned.MaxTokens = *cloned.MaxOutputTokens
		}
		return cloned, nil
	}

	return cloned, nil
}

func shouldForceUpstreamTextStream(downstreamMode int, upstreamMode int, downstreamStream bool) bool {
	if downstreamStream {
		return false
	}
	if downstreamMode != relaymode.ChatCompletions {
		return false
	}
	return upstreamMode == relaymode.Responses || upstreamMode == relaymode.Messages
}
