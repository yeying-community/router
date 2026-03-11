package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
	relaymeta "github.com/yeying-community/router/internal/relay/meta"
	"github.com/yeying-community/router/internal/relay/model"
)

type GeneralErrorResponse struct {
	Error    model.Error `json:"error"`
	Message  string      `json:"message"`
	Msg      string      `json:"msg"`
	Err      string      `json:"err"`
	ErrorMsg string      `json:"error_msg"`
	Header   struct {
		Message string `json:"message"`
	} `json:"header"`
	Response struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"response"`
}

func (e GeneralErrorResponse) ToMessage() string {
	if e.Error.Message != "" {
		return e.Error.Message
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Msg != "" {
		return e.Msg
	}
	if e.Err != "" {
		return e.Err
	}
	if e.ErrorMsg != "" {
		return e.ErrorMsg
	}
	if e.Header.Message != "" {
		return e.Header.Message
	}
	if e.Response.Error.Message != "" {
		return e.Response.Error.Message
	}
	return ""
}

func RelayErrorHandler(meta *relaymeta.Meta, resp *http.Response) (ErrorWithStatusCode *model.ErrorWithStatusCode) {
	if resp == nil {
		return &model.ErrorWithStatusCode{
			StatusCode: 500,
			Error: model.Error{
				Message: "resp is nil",
				Type:    "upstream_error",
				Code:    "bad_response",
			},
		}
	}
	ErrorWithStatusCode = &model.ErrorWithStatusCode{
		StatusCode: resp.StatusCode,
		Error: model.Error{
			Message: "",
			Type:    "upstream_error",
			Code:    "bad_response_status_code",
			Param:   strconv.Itoa(resp.StatusCode),
		},
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	if config.DebugEnabled {
		logger.SysLog(fmt.Sprintf("error happened, status code: %d, response: \n%s", resp.StatusCode, string(responseBody)))
	}
	logUpstreamFailure(meta, resp.StatusCode, responseBody)
	err = resp.Body.Close()
	if err != nil {
		return
	}
	var errResponse GeneralErrorResponse
	err = json.Unmarshal(responseBody, &errResponse)
	if err != nil {
		return
	}
	if errResponse.Error.Message != "" {
		// OpenAI format error, so we override the default one
		ErrorWithStatusCode.Error = errResponse.Error
	} else {
		ErrorWithStatusCode.Error.Message = errResponse.ToMessage()
	}
	if ErrorWithStatusCode.Error.Message == "" {
		ErrorWithStatusCode.Error.Message = fmt.Sprintf("bad response status code %d", resp.StatusCode)
	}
	return
}

func logUpstreamFailure(meta *relaymeta.Meta, statusCode int, responseBody []byte) {
	if meta == nil {
		return
	}
	if statusCode != http.StatusTooManyRequests && statusCode != http.StatusServiceUnavailable {
		return
	}
	bodyPreview := strings.TrimSpace(string(responseBody))
	if len(bodyPreview) > 1000 {
		bodyPreview = bodyPreview[:1000]
	}
	upstreamPath := strings.TrimSpace(meta.UpstreamRequestPath)
	if upstreamPath == "" {
		upstreamPath = strings.TrimSpace(meta.RequestURLPath)
	}
	logger.SysWarnf(
		"[upstream_error] status=%d channel_id=%s protocol=%d model=%s upstream_path=%s base_url=%s body=%s",
		statusCode,
		strings.TrimSpace(meta.ChannelId),
		meta.ChannelProtocol,
		strings.TrimSpace(meta.ActualModelName),
		upstreamPath,
		strings.TrimSpace(meta.BaseURL),
		bodyPreview,
	)
}
