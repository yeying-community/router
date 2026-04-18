package anthropic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/helper"
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
	applyClaudeUsageTotals(usage, &claudeResponse.Usage)
	return usage, nil
}

func calcClaudeTotalTokens(u *Usage) (inputTotal int, outputTotal int, total int) {
	if u == nil {
		return 0, 0, 0
	}
	if len(u.Iterations) > 0 {
		for _, it := range u.Iterations {
			inputTotal += it.InputTokens
			inputTotal += it.CacheReadInputTokens
			inputTotal += it.CacheCreationInputTokens
			outputTotal += it.OutputTokens
		}
		total = inputTotal + outputTotal
		return inputTotal, outputTotal, total
	}

	inputTotal += u.InputTokens
	inputTotal += u.CacheReadInputTokens
	inputTotal += u.CacheCreationInputTokens
	outputTotal += u.OutputTokens
	total = inputTotal + outputTotal
	return inputTotal, outputTotal, total
}

func applyUsageSnapshot(usage *model.Usage, claudeUsage *Usage) {
	if usage == nil || claudeUsage == nil {
		return
	}
	if claudeUsage.InputTokens > 0 {
		usage.PromptTokens = claudeUsage.InputTokens
	}
	if claudeUsage.OutputTokens > 0 {
		usage.CompletionTokens = claudeUsage.OutputTokens
	}
}

func applyClaudeUsageTotals(usage *model.Usage, claudeUsage *Usage) {
	if usage == nil || claudeUsage == nil {
		return
	}
	inputTotal, outputTotal, total := calcClaudeTotalTokens(claudeUsage)
	usage.PromptTokens = inputTotal
	usage.CompletionTokens = outputTotal
	usage.TotalTokens = total
}

func mergeClaudeUsageSnapshot(dst *Usage, src *Usage) *Usage {
	if src == nil {
		return dst
	}
	if dst == nil {
		copy := *src
		return &copy
	}
	if src.InputTokens > 0 {
		dst.InputTokens = src.InputTokens
	}
	if src.OutputTokens > 0 {
		dst.OutputTokens = src.OutputTokens
	}
	if src.CacheCreationInputTokens > 0 {
		dst.CacheCreationInputTokens = src.CacheCreationInputTokens
	}
	if src.CacheReadInputTokens > 0 {
		dst.CacheReadInputTokens = src.CacheReadInputTokens
	}
	if len(src.Iterations) > 0 {
		dst.Iterations = append([]UsageIteration(nil), src.Iterations...)
	}
	return dst
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
	var latestUsage *Usage
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
			applyUsageSnapshot(usage, &claudeResponse.Message.Usage)
			latestUsage = mergeClaudeUsageSnapshot(latestUsage, &claudeResponse.Message.Usage)
		}
		if claudeResponse.Usage != nil {
			applyUsageSnapshot(usage, claudeResponse.Usage)
			latestUsage = mergeClaudeUsageSnapshot(latestUsage, claudeResponse.Usage)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, openai.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}
	applyClaudeUsageTotals(usage, latestUsage)
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return usage, nil
}

func relayMessagesStreamAsChatResponse(c *gin.Context, resp *http.Response, promptTokens int, modelName string) (*model.Usage, *model.ErrorWithStatusCode) {
	if resp == nil {
		return nil, openai.ErrorWrapper(fmt.Errorf("resp is nil"), "nil_response", http.StatusInternalServerError)
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, openai.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}

	trimmedBody := strings.TrimSpace(string(responseBody))
	if !strings.HasPrefix(trimmedBody, "event:") && !strings.HasPrefix(trimmedBody, "data:") {
		fallbackResp := &http.Response{
			StatusCode: resp.StatusCode,
			Header:     resp.Header.Clone(),
			Body:       io.NopCloser(bytes.NewBuffer(responseBody)),
		}
		respErr, fallbackUsage := Handler(c, fallbackResp, promptTokens, modelName)
		return fallbackUsage, respErr
	}

	scanner := newAnthropicStreamScanner(bytes.NewReader(responseBody))
	scanner.Split(bufio.ScanLines)
	currentEvent := ""
	var responseTextBuilder strings.Builder
	responseID := ""
	responseModel := strings.TrimSpace(modelName)
	finishReason := "stop"
	var latestUsage *Usage
	usage := &model.Usage{
		PromptTokens: promptTokens,
	}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			currentEvent = ""
			continue
		}
		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			_ = currentEvent
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var payload StreamResponse
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			continue
		}

		switch payload.Type {
		case "message_start":
			if payload.Message != nil {
				if strings.TrimSpace(payload.Message.Id) != "" {
					responseID = strings.TrimSpace(payload.Message.Id)
				}
				if strings.TrimSpace(payload.Message.Model) != "" {
					responseModel = strings.TrimSpace(payload.Message.Model)
				}
				if payload.Message.Usage.InputTokens > 0 {
					usage.PromptTokens = payload.Message.Usage.InputTokens
				}
				if payload.Message.Usage.OutputTokens > 0 {
					usage.CompletionTokens = payload.Message.Usage.OutputTokens
				}
				latestUsage = mergeClaudeUsageSnapshot(latestUsage, &payload.Message.Usage)
			}
		case "content_block_start":
			if payload.ContentBlock != nil && payload.ContentBlock.Type == "text" && strings.TrimSpace(payload.ContentBlock.Text) != "" {
				responseTextBuilder.WriteString(payload.ContentBlock.Text)
			}
		case "content_block_delta":
			if payload.Delta != nil && strings.TrimSpace(payload.Delta.Text) != "" {
				responseTextBuilder.WriteString(payload.Delta.Text)
			}
		case "message_delta":
			if payload.Usage != nil {
				if payload.Usage.InputTokens > 0 {
					usage.PromptTokens = payload.Usage.InputTokens
				}
				if payload.Usage.OutputTokens > 0 {
					usage.CompletionTokens = payload.Usage.OutputTokens
				}
				latestUsage = mergeClaudeUsageSnapshot(latestUsage, payload.Usage)
			}
			if payload.Delta != nil && payload.Delta.StopReason != nil {
				reason := stopReasonClaude2OpenAI(payload.Delta.StopReason)
				if strings.TrimSpace(reason) != "" {
					finishReason = reason
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}

	responseText := strings.TrimSpace(responseTextBuilder.String())
	if usage.CompletionTokens == 0 && responseText != "" {
		usage.CompletionTokens = openai.CountTokenText(responseText, modelName)
	}
	if usage.PromptTokens == 0 {
		usage.PromptTokens = promptTokens
	}
	applyClaudeUsageTotals(usage, latestUsage)
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	response := openai.TextResponse{
		Id:      fmt.Sprintf("chatcmpl-%s", responseID),
		Model:   responseModel,
		Object:  "chat.completion",
		Created: helper.GetTimestamp(),
		Choices: []openai.TextResponseChoice{{
			Index: 0,
			Message: model.Message{
				Role:    "assistant",
				Content: responseText,
			},
			FinishReason: finishReason,
		}},
		Usage: *usage,
	}
	if strings.TrimSpace(responseID) == "" {
		response.Id = fmt.Sprintf("chatcmpl-%d", helper.GetTimestamp())
	}
	if strings.TrimSpace(response.Model) == "" {
		response.Model = modelName
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		return nil, openai.ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
	}
	for key, values := range resp.Header {
		if len(values) == 0 {
			continue
		}
		if strings.EqualFold(key, "Content-Type") || shouldSkipUpstreamResponseHeader(key) {
			continue
		}
		c.Writer.Header().Set(key, values[0])
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := c.Writer.Write(encoded); err != nil {
		return nil, openai.ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
	}
	return usage, nil
}
