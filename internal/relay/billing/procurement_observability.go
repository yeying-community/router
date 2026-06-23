package billing

import (
	"context"
	"strings"

	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
)

const (
	SettlementTruthModeReturnedUsageFinal = "returned_usage_final"
	SettlementTruthModeHybridUsageFinal   = "hybrid_usage_final"
	SettlementTruthModeLocalEstimateFinal = "local_estimate_final"
	SettlementTruthModeUnitBasedFinal     = "unit_based_final"

	ProcurementCostConfidenceReturnedUsage = "returned_usage"
	ProcurementCostConfidenceHybridUsage   = "hybrid_usage"
	ProcurementCostConfidenceLocalEstimate = "local_estimate"
	ProcurementCostConfidenceUnitBased     = "unit_based"

	PricingRuleVersionOfficialAnchorV1 = "official_anchor_v1"
	PricingRuleVersionCostFloorV1      = "cost_floor_v1"
	CostRuleVersionUnconfiguredV1      = "procurement_unconfigured_v1"
)

func ApplyProcurementCostObservation(logRow *model.Log) {
	if logRow == nil {
		return
	}
	if strings.TrimSpace(logRow.BillingSettlementTruthMode) == "" {
		logRow.BillingSettlementTruthMode = inferSettlementTruthMode(logRow)
	}
	if strings.TrimSpace(logRow.BillingProcurementCostConfidence) == "" {
		logRow.BillingProcurementCostConfidence = confidenceFromSettlementTruthMode(logRow.BillingSettlementTruthMode)
	}
	if strings.TrimSpace(logRow.BillingOfficialAnchorCurrency) == "" {
		logRow.BillingOfficialAnchorCurrency = strings.TrimSpace(logRow.BillingCurrency)
	}
	if logRow.BillingOfficialAnchorAmount == 0 {
		logRow.BillingOfficialAnchorAmount = logRow.BillingAmount
	}
	if logRow.BillingOfficialAnchorBaseAmount == 0 {
		logRow.BillingOfficialAnchorBaseAmount = convertBillingAmountToBaseAmount(logRow.BillingOfficialAnchorAmount, logRow.BillingOfficialAnchorCurrency)
	}
	if logRow.BillingSellBaseAmount == 0 {
		logRow.BillingSellBaseAmount = convertChargeAmountToBaseAmount(float64(logRow.BillingChargeAmount))
	}
	if strings.TrimSpace(logRow.BillingProcurementCostSource) == "" {
		logRow.BillingProcurementCostSource = model.ProcurementCostSourceNone
	}
	if strings.TrimSpace(logRow.BillingPricingRuleVersion) == "" {
		logRow.BillingPricingRuleVersion = PricingRuleVersionOfficialAnchorV1
	}
	if strings.TrimSpace(logRow.BillingCostRuleVersion) == "" {
		logRow.BillingCostRuleVersion = CostRuleVersionUnconfiguredV1
	}
	// Gross profit is only meaningful after actual or explicitly estimated procurement cost is attached.
	if logRow.BillingProcurementCostSource == model.ProcurementCostSourceActual ||
		logRow.BillingProcurementCostSource == model.ProcurementCostSourceEstimated ||
		logRow.BillingProcurementCostSource == model.ProcurementCostSourceZeroCost {
		logRow.BillingGrossProfitBaseAmount = logRow.BillingSellBaseAmount - logRow.BillingProcurementCostBaseAmount
		if logRow.BillingSellBaseAmount > 0 {
			logRow.BillingGrossMargin = logRow.BillingGrossProfitBaseAmount / logRow.BillingSellBaseAmount
		}
	}
}

func RecordProcurementConsumptionObservation(ctx context.Context, logRow *model.Log) {
	if logRow == nil {
		return
	}
	if strings.TrimSpace(logRow.Id) == "" || strings.TrimSpace(logRow.ChannelId) == "" {
		return
	}
	candidates := procurementConsumptionCandidates(logRow)
	if len(candidates) == 0 {
		return
	}
	for _, candidate := range candidates {
		result, err := model.ConsumeChannelProcurementBatches(model.ProcurementConsumeInput{
			RequestLogID:        logRow.Id,
			ChannelID:           logRow.ChannelId,
			ScopeType:           procurementScopeType(logRow),
			ScopeValue:          strings.TrimSpace(logRow.ModelName),
			CapacityUnit:        candidate.CapacityUnit,
			Quantity:            candidate.Quantity,
			SettlementTruthMode: strings.TrimSpace(logRow.BillingSettlementTruthMode),
		})
		if err != nil {
			logger.Errorf(ctx, "procurement consumption observation failed log_id=%s channel_id=%s model=%s capacity_unit=%s quantity=%f err=%q", strings.TrimSpace(logRow.Id), strings.TrimSpace(logRow.ChannelId), strings.TrimSpace(logRow.ModelName), strings.TrimSpace(candidate.CapacityUnit), candidate.Quantity, err.Error())
			return
		}
		if len(result.Consumptions) == 0 {
			continue
		}
		if err := model.UpdateLogProcurementCostObservation(logRow.Id, result.TotalCostAmount, result.CostSource, logRow.BillingSellBaseAmount); err != nil {
			logger.Errorf(ctx, "procurement cost log update failed log_id=%s channel_id=%s model=%s err=%q", strings.TrimSpace(logRow.Id), strings.TrimSpace(logRow.ChannelId), strings.TrimSpace(logRow.ModelName), err.Error())
		}
		return
	}
}

func inferSettlementTruthMode(logRow *model.Log) string {
	switch strings.TrimSpace(logRow.BillingSettlementMode) {
	case "provider_usage_final":
		return SettlementTruthModeReturnedUsageFinal
	case "local_estimate_final":
		return SettlementTruthModeLocalEstimateFinal
	case "estimate_only":
		return SettlementTruthModeUnitBasedFinal
	case "usage_final", "responses_image_tool_pending", "usage_plus_image_fee":
		return SettlementTruthModeHybridUsageFinal
	}
	switch strings.TrimSpace(logRow.BillingPriceUnit) {
	case model.ProviderPriceUnitPerImage,
		model.ProviderPriceUnitPerRequest,
		model.ProviderPriceUnitPerTask,
		model.ProviderPriceUnitPer1KChars,
		model.ProviderPriceUnitPerSecond,
		model.ProviderPriceUnitPerMinute,
		model.ProviderPriceUnitPerVideo:
		return SettlementTruthModeUnitBasedFinal
	default:
		return SettlementTruthModeHybridUsageFinal
	}
}

func confidenceFromSettlementTruthMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case SettlementTruthModeReturnedUsageFinal:
		return ProcurementCostConfidenceReturnedUsage
	case SettlementTruthModeLocalEstimateFinal:
		return ProcurementCostConfidenceLocalEstimate
	case SettlementTruthModeUnitBasedFinal:
		return ProcurementCostConfidenceUnitBased
	default:
		return ProcurementCostConfidenceHybridUsage
	}
}

func procurementScopeType(logRow *model.Log) string {
	if strings.TrimSpace(logRow.ModelName) == "" {
		return "global"
	}
	return "model"
}

func procurementCapacityUnit(logRow *model.Log) string {
	switch strings.TrimSpace(logRow.BillingPriceUnit) {
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
	case model.ProviderPriceUnitPer1KTokens, "":
		return "token"
	default:
		return strings.TrimSpace(strings.ToLower(logRow.BillingPriceUnit))
	}
}

type procurementConsumptionCandidate struct {
	CapacityUnit string
	Quantity     float64
}

func procurementConsumptionCandidates(logRow *model.Log) []procurementConsumptionCandidate {
	if logRow == nil {
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
	appendCandidate(procurementCurrencyEquivalentCapacityUnit(logRow), logRow.BillingAmount)
	appendCandidate(procurementCapacityUnit(logRow), procurementConsumptionQuantity(logRow))
	return candidates
}

func procurementCurrencyEquivalentCapacityUnit(logRow *model.Log) string {
	if logRow == nil {
		return ""
	}
	currency := strings.TrimSpace(strings.ToLower(logRow.BillingCurrency))
	if currency == "" {
		return ""
	}
	return currency + "_equivalent"
}

func procurementConsumptionQuantity(logRow *model.Log) float64 {
	switch procurementCapacityUnit(logRow) {
	case "image", "request", "char", "second", "minute", "video":
		return logRow.BillingInputQuantity + logRow.BillingOutputQuantity
	case "token":
		return logRow.BillingInputQuantity + logRow.BillingOutputQuantity
	default:
		return logRow.BillingInputQuantity + logRow.BillingOutputQuantity
	}
}

func convertBillingAmountToBaseAmount(amount float64, currency string) float64 {
	if amount == 0 {
		return 0
	}
	sourceChargeRate, err := model.GetBillingCurrencyChargeRate(currency)
	if err != nil || sourceChargeRate <= 0 {
		return 0
	}
	cnyChargeRate, err := model.GetBillingCurrencyChargeRate(model.BillingCurrencyCodeCNY)
	if err != nil || cnyChargeRate <= 0 {
		return 0
	}
	return amount * sourceChargeRate / cnyChargeRate
}

func convertChargeAmountToBaseAmount(chargeAmount float64) float64 {
	if chargeAmount == 0 {
		return 0
	}
	cnyChargeRate, err := model.GetBillingCurrencyChargeRate(model.BillingCurrencyCodeCNY)
	if err != nil || cnyChargeRate <= 0 {
		return 0
	}
	return chargeAmount / cnyChargeRate
}
