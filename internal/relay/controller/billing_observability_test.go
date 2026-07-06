package controller

import (
	"testing"

	adminmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/billing"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
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

func TestAnnotateAudioBillingSnapshot(t *testing.T) {
	t.Run("speech request payload final", func(t *testing.T) {
		snapshot := billing.BillingSnapshot{}
		annotateAudioBillingSnapshot(&snapshot, "provider_component", relaymode.AudioSpeech)
		if snapshot.PricingSource != "provider_component" {
			t.Fatalf("PricingSource = %q, want provider_component", snapshot.PricingSource)
		}
		if snapshot.UsageSource != billingUsageSourceRequestPayload {
			t.Fatalf("UsageSource = %q, want %q", snapshot.UsageSource, billingUsageSourceRequestPayload)
		}
		if snapshot.EstimateSource != billingEstimateSourceAudioTTSInputChars {
			t.Fatalf("EstimateSource = %q, want %q", snapshot.EstimateSource, billingEstimateSourceAudioTTSInputChars)
		}
		if snapshot.SettlementMode != billingSettlementModeAudioRequestFinal {
			t.Fatalf("SettlementMode = %q, want %q", snapshot.SettlementMode, billingSettlementModeAudioRequestFinal)
		}
	})

	t.Run("transcription response text final", func(t *testing.T) {
		snapshot := billing.BillingSnapshot{}
		annotateAudioBillingSnapshot(&snapshot, "provider_migration", relaymode.AudioTranscription)
		if snapshot.PricingSource != "provider_migration" {
			t.Fatalf("PricingSource = %q, want provider_migration", snapshot.PricingSource)
		}
		if snapshot.UsageSource != billingUsageSourceResponseText {
			t.Fatalf("UsageSource = %q, want %q", snapshot.UsageSource, billingUsageSourceResponseText)
		}
		if snapshot.EstimateSource != billingEstimateSourceAudioPreconsumeQuota {
			t.Fatalf("EstimateSource = %q, want %q", snapshot.EstimateSource, billingEstimateSourceAudioPreconsumeQuota)
		}
		if snapshot.SettlementMode != billingSettlementModeAudioResponseTextFinal {
			t.Fatalf("SettlementMode = %q, want %q", snapshot.SettlementMode, billingSettlementModeAudioResponseTextFinal)
		}
	})
}

func TestAnnotateVideoBillingSnapshot(t *testing.T) {
	snapshot := billing.BillingSnapshot{}
	annotateVideoBillingSnapshot(&snapshot, "provider_component")
	if snapshot.PricingSource != "provider_component" {
		t.Fatalf("PricingSource = %q, want provider_component", snapshot.PricingSource)
	}
	if snapshot.UsageSource != billingUsageSourceRequestPayload {
		t.Fatalf("UsageSource = %q, want %q", snapshot.UsageSource, billingUsageSourceRequestPayload)
	}
	if snapshot.EstimateSource != billingEstimateSourceVideoRequestRule {
		t.Fatalf("EstimateSource = %q, want %q", snapshot.EstimateSource, billingEstimateSourceVideoRequestRule)
	}
	if snapshot.SettlementMode != billingSettlementModeVideoTaskCreated {
		t.Fatalf("SettlementMode = %q, want %q", snapshot.SettlementMode, billingSettlementModeVideoTaskCreated)
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

func TestAnnotateTextPreConsumeLogFields(t *testing.T) {
	logRow := &adminmodel.Log{
		PromptTokens:          90,
		CompletionTokens:      40,
		BillingChargeAmount:   13,
		EstimatedPromptTokens: 1,
	}
	annotateTextPreConsumeLogFields(logRow, 100, 64, 21)
	if logRow.EstimatedPromptTokens != 100 {
		t.Fatalf("EstimatedPromptTokens = %d, want 100", logRow.EstimatedPromptTokens)
	}
	if logRow.EstimatedOutputTokens != 64 {
		t.Fatalf("EstimatedOutputTokens = %d, want 64", logRow.EstimatedOutputTokens)
	}
	if logRow.EstimatedChargeAmount != 21 {
		t.Fatalf("EstimatedChargeAmount = %d, want 21", logRow.EstimatedChargeAmount)
	}
	if logRow.BillingPromptTokenDelta != -10 {
		t.Fatalf("BillingPromptTokenDelta = %d, want -10", logRow.BillingPromptTokenDelta)
	}
	if logRow.BillingOutputTokenDelta != -24 {
		t.Fatalf("BillingOutputTokenDelta = %d, want -24", logRow.BillingOutputTokenDelta)
	}
	if logRow.BillingChargeDeltaAmount != -8 {
		t.Fatalf("BillingChargeDeltaAmount = %d, want -8", logRow.BillingChargeDeltaAmount)
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

func TestBillingSnapshotApplyToLogIncludesTextCacheFields(t *testing.T) {
	snapshot := billing.BillingSnapshot{
		CacheReadQuantity:  300,
		CacheWriteQuantity: 100,
		CacheReadAmount:    0.0006,
		CacheWriteAmount:   0.0012,
	}
	logRow := &adminmodel.Log{}
	snapshot.ApplyToLog(logRow)
	if logRow.BillingCacheReadQuantity != 300 {
		t.Fatalf("BillingCacheReadQuantity = %v, want 300", logRow.BillingCacheReadQuantity)
	}
	if logRow.BillingCacheWriteQuantity != 100 {
		t.Fatalf("BillingCacheWriteQuantity = %v, want 100", logRow.BillingCacheWriteQuantity)
	}
	if logRow.BillingCacheReadAmount != 0.0006 {
		t.Fatalf("BillingCacheReadAmount = %v, want 0.0006", logRow.BillingCacheReadAmount)
	}
	if logRow.BillingCacheWriteAmount != 0.0012 {
		t.Fatalf("BillingCacheWriteAmount = %v, want 0.0012", logRow.BillingCacheWriteAmount)
	}
}
