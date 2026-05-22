package channel

import "testing"

func TestResolveModelsURL_AliRootBaseURLUsesCompatibleModelsEndpoint(t *testing.T) {
	got := resolveModelsURL("https://dashscope.aliyuncs.com", "ali")
	want := "https://dashscope.aliyuncs.com/compatible-mode/v1/models"
	if got != want {
		t.Fatalf("resolveModelsURL() = %q, want %q", got, want)
	}
}

func TestResolveModelsURL_AliCompatibleBaseURLKeepsModelsEndpoint(t *testing.T) {
	got := resolveModelsURL("https://dashscope.aliyuncs.com/compatible-mode/v1", "ali")
	want := "https://dashscope.aliyuncs.com/compatible-mode/v1/models"
	if got != want {
		t.Fatalf("resolveModelsURL() = %q, want %q", got, want)
	}
}

func TestResolveModelsURL_OpenAIRootBaseURLUsesV1ModelsEndpoint(t *testing.T) {
	got := resolveModelsURL("https://api.openai.com", "openai")
	want := "https://api.openai.com/v1/models"
	if got != want {
		t.Fatalf("resolveModelsURL() = %q, want %q", got, want)
	}
}
