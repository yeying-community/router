package controller

import (
	"bytes"
	"encoding/json"
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
	"github.com/yeying-community/router/internal/relay/billing"
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

	usage := newRealtimeUsageObserver()
	pumpRealtimeConnection(c, clientConn, upstreamConn, usage)
	recordRealtimeProxyLog(c, meta, upstreamURL, usage.Usage())
	return nil
}

func buildRealtimeProxyLog(relayMeta *meta.Meta, upstreamURL string, usage *relaymodel.Usage) *adminmodel.Log {
	if relayMeta == nil {
		return nil
	}
	modelName := strings.TrimSpace(relayMeta.ActualModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(relayMeta.OriginModelName)
	}
	ratioModel := strings.TrimSpace(relayMeta.OriginModelName)
	if ratioModel == "" {
		ratioModel = modelName
	}
	billingRatio := adminmodel.GetRouteBillingRatio(relayMeta.Group, ratioModel, relayMeta.ChannelId)
	entry := &adminmodel.Log{
		UserId:                   strings.TrimSpace(relayMeta.UserId),
		GroupId:                  strings.TrimSpace(relayMeta.Group),
		ChannelId:                strings.TrimSpace(relayMeta.ChannelId),
		ModelName:                modelName,
		TokenName:                strings.TrimSpace(relayMeta.TokenName),
		Quota:                    0,
		BillingSource:            adminmodel.ResolveConsumeLogBillingSource(true),
		BillingUsageSource:       billingUsageSourceWebsocketProxy,
		BillingEstimateSource:    billingEstimateSourceRealtimeUnmeteredProxy,
		BillingSettlementMode:    billingSettlementModeRealtimeUnmeteredProxy,
		BillingEffectiveRatio:    billingRatio.EffectiveRatio,
		BillingGroupChannelRatio: billingRatio.GroupChannelRatio,
		BillingModelChannelRatio: billingRatio.ModelChannelRatio,
		BillingChargeAmount:      0,
		Content:                  "realtime websocket proxy connected; usage metering is not implemented yet; upstream_url=" + strings.TrimSpace(upstreamURL),
		IsStream:                 true,
		ElapsedTime:              helper.CalcElapsedTime(relayMeta.StartTime),
	}
	applyRouteObservabilityToLog(entry, relayMeta, modelName)
	if usage == nil || usage.PromptTokens+usage.CompletionTokens <= 0 {
		return entry
	}
	groupRatio := billingRatio.EffectiveRatio
	pricing, err := adminmodel.ResolveChannelModelPricing(relayMeta.ChannelProtocol, relayMeta.ChannelModelConfigs, modelName)
	if err != nil {
		if groupRatio == 0 {
			pricing = adminmodel.ResolvedModelPricing{
				Model:     modelName,
				Type:      adminmodel.InferModelType(modelName),
				PriceUnit: adminmodel.ProviderPriceUnitPer1KTokens,
				Currency:  adminmodel.ProviderPriceCurrencyUSD,
				Source:    "group_free",
			}
		} else {
			entry.Content = "realtime websocket proxy connected; usage returned but pricing is not configured; upstream_url=" + strings.TrimSpace(upstreamURL)
			return entry
		}
	}
	pricing = adminmodel.ResolveTextRequestPricing(pricing, relayMeta.UpstreamRequestPath)
	pricing = adminmodel.ResolveTextUsagePricing(pricing, relayMeta.UpstreamRequestPath, usage.PromptTokens, usage.CompletionTokens)
	snapshot, err := billing.ComputeTextBillingSnapshotWithUsage(*usage, pricing, groupRatio)
	if err != nil {
		entry.Content = "realtime websocket proxy connected; usage returned but billing snapshot failed; upstream_url=" + strings.TrimSpace(upstreamURL)
		return entry
	}
	snapshot.SetBillingRatioBreakdown(billingRatio)
	snapshot.PricingSource = strings.TrimSpace(pricing.Source)
	snapshot.UsageSource = billingUsageSourceUpstreamUsage
	snapshot.EstimateSource = billingEstimateSourceRealtimeUpstreamUsage
	snapshot.SettlementMode = billingSettlementModeRealtimeUsageFinal
	contentSuffix := "realtime websocket usage metered; upstream_url=" + strings.TrimSpace(upstreamURL)
	if err := billing.ApplyEstimatedProcurementCostFloor(&snapshot, relayMeta.ChannelId, modelName); err != nil {
		contentSuffix = "realtime websocket usage metered; procurement floor unavailable; upstream_url=" + strings.TrimSpace(upstreamURL)
	}
	entry.PromptTokens = usage.PromptTokens
	entry.CompletionTokens = usage.CompletionTokens
	entry.Quota = int(snapshot.ChargeAmount)
	entry.BillingSource = adminmodel.ResolveConsumeLogBillingSource(true)
	entry.Content = buildTextBillingLogContent(pricing, groupRatio, contentSuffix)
	snapshot.ApplyToLog(entry)
	return entry
}

func buildRealtimeUnmeteredProxyLog(relayMeta *meta.Meta, upstreamURL string) *adminmodel.Log {
	return buildRealtimeProxyLog(relayMeta, upstreamURL, nil)
}

func recordRealtimeProxyLog(c *gin.Context, relayMeta *meta.Meta, upstreamURL string, usage *relaymodel.Usage) {
	if c == nil || relayMeta == nil {
		return
	}
	entry := buildRealtimeProxyLog(relayMeta, upstreamURL, usage)
	if entry == nil {
		return
	}
	balanceSource := adminmodel.LogBillingSourceSnapshot{}
	if entry.Quota > 0 {
		if consumeResult, err := adminmodel.ConsumeUserBalanceLotsForGroupDetailed(relayMeta.UserId, relayMeta.Group, int64(entry.Quota)); err != nil {
			logger.Errorf(c.Request.Context(), "realtime billing lots consume failed code=consume_user_balance_lots_failed user_id=%s group=%s channel_id=%s model=%s total_quota=%d err=%q", strings.TrimSpace(relayMeta.UserId), strings.TrimSpace(relayMeta.Group), strings.TrimSpace(relayMeta.ChannelId), strings.TrimSpace(entry.ModelName), entry.Quota, err.Error())
		} else {
			balanceSource = consumeResult.LogBillingSourceSnapshot()
			if consumeResult.ConsumedAmount < int64(entry.Quota) {
				logger.Warnf(c.Request.Context(), "realtime billing lots consume partial user_id=%s group=%s channel_id=%s model=%s consumed=%d requested=%d", strings.TrimSpace(relayMeta.UserId), strings.TrimSpace(relayMeta.Group), strings.TrimSpace(relayMeta.ChannelId), strings.TrimSpace(entry.ModelName), consumeResult.ConsumedAmount, entry.Quota)
			}
		}
		if err := adminmodel.CacheUpdateUserQuota(c.Request.Context(), relayMeta.UserId); err != nil {
			logger.Errorf(c.Request.Context(), "realtime billing cache update failed code=update_user_quota_cache_failed user_id=%s group=%s channel_id=%s model=%s total_quota=%d err=%q", strings.TrimSpace(relayMeta.UserId), strings.TrimSpace(relayMeta.Group), strings.TrimSpace(relayMeta.ChannelId), strings.TrimSpace(entry.ModelName), entry.Quota, err.Error())
		}
		if err := adminmodel.CacheUpdateUserQuotaForGroup(c.Request.Context(), relayMeta.UserId, relayMeta.Group); err != nil {
			logger.Errorf(c.Request.Context(), "realtime billing group cache update failed code=update_user_group_quota_cache_failed user_id=%s group=%s channel_id=%s model=%s total_quota=%d err=%q", strings.TrimSpace(relayMeta.UserId), strings.TrimSpace(relayMeta.Group), strings.TrimSpace(relayMeta.ChannelId), strings.TrimSpace(entry.ModelName), entry.Quota, err.Error())
		}
	}
	adminmodel.ApplyConsumeLogBillingSource(entry, true, adminmodel.LogBillingSourceSnapshot{}, balanceSource)
	billing.ApplyProcurementCostObservation(entry)
	adminmodel.RecordConsumeLog(c.Request.Context(), entry)
	billing.RecordProcurementConsumptionObservation(c.Request.Context(), entry)
	adminmodel.UpdateUserUsedQuotaAndRequestCount(relayMeta.UserId, int64(entry.Quota))
	adminmodel.UpdateChannelUsedQuota(relayMeta.ChannelId, int64(entry.Quota))
	consumeTokenRequestCount(c.Request.Context(), relayMeta.TokenId, 1)
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

func pumpRealtimeConnection(c *gin.Context, clientConn *websocket.Conn, upstreamConn *websocket.Conn, usage *realtimeUsageObserver) {
	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = upstreamConn.Close()
			_ = clientConn.Close()
		})
	}
	defer closeBoth()

	errCh := make(chan error, 2)
	go proxyRealtimeFrames(errCh, upstreamConn, clientConn, "upstream_to_client", usage)
	go proxyRealtimeFrames(errCh, clientConn, upstreamConn, "client_to_upstream", nil)

	err := <-errCh
	if err != nil && c != nil {
		logger.Warnf(c.Request.Context(), "[realtime_proxy] channel=%s model=%s err=%v", strings.TrimSpace(c.GetString(ctxkey.ChannelId)), strings.TrimSpace(c.GetString(ctxkey.RequestModel)), err)
	}
}

func proxyRealtimeFrames(errCh chan<- error, dst *websocket.Conn, src *websocket.Conn, direction string, usage *realtimeUsageObserver) {
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
		if usage != nil && messageType == websocket.TextMessage {
			payload, readErr := io.ReadAll(reader)
			if readErr != nil {
				_ = writer.Close()
				errCh <- fmt.Errorf("%s read message failed: %w", direction, readErr)
				return
			}
			usage.Observe(payload)
			if _, err := io.Copy(writer, bytes.NewReader(payload)); err != nil {
				_ = writer.Close()
				errCh <- fmt.Errorf("%s copy failed: %w", direction, err)
				return
			}
		} else if _, err := io.Copy(writer, reader); err != nil {
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

type realtimeUsageObserver struct {
	mu    sync.Mutex
	usage relaymodel.Usage
}

func newRealtimeUsageObserver() *realtimeUsageObserver {
	return &realtimeUsageObserver{}
}

func (observer *realtimeUsageObserver) Observe(payload []byte) {
	if observer == nil || len(payload) == 0 {
		return
	}
	usage, ok := extractRealtimeUsage(payload)
	if !ok {
		return
	}
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.usage.PromptTokens += usage.PromptTokens
	observer.usage.CompletionTokens += usage.CompletionTokens
	observer.usage.TotalTokens += usage.TotalTokens
	observer.usage.ImageGenerationCalls += usage.ImageGenerationCalls
	if usage.PromptTokensDetails != nil {
		if observer.usage.PromptTokensDetails == nil {
			observer.usage.PromptTokensDetails = &relaymodel.PromptTokensDetails{}
		}
		observer.usage.PromptTokensDetails.CachedTokens += usage.PromptTokensDetails.CachedTokens
		observer.usage.PromptTokensDetails.CacheReadTokens += usage.PromptTokensDetails.CacheReadTokens
		observer.usage.PromptTokensDetails.CacheCreationTokens += usage.PromptTokensDetails.CacheCreationTokens
	}
}

func (observer *realtimeUsageObserver) Usage() *relaymodel.Usage {
	if observer == nil {
		return nil
	}
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if observer.usage.PromptTokens+observer.usage.CompletionTokens <= 0 {
		return nil
	}
	usage := observer.usage
	if observer.usage.PromptTokensDetails != nil {
		details := *observer.usage.PromptTokensDetails
		usage.PromptTokensDetails = &details
	}
	return &usage
}

type realtimeUsageEnvelope struct {
	Usage    *realtimeUsagePayload `json:"usage"`
	Response *struct {
		Usage *realtimeUsagePayload `json:"usage"`
	} `json:"response"`
}

type realtimeUsagePayload struct {
	PromptTokens            int                                 `json:"prompt_tokens"`
	InputTokens             int                                 `json:"input_tokens"`
	CompletionTokens        int                                 `json:"completion_tokens"`
	OutputTokens            int                                 `json:"output_tokens"`
	TotalTokens             int                                 `json:"total_tokens"`
	PromptTokensDetails     *relaymodel.PromptTokensDetails     `json:"prompt_tokens_details"`
	InputTokenDetails       *relaymodel.PromptTokensDetails     `json:"input_token_details"`
	CompletionTokensDetails *relaymodel.CompletionTokensDetails `json:"completion_tokens_details"`
	OutputTokenDetails      *relaymodel.CompletionTokensDetails `json:"output_token_details"`
}

func extractRealtimeUsage(payload []byte) (relaymodel.Usage, bool) {
	envelope := realtimeUsageEnvelope{}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return relaymodel.Usage{}, false
	}
	usagePayload := envelope.Usage
	if usagePayload == nil && envelope.Response != nil {
		usagePayload = envelope.Response.Usage
	}
	if usagePayload == nil {
		return relaymodel.Usage{}, false
	}
	usage := relaymodel.Usage{
		PromptTokens:     firstPositiveInt(usagePayload.PromptTokens, usagePayload.InputTokens),
		CompletionTokens: firstPositiveInt(usagePayload.CompletionTokens, usagePayload.OutputTokens),
		TotalTokens:      usagePayload.TotalTokens,
	}
	if usage.TotalTokens <= 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	if usagePayload.PromptTokensDetails != nil {
		details := *usagePayload.PromptTokensDetails
		usage.PromptTokensDetails = &details
	} else if usagePayload.InputTokenDetails != nil {
		details := *usagePayload.InputTokenDetails
		usage.PromptTokensDetails = &details
	}
	if usagePayload.CompletionTokensDetails != nil {
		details := *usagePayload.CompletionTokensDetails
		usage.CompletionTokensDetails = &details
	} else if usagePayload.OutputTokenDetails != nil {
		details := *usagePayload.OutputTokenDetails
		usage.CompletionTokensDetails = &details
	}
	return usage, usage.PromptTokens+usage.CompletionTokens > 0
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
