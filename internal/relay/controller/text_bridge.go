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
	if supportsMessagesUpstream(meta) {
		if requestEndpoint != adminmodel.ChannelModelEndpointMessages {
			return 0, "", fmt.Errorf("channel does not support %s without selected model endpoint config", requestEndpoint)
		}
		return relaymode.Messages, adminmodel.ChannelModelEndpointMessages, nil
	}
	switch requestEndpoint {
	case adminmodel.ChannelModelEndpointChat:
		return relaymode.ChatCompletions, adminmodel.ChannelModelEndpointChat, nil
	case adminmodel.ChannelModelEndpointResponses:
		return relaymode.Responses, adminmodel.ChannelModelEndpointResponses, nil
	default:
		return 0, "", fmt.Errorf("channel does not support %s without selected model endpoint config", requestEndpoint)
	}
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

func parsePositiveIntFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return typed, true
		}
	case int32:
		if typed > 0 {
			return int(typed), true
		}
	case int64:
		if typed > 0 {
			return int(typed), true
		}
	case float64:
		parsed := int(typed)
		if parsed > 0 && float64(parsed) == typed {
			return parsed, true
		}
	case json.Number:
		if parsed, err := typed.Int64(); err == nil && parsed > 0 {
			return int(parsed), true
		}
	}
	return 0, false
}

func normalizeResponsesToolDefinitions(payload map[string]any) {
	toolsRaw, ok := payload["tools"].([]any)
	if !ok || len(toolsRaw) == 0 {
		return
	}
	normalized := make([]any, 0, len(toolsRaw))
	changed := false
	for _, toolAny := range toolsRaw {
		toolMap, ok := toolAny.(map[string]any)
		if !ok {
			normalized = append(normalized, toolAny)
			continue
		}
		functionAny, hasFunction := toolMap["function"]
		if !hasFunction {
			normalized = append(normalized, toolAny)
			continue
		}
		functionMap, ok := functionAny.(map[string]any)
		if !ok {
			normalized = append(normalized, toolAny)
			continue
		}

		flat := make(map[string]any, len(toolMap)+4)
		for key, value := range toolMap {
			if key == "function" {
				continue
			}
			flat[key] = value
		}
		if strings.TrimSpace(fmt.Sprint(flat["type"])) == "" {
			flat["type"] = "function"
		}
		if name := strings.TrimSpace(fmt.Sprint(functionMap["name"])); name != "" {
			flat["name"] = name
		}
		if description := strings.TrimSpace(fmt.Sprint(functionMap["description"])); description != "" {
			flat["description"] = description
		}
		if parameters, exists := functionMap["parameters"]; exists {
			flat["parameters"] = parameters
		}
		if strict, exists := functionMap["strict"]; exists {
			flat["strict"] = strict
		}
		normalized = append(normalized, flat)
		changed = true
	}
	if changed {
		payload["tools"] = normalized
	}
}

func normalizeResponsesLegacyFunctions(payload map[string]any) {
	functionsRaw, ok := payload["functions"].([]any)
	if !ok || len(functionsRaw) == 0 {
		return
	}
	toolsRaw, _ := payload["tools"].([]any)
	normalizedTools := make([]any, 0, len(toolsRaw)+len(functionsRaw))
	normalizedTools = append(normalizedTools, toolsRaw...)
	for _, functionAny := range functionsRaw {
		functionMap, ok := functionAny.(map[string]any)
		if !ok {
			continue
		}
		tool := map[string]any{
			"type": "function",
		}
		if name := strings.TrimSpace(fmt.Sprint(functionMap["name"])); name != "" {
			tool["name"] = name
		}
		if description := strings.TrimSpace(fmt.Sprint(functionMap["description"])); description != "" {
			tool["description"] = description
		}
		if parameters, exists := functionMap["parameters"]; exists {
			tool["parameters"] = parameters
		}
		normalizedTools = append(normalizedTools, tool)
	}
	if len(normalizedTools) > 0 {
		payload["tools"] = normalizedTools
	}
	delete(payload, "functions")
}

func normalizeResponsesToolChoice(payload map[string]any) {
	toolChoice, exists := payload["tool_choice"]
	if exists {
		toolChoiceMap, ok := toolChoice.(map[string]any)
		if ok {
			if functionAny, hasFunction := toolChoiceMap["function"]; hasFunction {
				if functionMap, ok := functionAny.(map[string]any); ok {
					if name := strings.TrimSpace(fmt.Sprint(functionMap["name"])); name != "" {
						payload["tool_choice"] = map[string]any{
							"type": "function",
							"name": name,
						}
					}
				}
			}
		}
	}

	if payload["tool_choice"] != nil {
		return
	}
	functionCall, exists := payload["function_call"]
	if !exists {
		return
	}
	switch typed := functionCall.(type) {
	case string:
		candidate := strings.TrimSpace(typed)
		if candidate == "auto" || candidate == "none" || candidate == "required" {
			payload["tool_choice"] = candidate
		}
	case map[string]any:
		if name := strings.TrimSpace(fmt.Sprint(typed["name"])); name != "" {
			payload["tool_choice"] = map[string]any{
				"type": "function",
				"name": name,
			}
		}
	}
	delete(payload, "function_call")
}

func normalizeResponsesOutputFormat(payload map[string]any) {
	responseFormatAny, exists := payload["response_format"]
	if !exists {
		return
	}
	responseFormat, ok := responseFormatAny.(map[string]any)
	if !ok {
		return
	}
	formatType := strings.TrimSpace(fmt.Sprint(responseFormat["type"]))
	if formatType == "" {
		return
	}
	format := map[string]any{
		"type": formatType,
	}
	if formatType == "json_schema" {
		if schemaAny, ok := responseFormat["json_schema"].(map[string]any); ok {
			for _, key := range []string{"name", "schema", "strict", "description"} {
				if value, exists := schemaAny[key]; exists {
					format[key] = value
				}
			}
		}
	}
	text, _ := payload["text"].(map[string]any)
	if text == nil {
		text = map[string]any{}
	}
	text["format"] = format
	payload["text"] = text
	delete(payload, "response_format")
}

func normalizeResponsesMaxTokens(payload map[string]any) {
	if _, exists := payload["max_output_tokens"]; !exists {
		if maxOutput, ok := parsePositiveIntFromAny(payload["max_completion_tokens"]); ok {
			payload["max_output_tokens"] = maxOutput
		} else if maxOutput, ok := parsePositiveIntFromAny(payload["max_tokens"]); ok {
			payload["max_output_tokens"] = maxOutput
		}
	}
	delete(payload, "max_completion_tokens")
	delete(payload, "max_tokens")
}

func normalizeRequestBodyForResponses(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}

	normalizeResponsesLegacyFunctions(payload)
	normalizeResponsesToolDefinitions(payload)
	normalizeResponsesToolChoice(payload)
	normalizeResponsesOutputFormat(payload)
	normalizeResponsesMaxTokens(payload)

	// responses does not support `n`; keep single-choice semantics only.
	delete(payload, "n")

	return json.Marshal(payload)
}
