package model

import (
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/config"
	"gorm.io/gorm"
)

type PricingMatrixQuery struct {
	GroupID  string
	Provider string
	Model    string
	Endpoint string
}

type PricingMatrixItem struct {
	GroupID                    string  `json:"group_id"`
	Model                      string  `json:"model"`
	Endpoint                   string  `json:"endpoint"`
	ChannelID                  string  `json:"channel_id"`
	Provider                   string  `json:"provider"`
	UpstreamModel              string  `json:"upstream_model"`
	OfficialInputPrice         float64 `json:"official_input_price"`
	OfficialOutputPrice        float64 `json:"official_output_price"`
	ChannelInputPrice          float64 `json:"channel_input_price"`
	ChannelOutputPrice         float64 `json:"channel_output_price"`
	InputPrice                 float64 `json:"input_price"`
	OutputPrice                float64 `json:"output_price"`
	PriceUnit                  string  `json:"price_unit"`
	Currency                   string  `json:"currency"`
	GroupChannelRatio          float64 `json:"group_channel_ratio"`
	CurrentInputSell           float64 `json:"current_input_sell"`
	CurrentOutputSell          float64 `json:"current_output_sell"`
	PricingSource              string  `json:"pricing_source"`
	PricingState               string  `json:"pricing_state"`
	ProcurementCostState       string  `json:"procurement_cost_state"`
	ProcurementCostBasePerUnit float64 `json:"procurement_cost_base_per_unit"`
	ProcurementCostUnit        string  `json:"procurement_cost_unit"`
	ProcurementCostInPriceUnit float64 `json:"procurement_cost_in_price_unit"`
	ProcurementCostCurrency    string  `json:"procurement_cost_currency"`
	ProcurementCostFloorInput  float64 `json:"procurement_cost_floor_input"`
	ProcurementCostFloorOutput float64 `json:"procurement_cost_floor_output"`
	TargetMargin               float64 `json:"target_margin"`
	RiskBuffer                 float64 `json:"risk_buffer"`
	SelectedInputSell          float64 `json:"selected_input_sell"`
	SelectedOutputSell         float64 `json:"selected_output_sell"`
	FinalPricingState          string  `json:"final_pricing_state"`
	PricingDecisionReason      string  `json:"pricing_decision_reason"`
	CostFloorTriggered         bool    `json:"cost_floor_triggered"`
}

func ListPricingMatrixWithDB(db *gorm.DB, query PricingMatrixQuery) ([]PricingMatrixItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	where := []string{"gc.enabled = TRUE", "COALESCE(cm.publish_enabled, TRUE) = TRUE"}
	args := make([]any, 0, 4)
	if value := strings.TrimSpace(query.GroupID); value != "" {
		where = append(where, `gmc."group" = ?`)
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.Provider); value != "" {
		where = append(where, "LOWER(gmc.provider) = LOWER(?)")
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.Model); value != "" {
		where = append(where, "LOWER(gmc.model) = LOWER(?)")
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.Endpoint); value != "" {
		where = append(where, "LOWER(COALESCE(NULLIF(cm.endpoint, ''), gmc.model)) = LOWER(?)")
		args = append(args, value)
	}
	rows := make([]PricingMatrixItem, 0)
	selectSQL := `
		SELECT
			gmc."group" AS group_id,
			gmc.model AS model,
			COALESCE(NULLIF(cm.endpoint, ''), '') AS endpoint,
			gmc.channel_id AS channel_id,
			gmc.provider AS provider,
			COALESCE(NULLIF(gmc.upstream_model, ''), gmc.model) AS upstream_model,
			COALESCE(pm.input_price, 0) AS official_input_price,
			COALESCE(pm.output_price, 0) AS official_output_price,
			COALESCE(cm.input_price, 0) AS channel_input_price,
			COALESCE(cm.output_price, 0) AS channel_output_price,
			COALESCE(NULLIF(cm.input_price, 0), pm.input_price, 0) AS input_price,
			COALESCE(NULLIF(cm.output_price, 0), pm.output_price, 0) AS output_price,
			COALESCE(NULLIF(cm.price_unit, ''), pm.price_unit, '') AS price_unit,
			COALESCE(NULLIF(cm.currency, ''), pm.currency, '') AS currency,
			gc.billing_ratio AS group_channel_ratio
		FROM group_model_channels gmc
		JOIN group_channels gc ON gc."group" = gmc."group" AND gc.channel_id = gmc.channel_id
		LEFT JOIN channel_models cm ON cm.channel_id = gmc.channel_id AND cm.model = gmc.model
		LEFT JOIN provider_models pm ON pm.provider = gmc.provider AND pm.model = COALESCE(NULLIF(gmc.upstream_model, ''), gmc.model)
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY gmc."group" ASC, gmc.model ASC, gmc.channel_id ASC`
	if err := db.Raw(selectSQL, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	batches, err := ListProcurementBatchesWithDB(db, 5000)
	if err != nil {
		return nil, err
	}
	for index := range rows {
		row := &rows[index]
		row.CurrentInputSell = row.InputPrice * row.GroupChannelRatio
		row.CurrentOutputSell = row.OutputPrice * row.GroupChannelRatio
		switch {
		case row.InputPrice <= 0 && row.OutputPrice <= 0:
			row.PricingState = "price_missing"
		case row.ChannelInputPrice > 0 || row.ChannelOutputPrice > 0:
			row.PricingSource = "channel_override"
			row.PricingState = "configured"
		default:
			row.PricingSource = "provider_default"
			row.PricingState = "configured"
		}
		row.ProcurementCostState, row.ProcurementCostBasePerUnit, row.ProcurementCostUnit = resolveProcurementCost(batches, row.ChannelID, row.Model, row.PriceUnit)
		row.ProcurementCostCurrency = "CNY"
		if row.ProcurementCostState == "actual_available" && isCNY(row.Currency) {
			row.ProcurementCostInPriceUnit = costPerPriceUnit(row.ProcurementCostBasePerUnit, row.PriceUnit)
			row.TargetMargin = normalizeTargetMargin(config.BillingTargetMargin)
			row.RiskBuffer = normalizeRiskBuffer(config.BillingRiskBuffer)
			row.ProcurementCostFloorInput = costFloor(row.ProcurementCostInPriceUnit, row.TargetMargin, row.RiskBuffer)
			row.ProcurementCostFloorOutput = row.ProcurementCostFloorInput
			row.SelectedInputSell = maxFloat(row.CurrentInputSell, row.ProcurementCostFloorInput)
			row.SelectedOutputSell = maxFloat(row.CurrentOutputSell, row.ProcurementCostFloorOutput)
			if row.CurrentInputSell < row.ProcurementCostFloorInput || row.CurrentOutputSell < row.ProcurementCostFloorOutput {
				row.FinalPricingState = "below_cost_floor"
				row.PricingDecisionReason = "cost_floor"
				row.CostFloorTriggered = true
			} else {
				row.FinalPricingState = "healthy"
				row.PricingDecisionReason = "current_price_above_cost_floor"
			}
		} else if row.ProcurementCostState == "actual_available" && !isCNY(row.Currency) {
			row.ProcurementCostState = "currency_mismatch"
			row.FinalPricingState = "cost_currency_mismatch"
			row.PricingDecisionReason = "currency_mismatch"
		} else {
			row.FinalPricingState = "cost_unavailable"
			row.PricingDecisionReason = "cost_unavailable"
		}
	}
	return rows, nil
}

func resolveProcurementCost(batches []ChannelProcurementBatch, channelID string, modelName string, priceUnit string) (string, float64, string) {
	wantedUnit := normalizePricingCapacityUnit(priceUnit)
	found := false
	estimated := false
	pending := false
	for _, batch := range batches {
		if strings.TrimSpace(batch.ChannelId) != strings.TrimSpace(channelID) || batch.CapacityRemaining <= 0 {
			continue
		}
		scopeValue := strings.TrimSpace(batch.ScopeValue)
		if batch.ScopeType != "global" && scopeValue != strings.TrimSpace(modelName) {
			continue
		}
		if strings.TrimSpace(strings.ToLower(batch.CapacityUnit)) != wantedUnit {
			continue
		}
		found = true
		switch batch.CostSource {
		case ProcurementCostSourceActual, ProcurementCostSourceZeroCost:
			if batch.CostStatus == ProcurementCostStatusActive {
				return "actual_available", batch.CostPerUnitAmount, wantedUnit
			}
		case ProcurementCostSourceEstimated:
			estimated = true
		case "pending":
			pending = true
		}
	}
	if found && estimated {
		return "estimated_only", 0, wantedUnit
	}
	if found && pending {
		return "pending", 0, wantedUnit
	}
	if found {
		return "none", 0, wantedUnit
	}
	return "unit_mismatch", 0, wantedUnit
}

func normalizePricingCapacityUnit(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "per_1k_tokens", "tokens", "token":
		return "token"
	case "per_1k_chars", "chars", "char":
		return "char"
	default:
		return strings.TrimSpace(strings.ToLower(value))
	}
}

func costPerPriceUnit(costPerBaseUnit float64, priceUnit string) float64 {
	switch strings.TrimSpace(strings.ToLower(priceUnit)) {
	case "per_1k_tokens", "per_1k_chars":
		return costPerBaseUnit * 1000
	default:
		return costPerBaseUnit
	}
}

func isCNY(currency string) bool {
	currency = strings.TrimSpace(strings.ToUpper(currency))
	return currency == "" || currency == "CNY" || currency == "RMB" || currency == "¥"
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func normalizeTargetMargin(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value >= 0.95 {
		return 0.95
	}
	return value
}

func normalizeRiskBuffer(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func costFloor(cost, targetMargin, riskBuffer float64) float64 {
	if cost <= 0 || targetMargin >= 1 {
		return 0
	}
	return cost * (1 + riskBuffer) / (1 - targetMargin)
}
