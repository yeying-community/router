package model

import (
	"net/url"
	"testing"

	"github.com/yeying-community/router/common/config"
)

func TestBuildTopupOrderRedirectURL(t *testing.T) {
	previousSecret := config.TopUpSignSecret
	config.TopUpSignSecret = "test-sign-secret"
	t.Cleanup(func() {
		config.TopUpSignSecret = previousSecret
	})
	redirectURL, err := buildTopupOrderRedirectURL(
		"https://pay.example.com/checkout?source=router",
		TopupOrder{
			Id:            "order_1",
			UserID:        "user_1",
			Username:      "alice",
			TransactionID: "txn_1",
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parsed, err := url.Parse(redirectURL)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	query := parsed.Query()
	if got := query.Get("source"); got != "router" {
		t.Fatalf("expected source=router, got %q", got)
	}
	if got := query.Get("order_id"); got != "order_1" {
		t.Fatalf("expected order_id=order_1, got %q", got)
	}
	if got := query.Get("user_id"); got != "user_1" {
		t.Fatalf("expected user_id=user_1, got %q", got)
	}
	if got := query.Get("username"); got != "alice" {
		t.Fatalf("expected username=alice, got %q", got)
	}
	if got := query.Get("transaction_id"); got != "txn_1" {
		t.Fatalf("expected transaction_id=txn_1, got %q", got)
	}
	if got := query.Get("merchant_app"); got != "router" {
		t.Fatalf("expected merchant_app=router, got %q", got)
	}
	if got := query.Get("sign"); got == "" {
		t.Fatal("expected sign to be set")
	}
}

func TestBuildTopupOrderRedirectURLRejectsInvalidBaseLink(t *testing.T) {
	previousSecret := config.TopUpSignSecret
	config.TopUpSignSecret = "test-sign-secret"
	t.Cleanup(func() {
		config.TopUpSignSecret = previousSecret
	})
	if _, err := buildTopupOrderRedirectURL("://broken", TopupOrder{
		Id:            "order_1",
		UserID:        "user_1",
		Username:      "alice",
		TransactionID: "txn_1",
	}); err == nil {
		t.Fatal("expected error for invalid base link")
	}
}

func TestResolveTopupOrderBusinessType(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		packageID  string
		wantResult string
	}{
		{
			name:       "explicit balance",
			value:      TopupOrderBusinessBalance,
			wantResult: TopupOrderBusinessBalance,
		},
		{
			name:       "explicit package",
			value:      TopupOrderBusinessPackage,
			wantResult: TopupOrderBusinessPackage,
		},
		{
			name:       "infer package from package id",
			packageID:  "pkg_1",
			wantResult: TopupOrderBusinessPackage,
		},
		{
			name:       "fallback balance for legacy empty type",
			wantResult: TopupOrderBusinessBalance,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveTopupOrderBusinessType(tt.value, tt.packageID); got != tt.wantResult {
				t.Fatalf("expected %q, got %q", tt.wantResult, got)
			}
		})
	}
}

func TestTopupOrderSigningStringHelpers(t *testing.T) {
	payload := map[string]string{
		"b":    "2",
		"a":    "1",
		"skip": "",
		"sign": "ignored",
	}
	if got, want := topupOrderSigningBaseString(payload), "a=1&b=2"; got != want {
		t.Fatalf("unexpected signing base string: got %q want %q", got, want)
	}
	if got, want := topupOrderSigningString(payload, "secret-value"), "a=1&b=2&secret=secret-value"; got != want {
		t.Fatalf("unexpected signing string: got %q want %q", got, want)
	}
}
