package doubao

import (
	"fmt"
	"strings"

	"github.com/yeying-community/router/internal/relay/meta"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

func GetRequestURL(meta *meta.Meta) (string, error) {
	baseURL := normalizeArkBaseURL(meta.BaseURL)
	switch meta.Mode {
	case relaymode.ChatCompletions:
		return fmt.Sprintf("%s/api/v3/chat/completions", baseURL), nil
	case relaymode.Responses:
		return fmt.Sprintf("%s/api/v3/responses", baseURL), nil
	case relaymode.Embeddings:
		return fmt.Sprintf("%s/api/v3/embeddings", baseURL), nil
	case relaymode.ImagesGenerations:
		return fmt.Sprintf("%s/api/v3/images/generations", baseURL), nil
	default:
	}
	return "", fmt.Errorf("unsupported relay mode %d for doubao", meta.Mode)
}

func normalizeArkBaseURL(raw string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(raw), "/")
	lower := strings.ToLower(baseURL)
	if strings.HasSuffix(lower, "/api/v3") {
		return baseURL[:len(baseURL)-len("/api/v3")]
	}
	return baseURL
}
