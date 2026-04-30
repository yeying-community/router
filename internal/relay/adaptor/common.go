package adaptor

import (
	"bytes"
	"context"
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
	metaUserID := ""
	metaGroupID := ""
	metaChannelID := ""
	metaModelName := ""
	if meta != nil {
		metaUserID = strings.TrimSpace(meta.UserId)
		metaGroupID = strings.TrimSpace(meta.Group)
		metaChannelID = strings.TrimSpace(meta.ChannelId)
		metaModelName = strings.TrimSpace(meta.ActualModelName)
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
	if metaChannelID != "" {
		headers, _ := json.Marshal(maskHeaders(req.Header))
		logger.Debugf(
			c.Request.Context(),
			"[upstream_req] method=%s url=%s channel_id=%s model=%s headers=%s",
			req.Method,
			fullRequestURL,
			metaChannelID,
			metaModelName,
			string(headers),
		)
	}
	resp, err := DoRequest(c, req)
	if err != nil {
		fields := relaylogging.NewFields("UPSTREAM_ERR").
			String("method", req.Method).
			String("url", fullRequestURL).
			String("user_id", metaUserID).
			String("group", metaGroupID).
			String("channel_id", metaChannelID).
			String("model", metaModelName).
			String("endpoint", c.Request.URL.Path).
			String("error", err.Error())
		if isRequestContextCanceled(err) {
			fields.String("reason", "context_canceled")
			logger.RelayWarnf(c.Request.Context(), fields.Build())
		} else if errors.Is(err, context.DeadlineExceeded) {
			fields.String("reason", "deadline_exceeded")
			logger.RelayErrorf(c.Request.Context(), fields.Build())
		} else {
			logger.RelayErrorf(c.Request.Context(), fields.Build())
		}
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	c.Set(ctxkey.UpstreamStatus, resp.StatusCode)
	respFields := relaylogging.NewFields("UPSTREAM_RESP").
		String("method", req.Method).
		String("url", fullRequestURL).
		String("user_id", metaUserID).
		String("group", metaGroupID).
		String("channel_id", metaChannelID).
		String("model", metaModelName).
		String("endpoint", c.Request.URL.Path).
		Int("status", resp.StatusCode)
	if resp.StatusCode >= http.StatusBadRequest {
		contentType, bodyPreview := captureUpstreamErrorPreview(resp)
		respFields.String("content_type", contentType)
		respFields.String("body_preview", bodyPreview)
	}
	switch {
	case resp.StatusCode >= http.StatusInternalServerError:
		logger.RelayErrorf(c.Request.Context(), respFields.Build())
	case resp.StatusCode >= http.StatusBadRequest:
		logger.RelayWarnf(c.Request.Context(), respFields.Build())
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

func captureUpstreamErrorPreview(resp *http.Response) (string, string) {
	if resp == nil || resp.Body == nil {
		return "", ""
	}
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		resp.Body = io.NopCloser(bytes.NewBuffer(nil))
		return strings.TrimSpace(resp.Header.Get("Content-Type")), fmt.Sprintf("read_body_failed: %s", err.Error())
	}
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewBuffer(rawBody))
	return strings.TrimSpace(resp.Header.Get("Content-Type")), previewRelayResponseBody(rawBody)
}

func previewRelayResponseBody(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}
	normalized := strings.Join(strings.Fields(trimmed), " ")
	if len(normalized) > 480 {
		return normalized[:480] + "..."
	}
	return normalized
}

func isRequestContextCanceled(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(err.Error())), "context canceled")
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
