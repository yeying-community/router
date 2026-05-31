package controller

import (
	"testing"

	"github.com/yeying-community/router/internal/admin/model"
)

func TestNormalizeTopupCallbackInputSupportsProviderAliases(t *testing.T) {
	got := normalizeTopupCallbackInput(topupCallbackRequest{
		OrderID:        "order-1",
		OutTradeNo:     "transaction-1",
		TradeNo:        "provider-1",
		TradeStatus:    "success",
		Message:        "paid by provider",
		PayTime:        123,
		CallbackStatus: "ignored",
	})

	if got.OrderID != "order-1" {
		t.Fatalf("OrderID = %q", got.OrderID)
	}
	if got.ProviderOrderID != "provider-1" {
		t.Fatalf("ProviderOrderID = %q", got.ProviderOrderID)
	}
	if got.TransactionID != "transaction-1" {
		t.Fatalf("TransactionID = %q", got.TransactionID)
	}
	if got.Status != model.TopupOrderStatusPaid {
		t.Fatalf("Status = %q, want paid", got.Status)
	}
	if got.StatusMessage != "paid by provider" {
		t.Fatalf("StatusMessage = %q", got.StatusMessage)
	}
	if got.PaidAt != 123 {
		t.Fatalf("PaidAt = %d", got.PaidAt)
	}
}
