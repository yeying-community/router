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

type channelBillingOpenActivateRequest struct {
	CDK string `json:"cdk"`
}

type channelBillingManualSnapshotRequest struct {
	Items   []channelBillingManualQuotaItemRequest `json:"items"`
	Message string                                 `json:"message"`
}

type channelBillingManualQuotaItemRequest struct {
	ResourceType string  `json:"resource_type"`
	QuotaType    string  `json:"quota_type"`
	QuotaLabel   string  `json:"quota_label"`
	Amount       float64 `json:"amount"`
	Currency     string  `json:"currency"`
	ExpiresAt    int64   `json:"expires_at"`
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
	quotaItems := make([]model.ChannelBillingSnapshotItem, 0, len(req.Items))
	for index, item := range req.Items {
		resourceType := strings.TrimSpace(strings.ToLower(item.ResourceType))
		if !isSupportedBillingResourceType(resourceType) {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("第 %d 条权益类型无效", index+1)})
			return
		}
		quotaLabel := strings.TrimSpace(item.QuotaLabel)
		if quotaLabel == "" {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("第 %d 条额度名称不能为空", index+1)})
			return
		}
		if item.Amount < 0 {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("第 %d 条额度金额不能小于 0", index+1)})
			return
		}
		quotaItems = append(quotaItems, model.ChannelBillingSnapshotItem{
			ResourceType:    resourceType,
			QuotaType:       strings.TrimSpace(item.QuotaType),
			QuotaLabel:      quotaLabel,
			Amount:          item.Amount,
			LimitAmount:     item.Amount,
			RemainingAmount: item.Amount,
			Currency:        strings.TrimSpace(item.Currency),
			ExpiresAt:       item.ExpiresAt,
			SourceRef:       "manual",
			SortOrder:       index + 1,
		})
	}
	if len(quotaItems) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请至少填写一条权益项"})
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
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		snapshotRow, err := model.CreateChannelBillingSnapshotWithDB(tx, model.ChannelBillingSnapshot{
			ChannelId:      channelRow.Id,
			SourceType:     model.ChannelBillingSnapshotSourceManual,
			RawStatus:      "manual",
			Message:        strings.TrimSpace(req.Message),
			OperatorUserId: operatorUserID,
			CreatedAt:      now,
		})
		if err != nil {
			return err
		}
		createdItems, err := model.CreateChannelBillingSnapshotItemsWithDB(tx, snapshotRow.Id, channelRow.Id, quotaItems)
		if err != nil {
			return err
		}
		_, err = model.CreateChannelBillingActionWithDB(tx, model.ChannelBillingAction{
			ChannelId:      channelRow.Id,
			ActionType:     model.ChannelBillingActionTypeManualUpdateSnapshot,
			Status:         model.ChannelBillingActionStatusDone,
			RequestPayload: marshalLogJSON(req),
			ResultPayload:  marshalLogJSON(map[string]any{"items": createdItems}),
			Message:        strings.TrimSpace(req.Message),
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
