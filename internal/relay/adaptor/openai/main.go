package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/yeying-community/router/common/render"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/conv"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

const (
	dataPrefix                = "data: "
	done                      = "[DONE]"
	dataPrefixLength          = len(dataPrefix)
	openaiScannerMaxTokenSize = 8 * 1024 * 1024
)

func newOpenAIStreamScanner(body io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(body)
	scanner.Split(bufio.ScanLines)
	scanner.Buffer(make([]byte, 0, 64*1024), openaiScannerMaxTokenSize)
	return scanner
}

func StreamHandler(c *gin.Context, resp *http.Response, relayMode int) (*model.ErrorWithStatusCode, string, *model.Usage) {
	responseText := ""
	scanner := newOpenAIStreamScanner(resp.Body)
	var usage *model.Usage

	common.SetEventStreamHeaders(c)

	doneRendered := false
	for scanner.Scan() {
		data := scanner.Text()
		if len(data) < dataPrefixLength { // ignore blank line or wrong format
			continue
		}
		if data[:dataPrefixLength] != dataPrefix && data[:dataPrefixLength] != done {
			continue
		}
		if strings.HasPrefix(data[dataPrefixLength:], done) {
			render.StringData(c, data)
			doneRendered = true
			continue
		}
		switch relayMode {
		case relaymode.ChatCompletions:
			var streamResponse ChatCompletionsStreamResponse
			err := json.Unmarshal([]byte(data[dataPrefixLength:]), &streamResponse)
			if err != nil {
				logger.SysError("error unmarshalling stream response: " + err.Error())
				render.StringData(c, data) // if error happened, pass the data to client
				continue                   // just ignore the error
			}
			if len(streamResponse.Choices) == 0 && streamResponse.Usage == nil {
				// but for empty choice and no usage, we should not pass it to client, this is for azure
				continue // just ignore empty choice
			}
			render.StringData(c, data)
			for _, choice := range streamResponse.Choices {
				responseText += conv.AsString(choice.Delta.Content)
			}
			if streamResponse.Usage != nil {
				usage = streamResponse.Usage
			}
		case relaymode.Completions:
			render.StringData(c, data)
			var streamResponse CompletionsStreamResponse
			err := json.Unmarshal([]byte(data[dataPrefixLength:]), &streamResponse)
			if err != nil {
				logger.SysError("error unmarshalling stream response: " + err.Error())
				continue
			}
			for _, choice := range streamResponse.Choices {
				responseText += choice.Text
			}
		}
	}

	if err := scanner.Err(); err != nil {
		logOpenAIStreamReadError("chat", err)
	}

	if !doneRendered {
		render.Done(c)
	}

	err := resp.Body.Close()
	if err != nil {
		return ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), "", nil
	}

	return nil, responseText, usage
}

type responsesStreamEnvelope struct {
	Usage    *responsesUsage `json:"usage"`
	Response struct {
		Usage *responsesUsage `json:"usage"`
	} `json:"response"`
}

type responsesStreamTextPayload struct {
	Delta      string `json:"delta"`
	Text       string `json:"text"`
	OutputText string `json:"output_text"`
}

func StreamResponsesHandler(c *gin.Context, resp *http.Response, modelName string, promptTokens int) (*model.ErrorWithStatusCode, *model.Usage) {
	responseText := ""
	scanner := newOpenAIStreamScanner(resp.Body)
	var usage *model.Usage
	currentEvent := ""
	doneRendered := false

	common.SetEventStreamHeaders(c)

	for scanner.Scan() {
		rawLine := scanner.Text()
		line := strings.TrimSuffix(rawLine, "\r")

		_, _ = c.Writer.Write([]byte(rawLine + "\n"))
		c.Writer.Flush()

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
			doneRendered = true
			continue
		}
		if data == "" {
			continue
		}

		var envelope responsesStreamEnvelope
		if err := json.Unmarshal([]byte(data), &envelope); err == nil {
			if envelope.Usage != nil {
				usage = &model.Usage{
					PromptTokens:     envelope.Usage.InputTokens,
					CompletionTokens: envelope.Usage.OutputTokens,
					TotalTokens:      envelope.Usage.TotalTokens,
				}
			} else if envelope.Response.Usage != nil {
				usage = &model.Usage{
					PromptTokens:     envelope.Response.Usage.InputTokens,
					CompletionTokens: envelope.Response.Usage.OutputTokens,
					TotalTokens:      envelope.Response.Usage.TotalTokens,
				}
			}
		}

		var textPayload responsesStreamTextPayload
		if err := json.Unmarshal([]byte(data), &textPayload); err != nil {
			continue
		}
		switch currentEvent {
		case "response.output_text.delta":
			if textPayload.Delta != "" {
				responseText += textPayload.Delta
			} else if textPayload.Text != "" {
				responseText += textPayload.Text
			} else if textPayload.OutputText != "" {
				responseText += textPayload.OutputText
			}
		case "response.output_text":
			if textPayload.Text != "" {
				responseText += textPayload.Text
			} else if textPayload.OutputText != "" {
				responseText += textPayload.OutputText
			}
		case "response.completed":
			if textPayload.Text != "" {
				responseText += textPayload.Text
			} else if textPayload.OutputText != "" {
				responseText += textPayload.OutputText
			} else if textPayload.Delta != "" {
				responseText += textPayload.Delta
			}
		default:
			if textPayload.Text != "" {
				responseText += textPayload.Text
			} else if textPayload.OutputText != "" {
				responseText += textPayload.OutputText
			} else if textPayload.Delta != "" {
				responseText += textPayload.Delta
			}
		}
	}

	if err := scanner.Err(); err != nil {
		logOpenAIStreamReadError("responses", err)
	}

	if !doneRendered {
		render.Done(c)
	}

	if err := resp.Body.Close(); err != nil {
		return ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), nil
	}

	if usage == nil || usage.TotalTokens == 0 {
		usage = ResponseText2Usage(responseText, modelName, promptTokens)
	}

	return nil, usage
}

func logOpenAIStreamReadError(kind string, err error) {
	if err == nil {
		return
	}
	lowerMessage := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case errors.Is(err, context.Canceled) || strings.Contains(lowerMessage, "context canceled"):
		logger.SysWarnf("[openai.stream] read_stopped kind=%s reason=context_canceled err=%q", strings.TrimSpace(kind), err.Error())
	case strings.Contains(lowerMessage, "token too long"):
		logger.SysErrorf("[openai.stream] read_failed kind=%s reason=scanner_token_too_long max_token_bytes=%d err=%q", strings.TrimSpace(kind), openaiScannerMaxTokenSize, err.Error())
	default:
		logger.SysErrorf("[openai.stream] read_failed kind=%s err=%q", strings.TrimSpace(kind), err.Error())
	}
}

func Handler(c *gin.Context, resp *http.Response, promptTokens int, modelName string) (*model.ErrorWithStatusCode, *model.Usage) {
	var textResponse SlimTextResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError), nil
	}
	err = resp.Body.Close()
	if err != nil {
		return ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), nil
	}
	err = json.Unmarshal(responseBody, &textResponse)
	if err != nil {
		return ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError), nil
	}
	if textResponse.Error.Type != "" {
		return &model.ErrorWithStatusCode{
			Error:      textResponse.Error,
			StatusCode: resp.StatusCode,
		}, nil
	}
	// Reset response body
	resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))

	// We shouldn't set the header before we parse the response body, because the parse part may fail.
	// And then we will have to send an error response, but in this case, the header has already been set.
	// So the HTTPClient will be confused by the response.
	// For example, Postman will report error, and we cannot check the response at all.
	for k, v := range resp.Header {
		c.Writer.Header().Set(k, v[0])
	}
	c.Writer.WriteHeader(resp.StatusCode)
	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		return ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError), nil
	}
	err = resp.Body.Close()
	if err != nil {
		return ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), nil
	}

	if textResponse.Usage.TotalTokens == 0 || (textResponse.Usage.PromptTokens == 0 && textResponse.Usage.CompletionTokens == 0) {
		completionTokens := 0
		for _, choice := range textResponse.Choices {
			completionTokens += CountTokenText(choice.Message.StringContent(), modelName)
		}
		textResponse.Usage = model.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
	}
	return nil, &textResponse.Usage
}
