package zhipu

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/relay/adaptor"
	"github.com/yeying-community/router/internal/relay/adaptor/openai"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/meta"
	"github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

type Adaptor struct {
	APIVersion string
}

func (a *Adaptor) Init(meta *meta.Meta) {

}

func (a *Adaptor) SetVersionByModeName(modelName string) {
	if strings.HasPrefix(modelName, "glm-") {
		a.APIVersion = "v4"
	} else {
		a.APIVersion = "v3"
	}
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta) (string, error) {
	upstreamMode := meta.Mode
	if meta != nil && meta.UpstreamMode != 0 {
		upstreamMode = meta.UpstreamMode
	}
	if upstreamMode == relaymode.Messages {
		baseURL := strings.TrimRight(strings.TrimSpace(meta.BaseURL), "/")
		return baseURL + "/api/anthropic/v1/messages", nil
	}
	baseURL := strings.TrimRight(strings.TrimSpace(meta.BaseURL), "/")
	if upstreamMode == relaymode.Realtime {
		return baseURL + "/api/paas/v4/realtime", nil
	}
	switch meta.Mode {
	case relaymode.ImagesGenerations:
		return fmt.Sprintf("%s/api/paas/v4/images/generations", baseURL), nil
	case relaymode.Embeddings:
		return fmt.Sprintf("%s/api/paas/v4/embeddings", baseURL), nil
	}
	a.SetVersionByModeName(meta.ActualModelName)
	if a.APIVersion == "v4" {
		return fmt.Sprintf("%s/api/paas/v4/chat/completions", baseURL), nil
	}
	method := "invoke"
	if meta.IsStream {
		method = "sse-invoke"
	}
	return fmt.Sprintf("%s/api/paas/v3/model-api/%s/%s", baseURL, meta.ActualModelName, method), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *meta.Meta) error {
	adaptor.SetupCommonRequestHeader(c, req, meta)
	upstreamMode := meta.Mode
	if meta != nil && meta.UpstreamMode != 0 {
		upstreamMode = meta.UpstreamMode
	}
	if upstreamMode == relaymode.Messages {
		req.Header.Del("Authorization")
		req.Header.Set("x-api-key", meta.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		if meta.IsStream {
			req.Header.Set("Accept", "text/event-stream")
		} else {
			req.Header.Set("Accept", "application/json")
		}
		return nil
	}
	token := GetToken(meta.APIKey)
	req.Header.Set("Authorization", token)
	return nil
}

func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *model.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	switch relayMode {
	case relaymode.Embeddings:
		baiduEmbeddingRequest, err := ConvertEmbeddingRequest(*request)
		return baiduEmbeddingRequest, err
	default:
		// TopP [0.0, 1.0]
		request.TopP = helper.Float64PtrMax(request.TopP, 1)
		request.TopP = helper.Float64PtrMin(request.TopP, 0)

		// Temperature [0.0, 1.0]
		request.Temperature = helper.Float64PtrMax(request.Temperature, 1)
		request.Temperature = helper.Float64PtrMin(request.Temperature, 0)
		a.SetVersionByModeName(request.Model)
		if a.APIVersion == "v4" {
			return request, nil
		}
		return ConvertRequest(*request), nil
	}
}

func (a *Adaptor) ConvertImageRequest(request *model.ImageRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	newRequest := ImageRequest{
		Model:  request.Model,
		Prompt: request.Prompt,
		UserId: request.User,
	}
	return newRequest, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, meta *meta.Meta, requestBody io.Reader) (*http.Response, error) {
	return adaptor.DoRequestHelper(a, c, meta, requestBody)
}

func (a *Adaptor) DoResponseV4(c *gin.Context, resp *http.Response, meta *meta.Meta) (usage *model.Usage, err *model.ErrorWithStatusCode) {
	if meta.IsStream {
		err, _, usage = openai.StreamHandler(c, resp, meta.Mode)
	} else {
		err, usage = openai.Handler(c, resp, meta.PromptTokens, meta.ActualModelName)
	}
	return
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, meta *meta.Meta) (usage *model.Usage, err *model.ErrorWithStatusCode) {
	upstreamMode := meta.Mode
	if meta != nil && meta.UpstreamMode != 0 {
		upstreamMode = meta.UpstreamMode
	}
	if meta.Mode == relaymode.Messages && upstreamMode == relaymode.Messages {
		openAIAdaptor := &openai.Adaptor{}
		openAIAdaptor.Init(meta)
		return openAIAdaptor.DoResponse(c, resp, meta)
	}
	switch meta.Mode {
	case relaymode.Embeddings:
		err, usage = EmbeddingsHandler(c, resp)
		return
	case relaymode.ImagesGenerations:
		err, usage = openai.ImageHandler(c, resp)
		return
	}
	if a.APIVersion == "v4" {
		return a.DoResponseV4(c, resp, meta)
	}
	if meta.IsStream {
		err, usage = StreamHandler(c, resp)
	} else {
		if meta.Mode == relaymode.Embeddings {
			err, usage = EmbeddingsHandler(c, resp)
		} else {
			err, usage = Handler(c, resp)
		}
	}
	return
}

func ConvertEmbeddingRequest(request model.GeneralOpenAIRequest) (*EmbeddingRequest, error) {
	inputs := request.ParseInput()
	if len(inputs) != 1 {
		return nil, errors.New("invalid input length, zhipu only support one input")
	}
	return &EmbeddingRequest{
		Model: request.Model,
		Input: inputs[0],
	}, nil
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return relaychannel.ProtocolByType(relaychannel.Zhipu)
}
