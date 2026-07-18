package channel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/internal/admin/model"
)

func TestBuildBillingServiceQueryUsesAdapterProtocol(t *testing.T) {
	profile := model.ChannelBillingProfile{
		BillingMode:   model.ChannelBillingModeBuiltinCDK,
		BillingConfig: `{"api_base_url":"https://billing.example.com","cdk":"profile-cdk","currency":"CNY"}`,
	}
	query, err := buildBillingServiceQuery(&model.Channel{
		Id:       "channel-1",
		Protocol: "openai",
		Key:      "channel-key",
	}, profile)
	if err != nil {
		t.Fatal(err)
	}
	if query.Adapter != "aixhan" {
		t.Fatalf("adapter = %q", query.Adapter)
	}
	if query.BaseURL != "https://billing.example.com" {
		t.Fatalf("base_url = %q", query.BaseURL)
	}
	if query.Config["cdk"] != "profile-cdk" || query.Config["currency"] != "CNY" {
		t.Fatalf("unexpected config: %+v", query.Config)
	}
	body, err := json.Marshal(query)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "provider") {
		t.Fatalf("query must not use legacy provider field: %s", body)
	}
}

func TestCollectBillingServiceSnapshotConvertsResponse(t *testing.T) {
	oldBaseURL := config.BillingServiceBaseURL
	oldAPIKey := config.BillingServiceAPIKey
	oldTimeout := config.BillingServiceTimeoutSeconds
	defer func() {
		config.BillingServiceBaseURL = oldBaseURL
		config.BillingServiceAPIKey = oldAPIKey
		config.BillingServiceTimeoutSeconds = oldTimeout
	}()

	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != billingServiceQueryPath {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer service-key" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		var query map[string]any
		if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
			t.Fatal(err)
		}
		if query["adapter"] != "openrouter" || query["provider"] != nil {
			t.Fatalf("unexpected query: %+v", query)
		}
		if query["credential"] != "channel-secret" {
			t.Fatalf("credential = %q", query["credential"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"channel_id":"channel-1","adapter":"openrouter","fetched_at":"2026-07-18T00:00:00Z","items":[{"resource_type":"credit","quota_type":"total","quota_label":"OpenRouter credits","amount":15,"limit_amount":100,"used_amount":85,"remaining_amount":15,"currency":"USD","status":"low","source_ref":"openrouter_credits","metadata":{"source":"test"}}],"metadata":{"request_urls":["https://openrouter.ai/api/v1/credits"]}}}`))
	}))
	defer service.Close()

	config.BillingServiceBaseURL = service.URL
	config.BillingServiceAPIKey = "service-key"
	config.BillingServiceTimeoutSeconds = 5

	channelRow := &model.Channel{
		Id:       "channel-1",
		Protocol: "openrouter",
		Key:      "channel-secret",
	}
	profile := model.ChannelBillingProfile{
		ChannelId:   "channel-1",
		BillingMode: model.ChannelBillingModeBuiltinOpenRouter,
	}
	collected, err := collectBillingServiceSnapshot(channelRow, profile, "自动刷新账务")
	if err != nil {
		t.Fatal(err)
	}
	if collected.PrimaryAmount != 15 || collected.ShouldHardStop {
		t.Fatalf("unexpected collected summary: %+v", collected)
	}
	if collected.Snapshot.Balance != 15 || collected.Snapshot.Currency != "USD" || !strings.Contains(collected.Snapshot.Message, "adapter=openrouter") {
		t.Fatalf("unexpected snapshot: %+v", collected.Snapshot)
	}
	if collected.Snapshot.RequestURL != "https://openrouter.ai/api/v1/credits" {
		t.Fatalf("request url = %q", collected.Snapshot.RequestURL)
	}
	if len(collected.Items) != 1 || collected.Items[0].Metadata == "" || collected.Items[0].RemainingAmount != 15 {
		t.Fatalf("unexpected items: %+v", collected.Items)
	}
}

func TestCollectChannelBillingSnapshotPrefersBillingService(t *testing.T) {
	oldBaseURL := config.BillingServiceBaseURL
	oldAPIKey := config.BillingServiceAPIKey
	oldTimeout := config.BillingServiceTimeoutSeconds
	defer func() {
		config.BillingServiceBaseURL = oldBaseURL
		config.BillingServiceAPIKey = oldAPIKey
		config.BillingServiceTimeoutSeconds = oldTimeout
	}()

	called := false
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		var query map[string]any
		if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
			t.Fatal(err)
		}
		if query["adapter"] != "aixhan" {
			t.Fatalf("adapter = %v", query["adapter"])
		}
		configMap, ok := query["config"].(map[string]any)
		if !ok || configMap["cdk"] != "profile-cdk" {
			t.Fatalf("unexpected config: %+v", query["config"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"channel_id":"channel-1","adapter":"aixhan","fetched_at":"2026-07-18T00:00:00Z","items":[{"resource_type":"credit","quota_type":"total","quota_label":"Total quota","amount":50,"limit_amount":100,"used_amount":50,"remaining_amount":50,"currency":"CNY","status":"active","source_ref":"aixhan_total"}]}}`))
	}))
	defer service.Close()

	config.BillingServiceBaseURL = service.URL
	config.BillingServiceAPIKey = ""
	config.BillingServiceTimeoutSeconds = 5

	collected, err := collectChannelBillingSnapshot(&model.Channel{
		Id:       "channel-1",
		Protocol: "openai",
		Key:      "channel-key",
	}, model.ChannelBillingProfile{
		ChannelId:     "channel-1",
		BillingMode:   model.ChannelBillingModeBuiltinCDK,
		BillingConfig: `{"api_base_url":"https://billing.example.com","cdk":"profile-cdk","currency":"CNY"}`,
	}, "自动刷新账务")
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("billing service was not called")
	}
	if collected.PrimaryAmount != 50 || len(collected.Items) != 1 || collected.Items[0].SourceRef != "aixhan_total" {
		t.Fatalf("unexpected collected result: %+v", collected)
	}
}
