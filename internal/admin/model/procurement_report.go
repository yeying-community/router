package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const (
	ProcurementReportGroupByChannel = "channel"
	ProcurementReportGroupByModel   = "model"
)

const (
	ProcurementReportCostScopeAll          = "all"
	ProcurementReportCostScopeUnconfigured = "unconfigured"
)

type ProcurementReportQuery struct {
	StartAt   int64
	EndAt     int64
	GroupBy   string
	CostScope string
	GroupID   string
}

type ProcurementReportItem struct {
	DimensionType                string  `json:"dimension_type" gorm:"-"`
	DimensionKey                 string  `json:"dimension_key" gorm:"column:dimension_key"`
	RequestCount                 int64   `json:"request_count" gorm:"column:request_count"`
	ConfiguredCostRequestCount   int64   `json:"configured_cost_request_count" gorm:"column:configured_cost_request_count"`
	UnconfiguredCostRequestCount int64   `json:"unconfigured_cost_request_count" gorm:"column:unconfigured_cost_request_count"`
	SellBaseAmount               float64 `json:"sell_base_amount" gorm:"column:sell_base_amount"`
	ConfiguredSellBaseAmount     float64 `json:"configured_sell_base_amount" gorm:"column:configured_sell_base_amount"`
	UnconfiguredSellBaseAmount   float64 `json:"unconfigured_sell_base_amount" gorm:"column:unconfigured_sell_base_amount"`
	ProcurementCostBaseAmount    float64 `json:"procurement_cost_base_amount" gorm:"column:procurement_cost_base_amount"`
	GrossProfitBaseAmount        float64 `json:"gross_profit_base_amount" gorm:"column:gross_profit_base_amount"`
	GrossMargin                  float64 `json:"gross_margin" gorm:"-"`
	ActualCostBaseAmount         float64 `json:"actual_cost_base_amount" gorm:"column:actual_cost_base_amount"`
	EstimatedCostBaseAmount      float64 `json:"estimated_cost_base_amount" gorm:"column:estimated_cost_base_amount"`
	ZeroCostRequestCount         int64   `json:"zero_cost_request_count" gorm:"column:zero_cost_request_count"`
	FirstRequestAt               int64   `json:"first_request_at" gorm:"column:first_request_at"`
	LastRequestAt                int64   `json:"last_request_at" gorm:"column:last_request_at"`
}

type ProcurementReportSummary struct {
	GroupBy                      string                  `json:"group_by"`
	CostScope                    string                  `json:"cost_scope"`
	GroupID                      string                  `json:"group_id"`
	StartAt                      int64                   `json:"start_at"`
	EndAt                        int64                   `json:"end_at"`
	Items                        []ProcurementReportItem `json:"items"`
	RequestCount                 int64                   `json:"request_count"`
	ConfiguredCostRequestCount   int64                   `json:"configured_cost_request_count"`
	UnconfiguredCostRequestCount int64                   `json:"unconfigured_cost_request_count"`
	SellBaseAmount               float64                 `json:"sell_base_amount"`
	ConfiguredSellBaseAmount     float64                 `json:"configured_sell_base_amount"`
	UnconfiguredSellBaseAmount   float64                 `json:"unconfigured_sell_base_amount"`
	ProcurementCostBaseAmount    float64                 `json:"procurement_cost_base_amount"`
	GrossProfitBaseAmount        float64                 `json:"gross_profit_base_amount"`
	GrossMargin                  float64                 `json:"gross_margin"`
}

func NormalizeProcurementReportCostScope(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case ProcurementReportCostScopeUnconfigured:
		return ProcurementReportCostScopeUnconfigured
	default:
		return ProcurementReportCostScopeAll
	}
}

func NormalizeProcurementReportGroupBy(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case ProcurementReportGroupByModel:
		return ProcurementReportGroupByModel
	case ProcurementReportGroupByChannel, "":
		return ProcurementReportGroupByChannel
	default:
		return ProcurementReportGroupByChannel
	}
}

func procurementReportDimensionExpression(groupBy string) string {
	switch NormalizeProcurementReportGroupBy(groupBy) {
	case ProcurementReportGroupByModel:
		return "COALESCE(NULLIF(TRIM(model_name), ''), '-')"
	default:
		return "COALESCE(NULLIF(TRIM(channel_id), ''), '-')"
	}
}

func procurementReportUnconfiguredCostCondition() string {
	return "billing_procurement_cost_source NOT IN ? OR COALESCE(NULLIF(TRIM(billing_procurement_cost_source), ''), '') = ''"
}

func ListProcurementReportWithDB(db *gorm.DB, query ProcurementReportQuery) (ProcurementReportSummary, error) {
	if db == nil {
		return ProcurementReportSummary{}, fmt.Errorf("database handle is nil")
	}
	groupBy := NormalizeProcurementReportGroupBy(query.GroupBy)
	costScope := NormalizeProcurementReportCostScope(query.CostScope)
	summary := ProcurementReportSummary{
		GroupBy:   groupBy,
		CostScope: costScope,
		GroupID:   strings.TrimSpace(query.GroupID),
		StartAt:   query.StartAt,
		EndAt:     query.EndAt,
		Items:     []ProcurementReportItem{},
	}
	if query.StartAt <= 0 || query.EndAt <= 0 || query.EndAt < query.StartAt {
		return summary, nil
	}

	dimensionExpr := procurementReportDimensionExpression(groupBy)
	rows := make([]ProcurementReportItem, 0)
	configuredSources := []string{ProcurementCostSourceActual, ProcurementCostSourceEstimated, ProcurementCostSourceZeroCost}
	queryDB := db.Table(EventLogsTableName).
		Select(`
			`+dimensionExpr+` AS dimension_key,
			COUNT(1) AS request_count,
			COALESCE(SUM(CASE WHEN billing_procurement_cost_source IN ? THEN 1 ELSE 0 END), 0) AS configured_cost_request_count,
			COALESCE(SUM(CASE WHEN billing_procurement_cost_source NOT IN ? OR COALESCE(NULLIF(TRIM(billing_procurement_cost_source), ''), '') = '' THEN 1 ELSE 0 END), 0) AS unconfigured_cost_request_count,
			COALESCE(SUM(billing_sell_base_amount), 0) AS sell_base_amount,
			COALESCE(SUM(CASE WHEN billing_procurement_cost_source IN ? THEN billing_sell_base_amount ELSE 0 END), 0) AS configured_sell_base_amount,
			COALESCE(SUM(CASE WHEN billing_procurement_cost_source NOT IN ? OR COALESCE(NULLIF(TRIM(billing_procurement_cost_source), ''), '') = '' THEN billing_sell_base_amount ELSE 0 END), 0) AS unconfigured_sell_base_amount,
			COALESCE(SUM(CASE WHEN billing_procurement_cost_source IN ? THEN billing_procurement_cost_base_amount ELSE 0 END), 0) AS procurement_cost_base_amount,
			COALESCE(SUM(CASE WHEN billing_procurement_cost_source IN ? THEN billing_gross_profit_base_amount ELSE 0 END), 0) AS gross_profit_base_amount,
			COALESCE(SUM(CASE WHEN billing_procurement_cost_source = ? THEN billing_procurement_cost_base_amount ELSE 0 END), 0) AS actual_cost_base_amount,
			COALESCE(SUM(CASE WHEN billing_procurement_cost_source = ? THEN billing_procurement_cost_base_amount ELSE 0 END), 0) AS estimated_cost_base_amount,
			COALESCE(SUM(CASE WHEN billing_procurement_cost_source = ? THEN 1 ELSE 0 END), 0) AS zero_cost_request_count,
			COALESCE(MIN(created_at), 0) AS first_request_at,
			COALESCE(MAX(created_at), 0) AS last_request_at
		`, configuredSources, configuredSources, configuredSources, configuredSources, configuredSources, configuredSources, ProcurementCostSourceActual, ProcurementCostSourceEstimated, ProcurementCostSourceZeroCost).
		Where("type = ? AND created_at BETWEEN ? AND ?", LogTypeConsume, query.StartAt, query.EndAt)
	if summary.GroupID != "" {
		queryDB = queryDB.Where("group_id = ?", summary.GroupID)
	}
	if costScope == ProcurementReportCostScopeUnconfigured {
		queryDB = queryDB.Where(procurementReportUnconfiguredCostCondition(), configuredSources)
	}
	if err := queryDB.
		Group("dimension_key").
		Order("procurement_cost_base_amount DESC, sell_base_amount DESC").
		Scan(&rows).Error; err != nil {
		return summary, err
	}
	for index := range rows {
		rows[index].DimensionType = groupBy
		if rows[index].ConfiguredSellBaseAmount > 0 {
			rows[index].GrossMargin = rows[index].GrossProfitBaseAmount / rows[index].ConfiguredSellBaseAmount
		}
		summary.RequestCount += rows[index].RequestCount
		summary.ConfiguredCostRequestCount += rows[index].ConfiguredCostRequestCount
		summary.UnconfiguredCostRequestCount += rows[index].UnconfiguredCostRequestCount
		summary.SellBaseAmount += rows[index].SellBaseAmount
		summary.ConfiguredSellBaseAmount += rows[index].ConfiguredSellBaseAmount
		summary.UnconfiguredSellBaseAmount += rows[index].UnconfiguredSellBaseAmount
		summary.ProcurementCostBaseAmount += rows[index].ProcurementCostBaseAmount
		summary.GrossProfitBaseAmount += rows[index].GrossProfitBaseAmount
	}
	if summary.ConfiguredSellBaseAmount > 0 {
		summary.GrossMargin = summary.GrossProfitBaseAmount / summary.ConfiguredSellBaseAmount
	}
	summary.Items = rows
	return summary, nil
}
