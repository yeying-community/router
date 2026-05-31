package tokenestimate

import (
	"testing"

	openaiadaptor "github.com/yeying-community/router/internal/relay/adaptor/openai"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

func TestEstimateOpenAIChat(t *testing.T) {
	openaiadaptor.InitTokenEncoders()
	req := EstimateRequest{
		RelayMode: relaymode.ChatCompletions,
		Model:     "gpt-4o",
		Request: &relaymodel.GeneralOpenAIRequest{
			Model: "gpt-4o",
			Messages: []relaymodel.Message{
				{Role: "user", Content: "hello world"},
			},
		},
	}
	got, err := Estimate(req)
	if err != nil {
		t.Fatalf("Estimate returned error: %v", err)
	}
	if got.Estimator != "openai_exact" || got.Precision != PrecisionExact {
		t.Fatalf("unexpected result metadata: %+v", got)
	}
	if got.PromptTokens <= 0 {
		t.Fatalf("PromptTokens = %d, want > 0", got.PromptTokens)
	}
}

func TestEstimateAnthropicMessages(t *testing.T) {
	req := EstimateRequest{
		RelayMode: relaymode.Messages,
		Model:     "claude-sonnet-4-6",
		RawBody: []byte(`{
			"model":"claude-sonnet-4-6",
			"system":"You are helpful.",
			"messages":[
				{
					"role":"user",
					"content":[
						{"type":"text","text":"hello world"}
					]
				}
			]
		}`),
	}
	got, err := Estimate(req)
	if err != nil {
		t.Fatalf("Estimate returned error: %v", err)
	}
	if got.Estimator != "anthropic_heuristic" || got.Precision != PrecisionHeuristic {
		t.Fatalf("unexpected result metadata: %+v", got)
	}
	if got.PromptTokens <= 0 {
		t.Fatalf("PromptTokens = %d, want > 0", got.PromptTokens)
	}
}

func TestEstimateResponsesWithTools(t *testing.T) {
	openaiadaptor.InitTokenEncoders()
	req := EstimateRequest{
		RelayMode: relaymode.Responses,
		Model:     "gpt-4o",
		Request: &relaymodel.GeneralOpenAIRequest{
			Model: "gpt-4o",
			Input: "search docs",
			Tools: []relaymodel.Tool{
				{
					Type: "function",
					Function: relaymodel.Function{
						Name:        "search",
						Description: "search docs",
						Parameters:  map[string]any{"type": "object", "properties": map[string]any{"q": map[string]any{"type": "string"}}},
					},
				},
			},
		},
	}
	got, err := Estimate(req)
	if err != nil {
		t.Fatalf("Estimate returned error: %v", err)
	}
	if got.PromptTokens <= 0 {
		t.Fatalf("PromptTokens = %d, want > 0", got.PromptTokens)
	}
}

func TestEstimateUnknownFallback(t *testing.T) {
	req := EstimateRequest{
		RelayMode: relaymode.ChatCompletions,
		Model:     "custom-model",
		Request: &relaymodel.GeneralOpenAIRequest{
			Model: "custom-model",
			Messages: []relaymodel.Message{
				{Role: "user", Content: "hello world"},
			},
		},
	}
	got, err := Estimate(req)
	if err != nil {
		t.Fatalf("Estimate returned error: %v", err)
	}
	if got.Estimator != "unknown_heuristic" {
		t.Fatalf("Estimator = %q, want unknown_heuristic", got.Estimator)
	}
}

func TestEstimateGeminiRawRequest(t *testing.T) {
	req := EstimateRequest{
		RelayMode: relaymode.ChatCompletions,
		Model:     "gemini-2.0-flash",
		RawBody: []byte(`{
			"contents": [
				{"role": "user", "parts": [{"text": "hello gemini"}]}
			]
		}`),
	}
	got, err := Estimate(req)
	if err != nil {
		t.Fatalf("Estimate returned error: %v", err)
	}
	if got.Estimator != "gemini_heuristic" || got.Precision != PrecisionHeuristic {
		t.Fatalf("unexpected result metadata: %+v", got)
	}
	if got.PromptTokens <= 0 {
		t.Fatalf("PromptTokens = %d, want > 0", got.PromptTokens)
	}
}

func TestEstimateAnthropicRequiresRawBody(t *testing.T) {
	req := EstimateRequest{
		RelayMode: relaymode.Messages,
		Model:     "claude-sonnet-4-6",
		Request: &relaymodel.GeneralOpenAIRequest{
			Model: "claude-sonnet-4-6",
			Messages: []relaymodel.Message{
				{Role: "user", Content: "hello world"},
			},
		},
	}
	_, err := Estimate(req)
	if err == nil || err.Error() != "anthropic estimate raw request body is empty" {
		t.Fatalf("Estimate error = %v, want anthropic raw body error", err)
	}
}

func TestResolveEstimateModelFallsBackToStructuredRequest(t *testing.T) {
	req := EstimateRequest{
		Request: &relaymodel.GeneralOpenAIRequest{
			Model: "gpt-4o",
		},
	}
	if got := resolveEstimateModel(req); got != "gpt-4o" {
		t.Fatalf("resolveEstimateModel() = %q, want gpt-4o", got)
	}
}

func TestDetectFamilyModernOpenAIModels(t *testing.T) {
	tests := []string{
		"gpt-5.9-codex",
		"gpt-realtime-mini",
		"gpt-audio-preview",
		"gpt-image-1",
		"chatgpt-image-latest",
		"o5-preview",
	}
	for _, model := range tests {
		t.Run(model, func(t *testing.T) {
			if got := detectFamily(model); got != familyOpenAI {
				t.Fatalf("detectFamily(%q) = %q, want %q", model, got, familyOpenAI)
			}
		})
	}
}
