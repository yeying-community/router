package channel

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/monitor"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
)

type updateChannelEndpointRequest struct {
	Model    string `json:"model"`
	Endpoint string `json:"endpoint"`
	BaseURL  string `json:"base_url"`
	Enabled  *bool  `json:"enabled"`
}

type channelEndpointItem struct {
	ChannelId         string `json:"channel_id"`
	Model             string `json:"model"`
	Endpoint          string `json:"endpoint"`
	BaseURL           string `json:"base_url,omitempty"`
	Enabled           bool   `json:"enabled"`
	UpdatedAt         int64  `json:"updated_at"`
	DisabledReason    string `json:"disabled_reason,omitempty"`
	DisabledAt        int64  `json:"disabled_at,omitempty"`
	DisabledBy        string `json:"disabled_by,omitempty"`
	LastTestStatus    string `json:"last_test_status,omitempty"`
	LastTestedAt      int64  `json:"last_tested_at,omitempty"`
	LastTestError     string `json:"last_test_error,omitempty"`
	EnableBlockReason string `json:"enable_block_reason,omitempty"`
}

type channelRecentDisabledEndpointFeedItem struct {
	model.ChannelModelEndpoint
	ChannelName string `json:"channel_name"`
}

type channelRecentDisabledEndpointFeedData struct {
	Items []channelRecentDisabledEndpointFeedItem `json:"items"`
	Total int                                     `json:"total"`
}

func GetChannelEndpoints(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "渠道 ID 无效",
		})
		return
	}
	modelName := strings.TrimSpace(c.Query("model"))
	endpoint := strings.TrimSpace(c.Query("endpoint"))
	explicitRows, err := model.ListChannelModelEndpointsByChannelIDWithDB(model.DB, channelID, modelName, endpoint)
	if err != nil {
		logChannelAdminWarn(c, "list_endpoints", stringField("channel_id", channelID), stringField("model", modelName), stringField("endpoint", endpoint), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	snapshotRows, err := model.ListChannelModelEndpointCandidatesByChannelIDWithDB(model.DB, channelID, modelName, endpoint)
	if err != nil {
		logChannelAdminWarn(c, "list_endpoints", stringField("channel_id", channelID), stringField("model", modelName), stringField("endpoint", endpoint), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	mergedRows := model.MergeChannelModelEndpointListRows(snapshotRows, explicitRows)
	testResultRows, err := model.ListChannelModelEndpointTestResultsByChannelIDWithDB(model.DB, channelID)
	if err != nil {
		logChannelAdminWarn(c, "list_endpoints", stringField("channel_id", channelID), stringField("model", modelName), stringField("endpoint", endpoint), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	testResultByKey := make(map[string]model.ChannelModelEndpointTestResult, len(testResultRows))
	for _, row := range testResultRows {
		key := strings.TrimSpace(row.Model) + "::" + model.NormalizeRequestedChannelModelEndpoint(row.Endpoint)
		if strings.TrimSpace(row.Model) == "" || strings.TrimSpace(row.Endpoint) == "" {
			continue
		}
		testResultByKey[key] = row
	}
	channelRow, err := channelsvc.GetByID(channelID)
	if err != nil {
		logChannelAdminWarn(c, "list_endpoints", stringField("channel_id", channelID), stringField("model", modelName), stringField("endpoint", endpoint), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	items := make([]channelEndpointItem, 0, len(mergedRows))
	for _, row := range mergedRows {
		item := channelEndpointItem{
			ChannelId:      row.ChannelId,
			Model:          row.Model,
			Endpoint:       row.Endpoint,
			BaseURL:        row.BaseURL,
			Enabled:        row.Enabled,
			UpdatedAt:      row.UpdatedAt,
			DisabledReason: row.DisabledReason,
			DisabledAt:     row.DisabledAt,
			DisabledBy:     row.DisabledBy,
		}
		if testRow, ok := testResultByKey[strings.TrimSpace(row.Model)+"::"+model.NormalizeRequestedChannelModelEndpoint(row.Endpoint)]; ok {
			item.LastTestStatus = strings.TrimSpace(testRow.LastTestStatus)
			item.LastTestedAt = testRow.LastTestedAt
			item.LastTestError = strings.TrimSpace(testRow.LastError)
		}
		selectedRow, ok := model.FindSelectedChannelModelConfig(channelRow.GetChannelModels(), row.Model)
		if ok && !row.Enabled {
			reason, reasonErr := model.ExplainManualChannelEndpointEnableBlockWithDB(model.DB, channelID, selectedRow, row.Endpoint)
			if reasonErr != nil {
				logChannelAdminWarn(c, "list_endpoints", stringField("channel_id", channelID), stringField("model", modelName), stringField("endpoint", endpoint), stringField("reason", reasonErr.Error()))
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": reasonErr.Error(),
				})
				return
			}
			item.EnableBlockReason = strings.TrimSpace(reason)
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items": items,
			"total": len(items),
		},
	})
}

func GetRecentDisabledChannelEndpoints(c *gin.Context) {
	rows, err := model.ListRecentDisabledChannelModelEndpointsWithDB(model.DB, 20)
	if err != nil {
		logChannelAdminWarn(c, "list_recent_disabled_endpoints", stringField("reason", err.Error()))
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
			logChannelAdminWarn(c, "list_recent_disabled_endpoints", stringField("reason", err.Error()))
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
	items := make([]channelRecentDisabledEndpointFeedItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, channelRecentDisabledEndpointFeedItem{
			ChannelModelEndpoint: row,
			ChannelName:          channelNameByID[strings.TrimSpace(row.ChannelId)],
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": channelRecentDisabledEndpointFeedData{
			Items: items,
			Total: len(items),
		},
	})
}

func UpdateChannelEndpoint(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "渠道 ID 无效",
		})
		return
	}
	req := updateChannelEndpointRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		logChannelAdminWarn(c, "update_endpoint", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	channelRow, err := channelsvc.GetByID(channelID)
	if err != nil {
		logChannelAdminWarn(c, "update_endpoint", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	modelName := strings.TrimSpace(req.Model)
	endpoint := model.NormalizeRequestedChannelModelEndpoint(req.Endpoint)
	if modelName == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "model 不能为空",
		})
		return
	}
	if endpoint == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "endpoint 无效",
		})
		return
	}
	selectedRow, ok := model.FindSelectedChannelModelConfig(channelRow.GetChannelModels(), modelName)
	if !ok {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "模型未启用，不能更新端点状态",
		})
		return
	}
	snapshotRows, err := model.ListChannelModelEndpointCandidatesByChannelIDWithDB(model.DB, channelID, modelName, endpoint)
	if err != nil {
		logChannelAdminWarn(c, "update_endpoint", stringField("channel_id", channelID), stringField("model", modelName), stringField("endpoint", endpoint), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	explicitRows, err := model.ListChannelModelEndpointsByChannelIDWithDB(model.DB, channelID, "", "")
	if err != nil {
		logChannelAdminWarn(c, "update_endpoint", stringField("channel_id", channelID), stringField("model", modelName), stringField("endpoint", endpoint), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	mergedRows := model.MergeChannelModelEndpointListRows(snapshotRows, explicitRows)
	if !model.HasChannelModelEndpoint(mergedRows, modelName, endpoint) {
		message := "该渠道当前未启用该模型端点，无法更新端点状态"
		logChannelAdminWarn(c, "update_endpoint", stringField("channel_id", channelID), stringField("model", modelName), stringField("endpoint", endpoint), stringField("reason", message))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": message,
		})
		return
	}
	if enabled {
		if err := model.ValidateManualChannelEndpointEnableWithDB(model.DB, channelID, selectedRow, endpoint); err != nil {
			logChannelAdminWarn(c, "update_endpoint", stringField("channel_id", channelID), stringField("model", modelName), stringField("endpoint", endpoint), stringField("reason", err.Error()))
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	restoredEndpoint := enabled && isChannelModelEndpointRuntimeDisabled(mergedRows, modelName, endpoint)
	row := model.ChannelModelEndpoint{
		ChannelId: channelRow.Id,
		Model:     modelName,
		Endpoint:  endpoint,
		BaseURL:   strings.TrimSpace(req.BaseURL),
		Enabled:   enabled,
	}
	if err := model.ReplaceChannelModelEndpointsWithDB(model.DB, channelID, mergeUpdatedChannelEndpointRows(explicitRows, row)); err != nil {
		logChannelAdminWarn(c, "update_endpoint", stringField("channel_id", channelID), stringField("model", modelName), stringField("endpoint", endpoint), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logChannelAdminInfo(c, "update_endpoint", stringField("channel_id", channelID), stringField("model", modelName), stringField("endpoint", endpoint), stringField("enabled", map[bool]string{true: "true", false: "false"}[enabled]))
	if restoredEndpoint {
		monitor.NotifyChannelModelEndpointCapabilityRestored(channelID, channelRow.DisplayName(), modelName, endpoint, channelAdminOperator(c))
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": channelEndpointItem{
			ChannelId: channelRow.Id,
			Model:     modelName,
			Endpoint:  endpoint,
			BaseURL:   strings.TrimSpace(req.BaseURL),
			Enabled:   enabled,
		},
	})
}

func mergeUpdatedChannelEndpointRows(rows []model.ChannelModelEndpoint, updated model.ChannelModelEndpoint) []model.ChannelModelEndpoint {
	normalizedUpdated := model.ChannelModelEndpoint{
		ChannelId: strings.TrimSpace(updated.ChannelId),
		Model:     strings.TrimSpace(updated.Model),
		Endpoint:  model.NormalizeRequestedChannelModelEndpoint(updated.Endpoint),
		BaseURL:   strings.TrimSpace(updated.BaseURL),
		Enabled:   updated.Enabled,
		UpdatedAt: updated.UpdatedAt,
	}
	result := make([]model.ChannelModelEndpoint, 0, len(rows)+1)
	replaced := false
	for _, row := range rows {
		normalizedRow := model.ChannelModelEndpoint{
			ChannelId: strings.TrimSpace(row.ChannelId),
			Model:     strings.TrimSpace(row.Model),
			Endpoint:  model.NormalizeRequestedChannelModelEndpoint(row.Endpoint),
			BaseURL:   strings.TrimSpace(row.BaseURL),
			Enabled:   row.Enabled,
			UpdatedAt: row.UpdatedAt,
		}
		if normalizedRow.ChannelId == normalizedUpdated.ChannelId &&
			normalizedRow.Model == normalizedUpdated.Model &&
			normalizedRow.Endpoint == normalizedUpdated.Endpoint {
			normalizedRow.BaseURL = normalizedUpdated.BaseURL
			normalizedRow.Enabled = normalizedUpdated.Enabled
			replaced = true
		}
		if normalizedRow.ChannelId == "" || normalizedRow.Model == "" || normalizedRow.Endpoint == "" {
			continue
		}
		result = append(result, normalizedRow)
	}
	if !replaced && normalizedUpdated.ChannelId != "" && normalizedUpdated.Model != "" && normalizedUpdated.Endpoint != "" {
		result = append(result, normalizedUpdated)
	}
	return result
}

func isChannelModelEndpointRuntimeDisabled(rows []model.ChannelModelEndpoint, modelName string, endpoint string) bool {
	normalizedModel := strings.TrimSpace(modelName)
	normalizedEndpoint := model.NormalizeRequestedChannelModelEndpoint(endpoint)
	if normalizedModel == "" || normalizedEndpoint == "" {
		return false
	}
	for _, row := range rows {
		if strings.TrimSpace(row.Model) != normalizedModel {
			continue
		}
		if model.NormalizeRequestedChannelModelEndpoint(row.Endpoint) != normalizedEndpoint {
			continue
		}
		return !row.Enabled &&
			(row.DisabledAt > 0 ||
				strings.TrimSpace(row.DisabledReason) != "" ||
				strings.TrimSpace(row.DisabledBy) != "")
	}
	return false
}
