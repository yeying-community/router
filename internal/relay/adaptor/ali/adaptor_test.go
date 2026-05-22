package ali

import (
	"testing"

	"github.com/yeying-community/router/internal/relay/meta"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

func TestGetRequestURL_ChatUsesCompatibleMode(t *testing.T) {
	adaptor := &Adaptor{}
	got, err := adaptor.GetRequestURL(&meta.Meta{
		Mode:    relaymode.ChatCompletions,
		BaseURL: "https://dashscope.aliyuncs.com",
	})
	if err != nil {
		t.Fatalf("GetRequestURL() error = %v", err)
	}
	want := "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"
	if got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}

func TestGetRequestURL_ResponsesUsesAliCompatibleResponsesPath(t *testing.T) {
	adaptor := &Adaptor{}
	got, err := adaptor.GetRequestURL(&meta.Meta{
		Mode:    relaymode.Responses,
		BaseURL: "https://dashscope.aliyuncs.com",
	})
	if err != nil {
		t.Fatalf("GetRequestURL() error = %v", err)
	}
	want := "https://dashscope.aliyuncs.com/api/v2/apps/protocols/compatible-mode/v1/responses"
	if got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}
