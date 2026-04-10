package model

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yeying-community/router/common/config"
)

func TestCreateTopupOrderByExternalPayAPI(t *testing.T) {
	previousMode := config.TopUpMode
	previousCreateURL := config.TopUpAPICreateURL
	previousUniacid := config.TopUpAPIUniacid
	previousMerchantApp := config.TopUpMerchantApp
	previousSecret := config.TopUpSignSecret
	t.Cleanup(func() {
		config.TopUpMode = previousMode
		config.TopUpAPICreateURL = previousCreateURL
		config.TopUpAPIUniacid = previousUniacid
		config.TopUpMerchantApp = previousMerchantApp
		config.TopUpSignSecret = previousSecret
	})

	var captured map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("uniacid") != "9" {
			t.Fatalf("expected uniacid=9, got %q", r.URL.Query().Get("uniacid"))
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
			t.Fatalf("expected application/json, got %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		expectedSign := signTopupOrderPayload(map[string]string{
			"merchant_app":   "router-pay",
			"order_id":       "order_1",
			"transaction_id": "txn_1",
			"user_id":        "user_1",
			"username":       "alice",
			"business_type":  TopupOrderBusinessBalance,
			"operation_type": TopupOrderOperationTopup,
			"title":          "账户充值",
			"amount":         "12.50",
			"currency":       "CNY",
			"client_type":    topupOrderClientTypeMobile,
			"callback_url":   "https://router.example.com/api/v1/public/topup/callback",
			"return_url":     "https://router.example.com/topup/result",
			"timestamp":      captured["timestamp"],
			"nonce":          captured["nonce"],
			"quota":          "888",
		}, "topup-sign")
		if captured["sign"] != expectedSign {
			t.Fatalf("unexpected sign: got %q want %q", captured["sign"], expectedSign)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 1,
			"msg":  "success",
			"data": map[string]any{
				"trade_no":    "trade_1",
				"cashier_url": "https://pay.example.com/cashier/trade_1",
			},
		})
	}))
	defer server.Close()

	config.TopUpMode = config.TopUpModeAPI
	config.TopUpAPICreateURL = server.URL + "/addons/ymqzixishi/api.external_pay/create"
	config.TopUpAPIUniacid = 9
	config.TopUpMerchantApp = "router-pay"
	config.TopUpSignSecret = "topup-sign"

	result, err := createTopupOrderByExternalPayAPI(TopupOrder{
		Id:            "order_1",
		UserID:        "user_1",
		Username:      "alice",
		TransactionID: "txn_1",
		BusinessType:  TopupOrderBusinessBalance,
		Title:         "账户充值",
		Amount:        12.5,
		Currency:      "CNY",
		Quota:         888,
		CallbackURL:   "https://router.example.com/api/v1/public/topup/callback",
		ReturnURL:     "https://router.example.com/topup/result",
	}, "mobile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RedirectURL != "https://pay.example.com/cashier/trade_1" {
		t.Fatalf("unexpected redirect url: %q", result.RedirectURL)
	}
	if result.ProviderOrderID != "trade_1" {
		t.Fatalf("unexpected provider order id: %q", result.ProviderOrderID)
	}
	if captured["merchant_app"] != "router-pay" {
		t.Fatalf("unexpected merchant_app: %q", captured["merchant_app"])
	}
	if captured["client_type"] != "mobile" {
		t.Fatalf("unexpected client_type: %q", captured["client_type"])
	}
	if captured["operation_type"] != TopupOrderOperationTopup {
		t.Fatalf("unexpected operation_type: %q", captured["operation_type"])
	}
}

func TestBuildExternalPayCreateURLDerivesFromCashierLink(t *testing.T) {
	previousMode := config.TopUpMode
	previousLink := config.TopUpLink
	previousCreateURL := config.TopUpAPICreateURL
	previousUniacid := config.TopUpAPIUniacid
	t.Cleanup(func() {
		config.TopUpMode = previousMode
		config.TopUpLink = previousLink
		config.TopUpAPICreateURL = previousCreateURL
		config.TopUpAPIUniacid = previousUniacid
	})

	config.TopUpMode = config.TopUpModeAPI
	config.TopUpLink = "https://pay.example.com/addons/ymq_zixishi/public/index.php/addons/ymqzixishi/api.external_pay/cashier?foo=bar"
	config.TopUpAPICreateURL = ""
	config.TopUpAPIUniacid = 3

	got, err := buildExternalPayCreateURL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "/api.external_pay/create") {
		t.Fatalf("expected create endpoint, got %q", got)
	}
	if !strings.Contains(got, "uniacid=3") {
		t.Fatalf("expected uniacid query in %q", got)
	}
	if !strings.Contains(got, "foo=bar") {
		t.Fatalf("expected original query to be preserved in %q", got)
	}
}
