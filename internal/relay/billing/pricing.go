package billing

import (
	"fmt"
	"math"
	"strings"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/internal/admin/model"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

type BillingSnapshot struct {
	PriceUnit             string           `json:"price_unit,omitempty"`
	Currency              string           `json:"currency,omitempty"`
	PricingSource         string           `json:"pricing_source,omitempty"`
	UsageSource           string           `json:"usage_source,omitempty"`
	EstimateSource        string           `json:"estimate_source,omitempty"`
	SettlementMode        string           `json:"settlement_mode,omitempty"`
	EffectiveRatio        float64          `json:"effective_ratio,omitempty"`
	GroupChannelRatio     float64          `json:"group_channel_ratio,omitempty"`
	ModelChannelRatio     float64          `json:"model_channel_ratio,omitempty"`
	ChargeRate            float64          `json:"charge_rate,omitempty"`
	InputQuantity         float64          `json:"input_quantity,omitempty"`
	OutputQuantity        float64          `json:"output_quantity,omitempty"`
	CacheReadQuantity     float64          `json:"cache_read_quantity,omitempty"`
	CacheWriteQuantity    float64          `json:"cache_write_quantity,omitempty"`
	InputAmount           float64          `json:"input_amount,omitempty"`
	OutputAmount          float64          `json:"output_amount,omitempty"`
	CacheReadAmount       float64          `json:"cache_read_amount,omitempty"`
	CacheWriteAmount      float64          `json:"cache_write_amount,omitempty"`
	Amount                float64          `json:"amount,omitempty"`
	ChargeAmount          int64            `json:"charge_amount,omitempty"`
	PricingDecision       *PricingDecision `json:"pricing_decision,omitempty"`
	ImageToolCalls        int              `json:"image_tool_calls,omitempty"`
	ImageToolOutputTokens int              `json:"image_tool_output_tokens,omitempty"`
	ImageToolAmount       float64          `json:"image_tool_amount,omitempty"`
	ImageToolChargeAmount int64            `json:"image_tool_charge_amount,omitempty"`
}

type ImageBillingMode string

const (
	ImageBillingModeUnsupported ImageBillingMode = "unsupported"
	ImageBillingModePerImage    ImageBillingMode = "per_image"
	ImageBillingModePerCall     ImageBillingMode = "per_call"
	ImageBillingModeTokenBased  ImageBillingMode = "token_based"
)

func (snapshot BillingSnapshot) ApplyToLog(log *model.Log) {
	if log == nil {
		return
	}
	log.BillingPriceUnit = snapshot.PriceUnit
	log.BillingCurrency = snapshot.Currency
	log.BillingPricingSource = snapshot.PricingSource
	log.BillingUsageSource = snapshot.UsageSource
	log.BillingEstimateSource = snapshot.EstimateSource
	log.BillingSettlementMode = snapshot.SettlementMode
	log.BillingEffectiveRatio = snapshot.EffectiveRatio
	log.BillingGroupChannelRatio = snapshot.GroupChannelRatio
	log.BillingModelChannelRatio = snapshot.ModelChannelRatio
	log.BillingChargeRate = snapshot.ChargeRate
	log.BillingInputQuantity = snapshot.InputQuantity
	log.BillingOutputQuantity = snapshot.OutputQuantity
	log.BillingCacheReadQuantity = snapshot.CacheReadQuantity
	log.BillingCacheWriteQuantity = snapshot.CacheWriteQuantity
	log.BillingInputAmount = snapshot.InputAmount
	log.BillingOutputAmount = snapshot.OutputAmount
	log.BillingCacheReadAmount = snapshot.CacheReadAmount
	log.BillingCacheWriteAmount = snapshot.CacheWriteAmount
	log.BillingAmount = snapshot.Amount
	log.BillingChargeAmount = snapshot.ChargeAmount
	if snapshot.PricingDecision != nil {
		log.BillingOfficialAnchorAmount = snapshot.Amount
		log.BillingOfficialAnchorCurrency = snapshot.Currency
		log.BillingOfficialAnchorBaseAmount = snapshot.PricingDecision.OfficialAnchor.Amount
		log.BillingSellBaseAmount = snapshot.PricingDecision.SelectedSell.Amount
		log.BillingCostFloorBaseAmount = snapshot.PricingDecision.CostFloor.Amount
		log.BillingSelectedSellBaseAmount = snapshot.PricingDecision.SelectedSell.Amount
		log.BillingPricingDecisionReason = snapshot.PricingDecision.Reason
		log.BillingCostFloorTriggered = snapshot.PricingDecision.Reason == PricingDecisionReasonCostFloor
	}
	log.BillingImageToolCalls = snapshot.ImageToolCalls
	log.BillingImageToolOutputTokens = snapshot.ImageToolOutputTokens
	log.BillingImageToolAmount = snapshot.ImageToolAmount
	log.BillingImageToolChargeAmount = snapshot.ImageToolChargeAmount
	if snapshot.PricingDecision != nil {
		switch snapshot.PricingDecision.Reason {
		case PricingDecisionReasonCostFloor:
			log.BillingPricingRuleVersion = PricingRuleVersionCostFloorV1
		default:
			log.BillingPricingRuleVersion = PricingRuleVersionOfficialAnchorV1
		}
	}
}

func (snapshot *BillingSnapshot) SetBillingRatioBreakdown(ratio model.BillingRatioBreakdown) {
	if snapshot == nil {
		return
	}
	snapshot.EffectiveRatio = ratio.EffectiveRatio
	snapshot.GroupChannelRatio = ratio.GroupChannelRatio
	snapshot.ModelChannelRatio = ratio.ModelChannelRatio
}

func ComputeTextPreConsumedQuota(promptTokens int, maxCompletionTokens int, pricing model.ResolvedModelPricing, groupRatio float64) (int64, error) {
	snapshot, err := ComputeTextPreConsumedBillingSnapshot(promptTokens, maxCompletionTokens, pricing, groupRatio)
	if err != nil {
		return 0, err
	}
	return snapshot.ChargeAmount, nil
}

func ComputeTextPreConsumedBillingSnapshot(promptTokens int, maxCompletionTokens int, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	completionBudget := float64(config.PreConsumedQuota)
	if maxCompletionTokens > 0 {
		completionBudget += float64(maxCompletionTokens)
	}
	return buildBillingSnapshot(
		float64(promptTokens),
		completionBudget,
		pricing.InputPrice,
		pricing.OutputPrice,
		pricing,
		groupRatio,
		promptTokens > 0 || maxCompletionTokens > 0,
	)
}

func ComputeTextQuota(promptTokens int, completionTokens int, pricing model.ResolvedModelPricing, groupRatio float64) (int64, error) {
	snapshot, err := ComputeTextBillingSnapshot(promptTokens, completionTokens, pricing, groupRatio)
	if err != nil {
		return 0, err
	}
	return snapshot.ChargeAmount, nil
}

func ComputeTextBillingSnapshot(promptTokens int, completionTokens int, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	return buildBillingSnapshot(
		float64(promptTokens),
		float64(completionTokens),
		pricing.InputPrice,
		pricing.OutputPrice,
		pricing,
		groupRatio,
		promptTokens > 0 || completionTokens > 0,
	)
}

func ComputeTextBillingSnapshotWithUsage(usage relaymodel.Usage, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	promptTokens := usage.PromptTokens
	completionTokens := usage.CompletionTokens
	cacheReadTokens := 0
	cacheWriteTokens := 0
	if usage.PromptTokensDetails != nil {
		cacheReadTokens = usage.PromptTokensDetails.CacheReadTokens
		if cacheReadTokens <= 0 {
			cacheReadTokens = usage.PromptTokensDetails.CachedTokens
		}
		cacheWriteTokens = usage.PromptTokensDetails.CacheCreationTokens
	}
	if cacheReadTokens < 0 {
		cacheReadTokens = 0
	}
	if cacheWriteTokens < 0 {
		cacheWriteTokens = 0
	}
	if cacheReadTokens > promptTokens {
		cacheReadTokens = promptTokens
	}
	if cacheWriteTokens > promptTokens-cacheReadTokens {
		cacheWriteTokens = promptTokens - cacheReadTokens
	}
	regularInputTokens := promptTokens - cacheReadTokens - cacheWriteTokens
	if regularInputTokens < 0 {
		regularInputTokens = 0
	}
	inputAmount := billingAmountFromPrice(pricing.InputPrice, pricing.PriceUnit, float64(regularInputTokens))
	cacheReadAmount := billingAmountFromPrice(resolveTextCacheComponentPrice(pricing, model.ProviderModelPriceComponentTextCacheRead, pricing.InputPrice, promptTokens), pricing.PriceUnit, float64(cacheReadTokens))
	cacheWriteAmount := billingAmountFromPrice(resolveTextCacheComponentPrice(pricing, model.ProviderModelPriceComponentTextCacheWrite, pricing.InputPrice, promptTokens), pricing.PriceUnit, float64(cacheWriteTokens))
	outputAmount := billingAmountFromPrice(pricing.OutputPrice, pricing.PriceUnit, float64(completionTokens))
	snapshot, err := ComputeExplicitAmountBillingSnapshot(
		float64(promptTokens),
		float64(completionTokens),
		inputAmount+cacheReadAmount+cacheWriteAmount,
		outputAmount,
		pricing,
		groupRatio,
		promptTokens > 0 || completionTokens > 0,
	)
	if err != nil {
		return BillingSnapshot{}, err
	}
	snapshot.CacheReadQuantity = float64(cacheReadTokens)
	snapshot.CacheWriteQuantity = float64(cacheWriteTokens)
	snapshot.CacheReadAmount = cacheReadAmount
	snapshot.CacheWriteAmount = cacheWriteAmount
	return snapshot, nil
}

func resolveTextCacheComponentPrice(pricing model.ResolvedModelPricing, componentType string, fallbackPrice float64, promptTokens int) float64 {
	component, ok := model.SelectProviderPriceComponent(
		pricing.PriceComponents,
		strings.TrimSpace(strings.ToLower(componentType)),
		map[string]string{
			"mode":          "standard",
			"input_type":    "text_image_video",
			"prompt_tokens": fmt.Sprintf("%d", promptTokens),
		},
	)
	if ok {
		if component.InputPrice > 0 {
			return component.InputPrice
		}
		if component.OutputPrice > 0 {
			return component.OutputPrice
		}
	}
	return fallbackPrice
}

func ComputeImageQuota(imageCount int, multiplier float64, pricing model.ResolvedModelPricing, groupRatio float64) (int64, error) {
	snapshot, err := ComputeImageBillingSnapshot(imageCount, multiplier, pricing, groupRatio)
	if err != nil {
		return 0, err
	}
	return snapshot.ChargeAmount, nil
}

func ComputeImageBillingSnapshot(imageCount int, multiplier float64, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	switch ResolveImageBillingMode(pricing) {
	case ImageBillingModePerImage:
		return ComputeImagePerImageBillingSnapshot(imageCount, multiplier, pricing, groupRatio)
	case ImageBillingModePerCall:
		return ComputeImagePerCallBillingSnapshot(imageCount, pricing, groupRatio)
	case ImageBillingModeTokenBased:
		return BillingSnapshot{}, fmt.Errorf("image token-based billing requires explicit usage for model %s", strings.TrimSpace(pricing.Model))
	default:
		return BillingSnapshot{}, fmt.Errorf("unsupported image billing mode for model %s with price_unit %s", strings.TrimSpace(pricing.Model), strings.TrimSpace(pricing.PriceUnit))
	}
}

func ComputeTraditionalImageTokenBasedBillingSnapshot(promptTokens int, imageOutputTokens int, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	return ComputeTokenBasedBillingSnapshot(float64(promptTokens), float64(imageOutputTokens), pricing, groupRatio)
}

func ComputeTokenBasedBillingSnapshot(inputQuantity float64, outputQuantity float64, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	if ResolveImageBillingMode(pricing) != ImageBillingModeTokenBased {
		return BillingSnapshot{}, fmt.Errorf("traditional image token-based billing requires token-based pricing for model %s", strings.TrimSpace(pricing.Model))
	}
	return buildBillingSnapshot(
		inputQuantity,
		outputQuantity,
		pricing.InputPrice,
		pricing.OutputPrice,
		pricing,
		groupRatio,
		inputQuantity > 0 || outputQuantity > 0,
	)
}

func ComputeResponseImageToolTokenBasedBillingSnapshot(outputQuantity float64, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	if ResolveImageBillingMode(pricing) != ImageBillingModeTokenBased {
		return BillingSnapshot{}, fmt.Errorf("responses image tool token-based billing requires token-based pricing for model %s", strings.TrimSpace(pricing.Model))
	}
	return buildBillingSnapshot(
		0,
		outputQuantity,
		0,
		pricing.OutputPrice,
		pricing,
		groupRatio,
		outputQuantity > 0,
	)
}

func ComputeExplicitAmountBillingSnapshot(inputQuantity float64, outputQuantity float64, inputAmount float64, outputAmount float64, pricing model.ResolvedModelPricing, groupRatio float64, hasUsage bool) (BillingSnapshot, error) {
	snapshot := BillingSnapshot{
		PriceUnit:      normalizePriceUnit(pricing.PriceUnit),
		Currency:       normalizeCurrency(pricing.Currency),
		EffectiveRatio: groupRatio,
		InputQuantity:  inputQuantity,
		OutputQuantity: outputQuantity,
		InputAmount:    inputAmount,
		OutputAmount:   outputAmount,
	}
	snapshot.Amount = snapshot.InputAmount + snapshot.OutputAmount
	if snapshot.Amount > 0 {
		chargeRate, err := model.GetBillingCurrencyChargeRate(snapshot.Currency)
		if err != nil {
			if groupRatio != 0 {
				return BillingSnapshot{}, err
			}
		} else {
			snapshot.ChargeRate = chargeRate
		}
	}
	rawChargeAmount := snapshot.Amount * snapshot.ChargeRate * groupRatio
	snapshot.ChargeAmount = normalizeQuota(rawChargeAmount, hasUsage, pricing, groupRatio)
	applyPricingDecision(&snapshot)
	return snapshot, nil
}

func ResolveImageBillingMode(pricing model.ResolvedModelPricing) ImageBillingMode {
	switch normalizePriceUnit(pricing.PriceUnit) {
	case model.ProviderPriceUnitPerImage:
		return ImageBillingModePerImage
	case model.ProviderPriceUnitPerRequest, model.ProviderPriceUnitPerTask:
		return ImageBillingModePerCall
	case "", model.ProviderPriceUnitPer1KTokens, model.ProviderPriceUnitPer1KChars:
		return ImageBillingModeTokenBased
	default:
		return ImageBillingModeUnsupported
	}
}

func ComputeImagePerImageQuota(imageCount int, multiplier float64, pricing model.ResolvedModelPricing, groupRatio float64) (int64, error) {
	snapshot, err := ComputeImagePerImageBillingSnapshot(imageCount, multiplier, pricing, groupRatio)
	if err != nil {
		return 0, err
	}
	return snapshot.ChargeAmount, nil
}

func ComputeImagePerImageBillingSnapshot(imageCount int, multiplier float64, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	if imageCount <= 0 {
		return BillingSnapshot{
			PriceUnit:      normalizePriceUnit(pricing.PriceUnit),
			Currency:       normalizeCurrency(pricing.Currency),
			EffectiveRatio: groupRatio,
		}, nil
	}
	quantity := float64(imageCount) * multiplier
	return buildSingleSidedBillingSnapshot(quantity, primaryUnitPrice(pricing), pricing, groupRatio)
}

func ComputeImagePerCallQuota(imageCount int, pricing model.ResolvedModelPricing, groupRatio float64) (int64, error) {
	snapshot, err := ComputeImagePerCallBillingSnapshot(imageCount, pricing, groupRatio)
	if err != nil {
		return 0, err
	}
	return snapshot.ChargeAmount, nil
}

func ComputeImagePerCallBillingSnapshot(imageCount int, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	if imageCount <= 0 {
		return BillingSnapshot{
			PriceUnit:      normalizePriceUnit(pricing.PriceUnit),
			Currency:       normalizeCurrency(pricing.Currency),
			EffectiveRatio: groupRatio,
		}, nil
	}
	return buildSingleSidedBillingSnapshot(1, primaryUnitPrice(pricing), pricing, groupRatio)
}

func ComputeAudioSpeechQuota(charCount int, pricing model.ResolvedModelPricing, groupRatio float64) (int64, error) {
	snapshot, err := ComputeAudioSpeechBillingSnapshot(charCount, pricing, groupRatio)
	if err != nil {
		return 0, err
	}
	return snapshot.ChargeAmount, nil
}

func ComputeAudioSpeechBillingSnapshot(charCount int, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	if charCount <= 0 {
		return BillingSnapshot{
			PriceUnit:      normalizePriceUnit(pricing.PriceUnit),
			Currency:       normalizeCurrency(pricing.Currency),
			EffectiveRatio: groupRatio,
		}, nil
	}
	return buildSingleSidedBillingSnapshot(float64(charCount), primaryUnitPrice(pricing), pricing, groupRatio)
}

func ComputeAudioTextQuota(tokenCount int, pricing model.ResolvedModelPricing, groupRatio float64) (int64, error) {
	snapshot, err := ComputeAudioTextBillingSnapshot(tokenCount, pricing, groupRatio)
	if err != nil {
		return 0, err
	}
	return snapshot.ChargeAmount, nil
}

func ComputeAudioTextBillingSnapshot(tokenCount int, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	if tokenCount <= 0 {
		return BillingSnapshot{
			PriceUnit:      normalizePriceUnit(pricing.PriceUnit),
			Currency:       normalizeCurrency(pricing.Currency),
			EffectiveRatio: groupRatio,
		}, nil
	}
	return buildSingleSidedBillingSnapshot(float64(tokenCount), primaryUnitPrice(pricing), pricing, groupRatio)
}

func ComputeVideoQuota(quantity float64, pricing model.ResolvedModelPricing, groupRatio float64) (int64, error) {
	snapshot, err := ComputeVideoBillingSnapshot(quantity, pricing, groupRatio)
	if err != nil {
		return 0, err
	}
	return snapshot.ChargeAmount, nil
}

func ComputeVideoBillingSnapshot(quantity float64, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	if quantity <= 0 {
		return BillingSnapshot{
			PriceUnit:      normalizePriceUnit(pricing.PriceUnit),
			Currency:       normalizeCurrency(pricing.Currency),
			EffectiveRatio: groupRatio,
		}, nil
	}
	return buildSingleSidedBillingSnapshot(quantity, primaryUnitPrice(pricing), pricing, groupRatio)
}

func FormatPricingLog(pricing model.ResolvedModelPricing, groupRatio float64) string {
	source := strings.TrimSpace(pricing.Source)
	if source == "" {
		source = "unknown"
	}
	component := strings.TrimSpace(pricing.MatchedComponent)
	condition := strings.TrimSpace(pricing.MatchedCondition)
	return fmt.Sprintf(
		"计费: source=%s provider=%s type=%s component=%s condition=%s unit=%s currency=%s input=%.6f output=%.6f effective_ratio=%.6f",
		source,
		strings.TrimSpace(pricing.Provider),
		strings.TrimSpace(pricing.Type),
		component,
		condition,
		strings.TrimSpace(pricing.PriceUnit),
		strings.TrimSpace(pricing.Currency),
		pricing.InputPrice,
		pricing.OutputPrice,
		groupRatio,
	)
}

func chargeAmountFromPrice(price float64, priceUnit string, currency string, quantity float64, groupRatio float64) (float64, error) {
	if price <= 0 || quantity <= 0 || groupRatio == 0 {
		return 0, nil
	}
	chargeRate, err := model.GetBillingCurrencyChargeRate(currency)
	if err != nil {
		return 0, err
	}
	normalizedUnit := strings.TrimSpace(strings.ToLower(priceUnit))
	switch normalizedUnit {
	case "", model.ProviderPriceUnitPer1KTokens, model.ProviderPriceUnitPer1KChars:
		return quantity * price * chargeRate / 1000 * groupRatio, nil
	case model.ProviderPriceUnitPerImage,
		model.ProviderPriceUnitPerVideo,
		model.ProviderPriceUnitPerSecond,
		model.ProviderPriceUnitPerMinute,
		model.ProviderPriceUnitPerRequest,
		model.ProviderPriceUnitPerTask:
		return quantity * price * chargeRate * groupRatio, nil
	default:
		return quantity * price * chargeRate / 1000 * groupRatio, nil
	}
}

func buildSingleSidedBillingSnapshot(quantity float64, price float64, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	return buildBillingSnapshot(
		quantity,
		0,
		price,
		0,
		pricing,
		groupRatio,
		quantity > 0,
	)
}

func buildBillingSnapshot(inputQuantity float64, outputQuantity float64, inputPrice float64, outputPrice float64, pricing model.ResolvedModelPricing, groupRatio float64, hasUsage bool) (BillingSnapshot, error) {
	snapshot := BillingSnapshot{
		PriceUnit:      normalizePriceUnit(pricing.PriceUnit),
		Currency:       normalizeCurrency(pricing.Currency),
		EffectiveRatio: groupRatio,
		InputQuantity:  inputQuantity,
		OutputQuantity: outputQuantity,
	}
	snapshot.InputAmount = billingAmountFromPrice(inputPrice, snapshot.PriceUnit, inputQuantity)
	snapshot.OutputAmount = billingAmountFromPrice(outputPrice, snapshot.PriceUnit, outputQuantity)
	snapshot.Amount = snapshot.InputAmount + snapshot.OutputAmount
	if snapshot.Amount > 0 {
		chargeRate, err := model.GetBillingCurrencyChargeRate(snapshot.Currency)
		if err != nil {
			if groupRatio != 0 {
				return BillingSnapshot{}, err
			}
		} else {
			snapshot.ChargeRate = chargeRate
		}
	}
	rawChargeAmount := snapshot.Amount * snapshot.ChargeRate * groupRatio
	snapshot.ChargeAmount = normalizeQuota(rawChargeAmount, hasUsage, pricing, groupRatio)
	applyPricingDecision(&snapshot)
	return snapshot, nil
}

func applyPricingDecision(snapshot *BillingSnapshot) {
	applyPricingDecisionWithProcurementCost(snapshot, MoneyAmount{})
}

func applyPricingDecisionWithProcurementCost(snapshot *BillingSnapshot, procurementCost MoneyAmount) {
	if snapshot == nil {
		return
	}
	decision := DecidePricing(PricingDecisionInput{
		OfficialAnchor: MoneyAmount{
			Amount:   snapshot.Amount,
			Currency: snapshot.Currency,
		},
		CurrentCharge: MoneyAmount{
			Amount:   float64(snapshot.ChargeAmount),
			Currency: model.BillingCurrencyCodeYYC,
		},
		ProcurementCost: procurementCost,
		Policy:          CurrentPricingPolicy(),
	})
	snapshot.PricingDecision = &decision
	if decision.SelectedCharge.Amount > float64(snapshot.ChargeAmount) {
		snapshot.ChargeAmount = int64(decision.SelectedCharge.Amount)
	}
}

func ApplyEstimatedProcurementCostFloor(snapshot *BillingSnapshot, channelID string, modelName string) error {
	if snapshot == nil {
		return nil
	}
	candidates := procurementConsumptionCandidatesFromSnapshot(snapshot)
	if len(candidates) == 0 {
		return nil
	}
	for _, candidate := range candidates {
		result, err := model.EstimateChannelProcurementCost(model.ProcurementConsumeInput{
			ChannelID:    channelID,
			ScopeType:    procurementScopeTypeFromModelName(modelName),
			ScopeValue:   strings.TrimSpace(modelName),
			CapacityUnit: candidate.CapacityUnit,
			Quantity:     candidate.Quantity,
		})
		if err != nil {
			return err
		}
		// A cost floor may affect a user's charge only when the full usage is
		// covered by authoritative procurement data. Estimated or partial cost
		// coverage belongs in reconciliation, not in the online charge path.
		if result.CoveredQuantity <= 0 || result.MissingQuantity > 0 || result.TotalCostAmount <= 0 {
			continue
		}
		if result.CostSource != model.ProcurementCostSourceActual && result.CostSource != model.ProcurementCostSourceZeroCost {
			continue
		}
		applyPricingDecisionWithProcurementCost(snapshot, MoneyAmount{
			Amount:   result.TotalCostAmount,
			Currency: model.BillingCurrencyCodeCNY,
		})
		return nil
	}
	return nil
}

func procurementConsumptionCandidatesFromSnapshot(snapshot *BillingSnapshot) []procurementConsumptionCandidate {
	if snapshot == nil {
		return nil
	}
	candidates := make([]procurementConsumptionCandidate, 0, 2)
	seen := map[string]struct{}{}
	appendCandidate := func(capacityUnit string, quantity float64) {
		normalizedUnit := strings.TrimSpace(strings.ToLower(capacityUnit))
		if normalizedUnit == "" || quantity <= 0 {
			return
		}
		if _, ok := seen[normalizedUnit]; ok {
			return
		}
		seen[normalizedUnit] = struct{}{}
		candidates = append(candidates, procurementConsumptionCandidate{
			CapacityUnit: normalizedUnit,
			Quantity:     quantity,
		})
	}
	appendCandidate(procurementCurrencyEquivalentCapacityUnitFromSnapshot(snapshot), snapshot.Amount)
	appendCandidate(procurementCapacityUnitFromSnapshot(snapshot), procurementConsumptionQuantityFromSnapshot(snapshot))
	return candidates
}

func procurementCurrencyEquivalentCapacityUnitFromSnapshot(snapshot *BillingSnapshot) string {
	if snapshot == nil {
		return ""
	}
	currency := strings.TrimSpace(strings.ToLower(snapshot.Currency))
	if currency == "" {
		return ""
	}
	return currency + "_equivalent"
}

func procurementCapacityUnitFromSnapshot(snapshot *BillingSnapshot) string {
	if snapshot == nil {
		return ""
	}
	switch strings.TrimSpace(strings.ToLower(snapshot.PriceUnit)) {
	case model.ProviderPriceUnitPerImage:
		return "image"
	case model.ProviderPriceUnitPerRequest, model.ProviderPriceUnitPerTask:
		return "request"
	case model.ProviderPriceUnitPer1KChars:
		return "char"
	case model.ProviderPriceUnitPerSecond:
		return "second"
	case model.ProviderPriceUnitPerMinute:
		return "minute"
	case model.ProviderPriceUnitPerVideo:
		return "video"
	case "", model.ProviderPriceUnitPer1KTokens:
		return "token"
	default:
		return "token"
	}
}

func procurementConsumptionQuantityFromSnapshot(snapshot *BillingSnapshot) float64 {
	if snapshot == nil {
		return 0
	}
	switch procurementCapacityUnitFromSnapshot(snapshot) {
	case "token":
		return snapshot.InputQuantity + snapshot.OutputQuantity + snapshot.CacheReadQuantity + snapshot.CacheWriteQuantity
	default:
		return snapshot.InputQuantity + snapshot.OutputQuantity
	}
}

func procurementScopeTypeFromModelName(modelName string) string {
	if strings.TrimSpace(modelName) == "" {
		return "global"
	}
	return "model"
}

func primaryUnitPrice(pricing model.ResolvedModelPricing) float64 {
	if pricing.InputPrice > 0 {
		return pricing.InputPrice
	}
	if pricing.OutputPrice > 0 {
		return pricing.OutputPrice
	}
	return 0
}

func billingAmountFromPrice(price float64, priceUnit string, quantity float64) float64 {
	if price <= 0 || quantity <= 0 {
		return 0
	}
	switch normalizePriceUnit(priceUnit) {
	case "", model.ProviderPriceUnitPer1KTokens, model.ProviderPriceUnitPer1KChars:
		return quantity * price / 1000
	case model.ProviderPriceUnitPerImage,
		model.ProviderPriceUnitPerVideo,
		model.ProviderPriceUnitPerSecond,
		model.ProviderPriceUnitPerMinute,
		model.ProviderPriceUnitPerRequest,
		model.ProviderPriceUnitPerTask:
		return quantity * price
	default:
		return quantity * price / 1000
	}
}

func normalizePriceUnit(priceUnit string) string {
	return strings.TrimSpace(strings.ToLower(priceUnit))
}

func normalizeCurrency(currency string) string {
	normalized := strings.TrimSpace(strings.ToUpper(currency))
	if normalized == "" {
		return model.ProviderPriceCurrencyUSD
	}
	return normalized
}

func normalizeQuota(raw float64, hasUsage bool, pricing model.ResolvedModelPricing, groupRatio float64) int64 {
	if raw <= 0 {
		if hasUsage && groupRatio != 0 && pricing.IsConfigured() {
			return 1
		}
		return 0
	}
	return int64(math.Ceil(raw))
}
