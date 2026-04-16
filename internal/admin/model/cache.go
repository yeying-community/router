package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
)

var (
	TokenCacheSeconds         = config.SyncFrequency
	UserId2GroupCacheSeconds  = config.SyncFrequency
	UserId2QuotaCacheSeconds  = config.SyncFrequency
	UserId2StatusCacheSeconds = config.SyncFrequency
	GroupModelsCacheSeconds   = config.SyncFrequency
)

func CacheGetTokenByKey(key string) (*Token, error) {
	keyCol := `"key"`
	var token Token
	if !common.RedisEnabled {
		err := DB.Where(keyCol+" = ?", key).First(&token).Error
		return &token, err
	}
	tokenObjectString, err := common.RedisGet(fmt.Sprintf("token:%s", key))
	if err != nil {
		err := DB.Where(keyCol+" = ?", key).First(&token).Error
		if err != nil {
			return nil, err
		}
		jsonBytes, err := json.Marshal(token)
		if err != nil {
			return nil, err
		}
		err = common.RedisSet(fmt.Sprintf("token:%s", key), string(jsonBytes), time.Duration(TokenCacheSeconds)*time.Second)
		if err != nil {
			logger.SysError("Redis set token error: " + err.Error())
		}
		return &token, nil
	}
	err = json.Unmarshal([]byte(tokenObjectString), &token)
	return &token, err
}

func CacheGetUserGroup(id string) (group string, err error) {
	if !common.RedisEnabled {
		return GetUserGroup(id)
	}
	group, err = common.RedisGet(fmt.Sprintf("user_group:%s", id))
	if err != nil {
		group, err = GetUserGroup(id)
		if err != nil {
			return "", err
		}
		err = common.RedisSet(fmt.Sprintf("user_group:%s", id), group, time.Duration(UserId2GroupCacheSeconds)*time.Second)
		if err != nil {
			logger.SysError("Redis set user group error: " + err.Error())
		}
	}
	return group, err
}

func fetchAndUpdateUserQuota(ctx context.Context, id string) (quota int64, err error) {
	quota, err = GetUserQuota(id)
	if err != nil {
		return 0, err
	}
	err = common.RedisSet(fmt.Sprintf("user_quota:%s", id), fmt.Sprintf("%d", quota), time.Duration(UserId2QuotaCacheSeconds)*time.Second)
	if err != nil {
		logger.Error(ctx, "Redis set user quota error: "+err.Error())
	}
	return
}

func CacheGetUserQuota(ctx context.Context, id string) (quota int64, err error) {
	if !common.RedisEnabled {
		return GetUserQuota(id)
	}
	quotaString, err := common.RedisGet(fmt.Sprintf("user_quota:%s", id))
	if err != nil {
		return fetchAndUpdateUserQuota(ctx, id)
	}
	quota, err = strconv.ParseInt(quotaString, 10, 64)
	if err != nil {
		return 0, nil
	}
	if quota <= config.PreConsumedQuota { // when user's quota is less than pre-consumed quota, we need to fetch from db
		logger.Infof(ctx, "user %s's cached quota is too low: %d, refreshing from db", id, quota)
		return fetchAndUpdateUserQuota(ctx, id)
	}
	return quota, nil
}

func CacheUpdateUserQuota(ctx context.Context, id string) error {
	if !common.RedisEnabled {
		return nil
	}
	quota, err := CacheGetUserQuota(ctx, id)
	if err != nil {
		return err
	}
	err = common.RedisSet(fmt.Sprintf("user_quota:%s", id), fmt.Sprintf("%d", quota), time.Duration(UserId2QuotaCacheSeconds)*time.Second)
	return err
}

func CacheDecreaseUserQuota(id string, quota int64) error {
	if !common.RedisEnabled {
		return nil
	}
	err := common.RedisDecrease(fmt.Sprintf("user_quota:%s", id), int64(quota))
	return err
}

func CacheIsUserEnabled(userId string) (bool, error) {
	if !common.RedisEnabled {
		return IsUserEnabled(userId)
	}
	enabled, err := common.RedisGet(fmt.Sprintf("user_enabled:%s", userId))
	if err == nil {
		return enabled == "1", nil
	}

	userEnabled, err := IsUserEnabled(userId)
	if err != nil {
		return false, err
	}
	enabled = "0"
	if userEnabled {
		enabled = "1"
	}
	err = common.RedisSet(fmt.Sprintf("user_enabled:%s", userId), enabled, time.Duration(UserId2StatusCacheSeconds)*time.Second)
	if err != nil {
		logger.SysError("Redis set user enabled error: " + err.Error())
	}
	return userEnabled, err
}

func CacheGetGroupModels(ctx context.Context, group string) ([]string, error) {
	if !common.RedisEnabled {
		return GetGroupModels(ctx, group)
	}
	modelsStr, err := common.RedisGet(fmt.Sprintf("group_models:%s", group))
	if err == nil {
		return strings.Split(modelsStr, ","), nil
	}
	models, err := GetGroupModels(ctx, group)
	if err != nil {
		return nil, err
	}
	err = common.RedisSet(fmt.Sprintf("group_models:%s", group), strings.Join(models, ","), time.Duration(GroupModelsCacheSeconds)*time.Second)
	if err != nil {
		logger.SysError("Redis set group models error: " + err.Error())
	}
	return models, nil
}

var group2model2channels map[string]map[string][]*Channel
var group2model2channel2upstream map[string]map[string]map[string]string
var channel2model2endpointEnabled map[string]map[string]map[string]bool
var channelSyncLock sync.RWMutex

func InitChannelCache() {
	var channels []*Channel
	DB.Where("status = ?", ChannelStatusEnabled).Find(&channels)
	if err := HydrateChannelsWithModels(DB, channels); err != nil {
		logger.SysError("failed to hydrate channel models for cache: " + err.Error())
	}
	var abilities []*Ability
	DB.Where("enabled = ?", true).Find(&abilities)
	endpointRows := make([]ChannelModelEndpoint, 0)
	if err := DB.Find(&endpointRows).Error; err != nil {
		logger.SysError("failed to load channel model endpoint cache: " + err.Error())
	}
	groups := make(map[string]bool)
	for _, ability := range abilities {
		groupName := strings.TrimSpace(ability.Group)
		if groupName == "" {
			continue
		}
		groups[groupName] = true
	}
	newGroup2model2channels := make(map[string]map[string][]*Channel)
	newGroup2model2channel2upstream := make(map[string]map[string]map[string]string)
	newChannel2model2endpointEnabled := buildChannelModelEndpointSupportCache(endpointRows)
	for group := range groups {
		newGroup2model2channels[group] = make(map[string][]*Channel)
		newGroup2model2channel2upstream[group] = make(map[string]map[string]string)
	}
	channelByID := make(map[string]*Channel, len(channels))
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		channel.NormalizeIdentity()
		channelID := strings.TrimSpace(channel.Id)
		if channelID == "" {
			continue
		}
		channelByID[channelID] = channel
	}
	for _, ability := range abilities {
		if ability == nil {
			continue
		}
		groupName := strings.TrimSpace(ability.Group)
		modelName := strings.TrimSpace(ability.Model)
		channelID := strings.TrimSpace(ability.ChannelId)
		if groupName == "" || modelName == "" || channelID == "" {
			continue
		}
		channel, ok := channelByID[channelID]
		if !ok {
			continue
		}
		if _, ok := newGroup2model2channels[groupName][modelName]; !ok {
			newGroup2model2channels[groupName][modelName] = make([]*Channel, 0)
		}
		if _, ok := newGroup2model2channel2upstream[groupName][modelName]; !ok {
			newGroup2model2channel2upstream[groupName][modelName] = make(map[string]string)
		}
		newGroup2model2channels[groupName][modelName] = append(newGroup2model2channels[groupName][modelName], channel)
		newGroup2model2channel2upstream[groupName][modelName][channelID] = NormalizeAbilityUpstreamModel(modelName, ability.UpstreamModel)
	}

	// sort by priority
	for group, model2channels := range newGroup2model2channels {
		for model, channels := range model2channels {
			sort.Slice(channels, func(i, j int) bool {
				return channels[i].GetPriority() > channels[j].GetPriority()
			})
			newGroup2model2channels[group][model] = channels
		}
	}

	channelSyncLock.Lock()
	group2model2channels = newGroup2model2channels
	group2model2channel2upstream = newGroup2model2channel2upstream
	channel2model2endpointEnabled = newChannel2model2endpointEnabled
	channelSyncLock.Unlock()
	logger.SysLog("channels synced from database")
}

func CacheListSatisfiedChannels(group string, model string) ([]*Channel, error) {
	if !config.MemoryCacheEnabled {
		return ListSatisfiedChannels(group, model)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()
	channels := group2model2channels[group][model]
	if len(channels) == 0 {
		return nil, errors.New("channel not found")
	}
	result := make([]*Channel, 0, len(channels))
	result = append(result, channels...)
	return result, nil
}

func CacheListSatisfiedChannelsForRequest(group string, model string, requestPath string) ([]*Channel, error) {
	channels, err := CacheListSatisfiedChannels(group, model)
	if err != nil {
		return nil, err
	}
	return cacheFilterChannelsByRequestEndpoint(channels, model, requestPath)
}

func CacheGetChannelModelEndpointSupport(channelID string, modelCandidates ...string) map[string]bool {
	normalizedChannelID := strings.TrimSpace(channelID)
	normalizedCandidates := normalizeTrimmedValuesPreserveOrder(modelCandidates)
	if normalizedChannelID == "" || len(normalizedCandidates) == 0 {
		return nil
	}
	if config.MemoryCacheEnabled {
		channelSyncLock.RLock()
		modelMap := channel2model2endpointEnabled[normalizedChannelID]
		for _, modelName := range normalizedCandidates {
			if endpointMap, ok := modelMap[modelName]; ok && len(endpointMap) > 0 {
				result := make(map[string]bool, len(endpointMap))
				for endpoint, enabled := range endpointMap {
					result[endpoint] = enabled
				}
				channelSyncLock.RUnlock()
				return result
			}
		}
		channelSyncLock.RUnlock()
	}
	for _, modelName := range normalizedCandidates {
		supportByChannelID, err := listChannelModelEndpointSupportByChannelIDsWithDB(DB, []string{normalizedChannelID}, modelName)
		if err != nil {
			logger.SysError("load channel model endpoint support failed: " + err.Error())
			continue
		}
		endpointMap := supportByChannelID[normalizedChannelID]
		if len(endpointMap) == 0 {
			continue
		}
		result := make(map[string]bool, len(endpointMap))
		for endpoint, enabled := range endpointMap {
			result[endpoint] = enabled
		}
		return result
	}
	return nil
}

func CacheGetGroupModelMapping(group string, modelName string, channelID string) map[string]string {
	group = strings.TrimSpace(group)
	modelName = strings.TrimSpace(modelName)
	channelID = strings.TrimSpace(channelID)
	if group == "" || modelName == "" || channelID == "" {
		return nil
	}

	upstreamModel := ""
	if config.MemoryCacheEnabled {
		channelSyncLock.RLock()
		if groupModels, ok := group2model2channel2upstream[group]; ok {
			if channelMappings, ok := groupModels[modelName]; ok {
				upstreamModel = strings.TrimSpace(channelMappings[channelID])
			}
		}
		channelSyncLock.RUnlock()
	} else {
		groupCol := `"group"`
		record := Ability{}
		if err := DB.Where(groupCol+" = ? AND model = ? AND channel_id = ?", group, modelName, channelID).Take(&record).Error; err == nil {
			upstreamModel = NormalizeAbilityUpstreamModel(modelName, record.UpstreamModel)
		}
	}

	upstreamModel = strings.TrimSpace(upstreamModel)
	if upstreamModel == "" || upstreamModel == modelName {
		return nil
	}
	return map[string]string{
		modelName: upstreamModel,
	}
}

func RefreshAbilityCachesForGroups(groupIDs ...string) {
	if !common.RedisEnabled || common.RDB == nil {
		if config.MemoryCacheEnabled {
			InitChannelCache()
		}
		return
	}
	for _, groupID := range normalizeTrimmedValuesPreserveOrder(groupIDs) {
		if groupID == "" {
			continue
		}
		if err := common.RedisDel(fmt.Sprintf("group_models:%s", groupID)); err != nil {
			logger.SysError("Redis delete group models error: " + err.Error())
		}
	}
	if config.MemoryCacheEnabled {
		InitChannelCache()
	}
}

func RefreshUserGroupCaches(userIDs ...string) {
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	for _, userID := range normalizeTrimmedValuesPreserveOrder(userIDs) {
		if userID == "" {
			continue
		}
		if err := common.RedisDel(fmt.Sprintf("user_group:%s", userID)); err != nil {
			logger.SysError("Redis delete user group error: " + err.Error())
		}
	}
}

func SyncChannelCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		logger.SysLog("syncing channels from database")
		InitChannelCache()
	}
}

func CacheGetRandomSatisfiedChannel(group string, model string, ignoreFirstPriority bool) (*Channel, error) {
	return CacheGetRandomSatisfiedChannelExcluding(group, model, ignoreFirstPriority, nil)
}

func CacheGetRandomSatisfiedChannelForRequest(group string, model string, requestPath string, ignoreFirstPriority bool) (*Channel, error) {
	return CacheGetRandomSatisfiedChannelForRequestExcluding(group, model, requestPath, ignoreFirstPriority, nil)
}

func CacheGetRandomSatisfiedChannelExcluding(group string, model string, ignoreFirstPriority bool, excludedChannelIDs map[string]struct{}) (*Channel, error) {
	channel, _, err := CacheSelectRandomSatisfiedChannelExcluding(group, model, ignoreFirstPriority, excludedChannelIDs)
	return channel, err
}

func CacheGetRandomSatisfiedChannelForRequestExcluding(group string, model string, requestPath string, ignoreFirstPriority bool, excludedChannelIDs map[string]struct{}) (*Channel, error) {
	channel, _, err := CacheSelectRandomSatisfiedChannelForRequestExcluding(group, model, requestPath, ignoreFirstPriority, excludedChannelIDs)
	return channel, err
}

func CacheSelectRandomSatisfiedChannelExcluding(group string, model string, ignoreFirstPriority bool, excludedChannelIDs map[string]struct{}) (*Channel, SatisfiedChannelSelectionStats, error) {
	if !config.MemoryCacheEnabled {
		channels, err := ListSatisfiedChannels(group, model)
		if err != nil {
			return nil, SatisfiedChannelSelectionStats{}, err
		}
		channel, stats := SelectRandomSatisfiedChannelWithStats(channels, ignoreFirstPriority, excludedChannelIDs)
		if channel == nil {
			return nil, stats, errors.New("channel not found")
		}
		return channel, stats, nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()
	channels := group2model2channels[group][model]
	if len(channels) == 0 {
		return nil, SatisfiedChannelSelectionStats{}, errors.New("channel not found")
	}
	channel, stats := SelectRandomSatisfiedChannelWithStats(channels, ignoreFirstPriority, excludedChannelIDs)
	if channel == nil {
		return nil, stats, errors.New("channel not found")
	}
	return channel, stats, nil
}

func CacheSelectRandomSatisfiedChannelForRequestExcluding(group string, model string, requestPath string, ignoreFirstPriority bool, excludedChannelIDs map[string]struct{}) (*Channel, SatisfiedChannelSelectionStats, error) {
	channels, err := CacheListSatisfiedChannelsForRequest(group, model, requestPath)
	if err != nil {
		return nil, SatisfiedChannelSelectionStats{}, err
	}
	channel, stats := SelectRandomSatisfiedChannelWithStats(channels, ignoreFirstPriority, excludedChannelIDs)
	if channel == nil {
		return nil, stats, errors.New("channel not found")
	}
	return channel, stats, nil
}

func buildChannelModelEndpointSupportCache(rows []ChannelModelEndpoint) map[string]map[string]map[string]bool {
	result := make(map[string]map[string]map[string]bool)
	for _, row := range rows {
		channelID := strings.TrimSpace(row.ChannelId)
		modelName := strings.TrimSpace(row.Model)
		endpoint := NormalizeRequestedChannelModelEndpoint(row.Endpoint)
		if channelID == "" || modelName == "" || endpoint == "" {
			continue
		}
		if _, ok := result[channelID]; !ok {
			result[channelID] = make(map[string]map[string]bool)
		}
		if _, ok := result[channelID][modelName]; !ok {
			result[channelID][modelName] = make(map[string]bool)
		}
		result[channelID][modelName][endpoint] = row.Enabled
	}
	return result
}

func cacheFilterChannelsByRequestEndpoint(channels []*Channel, modelName string, requestPath string) ([]*Channel, error) {
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(requestPath)
	if normalizedEndpoint == "" || len(channels) == 0 {
		result := make([]*Channel, 0, len(channels))
		result = append(result, channels...)
		return result, nil
	}
	if !config.MemoryCacheEnabled {
		channelIDs := make([]string, 0, len(channels))
		for _, channel := range channels {
			if channel == nil {
				continue
			}
			channelIDs = append(channelIDs, channel.Id)
		}
		supportByChannelID, err := listChannelModelEndpointSupportByChannelIDsWithDB(DB, channelIDs, modelName)
		if err != nil {
			return nil, err
		}
		wrapped := make(map[string]map[string]map[string]bool, len(supportByChannelID))
		for channelID, endpointMap := range supportByChannelID {
			wrapped[channelID] = map[string]map[string]bool{
				strings.TrimSpace(modelName): endpointMap,
			}
		}
		return filterChannelsByRequestEndpoint(channels, modelName, requestPath, wrapped), nil
	}
	channelSyncLock.RLock()
	supportByChannelID := channel2model2endpointEnabled
	channelSyncLock.RUnlock()
	return filterChannelsByRequestEndpoint(channels, modelName, requestPath, supportByChannelID), nil
}

func filterChannelsByRequestEndpoint(channels []*Channel, modelName string, requestPath string, supportByChannelID map[string]map[string]map[string]bool) []*Channel {
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(requestPath)
	if normalizedEndpoint == "" || len(channels) == 0 {
		result := make([]*Channel, 0, len(channels))
		result = append(result, channels...)
		return result
	}
	result := make([]*Channel, 0, len(channels))
	normalizedModelName := strings.TrimSpace(modelName)
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		channelID := strings.TrimSpace(channel.Id)
		explicitSupport := map[string]bool(nil)
		if groupModels, ok := supportByChannelID[channelID]; ok {
			explicitSupport = groupModels[normalizedModelName]
		}
		if supported, explicit := IsChannelModelRequestEndpointSupportedByEndpointMap(explicitSupport, requestPath); explicit {
			if supported {
				result = append(result, channel)
			}
			continue
		}
		if IsChannelModelRequestEndpointSupportedByConfigs(channel.GetModelConfigs(), normalizedModelName, requestPath) {
			result = append(result, channel)
		}
	}
	return result
}
