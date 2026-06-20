package billing

import (
	"math"
	"strings"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/internal/admin/model"
)

const (
	PricingDecisionReasonOfficialAnchor = "official_anchor"
	PricingDecisionReasonCostFloor      = "cost_floor"
)

type PricingPolicy struct {
	OfficialMarkup float64 `json:"official_markup"`
	TargetMargin   float64 `json:"target_margin"`
	RiskBuffer     float64 `json:"risk_buffer"`
}

type MoneyAmount struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type PricingDecisionInput struct {
	OfficialAnchor  MoneyAmount   `json:"official_anchor"`
	CurrentCharge   MoneyAmount   `json:"current_charge"`
	ProcurementCost MoneyAmount   `json:"procurement_cost"`
	Policy          PricingPolicy `json:"policy"`
}

type PricingDecision struct {
	OfficialAnchor MoneyAmount   `json:"official_anchor"`
	OfficialSell   MoneyAmount   `json:"official_sell"`
	CostFloor      MoneyAmount   `json:"cost_floor"`
	SelectedSell   MoneyAmount   `json:"selected_sell"`
	SelectedCharge MoneyAmount   `json:"selected_charge"`
	Reason         string        `json:"reason"`
	Policy         PricingPolicy `json:"policy"`
}

func CurrentPricingPolicy() PricingPolicy {
	return PricingPolicy{
		OfficialMarkup: config.BillingOfficialMarkup,
		TargetMargin:   config.BillingTargetMargin,
		RiskBuffer:     config.BillingRiskBuffer,
	}
}

func NormalizePricingPolicy(policy PricingPolicy) PricingPolicy {
	if policy.OfficialMarkup <= 0 {
		policy.OfficialMarkup = 1
	}
	if policy.TargetMargin < 0 {
		policy.TargetMargin = 0
	}
	if policy.TargetMargin >= 0.95 {
		policy.TargetMargin = 0.95
	}
	if policy.RiskBuffer < 0 {
		policy.RiskBuffer = 0
	}
	return policy
}

func DecidePricing(input PricingDecisionInput) PricingDecision {
	policy := NormalizePricingPolicy(input.Policy)
	anchorAmount := convertBillingAmountToBaseAmount(input.OfficialAnchor.Amount, strings.TrimSpace(input.OfficialAnchor.Currency))
	anchorSellAmount := anchorAmount * policy.OfficialMarkup
	costFloorAmount := 0.0
	if strings.EqualFold(strings.TrimSpace(input.ProcurementCost.Currency), model.BillingCurrencyCodeCNY) && input.ProcurementCost.Amount > 0 {
		costFloorAmount = input.ProcurementCost.Amount * (1 + policy.RiskBuffer) / (1 - policy.TargetMargin)
	}
	selectedSellAmount := anchorSellAmount
	selectedCharge := input.CurrentCharge
	reason := PricingDecisionReasonOfficialAnchor
	if costFloorAmount > selectedSellAmount {
		selectedSellAmount = costFloorAmount
		selectedCharge = MoneyAmount{Amount: float64(chargeAmountFromMoneyCeil(selectedSellAmount, model.BillingCurrencyCodeCNY)), Currency: model.BillingCurrencyCodeYYC}
		reason = PricingDecisionReasonCostFloor
	}
	return PricingDecision{
		OfficialAnchor: MoneyAmount{Amount: anchorAmount, Currency: model.BillingCurrencyCodeCNY},
		OfficialSell:   MoneyAmount{Amount: anchorSellAmount, Currency: model.BillingCurrencyCodeCNY},
		CostFloor:      MoneyAmount{Amount: costFloorAmount, Currency: model.BillingCurrencyCodeCNY},
		SelectedSell:   MoneyAmount{Amount: selectedSellAmount, Currency: model.BillingCurrencyCodeCNY},
		SelectedCharge: selectedCharge,
		Reason:         reason,
		Policy:         policy,
	}
}

func chargeAmountFromMoneyCeil(amount float64, currency string) int64 {
	if amount <= 0 {
		return 0
	}
	rate, err := model.GetBillingCurrencyChargeRate(currency)
	if err != nil || rate <= 0 {
		return 0
	}
	return int64(math.Ceil(amount * rate))
}
