package controller

import (
	"testing"

	adminmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/billing"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/tokenestimate"
)

func TestResolveTextEstimateSourceLabel(t *testing.T) {
	got := resolveTextEstimateSourceLabel(tokenestimate.EstimateResult{
		Estimator: "openai_exact",
		Source:    "messages",
	})
	if got != "tokenestimate:openai_exact" {
		t.Fatalf("resolveTextEstimateSourceLabel() = %q, want %q", got, "tokenestimate:openai_exact")
	}
}

func TestHasResponsesImageGenerationTool(t *testing.T) {
	req := &relaymodel.GeneralOpenAIRequest{
		Tools: []relaymodel.Tool{
			{Type: adminmodel.ProviderModelPriceComponentImageGeneration},
		},
	}
	if !hasResponsesImageGenerationTool(adminmodel.ChannelModelEndpointResponses, req) {
		t.Fatal("hasResponsesImageGenerationTool() = false, want true")
	}
	if hasResponsesImageGenerationTool(adminmodel.ChannelModelEndpointChat, req) {
		t.Fatal("hasResponsesImageGenerationTool() = true for chat endpoint, want false")
	}
}

func TestAnnotateTextBillingSnapshotResponsesImagePending(t *testing.T) {
	snapshot := billing.BillingSnapshot{}
	req := &relaymodel.GeneralOpenAIRequest{
		Tools: []relaymodel.Tool{
			{Type: adminmodel.ProviderModelPriceComponentImageGeneration},
		},
	}
	annotateTextBillingSnapshot(&snapshot, "provider_component", "tokenestimate:openai_exact", adminmodel.ChannelModelEndpointResponses, req)
	if snapshot.PricingSource != "provider_component" {
		t.Fatalf("PricingSource = %q, want %q", snapshot.PricingSource, "provider_component")
	}
	if snapshot.UsageSource != billingUsageSourceUpstreamUsage {
		t.Fatalf("UsageSource = %q, want %q", snapshot.UsageSource, billingUsageSourceUpstreamUsage)
	}
	if snapshot.EstimateSource != "tokenestimate:openai_exact" {
		t.Fatalf("EstimateSource = %q, want %q", snapshot.EstimateSource, "tokenestimate:openai_exact")
	}
	if snapshot.SettlementMode != billingSettlementModeResponsesImagePending {
		t.Fatalf("SettlementMode = %q, want %q", snapshot.SettlementMode, billingSettlementModeResponsesImagePending)
	}
}

func TestAnnotateTextEstimateLogFields(t *testing.T) {
	logRow := &adminmodel.Log{}
	annotateTextEstimateLogFields(logRow, tokenestimate.EstimateResult{
		PromptTokens: 37,
		Estimator:    "openai_exact",
		Precision:    tokenestimate.PrecisionExact,
	})
	if logRow.EstimatedPromptTokens != 37 {
		t.Fatalf("EstimatedPromptTokens = %d, want 37", logRow.EstimatedPromptTokens)
	}
	if logRow.BillingEstimateEstimator != "openai_exact" {
		t.Fatalf("BillingEstimateEstimator = %q, want openai_exact", logRow.BillingEstimateEstimator)
	}
	if logRow.BillingEstimatePrecision != string(tokenestimate.PrecisionExact) {
		t.Fatalf("BillingEstimatePrecision = %q, want %q", logRow.BillingEstimatePrecision, tokenestimate.PrecisionExact)
	}
}

func TestParseResponsesImageToolSpecs(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-5",
		"tools":[
			{"type":"image_generation","model":"gpt-image-1","size":"1024x1024","quality":"high"},
			{"type":"function","name":"noop"}
		]
	}`)
	specs, err := parseResponsesImageToolSpecs(raw)
	if err != nil {
		t.Fatalf("parseResponsesImageToolSpecs() error = %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("len(specs) = %d, want 1", len(specs))
	}
	if specs[0].Model != "gpt-image-1" || specs[0].Size != "1024x1024" || specs[0].Quality != "high" {
		t.Fatalf("unexpected spec: %#v", specs[0])
	}
}

func TestBillingSnapshotApplyToLogIncludesImageToolFields(t *testing.T) {
	snapshot := billing.BillingSnapshot{
		ImageToolCalls:        1,
		ImageToolOutputTokens: 4160,
		ImageToolAmount:       0.1248,
		ImageToolChargeAmount: 74880,
	}
	logRow := &adminmodel.Log{}
	snapshot.ApplyToLog(logRow)
	if logRow.BillingImageToolCalls != 1 {
		t.Fatalf("BillingImageToolCalls = %d, want 1", logRow.BillingImageToolCalls)
	}
	if logRow.BillingImageToolOutputTokens != 4160 {
		t.Fatalf("BillingImageToolOutputTokens = %d, want 4160", logRow.BillingImageToolOutputTokens)
	}
	if logRow.BillingImageToolAmount != 0.1248 {
		t.Fatalf("BillingImageToolAmount = %v, want 0.1248", logRow.BillingImageToolAmount)
	}
	if logRow.BillingImageToolChargeAmount != 74880 {
		t.Fatalf("BillingImageToolChargeAmount = %d, want 74880", logRow.BillingImageToolChargeAmount)
	}
}
