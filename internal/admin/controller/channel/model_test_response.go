package channel

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	openaiadaptor "github.com/yeying-community/router/internal/relay/adaptor/openai"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

type responsesEnvelope struct {
	Status     string `json:"status"`
	OutputText string `json:"output_text"`
	Usage      *struct {
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Output []struct {
		Content []struct {
			Type       string `json:"type"`
			Text       string `json:"text"`
			OutputText string `json:"output_text"`
			Result     string `json:"result"`
		} `json:"content"`
	} `json:"output"`
}

type messagesEnvelope struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func parseChatModelTestResponse(resp string) (*openaiadaptor.TextResponse, string, error) {
	var response openaiadaptor.TextResponse
	err := json.Unmarshal([]byte(resp), &response)
	if err != nil {
		return nil, "", err
	}
	if len(response.Choices) == 0 {
		return nil, "", errors.New("response has no choices")
	}
	stringContent, ok := response.Choices[0].Content.(string)
	if !ok {
		return nil, "", errors.New("response content is not string")
	}
	return &response, stringContent, nil
}

func parseResponsesModelTestResponse(resp string) (string, error) {
	var env responsesEnvelope
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		return "", err
	}
	if strings.TrimSpace(env.OutputText) != "" {
		return strings.TrimSpace(env.OutputText), nil
	}
	contentTypes := make([]string, 0)
	for _, output := range env.Output {
		for _, content := range output.Content {
			if content.Type != "" {
				contentTypes = append(contentTypes, content.Type)
			} else {
				contentTypes = append(contentTypes, "<empty>")
			}
			if content.Text != "" {
				return content.Text, nil
			}
			if content.OutputText != "" {
				return content.OutputText, nil
			}
		}
	}
	if len(contentTypes) == 0 {
		if strings.EqualFold(strings.TrimSpace(env.Status), "completed") && env.Usage != nil && (env.Usage.OutputTokens > 0 || env.Usage.TotalTokens > 0) {
			return "responses 接口返回成功（无可见文本，已返回用量）", nil
		}
		return "", errors.New("response has no output text, output is empty")
	}
	return "", errors.New("response has no output text, content types: " + strings.Join(contentTypes, ","))
}

func parseResponsesImageTestResponse(resp string) (string, error) {
	var env responsesEnvelope
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		return "", err
	}
	contentTypes := make([]string, 0)
	imageCount := 0
	for _, output := range env.Output {
		for _, content := range output.Content {
			contentType := strings.TrimSpace(content.Type)
			if contentType == "" {
				contentType = "<empty>"
			}
			contentTypes = append(contentTypes, contentType)
			if content.Text != "" || content.OutputText != "" {
				return "responses 接口返回成功", nil
			}
			switch contentType {
			case "image_generation_call", "output_image", "image":
				imageCount++
			}
		}
	}
	if imageCount > 0 {
		return fmt.Sprintf("responses 接口返回 %d 个图片结果", imageCount), nil
	}
	return "", errors.New("response has no image output, content types: " + strings.Join(contentTypes, ","))
}

func parseMessagesModelTestResponse(resp string) (string, error) {
	var env messagesEnvelope
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		return "", err
	}
	if len(env.Content) == 0 {
		return "", errors.New("response has no messages content")
	}
	textParts := make([]string, 0, len(env.Content))
	contentTypes := make([]string, 0, len(env.Content))
	for _, content := range env.Content {
		contentType := strings.TrimSpace(content.Type)
		if contentType == "" {
			contentType = "<empty>"
		}
		contentTypes = append(contentTypes, contentType)
		if strings.EqualFold(contentType, "text") && strings.TrimSpace(content.Text) != "" {
			textParts = append(textParts, strings.TrimSpace(content.Text))
		}
	}
	if len(textParts) > 0 {
		return strings.Join(textParts, "\n"), nil
	}
	return "", errors.New("response has no messages text, content types: " + strings.Join(contentTypes, ","))
}

func parseTextModelTestResponse(resp string) (string, error) {
	_, chatText, chatErr := parseChatModelTestResponse(resp)
	if chatErr == nil {
		return chatText, nil
	}
	messagesText, messagesErr := parseMessagesModelTestResponse(resp)
	if messagesErr == nil {
		return messagesText, nil
	}
	responsesText, responsesErr := parseResponsesModelTestResponse(resp)
	if responsesErr == nil {
		return responsesText, nil
	}
	if isLikelySSEPayload(resp) {
		streamText, streamErr := parseTextModelTestStreamResponse(resp)
		if streamErr == nil {
			return streamText, nil
		}
		return "", fmt.Errorf("parse as chat failed: %v; parse as messages failed: %v; parse as responses failed: %v; parse as stream failed: %v", chatErr, messagesErr, responsesErr, streamErr)
	}
	return "", fmt.Errorf("parse as chat failed: %v; parse as messages failed: %v; parse as responses failed: %v", chatErr, messagesErr, responsesErr)
}

type streamEnvelope struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type streamTextPayload struct {
	Type       string `json:"type"`
	Delta      string `json:"delta"`
	Text       string `json:"text"`
	OutputText string `json:"output_text"`
	Message    string `json:"message"`
}

type chatStreamChoice struct {
	Delta relaymodel.Message `json:"delta"`
}

type chatStreamEnvelope struct {
	Choices []chatStreamChoice `json:"choices"`
}

func isLikelySSEPayload(resp string) bool {
	trimmed := strings.TrimSpace(resp)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "event:") || strings.HasPrefix(lower, "data:")
}

func parseTextModelTestStreamResponse(resp string) (string, error) {
	lines := strings.Split(resp, "\n")
	currentEvent := ""
	textParts := make([]string, 0, 8)
	sawData := false
	for _, rawLine := range lines {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" {
			currentEvent = ""
			continue
		}
		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		sawData = true
		if data == "[DONE]" {
			continue
		}

		var envelope streamEnvelope
		if err := json.Unmarshal([]byte(data), &envelope); err == nil && envelope.Error != nil {
			message := strings.TrimSpace(envelope.Error.Message)
			if message == "" {
				message = "stream response contains error"
			}
			return "", errors.New(message)
		}

		var responsePayload streamTextPayload
		if err := json.Unmarshal([]byte(data), &responsePayload); err == nil {
			text := extractTextFromStreamPayload(currentEvent, responsePayload)
			if text != "" {
				normalizedEvent := strings.TrimSpace(currentEvent)
				effectiveEvent := normalizedEvent
				if effectiveEvent == "" {
					effectiveEvent = strings.TrimSpace(responsePayload.Type)
				}
				if strings.HasPrefix(effectiveEvent, "response.output_text") || effectiveEvent == "response.completed" {
					joined := strings.Join(textParts, "")
					switch {
					case joined == "":
						textParts = append(textParts, text)
					case strings.Contains(text, joined):
						// completed payload already contains all deltas; keep a single copy.
						textParts = []string{text}
					case strings.Contains(joined, text):
						// completed payload is only a subset; keep existing accumulated text.
					default:
						// completed payload carries incremental tail (for example punctuation).
						textParts = append(textParts, text)
					}
				} else {
					textParts = append(textParts, text)
				}
				continue
			}
		}

		var chatPayload chatStreamEnvelope
		if err := json.Unmarshal([]byte(data), &chatPayload); err == nil {
			for _, choice := range chatPayload.Choices {
				if deltaText := choice.Delta.StringContent(); deltaText != "" {
					textParts = append(textParts, deltaText)
				}
			}
		}
	}

	if len(textParts) > 0 {
		return strings.TrimSpace(strings.Join(textParts, "")), nil
	}
	if sawData {
		return "流式接口返回成功", nil
	}
	return "", errors.New("stream response has no usable data")
}

func extractTextFromStreamPayload(event string, payload streamTextPayload) string {
	switch strings.TrimSpace(event) {
	case "response.output_text.delta":
		if payload.Delta != "" {
			return payload.Delta
		}
	case "response.output_text", "response.completed":
		if payload.Text != "" {
			return payload.Text
		}
		if payload.OutputText != "" {
			return payload.OutputText
		}
		if payload.Delta != "" {
			return payload.Delta
		}
	default:
		if payload.Text != "" {
			return payload.Text
		}
		if payload.OutputText != "" {
			return payload.OutputText
		}
		if payload.Delta != "" {
			return payload.Delta
		}
		if strings.TrimSpace(payload.Message) != "" {
			return strings.TrimSpace(payload.Message)
		}
	}
	if strings.HasPrefix(strings.TrimSpace(payload.Type), "response.output_text") {
		if payload.Delta != "" {
			return payload.Delta
		}
		if payload.Text != "" {
			return payload.Text
		}
		if payload.OutputText != "" {
			return payload.OutputText
		}
	}
	return ""
}
