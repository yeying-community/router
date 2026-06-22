package billing

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/admin/model"
	billingsvc "github.com/yeying-community/router/internal/admin/service/billing"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

func usdChargeRate() float64 {
	value, err := model.GetBillingCurrencyChargeRate(model.BillingCurrencyCodeUSD)
	if err != nil || value <= 0 {
		return config.QuotaPerUnit
	}
	return value
}

type upsertBillingCurrencyRequest struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Symbol     string  `json:"symbol"`
	MinorUnit  int     `json:"minor_unit"`
	ChargeRate float64 `json:"charge_rate"`
	Status     int     `json:"status"`
	Source     string  `json:"source"`
}

type publicBillingCurrencyItem struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Symbol     string  `json:"symbol"`
	MinorUnit  int     `json:"minor_unit"`
	ChargeRate float64 `json:"charge_rate"`
}

type procurementReportItem struct {
	model.ProcurementReportItem
	DimensionName          string                            `json:"dimension_name"`
	UnconfiguredChannels   []procurementReportRelatedChannel `json:"unconfigured_channels,omitempty"`
	UnconfiguredChannelCnt int                               `json:"unconfigured_channel_count,omitempty"`
}

type procurementReportRelatedChannel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	RequestCount  int64  `json:"request_count"`
	LastRequestAt int64  `json:"last_request_at"`
}

type procurementReportResponse struct {
	model.ProcurementReportSummary
	Items []procurementReportItem `json:"items"`
}

type billingHealthIssue struct {
	Key     string `json:"key"`
	Level   string `json:"level"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Count   int64  `json:"count,omitempty"`
	Link    string `json:"link,omitempty"`
}

type billingHealthResponse struct {
	Status        string               `json:"status"`
	CheckedAt     int64                `json:"checked_at"`
	WindowStartAt int64                `json:"window_start_at"`
	WindowEndAt   int64                `json:"window_end_at"`
	CriticalCount int                  `json:"critical_count"`
	WarningCount  int                  `json:"warning_count"`
	Issues        []billingHealthIssue `json:"issues"`
}

func parseBillingReportTimestamp(value string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func loadProcurementReportChannelNames(channelIDs []string) map[string]string {
	seen := make(map[string]struct{}, len(channelIDs))
	normalizedIDs := make([]string, 0, len(channelIDs))
	for _, raw := range channelIDs {
		channelID := strings.TrimSpace(raw)
		if channelID == "" || channelID == "-" {
			continue
		}
		if _, ok := seen[channelID]; ok {
			continue
		}
		seen[channelID] = struct{}{}
		normalizedIDs = append(normalizedIDs, channelID)
	}
	result := make(map[string]string, len(normalizedIDs))
	if len(normalizedIDs) == 0 {
		return result
	}
	rows := make([]model.Channel, 0, len(normalizedIDs))
	if err := model.DB.Select("id", "name").Where("id IN ?", normalizedIDs).Find(&rows).Error; err != nil {
		return result
	}
	for _, row := range rows {
		result[strings.TrimSpace(row.Id)] = strings.TrimSpace(row.DisplayName())
	}
	return result
}

func procurementReportUnconfiguredCostCondition() string {
	return "billing_procurement_cost_source NOT IN ? OR COALESCE(NULLIF(TRIM(billing_procurement_cost_source), ''), '') = ''"
}

func loadProcurementReportUnconfiguredModelChannels(summary model.ProcurementReportSummary) map[string][]procurementReportRelatedChannel {
	result := map[string][]procurementReportRelatedChannel{}
	if summary.GroupBy != model.ProcurementReportGroupByModel || summary.StartAt <= 0 || summary.EndAt <= 0 {
		return result
	}
	type modelChannelRow struct {
		ModelKey      string `gorm:"column:model_key"`
		ChannelID     string `gorm:"column:channel_id"`
		RequestCount  int64  `gorm:"column:request_count"`
		LastRequestAt int64  `gorm:"column:last_request_at"`
	}
	rows := make([]modelChannelRow, 0)
	configuredSources := []string{
		model.ProcurementCostSourceActual,
		model.ProcurementCostSourceEstimated,
		model.ProcurementCostSourceZeroCost,
	}
	query := model.LOG_DB.Table(model.EventLogsTableName).
		Select(`
			COALESCE(NULLIF(TRIM(model_name), ''), '-') AS model_key,
			COALESCE(NULLIF(TRIM(channel_id), ''), '-') AS channel_id,
			COUNT(1) AS request_count,
			COALESCE(MAX(created_at), 0) AS last_request_at
		`).
		Where("type = ? AND created_at BETWEEN ? AND ?", model.LogTypeConsume, summary.StartAt, summary.EndAt).
		Where(procurementReportUnconfiguredCostCondition(), configuredSources)
	if summary.GroupID != "" {
		query = query.Where("group_id = ?", summary.GroupID)
	}
	err := query.
		Group("model_key, channel_id").
		Order("model_key ASC, request_count DESC, last_request_at DESC").
		Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return result
	}
	channelIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		channelID := strings.TrimSpace(row.ChannelID)
		if channelID == "" || channelID == "-" {
			continue
		}
		channelIDs = append(channelIDs, channelID)
	}
	channelNames := loadProcurementReportChannelNames(channelIDs)
	for _, row := range rows {
		modelKey := strings.TrimSpace(row.ModelKey)
		channelID := strings.TrimSpace(row.ChannelID)
		if modelKey == "" || modelKey == "-" || channelID == "" || channelID == "-" {
			continue
		}
		name := strings.TrimSpace(channelNames[channelID])
		if name == "" {
			name = channelID
		}
		result[modelKey] = append(result[modelKey], procurementReportRelatedChannel{
			ID:            channelID,
			Name:          name,
			RequestCount:  row.RequestCount,
			LastRequestAt: row.LastRequestAt,
		})
	}
	return result
}

func buildProcurementReportResponse(summary model.ProcurementReportSummary) procurementReportResponse {
	response := procurementReportResponse{
		ProcurementReportSummary: summary,
		Items:                    make([]procurementReportItem, 0, len(summary.Items)),
	}
	channelNames := map[string]string{}
	if summary.GroupBy == model.ProcurementReportGroupByChannel {
		channelIDs := make([]string, 0, len(summary.Items))
		for _, item := range summary.Items {
			channelIDs = append(channelIDs, strings.TrimSpace(item.DimensionKey))
		}
		channelNames = loadProcurementReportChannelNames(channelIDs)
	}
	unconfiguredModelChannels := loadProcurementReportUnconfiguredModelChannels(summary)
	for _, item := range summary.Items {
		nextItem := procurementReportItem{ProcurementReportItem: item}
		switch summary.GroupBy {
		case model.ProcurementReportGroupByChannel:
			nextItem.DimensionName = channelNames[strings.TrimSpace(item.DimensionKey)]
		case model.ProcurementReportGroupByModel:
			nextItem.DimensionName = strings.TrimSpace(item.DimensionKey)
			relatedChannels := unconfiguredModelChannels[strings.TrimSpace(item.DimensionKey)]
			nextItem.UnconfiguredChannelCnt = len(relatedChannels)
			if len(relatedChannels) > 5 {
				nextItem.UnconfiguredChannels = relatedChannels[:5]
			} else {
				nextItem.UnconfiguredChannels = relatedChannels
			}
		}
		if strings.TrimSpace(nextItem.DimensionName) == "" {
			nextItem.DimensionName = strings.TrimSpace(item.DimensionKey)
		}
		response.Items = append(response.Items, nextItem)
	}
	response.ProcurementReportSummary.Items = nil
	return response
}

func appendBillingHealthIssue(response *billingHealthResponse, issue billingHealthIssue) {
	if response == nil {
		return
	}
	issue.Level = strings.TrimSpace(strings.ToLower(issue.Level))
	if issue.Level == "" {
		issue.Level = "warning"
	}
	response.Issues = append(response.Issues, issue)
	switch issue.Level {
	case "critical":
		response.CriticalCount++
	default:
		response.WarningCount++
	}
}

func appendBillingCurrencyHealthIssues(response *billingHealthResponse) {
	rows, err := model.ListBillingCurrencies()
	if err != nil {
		appendBillingHealthIssue(response, billingHealthIssue{
			Key:     "currency_catalog_load_failed",
			Level:   "critical",
			Title:   "计费币种加载失败",
			Message: "无法加载计费币种，所有按币种折算的请求都可能失败: " + err.Error(),
			Link:    "/admin/setting?tab=currency&section=catalog",
		})
		return
	}
	byCode := make(map[string]model.BillingCurrency, len(rows))
	for _, row := range rows {
		code := strings.ToUpper(strings.TrimSpace(row.Code))
		if code == "" {
			continue
		}
		row.Code = code
		byCode[code] = row
	}
	requiredCodes := []string{
		model.BillingCurrencyCodeUSD,
		model.BillingCurrencyCodeCNY,
		model.BillingCurrencyCodeYYC,
	}
	for _, code := range requiredCodes {
		row, ok := byCode[code]
		if !ok {
			appendBillingHealthIssue(response, billingHealthIssue{
				Key:     "currency_missing_" + strings.ToLower(code),
				Level:   "critical",
				Title:   "计费币种缺失: " + code,
				Message: "请求计费需要 " + code + " 的扣减比率，请在币种配置中补齐。",
				Link:    "/admin/setting?tab=currency&section=catalog",
			})
			continue
		}
		if row.Status != model.BillingCurrencyStatusEnabled {
			appendBillingHealthIssue(response, billingHealthIssue{
				Key:     "currency_disabled_" + strings.ToLower(code),
				Level:   "critical",
				Title:   "计费币种已停用: " + code,
				Message: code + " 已停用，命中该币种价格的请求会扣费失败。",
				Link:    "/admin/setting?tab=currency&section=catalog",
			})
		}
		if row.ChargeRate <= 0 {
			appendBillingHealthIssue(response, billingHealthIssue{
				Key:     "currency_charge_rate_invalid_" + strings.ToLower(code),
				Level:   "critical",
				Title:   "计费币种扣减比率无效: " + code,
				Message: code + " 的扣减比率必须大于 0，否则命中该币种价格的请求会扣费失败。",
				Link:    "/admin/setting?tab=currency&section=catalog",
			})
		}
	}
	for _, row := range rows {
		code := strings.ToUpper(strings.TrimSpace(row.Code))
		if code == "" {
			continue
		}
		if row.Status == model.BillingCurrencyStatusEnabled && row.ChargeRate <= 0 {
			if code == model.BillingCurrencyCodeUSD || code == model.BillingCurrencyCodeCNY || code == model.BillingCurrencyCodeYYC {
				continue
			}
			appendBillingHealthIssue(response, billingHealthIssue{
				Key:     "enabled_currency_charge_rate_invalid_" + strings.ToLower(code),
				Level:   "critical",
				Title:   "启用币种扣减比率无效: " + code,
				Message: code + " 处于启用状态，但扣减比率不是有效正数。",
				Link:    "/admin/setting?tab=currency&section=catalog",
			})
		}
	}
}

func appendProviderPricingHealthIssues(response *billingHealthResponse) {
	if model.DB == nil {
		appendBillingHealthIssue(response, billingHealthIssue{
			Key:     "provider_pricing_db_unavailable",
			Level:   "critical",
			Title:   "模型价格检查不可用",
			Message: "主数据库不可用，无法检查供应商模型价格。",
		})
		return
	}
	type missingPricingRow struct {
		Provider string `gorm:"column:provider"`
		Model    string `gorm:"column:model"`
	}
	baseQuery := `
		FROM provider_models pm
		LEFT JOIN (
			SELECT provider, model, COUNT(1) AS priced_components
			FROM provider_model_price_components
			WHERE COALESCE(input_price, 0) > 0 OR COALESCE(output_price, 0) > 0
			GROUP BY provider, model
		) pc ON pc.provider = pm.provider AND pc.model = pm.model
		WHERE COALESCE(pm.is_deleted, FALSE) = FALSE
		  AND COALESCE(NULLIF(TRIM(pm.status), ''), 'active') = 'active'
		  AND COALESCE(pm.input_price, 0) <= 0
		  AND COALESCE(pm.output_price, 0) <= 0
		  AND COALESCE(pc.priced_components, 0) = 0
	`
	var count int64
	if err := model.DB.Raw("SELECT COUNT(1) " + baseQuery).Scan(&count).Error; err != nil {
		appendBillingHealthIssue(response, billingHealthIssue{
			Key:     "provider_pricing_check_failed",
			Level:   "warning",
			Title:   "模型价格检查失败",
			Message: "无法检查供应商模型价格: " + err.Error(),
		})
		return
	}
	if count <= 0 {
		return
	}
	rows := make([]missingPricingRow, 0)
	_ = model.DB.Raw(`
		SELECT pm.provider, pm.model
		` + baseQuery + `
		ORDER BY pm.provider ASC, pm.model ASC
		LIMIT 8
	`).Scan(&rows).Error
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		label := strings.TrimSpace(row.Provider) + "/" + strings.TrimSpace(row.Model)
		labels = append(labels, strings.Trim(label, "/"))
	}
	message := "存在启用中的供应商模型没有配置价格，命中这些模型时会返回 model_pricing_not_configured。"
	if len(labels) > 0 {
		message += " 示例: " + strings.Join(labels, "、")
	}
	appendBillingHealthIssue(response, billingHealthIssue{
		Key:     "provider_pricing_missing",
		Level:   "critical",
		Title:   "供应商模型价格缺失",
		Message: message,
		Count:   count,
	})
}

func appendProcurementCostHealthIssues(response *billingHealthResponse) {
	if model.LOG_DB == nil {
		appendBillingHealthIssue(response, billingHealthIssue{
			Key:     "procurement_log_db_unavailable",
			Level:   "warning",
			Title:   "采购成本检查不可用",
			Message: "日志数据库不可用，无法检查近期未配置采购成本的请求。",
		})
		return
	}
	summary, err := model.ListProcurementReportWithDB(model.LOG_DB, model.ProcurementReportQuery{
		StartAt: response.WindowStartAt,
		EndAt:   response.WindowEndAt,
		GroupBy: model.ProcurementReportGroupByModel,
	})
	if err != nil {
		appendBillingHealthIssue(response, billingHealthIssue{
			Key:     "procurement_cost_check_failed",
			Level:   "warning",
			Title:   "采购成本检查失败",
			Message: "无法检查近期采购成本配置: " + err.Error(),
		})
		return
	}
	if summary.UnconfiguredCostRequestCount <= 0 {
		return
	}
	labels := make([]string, 0, 5)
	for _, item := range summary.Items {
		if item.UnconfiguredCostRequestCount <= 0 {
			continue
		}
		labels = append(labels, strings.TrimSpace(item.DimensionKey))
		if len(labels) >= 5 {
			break
		}
	}
	message := "最近 7 天存在请求没有归因到采购成本，毛利报表会低估成本风险。"
	if len(labels) > 0 {
		message += " 优先处理模型: " + strings.Join(labels, "、")
	}
	appendBillingHealthIssue(response, billingHealthIssue{
		Key:     "procurement_cost_unconfigured",
		Level:   "warning",
		Title:   "近期请求未配置采购成本",
		Message: message,
		Count:   summary.UnconfiguredCostRequestCount,
		Link:    "/admin/billing/procurement-report",
	})
}

func appendPricingPolicyHealthIssues(response *billingHealthResponse) {
	if config.BillingOfficialMarkup <= 1 && config.BillingTargetMargin <= 0 && config.BillingRiskBuffer <= 0 {
		appendBillingHealthIssue(response, billingHealthIssue{
			Key:     "pricing_policy_no_margin",
			Level:   "warning",
			Title:   "销售计价未配置利润保护",
			Message: "当前官方价格倍率、目标利润率和风险缓冲都没有形成加价保护。生产环境建议至少配置一个利润保护参数。",
			Link:    "/admin/setting?tab=operation&section=config",
		})
	}
	if config.FXAutoSyncEnabled && strings.TrimSpace(config.FXAutoSyncLastError) != "" {
		appendBillingHealthIssue(response, billingHealthIssue{
			Key:     "fx_sync_last_error",
			Level:   "warning",
			Title:   "汇率同步最近一次失败",
			Message: "汇率同步最近一次失败: " + strings.TrimSpace(config.FXAutoSyncLastError),
			Link:    "/admin/setting?tab=exchange&section=rates",
		})
	}
}

func GetBillingHealth(c *gin.Context) {
	now := helper.GetTimestamp()
	response := billingHealthResponse{
		Status:        "ok",
		CheckedAt:     now,
		WindowStartAt: now - 7*24*60*60,
		WindowEndAt:   now,
		Issues:        []billingHealthIssue{},
	}
	appendBillingCurrencyHealthIssues(&response)
	appendProviderPricingHealthIssues(&response)
	appendProcurementCostHealthIssues(&response)
	appendPricingPolicyHealthIssues(&response)
	if response.CriticalCount > 0 {
		response.Status = "critical"
	} else if response.WarningCount > 0 {
		response.Status = "warning"
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    response,
	})
}

func GetProcurementReport(c *gin.Context) {
	startAt := parseBillingReportTimestamp(c.Query("start_at"))
	if startAt == 0 {
		startAt = parseBillingReportTimestamp(c.Query("start_timestamp"))
	}
	endAt := parseBillingReportTimestamp(c.Query("end_at"))
	if endAt == 0 {
		endAt = parseBillingReportTimestamp(c.Query("end_timestamp"))
	}
	summary, err := model.ListProcurementReportWithDB(model.LOG_DB, model.ProcurementReportQuery{
		StartAt:   startAt,
		EndAt:     endAt,
		GroupBy:   c.Query("group_by"),
		CostScope: c.Query("cost_scope"),
		GroupID:   strings.TrimSpace(c.Query("group_id")),
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "加载采购成本报表失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    buildProcurementReportResponse(summary),
	})
}

func GetPublicBillingCurrencies(c *gin.Context) {
	rows, err := model.ListBillingCurrencies()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "加载计费币种失败: " + err.Error(),
		})
		return
	}

	items := make([]publicBillingCurrencyItem, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if row.Status != model.BillingCurrencyStatusEnabled || row.ChargeRate <= 0 {
			continue
		}
		code := strings.ToUpper(strings.TrimSpace(row.Code))
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		items = append(items, publicBillingCurrencyItem{
			Code:       code,
			Name:       row.Name,
			Symbol:     row.Symbol,
			MinorUnit:  row.MinorUnit,
			ChargeRate: row.ChargeRate,
		})
	}
	if _, ok := seen[model.BillingCurrencyCodeUSD]; !ok {
		items = append(items, publicBillingCurrencyItem{
			Code:       model.BillingCurrencyCodeUSD,
			Name:       "US Dollar",
			Symbol:     "$",
			MinorUnit:  6,
			ChargeRate: usdChargeRate(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"default_currency": model.BillingCurrencyCodeUSD,
			"items":            items,
		},
	})
}

func GetBillingCurrencies(c *gin.Context) {
	rows, err := model.ListBillingCurrencies()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "加载计费币种失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
}

func CreateBillingCurrency(c *gin.Context) {
	req := upsertBillingCurrencyRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	row, err := model.CreateBillingCurrencyWithDB(model.DB, model.BillingCurrency{
		Code:       req.Code,
		Name:       req.Name,
		Symbol:     req.Symbol,
		MinorUnit:  req.MinorUnit,
		ChargeRate: req.ChargeRate,
		Status:     req.Status,
		Source:     req.Source,
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    row,
	})
}

func UpdateBillingCurrency(c *gin.Context) {
	code := strings.TrimSpace(c.Param("code"))
	if code == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "币种代码不能为空",
		})
		return
	}
	req := upsertBillingCurrencyRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	row, err := model.UpdateBillingCurrencyWithDB(model.DB, code, func(current model.BillingCurrency) (model.BillingCurrency, error) {
		next := current
		next.Name = req.Name
		next.Symbol = req.Symbol
		next.MinorUnit = req.MinorUnit
		next.ChargeRate = req.ChargeRate
		next.Status = req.Status
		if strings.TrimSpace(req.Source) != "" {
			next.Source = req.Source
		} else if strings.TrimSpace(strings.ToLower(current.Source)) == model.BillingCurrencySourceSystemDefault {
			next.Source = model.BillingCurrencySourceManual
		}
		return next, nil
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    row,
	})
}

func DeleteBillingCurrency(c *gin.Context) {
	code := strings.TrimSpace(c.Param("code"))
	if code == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "币种代码不能为空",
		})
		return
	}
	if err := model.DeleteBillingCurrencyWithDB(model.DB, code); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func SyncBillingCurrenciesFromFX(c *gin.Context) {
	runAt := helper.GetTimestamp()
	_ = model.UpdateOption("FXAutoSyncLastRunAt", strconv.FormatInt(runAt, 10))

	result, err := billingsvc.SyncFXMarketRates(c.Request.Context())
	if err != nil {
		_ = model.UpdateOption("FXAutoSyncLastError", strings.TrimSpace(err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "同步汇率失败: " + err.Error(),
		})
		return
	}
	_ = model.UpdateOption("FXAutoSyncLastSuccessAt", strconv.FormatInt(runAt, 10))
	_ = model.UpdateOption("FXAutoSyncLastError", "")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}

func GetFXSyncStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"enabled":            config.FXAutoSyncEnabled,
			"interval_seconds":   config.FXAutoSyncIntervalSeconds,
			"provider":           config.FXAutoSyncProvider,
			"last_run_at":        config.FXAutoSyncLastRunAt,
			"last_success_at":    config.FXAutoSyncLastSuccessAt,
			"last_error":         config.FXAutoSyncLastError,
			"min_interval":       60,
			"loop_check_seconds": 30,
		},
	})
}

func GetFXMarketRates(c *gin.Context) {
	currenciesParam := strings.TrimSpace(c.Query("currencies"))
	currencies := make([]string, 0)
	if currenciesParam != "" {
		currencies = append(currencies, strings.Split(currenciesParam, ",")...)
	}

	result, err := billingsvc.GetFXMarketRates(c.Request.Context(), currencies)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "加载汇率列表失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}

func GetSubscription(c *gin.Context) {
	var remainQuota int64
	var usedQuota int64
	var err error
	var token *model.Token
	var expiredTime int64
	tokenId := c.GetString(ctxkey.TokenId)
	if tokenId != "" {
		token, err = billingsvc.GetTokenByID(tokenId)
		if err == nil {
			expiredTime = token.ExpiredTime
			remainQuota = token.RemainQuota
			usedQuota = token.UsedQuota
		}
	}
	if token == nil {
		userId := c.GetString(ctxkey.Id)
		remainQuota, err = billingsvc.GetUserQuota(userId)
		if err != nil {
			usedQuota, err = billingsvc.GetUserUsedQuota(userId)
		}
	}
	if expiredTime <= 0 {
		expiredTime = 0
	}
	if err != nil {
		Error := relaymodel.Error{
			Message: err.Error(),
			Type:    "upstream_error",
		}
		c.JSON(200, gin.H{
			"error": Error,
		})
		return
	}
	quota := remainQuota + usedQuota
	amount := float64(quota) / usdChargeRate()
	if token != nil && token.UnlimitedQuota {
		amount = 100000000
	}
	subscription := billingsvc.OpenAISubscriptionResponse{
		Object:             "billing_subscription",
		HasPaymentMethod:   true,
		SoftLimitUSD:       amount,
		HardLimitUSD:       amount,
		SystemHardLimitUSD: amount,
		AccessUntil:        expiredTime,
	}
	c.JSON(200, subscription)
	return
}

func GetUsage(c *gin.Context) {
	var quota int64
	var err error
	var token *model.Token
	tokenId := c.GetString(ctxkey.TokenId)
	if tokenId != "" {
		token, err = billingsvc.GetTokenByID(tokenId)
		if err == nil {
			quota = token.UsedQuota
		}
	}
	if token == nil {
		userId := c.GetString(ctxkey.Id)
		quota, err = billingsvc.GetUserUsedQuota(userId)
	}
	if err != nil {
		Error := relaymodel.Error{
			Message: err.Error(),
			Type:    "one_api_error",
		}
		c.JSON(200, gin.H{
			"error": Error,
		})
		return
	}
	amount := float64(quota) / usdChargeRate()
	usage := billingsvc.OpenAIUsageResponse{
		Object:     "list",
		TotalUsage: amount * 100,
	}
	c.JSON(200, usage)
	return
}
