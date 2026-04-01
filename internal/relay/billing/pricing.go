package billing

import (
	"fmt"
	"math"
	"strings"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/internal/admin/model"
)

type BillingSnapshot struct {
	PriceUnit      string  `json:"price_unit,omitempty"`
	Currency       string  `json:"currency,omitempty"`
	GroupRatio     float64 `json:"group_ratio,omitempty"`
	YYCRate        float64 `json:"yyc_rate,omitempty"`
	InputQuantity  float64 `json:"input_quantity,omitempty"`
	OutputQuantity float64 `json:"output_quantity,omitempty"`
	InputAmount    float64 `json:"input_amount,omitempty"`
	OutputAmount   float64 `json:"output_amount,omitempty"`
	Amount         float64 `json:"amount,omitempty"`
	YYCAmount      int64   `json:"yyc_amount,omitempty"`
}

func (snapshot BillingSnapshot) ApplyToLog(log *model.Log) {
	if log == nil {
		return
	}
	log.BillingPriceUnit = snapshot.PriceUnit
	log.BillingCurrency = snapshot.Currency
	log.BillingGroupRatio = snapshot.GroupRatio
	log.BillingYYCRate = snapshot.YYCRate
	log.BillingInputQuantity = snapshot.InputQuantity
	log.BillingOutputQuantity = snapshot.OutputQuantity
	log.BillingInputAmount = snapshot.InputAmount
	log.BillingOutputAmount = snapshot.OutputAmount
	log.BillingAmount = snapshot.Amount
	log.BillingYYCAmount = snapshot.YYCAmount
}

func ComputeTextPreConsumedQuota(promptTokens int, maxCompletionTokens int, pricing model.ResolvedModelPricing, groupRatio float64) (int64, error) {
	snapshot, err := ComputeTextPreConsumedBillingSnapshot(promptTokens, maxCompletionTokens, pricing, groupRatio)
	if err != nil {
		return 0, err
	}
	return snapshot.YYCAmount, nil
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
	return snapshot.YYCAmount, nil
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

func ComputeImageQuota(imageCount int, multiplier float64, pricing model.ResolvedModelPricing, groupRatio float64) (int64, error) {
	snapshot, err := ComputeImageBillingSnapshot(imageCount, multiplier, pricing, groupRatio)
	if err != nil {
		return 0, err
	}
	return snapshot.YYCAmount, nil
}

func ComputeImageBillingSnapshot(imageCount int, multiplier float64, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	if imageCount <= 0 {
		return BillingSnapshot{
			PriceUnit:  normalizePriceUnit(pricing.PriceUnit),
			Currency:   normalizeCurrency(pricing.Currency),
			GroupRatio: groupRatio,
		}, nil
	}
	quantity := float64(imageCount) * multiplier
	return buildSingleSidedBillingSnapshot(quantity, primaryUnitPrice(pricing), pricing, groupRatio)
}

func ComputeAudioSpeechQuota(charCount int, pricing model.ResolvedModelPricing, groupRatio float64) (int64, error) {
	snapshot, err := ComputeAudioSpeechBillingSnapshot(charCount, pricing, groupRatio)
	if err != nil {
		return 0, err
	}
	return snapshot.YYCAmount, nil
}

func ComputeAudioSpeechBillingSnapshot(charCount int, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	if charCount <= 0 {
		return BillingSnapshot{
			PriceUnit:  normalizePriceUnit(pricing.PriceUnit),
			Currency:   normalizeCurrency(pricing.Currency),
			GroupRatio: groupRatio,
		}, nil
	}
	return buildSingleSidedBillingSnapshot(float64(charCount), primaryUnitPrice(pricing), pricing, groupRatio)
}

func ComputeAudioTextQuota(tokenCount int, pricing model.ResolvedModelPricing, groupRatio float64) (int64, error) {
	snapshot, err := ComputeAudioTextBillingSnapshot(tokenCount, pricing, groupRatio)
	if err != nil {
		return 0, err
	}
	return snapshot.YYCAmount, nil
}

func ComputeAudioTextBillingSnapshot(tokenCount int, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	if tokenCount <= 0 {
		return BillingSnapshot{
			PriceUnit:  normalizePriceUnit(pricing.PriceUnit),
			Currency:   normalizeCurrency(pricing.Currency),
			GroupRatio: groupRatio,
		}, nil
	}
	return buildSingleSidedBillingSnapshot(float64(tokenCount), primaryUnitPrice(pricing), pricing, groupRatio)
}

func ComputeVideoQuota(quantity float64, pricing model.ResolvedModelPricing, groupRatio float64) (int64, error) {
	snapshot, err := ComputeVideoBillingSnapshot(quantity, pricing, groupRatio)
	if err != nil {
		return 0, err
	}
	return snapshot.YYCAmount, nil
}

func ComputeVideoBillingSnapshot(quantity float64, pricing model.ResolvedModelPricing, groupRatio float64) (BillingSnapshot, error) {
	if quantity <= 0 {
		return BillingSnapshot{
			PriceUnit:  normalizePriceUnit(pricing.PriceUnit),
			Currency:   normalizeCurrency(pricing.Currency),
			GroupRatio: groupRatio,
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
		"计费: source=%s provider=%s type=%s component=%s condition=%s unit=%s currency=%s input=%.6f output=%.6f group=%.2f",
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

func yycFromPrice(price float64, priceUnit string, currency string, quantity float64, groupRatio float64) (float64, error) {
	if price <= 0 || quantity <= 0 || groupRatio == 0 {
		return 0, nil
	}
	yycPerUnit, err := model.GetBillingCurrencyYYCPerUnit(currency)
	if err != nil {
		return 0, err
	}
	normalizedUnit := strings.TrimSpace(strings.ToLower(priceUnit))
	switch normalizedUnit {
	case "", model.ProviderPriceUnitPer1KTokens, model.ProviderPriceUnitPer1KChars:
		return quantity * price * yycPerUnit / 1000 * groupRatio, nil
	case model.ProviderPriceUnitPerImage,
		model.ProviderPriceUnitPerVideo,
		model.ProviderPriceUnitPerSecond,
		model.ProviderPriceUnitPerMinute,
		model.ProviderPriceUnitPerRequest,
		model.ProviderPriceUnitPerTask:
		return quantity * price * yycPerUnit * groupRatio, nil
	default:
		return quantity * price * yycPerUnit / 1000 * groupRatio, nil
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
		GroupRatio:     groupRatio,
		InputQuantity:  inputQuantity,
		OutputQuantity: outputQuantity,
	}
	snapshot.InputAmount = billingAmountFromPrice(inputPrice, snapshot.PriceUnit, inputQuantity)
	snapshot.OutputAmount = billingAmountFromPrice(outputPrice, snapshot.PriceUnit, outputQuantity)
	snapshot.Amount = snapshot.InputAmount + snapshot.OutputAmount
	if snapshot.Amount > 0 {
		yycRate, err := model.GetBillingCurrencyYYCPerUnit(snapshot.Currency)
		if err != nil {
			if groupRatio != 0 {
				return BillingSnapshot{}, err
			}
		} else {
			snapshot.YYCRate = yycRate
		}
	}
	rawYYC := snapshot.Amount * snapshot.YYCRate * groupRatio
	snapshot.YYCAmount = normalizeQuota(rawYYC, hasUsage, pricing, groupRatio)
	return snapshot, nil
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
