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
	BillingPortalURL      string                             `json:"billing_portal_url,omitempty"`
	ActivateSupported     bool                               `json:"activate_supported"`
	ManualUpdateSupported bool                               `json:"manual_update_supported"`
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
	BillingPortalURL   string   `json:"billing_portal_url,omitempty"`
}

type channelBillingListData[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

type channelBillingAlertFeedItem struct {
	model.ChannelBillingAlertEvent
	ChannelName string `json:"channel_name"`
}

type channelBillingOpenActivateRequest struct {
	CDK string `json:"cdk"`
}

type channelBillingManualSnapshotRequest struct {
	PurchaseAt         int64                                  `json:"purchase_at"`
	PurchaseCurrency   string                                 `json:"purchase_currency"`
	PurchaseAmount     float64                                `json:"purchase_amount"`
	PurchaseFXRate     float64                                `json:"purchase_fx_rate"`
	PurchaseCostAmount float64                                `json:"purchase_cost_amount"`
	Items              []channelBillingManualQuotaItemRequest `json:"items"`
	Message            string                                 `json:"message"`
}

type channelBillingManualQuotaItemRequest struct {
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

func extractBillingPortalURL(profile model.ChannelBillingProfile) string {
	actionConfig := profile.ParseActionConfig()
	if value, ok := actionConfig["billing_portal"]; ok {
		if portal, ok := value.(map[string]any); ok {
			return strings.TrimSpace(fmt.Sprintf("%v", portal["url"]))
		}
	}
	return ""
}

func buildChannelBillingSummary(channelRow *model.Channel, profile model.ChannelBillingProfile, snapshot model.ChannelBillingSnapshot) channelBillingSummaryData {
	capabilities := profile.ParseActionCapabilities()
	summary := channelBillingSummaryData{
		ChannelID:             strings.TrimSpace(channelRow.Id),
		ProfileEnabled:        profile.Enabled,
		BillingMode:           strings.TrimSpace(profile.BillingMode),
		ActionCapabilities:    capabilities,
		BillingPortalURL:      extractBillingPortalURL(profile),
		ActivateSupported:     profile.HasCapability(model.ChannelBillingCapabilityOpenActivatePage),
		ManualUpdateSupported: profile.HasCapability(model.ChannelBillingCapabilityManualUpdateSnapshot),
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
		BillingPortalURL:   extractBillingPortalURL(profile),
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
	totalEffectiveCapacity := 0.0
	for _, row := range rows {
		if row.CapacityEffective > 0 {
			totalEffectiveCapacity += row.CapacityEffective
		}
	}
	if len(rows) == 0 || totalEffectiveCapacity <= 0 {
		return nil
	}
	now := helper.GetTimestamp()
	for _, row := range rows {
		if row.CapacityEffective <= 0 {
			continue
		}
		ratio := row.CapacityEffective / totalEffectiveCapacity
		allocatedCostAmount := purchaseCostAmount * ratio
		if allocatedCostAmount <= 0 {
			continue
		}
		costPerUnitAmount := allocatedCostAmount / row.CapacityEffective
		if err := tx.Model(&model.ChannelProcurementBatch{}).
			Where("id = ?", row.Id).
			Updates(map[string]any{
				"purchase_currency":    strings.TrimSpace(strings.ToUpper(purchaseCurrency)),
				"purchase_amount":      purchaseAmount * ratio,
				"purchase_fx_rate":     purchaseFXRate,
				"purchase_cost_amount": allocatedCostAmount,
				"cost_per_unit_amount": costPerUnitAmount,
				"cost_source":          model.ProcurementCostSourceActual,
				"cost_status":          model.ProcurementCostStatusActive,
				"updated_at":           now,
			}).Error; err != nil {
			return err
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

func OpenChannelBillingActivatePage(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	req := channelBillingOpenActivateRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	channelRow, profile, err := getEffectiveChannelBillingProfile(channelID)
	if err != nil {
		logChannelAdminWarn(c, "open_billing_activate_page", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if !profile.HasCapability(model.ChannelBillingCapabilityOpenActivatePage) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "当前渠道不支持激活页操作"})
		return
	}
	openURL, err := model.RenderChannelBillingActivateOpenURL(profile, req.CDK)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	requestPayload := map[string]any{
		"cdk_masked": maskBillingSecret(req.CDK),
	}
	resultPayload := map[string]any{
		"open_url_masked": maskBillingSecret(openURL),
	}
	actionRow, err := model.CreateChannelBillingActionWithDB(model.DB, model.ChannelBillingAction{
		ChannelId:      channelRow.Id,
		ActionType:     model.ChannelBillingActionTypeOpenActivatePage,
		Status:         model.ChannelBillingActionStatusDone,
		RequestPayload: marshalLogJSON(requestPayload),
		ResultPayload:  marshalLogJSON(resultPayload),
		Message:        "已生成激活页链接",
		OperatorUserId: strings.TrimSpace(c.GetString(ctxkey.Id)),
	})
	if err != nil {
		logChannelAdminWarn(c, "open_billing_activate_page", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "open_billing_activate_page", stringField("channel_id", channelID), stringField("action_id", actionRow.Id))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"action_id": actionRow.Id,
			"open_url":  openURL,
		},
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

type normalizedManualBillingSnapshotRequest struct {
	PurchaseAt         int64
	PurchaseCurrency   string
	PurchaseAmount     float64
	PurchaseFXRate     float64
	PurchaseCostAmount float64
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
	quotaItems := make([]model.ChannelBillingSnapshotItem, 0, len(req.Items))
	for index, item := range req.Items {
		resourceType := strings.TrimSpace(strings.ToLower(item.ResourceType))
		if !isSupportedBillingResourceType(resourceType) {
			return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("第 %d 条权益类型无效", index+1)
		}
		quotaLabel := strings.TrimSpace(item.QuotaLabel)
		if quotaLabel == "" {
			return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("第 %d 条额度名称不能为空", index+1)
		}
		quotaType := strings.TrimSpace(strings.ToLower(item.QuotaType))
		if resourceType == model.ChannelBillingResourceTypePlan {
			quotaType = "plan"
			if item.ExpiresAt <= 0 {
				return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("第 %d 条套餐必须填写截止时间", index+1)
			}
		}
		if resourceType == model.ChannelBillingResourceTypeQuota {
			switch quotaType {
			case "daily", "weekly", "monthly", "total", "custom":
			default:
				return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("第 %d 条额度类型无效", index+1)
			}
			if (quotaType == "daily" || quotaType == "weekly" || quotaType == "monthly") && item.ExpiresAt <= 0 {
				return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("第 %d 条套餐周期额度必须填写截止时间", index+1)
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
			ResourceType:    resourceType,
			QuotaType:       quotaType,
			QuotaLabel:      quotaLabel,
			Amount:          amount,
			LimitAmount:     limitAmount,
			UsedAmount:      item.UsedAmount,
			RemainingAmount: remainingAmount,
			Currency:        strings.TrimSpace(item.Currency),
			ResetAt:         item.ResetAt,
			ExpiresAt:       item.ExpiresAt,
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
	channelRow, profile, err := getEffectiveChannelBillingProfile(channelID)
	if err != nil {
		logChannelAdminWarn(c, "create_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if !profile.HasCapability(model.ChannelBillingCapabilityManualUpdateSnapshot) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "当前渠道不支持人工更新余额"})
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
	channelRow, profile, err := getEffectiveChannelBillingProfile(channelID)
	if err != nil {
		logChannelAdminWarn(c, "update_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if !profile.HasCapability(model.ChannelBillingCapabilityManualUpdateSnapshot) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "当前渠道不支持人工更新余额"})
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
			return fmt.Errorf("该采购记录已经被消耗，不能修改")
		}
		updatedSnapshot, err := model.UpdateChannelBillingSnapshotPurchaseWithDB(tx, model.ChannelBillingSnapshot{
			Id:                 current.Id,
			ChannelId:          channelRow.Id,
			PurchaseAt:         normalizedReq.PurchaseAt,
			PurchaseCurrency:   normalizedReq.PurchaseCurrency,
			PurchaseAmount:     normalizedReq.PurchaseAmount,
			PurchaseFXRate:     normalizedReq.PurchaseFXRate,
			PurchaseCostAmount: normalizedReq.PurchaseCostAmount,
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
	channelRow, profile, err := getEffectiveChannelBillingProfile(channelID)
	if err != nil {
		logChannelAdminWarn(c, "delete_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if !profile.HasCapability(model.ChannelBillingCapabilityManualUpdateSnapshot) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "当前渠道不支持人工更新余额"})
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
