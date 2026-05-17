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

func TestResolveChannelTextUpstreamOpenAIResponsesModelDownstreamChatRejected(t *testing.T) {
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

	_, _, err := resolveChannelTextUpstream(meta, "gpt-4.1", "gpt-4.1")
	if err == nil {
		t.Fatalf("resolveChannelTextUpstream returned nil error, want endpoint-not-supported error")
	}
}

func TestResolveChannelTextUpstreamOpenAIResponsesWithoutEndpointTruthRejected(t *testing.T) {
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

	_, _, err := resolveChannelTextUpstream(meta, "gpt-4.1", "gpt-4.1")
	if err == nil {
		t.Fatalf("resolveChannelTextUpstream returned nil error, want endpoint truth required error")
	}
}

func TestResolveChannelTextUpstreamAnthropicMessagesWithoutEndpointTruthRejected(t *testing.T) {
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

	_, _, err := resolveChannelTextUpstream(meta, "claude-sonnet-4-6", "claude-sonnet-4-6")
	if err == nil {
		t.Fatalf("resolveChannelTextUpstream returned nil error, want endpoint truth required error")
	}
}

func TestResolveChannelTextUpstreamAnthropicMessagesModelDownstreamChatRejected(t *testing.T) {
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

	_, _, err := resolveChannelTextUpstream(meta, "claude-sonnet-4-6", "claude-sonnet-4-6")
	if err == nil {
		t.Fatalf("resolveChannelTextUpstream returned nil error, want endpoint-not-supported error")
	}
}

func TestResolveChannelTextUpstreamOpenAIDirectChatWithoutEndpointTruthRejected(t *testing.T) {
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

	_, _, err := resolveChannelTextUpstream(meta, "gpt-4.1", "gpt-4.1")
	if err == nil {
		t.Fatalf("resolveChannelTextUpstream returned nil error, want endpoint truth required error")
	}
}

func TestResolveChannelTextUpstreamOpenAIChatOnlyModelDownstreamResponsesRejected(t *testing.T) {
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

	_, _, err := resolveChannelTextUpstream(meta, "gpt-4.1", "gpt-4.1")
	if err == nil {
		t.Fatalf("resolveChannelTextUpstream returned nil error, want endpoint-not-supported error")
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

func TestResolveChannelTextUpstreamNoModelConfigsOpenAIDownstreamChatRejected(t *testing.T) {
	meta := &meta.Meta{
		Mode:           relaymode.ChatCompletions,
		RequestURLPath: adminmodel.ChannelModelEndpointChat,
		APIType:        apitype.OpenAI,
	}

	_, _, err := resolveChannelTextUpstream(meta, "gpt-5.4", "gpt-5.4")
	if err == nil {
		t.Fatalf("resolveChannelTextUpstream returned nil error, want endpoint config required error")
	}
}

func TestResolveChannelTextUpstreamNoModelConfigsAnthropicDownstreamMessagesRejected(t *testing.T) {
	meta := &meta.Meta{
		Mode:           relaymode.Messages,
		RequestURLPath: adminmodel.ChannelModelEndpointMessages,
		APIType:        apitype.Anthropic,
	}

	_, _, err := resolveChannelTextUpstream(meta, "claude-sonnet-4-6", "claude-sonnet-4-6")
	if err == nil {
		t.Fatalf("resolveChannelTextUpstream returned nil error, want endpoint config required error")
	}
}

func TestResolveChannelTextUpstreamNoModelConfigsAnthropicDownstreamChatRejected(t *testing.T) {
	meta := &meta.Meta{
		Mode:           relaymode.ChatCompletions,
		RequestURLPath: adminmodel.ChannelModelEndpointChat,
		APIType:        apitype.Anthropic,
	}

	_, _, err := resolveChannelTextUpstream(meta, "claude-sonnet-4-6", "claude-sonnet-4-6")
	if err == nil {
		t.Fatalf("resolveChannelTextUpstream returned nil error, want unsupported chat error")
	}
}

func TestResolveChannelTextUpstreamNoModelConfigsOpenAIMessagesRejected(t *testing.T) {
	meta := &meta.Meta{
		Mode:           relaymode.Messages,
		RequestURLPath: adminmodel.ChannelModelEndpointMessages,
		APIType:        apitype.OpenAI,
	}

	_, _, err := resolveChannelTextUpstream(meta, "gpt-5.4", "gpt-5.4")
	if err == nil {
		t.Fatalf("resolveChannelTextUpstream returned nil error, want unsupported messages error")
	}
}

func TestConvertTextRequestForUpstreamRejectsChatToResponses(t *testing.T) {
	req := &relaymodel.GeneralOpenAIRequest{
		Model: "gpt-4.1",
		Messages: []relaymodel.Message{{
			Role:    "user",
			Content: "hello",
		}},
		MaxTokens: 128,
	}

	if _, err := convertTextRequestForUpstream(req, relaymode.ChatCompletions, relaymode.Responses); err == nil {
		t.Fatalf("convertTextRequestForUpstream returned nil error, want endpoint-conversion error")
	}
}

func TestConvertTextRequestForUpstreamRejectsResponsesToChat(t *testing.T) {
	req := &relaymodel.GeneralOpenAIRequest{
		Model:           "gpt-4.1",
		Input:           "hello",
		MaxOutputTokens: func() *int { value := 256; return &value }(),
	}

	if _, err := convertTextRequestForUpstream(req, relaymode.Responses, relaymode.ChatCompletions); err == nil {
		t.Fatalf("convertTextRequestForUpstream returned nil error, want endpoint-conversion error")
	}
}

func TestConvertTextRequestForUpstreamRejectsResponsesToMessages(t *testing.T) {
	req := &relaymodel.GeneralOpenAIRequest{
		Model:           "claude-sonnet-4-6",
		Input:           "hello from responses input",
		Instructions:    "reply in haiku form",
		MaxOutputTokens: func() *int { value := 320; return &value }(),
	}

	if _, err := convertTextRequestForUpstream(req, relaymode.Responses, relaymode.Messages); err == nil {
		t.Fatalf("convertTextRequestForUpstream returned nil error, want endpoint-conversion error")
	}
}

func TestConvertTextRequestForUpstreamSameModePassesThrough(t *testing.T) {
	req := &relaymodel.GeneralOpenAIRequest{
		Model: "gpt-4.1",
		Messages: []relaymodel.Message{{
			Role:    "user",
			Content: "hello",
		}},
		MaxTokens: 128,
	}

	converted, err := convertTextRequestForUpstream(req, relaymode.ChatCompletions, relaymode.ChatCompletions)
	if err != nil {
		t.Fatalf("convertTextRequestForUpstream returned error: %v", err)
	}
	if len(converted.Messages) != 1 || converted.Messages[0].StringContent() != "hello" {
		t.Fatalf("converted.Messages = %#v, want original chat message", converted.Messages)
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
