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
	if logRow.BillingOfficialAnchorAmountCNY == 0 {
		logRow.BillingOfficialAnchorAmountCNY = convertBillingAmountToCNY(logRow.BillingOfficialAnchorAmount, logRow.BillingOfficialAnchorCurrency)
	}
	if logRow.BillingSellAmountCNY == 0 {
		logRow.BillingSellAmountCNY = convertYYCToCNY(float64(logRow.BillingYYCAmount))
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
		logRow.BillingGrossProfitCNY = logRow.BillingSellAmountCNY - logRow.BillingProcurementCostCNY
		if logRow.BillingSellAmountCNY > 0 {
			logRow.BillingGrossMargin = logRow.BillingGrossProfitCNY / logRow.BillingSellAmountCNY
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
	quantity := procurementConsumptionQuantity(logRow)
	if quantity <= 0 {
		return
	}
	result, err := model.ConsumeChannelProcurementBatches(model.ProcurementConsumeInput{
		RequestLogID:        logRow.Id,
		ChannelID:           logRow.ChannelId,
		ScopeType:           procurementScopeType(logRow),
		ScopeValue:          strings.TrimSpace(logRow.ModelName),
		CapacityUnit:        procurementCapacityUnit(logRow),
		Quantity:            quantity,
		SettlementTruthMode: strings.TrimSpace(logRow.BillingSettlementTruthMode),
	})
	if err != nil {
		logger.Errorf(ctx, "procurement consumption observation failed log_id=%s channel_id=%s model=%s err=%q", strings.TrimSpace(logRow.Id), strings.TrimSpace(logRow.ChannelId), strings.TrimSpace(logRow.ModelName), err.Error())
		return
	}
	if len(result.Consumptions) == 0 {
		return
	}
	if err := model.UpdateLogProcurementCostObservation(logRow.Id, result.TotalCostCNY, result.CostSource, logRow.BillingSellAmountCNY); err != nil {
		logger.Errorf(ctx, "procurement cost log update failed log_id=%s channel_id=%s model=%s err=%q", strings.TrimSpace(logRow.Id), strings.TrimSpace(logRow.ChannelId), strings.TrimSpace(logRow.ModelName), err.Error())
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

func convertBillingAmountToCNY(amount float64, currency string) float64 {
	if amount == 0 {
		return 0
	}
	sourceYYCPerUnit, err := model.GetBillingCurrencyYYCPerUnit(currency)
	if err != nil || sourceYYCPerUnit <= 0 {
		return 0
	}
	cnyYYCPerUnit, err := model.GetBillingCurrencyYYCPerUnit(model.BillingCurrencyCodeCNY)
	if err != nil || cnyYYCPerUnit <= 0 {
		return 0
	}
	return amount * sourceYYCPerUnit / cnyYYCPerUnit
}

func convertYYCToCNY(yycAmount float64) float64 {
	if yycAmount == 0 {
		return 0
	}
	cnyYYCPerUnit, err := model.GetBillingCurrencyYYCPerUnit(model.BillingCurrencyCodeCNY)
	if err != nil || cnyYYCPerUnit <= 0 {
		return 0
	}
	return yycAmount / cnyYYCPerUnit
}
