package channel

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/monitor"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
)

type channelModelListData struct {
	Items         []channelModelListItem `json:"items"`
	Total         int64                  `json:"total"`
	Page          int                    `json:"page"`
	PageSize      int                    `json:"page_size"`
	SelectedCount int                    `json:"selected_count"`
	ActiveCount   int                    `json:"active_count"`
	InactiveCount int                    `json:"inactive_count"`
}

type channelTestListData struct {
	Items        []model.ChannelTest `json:"items"`
	LastTestedAt int64               `json:"last_tested_at"`
}

type channelModelListItem struct {
	model.ChannelModel
	SyncStatus        string `json:"sync_status"`
	LastSyncedAt      int64  `json:"last_synced_at"`
	EnableBlockReason string `json:"enable_block_reason,omitempty"`
}

type updateChannelModelsRequest struct {
	ChannelModels []model.ChannelModel `json:"channel_models"`
}

const (
	defaultChannelModelPageSize = 10
	maxChannelModelPageSize     = 100
)

func parseChannelModelPageParams(c *gin.Context) (page int, pageSize int, keyword string) {
	page = 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	pageSize = defaultChannelModelPageSize
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	if pageSize > maxChannelModelPageSize {
		pageSize = maxChannelModelPageSize
	}
	keyword = strings.TrimSpace(c.Query("keyword"))
	return page, pageSize, keyword
}

func buildChannelModelListData(channelID string, page int, pageSize int, keyword string) (channelModelListData, error) {
	rows, total, err := model.ListChannelModelRowsPageWithDB(model.DB, channelID, page-1, pageSize, keyword)
	if err != nil {
		return channelModelListData{}, err
	}
	channelRow, err := channelsvc.GetByID(channelID)
	if err != nil {
		return channelModelListData{}, err
	}
	syncRows, err := model.ListChannelModelSyncResultsByChannelIDWithDB(model.DB, channelID)
	if err != nil {
		return channelModelListData{}, err
	}
	syncByModel := make(map[string]model.ChannelModelSyncResult, len(syncRows)*2)
	for _, row := range syncRows {
		if modelName := strings.TrimSpace(row.Model); modelName != "" {
			syncByModel[modelName] = row
		}
		if upstreamModel := strings.TrimSpace(row.UpstreamModel); upstreamModel != "" {
			if _, ok := syncByModel[upstreamModel]; !ok {
				syncByModel[upstreamModel] = row
			}
		}
	}
	items := make([]channelModelListItem, 0, len(rows))
	for _, row := range rows {
		item := channelModelListItem{
			ChannelModel: row,
			SyncStatus:   "unknown",
		}
		if syncRow, ok := syncByModel[strings.TrimSpace(row.Model)]; ok {
			item.LastSyncedAt = syncRow.LastSyncedAt
			if syncRow.Returned {
				item.SyncStatus = "returned"
			} else {
				item.SyncStatus = "not_returned"
			}
		} else if syncRow, ok := syncByModel[strings.TrimSpace(row.UpstreamModel)]; ok {
			item.LastSyncedAt = syncRow.LastSyncedAt
			if syncRow.Returned {
				item.SyncStatus = "returned"
			} else {
				item.SyncStatus = "not_returned"
			}
		}
		if !row.Inactive && !row.Selected {
			reason, reasonErr := model.ExplainManualChannelModelEnableBlockWithDB(model.DB, channelID, row)
			if reasonErr != nil {
				return channelModelListData{}, reasonErr
			}
			item.EnableBlockReason = strings.TrimSpace(reason)
		}
		items = append(items, item)
	}
	allRows := channelRow.GetChannelModels()
	selectedCount := 0
	activeCount := 0
	inactiveCount := 0
	for _, row := range allRows {
		if row.Inactive {
			inactiveCount++
			continue
		}
		activeCount++
		if row.Selected {
			selectedCount++
		}
	}
	return channelModelListData{
		Items:         items,
		Total:         total,
		Page:          page,
		PageSize:      pageSize,
		SelectedCount: selectedCount,
		ActiveCount:   activeCount,
		InactiveCount: inactiveCount,
	}, nil
}

func buildChannelTestListData(channelID string) (channelTestListData, error) {
	rows, err := model.ListChannelTestsByChannelIDWithDB(model.DB, channelID)
	if err != nil {
		return channelTestListData{}, err
	}
	return channelTestListData{
		Items:        rows,
		LastTestedAt: model.CalcChannelTestsLastTestedAt(rows),
	}, nil
}

func GetChannelModels(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	page, pageSize, keyword := parseChannelModelPageParams(c)
	data, err := buildChannelModelListData(channelID, page, pageSize, keyword)
	if err != nil {
		logChannelAdminWarn(c, "list_models", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

type channelRecentDisabledModelFeedItem struct {
	model.ChannelModel
	ChannelName string `json:"channel_name"`
}

type channelRecentDisabledModelFeedData struct {
	Items []channelRecentDisabledModelFeedItem `json:"items"`
	Total int                                  `json:"total"`
}

func GetRecentDisabledChannelModels(c *gin.Context) {
	rows, err := model.ListRecentDisabledChannelModelsWithDB(model.DB, 20)
	if err != nil {
		logChannelAdminWarn(c, "list_recent_disabled_models", stringField("reason", err.Error()))
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
			logChannelAdminWarn(c, "list_recent_disabled_models", stringField("reason", err.Error()))
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		for _, channelRow := range channels {
			channelNameByID[strings.TrimSpace(channelRow.Id)] = strings.TrimSpace(channelRow.DisplayName())
		}
	}
	items := make([]channelRecentDisabledModelFeedItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, channelRecentDisabledModelFeedItem{
			ChannelModel: row,
			ChannelName:  channelNameByID[strings.TrimSpace(row.ChannelId)],
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": channelRecentDisabledModelFeedData{
			Items: items,
			Total: len(items),
		},
	})
}

func UpdateChannelModels(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	req := updateChannelModelsRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		logChannelAdminWarn(c, "update_models", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	channelRow, err := channelsvc.GetByID(channelID)
	if err != nil {
		logChannelAdminWarn(c, "update_models", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	nextChannelRow := *channelRow
	nextChannelRow.SetChannelModels(req.ChannelModels)
	restoredModels := collectRestoredChannelModelCapabilities(channelRow.GetChannelModels(), nextChannelRow.GetChannelModels())
	if err := channelsvc.UpdateModels(channelID, req.ChannelModels); err != nil {
		logChannelAdminWarn(c, "update_models", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "update_models", stringField("channel_id", channelID), intField("model_count", len(model.NormalizeChannelModelsPreserveOrder(req.ChannelModels))))
	operator := channelAdminOperator(c)
	for _, modelName := range restoredModels {
		monitor.NotifyChannelModelCapabilityRestored(channelID, channelRow.DisplayName(), modelName, operator)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"channel_id": channelID,
		},
	})
}

func collectRestoredChannelModelCapabilities(currentRows []model.ChannelModel, nextRows []model.ChannelModel) []string {
	currentByModel := make(map[string]model.ChannelModel)
	for _, row := range model.NormalizeChannelModelsPreserveOrder(currentRows) {
		modelName := strings.TrimSpace(row.Model)
		if modelName == "" {
			continue
		}
		currentByModel[modelName] = row
	}
	restored := make([]string, 0)
	for _, row := range model.NormalizeChannelModelsPreserveOrder(nextRows) {
		modelName := strings.TrimSpace(row.Model)
		if modelName == "" {
			continue
		}
		current, ok := currentByModel[modelName]
		if !ok || !isChannelModelRuntimeDisabled(current) {
			continue
		}
		if row.Inactive {
			continue
		}
		restored = append(restored, modelName)
	}
	return restored
}

func isChannelModelRuntimeDisabled(row model.ChannelModel) bool {
	return row.Inactive &&
		(row.DisabledAt > 0 ||
			strings.TrimSpace(row.DisabledReason) != "" ||
			strings.TrimSpace(row.DisabledBy) != "")
}

func channelAdminOperator(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if username := strings.TrimSpace(c.GetString(ctxkey.Username)); username != "" {
		return username
	}
	return strings.TrimSpace(c.GetString(ctxkey.Id))
}

func GetChannelTests(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	data, err := buildChannelTestListData(channelID)
	if err != nil {
		logChannelAdminWarn(c, "list_tests", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

type channelRecentFailedTestFeedItem struct {
	model.ChannelTest
	ChannelName string `json:"channel_name"`
}

type channelRecentFailedTestFeedData struct {
	Items []channelRecentFailedTestFeedItem `json:"items"`
	Total int                               `json:"total"`
}

func GetRecentFailedChannelTests(c *gin.Context) {
	rows, err := model.ListRecentFailedChannelTestsWithDB(model.DB, 20)
	if err != nil {
		logChannelAdminWarn(c, "list_recent_failed_tests", stringField("reason", err.Error()))
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
			logChannelAdminWarn(c, "list_recent_failed_tests", stringField("reason", err.Error()))
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		for _, channelRow := range channels {
			channelNameByID[strings.TrimSpace(channelRow.Id)] = strings.TrimSpace(channelRow.DisplayName())
		}
	}
	items := make([]channelRecentFailedTestFeedItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, channelRecentFailedTestFeedItem{
			ChannelTest: row,
			ChannelName: channelNameByID[strings.TrimSpace(row.ChannelId)],
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": channelRecentFailedTestFeedData{
			Items: items,
			Total: len(items),
		},
	})
}
