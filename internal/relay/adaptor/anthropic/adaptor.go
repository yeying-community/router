package anthropic

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/relay/adaptor"
	"github.com/yeying-community/router/internal/relay/adaptor/openai"
	"github.com/yeying-community/router/internal/relay/meta"
	"github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

type Adaptor struct {
}

func (a *Adaptor) Init(meta *meta.Meta) {

}

func resolveUpstreamMode(meta *meta.Meta) int {
	if meta == nil {
		return relaymode.Messages
	}
	if meta.UpstreamMode != 0 {
		return meta.UpstreamMode
	}
	return meta.Mode
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta) (string, error) {
	upstreamMode := resolveUpstreamMode(meta)
	requestPath := strings.TrimSpace(meta.UpstreamRequestPath)
	if requestPath == "" {
		switch upstreamMode {
		case relaymode.ChatCompletions:
			requestPath = "/v1/chat/completions"
		case relaymode.Responses:
			requestPath = "/v1/responses"
		default:
			requestPath = "/v1/messages"
		}
	}
	return openai.GetFullRequestURL(meta.BaseURL, requestPath, meta.ChannelProtocol), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *meta.Meta) error {
	adaptor.SetupCommonRequestHeader(c, req, meta)
	if meta.IsStream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	upstreamMode := resolveUpstreamMode(meta)
	if upstreamMode != relaymode.Messages {
		req.Header.Set("Authorization", "Bearer "+meta.APIKey)
		req.Header.Set("x-api-key", meta.APIKey)
		return nil
	}
	req.Header.Set("x-api-key", meta.APIKey)
	anthropicVersion := c.Request.Header.Get("anthropic-version")
	if anthropicVersion == "" {
		anthropicVersion = "2023-06-01"
	}
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("anthropic-beta", "messages-2023-12-15")

	// https://x.com/alexalbert__/status/1812921642143900036
	// claude-3-5-sonnet can support 8k context
	if strings.HasPrefix(meta.ActualModelName, "claude-3-5-sonnet") {
		req.Header.Set("anthropic-beta", "max-tokens-3-5-sonnet-2024-07-15")
	}

	return nil
}

func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *model.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if relayMode != relaymode.Messages {
		return request, nil
	}
	return ConvertRequest(*request), nil
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
	upstreamMode := resolveUpstreamMode(meta)
	if meta.Mode == relaymode.Messages && upstreamMode == relaymode.Messages {
		logger.Debugf(
			c.Request.Context(),
			"[anthropic_messages_passthrough] channel_id=%s model=%s stream=%t",
			strings.TrimSpace(meta.ChannelId),
			strings.TrimSpace(meta.ActualModelName),
			meta.IsStream,
		)
		if meta.IsStream {
			return relayMessagesStreamResponse(c, resp)
		}
		return relayMessagesResponse(c, resp)
	}
	if meta.Mode == relaymode.ChatCompletions && upstreamMode == relaymode.Messages && !meta.IsStream {
		return relayMessagesStreamAsChatResponse(c, resp, meta.PromptTokens, meta.ActualModelName)
	}
	if upstreamMode != relaymode.Messages {
		openaiAdaptor := &openai.Adaptor{}
		openaiAdaptor.Init(meta)
		return openaiAdaptor.DoResponse(c, resp, meta)
	}
	if meta.IsStream {
		err, usage = StreamHandler(c, resp)
	} else {
		err, usage = Handler(c, resp, meta.PromptTokens, meta.ActualModelName)
	}
	return
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return "anthropic"
}
