package adaptor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/client"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/logger"
	relaylogging "github.com/yeying-community/router/internal/relay/logging"
	"github.com/yeying-community/router/internal/relay/meta"
)

func SetupCommonRequestHeader(c *gin.Context, req *http.Request, meta *meta.Meta) {
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	req.Header.Set("Accept", c.Request.Header.Get("Accept"))
	if userAgent := strings.TrimSpace(c.Request.Header.Get("User-Agent")); userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if meta.IsStream && c.Request.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "text/event-stream")
	}
}

func DoRequestHelper(a Adaptor, c *gin.Context, meta *meta.Meta, requestBody io.Reader) (*http.Response, error) {
	fullRequestURL, err := a.GetRequestURL(meta)
	if err != nil {
		return nil, fmt.Errorf("get request url failed: %w", err)
	}
	c.Set(ctxkey.UpstreamURL, fullRequestURL)
	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, fullRequestURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("new request failed: %w", err)
	}
	err = a.SetupRequestHeader(c, req, meta)
	if err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}
	if meta != nil && meta.ChannelId != "" {
		headers, _ := json.Marshal(maskHeaders(req.Header))
		logger.Debugf(
			c.Request.Context(),
			"[upstream_req] method=%s url=%s channel_id=%s model=%s headers=%s",
			req.Method,
			fullRequestURL,
			strings.TrimSpace(meta.ChannelId),
			strings.TrimSpace(meta.ActualModelName),
			string(headers),
		)
	}
	resp, err := DoRequest(c, req)
	if err != nil {
		logger.RelayErrorf(c.Request.Context(), relaylogging.NewFields("UPSTREAM_ERR").
			String("method", req.Method).
			String("url", fullRequestURL).
			String("error", err.Error()).
			Build())
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	c.Set(ctxkey.UpstreamStatus, resp.StatusCode)
	respFields := relaylogging.NewFields("UPSTREAM_RESP").
		String("method", req.Method).
		String("url", fullRequestURL).
		Int("status", resp.StatusCode).
		Build()
	switch {
	case resp.StatusCode >= http.StatusInternalServerError:
		logger.RelayErrorf(c.Request.Context(), respFields)
	case resp.StatusCode >= http.StatusBadRequest:
		logger.RelayWarnf(c.Request.Context(), respFields)
	}
	return resp, nil
}

func maskHeaders(header http.Header) map[string]string {
	masked := make(map[string]string, len(header))
	for key, values := range header {
		if len(values) == 0 {
			continue
		}
		if isSensitiveHeader(key) {
			if strings.EqualFold(key, "Authorization") && strings.HasPrefix(strings.ToLower(values[0]), "bearer ") {
				masked[key] = "Bearer ***"
			} else {
				masked[key] = "***"
			}
			continue
		}
		masked[key] = values[0]
	}
	return masked
}

func isSensitiveHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "authorization", "api-key", "x-api-key":
		return true
	default:
		return false
	}
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
