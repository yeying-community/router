package openai

import (
	"bufio"
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

func extractResponsesOutputText(envelope responsesBridgeEnvelope) string {
	if strings.TrimSpace(envelope.OutputText) != "" {
		return envelope.OutputText
	}
	builder := strings.Builder{}
	for _, item := range envelope.Output {
		itemType := strings.ToLower(strings.TrimSpace(item.Type))
		if itemType != "" && itemType != "message" && itemType != "output_text" {
			continue
		}
		if strings.TrimSpace(item.OutputText) != "" {
			builder.WriteString(item.OutputText)
		}
		if strings.TrimSpace(item.Text) != "" {
			builder.WriteString(item.Text)
		}
		for _, content := range item.Content {
			contentType := strings.ToLower(strings.TrimSpace(content.Type))
			if contentType != "" && contentType != "output_text" && contentType != "text" {
				continue
			}
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

func extractResponsesOutputTextFromAny(raw any) string {
	if raw == nil {
		return ""
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return ""
	}
	envelope := responsesBridgeEnvelope{}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return ""
	}
	return strings.TrimSpace(extractResponsesOutputText(envelope))
}

func extractResponsesStreamFallbackText(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if responsePayload, ok := payload["response"]; ok {
		if text := extractResponsesOutputTextFromAny(responsePayload); text != "" {
			return text
		}
	}
	return extractResponsesOutputTextFromAny(payload)
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
	copyUpstreamResponseHeaders(c, resp.Header, true)
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := c.Writer.Write(payload); err != nil {
		return nil, ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
	}
	return usage, nil
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
				if deltaText == "" {
					deltaText = extractResponsesStreamFallbackText(payload)
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
