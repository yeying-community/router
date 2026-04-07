package controller

import "testing"

func TestBuildBalanceRelayBillingPlanWithoutPackage(t *testing.T) {
	plan := buildBalanceRelayBillingPlan(false)
	if plan.Source != relayBillingSourceBalance {
		t.Fatalf("plan.Source = %q, want %q", plan.Source, relayBillingSourceBalance)
	}
	if !plan.ChargeUserBalance() {
		t.Fatalf("plan.ChargeUserBalance() = false, want true")
	}
}

func TestBuildBalanceRelayBillingPlanWithPackageFallback(t *testing.T) {
	plan := buildBalanceRelayBillingPlan(true)
	if plan.Source != relayBillingSourcePackageFallbackBalance {
		t.Fatalf("plan.Source = %q, want %q", plan.Source, relayBillingSourcePackageFallbackBalance)
	}
	if !plan.ChargeUserBalance() {
		t.Fatalf("plan.ChargeUserBalance() = false, want true")
	}
}
