package model

import (
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/config"
	"gorm.io/gorm"
)

const (
	ProcurementReportGroupByChannel  = "channel"
	ProcurementReportGroupByModel    = "model"
	ProcurementReportGroupByEndpoint = "endpoint"
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
	ChannelID string
	Provider  string
	Model     string
}

type ProcurementReportItem struct {
	DimensionType                string  `json:"dimension_type" gorm:"-"`
	DimensionKey                 string  `json:"dimension_key" gorm:"column:dimension_key"`
	RequestCount                 int64   `json:"request_count" gorm:"column:request_count"`
	ConfiguredCostRequestCount   int64   `json:"configured_cost_request_count" gorm:"column:configured_cost_request_count"`
	UnconfiguredCostRequestCount int64   `json:"unconfigured_cost_request_count" gorm:"column:unconfigured_cost_request_count"`
	EstimatedCostRequestCount    int64   `json:"estimated_cost_request_count" gorm:"column:estimated_cost_request_count"`
	PendingCostRequestCount      int64   `json:"pending_cost_request_count" gorm:"column:pending_cost_request_count"`
	RetryCostRequestCount        int64   `json:"retry_cost_request_count" gorm:"column:retry_cost_request_count"`
	InputQuantity                float64 `json:"input_quantity" gorm:"column:input_quantity"`
	OutputQuantity               float64 `json:"output_quantity" gorm:"column:output_quantity"`
	CacheReadQuantity            float64 `json:"cache_read_quantity" gorm:"column:cache_read_quantity"`
	CacheWriteQuantity           float64 `json:"cache_write_quantity" gorm:"column:cache_write_quantity"`
	RouterConsumedYYC            int64   `json:"router_consumed_yyc" gorm:"column:router_consumed_yyc"`
	SellBaseAmount               float64 `json:"sell_base_amount" gorm:"column:sell_base_amount"`
	ConfiguredSellBaseAmount     float64 `json:"configured_sell_base_amount" gorm:"column:configured_sell_base_amount"`
	UnconfiguredSellBaseAmount   float64 `json:"unconfigured_sell_base_amount" gorm:"column:unconfigured_sell_base_amount"`
	ProcurementCostBaseAmount    float64 `json:"procurement_cost_base_amount" gorm:"column:procurement_cost_base_amount"`
	GrossProfitBaseAmount        float64 `json:"gross_profit_base_amount" gorm:"column:gross_profit_base_amount"`
	GrossMargin                  float64 `json:"gross_margin" gorm:"-"`
	ProcurementCostYYC           float64 `json:"procurement_cost_yyc" gorm:"-"`
	GrossProfitYYC               float64 `json:"gross_profit_yyc" gorm:"-"`
	ActualCostBaseAmount         float64 `json:"actual_cost_base_amount" gorm:"column:actual_cost_base_amount"`
	EstimatedCostBaseAmount      float64 `json:"estimated_cost_base_amount" gorm:"column:estimated_cost_base_amount"`
	CostFloorTriggeredCount      int64   `json:"cost_floor_triggered_count" gorm:"column:cost_floor_triggered_count"`
	CostFloorTriggeredAmount     float64 `json:"cost_floor_triggered_amount" gorm:"column:cost_floor_triggered_amount"`
	ZeroCostRequestCount         int64   `json:"zero_cost_request_count" gorm:"column:zero_cost_request_count"`
	FirstRequestAt               int64   `json:"first_request_at" gorm:"column:first_request_at"`
	LastRequestAt                int64   `json:"last_request_at" gorm:"column:last_request_at"`
}

type ProcurementReportSummary struct {
	GroupBy                      string                  `json:"group_by"`
	CostScope                    string                  `json:"cost_scope"`
	GroupID                      string                  `json:"group_id"`
	Provider                     string                  `json:"provider"`
	StartAt                      int64                   `json:"start_at"`
	EndAt                        int64                   `json:"end_at"`
	Items                        []ProcurementReportItem `json:"items"`
	RequestCount                 int64                   `json:"request_count"`
	ConfiguredCostRequestCount   int64                   `json:"configured_cost_request_count"`
	UnconfiguredCostRequestCount int64                   `json:"unconfigured_cost_request_count"`
	EstimatedCostRequestCount    int64                   `json:"estimated_cost_request_count"`
	PendingCostRequestCount      int64                   `json:"pending_cost_request_count"`
	RetryCostRequestCount        int64                   `json:"retry_cost_request_count"`
	InputQuantity                float64                 `json:"input_quantity"`
	OutputQuantity               float64                 `json:"output_quantity"`
	CacheReadQuantity            float64                 `json:"cache_read_quantity"`
	CacheWriteQuantity           float64                 `json:"cache_write_quantity"`
	RouterConsumedYYC            int64                   `json:"router_consumed_yyc"`
	SellBaseAmount               float64                 `json:"sell_base_amount"`
	ConfiguredSellBaseAmount     float64                 `json:"configured_sell_base_amount"`
	UnconfiguredSellBaseAmount   float64                 `json:"unconfigured_sell_base_amount"`
	ProcurementCostBaseAmount    float64                 `json:"procurement_cost_base_amount"`
	GrossProfitBaseAmount        float64                 `json:"gross_profit_base_amount"`
	GrossMargin                  float64                 `json:"gross_margin"`
	ProcurementCostYYC           float64                 `json:"procurement_cost_yyc"`
	GrossProfitYYC               float64                 `json:"gross_profit_yyc"`
	TargetMargin                 float64                 `json:"target_margin"`
	RiskBuffer                   float64                 `json:"risk_buffer"`
	CostFloorTriggeredCount      int64                   `json:"cost_floor_triggered_count"`
	CostFloorTriggeredAmount     float64                 `json:"cost_floor_triggered_amount"`
}

type ProcurementTrendItem struct {
	Day                          string  `json:"day" gorm:"column:day"`
	RequestCount                 int64   `json:"request_count" gorm:"column:request_count"`
	ConfiguredCostRequestCount   int64   `json:"configured_cost_request_count" gorm:"column:configured_cost_request_count"`
	UnconfiguredCostRequestCount int64   `json:"unconfigured_cost_request_count" gorm:"column:unconfigured_cost_request_count"`
	InputQuantity                float64 `json:"input_quantity" gorm:"column:input_quantity"`
	OutputQuantity               float64 `json:"output_quantity" gorm:"column:output_quantity"`
	CacheReadQuantity            float64 `json:"cache_read_quantity" gorm:"column:cache_read_quantity"`
	CacheWriteQuantity           float64 `json:"cache_write_quantity" gorm:"column:cache_write_quantity"`
	RouterConsumedYYC            int64   `json:"router_consumed_yyc" gorm:"column:router_consumed_yyc"`
	SellBaseAmount               float64 `json:"sell_base_amount" gorm:"column:sell_base_amount"`
	ProcurementCostBaseAmount    float64 `json:"procurement_cost_base_amount" gorm:"column:procurement_cost_base_amount"`
	GrossProfitBaseAmount        float64 `json:"gross_profit_base_amount" gorm:"column:gross_profit_base_amount"`
	ProcurementCostYYC           float64 `json:"procurement_cost_yyc" gorm:"-"`
	GrossProfitYYC               float64 `json:"gross_profit_yyc" gorm:"-"`
	CostFloorTriggeredCount      int64   `json:"cost_floor_triggered_count" gorm:"column:cost_floor_triggered_count"`
	CostFloorTriggeredAmount     float64 `json:"cost_floor_triggered_amount" gorm:"column:cost_floor_triggered_amount"`
}

type ProcurementTrendQuery struct {
	StartAt   int64
	EndAt     int64
	GroupID   string
	ChannelID string
	Provider  string
	Model     string
}

func ListProcurementTrendWithDB(db *gorm.DB, query ProcurementTrendQuery) ([]ProcurementTrendItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	rows := make([]ProcurementTrendItem, 0)
	configuredStatuses := []string{ProcurementCostAttributionStatusActual, ProcurementCostAttributionStatusNone}
	unconfiguredCondition := procurementReportUnconfiguredCostCondition()
	dbQuery := db.Table(EventLogsTableName+" el").Joins("LEFT JOIN "+BillingSettlementsTableName+" bs ON bs.request_log_id = el.id").Joins("LEFT JOIN "+ProcurementAttributionsTableName+" pa ON pa.request_log_id = el.id").Select(`
		TO_CHAR(TO_TIMESTAMP(el.created_at), 'YYYY-MM-DD') AS day,
		COUNT(1) AS request_count,
		COALESCE(SUM(CASE WHEN pa.status IN ? THEN 1 ELSE 0 END), 0) AS configured_cost_request_count,
		COALESCE(SUM(CASE WHEN `+unconfiguredCondition+` THEN 1 ELSE 0 END), 0) AS unconfigured_cost_request_count,
		COALESCE(SUM(bs.input_quantity), 0) AS input_quantity,
		COALESCE(SUM(bs.output_quantity), 0) AS output_quantity,
		COALESCE(SUM(bs.cache_read_quantity), 0) AS cache_read_quantity,
		COALESCE(SUM(bs.cache_write_quantity), 0) AS cache_write_quantity,
		COALESCE(SUM(bs.charge_amount), 0) AS router_consumed_yyc,
		COALESCE(SUM(bs.sell_base_amount), 0) AS sell_base_amount,
		COALESCE(SUM(CASE WHEN pa.status IN ? THEN pa.cost_base_amount ELSE 0 END), 0) AS procurement_cost_base_amount,
		COALESCE(SUM(CASE WHEN pa.status IN ? THEN pa.gross_profit_base_amount ELSE 0 END), 0) AS gross_profit_base_amount,
		COALESCE(SUM(CASE WHEN bs.cost_floor_triggered = TRUE THEN 1 ELSE 0 END), 0) AS cost_floor_triggered_count,
		COALESCE(SUM(CASE WHEN bs.cost_floor_triggered = TRUE THEN bs.cost_floor_base_amount ELSE 0 END), 0) AS cost_floor_triggered_amount
	`, configuredStatuses, configuredStatuses, configuredStatuses).Where("el.type = ? AND el.created_at BETWEEN ? AND ?", LogTypeConsume, query.StartAt, query.EndAt)
	if strings.TrimSpace(query.GroupID) != "" {
		dbQuery = dbQuery.Where("el.group_id = ?", strings.TrimSpace(query.GroupID))
	}
	if strings.TrimSpace(query.ChannelID) != "" {
		dbQuery = dbQuery.Where("el.channel_id = ?", strings.TrimSpace(query.ChannelID))
	}
	if provider := NormalizeGroupModelProviderValue(query.Provider); provider != "" {
		dbQuery = dbQuery.Where("LOWER(TRIM(COALESCE(el.provider, ''))) = ?", strings.ToLower(provider))
	}
	if strings.TrimSpace(query.Model) != "" {
		dbQuery = dbQuery.Where("COALESCE(NULLIF(TRIM(el.actual_model_name), ''), NULLIF(TRIM(el.model_name), '')) = ?", strings.TrimSpace(query.Model))
	}
	err := dbQuery.Group("day").Order("day ASC").Scan(&rows).Error
	if err != nil {
		return rows, err
	}
	// Express CNY base cost/profit in YYC too (see §2 of the accounting standard).
	cnyChargeRate := procurementReportCNYChargeRate()
	for index := range rows {
		rows[index].ProcurementCostYYC = rows[index].ProcurementCostBaseAmount * cnyChargeRate
		rows[index].GrossProfitYYC = rows[index].GrossProfitBaseAmount * cnyChargeRate
	}
	return rows, nil
}

// procurementReportCNYChargeRate returns YYC-per-1-CNY so CNY base amounts can be
// expressed in the YYC accounting unit. Returns 0 when the rate is unavailable, in
// which case YYC-denominated fields stay 0 and callers fall back to the CNY columns.
func procurementReportCNYChargeRate() float64 {
	rate, err := GetBillingCurrencyChargeRate(BillingCurrencyCodeCNY)
	if err != nil || rate <= 0 {
		return 0
	}
	return rate
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
	case ProcurementReportGroupByEndpoint:
		return ProcurementReportGroupByEndpoint
	case ProcurementReportGroupByChannel, "":
		return ProcurementReportGroupByChannel
	default:
		return ProcurementReportGroupByChannel
	}
}

func procurementReportDimensionExpression(groupBy string) string {
	switch NormalizeProcurementReportGroupBy(groupBy) {
	case ProcurementReportGroupByModel:
		return "COALESCE(NULLIF(TRIM(el.actual_model_name), ''), NULLIF(TRIM(el.model_name), ''), '-')"
	case ProcurementReportGroupByEndpoint:
		return "COALESCE(NULLIF(TRIM(el.upstream_endpoint), ''), '-')"
	default:
		return "COALESCE(NULLIF(TRIM(el.channel_id), ''), '-')"
	}
}

func procurementReportUnconfiguredCostCondition() string {
	return "(pa.request_log_id IS NULL OR LOWER(TRIM(COALESCE(pa.status, ''))) = 'unconfigured')"
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
		Provider:  NormalizeGroupModelProviderValue(query.Provider),
		StartAt:   query.StartAt,
		EndAt:     query.EndAt,
		Items:     []ProcurementReportItem{},
	}
	if query.StartAt <= 0 || query.EndAt <= 0 || query.EndAt < query.StartAt {
		return summary, nil
	}

	dimensionExpr := procurementReportDimensionExpression(groupBy)
	rows := make([]ProcurementReportItem, 0)
	configuredStatuses := []string{ProcurementCostAttributionStatusActual, ProcurementCostAttributionStatusNone}
	unconfiguredCondition := procurementReportUnconfiguredCostCondition()
	queryDB := db.Table(EventLogsTableName+" el").Joins("LEFT JOIN "+BillingSettlementsTableName+" bs ON bs.request_log_id = el.id").Joins("LEFT JOIN "+ProcurementAttributionsTableName+" pa ON pa.request_log_id = el.id").
		Select(`
			`+dimensionExpr+` AS dimension_key,
			COUNT(1) AS request_count,
			COALESCE(SUM(CASE WHEN pa.status IN ? THEN 1 ELSE 0 END), 0) AS configured_cost_request_count,
			COALESCE(SUM(CASE WHEN `+unconfiguredCondition+` THEN 1 ELSE 0 END), 0) AS unconfigured_cost_request_count,
			COALESCE(SUM(CASE WHEN pa.status = ? THEN 1 ELSE 0 END), 0) AS estimated_cost_request_count,
			COALESCE(SUM(CASE WHEN pa.status = ? THEN 1 ELSE 0 END), 0) AS pending_cost_request_count,
			COALESCE(SUM(CASE WHEN pa.status = ? THEN 1 ELSE 0 END), 0) AS retry_cost_request_count,
			COALESCE(SUM(bs.input_quantity), 0) AS input_quantity,
			COALESCE(SUM(bs.output_quantity), 0) AS output_quantity,
			COALESCE(SUM(bs.cache_read_quantity), 0) AS cache_read_quantity,
			COALESCE(SUM(bs.cache_write_quantity), 0) AS cache_write_quantity,
			COALESCE(SUM(bs.charge_amount), 0) AS router_consumed_yyc,
			COALESCE(SUM(bs.sell_base_amount), 0) AS sell_base_amount,
			COALESCE(SUM(CASE WHEN pa.status IN ? THEN bs.sell_base_amount ELSE 0 END), 0) AS configured_sell_base_amount,
			COALESCE(SUM(CASE WHEN `+unconfiguredCondition+` THEN bs.sell_base_amount ELSE 0 END), 0) AS unconfigured_sell_base_amount,
			COALESCE(SUM(CASE WHEN pa.status IN ? THEN pa.cost_base_amount ELSE 0 END), 0) AS procurement_cost_base_amount,
			COALESCE(SUM(CASE WHEN pa.status IN ? THEN pa.gross_profit_base_amount ELSE 0 END), 0) AS gross_profit_base_amount,
			COALESCE(SUM(CASE WHEN pa.status = ? THEN pa.cost_base_amount ELSE 0 END), 0) AS actual_cost_base_amount,
			COALESCE(SUM(CASE WHEN pa.status = ? THEN pa.cost_base_amount ELSE 0 END), 0) AS estimated_cost_base_amount,
			COALESCE(SUM(CASE WHEN bs.cost_floor_triggered = TRUE THEN 1 ELSE 0 END), 0) AS cost_floor_triggered_count,
			COALESCE(SUM(CASE WHEN bs.cost_floor_triggered = TRUE THEN bs.cost_floor_base_amount ELSE 0 END), 0) AS cost_floor_triggered_amount,
			COALESCE(SUM(CASE WHEN pa.status = ? THEN 1 ELSE 0 END), 0) AS zero_cost_request_count,
			COALESCE(MIN(el.created_at), 0) AS first_request_at,
			COALESCE(MAX(el.created_at), 0) AS last_request_at
	`, configuredStatuses, ProcurementCostAttributionStatusEstimated, ProcurementCostAttributionStatusPending, ProcurementCostAttributionStatusRetry, configuredStatuses, configuredStatuses, configuredStatuses, ProcurementCostAttributionStatusActual, ProcurementCostAttributionStatusEstimated, ProcurementCostAttributionStatusNone).
		Where("el.type = ? AND el.created_at BETWEEN ? AND ?", LogTypeConsume, query.StartAt, query.EndAt)
	if summary.GroupID != "" {
		queryDB = queryDB.Where("el.group_id = ?", summary.GroupID)
	}
	if strings.TrimSpace(query.ChannelID) != "" {
		queryDB = queryDB.Where("el.channel_id = ?", strings.TrimSpace(query.ChannelID))
	}
	if summary.Provider != "" {
		queryDB = queryDB.Where("LOWER(TRIM(COALESCE(el.provider, ''))) = ?", strings.ToLower(summary.Provider))
	}
	if strings.TrimSpace(query.Model) != "" {
		queryDB = queryDB.Where("COALESCE(NULLIF(TRIM(el.actual_model_name), ''), NULLIF(TRIM(el.model_name), '')) = ?", strings.TrimSpace(query.Model))
	}
	if costScope == ProcurementReportCostScopeUnconfigured {
		queryDB = queryDB.Where(procurementReportUnconfiguredCostCondition())
	}
	if err := queryDB.
		Group("dimension_key").
		Order("procurement_cost_base_amount DESC, sell_base_amount DESC").
		Scan(&rows).Error; err != nil {
		return summary, err
	}
	// YYC is the accounting unit (see docs/商业计费/成本与盈利核算标准.md §2). Cost/
	// profit are stored as CNY base amounts; express them in YYC too so cost/profit
	// share the same unit as revenue (router_consumed_yyc). Rate = YYC per 1 CNY.
	cnyChargeRate := procurementReportCNYChargeRate()
	for index := range rows {
		rows[index].DimensionType = groupBy
		if rows[index].ConfiguredSellBaseAmount > 0 {
			rows[index].GrossMargin = rows[index].GrossProfitBaseAmount / rows[index].ConfiguredSellBaseAmount
		}
		rows[index].ProcurementCostYYC = rows[index].ProcurementCostBaseAmount * cnyChargeRate
		rows[index].GrossProfitYYC = rows[index].GrossProfitBaseAmount * cnyChargeRate
		summary.RequestCount += rows[index].RequestCount
		summary.ConfiguredCostRequestCount += rows[index].ConfiguredCostRequestCount
		summary.UnconfiguredCostRequestCount += rows[index].UnconfiguredCostRequestCount
		summary.EstimatedCostRequestCount += rows[index].EstimatedCostRequestCount
		summary.PendingCostRequestCount += rows[index].PendingCostRequestCount
		summary.RetryCostRequestCount += rows[index].RetryCostRequestCount
		summary.InputQuantity += rows[index].InputQuantity
		summary.OutputQuantity += rows[index].OutputQuantity
		summary.CacheReadQuantity += rows[index].CacheReadQuantity
		summary.CacheWriteQuantity += rows[index].CacheWriteQuantity
		summary.RouterConsumedYYC += rows[index].RouterConsumedYYC
		summary.SellBaseAmount += rows[index].SellBaseAmount
		summary.ConfiguredSellBaseAmount += rows[index].ConfiguredSellBaseAmount
		summary.UnconfiguredSellBaseAmount += rows[index].UnconfiguredSellBaseAmount
		summary.ProcurementCostBaseAmount += rows[index].ProcurementCostBaseAmount
		summary.GrossProfitBaseAmount += rows[index].GrossProfitBaseAmount
		summary.CostFloorTriggeredCount += rows[index].CostFloorTriggeredCount
		summary.CostFloorTriggeredAmount += rows[index].CostFloorTriggeredAmount
	}
	if summary.ConfiguredSellBaseAmount > 0 {
		summary.GrossMargin = summary.GrossProfitBaseAmount / summary.ConfiguredSellBaseAmount
	}
	summary.ProcurementCostYYC = summary.ProcurementCostBaseAmount * cnyChargeRate
	summary.GrossProfitYYC = summary.GrossProfitBaseAmount * cnyChargeRate
	summary.TargetMargin = normalizeTargetMargin(config.BillingTargetMargin)
	summary.RiskBuffer = normalizeRiskBuffer(config.BillingRiskBuffer)
	summary.Items = rows
	return summary, nil
}
