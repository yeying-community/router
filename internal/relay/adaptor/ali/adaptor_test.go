package ali

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	openaiadaptor "github.com/yeying-community/router/internal/relay/adaptor/openai"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
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
	want := "https://dashscope.aliyuncs.com/compatible-mode/v1/responses"
	if got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}

func TestResponseAli2OpenAIRetainsActualModelName(t *testing.T) {
	resp := responseAli2OpenAI(&ChatResponse{
		Error: Error{RequestId: "req_123"},
		Output: Output{
			Choices: []openaiadaptor.TextResponseChoice{
				{
					Message: relaymodel.Message{Content: "ok"},
				},
			},
		},
		Usage: Usage{InputTokens: 3, OutputTokens: 5},
	}, "qwen-plus-latest")

	if resp.Model != "qwen-plus-latest" {
		t.Fatalf("responseAli2OpenAI().Model = %q, want %q", resp.Model, "qwen-plus-latest")
	}
}

func TestStreamResponseAli2OpenAIRetainsActualModelName(t *testing.T) {
	resp := streamResponseAli2OpenAI(&ChatResponse{
		Error: Error{RequestId: "req_123"},
		Output: Output{
			Choices: []openaiadaptor.TextResponseChoice{
				{
					Message:      relaymodel.Message{Content: "delta"},
					FinishReason: "stop",
				},
			},
		},
	}, "qwen-max-latest")

	if resp == nil {
		t.Fatal("streamResponseAli2OpenAI() returned nil")
	}
	if resp.Model != "qwen-max-latest" {
		t.Fatalf("streamResponseAli2OpenAI().Model = %q, want %q", resp.Model, "qwen-max-latest")
	}
}

func TestHandlerWritesActualModelName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	body := []byte(`{"request_id":"req_123","output":{"choices":[{"message":{"content":"ok"}}]},"usage":{"input_tokens":2,"output_tokens":4}}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(body)),
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "application/json")

	if err, _ := Handler(ctx, resp, "qwen-turbo-latest"); err != nil {
		t.Fatalf("Handler() error = %+v", err)
	}

	var payload openaiadaptor.TextResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal handler response failed: %v", err)
	}
	if payload.Model != "qwen-turbo-latest" {
		t.Fatalf("handler payload model = %q, want %q", payload.Model, "qwen-turbo-latest")
	}
}
