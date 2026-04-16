package openai

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/internal/relay/adaptor"
	"github.com/yeying-community/router/internal/relay/adaptor/alibailian"
	"github.com/yeying-community/router/internal/relay/adaptor/baiduv2"
	"github.com/yeying-community/router/internal/relay/adaptor/doubao"
	"github.com/yeying-community/router/internal/relay/adaptor/geminiv2"
	"github.com/yeying-community/router/internal/relay/adaptor/minimax"
	"github.com/yeying-community/router/internal/relay/adaptor/novita"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/meta"
	"github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

type Adaptor struct {
	ChannelProtocol int
}

func (a *Adaptor) Init(meta *meta.Meta) {
	a.ChannelProtocol = meta.ChannelProtocol
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta) (string, error) {
	upstreamMode := meta.Mode
	if meta.UpstreamMode != 0 {
		upstreamMode = meta.UpstreamMode
	}
	requestURLPath := meta.RequestURLPath
	if strings.TrimSpace(meta.UpstreamRequestPath) != "" {
		requestURLPath = meta.UpstreamRequestPath
	}
	switch meta.ChannelProtocol {
	case relaychannel.Azure:
		if upstreamMode == relaymode.ImagesGenerations {
			// https://learn.microsoft.com/en-us/azure/ai-services/openai/dall-e-quickstart?tabs=dalle3%2Ccommand-line&pivots=rest-api
			// https://{resource_name}.openai.azure.com/openai/deployments/dall-e-3/images/generations?api-version=2024-03-01-preview
			fullRequestURL := fmt.Sprintf("%s/openai/deployments/%s/images/generations?api-version=%s", meta.BaseURL, meta.ActualModelName, meta.Config.APIVersion)
			return fullRequestURL, nil
		}

		// https://learn.microsoft.com/en-us/azure/cognitive-services/openai/chatgpt-quickstart?pivots=rest-api&tabs=command-line#rest-api
		requestURL := strings.Split(requestURLPath, "?")[0]
		requestURL = fmt.Sprintf("%s?api-version=%s", requestURL, meta.Config.APIVersion)
		task := strings.TrimPrefix(requestURL, "/v1/")
		model_ := meta.ActualModelName
		model_ = strings.Replace(model_, ".", "", -1)
		//https://github.com/yeying-community/router/issues/1191
		// {your endpoint}/openai/deployments/{your azure_model}/chat/completions?api-version={api_version}
		requestURL = fmt.Sprintf("/openai/deployments/%s/%s", model_, task)
		return GetFullRequestURL(meta.BaseURL, requestURL, meta.ChannelProtocol), nil
	case relaychannel.Minimax:
		return minimax.GetRequestURL(meta)
	case relaychannel.Doubao:
		return doubao.GetRequestURL(meta)
	case relaychannel.Novita:
		return novita.GetRequestURL(meta)
	case relaychannel.BaiduV2:
		return baiduv2.GetRequestURL(meta)
	case relaychannel.AliBailian:
		return alibailian.GetRequestURL(meta)
	case relaychannel.GeminiOpenAICompatible:
		return geminiv2.GetRequestURL(meta)
	default:
		return GetFullRequestURL(meta.BaseURL, requestURLPath, meta.ChannelProtocol), nil
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *meta.Meta) error {
	adaptor.SetupCommonRequestHeader(c, req, meta)
	if meta.ForceUpstreamStream {
		req.Header.Set("Accept", "text/event-stream")
	}
	if meta.ChannelProtocol == relaychannel.Azure {
		req.Header.Set("api-key", meta.APIKey)
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+meta.APIKey)
	if meta.ChannelProtocol == relaychannel.OpenRouter {
		req.Header.Set("HTTP-Referer", "https://github.com/yeying-community/router")
		req.Header.Set("X-Title", "Router")
	}
	return nil
}

func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *model.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if request.Stream {
		// always return usage in stream mode
		if request.StreamOptions == nil {
			request.StreamOptions = &model.StreamOptions{}
		}
		request.StreamOptions.IncludeUsage = true
	}
	return request, nil
}

func (a *Adaptor) ConvertImageRequest(request *model.ImageRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, meta *meta.Meta, requestBody io.Reader) (*http.Response, error) {
	return adaptor.DoRequestHelper(a, c, meta, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, meta *meta.Meta) (usage *model.Usage, err *model.ErrorWithStatusCode) {
	upstreamMode := meta.Mode
	if meta.UpstreamMode != 0 {
		upstreamMode = meta.UpstreamMode
	}
	if meta.Mode == relaymode.Messages && upstreamMode == relaymode.Messages {
		if meta.IsStream {
			return relayMessagesStreamResponse(c, resp, meta.PromptTokens, meta.ActualModelName)
		}
		return relayMessagesResponse(c, resp, meta.PromptTokens, meta.ActualModelName)
	}
	if meta.Mode == relaymode.ChatCompletions && upstreamMode == relaymode.Responses {
		if meta.IsStream {
			respErr, usage := StreamResponsesAsChatHandler(c, resp, meta.ActualModelName, meta.PromptTokens)
			return usage, respErr
		}
		return relayResponsesAsChatResponse(c, resp, meta.ActualModelName, meta.PromptTokens)
	}
	if meta.Mode == relaymode.Responses && upstreamMode == relaymode.ChatCompletions {
		if meta.IsStream {
			respErr, usage := StreamChatAsResponsesHandler(c, resp, meta.ActualModelName, meta.PromptTokens)
			return usage, respErr
		}
		return relayChatAsResponsesResponse(c, resp, meta.ActualModelName, meta.PromptTokens)
	}
	if upstreamMode == relaymode.Responses {
		if resp == nil {
			return nil, ErrorWrapper(errors.New("resp is nil"), "nil_response", http.StatusInternalServerError)
		}
		if meta.IsStream {
			respErr, usage := StreamResponsesHandler(c, resp, meta.ActualModelName, meta.PromptTokens)
			if respErr != nil {
				return nil, respErr
			}
			if usage != nil && usage.TotalTokens != 0 && usage.PromptTokens == 0 {
				usage.PromptTokens = meta.PromptTokens
				usage.CompletionTokens = usage.TotalTokens - meta.PromptTokens
			}
			return usage, nil
		}
		usage, respErr := relayResponsesResponse(c, resp)
		if respErr != nil {
			return nil, respErr
		}
		return usage, nil
	}
	if meta.IsStream {
		var responseText string
		err, responseText, usage = StreamHandler(c, resp, meta.Mode)
		if usage == nil || usage.TotalTokens == 0 {
			usage = ResponseText2Usage(responseText, meta.ActualModelName, meta.PromptTokens)
		}
		if usage.TotalTokens != 0 && usage.PromptTokens == 0 { // some channels don't return prompt tokens & completion tokens
			usage.PromptTokens = meta.PromptTokens
			usage.CompletionTokens = usage.TotalTokens - meta.PromptTokens
		}
	} else {
		switch meta.Mode {
		case relaymode.ImagesGenerations:
			err, _ = ImageHandler(c, resp)
		default:
			err, usage = Handler(c, resp, meta.PromptTokens, meta.ActualModelName)
		}
	}
	return
}

func relayRawResponse(c *gin.Context, resp *http.Response) *model.ErrorWithStatusCode {
	copyUpstreamResponseHeaders(c, resp.Header, false)
	c.Writer.WriteHeader(resp.StatusCode)
	_, err := io.Copy(c.Writer, resp.Body)
	if err != nil {
		return ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
	}
	if err := resp.Body.Close(); err != nil {
		return ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}
	return nil
}

type responsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type messagesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type messagesEnvelope struct {
	Usage   *messagesUsage `json:"usage"`
	Message struct {
		Usage *messagesUsage `json:"usage"`
	} `json:"message"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type responsesEnvelope struct {
	Usage *responsesUsage `json:"usage"`
}

func relayMessagesResponse(c *gin.Context, resp *http.Response, promptTokens int, modelName string) (*model.Usage, *model.ErrorWithStatusCode) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}
	copyUpstreamResponseHeaders(c, resp.Header, false)
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := c.Writer.Write(responseBody); err != nil {
		return nil, ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
	}

	usage := &model.Usage{
		PromptTokens: promptTokens,
	}
	var envelope messagesEnvelope
	if err := json.Unmarshal(responseBody, &envelope); err == nil {
		applyMessagesUsage(usage, envelope.Usage)
		applyMessagesUsage(usage, envelope.Message.Usage)
		if usage.CompletionTokens == 0 && usage.TotalTokens == 0 && len(envelope.Content) > 0 {
			var textBuilder strings.Builder
			for _, item := range envelope.Content {
				if item.Type != "text" {
					continue
				}
				if strings.TrimSpace(item.Text) == "" {
					continue
				}
				textBuilder.WriteString(item.Text)
			}
			if textBuilder.Len() > 0 {
				usage.CompletionTokens = CountTokenText(textBuilder.String(), modelName)
			}
		}
	}
	if usage.PromptTokens == 0 {
		usage.PromptTokens = promptTokens
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return usage, nil
}

func relayMessagesStreamResponse(c *gin.Context, resp *http.Response, promptTokens int, modelName string) (*model.Usage, *model.ErrorWithStatusCode) {
	if resp == nil {
		return nil, ErrorWrapper(errors.New("resp is nil"), "nil_response", http.StatusInternalServerError)
	}
	copyUpstreamResponseHeaders(c, resp.Header, false)
	c.Writer.WriteHeader(resp.StatusCode)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	usage := &model.Usage{
		PromptTokens: promptTokens,
	}
	var completionText strings.Builder
	for scanner.Scan() {
		rawLine := scanner.Text()
		line := strings.TrimSuffix(rawLine, "\r")
		_, _ = c.Writer.Write([]byte(rawLine + "\n"))
		c.Writer.Flush()

		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		payload := map[string]any{}
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			continue
		}
		if currentUsage, ok := payload["usage"].(map[string]any); ok {
			applyMessagesUsageMap(usage, currentUsage)
		}
		if messagePayload, ok := payload["message"].(map[string]any); ok {
			if messageUsage, ok := messagePayload["usage"].(map[string]any); ok {
				applyMessagesUsageMap(usage, messageUsage)
			}
		}
		if deltaPayload, ok := payload["delta"].(map[string]any); ok {
			if textDelta, ok := deltaPayload["text"].(string); ok && strings.TrimSpace(textDelta) != "" {
				completionText.WriteString(textDelta)
			}
		}
		if contentBlockPayload, ok := payload["content_block"].(map[string]any); ok {
			if strings.TrimSpace(fmt.Sprint(contentBlockPayload["type"])) == "text" {
				if textPart, ok := contentBlockPayload["text"].(string); ok && strings.TrimSpace(textPart) != "" {
					completionText.WriteString(textPart)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}

	if usage.CompletionTokens == 0 && completionText.Len() > 0 {
		usage.CompletionTokens = CountTokenText(completionText.String(), modelName)
	}
	if usage.PromptTokens == 0 {
		usage.PromptTokens = promptTokens
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return usage, nil
}

func applyMessagesUsage(target *model.Usage, usage *messagesUsage) {
	if target == nil || usage == nil {
		return
	}
	if usage.InputTokens > 0 {
		target.PromptTokens = usage.InputTokens
	}
	if usage.OutputTokens > 0 {
		target.CompletionTokens = usage.OutputTokens
	}
	if usage.TotalTokens > 0 {
		target.TotalTokens = usage.TotalTokens
	}
}

func applyMessagesUsageMap(target *model.Usage, payload map[string]any) {
	if target == nil || payload == nil {
		return
	}
	if inputTokens, ok := parseIntFromAny(payload["input_tokens"]); ok && inputTokens > 0 {
		target.PromptTokens = inputTokens
	}
	if outputTokens, ok := parseIntFromAny(payload["output_tokens"]); ok && outputTokens > 0 {
		target.CompletionTokens = outputTokens
	}
	if totalTokens, ok := parseIntFromAny(payload["total_tokens"]); ok && totalTokens > 0 {
		target.TotalTokens = totalTokens
	}
}

func parseIntFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(parsed), true
	default:
		return 0, false
	}
}

func relayResponsesResponse(c *gin.Context, resp *http.Response) (*model.Usage, *model.ErrorWithStatusCode) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}
	var envelope responsesEnvelope
	_ = json.Unmarshal(responseBody, &envelope)
	copyUpstreamResponseHeaders(c, resp.Header, false)
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := c.Writer.Write(responseBody); err != nil {
		return nil, ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
	}
	if envelope.Usage == nil {
		return nil, nil
	}
	return &model.Usage{
		PromptTokens:     envelope.Usage.InputTokens,
		CompletionTokens: envelope.Usage.OutputTokens,
		TotalTokens:      envelope.Usage.TotalTokens,
	}, nil
}

func (a *Adaptor) GetModelList() []string {
	_, modelList := GetCompatibleChannelMeta(a.ChannelProtocol)
	return modelList
}

func (a *Adaptor) GetChannelName() string {
	channelName, _ := GetCompatibleChannelMeta(a.ChannelProtocol)
	return channelName
}
