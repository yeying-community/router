package doubao

import (
	"testing"

	"github.com/yeying-community/router/internal/relay/meta"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

func TestGetRequestURLSupportsOfficialVolcengineEndpoints(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		mode    int
		want    string
	}{
		{
			name:    "chat with root base url",
			baseURL: "https://ark.cn-beijing.volces.com",
			mode:    relaymode.ChatCompletions,
			want:    "https://ark.cn-beijing.volces.com/api/v3/chat/completions",
		},
		{
			name:    "chat with api v3 base url",
			baseURL: "https://ark.cn-beijing.volces.com/api/v3",
			mode:    relaymode.ChatCompletions,
			want:    "https://ark.cn-beijing.volces.com/api/v3/chat/completions",
		},
		{
			name:    "responses",
			baseURL: "https://ark.cn-beijing.volces.com",
			mode:    relaymode.Responses,
			want:    "https://ark.cn-beijing.volces.com/api/v3/responses",
		},
		{
			name:    "embeddings",
			baseURL: "https://ark.cn-beijing.volces.com",
			mode:    relaymode.Embeddings,
			want:    "https://ark.cn-beijing.volces.com/api/v3/embeddings",
		},
		{
			name:    "image generation",
			baseURL: "https://ark.cn-beijing.volces.com",
			mode:    relaymode.ImagesGenerations,
			want:    "https://ark.cn-beijing.volces.com/api/v3/images/generations",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestMeta := meta.Meta{BaseURL: tt.baseURL, Mode: tt.mode}
			got, err := GetRequestURL(&requestMeta)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("GetRequestURL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetRequestURLRejectsMessages(t *testing.T) {
	_, err := GetRequestURL(&meta.Meta{
		BaseURL: "https://ark.cn-beijing.volces.com",
		Mode:    relaymode.Messages,
	})
	if err == nil {
		t.Fatal("GetRequestURL messages returned nil error, want unsupported mode error")
	}
}
