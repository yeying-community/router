package channel

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/admin/model"
)

type channelModelTestsRequest struct {
	TargetModels  []string                     `json:"target_models"`
	TargetConfigs []channelModelTestTargetItem `json:"target_configs"`
	TestModel     string                       `json:"test_model,omitempty"`
	AudioLanguage string                       `json:"audio_language,omitempty"`
	ImageEditURL  string                       `json:"image_edit_url,omitempty"`
	ImageEditData string                       `json:"image_edit_data,omitempty"`
}

type refreshChannelRequest struct {
	Action string `json:"action,omitempty"`
}

func RefreshChannel(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		logChannelAdminWarn(c, "refresh_channel", stringField("reason", "渠道 ID 无效"))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	var req refreshChannelRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			logChannelAdminWarn(c, "refresh_channel", stringField("channel_id", channelID), stringField("reason", err.Error()))
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
	}
	action := strings.TrimSpace(strings.ToLower(req.Action))
	if action == "" {
		action = "models"
	}
	createdBy := c.GetString(ctxkey.Id)
	traceID := c.GetString(helper.TraceIDKey)
	var (
		taskRow   model.AsyncTask
		reused    bool
		err       error
		logAction string
	)
	switch action {
	case "models":
		logAction = "refresh_models"
		task, reusedValue, taskErr := CreateChannelRefreshModelsTask(channelID, createdBy, traceID)
		taskRow, reused, err = task, reusedValue, taskErr
	case "billing":
		logAction = "refresh_billing"
		task, reusedValue, taskErr := CreateChannelRefreshBillingTask(channelID, createdBy, traceID)
		taskRow, reused, err = task, reusedValue, taskErr
	default:
		logChannelAdminWarn(c, "refresh_channel", stringField("channel_id", channelID), stringField("action", action), stringField("reason", "不支持的刷新动作"))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "不支持的刷新动作"})
		return
	}
	if err != nil {
		logChannelAdminWarn(c, logAction, stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, logAction, stringField("channel_id", channelID), stringField("task_id", taskRow.Id), stringField("status", taskRow.Status))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{"task": taskRow},
		"meta":    gin.H{"channel_id": channelID, "reused": reused, "action": action},
	})
}

func TestChannelModels(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		logChannelAdminWarn(c, "test_models", stringField("reason", "渠道 ID 无效"))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	var req channelModelTestsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logChannelAdminWarn(c, "test_models", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	tasks, createdCount, reusedCount, err := CreateChannelModelTestTasks(
		channelID,
		c.GetString(ctxkey.Id),
		strings.TrimSpace(req.TestModel),
		req.TargetModels,
		req.TargetConfigs,
		c.GetString(helper.TraceIDKey),
		req.AudioLanguage,
		req.ImageEditURL,
		req.ImageEditData,
	)
	if err != nil {
		logChannelAdminWarn(c, "test_models", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "test_models", stringField("channel_id", channelID), intField("created", createdCount), intField("reused", reusedCount))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{"tasks": tasks},
		"meta":    gin.H{"channel_id": channelID, "created": createdCount, "reused": reusedCount},
	})
}
