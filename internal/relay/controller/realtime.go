package controller

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	adminmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay"
	"github.com/yeying-community/router/internal/relay/adaptor/openai"
	volcenginerealtime "github.com/yeying-community/router/internal/relay/adaptor/volcengine/realtime"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

var realtimeUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const (
	realtimeBrowserAPIKeySubprotocolPrefix = "openai-insecure-api-key."
	realtimeOpenAIBetaSubprotocol          = "openai-beta.realtime-v1"
)

func RelayRealtimeHelper(c *gin.Context) *relaymodel.ErrorWithStatusCode {
	if c == nil || c.Request == nil {
		return openai.ErrorWrapper(fmt.Errorf("request context is nil"), "invalid_realtime_request", http.StatusBadRequest)
	}
	if !websocket.IsWebSocketUpgrade(c.Request) {
		return RelayProxyHelper(c, 0)
	}

	meta := meta.GetByContext(c)
	adaptor := relay.GetAdaptor(meta.APIType)
	if adaptor == nil {
		return openai.ErrorWrapper(fmt.Errorf("invalid api type: %d", meta.APIType), "invalid_api_type", http.StatusBadRequest)
	}
	adaptor.Init(meta)

	fullRequestURL, err := adaptor.GetRequestURL(meta)
	if err != nil {
		return openai.ErrorWrapper(err, "get_request_url_failed", http.StatusInternalServerError)
	}
	upstreamURL, err := normalizeRealtimeWebSocketURL(fullRequestURL)
	if err != nil {
		return openai.ErrorWrapper(err, "invalid_realtime_upstream_url", http.StatusInternalServerError)
	}
	upstreamURL, err = mergeRealtimeUpstreamQuery(upstreamURL, c.Request.URL, meta)
	if err != nil {
		return openai.ErrorWrapper(err, "invalid_realtime_upstream_url", http.StatusInternalServerError)
	}
	c.Set(ctxkey.UpstreamURL, upstreamURL)

	clientHeader := cloneRealtimeRequestHeaders(c.Request.Header, meta)
	upstreamSubprotocols := realtimeUpstreamSubprotocols(c.Request.Header, meta)
	dialer := websocket.Dialer{
		Subprotocols: upstreamSubprotocols,
	}
	upstreamConn, resp, err := dialer.DialContext(c.Request.Context(), upstreamURL, clientHeader)
	if err != nil {
		return wrapRealtimeDialError(err, resp)
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	clientConn, err := realtimeUpgrader.Upgrade(c.Writer, c.Request, realtimeUpgradeHeaders(upstreamConn, c.Request.Header))
	if err != nil {
		_ = upstreamConn.Close()
		return openai.ErrorWrapper(err, "upgrade_client_websocket_failed", http.StatusBadRequest)
	}

	recordRealtimeUnmeteredProxyLog(c, meta, upstreamURL)
	pumpRealtimeConnection(c, clientConn, upstreamConn)
	return nil
}

func buildRealtimeUnmeteredProxyLog(relayMeta *meta.Meta, upstreamURL string) *adminmodel.Log {
	if relayMeta == nil {
		return nil
	}
	modelName := strings.TrimSpace(relayMeta.ActualModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(relayMeta.OriginModelName)
	}
	entry := &adminmodel.Log{
		UserId:                strings.TrimSpace(relayMeta.UserId),
		GroupId:               strings.TrimSpace(relayMeta.Group),
		ChannelId:             strings.TrimSpace(relayMeta.ChannelId),
		ModelName:             modelName,
		TokenName:             strings.TrimSpace(relayMeta.TokenName),
		Quota:                 0,
		BillingSource:         adminmodel.ResolveConsumeLogBillingSource(true),
		BillingUsageSource:    billingUsageSourceWebsocketProxy,
		BillingEstimateSource: billingEstimateSourceRealtimeUnmeteredProxy,
		BillingSettlementMode: billingSettlementModeRealtimeUnmeteredProxy,
		BillingChargeAmount:   0,
		Content:               "realtime websocket proxy connected; usage metering is not implemented yet; upstream_url=" + strings.TrimSpace(upstreamURL),
		IsStream:              true,
		ElapsedTime:           helper.CalcElapsedTime(relayMeta.StartTime),
	}
	applyRouteObservabilityToLog(entry, relayMeta, modelName)
	return entry
}

func recordRealtimeUnmeteredProxyLog(c *gin.Context, relayMeta *meta.Meta, upstreamURL string) {
	if c == nil || relayMeta == nil {
		return
	}
	entry := buildRealtimeUnmeteredProxyLog(relayMeta, upstreamURL)
	if entry == nil {
		return
	}
	adminmodel.RecordConsumeLog(c.Request.Context(), entry)
	adminmodel.UpdateUserUsedQuotaAndRequestCount(relayMeta.UserId, 0)
	adminmodel.UpdateChannelUsedQuota(relayMeta.ChannelId, 0)
}

func normalizeRealtimeWebSocketURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "https":
		parsed.Scheme = "wss"
	case "http":
		parsed.Scheme = "ws"
	case "wss", "ws":
	default:
		return "", fmt.Errorf("unsupported upstream scheme: %s", parsed.Scheme)
	}
	return parsed.String(), nil
}

func mergeRealtimeUpstreamQuery(upstreamURL string, requestURL *url.URL, relayMeta *meta.Meta) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(upstreamURL))
	if err != nil {
		return "", err
	}
	if relayMeta != nil && relaychannel.IsVolcengineRealtimeRequest(
		relayMeta.ChannelProtocol,
		relayMeta.UpstreamRequestPath,
		relayMeta.RequestURLPath,
	) {
		return parsed.String(), nil
	}
	query := parsed.Query()
	if requestURL != nil {
		for key, values := range requestURL.Query() {
			if key == "" || key == "model" || query.Has(key) {
				continue
			}
			for _, value := range values {
				query.Add(key, value)
			}
		}
	}
	if relayMeta != nil {
		modelName := strings.TrimSpace(relayMeta.ActualModelName)
		if modelName == "" {
			modelName = strings.TrimSpace(relayMeta.OriginModelName)
		}
		if modelName != "" {
			query.Set("model", modelName)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func cloneRealtimeRequestHeaders(header http.Header, relayMeta *meta.Meta) http.Header {
	cloned := make(http.Header, len(header))
	for key, values := range header {
		lower := strings.ToLower(strings.TrimSpace(key))
		switch lower {
		case "host", "connection", "upgrade", "sec-websocket-key", "sec-websocket-version", "sec-websocket-extensions", "sec-websocket-protocol", "authorization", "api-key":
			continue
		}
		for _, value := range values {
			cloned.Add(key, value)
		}
	}
	if relayMeta != nil {
		switch relayMeta.ChannelProtocol {
		case relaychannel.Azure:
			cloned.Set("api-key", relayMeta.APIKey)
		default:
			if relaychannel.IsVolcengineRealtimeRequest(
				relayMeta.ChannelProtocol,
				relayMeta.UpstreamRequestPath,
				relayMeta.RequestURLPath,
			) {
				volcenginerealtime.ApplyRealtimeHeaders(
					cloned,
					relayMeta.Config.AppID,
					relayMeta.APIKey,
					relayMeta.Config.ResourceID,
				)
				break
			}
			if strings.TrimSpace(relayMeta.APIKey) != "" {
				cloned.Set("Authorization", "Bearer "+relayMeta.APIKey)
			}
		}
	}
	return cloned
}

func realtimeUsesVolcengineBehavior(relayMeta *meta.Meta) bool {
	if relayMeta == nil {
		return false
	}
	return relaychannel.IsVolcengineRealtimeRequest(
		relayMeta.ChannelProtocol,
		relayMeta.UpstreamRequestPath,
		relayMeta.RequestURLPath,
	)
}

func realtimeUpstreamSubprotocols(header http.Header, relayMeta *meta.Meta) []string {
	if relayMeta != nil && (relayMeta.ChannelProtocol == relaychannel.Ali || relayMeta.ChannelProtocol == relaychannel.Zhipu) {
		return nil
	}
	protocols := make([]string, 0, 4)
	seen := map[string]struct{}{}
	for key, values := range header {
		if !strings.EqualFold(strings.TrimSpace(key), "Sec-WebSocket-Protocol") {
			continue
		}
		for _, rawProtocolHeader := range values {
			for _, rawProtocol := range strings.Split(rawProtocolHeader, ",") {
				protocol := strings.TrimSpace(rawProtocol)
				if protocol == "" {
					continue
				}
				if strings.HasPrefix(protocol, realtimeBrowserAPIKeySubprotocolPrefix) {
					continue
				}
				if realtimeUsesVolcengineBehavior(relayMeta) && protocol == realtimeOpenAIBetaSubprotocol {
					continue
				}
				if _, ok := seen[protocol]; ok {
					continue
				}
				seen[protocol] = struct{}{}
				protocols = append(protocols, protocol)
			}
		}
	}
	return protocols
}

func realtimeDownstreamSubprotocol(header http.Header) string {
	fallback := ""
	for key, values := range header {
		if !strings.EqualFold(strings.TrimSpace(key), "Sec-WebSocket-Protocol") {
			continue
		}
		for _, rawProtocolHeader := range values {
			for _, rawProtocol := range strings.Split(rawProtocolHeader, ",") {
				protocol := strings.TrimSpace(rawProtocol)
				if protocol == "" || strings.HasPrefix(protocol, realtimeBrowserAPIKeySubprotocolPrefix) {
					continue
				}
				if protocol == "realtime" {
					return protocol
				}
				if fallback == "" {
					fallback = protocol
				}
			}
		}
	}
	return fallback
}

func realtimeUpgradeHeaders(upstreamConn *websocket.Conn, requestHeader http.Header) http.Header {
	header := http.Header{}
	if upstreamConn != nil {
		if subprotocol := strings.TrimSpace(upstreamConn.Subprotocol()); subprotocol != "" {
			header.Set("Sec-WebSocket-Protocol", subprotocol)
			return header
		}
	}
	if subprotocol := realtimeDownstreamSubprotocol(requestHeader); subprotocol != "" {
		header.Set("Sec-WebSocket-Protocol", subprotocol)
	}
	return header
}

func pumpRealtimeConnection(c *gin.Context, clientConn *websocket.Conn, upstreamConn *websocket.Conn) {
	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = upstreamConn.Close()
			_ = clientConn.Close()
		})
	}
	defer closeBoth()

	errCh := make(chan error, 2)
	go proxyRealtimeFrames(errCh, upstreamConn, clientConn, "upstream_to_client")
	go proxyRealtimeFrames(errCh, clientConn, upstreamConn, "client_to_upstream")

	err := <-errCh
	if err != nil && c != nil {
		logger.Warnf(c.Request.Context(), "[realtime_proxy] channel=%s model=%s err=%v", strings.TrimSpace(c.GetString(ctxkey.ChannelId)), strings.TrimSpace(c.GetString(ctxkey.RequestModel)), err)
	}
}

func proxyRealtimeFrames(errCh chan<- error, dst *websocket.Conn, src *websocket.Conn, direction string) {
	for {
		messageType, reader, err := src.NextReader()
		if err != nil {
			errCh <- fmt.Errorf("%s read failed: %w", direction, err)
			return
		}
		writer, err := dst.NextWriter(messageType)
		if err != nil {
			errCh <- fmt.Errorf("%s next writer failed: %w", direction, err)
			return
		}
		if _, err := io.Copy(writer, reader); err != nil {
			_ = writer.Close()
			errCh <- fmt.Errorf("%s copy failed: %w", direction, err)
			return
		}
		if err := writer.Close(); err != nil {
			errCh <- fmt.Errorf("%s writer close failed: %w", direction, err)
			return
		}
	}
}

func wrapRealtimeDialError(err error, resp *http.Response) *relaymodel.ErrorWithStatusCode {
	if resp == nil {
		return openai.ErrorWrapper(err, "dial_realtime_upstream_failed", http.StatusBadGateway)
	}
	statusCode := resp.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusBadGateway
	}
	return openai.ErrorWrapper(err, "dial_realtime_upstream_failed", statusCode)
}
