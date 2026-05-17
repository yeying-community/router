package anthropic

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/internal/relay/adaptor/openai"
	"github.com/yeying-community/router/internal/relay/model"
)

func copyResponseHeaders(target *gin.Context, source http.Header) {
	for key, values := range source {
		if len(values) == 0 {
			continue
		}
		if shouldSkipUpstreamResponseHeader(key) {
			continue
		}
		for _, value := range values {
			target.Writer.Header().Add(key, value)
		}
	}
}

func shouldSkipUpstreamResponseHeader(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return true
	}
	if strings.HasPrefix(normalized, "access-control-") {
		return true
	}
	switch normalized {
	case "connection", "content-length", "transfer-encoding":
		return true
	default:
		return false
	}
}

func relayMessagesResponse(c *gin.Context, resp *http.Response) (*model.Usage, *model.ErrorWithStatusCode) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, openai.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}

	copyResponseHeaders(c, resp.Header)
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := c.Writer.Write(responseBody); err != nil {
		return nil, openai.ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
	}

	var claudeResponse Response
	if err := json.Unmarshal(responseBody, &claudeResponse); err != nil {
		return nil, openai.ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	usage := &model.Usage{
		PromptTokens:     claudeResponse.Usage.InputTokens,
		CompletionTokens: claudeResponse.Usage.OutputTokens,
		TotalTokens:      claudeResponse.Usage.InputTokens + claudeResponse.Usage.OutputTokens,
	}
	return usage, nil
}

func relayMessagesStreamResponse(c *gin.Context, resp *http.Response) (*model.Usage, *model.ErrorWithStatusCode) {
	copyResponseHeaders(c, resp.Header)
	c.Writer.WriteHeader(resp.StatusCode)

	scanner := newAnthropicStreamScanner(resp.Body)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := strings.IndexByte(string(data), '\n'); i >= 0 {
			return i + 1, data[:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	})

	usage := &model.Usage{}
	for scanner.Scan() {
		line := scanner.Text()
		if _, err := c.Writer.Write([]byte(line + "\n")); err != nil {
			return nil, openai.ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
		}
		c.Writer.Flush()

		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var claudeResponse StreamResponse
		if err := json.Unmarshal([]byte(data), &claudeResponse); err != nil {
			continue
		}
		if claudeResponse.Message != nil {
			if claudeResponse.Message.Usage.InputTokens > usage.PromptTokens {
				usage.PromptTokens = claudeResponse.Message.Usage.InputTokens
			}
			if claudeResponse.Message.Usage.OutputTokens > usage.CompletionTokens {
				usage.CompletionTokens = claudeResponse.Message.Usage.OutputTokens
			}
		}
		if claudeResponse.Usage != nil {
			if claudeResponse.Usage.InputTokens > usage.PromptTokens {
				usage.PromptTokens = claudeResponse.Usage.InputTokens
			}
			if claudeResponse.Usage.OutputTokens > usage.CompletionTokens {
				usage.CompletionTokens = claudeResponse.Usage.OutputTokens
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, openai.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return usage, nil
}
