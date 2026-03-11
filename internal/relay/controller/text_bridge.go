package controller

import (
	"encoding/json"
	"fmt"
	"strings"

	adminmodel "github.com/yeying-community/router/internal/admin/model"
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

func normalizeResponsesRequestBody(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	normalized, changed := normalizeResponsesInputValue(payload["input"])
	if changed {
		payload["input"] = normalized
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

func resolveChannelTextUpstream(meta *meta.Meta, originModelName string, actualModelName string) (int, string, error) {
	if meta == nil {
		return relaymode.ChatCompletions, adminmodel.ChannelModelEndpointChat, nil
	}
	if row, ok := adminmodel.FindSelectedChannelModelConfig(meta.ChannelModelConfigs, originModelName, actualModelName); ok {
		endpoint := adminmodel.NormalizeChannelModelEndpoint(row.Type, row.Endpoint)
		if endpoint == adminmodel.ChannelModelEndpointResponses {
			return relaymode.Responses, endpoint, nil
		}
		if meta.Mode == relaymode.Responses {
			return 0, "", fmt.Errorf("channel model %q does not support %s", strings.TrimSpace(row.Model), adminmodel.ChannelModelEndpointResponses)
		}
		return relaymode.ChatCompletions, adminmodel.ChannelModelEndpointChat, nil
	}

	fallbackEndpoint := ""
	for _, row := range adminmodel.NormalizeChannelModelConfigsPreserveOrder(meta.ChannelModelConfigs) {
		if !row.Selected {
			continue
		}
		if adminmodel.InferModelType(row.Model) != adminmodel.ProviderModelTypeText &&
			adminmodel.InferModelType(row.UpstreamModel) != adminmodel.ProviderModelTypeText &&
			adminmodel.NormalizeChannelModelEndpoint(row.Type, row.Endpoint) != adminmodel.ChannelModelEndpointChat &&
			adminmodel.NormalizeChannelModelEndpoint(row.Type, row.Endpoint) != adminmodel.ChannelModelEndpointResponses {
			continue
		}
		endpoint := adminmodel.NormalizeChannelModelEndpoint(row.Type, row.Endpoint)
		if endpoint == adminmodel.ChannelModelEndpointResponses {
			return relaymode.Responses, adminmodel.ChannelModelEndpointResponses, nil
		}
		if fallbackEndpoint == "" {
			fallbackEndpoint = endpoint
		}
	}
	if fallbackEndpoint == adminmodel.ChannelModelEndpointChat {
		if meta.Mode == relaymode.Responses {
			return 0, "", fmt.Errorf("selected channel models do not support %s", adminmodel.ChannelModelEndpointResponses)
		}
		return relaymode.ChatCompletions, fallbackEndpoint, nil
	}
	if fallbackEndpoint == adminmodel.ChannelModelEndpointResponses {
		return relaymode.Responses, fallbackEndpoint, nil
	}
	if meta.Mode == relaymode.Responses {
		return 0, "", fmt.Errorf("channel does not support %s", adminmodel.ChannelModelEndpointResponses)
	}
	return relaymode.ChatCompletions, adminmodel.ChannelModelEndpointChat, nil
}

func convertTextRequestForUpstream(req *relaymodel.GeneralOpenAIRequest, downstreamMode int, upstreamMode int) (*relaymodel.GeneralOpenAIRequest, error) {
	cloned, err := cloneGeneralOpenAIRequest(req)
	if err != nil {
		return nil, err
	}

	if upstreamMode == relaymode.Responses {
		if cloned.Input == nil && len(cloned.Messages) > 0 {
			cloned.Input = cloned.Messages
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

	return cloned, nil
}
