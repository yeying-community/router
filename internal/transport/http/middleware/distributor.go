package middleware

import (
	"fmt"
	"math/rand"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
)

type ModelRequest struct {
	Model string `json:"model" form:"model"`
}

func pickChannelByPriority(channels []*model.Channel, ignoreFirstPriority bool) *model.Channel {
	if len(channels) == 0 {
		return nil
	}
	endIdx := len(channels)
	firstPriority := channels[0].GetPriority()
	if firstPriority > 0 {
		for i := range channels {
			if channels[i].GetPriority() != firstPriority {
				endIdx = i
				break
			}
		}
	}
	targets := channels[:endIdx]
	if ignoreFirstPriority && endIdx < len(channels) {
		targets = channels[endIdx:]
	}
	if len(targets) == 0 {
		return nil
	}
	return targets[rand.Intn(len(targets))]
}

func Distribute() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userId := c.GetString(ctxkey.Id)
		userGroup, _ := model.CacheGetUserGroup(userId)
		c.Set(ctxkey.Group, userGroup)
		var requestModel string
		var channel *model.Channel
		var err error
		channelId, ok := c.Get(ctxkey.SpecificChannelId)
		if ok {
			id := fmt.Sprintf("%v", channelId)
			channel, err = model.GetChannelById(id)
			if err != nil {
				abortWithMessage(c, http.StatusBadRequest, "无效的渠道 Id")
				return
			}
			if channel.Status != model.ChannelStatusEnabled {
				abortWithMessage(c, http.StatusForbidden, "该渠道已被禁用")
				return
			}
		} else {
			requestModel = c.GetString(ctxkey.RequestModel)
			candidates, err := model.CacheListSatisfiedChannels(userGroup, requestModel)
			if err != nil {
				message := fmt.Sprintf("当前分组 %s 下对于模型 %s 无可用渠道", userGroup, requestModel)
				if channel != nil {
					logger.SysError(fmt.Sprintf("渠道不存在：%s", channel.Id))
					message = "数据库一致性已被破坏，请联系管理员"
				}
				abortWithMessage(c, http.StatusServiceUnavailable, message)
				return
			}
			channel = pickChannelByPriority(candidates, false)
			if channel == nil {
				message := fmt.Sprintf("当前分组 %s 下对于模型 %s 无可用渠道", userGroup, requestModel)
				abortWithMessage(c, http.StatusServiceUnavailable, message)
				return
			}
		}
		logger.Debugf(ctx, "user id %s, user group: %s, request model: %s, using channel #%s", userId, userGroup, requestModel, channel.Id)
		SetupContextForSelectedChannel(c, channel, requestModel)
		c.Next()
	}
}

func SetupContextForSelectedChannel(c *gin.Context, channel *model.Channel, modelName string) {
	channelProtocol := channel.GetChannelProtocol()
	c.Set(ctxkey.Channel, channelProtocol)
	c.Set(ctxkey.ChannelId, channel.Id)
	c.Set(ctxkey.ChannelName, channel.DisplayName())
	if channel.SystemPrompt != nil && *channel.SystemPrompt != "" {
		c.Set(ctxkey.SystemPrompt, *channel.SystemPrompt)
	}
	c.Set(ctxkey.ChannelModelConfigs, channel.GetSelectedModelConfigs())
	mapping := channel.GetModelMapping()
	if groupID := c.GetString(ctxkey.Group); groupID != "" {
		if override := model.CacheGetGroupModelMapping(groupID, modelName, channel.Id); len(override) > 0 {
			if mapping == nil {
				mapping = make(map[string]string, len(override))
			}
			for key, value := range override {
				mapping[key] = value
			}
		}
	}
	c.Set(ctxkey.ModelMapping, mapping)
	c.Set(ctxkey.OriginalModel, modelName) // for retry
	c.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", channel.Key))
	c.Set(ctxkey.BaseURL, channel.GetBaseURL())
	cfg, _ := channel.LoadConfig()
	// Some protocol-specific fields are still persisted in channel.other.
	if channel.Other != nil {
		switch channelProtocol {
		case relaychannel.Azure:
			if cfg.APIVersion == "" {
				cfg.APIVersion = *channel.Other
			}
		case relaychannel.Xunfei:
			if cfg.APIVersion == "" {
				cfg.APIVersion = *channel.Other
			}
		case relaychannel.Gemini:
			if cfg.APIVersion == "" {
				cfg.APIVersion = *channel.Other
			}
		case relaychannel.AIProxyLibrary:
			if cfg.LibraryID == "" {
				cfg.LibraryID = *channel.Other
			}
		case relaychannel.Ali:
			if cfg.Plugin == "" {
				cfg.Plugin = *channel.Other
			}
		}
	}
	c.Set(ctxkey.Config, cfg)
}
