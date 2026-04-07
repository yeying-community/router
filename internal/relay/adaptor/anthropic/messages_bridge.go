package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"

	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

type inboundMessagesRequest struct {
	Model         string                     `json:"model"`
	Messages      []inboundMessagesItem      `json:"messages"`
	System        any                        `json:"system,omitempty"`
	MaxTokens     int                        `json:"max_tokens,omitempty"`
	StopSequences []string                   `json:"stop_sequences,omitempty"`
	Stream        bool                       `json:"stream,omitempty"`
	Temperature   *float64                   `json:"temperature,omitempty"`
	TopP          *float64                   `json:"top_p,omitempty"`
	TopK          int                        `json:"top_k,omitempty"`
	Tools         []Tool                     `json:"tools,omitempty"`
	ToolChoice    any                        `json:"tool_choice,omitempty"`
	Metadata      map[string]json.RawMessage `json:"metadata,omitempty"`
}

type inboundMessagesItem struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type openAIContentBuilder struct {
	textParts []string
	parts     []any
}

func ParseMessagesRequestToGeneralOpenAIRequest(raw []byte) (*relaymodel.GeneralOpenAIRequest, error) {
	request := &inboundMessagesRequest{}
	if err := json.Unmarshal(raw, request); err != nil {
		return nil, err
	}
	return convertMessagesRequestToGeneralOpenAIRequest(request), nil
}

func convertMessagesRequestToGeneralOpenAIRequest(request *inboundMessagesRequest) *relaymodel.GeneralOpenAIRequest {
	if request == nil {
		return &relaymodel.GeneralOpenAIRequest{}
	}
	converted := &relaymodel.GeneralOpenAIRequest{
		Model:       strings.TrimSpace(request.Model),
		MaxTokens:   request.MaxTokens,
		Stream:      request.Stream,
		Temperature: request.Temperature,
		TopP:        request.TopP,
		TopK:        request.TopK,
		Tools:       convertAnthropicToolsToOpenAITools(request.Tools),
		ToolChoice:  convertAnthropicToolChoiceToOpenAI(request.ToolChoice),
		Messages:    convertAnthropicMessagesToOpenAIMessages(request.Messages),
	}
	if len(request.StopSequences) > 0 {
		converted.Stop = append([]string(nil), request.StopSequences...)
	}
	if systemMessage := convertAnthropicSystemToOpenAIMessage(request.System); systemMessage != nil {
		converted.Messages = append([]relaymodel.Message{*systemMessage}, converted.Messages...)
	}
	return converted
}

func convertAnthropicToolsToOpenAITools(tools []Tool) []relaymodel.Tool {
	if len(tools) == 0 {
		return nil
	}
	converted := make([]relaymodel.Tool, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		parameters := map[string]any{
			"type": "object",
		}
		if schemaType := strings.TrimSpace(tool.InputSchema.Type); schemaType != "" {
			parameters["type"] = schemaType
		}
		if tool.InputSchema.Properties != nil {
			parameters["properties"] = tool.InputSchema.Properties
		}
		parameters["required"] = normalizeToolRequired(tool.InputSchema.Required)
		converted = append(converted, relaymodel.Tool{
			Type: "function",
			Function: relaymodel.Function{
				Name:        name,
				Description: strings.TrimSpace(tool.Description),
				Parameters:  parameters,
			},
		})
	}
	if len(converted) == 0 {
		return nil
	}
	return converted
}

func normalizeToolRequired(value any) []string {
	switch typed := value.(type) {
	case nil:
		return []string{}
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			name := strings.TrimSpace(item)
			if name == "" {
				continue
			}
			result = append(result, name)
		}
		return result
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			name := strings.TrimSpace(fmt.Sprint(item))
			if name == "" {
				continue
			}
			result = append(result, name)
		}
		return result
	default:
		return []string{}
	}
}

func convertAnthropicToolChoiceToOpenAI(choice any) any {
	if choice == nil {
		return nil
	}
	switch value := choice.(type) {
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "any":
			return "required"
		case "auto", "none", "required":
			return strings.ToLower(strings.TrimSpace(value))
		default:
			return strings.TrimSpace(value)
		}
	case map[string]any:
		choiceType := strings.ToLower(strings.TrimSpace(fmt.Sprint(value["type"])))
		switch choiceType {
		case "tool":
			name := strings.TrimSpace(fmt.Sprint(value["name"]))
			if name == "" {
				return "auto"
			}
			return map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": name,
				},
			}
		case "any":
			return "required"
		case "auto", "none", "required":
			return choiceType
		default:
			return value
		}
	default:
		return choice
	}
}

func convertAnthropicSystemToOpenAIMessage(system any) *relaymodel.Message {
	systemText := extractTextFromAnthropicContent(system)
	if strings.TrimSpace(systemText) == "" {
		return nil
	}
	return &relaymodel.Message{
		Role:    "system",
		Content: systemText,
	}
}

func convertAnthropicMessagesToOpenAIMessages(messages []inboundMessagesItem) []relaymodel.Message {
	if len(messages) == 0 {
		return nil
	}
	converted := make([]relaymodel.Message, 0, len(messages))
	for _, message := range messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role == "" {
			role = "user"
		}

		baseMessage := relaymodel.Message{Role: role}
		builder := &openAIContentBuilder{}
		toolResults := make([]relaymodel.Message, 0)

		contentBlocks := normalizeAnthropicContentBlocks(message.Content)
		if len(contentBlocks) == 0 {
			fallbackText := extractTextFromAnthropicContent(message.Content)
			if strings.TrimSpace(fallbackText) != "" {
				baseMessage.Content = fallbackText
				converted = append(converted, baseMessage)
			}
			continue
		}

		for _, block := range contentBlocks {
			blockType := strings.ToLower(strings.TrimSpace(fmt.Sprint(block["type"])))
			switch blockType {
			case "text":
				builder.addText(strings.TrimSpace(fmt.Sprint(block["text"])))
			case "image":
				dataURL := parseAnthropicImageDataURL(block["source"])
				if dataURL == "" {
					continue
				}
				builder.addImageURL(dataURL)
			case "tool_use":
				toolCall := buildToolCallFromAnthropicBlock(block)
				if strings.TrimSpace(toolCall.Function.Name) == "" {
					continue
				}
				baseMessage.ToolCalls = append(baseMessage.ToolCalls, toolCall)
			case "tool_result":
				toolResult := relaymodel.Message{
					Role:       "tool",
					ToolCallId: strings.TrimSpace(fmt.Sprint(block["tool_use_id"])),
					Content:    extractTextFromAnthropicContent(block["content"]),
				}
				if strings.TrimSpace(toolResult.Content.(string)) == "" {
					continue
				}
				toolResults = append(toolResults, toolResult)
			default:
				if text := strings.TrimSpace(fmt.Sprint(block["text"])); text != "" {
					builder.addText(text)
				}
			}
		}

		if builder.hasContent() {
			baseMessage.Content = builder.content()
		}
		if hasMessageContent(baseMessage) || len(baseMessage.ToolCalls) > 0 {
			converted = append(converted, baseMessage)
		}
		converted = append(converted, toolResults...)
	}
	if len(converted) == 0 {
		return nil
	}
	return converted
}

func normalizeAnthropicContentBlocks(content any) []map[string]any {
	switch value := content.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return []map[string]any{{
			"type": "text",
			"text": value,
		}}
	case map[string]any:
		return []map[string]any{value}
	case []any:
		blocks := make([]map[string]any, 0, len(value))
		for _, item := range value {
			switch typed := item.(type) {
			case map[string]any:
				blocks = append(blocks, typed)
			case string:
				if strings.TrimSpace(typed) == "" {
					continue
				}
				blocks = append(blocks, map[string]any{
					"type": "text",
					"text": typed,
				})
			default:
				raw, err := json.Marshal(item)
				if err != nil {
					continue
				}
				obj := map[string]any{}
				if err := json.Unmarshal(raw, &obj); err == nil {
					blocks = append(blocks, obj)
				}
			}
		}
		return blocks
	default:
		return nil
	}
}

func extractTextFromAnthropicContent(content any) string {
	switch value := content.(type) {
	case string:
		return strings.TrimSpace(value)
	case []any:
		texts := make([]string, 0, len(value))
		for _, item := range value {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			blockType := strings.ToLower(strings.TrimSpace(fmt.Sprint(block["type"])))
			if blockType != "text" {
				continue
			}
			text := strings.TrimSpace(fmt.Sprint(block["text"]))
			if text == "" {
				continue
			}
			texts = append(texts, text)
		}
		if len(texts) > 0 {
			return strings.Join(texts, "\n")
		}
	case map[string]any:
		blockType := strings.ToLower(strings.TrimSpace(fmt.Sprint(value["type"])))
		if blockType == "text" {
			text := strings.TrimSpace(fmt.Sprint(value["text"]))
			if text != "" {
				return text
			}
		}
	}
	raw, err := json.Marshal(content)
	if err != nil || string(raw) == "null" {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func parseAnthropicImageDataURL(source any) string {
	sourceMap, ok := source.(map[string]any)
	if !ok {
		return ""
	}
	mediaType := strings.TrimSpace(fmt.Sprint(sourceMap["media_type"]))
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	data := strings.TrimSpace(fmt.Sprint(sourceMap["data"]))
	if data == "" {
		return ""
	}
	return fmt.Sprintf("data:%s;base64,%s", mediaType, data)
}

func buildToolCallFromAnthropicBlock(block map[string]any) relaymodel.Tool {
	name := strings.TrimSpace(fmt.Sprint(block["name"]))
	arguments := "{}"
	if block["input"] != nil {
		raw, err := json.Marshal(block["input"])
		if err == nil && len(raw) > 0 && string(raw) != "null" {
			arguments = string(raw)
		}
	}
	return relaymodel.Tool{
		Id:   strings.TrimSpace(fmt.Sprint(block["id"])),
		Type: "function",
		Function: relaymodel.Function{
			Name:      name,
			Arguments: arguments,
		},
	}
}

func hasMessageContent(message relaymodel.Message) bool {
	switch value := message.Content.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(value) != ""
	case []any:
		return len(value) > 0
	default:
		return true
	}
}

func (b *openAIContentBuilder) addText(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	if len(b.parts) > 0 {
		b.parts = append(b.parts, map[string]any{
			"type": "text",
			"text": text,
		})
		return
	}
	b.textParts = append(b.textParts, text)
}

func (b *openAIContentBuilder) addImageURL(url string) {
	if strings.TrimSpace(url) == "" {
		return
	}
	if len(b.parts) == 0 && len(b.textParts) > 0 {
		for _, text := range b.textParts {
			b.parts = append(b.parts, map[string]any{
				"type": "text",
				"text": text,
			})
		}
		b.textParts = nil
	}
	b.parts = append(b.parts, map[string]any{
		"type": "image_url",
		"image_url": map[string]any{
			"url": url,
		},
	})
}

func (b *openAIContentBuilder) hasContent() bool {
	if len(b.parts) > 0 {
		return true
	}
	for _, text := range b.textParts {
		if strings.TrimSpace(text) != "" {
			return true
		}
	}
	return false
}

func (b *openAIContentBuilder) content() any {
	if len(b.parts) > 0 {
		return b.parts
	}
	return strings.Join(b.textParts, "\n")
}
