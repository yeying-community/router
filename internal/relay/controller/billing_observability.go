package controller

import (
	"fmt"
	"strings"

	adminmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/billing"
	"github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/tokenestimate"
)

const (
	billingUsageSourceUpstreamUsage            = "upstream_usage"
	billingEstimateSourceUnknown               = "unknown"
	billingSettlementModeUsageFinal            = "usage_final"
	billingSettlementModeResponsesImagePending = "responses_image_tool_pending"
)

func resolveTextEstimateSourceLabel(result tokenestimate.EstimateResult) string {
	estimator := strings.TrimSpace(result.Estimator)
	if estimator != "" {
		return "tokenestimate:" + estimator
	}
	source := strings.TrimSpace(result.Source)
	if source != "" {
		return "tokenestimate_source:" + source
	}
	return billingEstimateSourceUnknown
}

func hasResponsesImageGenerationTool(endpoint string, req *model.GeneralOpenAIRequest) bool {
	if strings.TrimSpace(strings.ToLower(endpoint)) != adminmodel.ChannelModelEndpointResponses {
		return false
	}
	if req == nil || len(req.Tools) == 0 {
		return false
	}
	for _, tool := range req.Tools {
		if strings.TrimSpace(strings.ToLower(tool.Type)) == adminmodel.ProviderModelPriceComponentImageGeneration {
			return true
		}
	}
	return false
}

func resolveTextSettlementMode(endpoint string, req *model.GeneralOpenAIRequest) string {
	if hasResponsesImageGenerationTool(endpoint, req) {
		return billingSettlementModeResponsesImagePending
	}
	return billingSettlementModeUsageFinal
}

func annotateTextBillingSnapshot(snapshot *billing.BillingSnapshot, pricingSource string, estimateSource string, endpoint string, req *model.GeneralOpenAIRequest) {
	if snapshot == nil {
		return
	}
	snapshot.PricingSource = strings.TrimSpace(pricingSource)
	snapshot.UsageSource = billingUsageSourceUpstreamUsage
	snapshot.EstimateSource = strings.TrimSpace(estimateSource)
	snapshot.SettlementMode = resolveTextSettlementMode(endpoint, req)
}

func annotateTextEstimateLogFields(logRow *adminmodel.Log, result tokenestimate.EstimateResult) {
	if logRow == nil {
		return
	}
	logRow.EstimatedPromptTokens = result.PromptTokens
	logRow.BillingEstimateEstimator = strings.TrimSpace(result.Estimator)
	logRow.BillingEstimatePrecision = strings.TrimSpace(string(result.Precision))
}

func buildTextBillingLogContent(pricing adminmodel.ResolvedModelPricing, groupRatio float64, suffix string) string {
	content := billing.FormatPricingLog(pricing, groupRatio)
	if strings.TrimSpace(suffix) == "" {
		return content
	}
	return fmt.Sprintf("%s; %s", content, strings.TrimSpace(suffix))
}
