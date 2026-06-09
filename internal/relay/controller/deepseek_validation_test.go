package controller

import (
	"strings"
	"testing"

	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

func TestValidateDeepSeekTextRequestRejectsDeveloperRole(t *testing.T) {
	err := validateProviderSpecificTextRequest(&meta.Meta{
		ChannelProtocol: relaychannel.DeepSeek,
		BaseURL:         "https://api.deepseek.com",
	}, &relaymodel.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro",
		Messages: []relaymodel.Message{
			{Role: "developer", Content: "You are helpful"},
			{Role: "user", Content: "hi"},
		},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "developer role") {
		t.Fatalf("expected developer role error, got %v", err)
	}
}

func TestValidateDeepSeekTextRequestRejectsMaxCompletionTokens(t *testing.T) {
	value := 128
	err := validateProviderSpecificTextRequest(&meta.Meta{
		ChannelProtocol: relaychannel.DeepSeek,
		BaseURL:         "https://api.deepseek.com",
	}, &relaymodel.GeneralOpenAIRequest{
		Model:               "deepseek-v4-pro",
		MaxCompletionTokens: &value,
		Messages: []relaymodel.Message{
			{Role: "user", Content: "hi"},
		},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "max_completion_tokens") {
		t.Fatalf("expected max_completion_tokens error, got %v", err)
	}
}

func TestValidateDeepSeekTextRequestAllowsSamplingParamsWhenThinkingOmitted(t *testing.T) {
	temperature := 0.7
	topP := 0.9
	err := validateProviderSpecificTextRequest(&meta.Meta{
		ChannelProtocol: relaychannel.DeepSeek,
		BaseURL:         "https://api.deepseek.com",
	}, &relaymodel.GeneralOpenAIRequest{
		Model:       "deepseek-v4-pro",
		Temperature: &temperature,
		TopP:        &topP,
		Messages: []relaymodel.Message{
			{Role: "user", Content: "hi"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("expected no error when thinking is omitted, got %v", err)
	}
}

func TestValidateDeepSeekTextRequestRejectsExplicitThinkingModeSamplingParams(t *testing.T) {
	temperature := 0.7
	topP := 0.9
	err := validateProviderSpecificTextRequest(&meta.Meta{
		ChannelProtocol: relaychannel.DeepSeek,
		BaseURL:         "https://api.deepseek.com",
	}, &relaymodel.GeneralOpenAIRequest{
		Model:       "deepseek-v4-pro",
		Thinking:    map[string]any{"type": "enabled"},
		Temperature: &temperature,
		TopP:        &topP,
		Messages: []relaymodel.Message{
			{Role: "user", Content: "hi"},
		},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "thinking 模式不支持参数") {
		t.Fatalf("expected thinking params error, got %v", err)
	}
}

func TestValidateDeepSeekTextRequestAllowsSamplingParamsWhenThinkingDisabled(t *testing.T) {
	temperature := 0.7
	err := validateProviderSpecificTextRequest(&meta.Meta{
		ChannelProtocol: relaychannel.DeepSeek,
		BaseURL:         "https://api.deepseek.com",
	}, &relaymodel.GeneralOpenAIRequest{
		Model:       "deepseek-v4-pro",
		Thinking:    map[string]any{"type": "disabled"},
		Temperature: &temperature,
		Messages: []relaymodel.Message{
			{Role: "user", Content: "hi"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("expected no error when thinking disabled, got %v", err)
	}
}

func TestValidateDeepSeekTextRequestRejectsInvalidStrictToolSchema(t *testing.T) {
	strict := true
	err := validateProviderSpecificTextRequest(&meta.Meta{
		ChannelProtocol: relaychannel.DeepSeek,
		BaseURL:         "https://api.deepseek.com",
	}, &relaymodel.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro",
		Messages: []relaymodel.Message{
			{Role: "user", Content: "hi"},
		},
		Tools: []relaymodel.Tool{
			{
				Type: "function",
				Function: relaymodel.Function{
					Name:   "get_weather",
					Strict: &strict,
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{"type": "string"},
						},
						"required": []any{},
					},
				},
			},
		},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "additionalProperties=false") {
		t.Fatalf("expected strict schema error, got %v", err)
	}
}

func TestValidateDeepSeekTextRequestAllowsValidStrictToolSchema(t *testing.T) {
	strict := true
	err := validateProviderSpecificTextRequest(&meta.Meta{
		ChannelProtocol: relaychannel.DeepSeek,
		BaseURL:         "https://api.deepseek.com",
	}, &relaymodel.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro",
		Messages: []relaymodel.Message{
			{Role: "user", Content: "hi"},
		},
		Tools: []relaymodel.Tool{
			{
				Type: "function",
				Function: relaymodel.Function{
					Name:   "get_weather",
					Strict: &strict,
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{"type": "string"},
						},
						"required":             []any{"location"},
						"additionalProperties": false,
					},
				},
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("expected no error for valid strict schema, got %v", err)
	}
}

func TestValidateDeepSeekTextRequestRejectsV1BaseURL(t *testing.T) {
	err := validateProviderSpecificTextRequest(&meta.Meta{
		ChannelProtocol: relaychannel.DeepSeek,
		BaseURL:         "https://api.deepseek.com/v1",
	}, &relaymodel.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro",
		Messages: []relaymodel.Message{
			{Role: "user", Content: "hi"},
		},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "不能追加 /v1") {
		t.Fatalf("expected base_url error, got %v", err)
	}
}
