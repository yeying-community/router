package channel

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/presenter"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
)

const maxChannelListPageSize = 100

type channelListItem struct {
	ID                    string                         `json:"id"`
	Protocol              string                         `json:"protocol"`
	Status                int                            `json:"status"`
	Name                  string                         `json:"name"`
	Weight                *uint                          `json:"weight,omitempty"`
	CreatedTime           int64                          `json:"created_time"`
	UpdatedAt             int64                          `json:"updated_at"`
	TestTime              int64                          `json:"test_time"`
	Capabilities          []string                       `json:"capabilities"`
	BaseURL               string                         `json:"base_url,omitempty"`
	Other                 string                         `json:"other,omitempty"`
	BillingSummary        string                         `json:"billing_summary"`
	BillingSnapshotAt     int64                          `json:"billing_snapshot_at"`
	BillingQuotaItemCount int                            `json:"billing_quota_item_count"`
	UsedQuota             int64                          `json:"used_quota"`
	YYCUsed               int64                          `json:"yyc_used"`
	Priority              int64                          `json:"priority"`
	CircuitBreaker        *channelCircuitBreakerListItem `json:"circuit_breaker,omitempty"`
}

type channelCircuitBreakerListItem struct {
	State        string  `json:"state"`
	Reason       string  `json:"reason"`
	SuccessRate  float64 `json:"success_rate"`
	DisabledAt   int64   `json:"disabled_at"`
	RecoverAfter int64   `json:"recover_after"`
	RecoveredAt  int64   `json:"recovered_at"`
	UpdatedAt    int64   `json:"updated_at"`
}

type channelCircuitBreakerEventsData struct {
	Items []model.ChannelCircuitBreakerEvent `json:"items"`
}

type channelCircuitBreakerFeedItem struct {
	model.ChannelCircuitBreakerEvent
	ChannelName string `json:"channel_name"`
}

type channelCircuitBreakerFeedData struct {
	Items []channelCircuitBreakerFeedItem `json:"items"`
	Total int                             `json:"total"`
}

type channelListPageData struct {
	Items    []channelListItem `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

type channelListCompactItem struct {
	ID       string `json:"id"`
	Protocol string `json:"protocol"`
	Status   int    `json:"status"`
	Name     string `json:"name"`
}

type channelListCompactPageData struct {
	Items    []channelListCompactItem `json:"items"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

func sanitizeChannelForResponse(channel *model.Channel) {
	if channel == nil {
		return
	}
	channel.NormalizeProtocol()
	channel.Id = strings.TrimSpace(channel.Id)
	channel.TestModel = strings.TrimSpace(channel.TestModel)
	channel.Models = strings.TrimSpace(channel.Models)
	channel.AvailableModels = model.NormalizeChannelModelIDsPreserveOrder(channel.AvailableModels)
	channel.ChannelModels = model.NormalizeChannelModelsPreserveOrder(channel.ChannelModels)
	channel.SetChannelTests(channel.Tests)
	channel.KeySet = strings.TrimSpace(channel.Key) != ""
	channel.Key = ""
}

func buildChannelListItem(channel *model.Channel) channelListItem {
	if channel == nil {
		return channelListItem{}
	}
	channel.NormalizeProtocol()
	capabilities := collectChannelCapabilities(channel)
	baseURL := ""
	if channel.BaseURL != nil {
		baseURL = strings.TrimSpace(*channel.BaseURL)
	}
	other := ""
	if channel.Other != nil {
		other = strings.TrimSpace(*channel.Other)
	}
	return channelListItem{
		ID:           strings.TrimSpace(channel.Id),
		Protocol:     strings.TrimSpace(channel.Protocol),
		Status:       channel.Status,
		Name:         strings.TrimSpace(channel.Name),
		Weight:       channel.Weight,
		CreatedTime:  channel.CreatedTime,
		UpdatedAt:    channel.UpdatedAt,
		TestTime:     channel.TestTime,
		Capabilities: capabilities,
		BaseURL:      baseURL,
		Other:        other,
		UsedQuota:    channel.UsedQuota,
		YYCUsed:      channel.UsedQuota,
		Priority:     channel.GetPriority(),
	}
}

func buildChannelCircuitBreakerListItem(row model.ChannelCircuitBreakerState) *channelCircuitBreakerListItem {
	if strings.TrimSpace(row.ChannelId) == "" {
		return nil
	}
	return &channelCircuitBreakerListItem{
		State:        strings.TrimSpace(row.State),
		Reason:       strings.TrimSpace(row.Reason),
		SuccessRate:  row.SuccessRate,
		DisabledAt:   row.DisabledAt,
		RecoverAfter: row.RecoverAfter,
		RecoveredAt:  row.RecoveredAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

func summarizeChannelBillingSnapshot(snapshot model.ChannelBillingSnapshot) (string, int64, int) {
	items := model.NormalizeChannelBillingSnapshotItems(snapshot.Items)
	if len(items) == 0 {
		return "-", snapshot.CreatedAt, 0
	}
	parts := make([]string, 0, len(items))
	for index, item := range items {
		if index >= 2 {
			parts = append(parts, fmt.Sprintf("+%d", len(items)-index))
			break
		}
		label := strings.TrimSpace(item.QuotaLabel)
		if label == "" {
			label = strings.TrimSpace(item.QuotaType)
		}
		if label == "" {
			label = "quota"
		}
		amountText := strconv.FormatFloat(item.Amount, 'f', -1, 64)
		currency := strings.TrimSpace(item.Currency)
		if currency != "" {
			parts = append(parts, fmt.Sprintf("%s %s %s", label, amountText, currency))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s", label, amountText))
	}
	return strings.Join(parts, " / "), snapshot.CreatedAt, len(items)
}

func collectChannelCapabilities(channel *model.Channel) []string {
	if channel == nil {
		return []string{}
	}
	selectedTypes := map[string]struct{}{}
	for _, row := range channel.GetChannelModels() {
		if !row.Selected || row.Inactive {
			continue
		}
		modelType := strings.TrimSpace(strings.ToLower(row.Type))
		switch modelType {
		case "image", "audio", "video":
			selectedTypes[modelType] = struct{}{}
		default:
			selectedTypes["text"] = struct{}{}
		}
	}
	order := []string{"text", "image", "audio", "video"}
	result := make([]string, 0, len(order))
	for _, item := range order {
		if _, ok := selectedTypes[item]; ok {
			result = append(result, item)
		}
	}
	return result
}

func parseChannelListPageParams(c *gin.Context) (page int, pageSize int, keyword string) {
	page = 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	pageSize = config.ItemsPerPage
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	if pageSize > maxChannelListPageSize {
		pageSize = maxChannelListPageSize
	}
	keyword = strings.TrimSpace(c.Query("keyword"))
	return page, pageSize, keyword
}

func parseCompactMode(c *gin.Context) bool {
	raw := strings.TrimSpace(c.Query("compact"))
	return raw == "1" || strings.EqualFold(raw, "true")
}

func listChannelsPage(page int, pageSize int, keyword string) (channelListPageData, error) {
	rows, total, err := channelsvc.ListPage(page, pageSize, keyword)
	if err != nil {
		return channelListPageData{}, err
	}
	items := make([]channelListItem, 0, len(rows))
	channelIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		channelIDs = append(channelIDs, strings.TrimSpace(row.Id))
	}
	latestSnapshots, err := model.ListLatestChannelBillingSnapshotsByChannelIDsWithDB(model.DB, channelIDs)
	if err != nil {
		return channelListPageData{}, err
	}
	latestSnapshotMap := make(map[string]model.ChannelBillingSnapshot, len(latestSnapshots))
	for _, snapshot := range latestSnapshots {
		latestSnapshotMap[strings.TrimSpace(snapshot.ChannelId)] = snapshot
	}
	circuitRows, err := model.ListChannelCircuitBreakerStatesByChannelIDsWithDB(model.DB, channelIDs)
	if err != nil {
		return channelListPageData{}, err
	}
	circuitByChannelID := make(map[string]model.ChannelCircuitBreakerState, len(circuitRows))
	for _, row := range circuitRows {
		circuitByChannelID[strings.TrimSpace(row.ChannelId)] = row
	}
	for _, row := range rows {
		item := buildChannelListItem(row)
		if snapshot, ok := latestSnapshotMap[strings.TrimSpace(row.Id)]; ok {
			item.BillingSummary, item.BillingSnapshotAt, item.BillingQuotaItemCount = summarizeChannelBillingSnapshot(snapshot)
		} else {
			item.BillingSummary = "-"
		}
		if circuitRow, ok := circuitByChannelID[strings.TrimSpace(row.Id)]; ok {
			item.CircuitBreaker = buildChannelCircuitBreakerListItem(circuitRow)
		}
		items = append(items, item)
	}
	return channelListPageData{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func isModelInChannelModels(testModel string, models string) bool {
	normalized := strings.TrimSpace(testModel)
	if normalized == "" {
		return true
	}
	for _, item := range model.ParseChannelModelCSV(models) {
		if item == normalized {
			return true
		}
	}
	return false
}

func GetChannels(c *gin.Context) {
	page, pageSize, keyword := parseChannelListPageParams(c)
	data, err := listChannelsPage(page, pageSize, keyword)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if parseCompactMode(c) {
		compactItems := make([]channelListCompactItem, 0, len(data.Items))
		for _, item := range data.Items {
			compactItems = append(compactItems, channelListCompactItem{
				ID:       strings.TrimSpace(item.ID),
				Protocol: strings.TrimSpace(item.Protocol),
				Status:   item.Status,
				Name:     strings.TrimSpace(item.Name),
			})
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data": channelListCompactPageData{
				Items:    compactItems,
				Total:    data.Total,
				Page:     data.Page,
				PageSize: data.PageSize,
			},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

func GetChannelCircuitBreakerEvents(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	limit := 20
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	rows, err := model.ListChannelCircuitBreakerEventsWithDB(model.DB, channelID, limit)
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
		"data": channelCircuitBreakerEventsData{
			Items: rows,
		},
	})
}

func GetRecentChannelCircuitBreakerEvents(c *gin.Context) {
	rows, err := model.ListRecentChannelCircuitBreakerEventsWithDB(model.DB, 20)
	if err != nil {
		logChannelAdminWarn(c, "list_recent_circuit_breaker_events", stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
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
			logChannelAdminWarn(c, "list_recent_circuit_breaker_events", stringField("reason", err.Error()))
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		for _, channelRow := range channels {
			channelNameByID[strings.TrimSpace(channelRow.Id)] = strings.TrimSpace(channelRow.DisplayName())
		}
	}
	items := make([]channelCircuitBreakerFeedItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, channelCircuitBreakerFeedItem{
			ChannelCircuitBreakerEvent: row,
			ChannelName:                channelNameByID[strings.TrimSpace(row.ChannelId)],
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": channelCircuitBreakerFeedData{
			Items: items,
			Total: len(items),
		},
	})
}

func GetChannel(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "id 为空",
		})
		return
	}
	var err error
	channel, err := channelsvc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	sanitizeChannelForResponse(channel)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    presenter.NewChannel(channel),
	})
	return
}

func AddChannel(c *gin.Context) {
	channel := model.Channel{}
	err := c.ShouldBindJSON(&channel)
	if err != nil {
		logChannelAdminWarn(c, "create", stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	channel.NormalizeChannelModelState()
	channel.CreatedTime = helper.GetTimestamp()
	channel.UpdatedAt = channel.CreatedTime
	channel.NormalizeIdentity()
	err = channelsvc.Insert(&channel)
	if err != nil {
		logChannelAdminWarn(c, "create", stringField("channel_id", channel.Id), stringField("name", channel.DisplayName()), stringField("protocol", channel.GetProtocol()), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logChannelAdminInfo(c, "create", stringField("channel_id", channel.Id), stringField("name", channel.DisplayName()), stringField("protocol", channel.GetProtocol()))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"id": channel.Id,
		},
	})
	return
}

func DeleteChannel(c *gin.Context) {
	id := c.Param("id")
	channel := model.Channel{Id: id}
	err := channelsvc.DeleteByID(channel.Id)
	if err != nil {
		logChannelAdminWarn(c, "delete", stringField("channel_id", channel.Id), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logChannelAdminInfo(c, "delete", stringField("channel_id", channel.Id))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func DeleteDisabledChannel(c *gin.Context) {
	rows, err := channelsvc.DeleteDisabled()
	if err != nil {
		logChannelAdminWarn(c, "delete_disabled", stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logChannelAdminInfo(c, "delete_disabled", int64Field("rows", rows))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
	return
}

func UpdateChannel(c *gin.Context) {
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "读取请求体失败",
		})
		return
	}
	if len(rawBody) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请求体不能为空",
		})
		return
	}
	channel := model.Channel{}
	err = json.Unmarshal(rawBody, &channel)
	if err != nil {
		logChannelAdminWarn(c, "update", stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	rawFields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(rawBody, &rawFields); err != nil {
		logChannelAdminWarn(c, "update", stringField("channel_id", channel.Id), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	_, channel.NameProvided = rawFields["name"]
	_, channel.ModelsProvided = rawFields["models"]
	_, channel.ChannelModelsProvided = rawFields["channel_models"]
	channel.NormalizeChannelModelState()
	err = channelsvc.Update(&channel)
	if err != nil {
		var blockedErr *model.ChannelDisableBlockedError
		if errors.As(err, &blockedErr) {
			logChannelAdminWarn(c, "update", stringField("channel_id", channel.Id), stringField("name", channel.DisplayName()), stringField("protocol", channel.GetProtocol()), stringField("reason", blockedErr.Error()))
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": blockedErr.Error(),
				"data": gin.H{
					"code":   "channel_disable_blocked",
					"impact": blockedErr.Impact,
				},
			})
			return
		}
		logChannelAdminWarn(c, "update", stringField("channel_id", channel.Id), stringField("name", channel.DisplayName()), stringField("protocol", channel.GetProtocol()), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logChannelAdminInfo(c, "update", stringField("channel_id", channel.Id), stringField("name", channel.DisplayName()), stringField("protocol", channel.GetProtocol()), intField("model_count", len(model.ParseChannelModelCSV(channel.Models))))
	sanitizeChannelForResponse(&channel)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    presenter.NewChannel(&channel),
	})
	return
}
