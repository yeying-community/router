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

type updateChannelTestModelRequest struct {
	ID        string `json:"id"`
	TestModel string `json:"test_model"`
}

type createChannelDraftRequest struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Key      string `json:"key"`
	BaseURL  string `json:"base_url"`
	Config   string `json:"config"`
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
	channel.SetCapabilityProfiles(channel.CapabilityProfiles)
	channel.SetCapabilityResults(channel.CapabilityResults)
	channel.KeySet = strings.TrimSpace(channel.Key) != ""
	channel.Key = ""
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

// GetAllChannels godoc
// @Summary List channels (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param p query int false "Page index"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel [get]
func GetAllChannels(c *gin.Context) {
	p, _ := strconv.Atoi(c.Query("p"))
	if p < 0 {
		p = 0
	}
	channels, err := channelsvc.GetAll(p*config.ItemsPerPage, config.ItemsPerPage, "limited")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	for _, channel := range channels {
		sanitizeChannelForResponse(channel)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channels,
	})
	return
}

// SearchChannels godoc
// @Summary Search channels (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param keyword query string false "Keyword"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/search [get]
func SearchChannels(c *gin.Context) {
	keyword := c.Query("keyword")
	channels, err := channelsvc.Search(keyword)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	for _, channel := range channels {
		sanitizeChannelForResponse(channel)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channels,
	})
	return
}

// GetChannel godoc
// @Summary Get channel by ID (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "Channel ID"
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
	selectAll := false
	selectAllRaw := strings.TrimSpace(c.Query("select_all"))
	if selectAllRaw == "1" || strings.EqualFold(selectAllRaw, "true") {
		selectAll = true
	}
	var err error
	channel, err := channelsvc.GetByID(id, selectAll)
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
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	channel.CreatedTime = helper.GetTimestamp()
	keys := strings.Split(channel.Key, "\n")
	channels := make([]model.Channel, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		localChannel := channel
		localChannel.Key = key
		channels = append(channels, localChannel)
	}
	err = channelsvc.BatchInsert(channels)
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
	})
	return
}

// CreateChannelDraft godoc
// @Summary Create channel draft (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/draft [post]
func CreateChannelDraft(c *gin.Context) {
	req := createChannelDraftRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	name := strings.TrimSpace(req.Name)
	key := strings.TrimSpace(req.Key)
	if name == "" || key == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "渠道名称和密钥不能为空",
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
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
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
// @Param id path int true "Channel ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/{id} [delete]
func DeleteChannel(c *gin.Context) {
	id := c.Param("id")
	channel := model.Channel{Id: id}
	err := channelsvc.DeleteByID(channel.Id)
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
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
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
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	rawFields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(rawBody, &rawFields); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	_, channel.ModelsProvided = rawFields["models"]
	_, channel.CapabilityProfilesProvided = rawFields["capability_profiles"]
	err = channelsvc.Update(&channel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
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
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "渠道 ID 无效",
		})
		return
	}
	channel, err := channelsvc.GetByID(req.ID, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	req.TestModel = strings.TrimSpace(req.TestModel)
	if !isModelInChannelModels(req.TestModel, channel.Models) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "测试模型不在渠道支持的模型列表中",
		})
		return
	}
	if err := channelsvc.UpdateTestModelByID(req.ID, req.TestModel); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	channel.TestModel = req.TestModel
	sanitizeChannelForResponse(channel)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channel,
	})
}
