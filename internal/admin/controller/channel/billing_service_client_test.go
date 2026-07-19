package channel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/internal/admin/model"
)

func TestResolveBillingServiceAdapterUsesAdapterName(t *testing.T) {
	if got := resolveBillingServiceAdapter(model.ChannelBillingProfile{BillingSource: "vendor-a"}); got != "vendor-a" {
		t.Fatalf("vendor-a adapter = %q", got)
	}
	if got := resolveBillingServiceAdapter(model.ChannelBillingProfile{BillingSource: " Vendor-B "}); got != "vendor-b" {
		t.Fatalf("vendor-b adapter = %q", got)
	}
	if got := resolveBillingServiceAdapter(model.ChannelBillingProfile{BillingSource: model.ChannelBillingSourceManual}); got != "" {
		t.Fatalf("manual adapter = %q", got)
	}
}

func TestListBillingServiceAdaptersNormalizesServiceResponse(t *testing.T) {
	oldBaseURL := config.BillingServiceBaseURL
	oldAPIKey := config.BillingServiceAPIKey
	oldTimeout := config.BillingServiceTimeoutSeconds
	defer func() {
		config.BillingServiceBaseURL = oldBaseURL
		config.BillingServiceAPIKey = oldAPIKey
		config.BillingServiceTimeoutSeconds = oldTimeout
	}()

	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != billingServiceAdaptersPath {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer service-key" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"name":" Vendor-B ","capabilities":["refresh_billing"]},{"name":"vendor-b"},{"name":"vendor-a"},{"name":""}]}`))
	}))
	defer service.Close()

	config.BillingServiceBaseURL = service.URL
	config.BillingServiceAPIKey = "service-key"
	config.BillingServiceTimeoutSeconds = 5

	items, err := listBillingServiceAdapters(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Name != "vendor-a" || items[1].Name != "vendor-b" {
		t.Fatalf("unexpected adapters: %+v", items)
	}
	exists, err := billingServiceAdapterExists(context.Background(), " Vendor-B ")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected vendor-b adapter to exist")
	}
	exists, err = billingServiceAdapterExists(context.Background(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("missing adapter should not exist")
	}
}

func TestBuildBillingServiceQueryUsesAdapterProtocol(t *testing.T) {
	profile := model.ChannelBillingProfile{
		BillingSource: "vendor-a",
		BillingConfig: `{"billing_key":"billing-secret"}`,
	}
	query, err := buildBillingServiceQuery(&model.Channel{
		Id:       "channel-1",
		Protocol: "vendor-protocol",
		Key:      "model-secret-must-not-be-used",
	}, profile)
	if err != nil {
		t.Fatal(err)
	}
	if query.Adapter != "vendor-a" {
		t.Fatalf("adapter = %q", query.Adapter)
	}
	if query.Credential != "billing-secret" {
		t.Fatalf("credential = %q", query.Credential)
	}
	if query.Config != nil {
		t.Fatalf("config must be owned by billing service adapters, got %+v", query.Config)
	}
	body, err := json.Marshal(query)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "provider") {
		t.Fatalf("query must not use legacy provider field: %s", body)
	}
}

func TestBuildBillingServiceQueryFallsBackToChannelKey(t *testing.T) {
	profile := model.ChannelBillingProfile{
		BillingSource: "vendor-a",
		BillingConfig: `{}`,
	}
	query, err := buildBillingServiceQuery(&model.Channel{
		Id:  "channel-1",
		Key: "model-secret",
	}, profile)
	if err != nil {
		t.Fatal(err)
	}
	if query.Credential != "model-secret" {
		t.Fatalf("credential = %q", query.Credential)
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
		if query["adapter"] != "vendor-b" || query["provider"] != nil {
			t.Fatalf("unexpected query: %+v", query)
		}
		if query["credential"] != "billing-secret" {
			t.Fatalf("credential = %q", query["credential"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"channel_id":"channel-1","adapter":"vendor-b","fetched_at":"2026-07-18T00:00:00Z","items":[{"resource_type":"credit","quota_type":"total","quota_label":"Vendor credits","amount":15,"limit_amount":100,"used_amount":85,"remaining_amount":15,"currency":"USD","status":"low","source_ref":"vendor_credits","metadata":{"source":"test"}}],"metadata":{"request_urls":["https://billing.example.com/credits"]}}}`))
	}))
	defer service.Close()

	config.BillingServiceBaseURL = service.URL
	config.BillingServiceAPIKey = "service-key"
	config.BillingServiceTimeoutSeconds = 5

	channelRow := &model.Channel{
		Id:       "channel-1",
		Protocol: "vendor-protocol",
		Key:      "model-secret-must-not-be-used",
	}
	profile := model.ChannelBillingProfile{
		ChannelId:     "channel-1",
		BillingSource: "vendor-b",
		BillingConfig: `{"billing_key":"billing-secret"}`,
	}
	collected, err := collectBillingServiceSnapshot(channelRow, profile, "自动刷新账务")
	if err != nil {
		t.Fatal(err)
	}
	if collected.PrimaryAmount != 15 || collected.ShouldHardStop {
		t.Fatalf("unexpected collected summary: %+v", collected)
	}
	if collected.Snapshot.Balance != 15 || collected.Snapshot.Currency != "USD" || !strings.Contains(collected.Snapshot.Message, "adapter=vendor-b") {
		t.Fatalf("unexpected snapshot: %+v", collected.Snapshot)
	}
	if collected.Snapshot.RequestURL != "https://billing.example.com/credits" {
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
		if query["adapter"] != "vendor-a" {
			t.Fatalf("adapter = %v", query["adapter"])
		}
		if query["credential"] != "billing-secret" {
			t.Fatalf("credential = %v", query["credential"])
		}
		if query["config"] != nil {
			t.Fatalf("unexpected config: %+v", query["config"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"channel_id":"channel-1","adapter":"vendor-a","fetched_at":"2026-07-18T00:00:00Z","items":[{"resource_type":"credit","quota_type":"total","quota_label":"Total quota","amount":50,"limit_amount":100,"used_amount":50,"remaining_amount":50,"currency":"CNY","status":"active","source_ref":"vendor_total"}]}}`))
	}))
	defer service.Close()

	config.BillingServiceBaseURL = service.URL
	config.BillingServiceAPIKey = ""
	config.BillingServiceTimeoutSeconds = 5

	collected, err := collectChannelBillingSnapshot(&model.Channel{
		Id:       "channel-1",
		Protocol: "vendor-protocol",
		Key:      "model-secret-must-not-be-used",
	}, model.ChannelBillingProfile{
		ChannelId:     "channel-1",
		BillingSource: "vendor-a",
		BillingConfig: `{"billing_key":"billing-secret"}`,
	}, "自动刷新账务")
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("billing service was not called")
	}
	if collected.PrimaryAmount != 50 || len(collected.Items) != 1 || collected.Items[0].SourceRef != "vendor_total" {
		t.Fatalf("unexpected collected result: %+v", collected)
	}
}
