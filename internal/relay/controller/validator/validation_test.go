package validator

import (
	"strings"
	"testing"

	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

func TestValidateTextRequestRejectsInvalidMaxCompletionTokens(t *testing.T) {
	value := -1
	err := ValidateTextRequest(&relaymodel.GeneralOpenAIRequest{
		Model:               "gpt-test",
		Messages:            []relaymodel.Message{{Role: "user", Content: "hello"}},
		MaxCompletionTokens: &value,
	}, relaymode.ChatCompletions)
	if err == nil || !strings.Contains(err.Error(), "max_completion_tokens") {
		t.Fatalf("expected max_completion_tokens error, got %v", err)
	}
}

func TestValidateTextRequestRejectsInvalidMaxOutputTokens(t *testing.T) {
	value := -1
	err := ValidateTextRequest(&relaymodel.GeneralOpenAIRequest{
		Model:           "gpt-test",
		Input:           "hello",
		MaxOutputTokens: &value,
	}, relaymode.Responses)
	if err == nil || !strings.Contains(err.Error(), "max_output_tokens") {
		t.Fatalf("expected max_output_tokens error, got %v", err)
	}
}
