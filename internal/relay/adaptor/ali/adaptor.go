package ali

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/internal/relay/adaptor"
	openaiadaptor "github.com/yeying-community/router/internal/relay/adaptor/openai"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/meta"
	"github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

type Adaptor struct {
	meta *meta.Meta
}

func (a *Adaptor) Init(meta *meta.Meta) {
	a.meta = meta
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta) (string, error) {
	fullRequestURL := ""
	switch meta.Mode {
	case relaymode.Embeddings:
		fullRequestURL = fmt.Sprintf("%s/compatible-mode/v1/embeddings", meta.BaseURL)
	case relaymode.Responses:
		fullRequestURL = fmt.Sprintf("%s/compatible-mode/v1/responses", meta.BaseURL)
	case relaymode.AudioSpeech, relaymode.AudioTranslation, relaymode.AudioTranscription, relaymode.Realtime, relaymode.Videos:
		fullRequestURL = openaiadaptor.GetFullRequestURL(meta.BaseURL, meta.RequestURLPath, relaychannel.OpenAI)
	case relaymode.ImagesGenerations:
		fullRequestURL = fmt.Sprintf("%s/api/v1/services/aigc/text2image/image-synthesis", meta.BaseURL)
	case relaymode.Completions:
		fullRequestURL = fmt.Sprintf("%s/compatible-mode/v1/completions", meta.BaseURL)
	default:
		fullRequestURL = fmt.Sprintf("%s/compatible-mode/v1/chat/completions", meta.BaseURL)
	}

	return fullRequestURL, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *meta.Meta) error {
	adaptor.SetupCommonRequestHeader(c, req, meta)
	if meta.IsStream {
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("X-DashScope-SSE", "enable")
	}
	req.Header.Set("Authorization", "Bearer "+meta.APIKey)

	if meta.Mode == relaymode.ImagesGenerations {
		req.Header.Set("X-DashScope-Async", "enable")
	}
	if a.meta.Config.Plugin != "" {
		req.Header.Set("X-DashScope-Plugin", a.meta.Config.Plugin)
	}
	return nil
}

func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *model.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	switch relayMode {
	case relaymode.ImagesGenerations:
		aliRequest := ConvertRequest(*request)
		return aliRequest, nil
	default:
		compatibleAdaptor := openaiadaptor.Adaptor{}
		return compatibleAdaptor.ConvertRequest(c, relayMode, request)
	}
}

func (a *Adaptor) ConvertImageRequest(request *model.ImageRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	aliRequest := ConvertImageRequest(*request)
	return aliRequest, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, meta *meta.Meta, requestBody io.Reader) (*http.Response, error) {
	return adaptor.DoRequestHelper(a, c, meta, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, meta *meta.Meta) (usage *model.Usage, err *model.ErrorWithStatusCode) {
	switch meta.Mode {
	case relaymode.ImagesGenerations:
		err, usage = ImageHandler(c, resp)
	case relaymode.Embeddings:
		err, usage = relayCompatibleEmbeddingResponse(c, resp)
	case relaymode.Responses, relaymode.ChatCompletions, relaymode.Completions:
		compatibleAdaptor := openaiadaptor.Adaptor{}
		return compatibleAdaptor.DoResponse(c, resp, meta)
	default:
		if meta.IsStream {
			err, usage = StreamHandler(c, resp, meta.ActualModelName)
		} else {
			err, usage = Handler(c, resp, meta.ActualModelName)
		}
	}
	return
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return "ali"
}
