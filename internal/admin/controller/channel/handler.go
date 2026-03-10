package channel

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/admin/model"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
)

const maxChannelListPageSize = 100

type updateChannelTestModelRequest struct {
	ID        string `json:"id"`
	TestModel string `json:"test_model"`
}

type createChannelRequest struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Key      string `json:"key"`
	BaseURL  string `json:"base_url"`
	Config   string `json:"config"`
}

type channelListItem struct {
	ID                 string   `json:"id"`
	Protocol           string   `json:"protocol"`
	Status             int      `json:"status"`
	Name               string   `json:"name"`
	Weight             *uint    `json:"weight,omitempty"`
	CreatedTime        int64    `json:"created_time"`
	TestTime           int64    `json:"test_time"`
	Capabilities       []string `json:"capabilities"`
	BaseURL            string   `json:"base_url,omitempty"`
	Other              string   `json:"other,omitempty"`
	Balance            float64  `json:"balance"`
	BalanceUpdatedTime int64    `json:"balance_updated_time"`
	UsedQuota          int64    `json:"used_quota"`
	Priority           int64    `json:"priority"`
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
	channel.ModelConfigs = model.NormalizeChannelModelConfigsPreserveOrder(channel.ModelConfigs)
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
		ID:                 strings.TrimSpace(channel.Id),
		Protocol:           strings.TrimSpace(channel.Protocol),
		Status:             channel.Status,
		Name:               strings.TrimSpace(channel.Name),
		Weight:             channel.Weight,
		CreatedTime:        channel.CreatedTime,
		TestTime:           channel.TestTime,
		Capabilities:       capabilities,
		BaseURL:            baseURL,
		Other:              other,
		Balance:            channel.Balance,
		BalanceUpdatedTime: channel.BalanceUpdatedTime,
		UsedQuota:          channel.UsedQuota,
		Priority:           channel.GetPriority(),
	}
}

func collectChannelCapabilities(channel *model.Channel) []string {
	if channel == nil {
		return []string{}
	}
	selectedTypes := map[string]struct{}{}
	for _, row := range channel.GetModelConfigs() {
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
	// Backward compatibility: legacy p is zero-based page index.
	if raw := strings.TrimSpace(c.Query("p")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			page = parsed + 1
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
	for _, row := range rows {
		items = append(items, buildChannelListItem(row))
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

// GetChannels godoc
// @Summary List channels with pagination (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page (1-based)"
// @Param page_size query int false "Page size"
// @Param keyword query string false "Keyword"
// @Param compact query int false "Compact mode (1=true)"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channels [get]
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

// GetChannel godoc
// @Summary Get channel by ID (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Channel ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/{id} [get]
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
	channel, err := channelsvc.GetBasicByID(id)
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
		"data":    channel,
	})
	return
}

// AddChannel godoc
// @Summary Create channel (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.ChannelCreateRequest true "Channel payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel [post]
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
	channel.NormalizeModelConfigState()
	channel.CreatedTime = helper.GetTimestamp()
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
	})
	return
}

// CreateChannel godoc
// @Summary Create channel record (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.ChannelCreateRecordRequest true "Channel create payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/create [post]
func CreateChannel(c *gin.Context) {
	req := createChannelRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		logChannelAdminWarn(c, "create_record", stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	name := model.NormalizeChannelIdentifier(req.Name)
	key := strings.TrimSpace(req.Key)
	if err := model.ValidateChannelIdentifier(name); err != nil {
		logChannelAdminWarn(c, "create_record", stringField("name", name), stringField("protocol", req.Protocol), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if key == "" {
		logChannelAdminWarn(c, "create_record", stringField("name", name), stringField("protocol", req.Protocol), stringField("reason", "渠道密钥不能为空"))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "渠道密钥不能为空",
		})
		return
	}
	baseURL := strings.TrimSpace(req.BaseURL)
	channel := model.Channel{
		Name:        name,
		Protocol:    strings.TrimSpace(req.Protocol),
		Key:         key,
		Status:      model.ChannelStatusCreating,
		Models:      "",
		BaseURL:     &baseURL,
		Config:      strings.TrimSpace(req.Config),
		CreatedTime: helper.GetTimestamp(),
	}
	if err := channelsvc.Insert(&channel); err != nil {
		logChannelAdminWarn(c, "create_record", stringField("name", channel.DisplayName()), stringField("protocol", channel.GetProtocol()), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logChannelAdminInfo(c, "create_record", stringField("channel_id", channel.Id), stringField("name", channel.DisplayName()), stringField("protocol", channel.GetProtocol()))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"id": channel.Id,
		},
	})
}

// DeleteChannel godoc
// @Summary Delete channel (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Channel ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/{id} [delete]
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

// DeleteDisabledChannel godoc
// @Summary Delete disabled channels (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/disabled [delete]
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

// UpdateChannel godoc
// @Summary Update channel (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.ChannelUpdateRequest true "Channel update payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel [put]
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
	_, channel.ModelConfigsProvided = rawFields["model_configs"]
	channel.NormalizeModelConfigState()
	err = channelsvc.Update(&channel)
	if err != nil {
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
		"data":    channel,
	})
	return
}

// UpdateChannelTestModel godoc
// @Summary Update channel test model (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body object true "Channel test model payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/test_model [put]
func UpdateChannelTestModel(c *gin.Context) {
	req := updateChannelTestModelRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		logChannelAdminWarn(c, "update_test_model", stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		logChannelAdminWarn(c, "update_test_model", stringField("reason", "渠道 ID 无效"))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "渠道 ID 无效",
		})
		return
	}
	channel, err := channelsvc.GetByID(req.ID)
	if err != nil {
		logChannelAdminWarn(c, "update_test_model", stringField("channel_id", req.ID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	req.TestModel = strings.TrimSpace(req.TestModel)
	if !isModelInChannelModels(req.TestModel, channel.Models) {
		logChannelAdminWarn(c, "update_test_model", stringField("channel_id", req.ID), stringField("test_model", req.TestModel), stringField("reason", "测试模型不在渠道支持的模型列表中"))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "测试模型不在渠道支持的模型列表中",
		})
		return
	}
	if err := channelsvc.UpdateTestModelByID(req.ID, req.TestModel); err != nil {
		logChannelAdminWarn(c, "update_test_model", stringField("channel_id", req.ID), stringField("test_model", req.TestModel), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logChannelAdminInfo(c, "update_test_model", stringField("channel_id", req.ID), stringField("test_model", req.TestModel))
	channel.TestModel = req.TestModel
	sanitizeChannelForResponse(channel)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channel,
	})
}
