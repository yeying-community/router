package adaptor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/client"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/relay/meta"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

func SetupCommonRequestHeader(c *gin.Context, req *http.Request, meta *meta.Meta) {
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	req.Header.Set("Accept", c.Request.Header.Get("Accept"))
	if meta.IsStream && c.Request.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "text/event-stream")
	}
}

func DoRequestHelper(a Adaptor, c *gin.Context, meta *meta.Meta, requestBody io.Reader) (*http.Response, error) {
	fullRequestURL, err := a.GetRequestURL(meta)
	if err != nil {
		return nil, fmt.Errorf("get request url failed: %w", err)
	}
	req, err := http.NewRequest(c.Request.Method, fullRequestURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("new request failed: %w", err)
	}
	err = a.SetupRequestHeader(c, req, meta)
	if err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}
	if meta.Mode == relaymode.Responses {
		headers := make(map[string]string, len(req.Header))
		for key, values := range req.Header {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
		if _, ok := headers["Authorization"]; ok {
			headers["Authorization"] = "Bearer ***"
		}
		if _, ok := headers["api-key"]; ok {
			headers["api-key"] = "***"
		}
		if _, ok := headers["Api-Key"]; ok {
			headers["Api-Key"] = "***"
		}
		b, _ := json.Marshal(headers)
		logger.ApiLogf(c.Request.Context(), "INFO", "UPSTREAM url=%s headers=%s", fullRequestURL, string(b))
	}
	resp, err := DoRequest(c, req)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	return resp, nil
}

func DoRequest(c *gin.Context, req *http.Request) (*http.Response, error) {
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("resp is nil")
	}
	_ = req.Body.Close()
	_ = c.Request.Body.Close()
	return resp, nil
}
