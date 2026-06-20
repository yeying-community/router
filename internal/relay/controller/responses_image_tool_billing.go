package controller

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	adminmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/billing"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

const (
	defaultResponsesImageToolModel         = "gpt-image-1"
	billingSettlementModeUsagePlusImageFee = "usage_plus_image_fee"
	mixedBillingPriceUnit                  = "mixed"
)

type responsesImageToolSpec struct {
	Model   string
	Size    string
	Quality string
}

type responsesImageToolBillingDetail struct {
	Model        string
	Calls        int
	OutputTokens int
	Amount       float64
	ChargeAmount int64
	PriceUnit    string
	Applied      bool
}

func parseResponsesImageToolSpecs(rawBody []byte) ([]responsesImageToolSpec, error) {
	if len(rawBody) == 0 {
		return nil, nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, err
	}
	toolsRaw, ok := payload["tools"].([]any)
	if !ok || len(toolsRaw) == 0 {
		return nil, nil
	}
	specs := make([]responsesImageToolSpec, 0, len(toolsRaw))
	for _, toolAny := range toolsRaw {
		toolMap, ok := toolAny.(map[string]any)
		if !ok {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(fmt.Sprint(toolMap["type"])), adminmodel.ProviderModelPriceComponentImageGeneration) {
			continue
		}
		specs = append(specs, responsesImageToolSpec{
			Model:   normalizeResponsesImageToolString(toolMap["model"], defaultResponsesImageToolModel),
			Size:    normalizeResponsesImageToolString(toolMap["size"], "auto"),
			Quality: normalizeResponsesImageToolString(toolMap["quality"], "auto"),
		})
	}
	return specs, nil
}

func normalizeResponsesImageToolString(value any, fallback string) string {
	normalized := strings.TrimSpace(strings.ToLower(fmt.Sprint(value)))
	if normalized == "" || normalized == "<nil>" {
		return fallback
	}
	return normalized
}

func maybeApplyResponsesImageToolBilling(snapshot *billing.BillingSnapshot, usage *relaymodel.Usage, channelProtocol int, channelModels []adminmodel.ChannelModel, groupRatio float64, specs []responsesImageToolSpec) (responsesImageToolBillingDetail, string, error) {
	if snapshot == nil || usage == nil || usage.ImageGenerationCalls <= 0 {
		return responsesImageToolBillingDetail{}, "", nil
	}
	if len(specs) != 1 {
		return responsesImageToolBillingDetail{}, "", nil
	}
	spec := specs[0]
	pricing, err := adminmodel.ResolveChannelModelPricing(channelProtocol, channelModels, spec.Model)
	if err != nil {
		return responsesImageToolBillingDetail{}, "", err
	}
	pricing = adminmodel.ResolveImageRequestPricing(pricing, spec.Size, spec.Quality)
	mode := billing.ResolveImageBillingMode(pricing)
	var (
		extraSnapshot     billing.BillingSnapshot
		errCompute        error
		imageOutputTokens int
	)
	switch mode {
	case billing.ImageBillingModePerImage, billing.ImageBillingModePerCall:
		extraSnapshot, errCompute = billing.ComputeImageBillingSnapshot(usage.ImageGenerationCalls, 1, pricing, groupRatio)
	case billing.ImageBillingModeTokenBased:
		if !supportsTraditionalImageTokenBilling(pricing) {
			return responsesImageToolBillingDetail{}, "", nil
		}
		if isGPTImage2Model(pricing.Model) {
			outputAmount, _, _, estimateErr := estimateGPTImage2OutputAmount(spec.Size, spec.Quality, usage.ImageGenerationCalls)
			if estimateErr != nil {
				return responsesImageToolBillingDetail{}, "", estimateErr
			}
			outputQuantity := outputAmount * 1000 / pricing.OutputPrice
			imageOutputTokens = int(math.Round(outputQuantity))
			extraSnapshot, errCompute = billing.ComputeResponseImageToolTokenBasedBillingSnapshot(outputQuantity, pricing, groupRatio)
			break
		}
		var estimateErr error
		imageOutputTokens, estimateErr = estimateTraditionalImageOutputTokens(spec.Model, spec.Size, spec.Quality, usage.ImageGenerationCalls)
		if estimateErr != nil {
			return responsesImageToolBillingDetail{}, "", estimateErr
		}
		extraSnapshot, errCompute = billing.ComputeResponseImageToolTokenBasedBillingSnapshot(float64(imageOutputTokens), pricing, groupRatio)
	default:
		return responsesImageToolBillingDetail{}, "", nil
	}
	if errCompute != nil {
		return responsesImageToolBillingDetail{}, "", errCompute
	}
	if snapshot.Currency != "" && extraSnapshot.Currency != "" && !strings.EqualFold(snapshot.Currency, extraSnapshot.Currency) {
		return responsesImageToolBillingDetail{}, "", nil
	}
	snapshot.InputAmount += extraSnapshot.Amount
	snapshot.Amount += extraSnapshot.Amount
	snapshot.ChargeAmount += extraSnapshot.ChargeAmount
	if snapshot.ChargeRate == 0 {
		snapshot.ChargeRate = extraSnapshot.ChargeRate
	}
	if !strings.EqualFold(strings.TrimSpace(snapshot.PriceUnit), strings.TrimSpace(extraSnapshot.PriceUnit)) {
		snapshot.PriceUnit = mixedBillingPriceUnit
	}
	snapshot.SettlementMode = billingSettlementModeUsagePlusImageFee
	detail := responsesImageToolBillingDetail{
		Model:        spec.Model,
		Calls:        usage.ImageGenerationCalls,
		OutputTokens: imageOutputTokens,
		Amount:       extraSnapshot.Amount,
		ChargeAmount: extraSnapshot.ChargeAmount,
		PriceUnit:    extraSnapshot.PriceUnit,
		Applied:      true,
	}
	snapshot.ImageToolCalls = detail.Calls
	snapshot.ImageToolOutputTokens = detail.OutputTokens
	snapshot.ImageToolAmount = detail.Amount
	snapshot.ImageToolChargeAmount = detail.ChargeAmount
	return detail, fmt.Sprintf("responses_image_fee model=%s calls=%d size=%s quality=%s unit=%s amount=%.6f quota=%d", spec.Model, usage.ImageGenerationCalls, spec.Size, spec.Quality, extraSnapshot.PriceUnit, extraSnapshot.Amount, extraSnapshot.ChargeAmount), nil
}
