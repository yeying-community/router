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
