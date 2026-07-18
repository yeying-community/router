package channel

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/admin/model"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
	"gorm.io/gorm"
)

type channelBillingSummaryData struct {
	ChannelID             string                             `json:"channel_id"`
	ProfileEnabled        bool                               `json:"profile_enabled"`
	BillingMode           string                             `json:"billing_mode"`
	ActionCapabilities    []string                           `json:"action_capabilities"`
	RefreshSupported      bool                               `json:"refresh_supported"`
	LatestSnapshotAt      int64                              `json:"latest_snapshot_at"`
	LatestSnapshotStatus  string                             `json:"latest_snapshot_status"`
	LatestSnapshotMessage string                             `json:"latest_snapshot_message"`
	QuotaItems            []model.ChannelBillingSnapshotItem `json:"quota_items"`
}

type channelBillingProfileData struct {
	ChannelID          string   `json:"channel_id"`
	Enabled            bool     `json:"enabled"`
	BillingMode        string   `json:"billing_mode"`
	BillingAPIBaseURL  string   `json:"billing_api_base_url"`
	CDK                string   `json:"cdk"`
	Currency           string   `json:"currency"`
	ActionCapabilities []string `json:"action_capabilities"`
}

type channelBillingListData[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

type channelBillingAlertFeedItem struct {
	model.ChannelBillingAlertEvent
	ChannelName string `json:"channel_name"`
}

type channelBillingManualSnapshotRequest struct {
	PurchaseAt         int64                                  `json:"purchase_at"`
	PurchaseCurrency   string                                 `json:"purchase_currency"`
	PurchaseAmount     float64                                `json:"purchase_amount"`
	PurchaseFXRate     float64                                `json:"purchase_fx_rate"`
	PurchaseCostAmount float64                                `json:"purchase_cost_amount"`
	EntitlementName    string                                 `json:"entitlement_name"`
	ValidFrom          int64                                  `json:"valid_from"`
	ValidUntil         int64                                  `json:"valid_until"`
	Items              []channelBillingManualQuotaItemRequest `json:"items"`
	Message            string                                 `json:"message"`
}

type channelBillingManualQuotaItemRequest struct {
	Id              string  `json:"id"`
	ResourceType    string  `json:"resource_type"`
	QuotaType       string  `json:"quota_type"`
	QuotaLabel      string  `json:"quota_label"`
	Amount          float64 `json:"amount"`
	LimitAmount     float64 `json:"limit_amount"`
	UsedAmount      float64 `json:"used_amount"`
	RemainingAmount float64 `json:"remaining_amount"`
	Currency        string  `json:"currency"`
	ResetAt         int64   `json:"reset_at"`
	ExpiresAt       int64   `json:"expires_at"`
	SourceRef       string  `json:"source_ref"`
}

type channelProcurementBatchCostUpdateRequest struct {
	PurchaseCurrency   string  `json:"purchase_currency"`
	PurchaseAmount     float64 `json:"purchase_amount"`
	PurchaseFXRate     float64 `json:"purchase_fx_rate"`
	PurchaseCostAmount float64 `json:"purchase_cost_amount"`
	CapacityEffective  float64 `json:"capacity_effective"`
	CostSource         string  `json:"cost_source"`
	CostStatus         string  `json:"cost_status"`
	ScopeType          string  `json:"scope_type"`
	ScopeValue         string  `json:"scope_value"`
}

type channelProcurementBatchStatusUpdateRequest struct {
	CostStatus string `json:"cost_status"`
}

func isSupportedBillingResourceType(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case model.ChannelBillingResourceTypeQuota,
		model.ChannelBillingResourceTypeBalance,
		model.ChannelBillingResourceTypeCredit,
		model.ChannelBillingResourceTypePlan:
		return true
	default:
		return false
	}
}

type channelBillingProfileUpdateRequest struct {
	BillingMode       string `json:"billing_mode"`
	BillingAPIBaseURL string `json:"billing_api_base_url"`
	CDK               string `json:"cdk"`
	Currency          string `json:"currency"`
}

func buildChannelBillingSummary(channelRow *model.Channel, profile model.ChannelBillingProfile, snapshot model.ChannelBillingSnapshot) channelBillingSummaryData {
	capabilities := profile.ParseActionCapabilities()
	summary := channelBillingSummaryData{
		ChannelID:             strings.TrimSpace(channelRow.Id),
		ProfileEnabled:        profile.Enabled,
		BillingMode:           strings.TrimSpace(profile.BillingMode),
		ActionCapabilities:    capabilities,
		RefreshSupported:      profile.HasCapability(model.ChannelBillingCapabilityRefreshBilling),
		LatestSnapshotAt:      snapshot.CreatedAt,
		LatestSnapshotStatus:  strings.TrimSpace(snapshot.RawStatus),
		LatestSnapshotMessage: strings.TrimSpace(snapshot.Message),
		QuotaItems:            model.NormalizeChannelBillingSnapshotItems(snapshot.Items),
	}
	if summary.BillingMode == "" {
		summary.BillingMode = model.ChannelBillingModeUnsupported
	}
	return summary
}

func buildChannelBillingProfileData(channelRow *model.Channel, profile model.ChannelBillingProfile) channelBillingProfileData {
	fetchConfig := profile.ParseBillingConfig()
	return channelBillingProfileData{
		ChannelID:          strings.TrimSpace(channelRow.Id),
		Enabled:            profile.Enabled,
		BillingMode:        strings.TrimSpace(profile.BillingMode),
		BillingAPIBaseURL:  strings.TrimSpace(fetchConfig.APIBaseURL),
		CDK:                strings.TrimSpace(fetchConfig.CDK),
		Currency:           strings.TrimSpace(fetchConfig.Currency),
		ActionCapabilities: profile.ParseActionCapabilities(),
	}
}

func attachManualPurchaseCostToSnapshotBatches(tx *gorm.DB, snapshotID string, purchaseCurrency string, purchaseAmount float64, purchaseFXRate float64, purchaseCostAmount float64) error {
	normalizedSnapshotID := strings.TrimSpace(snapshotID)
	if normalizedSnapshotID == "" || purchaseCostAmount <= 0 {
		return nil
	}
	rows := make([]model.ChannelProcurementBatch, 0)
	if err := tx.Where("source_snapshot_id = ?", normalizedSnapshotID).Find(&rows).Error; err != nil {
		return err
	}
	type costGroup struct {
		rows     []model.ChannelProcurementBatch
		primary  int
		capacity float64
	}
	groups := make(map[string]*costGroup)
	groupOrder := make([]string, 0)
	for _, row := range rows {
		key := strings.Join([]string{row.ScopeType, row.ScopeValue, row.CapacityUnit}, "|")
		group := groups[key]
		if group == nil {
			group = &costGroup{primary: -1}
			groups[key] = group
			groupOrder = append(groupOrder, key)
		}
		group.rows = append(group.rows, row)
		rowIndex := len(group.rows) - 1
		if group.primary < 0 || row.QuotaType == "total" || row.QuotaType == "custom" || row.ResetCycle == "none" {
			group.primary = rowIndex
			group.capacity = row.CapacityEffective
		}
	}
	totalEffectiveCapacity := 0.0
	for _, key := range groupOrder {
		if groups[key].capacity > 0 {
			totalEffectiveCapacity += groups[key].capacity
		}
	}
	if len(rows) == 0 || totalEffectiveCapacity <= 0 {
		return nil
	}
	now := helper.GetTimestamp()
	for _, key := range groupOrder {
		group := groups[key]
		if group.primary < 0 || group.capacity <= 0 {
			continue
		}
		ratio := group.capacity / totalEffectiveCapacity
		allocatedCostAmount := purchaseCostAmount * ratio
		if allocatedCostAmount <= 0 {
			continue
		}
		costPerUnitAmount := allocatedCostAmount / group.capacity
		for rowIndex, row := range group.rows {
			rowPurchaseAmount := 0.0
			rowCostAmount := 0.0
			rowUnitCost := 0.0
			if rowIndex == group.primary {
				rowPurchaseAmount = purchaseAmount * ratio
				rowCostAmount = allocatedCostAmount
				rowUnitCost = costPerUnitAmount
			}
			if err := tx.Model(&model.ChannelProcurementBatch{}).Where("id = ?", row.Id).Updates(map[string]any{
				"purchase_currency":    strings.TrimSpace(strings.ToUpper(purchaseCurrency)),
				"purchase_amount":      rowPurchaseAmount,
				"purchase_fx_rate":     purchaseFXRate,
				"purchase_cost_amount": rowCostAmount,
				"cost_per_unit_amount": rowUnitCost,
				"cost_source":          model.ProcurementCostSourceActual,
				"cost_status":          model.ProcurementCostStatusActive,
				"updated_at":           now,
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func getEffectiveChannelBillingProfile(channelID string) (*model.Channel, model.ChannelBillingProfile, error) {
	channelRow, err := channelsvc.GetByID(channelID)
	if err != nil {
		return nil, model.ChannelBillingProfile{}, err
	}
	profile, _, err := model.GetEffectiveChannelBillingProfileWithDB(model.DB, channelRow)
	if err != nil {
		return nil, model.ChannelBillingProfile{}, err
	}
	return channelRow, profile, nil
}

func GetChannelBilling(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	channelRow, profile, err := getEffectiveChannelBillingProfile(channelID)
	if err != nil {
		logChannelAdminWarn(c, "get_billing", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	latestSnapshot, err := model.GetLatestChannelBillingSnapshotByChannelIDWithDB(model.DB, channelID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logChannelAdminWarn(c, "get_billing", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    buildChannelBillingSummary(channelRow, profile, latestSnapshot),
	})
}

func GetChannelBillingProfile(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	channelRow, profile, err := getEffectiveChannelBillingProfile(channelID)
	if err != nil {
		logChannelAdminWarn(c, "get_billing_profile", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    buildChannelBillingProfileData(channelRow, profile),
	})
}

func GetChannelBillingSnapshots(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	rows, err := model.ListChannelBillingSnapshotsByChannelIDWithDB(model.DB, channelID, 50)
	if err != nil {
		logChannelAdminWarn(c, "list_billing_snapshots", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channelBillingListData[model.ChannelBillingSnapshot]{Items: rows, Total: len(rows)}})
}

func GetChannelBillingActions(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	rows, err := model.ListChannelBillingActionsByChannelIDWithDB(model.DB, channelID, 50)
	if err != nil {
		logChannelAdminWarn(c, "list_billing_actions", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channelBillingListData[model.ChannelBillingAction]{Items: rows, Total: len(rows)}})
}

func GetChannelBillingAlerts(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	rows, err := model.ListChannelBillingAlertEventsByChannelIDWithDB(model.DB, channelID, 50)
	if err != nil {
		logChannelAdminWarn(c, "list_billing_alerts", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channelBillingListData[model.ChannelBillingAlertEvent]{Items: rows, Total: len(rows)}})
}

func GetChannelProcurementBatches(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	rows, err := model.ListChannelProcurementBatchesByChannelIDWithDB(model.DB, channelID, 100)
	if err != nil {
		logChannelAdminWarn(c, "list_procurement_batches", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channelBillingListData[model.ChannelProcurementBatch]{Items: rows, Total: len(rows)}})
}

func UpdateChannelProcurementBatchCost(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	batchID := strings.TrimSpace(c.Param("batch_id"))
	if channelID == "" || batchID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次 ID 无效"})
		return
	}
	req := channelProcurementBatchCostUpdateRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请求参数无效"})
		return
	}
	current, err := model.GetChannelProcurementBatchByIDWithDB(model.DB, batchID)
	if err != nil {
		logChannelAdminWarn(c, "update_procurement_batch_cost", stringField("channel_id", channelID), stringField("batch_id", batchID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次不存在"})
		return
	}
	if strings.TrimSpace(current.ChannelId) != channelID {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次不属于当前渠道"})
		return
	}
	row, err := model.UpdateChannelProcurementBatchCostWithDB(model.DB, batchID, model.ProcurementBatchCostUpdate{
		PurchaseCurrency:   req.PurchaseCurrency,
		PurchaseAmount:     req.PurchaseAmount,
		PurchaseFXRate:     req.PurchaseFXRate,
		PurchaseCostAmount: req.PurchaseCostAmount,
		CapacityEffective:  req.CapacityEffective,
		CostSource:         req.CostSource,
		CostStatus:         req.CostStatus,
		ScopeType:          req.ScopeType,
		ScopeValue:         req.ScopeValue,
	})
	if err != nil {
		logChannelAdminWarn(c, "update_procurement_batch_cost", stringField("channel_id", channelID), stringField("batch_id", batchID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "update_procurement_batch_cost", stringField("channel_id", channelID), stringField("batch_id", batchID))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func UpdateChannelProcurementBatchStatus(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	batchID := strings.TrimSpace(c.Param("batch_id"))
	if channelID == "" || batchID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次 ID 无效"})
		return
	}
	req := channelProcurementBatchStatusUpdateRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请求参数无效"})
		return
	}
	current, err := model.GetChannelProcurementBatchByIDWithDB(model.DB, batchID)
	if err != nil {
		logChannelAdminWarn(c, "update_procurement_batch_status", stringField("channel_id", channelID), stringField("batch_id", batchID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次不存在"})
		return
	}
	if strings.TrimSpace(current.ChannelId) != channelID {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次不属于当前渠道"})
		return
	}
	row, err := model.UpdateChannelProcurementBatchStatusWithDB(model.DB, batchID, model.ProcurementBatchStatusUpdate{
		CostStatus: req.CostStatus,
	})
	if err != nil {
		logChannelAdminWarn(c, "update_procurement_batch_status", stringField("channel_id", channelID), stringField("batch_id", batchID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "update_procurement_batch_status", stringField("channel_id", channelID), stringField("batch_id", batchID), stringField("cost_status", row.CostStatus))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func GetChannelProcurementBatchConsumptions(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	batchID := strings.TrimSpace(c.Param("batch_id"))
	if channelID == "" || batchID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次 ID 无效"})
		return
	}
	current, err := model.GetChannelProcurementBatchByIDWithDB(model.DB, batchID)
	if err != nil {
		logChannelAdminWarn(c, "list_procurement_batch_consumptions", stringField("channel_id", channelID), stringField("batch_id", batchID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次不存在"})
		return
	}
	if strings.TrimSpace(current.ChannelId) != channelID {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次不属于当前渠道"})
		return
	}
	rows, err := model.ListRequestProcurementConsumptionsByBatchIDWithDB(model.DB, batchID, 100)
	if err != nil {
		logChannelAdminWarn(c, "list_procurement_batch_consumptions", stringField("channel_id", channelID), stringField("batch_id", batchID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channelBillingListData[model.RequestProcurementConsumption]{Items: rows, Total: len(rows)}})
}

func GetRecentChannelBillingAlerts(c *gin.Context) {
	rows, err := model.ListRecentChannelBillingAlertEventsWithDB(model.DB, 20)
	if err != nil {
		logChannelAdminWarn(c, "list_recent_billing_alerts", stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	channelIDs := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		channelID := strings.TrimSpace(row.ChannelId)
		if channelID == "" {
			continue
		}
		if _, ok := seen[channelID]; ok {
			continue
		}
		seen[channelID] = struct{}{}
		channelIDs = append(channelIDs, channelID)
	}
	channelNameByID := make(map[string]string, len(channelIDs))
	if len(channelIDs) > 0 {
		channels := make([]model.Channel, 0, len(channelIDs))
		if err := model.DB.Select("id", "name").Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
			logChannelAdminWarn(c, "list_recent_billing_alerts", stringField("reason", err.Error()))
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		for _, channelRow := range channels {
			channelNameByID[strings.TrimSpace(channelRow.Id)] = strings.TrimSpace(channelRow.DisplayName())
		}
	}
	items := make([]channelBillingAlertFeedItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, channelBillingAlertFeedItem{
			ChannelBillingAlertEvent: row,
			ChannelName:              channelNameByID[strings.TrimSpace(row.ChannelId)],
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": channelBillingListData[channelBillingAlertFeedItem]{
			Items: items,
			Total: len(items),
		},
	})
}

func UpdateChannelBillingProfile(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	req := channelBillingProfileUpdateRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	channelRow, err := channelsvc.GetByID(channelID)
	if err != nil {
		logChannelAdminWarn(c, "update_billing_profile", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	profileRow, err := model.GetChannelBillingProfileByChannelIDWithDB(model.DB, channelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			initialProfile, ok := model.BuildChannelBillingProfileFromChannelConfig(channelRow)
			if !ok {
				profileRow = model.ChannelBillingProfile{
					ChannelId:          channelID,
					Enabled:            true,
					BillingMode:        model.ChannelBillingModeUnsupported,
					ActionCapabilities: "[]",
				}
			} else {
				profileRow = initialProfile
			}
		} else {
			logChannelAdminWarn(c, "update_billing_profile", stringField("channel_id", channelID), stringField("reason", err.Error()))
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
	}
	nextMode := strings.TrimSpace(req.BillingMode)
	if nextMode == "" {
		nextMode = model.ChannelBillingModeUnsupported
	}
	switch nextMode {
	case model.ChannelBillingModeUnsupported,
		model.ChannelBillingModeManual,
		model.ChannelBillingModeBuiltinOpenAI,
		model.ChannelBillingModeBuiltinCloseAI,
		model.ChannelBillingModeBuiltinOpenAISB,
		model.ChannelBillingModeBuiltinAIProxy,
		model.ChannelBillingModeBuiltinAPI2GPT,
		model.ChannelBillingModeBuiltinAIGC2D,
		model.ChannelBillingModeBuiltinSiliconFlow,
		model.ChannelBillingModeBuiltinDeepSeek,
		model.ChannelBillingModeBuiltinOpenRouter,
		model.ChannelBillingModeBuiltinCDK:
	default:
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "账务刷新方式无效"})
		return
	}
	profileRow.Enabled = true
	profileRow.BillingMode = nextMode
	nextConfig := map[string]any{
		"api_base_url": strings.TrimSpace(req.BillingAPIBaseURL),
		"cdk":          strings.TrimSpace(req.CDK),
		"currency":     strings.TrimSpace(strings.ToUpper(req.Currency)),
	}
	profileRow.BillingConfig = marshalLogJSON(nextConfig)
	capabilities := []string{model.ChannelBillingCapabilityManualUpdateSnapshot}
	if nextMode != model.ChannelBillingModeUnsupported && nextMode != model.ChannelBillingModeManual {
		capabilities = append(capabilities, model.ChannelBillingCapabilityRefreshBilling)
	}
	profileRow.ActionCapabilities = marshalLogJSON(capabilities)
	savedRow, err := model.SaveChannelBillingProfileWithDB(model.DB, profileRow)
	if err != nil {
		logChannelAdminWarn(c, "update_billing_profile", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "update_billing_profile", stringField("channel_id", channelID))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    buildChannelBillingProfileData(channelRow, savedRow),
	})
}

func marshalLogJSON(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(body)
}

func maskBillingSecret(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 6 {
		return "***"
	}
	return trimmed[:3] + "***" + trimmed[len(trimmed)-3:]
}

func nearlyEqualFloat(left float64, right float64) bool {
	if left > right {
		return left-right < 0.0000001
	}
	return right-left < 0.0000001
}

func ensureConsumedSnapshotImmutableFields(current model.ChannelBillingSnapshot, next normalizedManualBillingSnapshotRequest) error {
	if current.PurchaseAt != next.PurchaseAt ||
		strings.TrimSpace(strings.ToUpper(current.PurchaseCurrency)) != strings.TrimSpace(strings.ToUpper(next.PurchaseCurrency)) ||
		!nearlyEqualFloat(current.PurchaseAmount, next.PurchaseAmount) ||
		!nearlyEqualFloat(current.PurchaseFXRate, next.PurchaseFXRate) ||
		!nearlyEqualFloat(current.PurchaseCostAmount, next.PurchaseCostAmount) {
		return fmt.Errorf("该采购记录已经被消耗，只能修正权益类型、有效期、名称和备注")
	}
	return nil
}

func sumProcurementConsumedQuantityWithDB(tx *gorm.DB, batchID string) (float64, error) {
	normalizedBatchID := strings.TrimSpace(batchID)
	if normalizedBatchID == "" {
		return 0, nil
	}
	var consumed float64
	err := tx.Model(&model.RequestProcurementConsumption{}).
		Where("procurement_batch_id = ?", normalizedBatchID).
		Select("COALESCE(SUM(consumed_quantity), 0)").
		Scan(&consumed).Error
	return consumed, err
}

func updateConsumedManualBillingSnapshotWithDB(tx *gorm.DB, current model.ChannelBillingSnapshot, channelID string, req normalizedManualBillingSnapshotRequest, operatorUserID string) ([]model.ChannelBillingSnapshotItem, error) {
	if err := ensureConsumedSnapshotImmutableFields(current, req); err != nil {
		return nil, err
	}
	existingItems := model.NormalizeChannelBillingSnapshotItems(current.Items)
	if len(existingItems) != len(req.Items) {
		return nil, fmt.Errorf("该采购记录已经被消耗，不能新增或删除权益项")
	}
	itemsByID := make(map[string]model.ChannelBillingSnapshotItem, len(existingItems))
	for _, item := range existingItems {
		if strings.TrimSpace(item.Id) != "" {
			itemsByID[strings.TrimSpace(item.Id)] = item
		}
	}
	updatedSnapshot, err := model.UpdateChannelBillingSnapshotPurchaseWithDB(tx, model.ChannelBillingSnapshot{
		Id:                 current.Id,
		ChannelId:          channelID,
		PurchaseAt:         req.PurchaseAt,
		PurchaseCurrency:   req.PurchaseCurrency,
		PurchaseAmount:     req.PurchaseAmount,
		PurchaseFXRate:     req.PurchaseFXRate,
		PurchaseCostAmount: req.PurchaseCostAmount,
		EntitlementName:    req.EntitlementName,
		ValidFrom:          req.ValidFrom,
		ValidUntil:         req.ValidUntil,
		Message:            req.Message,
		OperatorUserId:     operatorUserID,
	})
	if err != nil {
		return nil, err
	}
	now := helper.GetTimestamp()
	updatedItems := make([]model.ChannelBillingSnapshotItem, 0, len(req.Items))
	for index, nextItem := range req.Items {
		currentItem := model.ChannelBillingSnapshotItem{}
		if itemID := strings.TrimSpace(nextItem.Id); itemID != "" {
			matched, ok := itemsByID[itemID]
			if !ok {
				return nil, fmt.Errorf("该采购记录已经被消耗，权益项不存在")
			}
			currentItem = matched
		} else {
			currentItem = existingItems[index]
		}
		if strings.TrimSpace(currentItem.ResourceType) != strings.TrimSpace(nextItem.ResourceType) ||
			strings.TrimSpace(strings.ToUpper(currentItem.Currency)) != strings.TrimSpace(strings.ToUpper(nextItem.Currency)) ||
			!nearlyEqualFloat(currentItem.Amount, nextItem.Amount) ||
			!nearlyEqualFloat(currentItem.LimitAmount, nextItem.LimitAmount) ||
			!nearlyEqualFloat(currentItem.UsedAmount, nextItem.UsedAmount) ||
			!nearlyEqualFloat(currentItem.RemainingAmount, nextItem.RemainingAmount) {
			return nil, fmt.Errorf("该采购记录已经被消耗，不能修改权益容量或币种")
		}
		nextItem.Id = currentItem.Id
		nextItem.SnapshotId = current.Id
		nextItem.ChannelId = channelID
		nextItem.CreatedAt = currentItem.CreatedAt
		nextItem.SortOrder = currentItem.SortOrder
		if nextItem.SortOrder == 0 {
			nextItem.SortOrder = index + 1
		}
		if err := tx.Model(&model.ChannelBillingSnapshotItem{}).
			Where("id = ? AND snapshot_id = ?", currentItem.Id, current.Id).
			Updates(map[string]any{
				"quota_type":       strings.TrimSpace(strings.ToLower(nextItem.QuotaType)),
				"quota_label":      strings.TrimSpace(nextItem.QuotaLabel),
				"reset_at":         nextItem.ResetAt,
				"expires_at":       nextItem.ExpiresAt,
				"source_ref":       strings.TrimSpace(nextItem.SourceRef),
				"sort_order":       nextItem.SortOrder,
				"resource_type":    strings.TrimSpace(strings.ToLower(nextItem.ResourceType)),
				"amount":           nextItem.Amount,
				"limit_amount":     nextItem.LimitAmount,
				"used_amount":      nextItem.UsedAmount,
				"remaining_amount": nextItem.RemainingAmount,
				"currency":         strings.TrimSpace(strings.ToUpper(nextItem.Currency)),
			}).Error; err != nil {
			return nil, err
		}
		batchTemplate, ok := model.BuildProcurementBatchFromBillingSnapshotItem(updatedSnapshot, nextItem)
		if !ok {
			return nil, fmt.Errorf("该采购记录已经被消耗，不能移除可消耗权益项")
		}
		batches, err := model.ListChannelProcurementBatchesBySourceSnapshotIDWithDB(tx, current.Id)
		if err != nil {
			return nil, err
		}
		for _, batch := range batches {
			if strings.TrimSpace(batch.SourceSnapshotItemId) != strings.TrimSpace(currentItem.Id) {
				continue
			}
			consumed, err := sumProcurementConsumedQuantityWithDB(tx, batch.Id)
			if err != nil {
				return nil, err
			}
			capacityRemaining := batchTemplate.CapacityEffective - consumed
			if capacityRemaining < 0 {
				capacityRemaining = 0
			}
			costStatus := batch.CostStatus
			if costStatus != model.ProcurementCostStatusDisabled && costStatus != model.ProcurementCostStatusCostUnconfigured {
				if capacityRemaining <= 0 {
					costStatus = model.ProcurementCostStatusExhausted
				} else {
					costStatus = model.ProcurementCostStatusActive
				}
			}
			costPerUnitAmount := batch.CostPerUnitAmount
			if batch.PurchaseCostAmount > 0 && batchTemplate.CapacityEffective > 0 {
				costPerUnitAmount = batch.PurchaseCostAmount / batchTemplate.CapacityEffective
			}
			windowRemaining := batchTemplate.CapacityTotal - consumed
			if windowRemaining < 0 {
				windowRemaining = 0
			}
			updates := map[string]any{
				"resource_type":        batchTemplate.ResourceType,
				"quota_type":           batchTemplate.QuotaType,
				"capacity_total":       batchTemplate.CapacityTotal,
				"capacity_effective":   batchTemplate.CapacityEffective,
				"capacity_remaining":   capacityRemaining,
				"cost_per_unit_amount": costPerUnitAmount,
				"valid_from":           batchTemplate.ValidFrom,
				"expire_at":            batchTemplate.ExpireAt,
				"reset_cycle":          batchTemplate.ResetCycle,
				"window_started_at":    batchTemplate.WindowStartedAt,
				"window_remaining":     windowRemaining,
				"source_ref":           batchTemplate.SourceRef,
				"metadata":             batchTemplate.Metadata,
				"cost_status":          costStatus,
				"updated_at":           now,
			}
			if batchTemplate.ResetCycle == "none" || batchTemplate.ResetCycle == "" {
				updates["window_started_at"] = int64(0)
				updates["window_remaining"] = float64(0)
			}
			if err := tx.Model(&model.ChannelProcurementBatch{}).Where("id = ?", batch.Id).Updates(updates).Error; err != nil {
				return nil, err
			}
		}
		updatedItems = append(updatedItems, nextItem)
	}
	return model.NormalizeChannelBillingSnapshotItems(updatedItems), nil
}

type normalizedManualBillingSnapshotRequest struct {
	PurchaseAt         int64
	PurchaseCurrency   string
	PurchaseAmount     float64
	PurchaseFXRate     float64
	PurchaseCostAmount float64
	EntitlementName    string
	ValidFrom          int64
	ValidUntil         int64
	Items              []model.ChannelBillingSnapshotItem
	Message            string
}

func normalizeManualBillingSnapshotRequest(req channelBillingManualSnapshotRequest, now int64) (normalizedManualBillingSnapshotRequest, error) {
	purchaseCurrency := strings.TrimSpace(strings.ToUpper(req.PurchaseCurrency))
	purchaseAmount := req.PurchaseAmount
	purchaseFXRate := req.PurchaseFXRate
	purchaseCostAmount := req.PurchaseCostAmount
	if purchaseCurrency == "" {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("采购币种不能为空")
	}
	if purchaseAmount <= 0 {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("实付金额必须大于 0")
	}
	if purchaseFXRate < 0 || purchaseCostAmount < 0 {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("采购汇率和采购成本不能小于 0")
	}
	if purchaseCurrency == "CNY" {
		if purchaseFXRate <= 0 {
			purchaseFXRate = 1
		}
		if purchaseCostAmount <= 0 {
			purchaseCostAmount = purchaseAmount
		}
	} else if purchaseCostAmount <= 0 && purchaseFXRate > 0 {
		purchaseCostAmount = purchaseAmount * purchaseFXRate
	}
	if purchaseCostAmount <= 0 {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("采购成本 CNY 必须大于 0")
	}
	purchaseAt := req.PurchaseAt
	if purchaseAt <= 0 {
		purchaseAt = now
	}
	validFrom := req.ValidFrom
	validUntil := req.ValidUntil
	if validFrom < 0 || validUntil < 0 {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("有效期无效")
	}
	if validUntil > 0 && validFrom > 0 && validUntil <= validFrom {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("有效期结束时间必须晚于开始时间")
	}
	entitlementName := strings.TrimSpace(req.EntitlementName)
	if entitlementName == "" {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("权益名称不能为空")
	}
	quotaItems := make([]model.ChannelBillingSnapshotItem, 0, len(req.Items))
	for index, item := range req.Items {
		resourceType := strings.TrimSpace(strings.ToLower(item.ResourceType))
		if !isSupportedBillingResourceType(resourceType) {
			return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("第 %d 条权益类型无效", index+1)
		}
		quotaType := strings.TrimSpace(strings.ToLower(item.QuotaType))
		itemExpiresAt := item.ExpiresAt
		if itemExpiresAt <= 0 {
			itemExpiresAt = validUntil
		}
		if resourceType == model.ChannelBillingResourceTypePlan {
			quotaType = "plan"
		}
		if resourceType == model.ChannelBillingResourceTypeQuota {
			switch quotaType {
			case "daily", "weekly", "monthly", "total", "custom":
			default:
				return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("第 %d 条额度类型无效", index+1)
			}
		}
		quotaLabel := strings.TrimSpace(item.QuotaLabel)
		if quotaLabel == "" {
			quotaLabel = resourceType
			if quotaType != "" && quotaType != resourceType {
				quotaLabel += ":" + quotaType
			}
		}
		limitAmount := item.LimitAmount
		if limitAmount <= 0 {
			limitAmount = item.Amount
		}
		remainingAmount := item.RemainingAmount
		if remainingAmount <= 0 && item.UsedAmount == 0 {
			remainingAmount = item.Amount
		}
		amount := item.Amount
		if amount <= 0 {
			amount = remainingAmount
		}
		if amount <= 0 && limitAmount > 0 {
			amount = limitAmount
		}
		if item.Amount < 0 || item.LimitAmount < 0 || item.UsedAmount < 0 || item.RemainingAmount < 0 {
			return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("第 %d 条额度数值不能小于 0", index+1)
		}
		if resourceType != model.ChannelBillingResourceTypePlan && amount <= 0 && remainingAmount <= 0 && limitAmount <= 0 {
			return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("第 %d 条额度数值不能为空", index+1)
		}
		sourceRef := strings.TrimSpace(item.SourceRef)
		if sourceRef == "" {
			sourceRef = "manual"
		}
		quotaItems = append(quotaItems, model.ChannelBillingSnapshotItem{
			Id:              strings.TrimSpace(item.Id),
			ResourceType:    resourceType,
			QuotaType:       quotaType,
			QuotaLabel:      quotaLabel,
			Amount:          amount,
			LimitAmount:     limitAmount,
			UsedAmount:      item.UsedAmount,
			RemainingAmount: remainingAmount,
			Currency:        strings.TrimSpace(item.Currency),
			ResetAt:         item.ResetAt,
			ExpiresAt:       itemExpiresAt,
			SourceRef:       sourceRef,
			SortOrder:       index + 1,
		})
	}
	if len(quotaItems) == 0 {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("请至少填写一条权益项")
	}
	return normalizedManualBillingSnapshotRequest{
		PurchaseAt:         purchaseAt,
		PurchaseCurrency:   purchaseCurrency,
		PurchaseAmount:     purchaseAmount,
		PurchaseFXRate:     purchaseFXRate,
		PurchaseCostAmount: purchaseCostAmount,
		EntitlementName:    entitlementName,
		ValidFrom:          validFrom,
		ValidUntil:         validUntil,
		Items:              quotaItems,
		Message:            strings.TrimSpace(req.Message),
	}, nil
}

func CreateChannelBillingSnapshot(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	req := channelBillingManualSnapshotRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	channelRow, _, err := getEffectiveChannelBillingProfile(channelID)
	if err != nil {
		logChannelAdminWarn(c, "create_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	operatorUserID := strings.TrimSpace(c.GetString(ctxkey.Id))
	now := helper.GetTimestamp()
	normalizedReq, err := normalizeManualBillingSnapshotRequest(req, now)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		snapshotRow, err := model.CreateChannelBillingSnapshotWithDB(tx, model.ChannelBillingSnapshot{
			ChannelId:          channelRow.Id,
			SourceType:         model.ChannelBillingSnapshotSourceManual,
			PurchaseAt:         normalizedReq.PurchaseAt,
			PurchaseCurrency:   normalizedReq.PurchaseCurrency,
			PurchaseAmount:     normalizedReq.PurchaseAmount,
			PurchaseFXRate:     normalizedReq.PurchaseFXRate,
			PurchaseCostAmount: normalizedReq.PurchaseCostAmount,
			EntitlementName:    normalizedReq.EntitlementName,
			ValidFrom:          normalizedReq.ValidFrom,
			ValidUntil:         normalizedReq.ValidUntil,
			RawStatus:          "manual",
			Message:            normalizedReq.Message,
			OperatorUserId:     operatorUserID,
			CreatedAt:          now,
		})
		if err != nil {
			return err
		}
		createdItems, err := model.CreateChannelBillingSnapshotItemsWithDB(tx, snapshotRow.Id, channelRow.Id, normalizedReq.Items)
		if err != nil {
			return err
		}
		if err := attachManualPurchaseCostToSnapshotBatches(tx, snapshotRow.Id, normalizedReq.PurchaseCurrency, normalizedReq.PurchaseAmount, normalizedReq.PurchaseFXRate, normalizedReq.PurchaseCostAmount); err != nil {
			return err
		}
		_, err = model.CreateChannelBillingActionWithDB(tx, model.ChannelBillingAction{
			ChannelId:      channelRow.Id,
			ActionType:     model.ChannelBillingActionTypeManualUpdateSnapshot,
			Status:         model.ChannelBillingActionStatusDone,
			RequestPayload: marshalLogJSON(req),
			ResultPayload:  marshalLogJSON(map[string]any{"items": createdItems}),
			Message:        normalizedReq.Message,
			OperatorUserId: operatorUserID,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
		return err
	})
	if err != nil {
		logChannelAdminWarn(c, "create_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "create_billing_snapshot", stringField("channel_id", channelID))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"channel_id": channelID}})
}

func UpdateChannelBillingSnapshot(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	snapshotID := strings.TrimSpace(c.Param("snapshot_id"))
	if channelID == "" || snapshotID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "参数无效"})
		return
	}
	req := channelBillingManualSnapshotRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	channelRow, _, err := getEffectiveChannelBillingProfile(channelID)
	if err != nil {
		logChannelAdminWarn(c, "update_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	operatorUserID := strings.TrimSpace(c.GetString(ctxkey.Id))
	now := helper.GetTimestamp()
	normalizedReq, err := normalizeManualBillingSnapshotRequest(req, now)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		current, err := model.GetChannelBillingSnapshotByIDWithDB(tx, snapshotID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(current.ChannelId) != channelRow.Id {
			return gorm.ErrRecordNotFound
		}
		if strings.TrimSpace(current.SourceType) != model.ChannelBillingSnapshotSourceManual {
			return fmt.Errorf("只能修改人工采购记录")
		}
		count, err := model.CountRequestProcurementConsumptionsBySourceSnapshotIDWithDB(tx, snapshotID)
		if err != nil {
			return err
		}
		if count > 0 {
			updatedItems, err := updateConsumedManualBillingSnapshotWithDB(tx, current, channelRow.Id, normalizedReq, operatorUserID)
			if err != nil {
				return err
			}
			_, err = model.CreateChannelBillingActionWithDB(tx, model.ChannelBillingAction{
				ChannelId:      channelRow.Id,
				ActionType:     model.ChannelBillingActionTypeManualUpdateSnapshot,
				Status:         model.ChannelBillingActionStatusDone,
				RequestPayload: marshalLogJSON(req),
				ResultPayload:  marshalLogJSON(map[string]any{"items": updatedItems, "consumed_metadata_update": true}),
				Message:        normalizedReq.Message,
				OperatorUserId: operatorUserID,
				CreatedAt:      now,
				UpdatedAt:      now,
			})
			return err
		}
		updatedSnapshot, err := model.UpdateChannelBillingSnapshotPurchaseWithDB(tx, model.ChannelBillingSnapshot{
			Id:                 current.Id,
			ChannelId:          channelRow.Id,
			PurchaseAt:         normalizedReq.PurchaseAt,
			PurchaseCurrency:   normalizedReq.PurchaseCurrency,
			PurchaseAmount:     normalizedReq.PurchaseAmount,
			PurchaseFXRate:     normalizedReq.PurchaseFXRate,
			PurchaseCostAmount: normalizedReq.PurchaseCostAmount,
			EntitlementName:    normalizedReq.EntitlementName,
			ValidFrom:          normalizedReq.ValidFrom,
			ValidUntil:         normalizedReq.ValidUntil,
			Message:            normalizedReq.Message,
			OperatorUserId:     operatorUserID,
		})
		if err != nil {
			return err
		}
		createdItems, err := model.ReplaceChannelBillingSnapshotItemsWithDB(tx, updatedSnapshot.Id, channelRow.Id, normalizedReq.Items)
		if err != nil {
			return err
		}
		if err := attachManualPurchaseCostToSnapshotBatches(tx, updatedSnapshot.Id, normalizedReq.PurchaseCurrency, normalizedReq.PurchaseAmount, normalizedReq.PurchaseFXRate, normalizedReq.PurchaseCostAmount); err != nil {
			return err
		}
		_, err = model.CreateChannelBillingActionWithDB(tx, model.ChannelBillingAction{
			ChannelId:      channelRow.Id,
			ActionType:     model.ChannelBillingActionTypeManualUpdateSnapshot,
			Status:         model.ChannelBillingActionStatusDone,
			RequestPayload: marshalLogJSON(req),
			ResultPayload:  marshalLogJSON(map[string]any{"items": createdItems}),
			Message:        normalizedReq.Message,
			OperatorUserId: operatorUserID,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
		return err
	})
	if err != nil {
		logChannelAdminWarn(c, "update_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "update_billing_snapshot", stringField("channel_id", channelID), stringField("snapshot_id", snapshotID))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"channel_id": channelID, "snapshot_id": snapshotID}})
}

func DeleteChannelBillingSnapshot(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	snapshotID := strings.TrimSpace(c.Param("snapshot_id"))
	if channelID == "" || snapshotID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "参数无效"})
		return
	}
	channelRow, _, err := getEffectiveChannelBillingProfile(channelID)
	if err != nil {
		logChannelAdminWarn(c, "delete_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	current, err := model.GetChannelBillingSnapshotByIDWithDB(model.DB, snapshotID)
	if err != nil {
		logChannelAdminWarn(c, "delete_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if strings.TrimSpace(current.ChannelId) != channelRow.Id {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购记录不存在"})
		return
	}
	if strings.TrimSpace(current.SourceType) != model.ChannelBillingSnapshotSourceManual {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "只能删除人工采购记录"})
		return
	}
	count, err := model.CountRequestProcurementConsumptionsBySourceSnapshotIDWithDB(model.DB, snapshotID)
	if err != nil {
		logChannelAdminWarn(c, "delete_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if count > 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "该采购记录已经被消耗，不能删除"})
		return
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		return model.DeleteChannelBillingSnapshotPurchaseWithDB(tx, snapshotID, channelRow.Id)
	}); err != nil {
		logChannelAdminWarn(c, "delete_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "delete_billing_snapshot", stringField("channel_id", channelID), stringField("snapshot_id", snapshotID))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"channel_id": channelID, "snapshot_id": snapshotID}})
}
