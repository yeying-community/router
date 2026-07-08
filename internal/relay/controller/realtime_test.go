package controller

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	adminmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/billing"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

func TestNormalizeRealtimeWebSocketURL(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "https://api.openai.com/v1/realtime?model=gpt-realtime-2", want: "wss://api.openai.com/v1/realtime?model=gpt-realtime-2"},
		{raw: "http://localhost:8080/v1/realtime", want: "ws://localhost:8080/v1/realtime"},
	}
	for _, tt := range tests {
		got, err := normalizeRealtimeWebSocketURL(tt.raw)
		if err != nil {
			t.Fatalf("normalizeRealtimeWebSocketURL(%q) returned error: %v", tt.raw, err)
		}
		if got != tt.want {
			t.Fatalf("normalizeRealtimeWebSocketURL(%q)=%q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestMergeRealtimeUpstreamQueryUsesResolvedModel(t *testing.T) {
	requestURL, err := url.Parse("/v1/realtime?model=qwen3.5-omni-plus-realtime&foo=bar")
	if err != nil {
		t.Fatal(err)
	}
	got, err := mergeRealtimeUpstreamQuery(
		"wss://dashscope.aliyuncs.com/api-ws/v1/realtime",
		requestURL,
		&meta.Meta{ChannelProtocol: relaychannel.Ali, ActualModelName: "qwen-upstream-realtime"},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "wss://dashscope.aliyuncs.com/api-ws/v1/realtime?foo=bar&model=qwen-upstream-realtime"
	if got != want {
		t.Fatalf("mergeRealtimeUpstreamQuery()=%q, want %q", got, want)
	}
}

func TestMergeRealtimeUpstreamQueryAddsResolvedModel(t *testing.T) {
	got, err := mergeRealtimeUpstreamQuery(
		"wss://api.openai.com/v1/realtime",
		&url.URL{Path: "/v1/realtime"},
		&meta.Meta{ChannelProtocol: relaychannel.OpenAI, ActualModelName: "gpt-realtime-2"},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "wss://api.openai.com/v1/realtime?model=gpt-realtime-2"
	if got != want {
		t.Fatalf("mergeRealtimeUpstreamQuery()=%q, want %q", got, want)
	}
}

func TestMergeRealtimeUpstreamQuerySkipsCanonicalVolcengineRealtime(t *testing.T) {
	requestURL, err := url.Parse("/v1/realtime?model=gpt-realtime-2")
	if err != nil {
		t.Fatal(err)
	}
	got, err := mergeRealtimeUpstreamQuery(
		"wss://openspeech.bytedance.com/api/v3/realtime/dialogue",
		requestURL,
		&meta.Meta{
			ChannelProtocol: relaychannel.VolcEngine,
			RequestURLPath:  adminmodel.ChannelModelEndpointRealtime,
			ActualModelName: "gpt-realtime-2",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "wss://openspeech.bytedance.com/api/v3/realtime/dialogue"
	if got != want {
		t.Fatalf("mergeRealtimeUpstreamQuery()=%q, want %q", got, want)
	}
}

func TestBuildRealtimeUnmeteredProxyLog(t *testing.T) {
	start := time.Now().Add(-100 * time.Millisecond)
	entry := buildRealtimeUnmeteredProxyLog(&meta.Meta{
		UserId:              "user-1",
		Group:               "group-1",
		ChannelId:           "channel-1",
		TokenName:           "prod-token",
		OriginModelName:     "gpt-realtime-2",
		ActualModelName:     "gpt-realtime-upstream",
		ChannelProtocol:     relaychannel.OpenAI,
		RequestURLPath:      adminmodel.ChannelModelEndpointRealtime,
		UpstreamRequestPath: adminmodel.ChannelModelEndpointRealtime,
		StartTime:           start,
	}, "wss://api.openai.com/v1/realtime?model=gpt-realtime-upstream")
	if entry == nil {
		t.Fatal("buildRealtimeUnmeteredProxyLog() returned nil")
	}
	if entry.Quota != 0 || entry.BillingChargeAmount != 0 {
		t.Fatalf("quota fields = quota:%d charge:%d, want zero", entry.Quota, entry.BillingChargeAmount)
	}
	if entry.BillingUsageSource != billingUsageSourceWebsocketProxy {
		t.Fatalf("BillingUsageSource = %q, want %q", entry.BillingUsageSource, billingUsageSourceWebsocketProxy)
	}
	if entry.BillingEstimateSource != billingEstimateSourceRealtimeUnmeteredProxy {
		t.Fatalf("BillingEstimateSource = %q, want %q", entry.BillingEstimateSource, billingEstimateSourceRealtimeUnmeteredProxy)
	}
	if entry.BillingSettlementMode != billingSettlementModeRealtimeUnmeteredProxy {
		t.Fatalf("BillingSettlementMode = %q, want %q", entry.BillingSettlementMode, billingSettlementModeRealtimeUnmeteredProxy)
	}
	billing.ApplyProcurementCostObservation(entry)
	if entry.BillingSettlementTruthMode != billing.SettlementTruthModeUnmeteredProxy {
		t.Fatalf("BillingSettlementTruthMode = %q, want %q", entry.BillingSettlementTruthMode, billing.SettlementTruthModeUnmeteredProxy)
	}
	if entry.BillingProcurementCostConfidence != billing.ProcurementCostConfidenceUnmetered {
		t.Fatalf("BillingProcurementCostConfidence = %q, want %q", entry.BillingProcurementCostConfidence, billing.ProcurementCostConfidenceUnmetered)
	}
	if entry.ModelName != "gpt-realtime-upstream" || entry.RequestModelName != "gpt-realtime-2" || entry.ActualModelName != "gpt-realtime-upstream" {
		t.Fatalf("model fields = model:%q request:%q actual:%q", entry.ModelName, entry.RequestModelName, entry.ActualModelName)
	}
	if entry.UpstreamEndpoint != adminmodel.ChannelModelEndpointRealtime {
		t.Fatalf("UpstreamEndpoint = %q, want %q", entry.UpstreamEndpoint, adminmodel.ChannelModelEndpointRealtime)
	}
	if !entry.IsStream {
		t.Fatal("IsStream = false, want true")
	}
	if !strings.Contains(entry.Content, "usage metering is not implemented yet") {
		t.Fatalf("Content = %q, want unmetered note", entry.Content)
	}
}

func TestExtractRealtimeUsageSupportsResponseEnvelope(t *testing.T) {
	usage, ok := extractRealtimeUsage([]byte(`{
		"type": "response.done",
		"response": {
			"usage": {
				"input_tokens": 12,
				"output_tokens": 5,
				"total_tokens": 17,
				"input_token_details": {
					"cached_tokens": 3,
					"cache_read_tokens": 2,
					"cache_creation_tokens": 1
				}
			}
		}
	}`))
	if !ok {
		t.Fatal("extractRealtimeUsage returned ok=false")
	}
	if usage.PromptTokens != 12 || usage.CompletionTokens != 5 || usage.TotalTokens != 17 {
		t.Fatalf("usage=%+v, want prompt=12 completion=5 total=17", usage)
	}
	if usage.PromptTokensDetails == nil {
		t.Fatal("PromptTokensDetails = nil, want details")
	}
	if usage.PromptTokensDetails.CachedTokens != 3 || usage.PromptTokensDetails.CacheReadTokens != 2 || usage.PromptTokensDetails.CacheCreationTokens != 1 {
		t.Fatalf("PromptTokensDetails=%+v, want cached=3 read=2 write=1", *usage.PromptTokensDetails)
	}
}

func TestRealtimeUsageObserverAggregatesUsageEvents(t *testing.T) {
	observer := newRealtimeUsageObserver()
	observer.Observe([]byte(`{"type":"session.created"}`))
	observer.Observe([]byte(`{"type":"response.done","response":{"usage":{"input_tokens":10,"output_tokens":4,"input_token_details":{"cached_tokens":2}}}}`))
	observer.Observe([]byte(`{"type":"response.done","usage":{"prompt_tokens":6,"completion_tokens":3,"prompt_tokens_details":{"cache_read_tokens":1,"cache_creation_tokens":2}}}`))

	usage := observer.Usage()
	if usage == nil {
		t.Fatal("Usage() returned nil")
	}
	if usage.PromptTokens != 16 || usage.CompletionTokens != 7 || usage.TotalTokens != 23 {
		t.Fatalf("usage=%+v, want prompt=16 completion=7 total=23", *usage)
	}
	if usage.PromptTokensDetails == nil {
		t.Fatal("PromptTokensDetails = nil, want details")
	}
	if usage.PromptTokensDetails.CachedTokens != 2 || usage.PromptTokensDetails.CacheReadTokens != 1 || usage.PromptTokensDetails.CacheCreationTokens != 2 {
		t.Fatalf("PromptTokensDetails=%+v, want cached=2 read=1 write=2", *usage.PromptTokensDetails)
	}
}

func TestBuildRealtimeProxyLogWithReturnedUsageMetersBilling(t *testing.T) {
	inputPrice := 0.01
	outputPrice := 0.02
	entry := buildRealtimeProxyLog(&meta.Meta{
		UserId:              "user-1",
		Group:               "group-1",
		ChannelId:           "channel-1",
		TokenName:           "prod-token",
		OriginModelName:     "gpt-realtime-2",
		ActualModelName:     "gpt-realtime-upstream",
		ChannelProtocol:     relaychannel.OpenAI,
		RequestURLPath:      adminmodel.ChannelModelEndpointRealtime,
		UpstreamRequestPath: adminmodel.ChannelModelEndpointRealtime,
		StartTime:           time.Now().Add(-100 * time.Millisecond),
		ChannelModelConfigs: []adminmodel.ChannelModel{
			{
				ChannelId:   "channel-1",
				Model:       "gpt-realtime-upstream",
				Type:        adminmodel.ProviderModelTypeText,
				Selected:    true,
				InputPrice:  &inputPrice,
				OutputPrice: &outputPrice,
				PriceUnit:   adminmodel.ProviderPriceUnitPer1KTokens,
				Currency:    adminmodel.ProviderPriceCurrencyUSD,
			},
		},
	}, "wss://api.openai.com/v1/realtime?model=gpt-realtime-upstream", &relaymodel.Usage{
		PromptTokens:     1000,
		CompletionTokens: 1000,
		TotalTokens:      2000,
	})
	if entry == nil {
		t.Fatal("buildRealtimeProxyLog() returned nil")
	}
	if entry.PromptTokens != 1000 || entry.CompletionTokens != 1000 {
		t.Fatalf("token fields prompt/completion=%d/%d, want 1000/1000", entry.PromptTokens, entry.CompletionTokens)
	}
	if entry.Quota <= 0 || entry.BillingChargeAmount <= 0 {
		t.Fatalf("quota fields quota=%d charge=%d, want positive metered charge", entry.Quota, entry.BillingChargeAmount)
	}
	if entry.BillingUsageSource != billingUsageSourceUpstreamUsage {
		t.Fatalf("BillingUsageSource=%q, want %q", entry.BillingUsageSource, billingUsageSourceUpstreamUsage)
	}
	if entry.BillingEstimateSource != billingEstimateSourceRealtimeUpstreamUsage {
		t.Fatalf("BillingEstimateSource=%q, want %q", entry.BillingEstimateSource, billingEstimateSourceRealtimeUpstreamUsage)
	}
	if entry.BillingSettlementMode != billingSettlementModeRealtimeUsageFinal {
		t.Fatalf("BillingSettlementMode=%q, want %q", entry.BillingSettlementMode, billingSettlementModeRealtimeUsageFinal)
	}
	billing.ApplyProcurementCostObservation(entry)
	if entry.BillingSettlementTruthMode != billing.SettlementTruthModeReturnedUsageFinal {
		t.Fatalf("BillingSettlementTruthMode=%q, want %q", entry.BillingSettlementTruthMode, billing.SettlementTruthModeReturnedUsageFinal)
	}
	if !strings.Contains(entry.Content, "realtime websocket usage metered") {
		t.Fatalf("Content=%q, want metered note", entry.Content)
	}
}

func TestRealtimeUpgradeHeadersCopiesSubprotocol(t *testing.T) {
	header := realtimeUpgradeHeaders(nil, nil)
	if got := header.Get("Sec-WebSocket-Protocol"); got != "" {
		t.Fatalf("Sec-WebSocket-Protocol = %q, want empty", got)
	}
}

func TestRealtimeUpgradeHeadersSelectsSafeClientFallbackSubprotocol(t *testing.T) {
	requestHeader := http.Header{
		"Sec-WebSocket-Protocol": []string{"openai-insecure-api-key.user-token, realtime, openai-beta.realtime-v1"},
	}
	header := realtimeUpgradeHeaders(nil, requestHeader)
	if got := header.Get("Sec-WebSocket-Protocol"); got != "realtime" {
		t.Fatalf("Sec-WebSocket-Protocol = %q, want realtime", got)
	}
}

func TestRealtimeDownstreamSubprotocolNeverSelectsBrowserToken(t *testing.T) {
	requestHeader := http.Header{
		"Sec-WebSocket-Protocol": []string{"openai-insecure-api-key.user-token, openai-beta.realtime-v1"},
	}
	if got := realtimeDownstreamSubprotocol(requestHeader); got != "openai-beta.realtime-v1" {
		t.Fatalf("realtimeDownstreamSubprotocol = %q, want openai-beta.realtime-v1", got)
	}
}

func TestCloneRealtimeRequestHeadersDropsHopByHop(t *testing.T) {
	header := http.Header{
		"Authorization":          []string{"Bearer sk-test"},
		"OpenAI-Beta":            []string{"realtime=v1"},
		"Connection":             []string{"Upgrade"},
		"Sec-WebSocket-Key":      []string{"secret"},
		"Sec-WebSocket-Version":  []string{"13"},
		"Sec-WebSocket-Protocol": []string{"realtime, openai-insecure-api-key.user-token, openai-beta.realtime-v1"},
	}
	cloned := cloneRealtimeRequestHeaders(header, &meta.Meta{APIKey: "upstream-key"})
	if cloned.Get("Authorization") != "Bearer upstream-key" {
		t.Fatalf("Authorization = %q, want Bearer upstream-key", cloned.Get("Authorization"))
	}
	if cloned.Get("OpenAI-Beta") != "realtime=v1" {
		t.Fatalf("OpenAI-Beta = %q, want realtime=v1", cloned.Get("OpenAI-Beta"))
	}
	if cloned.Get("Connection") != "" {
		t.Fatalf("Connection = %q, want empty", cloned.Get("Connection"))
	}
	if cloned.Get("Sec-WebSocket-Key") != "" {
		t.Fatalf("Sec-WebSocket-Key = %q, want empty", cloned.Get("Sec-WebSocket-Key"))
	}
	if cloned.Get("Sec-WebSocket-Protocol") != "" {
		t.Fatalf("Sec-WebSocket-Protocol = %q, want empty", cloned.Get("Sec-WebSocket-Protocol"))
	}
}

func TestRealtimeUpstreamSubprotocolsDropsBrowserToken(t *testing.T) {
	header := http.Header{
		"Sec-WebSocket-Protocol": []string{"realtime, openai-insecure-api-key.user-token, openai-beta.realtime-v1", "realtime"},
	}
	got := realtimeUpstreamSubprotocols(header, &meta.Meta{APIKey: "upstream-key"})
	want := []string{"realtime", "openai-beta.realtime-v1"}
	if len(got) != len(want) {
		t.Fatalf("realtimeUpstreamSubprotocols length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("realtimeUpstreamSubprotocols[%d] = %q, want %q; got %#v", i, got[i], want[i], got)
		}
	}
}

func TestRealtimeUpstreamSubprotocolsForAliDropsAllClientProtocols(t *testing.T) {
	header := http.Header{
		"Sec-WebSocket-Protocol": []string{"realtime, openai-insecure-api-key.user-token, openai-beta.realtime-v1"},
	}
	if got := realtimeUpstreamSubprotocols(header, &meta.Meta{ChannelProtocol: relaychannel.Ali}); len(got) != 0 {
		t.Fatalf("realtimeUpstreamSubprotocols = %#v, want empty for ali", got)
	}
}

func TestRealtimeUpstreamSubprotocolsForZhipuDropsAllClientProtocols(t *testing.T) {
	header := http.Header{
		"Sec-WebSocket-Protocol": []string{"realtime, openai-insecure-api-key.user-token, openai-beta.realtime-v1"},
	}
	if got := realtimeUpstreamSubprotocols(header, &meta.Meta{ChannelProtocol: relaychannel.Zhipu}); len(got) != 0 {
		t.Fatalf("realtimeUpstreamSubprotocols = %#v, want empty for zhipu", got)
	}
}

func TestRealtimeUpstreamSubprotocolsForCanonicalVolcengineRealtimeDropsOpenAIBeta(t *testing.T) {
	header := http.Header{
		"Sec-WebSocket-Protocol": []string{"realtime, openai-insecure-api-key.user-token, openai-beta.realtime-v1"},
	}
	got := realtimeUpstreamSubprotocols(header, &meta.Meta{
		ChannelProtocol: relaychannel.VolcEngine,
		RequestURLPath:  adminmodel.ChannelModelEndpointRealtime,
	})
	want := []string{"realtime"}
	if len(got) != len(want) {
		t.Fatalf("realtimeUpstreamSubprotocols length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("realtimeUpstreamSubprotocols[%d] = %q, want %q; got %#v", i, got[i], want[i], got)
		}
	}
}

func TestCloneRealtimeRequestHeadersUsesCanonicalVolcengineRealtimeHeaders(t *testing.T) {
	header := http.Header{
		"Authorization":          []string{"Bearer sk-test"},
		"OpenAI-Beta":            []string{"realtime=v1"},
		"Sec-WebSocket-Protocol": []string{"realtime"},
	}
	cloned := cloneRealtimeRequestHeaders(header, &meta.Meta{
		ChannelProtocol: relaychannel.VolcEngine,
		RequestURLPath:  adminmodel.ChannelModelEndpointRealtime,
		APIKey:          "access-456",
		Config: adminmodel.ChannelConfig{
			AppID:      "app-123",
			ResourceID: "resource-789",
		},
	})
	if got := cloned.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want empty", got)
	}
	if got := cloned.Get("OpenAI-Beta"); got != "" {
		t.Fatalf("OpenAI-Beta = %q, want empty", got)
	}
	if got := cloned.Get("X-Api-App-ID"); got != "app-123" {
		t.Fatalf("X-Api-App-ID = %q, want %q", got, "app-123")
	}
	if got := cloned.Get("X-Api-Access-Key"); got != "access-456" {
		t.Fatalf("X-Api-Access-Key = %q, want %q", got, "access-456")
	}
	if got := cloned.Get("X-Api-Resource-Id"); got != "resource-789" {
		t.Fatalf("X-Api-Resource-Id = %q, want %q", got, "resource-789")
	}
}
