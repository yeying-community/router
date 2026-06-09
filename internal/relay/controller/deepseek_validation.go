package controller

import (
	"fmt"
	"strings"

	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

func validateProviderSpecificTextRequest(meta *meta.Meta, textRequest *relaymodel.GeneralOpenAIRequest, rawBody []byte) error {
	if meta == nil || textRequest == nil {
		return nil
	}
	switch meta.ChannelProtocol {
	case relaychannel.DeepSeek:
		return validateDeepSeekTextRequest(meta, textRequest, rawBody)
	default:
		return nil
	}
}

func validateDeepSeekTextRequest(meta *meta.Meta, textRequest *relaymodel.GeneralOpenAIRequest, rawBody []byte) error {
	if strings.HasSuffix(strings.ToLower(strings.TrimRight(strings.TrimSpace(meta.BaseURL), "/")), "/v1") {
		return fmt.Errorf("DeepSeek base_url 不能追加 /v1，请使用 https://api.deepseek.com 或 https://api.deepseek.com/beta")
	}
	for _, message := range textRequest.Messages {
		if strings.EqualFold(strings.TrimSpace(message.Role), "developer") {
			return fmt.Errorf("DeepSeek 不支持 developer role，请改用 system role")
		}
	}
	if textRequest.MaxCompletionTokens != nil {
		return fmt.Errorf("DeepSeek 不支持 max_completion_tokens，请改用 max_tokens")
	}

	if isDeepSeekThinkingEnabled(textRequest, rawBody) {
		unsupported := make([]string, 0, 4)
		if textRequest.Temperature != nil {
			unsupported = append(unsupported, "temperature")
		}
		if textRequest.TopP != nil {
			unsupported = append(unsupported, "top_p")
		}
		if textRequest.PresencePenalty != nil {
			unsupported = append(unsupported, "presence_penalty")
		}
		if textRequest.FrequencyPenalty != nil {
			unsupported = append(unsupported, "frequency_penalty")
		}
		if len(unsupported) > 0 {
			return fmt.Errorf("DeepSeek thinking 模式不支持参数: %s", strings.Join(unsupported, ", "))
		}
	}

	for _, tool := range textRequest.Tools {
		if tool.Function.Strict == nil || !*tool.Function.Strict {
			continue
		}
		if err := validateDeepSeekStrictToolSchema(tool.Function.Parameters, "tools[].function.parameters"); err != nil {
			return err
		}
	}
	return nil
}

func isDeepSeekThinkingEnabled(textRequest *relaymodel.GeneralOpenAIRequest, rawBody []byte) bool {
	if textRequest == nil {
		return false
	}
	if thinkingMap, ok := textRequest.Thinking.(map[string]any); ok {
		if strings.EqualFold(strings.TrimSpace(readStringValue(thinkingMap["type"])), "disabled") {
			return false
		}
		return true
	}
	return false
}

func validateDeepSeekStrictToolSchema(schema any, path string) error {
	schemaMap, ok := schema.(map[string]any)
	if !ok {
		return nil
	}

	if strings.EqualFold(readStringValue(schemaMap["type"]), "object") {
		properties, _ := schemaMap["properties"].(map[string]any)
		required, _ := schemaMap["required"].([]any)
		additionalProperties, hasAdditionalProperties := schemaMap["additionalProperties"].(bool)
		if !hasAdditionalProperties || additionalProperties {
			return fmt.Errorf("DeepSeek strict tool schema 要求 %s 的 object 节点设置 additionalProperties=false", path)
		}
		if len(properties) > 0 {
			requiredSet := make(map[string]struct{}, len(required))
			for _, item := range required {
				name := strings.TrimSpace(readStringValue(item))
				if name == "" {
					continue
				}
				requiredSet[name] = struct{}{}
			}
			for name := range properties {
				if _, ok := requiredSet[name]; !ok {
					return fmt.Errorf("DeepSeek strict tool schema 要求 %s 的 object 节点把所有 properties 都列入 required，缺少 %s", path, name)
				}
			}
		}
		for name, propertySchema := range properties {
			if err := validateDeepSeekStrictToolSchema(propertySchema, path+".properties."+name); err != nil {
				return err
			}
		}
	}

	if items, ok := schemaMap["items"]; ok {
		if err := validateDeepSeekStrictToolSchema(items, path+".items"); err != nil {
			return err
		}
	}
	for _, key := range []string{"$defs", "definitions"} {
		childDefs, ok := schemaMap[key].(map[string]any)
		if !ok {
			continue
		}
		for name, childSchema := range childDefs {
			if err := validateDeepSeekStrictToolSchema(childSchema, path+"."+key+"."+name); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		childList, ok := schemaMap[key].([]any)
		if !ok {
			continue
		}
		for idx, childSchema := range childList {
			if err := validateDeepSeekStrictToolSchema(childSchema, fmt.Sprintf("%s.%s[%d]", path, key, idx)); err != nil {
				return err
			}
		}
	}
	return nil
}

func readStringValue(value any) string {
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}
