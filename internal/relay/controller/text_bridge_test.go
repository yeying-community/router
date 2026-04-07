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

func TestResolveChannelTextUpstreamPrefersSelectedModelEndpoint(t *testing.T) {
	meta := &meta.Meta{
		Mode: relaymode.ChatCompletions,
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

func TestResolveChannelTextUpstreamPrefersSelectedMessagesEndpoint(t *testing.T) {
	meta := &meta.Meta{
		Mode: relaymode.Messages,
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

func TestResolveChannelTextUpstreamFallsBackToSelectedModels(t *testing.T) {
	meta := &meta.Meta{
		Mode: relaymode.ChatCompletions,
		ChannelModelConfigs: []adminmodel.ChannelModel{{
			Model:    "gpt-4.1",
			Type:     adminmodel.ProviderModelTypeText,
			Selected: true,
			Endpoint: adminmodel.ChannelModelEndpointResponses,
		}},
	}

	mode, path, err := resolveChannelTextUpstream(meta, "unknown", "unknown")
	if err != nil {
		t.Fatalf("resolveChannelTextUpstream returned error: %v", err)
	}
	if mode != relaymode.Responses || path != adminmodel.ChannelModelEndpointResponses {
		t.Fatalf("resolveChannelTextUpstream selected-model fallback = (%d, %q), want (%d, %q)", mode, path, relaymode.Responses, adminmodel.ChannelModelEndpointResponses)
	}
}

func TestResolveChannelTextUpstreamRejectsResponsesWhenChannelOnlySupportsChat(t *testing.T) {
	meta := &meta.Meta{
		Mode: relaymode.Responses,
		ChannelModelConfigs: []adminmodel.ChannelModel{{
			Model:    "gpt-4.1",
			Type:     adminmodel.ProviderModelTypeText,
			Selected: true,
			Endpoint: adminmodel.ChannelModelEndpointChat,
		}},
	}

	_, _, err := resolveChannelTextUpstream(meta, "gpt-4.1", "gpt-4.1")
	if err == nil {
		t.Fatalf("resolveChannelTextUpstream returned nil error, want unsupported responses endpoint")
	}
}

func TestResolveChannelTextUpstreamAnthropicForcesMessagesUpstream(t *testing.T) {
	meta := &meta.Meta{
		Mode:    relaymode.Messages,
		APIType: apitype.Anthropic,
		ChannelModelConfigs: []adminmodel.ChannelModel{{
			Model:    "claude-sonnet-4-6",
			Type:     adminmodel.ProviderModelTypeText,
			Selected: true,
			Endpoint: adminmodel.ChannelModelEndpointResponses,
		}},
	}

	mode, path, err := resolveChannelTextUpstream(meta, "claude-sonnet-4-6", "claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("resolveChannelTextUpstream returned error: %v", err)
	}
	if mode != relaymode.Messages || path != adminmodel.ChannelModelEndpointMessages {
		t.Fatalf("resolveChannelTextUpstream anthropic selected = (%d, %q), want (%d, %q)", mode, path, relaymode.Messages, adminmodel.ChannelModelEndpointMessages)
	}
}

func TestResolveChannelTextUpstreamAnthropicRejectsResponsesMode(t *testing.T) {
	meta := &meta.Meta{
		Mode:    relaymode.Responses,
		APIType: apitype.AwsClaude,
	}

	_, _, err := resolveChannelTextUpstream(meta, "claude-sonnet-4-6", "claude-sonnet-4-6")
	if err == nil {
		t.Fatalf("resolveChannelTextUpstream returned nil error for anthropic responses mode")
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

func TestNormalizeResponsesRequestBodyPreservesUnknownFields(t *testing.T) {
	raw := []byte(`{"model":"gpt-5.2-codex","instructions":"keep me","input":"hello","tools":[{"type":"web_search"}]}`)
	normalized, err := normalizeResponsesRequestBody(raw)
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
