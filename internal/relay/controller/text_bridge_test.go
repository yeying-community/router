package controller

import (
	"encoding/json"
	"testing"

	adminmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/apitype"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

func TestResolveChannelTextUpstreamOpenAIResponsesModelDownstreamChatBridgesToResponses(t *testing.T) {
	meta := &meta.Meta{
		Mode:           relaymode.ChatCompletions,
		RequestURLPath: adminmodel.ChannelModelEndpointChat,
		ChannelModelConfigs: []adminmodel.ChannelModel{{
			Model:     "gpt-4.1",
			Type:      adminmodel.ProviderModelTypeText,
			Selected:  true,
			Endpoint:  adminmodel.ChannelModelEndpointResponses,
			SortOrder: 1,
		}},
	}

	mode, path, err := resolveChannelTextUpstream(meta, "gpt-4.1", "gpt-4.1")
	if err != nil {
		t.Fatalf("resolveChannelTextUpstream returned error: %v", err)
	}
	if mode != relaymode.Responses || path != adminmodel.ChannelModelEndpointResponses {
		t.Fatalf("resolveChannelTextUpstream selected responses bridge = (%d, %q), want (%d, %q)", mode, path, relaymode.Responses, adminmodel.ChannelModelEndpointResponses)
	}
}

func TestResolveChannelTextUpstreamOpenAIResponsesModelDownstreamResponsesUsesResponses(t *testing.T) {
	meta := &meta.Meta{
		Mode:           relaymode.Responses,
		RequestURLPath: adminmodel.ChannelModelEndpointResponses,
		APIType:        apitype.OpenAI,
		ChannelModelConfigs: []adminmodel.ChannelModel{{
			Model:     "gpt-4.1",
			Type:      adminmodel.ProviderModelTypeText,
			Selected:  true,
			Endpoint:  adminmodel.ChannelModelEndpointResponses,
			SortOrder: 1,
		}},
	}

	mode, path, err := resolveChannelTextUpstream(meta, "gpt-4.1", "gpt-4.1")
	if err != nil {
		t.Fatalf("resolveChannelTextUpstream returned error: %v", err)
	}
	if mode != relaymode.Responses || path != adminmodel.ChannelModelEndpointResponses {
		t.Fatalf("resolveChannelTextUpstream selected responses = (%d, %q), want (%d, %q)", mode, path, relaymode.Responses, adminmodel.ChannelModelEndpointResponses)
	}
}

func TestResolveChannelTextUpstreamAnthropicMessagesModelDownstreamMessagesUsesMessages(t *testing.T) {
	meta := &meta.Meta{
		Mode:           relaymode.Messages,
		RequestURLPath: adminmodel.ChannelModelEndpointMessages,
		APIType:        apitype.Anthropic,
		ChannelModelConfigs: []adminmodel.ChannelModel{{
			Model:     "claude-sonnet-4-6",
			Type:      adminmodel.ProviderModelTypeText,
			Selected:  true,
			Endpoint:  adminmodel.ChannelModelEndpointMessages,
			SortOrder: 1,
		}},
	}

	mode, path, err := resolveChannelTextUpstream(meta, "claude-sonnet-4-6", "claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("resolveChannelTextUpstream returned error: %v", err)
	}
	if mode != relaymode.Messages || path != adminmodel.ChannelModelEndpointMessages {
		t.Fatalf("resolveChannelTextUpstream selected messages = (%d, %q), want (%d, %q)", mode, path, relaymode.Messages, adminmodel.ChannelModelEndpointMessages)
	}
}

func TestResolveChannelTextUpstreamAnthropicMessagesModelDownstreamChatBridgesToMessages(t *testing.T) {
	meta := &meta.Meta{
		Mode:           relaymode.ChatCompletions,
		RequestURLPath: adminmodel.ChannelModelEndpointChat,
		APIType:        apitype.Anthropic,
		ChannelModelConfigs: []adminmodel.ChannelModel{{
			Model:     "claude-sonnet-4-6",
			Type:      adminmodel.ProviderModelTypeText,
			Selected:  true,
			Endpoint:  adminmodel.ChannelModelEndpointMessages,
			SortOrder: 1,
		}},
	}

	mode, path, err := resolveChannelTextUpstream(meta, "claude-sonnet-4-6", "claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("resolveChannelTextUpstream returned error: %v", err)
	}
	if mode != relaymode.Messages || path != adminmodel.ChannelModelEndpointMessages {
		t.Fatalf("resolveChannelTextUpstream selected messages bridge = (%d, %q), want (%d, %q)", mode, path, relaymode.Messages, adminmodel.ChannelModelEndpointMessages)
	}
}

func TestResolveChannelTextUpstreamOpenAIDirectChatEndpointPreferredForDownstreamChat(t *testing.T) {
	meta := &meta.Meta{
		Mode:           relaymode.ChatCompletions,
		RequestURLPath: adminmodel.ChannelModelEndpointChat,
		ChannelModelConfigs: []adminmodel.ChannelModel{{
			Model:     "gpt-4.1",
			Type:      adminmodel.ProviderModelTypeText,
			Selected:  true,
			Endpoint:  adminmodel.ChannelModelEndpointChat,
			Endpoints: []string{adminmodel.ChannelModelEndpointChat, adminmodel.ChannelModelEndpointResponses},
			SortOrder: 1,
		}},
	}

	mode, path, err := resolveChannelTextUpstream(meta, "gpt-4.1", "gpt-4.1")
	if err != nil {
		t.Fatalf("resolveChannelTextUpstream returned error: %v", err)
	}
	if mode != relaymode.ChatCompletions || path != adminmodel.ChannelModelEndpointChat {
		t.Fatalf("resolveChannelTextUpstream selected chat direct = (%d, %q), want (%d, %q)", mode, path, relaymode.ChatCompletions, adminmodel.ChannelModelEndpointChat)
	}
}

func TestResolveChannelTextUpstreamOpenAIChatOnlyModelDownstreamResponsesFallsBackToChat(t *testing.T) {
	meta := &meta.Meta{
		Mode:           relaymode.Responses,
		RequestURLPath: adminmodel.ChannelModelEndpointResponses,
		APIType:        apitype.OpenAI,
		ChannelModelConfigs: []adminmodel.ChannelModel{{
			Model:    "gpt-4.1",
			Type:     adminmodel.ProviderModelTypeText,
			Selected: true,
			Endpoint: adminmodel.ChannelModelEndpointChat,
		}},
	}

	mode, path, err := resolveChannelTextUpstream(meta, "gpt-4.1", "gpt-4.1")
	if err != nil {
		t.Fatalf("resolveChannelTextUpstream returned error: %v", err)
	}
	if mode != relaymode.ChatCompletions || path != adminmodel.ChannelModelEndpointChat {
		t.Fatalf("resolveChannelTextUpstream selected chat fallback = (%d, %q), want (%d, %q)", mode, path, relaymode.ChatCompletions, adminmodel.ChannelModelEndpointChat)
	}
}

func TestResolveChannelTextUpstreamRejectsWhenRequestedModelNotSelected(t *testing.T) {
	meta := &meta.Meta{
		Mode: relaymode.ChatCompletions,
		ChannelModelConfigs: []adminmodel.ChannelModel{{
			Model:    "gpt-4.1",
			Type:     adminmodel.ProviderModelTypeText,
			Selected: true,
			Endpoint: adminmodel.ChannelModelEndpointResponses,
		}},
	}

	_, _, err := resolveChannelTextUpstream(meta, "unknown", "unknown")
	if err == nil {
		t.Fatalf("resolveChannelTextUpstream returned nil error, want model-not-selected error")
	}
}

func TestResolveChannelTextUpstreamNoModelConfigsOpenAIDefaultsResponses(t *testing.T) {
	meta := &meta.Meta{
		Mode:           relaymode.ChatCompletions,
		RequestURLPath: adminmodel.ChannelModelEndpointChat,
		APIType:        apitype.OpenAI,
	}

	mode, path, err := resolveChannelTextUpstream(meta, "gpt-5.4", "gpt-5.4")
	if err != nil {
		t.Fatalf("resolveChannelTextUpstream returned error: %v", err)
	}
	if mode != relaymode.Responses || path != adminmodel.ChannelModelEndpointResponses {
		t.Fatalf("resolveChannelTextUpstream no-config openai responses = (%d, %q), want (%d, %q)", mode, path, relaymode.Responses, adminmodel.ChannelModelEndpointResponses)
	}
}

func TestResolveChannelTextUpstreamNoModelConfigsAnthropicDefaultsMessages(t *testing.T) {
	meta := &meta.Meta{
		Mode:           relaymode.ChatCompletions,
		RequestURLPath: adminmodel.ChannelModelEndpointChat,
		APIType:        apitype.Anthropic,
	}

	mode, path, err := resolveChannelTextUpstream(meta, "claude-sonnet-4-6", "claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("resolveChannelTextUpstream returned error: %v", err)
	}
	if mode != relaymode.Messages || path != adminmodel.ChannelModelEndpointMessages {
		t.Fatalf("resolveChannelTextUpstream no-config anthropic messages = (%d, %q), want (%d, %q)", mode, path, relaymode.Messages, adminmodel.ChannelModelEndpointMessages)
	}
}

func TestConvertTextRequestForUpstreamToResponses(t *testing.T) {
	req := &relaymodel.GeneralOpenAIRequest{
		Model: "gpt-4.1",
		Messages: []relaymodel.Message{{
			Role:    "user",
			Content: "hello",
		}},
		MaxTokens: 128,
	}

	converted, err := convertTextRequestForUpstream(req, relaymode.ChatCompletions, relaymode.Responses)
	if err != nil {
		t.Fatalf("convertTextRequestForUpstream returned error: %v", err)
	}
	if len(converted.Messages) != 0 {
		t.Fatalf("converted.Messages = %#v, want empty", converted.Messages)
	}
	if converted.Input == nil {
		t.Fatalf("converted.Input = nil, want messages copied into input")
	}
	if converted.MaxOutputTokens == nil || *converted.MaxOutputTokens != 128 {
		t.Fatalf("converted.MaxOutputTokens = %#v, want 128", converted.MaxOutputTokens)
	}
}

func TestConvertTextRequestForUpstreamToResponsesWithImageContent(t *testing.T) {
	req := &relaymodel.GeneralOpenAIRequest{
		Model: "gpt-5.4",
		Messages: []relaymodel.Message{{
			Role: "user",
			Content: []any{
				map[string]any{
					"type": "text",
					"text": "describe this image",
				},
				map[string]any{
					"type": "image_url",
					"image_url": map[string]any{
						"url":    "https://example.com/a.png",
						"detail": "high",
					},
				},
			},
		}},
	}

	converted, err := convertTextRequestForUpstream(req, relaymode.ChatCompletions, relaymode.Responses)
	if err != nil {
		t.Fatalf("convertTextRequestForUpstream returned error: %v", err)
	}
	inputList, ok := converted.Input.([]any)
	if !ok || len(inputList) != 1 {
		t.Fatalf("converted.Input = %#v, want one-item input list", converted.Input)
	}
	first, ok := inputList[0].(map[string]any)
	if !ok {
		t.Fatalf("converted.Input[0] = %#v, want map", inputList[0])
	}
	contentList, ok := first["content"].([]any)
	if !ok || len(contentList) != 2 {
		t.Fatalf("converted.Input[0].content = %#v, want two content blocks", first["content"])
	}
	textPart, ok := contentList[0].(map[string]any)
	if !ok || textPart["type"] != "input_text" || textPart["text"] != "describe this image" {
		t.Fatalf("contentList[0] = %#v, want input_text", contentList[0])
	}
	imagePart, ok := contentList[1].(map[string]any)
	if !ok || imagePart["type"] != "input_image" || imagePart["image_url"] != "https://example.com/a.png" || imagePart["detail"] != "high" {
		t.Fatalf("contentList[1] = %#v, want input_image", contentList[1])
	}
}

func TestConvertTextRequestForUpstreamToResponsesWithImageURLStringContent(t *testing.T) {
	req := &relaymodel.GeneralOpenAIRequest{
		Model: "gpt-5.4",
		Messages: []relaymodel.Message{{
			Role: "user",
			Content: []any{
				map[string]any{
					"type":      "image_url",
					"image_url": "https://example.com/raw.png",
				},
			},
		}},
	}

	converted, err := convertTextRequestForUpstream(req, relaymode.ChatCompletions, relaymode.Responses)
	if err != nil {
		t.Fatalf("convertTextRequestForUpstream returned error: %v", err)
	}
	inputList, ok := converted.Input.([]any)
	if !ok || len(inputList) != 1 {
		t.Fatalf("converted.Input = %#v, want one-item input list", converted.Input)
	}
	first, ok := inputList[0].(map[string]any)
	if !ok {
		t.Fatalf("converted.Input[0] = %#v, want map", inputList[0])
	}
	contentList, ok := first["content"].([]any)
	if !ok || len(contentList) != 1 {
		t.Fatalf("converted.Input[0].content = %#v, want one content block", first["content"])
	}
	imagePart, ok := contentList[0].(map[string]any)
	if !ok || imagePart["type"] != "input_image" || imagePart["image_url"] != "https://example.com/raw.png" {
		t.Fatalf("contentList[0] = %#v, want input_image with image_url", contentList[0])
	}
}

func TestConvertTextRequestForUpstreamToChat(t *testing.T) {
	req := &relaymodel.GeneralOpenAIRequest{
		Model:           "gpt-4.1",
		Input:           "hello",
		MaxOutputTokens: func() *int { value := 256; return &value }(),
	}

	converted, err := convertTextRequestForUpstream(req, relaymode.Responses, relaymode.ChatCompletions)
	if err != nil {
		t.Fatalf("convertTextRequestForUpstream returned error: %v", err)
	}
	if len(converted.Messages) != 1 || converted.Messages[0].StringContent() != "hello" {
		t.Fatalf("converted.Messages = %#v, want single user message", converted.Messages)
	}
	if converted.Input != nil {
		t.Fatalf("converted.Input = %#v, want nil", converted.Input)
	}
	if converted.MaxTokens != 256 {
		t.Fatalf("converted.MaxTokens = %d, want 256", converted.MaxTokens)
	}
}

func TestConvertTextRequestForUpstreamToMessagesFromInput(t *testing.T) {
	req := &relaymodel.GeneralOpenAIRequest{
		Model:           "claude-sonnet-4-6",
		Input:           "hello from responses input",
		MaxOutputTokens: func() *int { value := 320; return &value }(),
	}

	converted, err := convertTextRequestForUpstream(req, relaymode.Responses, relaymode.Messages)
	if err != nil {
		t.Fatalf("convertTextRequestForUpstream returned error: %v", err)
	}
	if converted.Input != nil {
		t.Fatalf("converted.Input = %#v, want nil", converted.Input)
	}
	if len(converted.Messages) != 1 || converted.Messages[0].StringContent() != "hello from responses input" {
		t.Fatalf("converted.Messages = %#v, want single converted user message", converted.Messages)
	}
	if converted.MaxTokens != 320 {
		t.Fatalf("converted.MaxTokens = %d, want 320", converted.MaxTokens)
	}
}

func TestNormalizeResponsesRequestBodyPreservesUnknownFields(t *testing.T) {
	raw := []byte(`{"model":"gpt-5.2-codex","instructions":"keep me","input":"hello","tools":[{"type":"web_search"}]}`)
	normalized, err := normalizeResponsesRequestBody(raw, "gpt-5.4")
	if err != nil {
		t.Fatalf("normalizeResponsesRequestBody returned error: %v", err)
	}

	payload := map[string]any{}
	if err := json.Unmarshal(normalized, &payload); err != nil {
		t.Fatalf("json.Unmarshal normalized body returned error: %v", err)
	}
	if payload["instructions"] != "keep me" {
		t.Fatalf("payload.instructions = %#v, want %q", payload["instructions"], "keep me")
	}
	if payload["model"] != "gpt-5.4" {
		t.Fatalf("payload.model = %#v, want %q", payload["model"], "gpt-5.4")
	}
	input, ok := payload["input"].([]any)
	if !ok || len(input) != 1 {
		t.Fatalf("payload.input = %#v, want single-item array", payload["input"])
	}
	first, ok := input[0].(map[string]any)
	if !ok {
		t.Fatalf("payload.input[0] = %#v, want object", input[0])
	}
	if first["role"] != "user" || first["content"] != "hello" {
		t.Fatalf("payload.input[0] = %#v, want user message", first)
	}
}

func TestNormalizeResponsesRequestBodyConvertsMessagesWithImageContent(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-5.4",
		"messages":[
			{
				"role":"user",
				"content":[
					{"type":"text","text":"what is in this image"},
					{"type":"image_url","image_url":{"url":"https://example.com/img.png","detail":"low"}}
				]
			}
		]
	}`)
	normalized, err := normalizeResponsesRequestBody(raw, "gpt-5.4")
	if err != nil {
		t.Fatalf("normalizeResponsesRequestBody returned error: %v", err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(normalized, &payload); err != nil {
		t.Fatalf("json.Unmarshal normalized body returned error: %v", err)
	}
	if _, exists := payload["messages"]; exists {
		t.Fatalf("payload.messages = %#v, want removed", payload["messages"])
	}
	input, ok := payload["input"].([]any)
	if !ok || len(input) != 1 {
		t.Fatalf("payload.input = %#v, want one-item array", payload["input"])
	}
	first, ok := input[0].(map[string]any)
	if !ok {
		t.Fatalf("payload.input[0] = %#v, want map", input[0])
	}
	content, ok := first["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("payload.input[0].content = %#v, want two blocks", first["content"])
	}
	textPart, ok := content[0].(map[string]any)
	if !ok || textPart["type"] != "input_text" || textPart["text"] != "what is in this image" {
		t.Fatalf("content[0] = %#v, want input_text", content[0])
	}
	imagePart, ok := content[1].(map[string]any)
	if !ok || imagePart["type"] != "input_image" || imagePart["image_url"] != "https://example.com/img.png" || imagePart["detail"] != "low" {
		t.Fatalf("content[1] = %#v, want input_image", content[1])
	}
}

func TestNormalizeMessagesRequestBodyUpdatesModel(t *testing.T) {
	raw := []byte(`{"model":"claude-old","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	normalized, err := normalizeMessagesRequestBody(raw, "claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("normalizeMessagesRequestBody returned error: %v", err)
	}

	payload := map[string]any{}
	if err := json.Unmarshal(normalized, &payload); err != nil {
		t.Fatalf("json.Unmarshal normalized body returned error: %v", err)
	}
	if payload["model"] != "claude-sonnet-4-6" {
		t.Fatalf("payload.model = %#v, want %q", payload["model"], "claude-sonnet-4-6")
	}
	if payload["stream"] != true {
		t.Fatalf("payload.stream = %#v, want true", payload["stream"])
	}
}
