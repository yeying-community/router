package channel

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
)

func resolveChannelBillingAPIBaseURL(channel *model.Channel, profile model.ChannelBillingProfile) string {
	if channel == nil {
		return ""
	}
	return normalizeChannelBillingAPIBaseURL(profile.ParseBillingConfig().APIBaseURL)
}

func normalizeChannelBillingAPIBaseURL(raw string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return trimmed
}

func resolveChannelCDKKey(channel *model.Channel, profile model.ChannelBillingProfile) string {
	if cdk := profile.ParseBillingConfig().CDK; cdk != "" {
		return cdk
	}
	if channel == nil {
		return ""
	}
	return strings.TrimSpace(channel.Key)
}

func resolveChannelCDKBillingCurrency(profile model.ChannelBillingProfile) string {
	if currency := profile.ParseBillingConfig().Currency; currency != "" {
		return currency
	}
	return "USD"
}

func resolveChannelBillingSnapshotCurrency(channel *model.Channel) string {
	if channel == nil {
		return "USD"
	}
	switch strings.TrimSpace(strings.ToLower(channel.GetProtocol())) {
	case "closeai", "openai-sb", "api2gpt", "deepseek", "siliconflow":
		return "CNY"
	default:
		return "USD"
	}
}

// UpdateChannelBilling submits a single-channel billing refresh task.
// The admin HTTP route is unified under POST /api/v1/admin/channel/{id}/refresh with action=billing.
func UpdateChannelBilling(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		logChannelAdminWarn(c, "refresh_billing", stringField("reason", "id 为空"))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "id 为空",
		})
		return
	}
	taskRow, reused, err := CreateChannelRefreshBillingTask(id, c.GetString(ctxkey.Id), c.GetString(helper.TraceIDKey))
	if err != nil {
		logChannelAdminWarn(c, "refresh_billing", stringField("channel_id", id), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logChannelAdminInfo(c, "refresh_billing", stringField("channel_id", taskRow.ChannelId), stringField("task_id", taskRow.Id), stringField("status", taskRow.Status))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"task": taskRow,
		},
		"meta": gin.H{
			"reused": reused,
		},
	})
}

func refreshAllChannelsBilling() error {
	channels, err := channelsvc.GetAllBasic(0, 0, "all", true)
	if err != nil {
		return err
	}
	for _, channel := range channels {
		if !shouldRefreshChannelBillingInBatch(channel) {
			continue
		}
		profile, err := model.GetChannelBillingProfileByChannelIDWithDB(model.DB, channel.Id)
		if err != nil {
			time.Sleep(config.RequestInterval)
			continue
		}
		if _, _, err := refreshAndPersistChannelBillingEntitlements(channel, profile, "批量自动刷新账务"); err != nil {
			time.Sleep(config.RequestInterval)
			continue
		}
		time.Sleep(config.RequestInterval)
	}
	return nil
}

func shouldRefreshChannelBillingInBatch(channel *model.Channel) bool {
	if channel == nil {
		return false
	}
	return channel.Status == model.ChannelStatusEnabled
}

func AutomaticallyRefreshChannelBilling(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Minute)
		logger.SysLog("refreshing channel billing")
		_ = refreshAllChannelsBilling()
		logger.SysLog("channel billing refresh done")
	}
}
