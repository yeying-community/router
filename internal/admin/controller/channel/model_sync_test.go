package channel

import (
	"testing"

	adminmodel "github.com/yeying-community/router/internal/admin/model"
)

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

func TestResolveModelsURL_DeepSeekRootBaseURLUsesModelsEndpoint(t *testing.T) {
	got := resolveModelsURL("https://api.deepseek.com", "deepseek")
	want := "https://api.deepseek.com/models"
	if got != want {
		t.Fatalf("resolveModelsURL() = %q, want %q", got, want)
	}
}

func TestZhipuModelSyncUsesProviderOfficialModels(t *testing.T) {
	if !usesProviderOfficialModelsForSync("zhipu") {
		t.Fatalf("zhipu model sync should use provider official models")
	}
}

func TestDoubaoModelSyncUsesProviderOfficialModels(t *testing.T) {
	if !usesProviderOfficialModelsForSync("doubao") {
		t.Fatalf("doubao model sync should use provider official models")
	}
}

func TestProviderOfficialUpstreamModelKeepsVolcengineOfficialModel(t *testing.T) {
	got := providerOfficialUpstreamModel("volcengine", "doubao-seed-2-0-pro-260215")
	want := "doubao-seed-2-0-pro-260215"
	if got != want {
		t.Fatalf("providerOfficialUpstreamModel() = %q, want %q", got, want)
	}
}

func TestProviderOfficialUpstreamModelKeepsNonVolcengineModel(t *testing.T) {
	got := providerOfficialUpstreamModel("zhipu", "glm-4.7")
	if got != "glm-4.7" {
		t.Fatalf("providerOfficialUpstreamModel() = %q, want glm-4.7", got)
	}
}

func TestOpenAIModelSyncUsesUpstreamModelsEndpoint(t *testing.T) {
	if usesProviderOfficialModelsForSync("openai") {
		t.Fatalf("openai model sync should use upstream models endpoint")
	}
}

func TestNormalizeChannelModelTypeHintRecognizesEmbedding(t *testing.T) {
	if got := normalizeChannelModelTypeHint("text embedding model"); got != adminmodel.ProviderModelTypeEmbedding {
		t.Fatalf("normalizeChannelModelTypeHint() = %q, want embedding", got)
	}
}
