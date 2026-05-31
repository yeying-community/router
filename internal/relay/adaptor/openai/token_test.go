package openai

import "testing"

func TestResolveTokenizerEncodingName(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{model: "gpt-4o", want: "o200k_base"},
		{model: "gpt-4o-2024-08-06", want: "o200k_base"},
		{model: "chatgpt-4o-latest", want: "o200k_base"},
		{model: "gpt-4.1", want: "o200k_base"},
		{model: "gpt-4.1-mini", want: "o200k_base"},
		{model: "gpt-4.5", want: "o200k_base"},
		{model: "gpt-4.5-preview", want: "o200k_base"},
		{model: "gpt-5", want: "o200k_base"},
		{model: "gpt-5-mini", want: "o200k_base"},
		{model: "gpt-5.9-codex", want: "o200k_base"},
		{model: "gpt-realtime", want: "o200k_base"},
		{model: "gpt-realtime-mini", want: "o200k_base"},
		{model: "gpt-audio", want: "o200k_base"},
		{model: "gpt-audio-preview", want: "o200k_base"},
		{model: "o1-preview", want: "o200k_base"},
		{model: "o3", want: "o200k_base"},
		{model: "o4-mini", want: "o200k_base"},
		{model: "o5-preview", want: "o200k_base"},
		{model: "gpt-image-2", want: "o200k_base"},
		{model: "gpt-image-1", want: "o200k_base"},
		{model: "chatgpt-image-latest", want: "o200k_base"},
		{model: "gpt-4", want: "cl100k_base"},
		{model: "gpt-4-0613", want: "cl100k_base"},
		{model: "gpt-3.5-turbo", want: "cl100k_base"},
		{model: "text-embedding-3-large", want: "cl100k_base"},
		{model: "gpt-6-experimental", want: "o200k_base"},
		{model: "text-embedding-4-small", want: "cl100k_base"},
		{model: "text-davinci-003", want: "p50k_base"},
		{model: "text-davinci-edit-001", want: "p50k_edit"},
		{model: "text-curie-001", want: "r50k_base"},
		{model: "unknown-model", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := resolveTokenizerEncodingName(tt.model); got != tt.want {
				t.Fatalf("resolveTokenizerEncodingName(%q) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}
}

func TestResolveTokenEncoderForModernOpenAIFamilies(t *testing.T) {
	tests := []string{
		"gpt-image-2",
		"gpt-4.1",
		"gpt-4.5",
		"gpt-5",
		"gpt-realtime-mini",
		"gpt-audio-preview",
		"o1-preview",
		"o3",
		"o4-mini",
		"o5-preview",
		"gpt-6-experimental",
	}

	for _, model := range tests {
		t.Run(model, func(t *testing.T) {
			tokenEncoder, encodingName, err := resolveTokenEncoder(model)
			if err != nil {
				t.Fatalf("resolveTokenEncoder(%q) error = %v", model, err)
			}
			if tokenEncoder == nil {
				t.Fatalf("resolveTokenEncoder(%q) returned nil encoder", model)
			}
			if encodingName != "o200k_base" {
				t.Fatalf("resolveTokenEncoder(%q) encodingName = %q, want %q", model, encodingName, "o200k_base")
			}
		})
	}
}
