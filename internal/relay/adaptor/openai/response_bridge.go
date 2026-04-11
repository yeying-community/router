package openai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/render"
	"github.com/yeying-community/router/internal/relay/model"
)

type responsesOutputContent struct {
	Type       string `json:"type,omitempty"`
	Text       string `json:"text,omitempty"`
	OutputText string `json:"output_text,omitempty"`
}

type responsesOutputItem struct {
	Type       string                   `json:"type,omitempty"`
	Role       string                   `json:"role,omitempty"`
	Text       string                   `json:"text,omitempty"`
	OutputText string                   `json:"output_text,omitempty"`
	Content    []responsesOutputContent `json:"content,omitempty"`
}

type responsesBridgeEnvelope struct {
	ID         string                `json:"id,omitempty"`
	Object     string                `json:"object,omitempty"`
	Model      string                `json:"model,omitempty"`
	CreatedAt  int64                 `json:"created_at,omitempty"`
	OutputText string                `json:"output_text,omitempty"`
	Output     []responsesOutputItem `json:"output,omitempty"`
	Usage      *responsesUsage       `json:"usage,omitempty"`
}

func responseUsageToOpenAI(usage *responsesUsage) *model.Usage {
	if usage == nil {
		return nil
	}
	return &model.Usage{
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func openAIUsageToResponses(usage model.Usage) *responsesUsage {
	return &responsesUsage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		TotalTokens:  usage.TotalTokens,
	}
}

func extractResponsesOutputText(envelope responsesBridgeEnvelope) string {
	if strings.TrimSpace(envelope.OutputText) != "" {
		return strings.TrimSpace(envelope.OutputText)
	}
	builder := strings.Builder{}
	for _, item := range envelope.Output {
		if strings.TrimSpace(item.OutputText) != "" {
			builder.WriteString(item.OutputText)
		}
		if strings.TrimSpace(item.Text) != "" {
			builder.WriteString(item.Text)
		}
		for _, content := range item.Content {
			if strings.TrimSpace(content.OutputText) != "" {
				builder.WriteString(content.OutputText)
			}
			if strings.TrimSpace(content.Text) != "" {
				builder.WriteString(content.Text)
			}
		}
	}
	return builder.String()
}

func relayResponsesAsChatResponse(c *gin.Context, resp *http.Response, modelName string, promptTokens int) (*model.Usage, *model.ErrorWithStatusCode) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}

	envelope := responsesBridgeEnvelope{}
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return nil, ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}

	responseText := extractResponsesOutputText(envelope)
	usage := responseUsageToOpenAI(envelope.Usage)
	if usage == nil || usage.TotalTokens == 0 {
		usage = ResponseText2Usage(responseText, modelName, promptTokens)
	}
	createdAt := envelope.CreatedAt
	if createdAt == 0 {
		createdAt = time.Now().Unix()
	}
	chatResponse := TextResponse{
		Id:      strings.TrimSpace(envelope.ID),
		Object:  "chat.completion",
		Created: createdAt,
		Model:   strings.TrimSpace(envelope.Model),
		Choices: []TextResponseChoice{{
			Index: 0,
			Message: model.Message{
				Role:    "assistant",
				Content: responseText,
			},
			FinishReason: "stop",
		}},
		Usage: *usage,
	}
	if chatResponse.Id == "" {
		chatResponse.Id = fmt.Sprintf("chatcmpl_%d", time.Now().UnixNano())
	}
	if chatResponse.Model == "" {
		chatResponse.Model = modelName
	}

	payload, err := json.Marshal(chatResponse)
	if err != nil {
		return nil, ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
	}
	for k, v := range resp.Header {
		if len(v) == 0 {
			continue
		}
		c.Writer.Header().Set(k, v[0])
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := c.Writer.Write(payload); err != nil {
		return nil, ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
	}
	return usage, nil
}

func relayResponsesStreamAsChatResponse(c *gin.Context, resp *http.Response, modelName string, promptTokens int) (*model.Usage, *model.ErrorWithStatusCode) {
	if resp == nil {
		return nil, ErrorWrapper(fmt.Errorf("resp is nil"), "nil_response", http.StatusInternalServerError)
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}

	trimmedBody := strings.TrimSpace(string(responseBody))
	if !strings.HasPrefix(trimmedBody, "event:") && !strings.HasPrefix(trimmedBody, "data:") {
		fallbackResp := &http.Response{
			StatusCode: resp.StatusCode,
			Header:     resp.Header.Clone(),
			Body:       io.NopCloser(bytes.NewBuffer(responseBody)),
		}
		return relayResponsesAsChatResponse(c, fallbackResp, modelName, promptTokens)
	}

	scanner := bufio.NewScanner(bytes.NewReader(responseBody))
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	scanner.Split(bufio.ScanLines)

	currentEvent := ""
	var responseTextBuilder strings.Builder
	sawDelta := false
	responseID := ""
	responseModel := strings.TrimSpace(modelName)
	createdAt := time.Now().Unix()
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
			continue
		}
		if !strings.HasPrefix(line, dataPrefix) {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, dataPrefix))
		if data == "" || data == done {
			continue
		}

		payload := map[string]any{}
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			continue
		}

		if usagePayload, ok := payload["usage"].(map[string]any); ok {
			if prompt, ok := parseIntFromAny(usagePayload["input_tokens"]); ok && prompt > 0 {
				usage.PromptTokens = prompt
			}
			if completion, ok := parseIntFromAny(usagePayload["output_tokens"]); ok && completion > 0 {
				usage.CompletionTokens = completion
			}
			if total, ok := parseIntFromAny(usagePayload["total_tokens"]); ok && total > 0 {
				usage.TotalTokens = total
			}
		}
		if responsePayload, ok := payload["response"].(map[string]any); ok {
			if value, ok := responsePayload["id"].(string); ok && strings.TrimSpace(value) != "" && responseID == "" {
				responseID = strings.TrimSpace(value)
			}
			if value, ok := responsePayload["model"].(string); ok && strings.TrimSpace(value) != "" {
				responseModel = strings.TrimSpace(value)
			}
			if value, ok := parseIntFromAny(responsePayload["created_at"]); ok && value > 0 {
				createdAt = int64(value)
			}
			if usagePayload, ok := responsePayload["usage"].(map[string]any); ok {
				if prompt, ok := parseIntFromAny(usagePayload["input_tokens"]); ok && prompt > 0 {
					usage.PromptTokens = prompt
				}
				if completion, ok := parseIntFromAny(usagePayload["output_tokens"]); ok && completion > 0 {
					usage.CompletionTokens = completion
				}
				if total, ok := parseIntFromAny(usagePayload["total_tokens"]); ok && total > 0 {
					usage.TotalTokens = total
				}
			}
		}
		if value, ok := payload["id"].(string); ok && strings.TrimSpace(value) != "" && responseID == "" {
			responseID = strings.TrimSpace(value)
		}
		if value, ok := payload["model"].(string); ok && strings.TrimSpace(value) != "" {
			responseModel = strings.TrimSpace(value)
		}
		if value, ok := parseIntFromAny(payload["created_at"]); ok && value > 0 {
			createdAt = int64(value)
		}

		textPayload := responsesStreamTextPayload{}
		if err := json.Unmarshal([]byte(data), &textPayload); err != nil {
			continue
		}
		eventName := currentEvent
		if eventName == "" {
			if payloadEvent, ok := payload["type"].(string); ok {
				eventName = strings.TrimSpace(payloadEvent)
			}
		}
		switch eventName {
		case "response.output_text.delta":
			deltaText := textPayload.Delta
			if strings.TrimSpace(deltaText) == "" {
				deltaText = textPayload.Text
			}
			if strings.TrimSpace(deltaText) == "" {
				deltaText = textPayload.OutputText
			}
			if strings.TrimSpace(deltaText) != "" {
				responseTextBuilder.WriteString(deltaText)
				sawDelta = true
			}
		case "response.output_text":
			if sawDelta {
				continue
			}
			text := textPayload.Text
			if strings.TrimSpace(text) == "" {
				text = textPayload.OutputText
			}
			if strings.TrimSpace(text) != "" {
				responseTextBuilder.WriteString(text)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}

	responseText := strings.TrimSpace(responseTextBuilder.String())
	if usage.CompletionTokens == 0 && responseText != "" {
		usage.CompletionTokens = CountTokenText(responseText, modelName)
	}
	if usage.PromptTokens == 0 {
		usage.PromptTokens = promptTokens
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	if responseID == "" {
		responseID = fmt.Sprintf("resp_%d", time.Now().UnixNano())
	}

	chatResponse := TextResponse{
		Id:      strings.TrimSpace(responseID),
		Object:  "chat.completion",
		Created: createdAt,
		Model:   strings.TrimSpace(responseModel),
		Choices: []TextResponseChoice{{
			Index: 0,
			Message: model.Message{
				Role:    "assistant",
				Content: responseText,
			},
			FinishReason: "stop",
		}},
		Usage: *usage,
	}
	if chatResponse.Model == "" {
		chatResponse.Model = modelName
	}

	encoded, err := json.Marshal(chatResponse)
	if err != nil {
		return nil, ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
	}
	for k, v := range resp.Header {
		if len(v) == 0 {
			continue
		}
		if strings.EqualFold(k, "Content-Type") || strings.EqualFold(k, "Content-Length") {
			continue
		}
		c.Writer.Header().Set(k, v[0])
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := c.Writer.Write(encoded); err != nil {
		return nil, ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
	}
	return usage, nil
}

func relayChatAsResponsesResponse(c *gin.Context, resp *http.Response, modelName string, promptTokens int) (*model.Usage, *model.ErrorWithStatusCode) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}

	textResponse := TextResponse{}
	if err := json.Unmarshal(responseBody, &textResponse); err != nil {
		return nil, ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	responseText := ""
	if len(textResponse.Choices) > 0 {
		responseText = textResponse.Choices[0].Message.StringContent()
	}
	usage := &textResponse.Usage
	if usage == nil || usage.TotalTokens == 0 {
		usage = ResponseText2Usage(responseText, modelName, promptTokens)
	}

	payload := responsesBridgeEnvelope{
		ID:         strings.TrimSpace(textResponse.Id),
		Object:     "response",
		Model:      strings.TrimSpace(textResponse.Model),
		CreatedAt:  textResponse.Created,
		OutputText: responseText,
		Output: []responsesOutputItem{{
			Type: "message",
			Role: "assistant",
			Content: []responsesOutputContent{{
				Type: "output_text",
				Text: responseText,
			}},
		}},
		Usage: openAIUsageToResponses(*usage),
	}
	if payload.ID == "" {
		payload.ID = fmt.Sprintf("resp_%d", time.Now().UnixNano())
	}
	if payload.Model == "" {
		payload.Model = modelName
	}
	if payload.CreatedAt == 0 {
		payload.CreatedAt = time.Now().Unix()
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
	}
	for k, v := range resp.Header {
		if len(v) == 0 {
			continue
		}
		c.Writer.Header().Set(k, v[0])
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := c.Writer.Write(encoded); err != nil {
		return nil, ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
	}
	return usage, nil
}

func emitResponsesEvent(c *gin.Context, event string, payload any) *model.ErrorWithStatusCode {
	if strings.TrimSpace(event) != "" {
		if _, err := c.Writer.Write([]byte("event: " + event + "\n")); err != nil {
			return ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
	}
	if _, err := c.Writer.Write([]byte("data: " + string(encoded) + "\n\n")); err != nil {
		return ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
	}
	c.Writer.Flush()
	return nil
}

func StreamResponsesAsChatHandler(c *gin.Context, resp *http.Response, modelName string, promptTokens int) (*model.ErrorWithStatusCode, *model.Usage) {
	responseText := ""
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)
	var usage *model.Usage
	currentEvent := ""
	firstDelta := true
	sawDelta := false
	finishRendered := false
	createdAt := time.Now().Unix()

	common.SetEventStreamHeaders(c)

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			currentEvent = ""
			continue
		}
		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if !strings.HasPrefix(line, dataPrefix) {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, dataPrefix))
		if data == done {
			break
		}
		envelope := responsesStreamEnvelope{}
		_ = json.Unmarshal([]byte(data), &envelope)
		if envelope.Usage != nil {
			usage = responseUsageToOpenAI(envelope.Usage)
		} else if envelope.Response.Usage != nil {
			usage = responseUsageToOpenAI(envelope.Response.Usage)
		}

		textPayload := responsesStreamTextPayload{}
		_ = json.Unmarshal([]byte(data), &textPayload)
		payload := map[string]any{}
		_ = json.Unmarshal([]byte(data), &payload)
		eventName := currentEvent
		if eventName == "" {
			if payloadEvent, ok := payload["type"].(string); ok {
				eventName = strings.TrimSpace(payloadEvent)
			}
		}
		deltaText := ""
		switch eventName {
		case "response.output_text.delta":
			deltaText = textPayload.Delta
			if deltaText == "" {
				deltaText = textPayload.Text
			}
			if deltaText == "" {
				deltaText = textPayload.OutputText
			}
			if deltaText != "" {
				sawDelta = true
			}
		case "response.output_text", "response.output_text.done", "response.completed":
			if !sawDelta {
				deltaText = textPayload.Text
				if deltaText == "" {
					deltaText = textPayload.OutputText
				}
				if deltaText == "" {
					deltaText = textPayload.Delta
				}
			}
		default:
			deltaText = textPayload.Delta
			if deltaText == "" {
				deltaText = textPayload.Text
			}
			if deltaText == "" {
				deltaText = textPayload.OutputText
			}
		}
		if deltaText != "" {
			responseText += deltaText
			delta := model.Message{Content: deltaText}
			if firstDelta {
				delta.Role = "assistant"
				firstDelta = false
			}
			chunk := ChatCompletionsStreamResponse{
				Id:      fmt.Sprintf("chatcmpl_%d", time.Now().UnixNano()),
				Object:  "chat.completion.chunk",
				Created: createdAt,
				Model:   modelName,
				Choices: []ChatCompletionsStreamResponseChoice{{
					Index: 0,
					Delta: delta,
				}},
			}
			if usage != nil {
				chunk.Usage = usage
			}
			if err := render.ObjectData(c, chunk); err != nil {
				return ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError), usage
			}
		}
		if eventName == "response.completed" && !finishRendered {
			chunk := ChatCompletionsStreamResponse{
				Id:      fmt.Sprintf("chatcmpl_%d", time.Now().UnixNano()),
				Object:  "chat.completion.chunk",
				Created: createdAt,
				Model:   modelName,
				Choices: []ChatCompletionsStreamResponseChoice{{
					Index:        0,
					Delta:        model.Message{},
					FinishReason: func() *string { value := "stop"; return &value }(),
				}},
			}
			if usage != nil {
				chunk.Usage = usage
			}
			if err := render.ObjectData(c, chunk); err != nil {
				return ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError), usage
			}
			finishRendered = true
		}
	}
	if err := scanner.Err(); err != nil {
		return ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError), usage
	}
	if err := resp.Body.Close(); err != nil {
		return ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), usage
	}
	if usage == nil || usage.TotalTokens == 0 {
		usage = ResponseText2Usage(responseText, modelName, promptTokens)
	}
	if !finishRendered {
		chunk := ChatCompletionsStreamResponse{
			Id:      fmt.Sprintf("chatcmpl_%d", time.Now().UnixNano()),
			Object:  "chat.completion.chunk",
			Created: createdAt,
			Model:   modelName,
			Choices: []ChatCompletionsStreamResponseChoice{{
				Index:        0,
				Delta:        model.Message{},
				FinishReason: func() *string { value := "stop"; return &value }(),
			}},
			Usage: usage,
		}
		if err := render.ObjectData(c, chunk); err != nil {
			return ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError), usage
		}
	}
	render.Done(c)
	return nil, usage
}

func StreamChatAsResponsesHandler(c *gin.Context, resp *http.Response, modelName string, promptTokens int) (*model.ErrorWithStatusCode, *model.Usage) {
	responseText := ""
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)
	var usage *model.Usage
	completed := false

	common.SetEventStreamHeaders(c)

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, dataPrefix) {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, dataPrefix))
		if data == done {
			break
		}
		chunk := ChatCompletionsStreamResponse{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		for _, choice := range chunk.Choices {
			deltaText := choice.Delta.StringContent()
			if deltaText != "" {
				responseText += deltaText
				if err := emitResponsesEvent(c, "response.output_text.delta", map[string]string{"delta": deltaText}); err != nil {
					return err, usage
				}
			}
			if choice.FinishReason != nil && *choice.FinishReason != "" && !completed {
				if usage == nil || usage.TotalTokens == 0 {
					usage = ResponseText2Usage(responseText, modelName, promptTokens)
				}
				if err := emitResponsesEvent(c, "response.completed", map[string]any{
					"response": map[string]any{
						"usage": openAIUsageToResponses(*usage),
					},
				}); err != nil {
					return err, usage
				}
				completed = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError), usage
	}
	if err := resp.Body.Close(); err != nil {
		return ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), usage
	}
	if usage == nil || usage.TotalTokens == 0 {
		usage = ResponseText2Usage(responseText, modelName, promptTokens)
	}
	if !completed {
		if err := emitResponsesEvent(c, "response.completed", map[string]any{
			"response": map[string]any{
				"usage": openAIUsageToResponses(*usage),
			},
		}); err != nil {
			return err, usage
		}
	}
	if _, err := c.Writer.Write([]byte("data: [DONE]\n\n")); err != nil {
		return ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError), usage
	}
	c.Writer.Flush()
	return nil, usage
}
