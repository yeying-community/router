package middleware

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/responsestate"
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

func channelIDInList(channels []*model.Channel, channelID string) bool {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return false
	}
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		if strings.TrimSpace(channel.Id) == normalizedChannelID {
			return true
		}
	}
	return false
}

func selectPinnedResponsesChannel(c *gin.Context, userGroup string, requestModel string, requestPath string) (*model.Channel, bool) {
	previousResponseID := strings.TrimSpace(c.GetString(ctxkey.ResponsesPreviousResponseID))
	if previousResponseID == "" {
		return nil, false
	}
	channelID, ok := responsestate.LookupRoute(previousResponseID)
	if !ok {
		logger.RelayInfof(c.Request.Context(), "DISTRIBUTE decision=miss reason=responses_route_missing user_id=%s group=%s response_id=%s endpoint=%s", c.GetString(ctxkey.Id), userGroup, previousResponseID, requestPath)
		return nil, false
	}
	channel, err := model.GetChannelById(channelID)
	if err != nil {
		logger.RelayWarnf(c.Request.Context(), "DISTRIBUTE decision=miss reason=responses_route_channel_lookup_failed user_id=%s group=%s response_id=%s channel_id=%s endpoint=%s error=%q", c.GetString(ctxkey.Id), userGroup, previousResponseID, channelID, requestPath, err.Error())
		return nil, false
	}
	if channel.Status != model.ChannelStatusEnabled {
		logger.RelayWarnf(c.Request.Context(), "DISTRIBUTE decision=miss reason=responses_route_channel_disabled user_id=%s group=%s response_id=%s channel_id=%s endpoint=%s", c.GetString(ctxkey.Id), userGroup, previousResponseID, channelID, requestPath)
		return nil, false
	}
	if strings.TrimSpace(requestModel) != "" {
		channels, err := model.CacheListSatisfiedChannelsForRequest(userGroup, requestModel, requestPath)
		if err != nil {
			logger.RelayWarnf(c.Request.Context(), "DISTRIBUTE decision=miss reason=responses_route_validation_failed user_id=%s group=%s response_id=%s channel_id=%s model=%s endpoint=%s error=%q", c.GetString(ctxkey.Id), userGroup, previousResponseID, channelID, requestModel, requestPath, err.Error())
			return nil, false
		}
		if !channelIDInList(channels, channelID) {
			logger.RelayWarnf(c.Request.Context(), "DISTRIBUTE decision=miss reason=responses_route_not_eligible user_id=%s group=%s response_id=%s channel_id=%s model=%s endpoint=%s", c.GetString(ctxkey.Id), userGroup, previousResponseID, channelID, requestModel, requestPath)
			return nil, false
		}
	}
	logger.RelayInfof(c.Request.Context(), "DISTRIBUTE decision=pin reason=responses_route_match user_id=%s group=%s response_id=%s channel_id=%s model=%s endpoint=%s", c.GetString(ctxkey.Id), userGroup, previousResponseID, channelID, requestModel, requestPath)
	return channel, true
}

func selectEntitlementChannelForRequest(ctx context.Context, c *gin.Context, userID string, initialGroup string, initialSource *model.UserEntitlementSource, requestModel string) (*model.Channel, string, *model.UserEntitlementSource, error) {
	requestPath := c.Request.URL.Path
	if pinnedChannel, ok := selectPinnedResponsesChannel(c, initialGroup, requestModel, requestPath); ok {
		return pinnedChannel, initialGroup, initialSource, nil
	}
	type candidateSource struct {
		groupID string
		source  *model.UserEntitlementSource
	}
	candidateSources := []candidateSource{{groupID: initialGroup, source: initialSource}}
	if strings.TrimSpace(requestModel) != "" {
		if payload, err := model.BuildUserEntitlementModels(ctx, userID); err == nil {
			candidateSources = candidateSources[:0]
			for _, source := range payload.ByModel[strings.TrimSpace(requestModel)] {
				next := source
				candidateSources = append(candidateSources, candidateSource{
					groupID: strings.TrimSpace(source.GroupID),
					source:  &next,
				})
			}
		} else {
			logger.RelayWarnf(ctx, "DISTRIBUTE entitlement source reload failed user_id=%s model=%s endpoint=%s error=%q", userID, requestModel, requestPath, err.Error())
		}
	}
	if len(candidateSources) == 0 {
		candidateSources = append(candidateSources, candidateSource{groupID: initialGroup, source: initialSource})
	}
	var lastStats model.ChannelCandidateStats
	var lastErr error
	for _, candidate := range candidateSources {
		groupID := strings.TrimSpace(candidate.groupID)
		if groupID == "" {
			continue
		}
		if pinnedChannel, ok := selectPinnedResponsesChannel(c, groupID, requestModel, requestPath); ok {
			return pinnedChannel, groupID, candidate.source, nil
		}
		candidates, stats, err := model.CacheListSatisfiedChannelsForRequestWithStats(groupID, requestModel, requestPath)
		lastStats = stats
		if err != nil {
			lastErr = err
			logger.RelayWarnf(ctx, "DISTRIBUTE decision=skip reason=list_candidates_failed user_id=%s group=%s model=%s endpoint=%s listed_candidates=%d endpoint_filtered_candidates=%d error=%q", userID, groupID, requestModel, requestPath, stats.ListedCount, stats.EndpointFilteredCount, err.Error())
			continue
		}
		channel := pickChannelByPriority(candidates, false)
		if channel == nil {
			logger.RelayWarnf(ctx, "DISTRIBUTE decision=skip reason=no_available_channel user_id=%s group=%s model=%s endpoint=%s listed_candidates=%d endpoint_filtered_candidates=%d", userID, groupID, requestModel, requestPath, stats.ListedCount, stats.EndpointFilteredCount)
			continue
		}
		return channel, groupID, candidate.source, nil
	}
	message := fmt.Sprintf("当前权益下对于模型 %s 无可用渠道", requestModel)
	if strings.TrimSpace(initialGroup) != "" {
		message = fmt.Sprintf("当前分组 %s 下对于模型 %s 无可用渠道", initialGroup, requestModel)
	}
	if lastErr != nil {
		logger.RelayErrorf(ctx, "DISTRIBUTE decision=abort reason=no_entitlement_channel user_id=%s group=%s model=%s endpoint=%s listed_candidates=%d endpoint_filtered_candidates=%d message=%q error=%q", userID, initialGroup, requestModel, requestPath, lastStats.ListedCount, lastStats.EndpointFilteredCount, message, lastErr.Error())
	} else {
		logger.RelayErrorf(ctx, "DISTRIBUTE decision=abort reason=no_entitlement_channel user_id=%s group=%s model=%s endpoint=%s message=%q", userID, initialGroup, requestModel, requestPath, message)
	}
	return nil, "", nil, fmt.Errorf("%s", message)
}

func Distribute() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userId := c.GetString(ctxkey.Id)
		requestModel := c.GetString(ctxkey.RequestModel)
		userGroup, entitlementSource, groupErr := model.ResolveUserEntitlementGroupForModel(ctx, userId, requestModel)
		if groupErr != nil {
			logger.RelayWarnf(ctx, "DISTRIBUTE decision=abort reason=entitlement_group_missing user_id=%s model=%s endpoint=%s error=%q", userId, requestModel, c.Request.URL.Path, groupErr.Error())
			abortWithMessage(c, http.StatusServiceUnavailable, groupErr.Error())
			return
		}
		c.Set(ctxkey.Group, userGroup)
		if entitlementSource != nil {
			c.Set(ctxkey.EntitlementSourceType, entitlementSource.SourceType)
			c.Set(ctxkey.EntitlementSourceId, entitlementSource.SourceID)
			c.Set(ctxkey.EntitlementSourceName, entitlementSource.SourceName)
		}
		var channel *model.Channel
		var err error
		channelId, ok := c.Get(ctxkey.SpecificChannelId)
		if ok {
			id := fmt.Sprintf("%v", channelId)
			channel, err = model.GetChannelById(id)
			if err != nil {
				logger.RelayWarnf(ctx, "DISTRIBUTE decision=abort reason=invalid_specific_channel user_id=%s group=%s channel_id=%s endpoint=%s error=%q", userId, userGroup, id, c.Request.URL.Path, err.Error())
				abortWithMessage(c, http.StatusBadRequest, "无效的渠道 Id")
				return
			}
			if channel.Status != model.ChannelStatusEnabled {
				logger.RelayWarnf(ctx, "DISTRIBUTE decision=abort reason=specific_channel_disabled user_id=%s group=%s channel_id=%s channel_name=%s endpoint=%s", userId, userGroup, id, channel.DisplayName(), c.Request.URL.Path)
				abortWithMessage(c, http.StatusForbidden, "该渠道已被禁用")
				return
			}
		} else {
			if channel, userGroup, entitlementSource, err = selectEntitlementChannelForRequest(ctx, c, userId, userGroup, entitlementSource, requestModel); err != nil {
				abortWithMessage(c, http.StatusServiceUnavailable, err.Error())
				return
			}
			c.Set(ctxkey.Group, userGroup)
			if entitlementSource != nil {
				c.Set(ctxkey.EntitlementSourceType, entitlementSource.SourceType)
				c.Set(ctxkey.EntitlementSourceId, entitlementSource.SourceID)
				c.Set(ctxkey.EntitlementSourceName, entitlementSource.SourceName)
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
	c.Set(ctxkey.ChannelModelConfigs, channel.GetSelectedChannelModels())
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
	c.Set(ctxkey.BaseURL, channel.ResolveAPIBaseURL(""))
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
