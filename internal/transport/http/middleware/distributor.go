package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/responsestate"
	"github.com/yeying-community/router/internal/relay/routeobs"
	"github.com/yeying-community/router/internal/relay/routing"
	"gorm.io/gorm"
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

func pickChannelByPolicy(channels []*model.Channel, policy routing.ProviderRoutingPolicy) *model.Channel {
	if policy.SelectionMethod != routing.SelectionWeightedRandom {
		return pickChannelByPriority(channels, false)
	}
	if len(channels) == 0 {
		return nil
	}
	firstPriority := channels[0].GetPriority()
	tier := make([]*model.Channel, 0, len(channels))
	for _, channel := range channels {
		if channel.GetPriority() != firstPriority {
			break
		}
		tier = append(tier, channel)
	}
	if len(tier) == 0 {
		return nil
	}
	var total uint64
	for _, channel := range tier {
		weight := channel.GetWeight()
		if weight == 0 {
			weight = 1
		}
		total += uint64(weight)
	}
	target := uint64(rand.Int63n(int64(total)))
	for _, channel := range tier {
		weight := channel.GetWeight()
		if weight == 0 {
			weight = 1
		}
		if target < uint64(weight) {
			return channel
		}
		target -= uint64(weight)
	}
	return tier[len(tier)-1]
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

func channelIDs(channels []*model.Channel) []string {
	result := make([]string, 0, len(channels))
	for _, channel := range channels {
		if channel == nil || strings.TrimSpace(channel.Id) == "" {
			continue
		}
		result = append(result, channel.Id)
	}
	return result
}

func channelProvider(channel *model.Channel, requestModel string) string {
	if channel == nil {
		return ""
	}
	for _, row := range channel.GetSelectedChannelModels() {
		if requestModel == row.Model || requestModel == row.UpstreamModel {
			return model.NormalizeGroupModelProviderValue(row.Provider)
		}
	}
	return ""
}

func applyProviderRoutingPolicy(channels []*model.Channel, requestModel string, policy routing.ProviderRoutingPolicy) ([]*model.Channel, []model.ChannelCandidateFilter) {
	if len(channels) == 0 || policy.ProviderScope.Mode == routing.ProviderScopeAny && len(policy.ProviderOrder) == 0 {
		return channels, nil
	}
	allowed := make(map[string]struct{}, len(policy.ProviderScope.Providers))
	for _, provider := range policy.ProviderScope.Providers {
		allowed[provider] = struct{}{}
	}
	ordered := make(map[string]int, len(policy.ProviderOrder))
	for index, provider := range policy.ProviderOrder {
		ordered[provider] = index
	}
	result := make([]*model.Channel, 0, len(channels))
	filtered := make([]model.ChannelCandidateFilter, 0)
	for _, channel := range channels {
		provider := channelProvider(channel, requestModel)
		_, listed := allowed[provider]
		excluded := (policy.ProviderScope.Mode == routing.ProviderScopeAllowList && !listed) ||
			(policy.ProviderScope.Mode == routing.ProviderScopeDenyList && listed)
		if excluded {
			filtered = append(filtered, model.ChannelCandidateFilter{ChannelID: channel.Id, Reason: "provider_scope"})
			continue
		}
		result = append(result, channel)
	}
	if len(ordered) > 0 {
		sort.SliceStable(result, func(i, j int) bool {
			left, leftOK := ordered[channelProvider(result[i], requestModel)]
			right, rightOK := ordered[channelProvider(result[j], requestModel)]
			if leftOK != rightOK {
				return leftOK
			}
			if leftOK && left != right {
				return left < right
			}
			return result[i].GetPriority() > result[j].GetPriority()
		})
	}
	return result, filtered
}

func recordRouteDecision(c *gin.Context, source string, groupID string, requestModel string, requestPath string, candidates []*model.Channel, filteredCandidates []model.ChannelCandidateFilter, selected *model.Channel, selectionMode string) {
	if selected == nil {
		return
	}
	filtered := make([]routeobs.FilteredCandidate, 0, len(filteredCandidates))
	for _, candidate := range filteredCandidates {
		filtered = append(filtered, routeobs.FilteredCandidate{ChannelID: candidate.ChannelID, Reason: candidate.Reason})
	}
	routeobs.SetRouteDecision(c, routeobs.RouteDecision{
		Source:              source,
		GroupID:             groupID,
		Model:               requestModel,
		Endpoint:            requestPath,
		CandidateChannelIDs: channelIDs(candidates),
		FilteredCandidates:  filtered,
		SelectedPriority:    selected.GetPriority(),
		SelectionMode:       selectionMode,
		InitialChannelID:    selected.Id,
		InitialChannelName:  selected.DisplayName(),
	})
}

func selectPinnedResponsesChannel(c *gin.Context, userID string, userGroup string, requestModel string, requestPath string) (*model.Channel, bool) {
	channelID, ok, conflict := lookupPinnedResponsesChannelID(c)
	if conflict {
		previousResponseID := strings.TrimSpace(c.GetString(ctxkey.ResponsesPreviousResponseID))
		itemIDs := responseItemIDsFromContext(c)
		logger.RelayWarnf(c.Request.Context(), "DISTRIBUTE decision=state_conflict reason=responses_route_multiple_channels user_id=%s group=%s response_id=%s item_ids=%s endpoint=%s", c.GetString(ctxkey.Id), userGroup, previousResponseID, strings.Join(itemIDs, ","), requestPath)
		// The pinned state is no longer trustworthy. Let automatic routing pick
		// a currently healthy channel and replay the request without old IDs.
		return nil, false
	}
	if !ok {
		previousResponseID := strings.TrimSpace(c.GetString(ctxkey.ResponsesPreviousResponseID))
		logger.RelayInfof(c.Request.Context(), "DISTRIBUTE decision=miss reason=responses_route_missing user_id=%s group=%s response_id=%s endpoint=%s", c.GetString(ctxkey.Id), userGroup, previousResponseID, requestPath)
		return nil, false
	}
	previousResponseID := strings.TrimSpace(c.GetString(ctxkey.ResponsesPreviousResponseID))
	var channel *model.Channel
	var err error
	if model.IsPersonalProviderChannelID(channelID) {
		channel, err = model.ResolvePersonalProviderChannel(userID, channelID)
	} else if model.IsCommunityOfferChannelID(channelID) {
		channel, err = model.ResolveCommunityOfferChannel(model.CommunityOfferIDFromChannelID(channelID), requestModel, requestPath)
	} else {
		channel, err = model.GetChannelById(channelID)
	}
	if err != nil {
		logger.RelayWarnf(c.Request.Context(), "DISTRIBUTE decision=miss reason=responses_route_channel_lookup_failed user_id=%s group=%s response_id=%s channel_id=%s endpoint=%s error=%q", c.GetString(ctxkey.Id), userGroup, previousResponseID, channelID, requestPath, err.Error())
		return nil, false
	}
	if channel.Status != model.ChannelStatusEnabled {
		logger.RelayWarnf(c.Request.Context(), "DISTRIBUTE decision=miss reason=responses_route_channel_disabled user_id=%s group=%s response_id=%s channel_id=%s endpoint=%s", c.GetString(ctxkey.Id), userGroup, previousResponseID, channelID, requestPath)
		return nil, false
	}
	if strings.TrimSpace(requestModel) != "" && !model.IsPersonalProviderChannelID(channelID) && !model.IsCommunityOfferChannelID(channelID) {
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
	recordRouteDecision(c, "responses_pin", userGroup, requestModel, requestPath, []*model.Channel{channel}, nil, channel, "pinned")
	return channel, true
}

func lookupPinnedResponsesChannelID(c *gin.Context) (string, bool, bool) {
	if c == nil {
		return "", false, false
	}
	previousResponseID := strings.TrimSpace(c.GetString(ctxkey.ResponsesPreviousResponseID))
	itemIDs := responseItemIDsFromContext(c)
	if previousResponseID == "" && len(itemIDs) == 0 {
		return "", false, false
	}
	lookupIDs := append([]string{}, itemIDs...)
	if previousResponseID != "" {
		lookupIDs = append(lookupIDs, previousResponseID)
	}
	return responsestate.LookupRoutes(lookupIDs)
}

func responseItemIDsFromContext(c *gin.Context) []string {
	value, exists := c.Get(ctxkey.ResponsesItemIDs)
	if !exists {
		return nil
	}
	ids, ok := value.([]string)
	if !ok {
		return nil
	}
	return ids
}

// SelectEntitlementChannelForRequest selects a community channel from the
// user's package and balance entitlements. It is also used by relay retry when
// a personal-first request needs to fall back after its private upstream fails.
func SelectEntitlementChannelForRequest(ctx context.Context, c *gin.Context, userID string, initialGroup string, initialSource *model.UserEntitlementSource, requestModel string) (*model.Channel, string, *model.UserEntitlementSource, error) {
	requestPath := c.Request.URL.Path
	if responseStateConflict(c) {
		logger.RelayWarnf(ctx, "DISTRIBUTE decision=state_recovery reason=responses_route_multiple_channels user_id=%s group=%s model=%s endpoint=%s", userID, initialGroup, requestModel, requestPath)
		if err := resetConflictingResponsesState(c); err != nil {
			return nil, initialGroup, initialSource, fmt.Errorf("responses state recovery failed: %w", err)
		}
	}
	if pinnedChannel, ok := selectPinnedResponsesChannel(c, userID, initialGroup, requestModel, requestPath); ok {
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
		if pinnedChannel, ok := selectPinnedResponsesChannel(c, userID, groupID, requestModel, requestPath); ok {
			return pinnedChannel, groupID, candidate.source, nil
		}
		candidates, stats, err := model.CacheListSatisfiedChannelsForRequestWithStats(groupID, requestModel, requestPath)
		lastStats = stats
		if err != nil {
			lastErr = err
			logger.RelayWarnf(ctx, "DISTRIBUTE decision=skip reason=list_candidates_failed user_id=%s group=%s model=%s endpoint=%s listed_candidates=%d endpoint_filtered_candidates=%d error=%q", userID, groupID, requestModel, requestPath, stats.ListedCount, stats.EndpointFilteredCount, err.Error())
			continue
		}
		policyValue, _ := c.Get(ctxkey.ProviderRoutingPolicy)
		policy, policyOK := policyValue.(routing.ProviderRoutingPolicy)
		policyFiltered := []model.ChannelCandidateFilter(nil)
		if policyOK {
			candidates, policyFiltered = applyProviderRoutingPolicy(candidates, requestModel, policy)
			stats.FilteredCandidates = append(stats.FilteredCandidates, policyFiltered...)
		}
		channel := pickChannelByPolicy(candidates, policy)
		if channel == nil {
			logger.RelayWarnf(ctx, "DISTRIBUTE decision=skip reason=no_available_channel user_id=%s group=%s model=%s endpoint=%s listed_candidates=%d endpoint_filtered_candidates=%d", userID, groupID, requestModel, requestPath, stats.ListedCount, stats.EndpointFilteredCount)
			continue
		}
		recordRouteDecision(c, "automatic", groupID, requestModel, requestPath, candidates, stats.FilteredCandidates, channel, "priority_random")
		return channel, groupID, candidate.source, nil
	}
	message := fmt.Sprintf("当前权益下对于模型 %s 无可用渠道", requestModel)
	if strings.TrimSpace(initialGroup) != "" {
		message = fmt.Sprintf("当前分组 %s 下对于模型 %s 无可用渠道", initialGroup, requestModel)
	}
	if !model.HasPublishedModelChannelBinding(initialGroup, requestModel) {
		c.Set(ctxkey.RelayModelUnavailable, true)
		c.Set(ctxkey.RelayErrorCode, "model_not_found")
		message = fmt.Sprintf("模型 %s 当前未发布或不支持使用，请从可用模型列表中选择其他模型", requestModel)
		logger.RelayWarnf(ctx, "DISTRIBUTE decision=abort reason=model_unavailable user_id=%s group=%s model=%s endpoint=%s listed_candidates=%d message=%q", userID, initialGroup, requestModel, requestPath, lastStats.ListedCount, message)
		return nil, "", nil, fmt.Errorf("%s", message)
	}
	if lastErr != nil {
		logger.RelayErrorf(ctx, "DISTRIBUTE decision=abort reason=no_entitlement_channel user_id=%s group=%s model=%s endpoint=%s listed_candidates=%d endpoint_filtered_candidates=%d message=%q error=%q", userID, initialGroup, requestModel, requestPath, lastStats.ListedCount, lastStats.EndpointFilteredCount, message, lastErr.Error())
	} else {
		logger.RelayErrorf(ctx, "DISTRIBUTE decision=abort reason=no_entitlement_channel user_id=%s group=%s model=%s endpoint=%s message=%q", userID, initialGroup, requestModel, requestPath, message)
	}
	return nil, "", nil, fmt.Errorf("%s", message)
}

// resetConflictingResponsesState removes identifiers owned by another upstream
// provider while preserving replayable content and tool arguments.
func resetConflictingResponsesState(c *gin.Context) error {
	raw, err := common.GetRequestBody(c)
	if err != nil || len(raw) == 0 {
		return err
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	delete(payload, "previous_response_id")
	stripResponseItemIDs(payload)
	updated, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	c.Set(ctxkey.KeyRequestBody, updated)
	c.Request.Body = io.NopCloser(bytes.NewReader(updated))
	c.Set(ctxkey.ResponsesPreviousResponseID, "")
	c.Set(ctxkey.ResponsesItemIDs, nil)
	c.Set(ctxkey.ResponsesStatefulRequest, false)
	return nil
}

func stripResponseItemIDs(value any) {
	switch typed := value.(type) {
	case map[string]any:
		if itemType, ok := typed["type"].(string); ok && strings.TrimSpace(itemType) != "response" {
			delete(typed, "id")
		}
		for _, child := range typed {
			stripResponseItemIDs(child)
		}
	case []any:
		for _, child := range typed {
			stripResponseItemIDs(child)
		}
	}
}

func responseStateConflict(c *gin.Context) bool {
	if c == nil {
		return false
	}
	ids := responseItemIDsFromContext(c)
	if previous := strings.TrimSpace(c.GetString(ctxkey.ResponsesPreviousResponseID)); previous != "" {
		ids = append(ids, previous)
	}
	if len(ids) == 0 {
		return false
	}
	_, _, conflict := responsestate.LookupRoutes(ids)
	return conflict
}

// requestBodyIsForm 判断请求体是否为表单编码（multipart/form-data 或
// x-www-form-urlencoded）。这类请求（如 /v1/images/edits、音频转写）无法携带
// JSON 路由策略，直接按默认策略处理，避免把 multipart 边界当 JSON 解析报错。
func requestBodyIsForm(c *gin.Context) bool {
	contentType := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))
	return strings.HasPrefix(contentType, "multipart/form-data") ||
		strings.HasPrefix(contentType, "application/x-www-form-urlencoded")
}

func Distribute() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if requestBodyIsForm(c) {
			c.Set(ctxkey.ProviderRoutingPolicy, routing.DefaultPolicy())
		} else if rawBody, bodyErr := common.GetRequestBody(c); bodyErr != nil {
			abortWithMessage(c, http.StatusBadRequest, "读取请求体失败")
			return
		} else if policy, policyErr := routing.ParseRequestPolicy(rawBody); policyErr != nil {
			c.Set(ctxkey.RelayErrorCode, "invalid_provider_routing_policy")
			abortWithMessage(c, http.StatusBadRequest, policyErr.Error())
			return
		} else {
			c.Set(ctxkey.ProviderRoutingPolicy, policy)
		}
		userId := c.GetString(ctxkey.Id)
		requestModel := c.GetString(ctxkey.RequestModel)
		userGroup := ""
		var entitlementSource *model.UserEntitlementSource
		resolveEntitlement := func() error {
			group, source, err := model.ResolveUserEntitlementGroupForModel(ctx, userId, requestModel)
			if err != nil {
				return err
			}
			userGroup = group
			entitlementSource = source
			c.Set(ctxkey.Group, userGroup)
			if entitlementSource != nil {
				c.Set(ctxkey.EntitlementSourceType, entitlementSource.SourceType)
				c.Set(ctxkey.EntitlementSourceId, entitlementSource.SourceID)
				c.Set(ctxkey.EntitlementSourceName, entitlementSource.SourceName)
			}
			return nil
		}
		abortEntitlementResolution := func(groupErr error) {
			statusCode := http.StatusServiceUnavailable
			errorCode := "request_aborted"
			reason := "entitlement_resolution_failed"
			var entitlementErr *model.EntitlementUnavailableError
			if errors.As(groupErr, &entitlementErr) {
				statusCode = http.StatusForbidden
				errorCode = "entitlement_unavailable"
				reason = "entitlement_unavailable"
				if entitlementErr.ModelUnavailable {
					c.Set(ctxkey.RelayModelUnavailable, true)
					c.Set(ctxkey.RelayErrorCode, "model_not_found")
					reason = "model_unavailable"
				}
			}
			if c.GetString(ctxkey.RelayErrorCode) == "" {
				c.Set(ctxkey.RelayErrorCode, errorCode)
			}
			logger.RelayWarnf(ctx, "DISTRIBUTE decision=abort reason=%s user_id=%s model=%s endpoint=%s status=%d error=%q", reason, userId, requestModel, c.Request.URL.Path, statusCode, groupErr.Error())
			abortWithMessage(c, statusCode, groupErr.Error())
		}
		var channel *model.Channel
		var err error
		channelId, ok := c.Get(ctxkey.SpecificChannelId)
		if ok {
			if groupErr := resolveEntitlement(); groupErr != nil {
				abortEntitlementResolution(groupErr)
				return
			}
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
			recordRouteDecision(c, "specific_channel", userGroup, requestModel, c.Request.URL.Path, []*model.Channel{channel}, nil, channel, "explicit")
		} else {
			personalPolicy := model.ResolvePersonalRoutePolicy(userId, &model.Token{RoutePolicy: c.GetString(ctxkey.PersonalRoutePolicy)}, requestModel)
			c.Set(ctxkey.PersonalRoutePolicy, personalPolicy)
			personalSupported := supportsPersonalProviderRoute(c.Request.URL.Path)
			// A stateful Responses request must retain the exact upstream account
			// that issued its response or tool item. It takes precedence over every
			// source policy, including personal_first and personal_only.
			if responseStateConflict(c) {
				if resetErr := resetConflictingResponsesState(c); resetErr != nil {
					abortWithMessage(c, http.StatusBadRequest, "Responses 会话状态恢复失败")
					return
				}
			}
			if pinnedChannelID, pinned, _ := lookupPinnedResponsesChannelID(c); pinned {
				if !model.IsPersonalProviderChannelID(pinnedChannelID) && !model.IsCommunityOfferChannelID(pinnedChannelID) {
					if groupErr := resolveEntitlement(); groupErr != nil {
						abortEntitlementResolution(groupErr)
						return
					}
				}
				channel, _ = selectPinnedResponsesChannel(c, userId, userGroup, requestModel, c.Request.URL.Path)
				if channel == nil {
					abortWithMessage(c, http.StatusConflict, "已绑定的 Responses 会话上游不可用，请重新开始会话")
					return
				}
			}
			selectPersonalChannel := func(decision string, reason string) (*model.Channel, error) {
				personalChannels, personalErr := model.ListPersonalProviderChannelsForModel(userId, requestModel)
				if personalErr != nil {
					logger.RelayWarnf(ctx, "DISTRIBUTE decision=personal_provider_unavailable user_id=%s model=%s endpoint=%s error=%q", userId, requestModel, c.Request.URL.Path, personalErr.Error())
					return nil, personalErr
				}
				selected := pickChannelByPriority(personalChannels, false)
				if selected != nil {
					recordRouteDecision(c, decision, userGroup, requestModel, c.Request.URL.Path, personalChannels, nil, selected, reason)
				}
				return selected, nil
			}
			// A community offer is never an automatic fallback candidate. A user
			// must select it explicitly for this model, after which the request is
			// fixed to that exact offer until the rule is removed or the offer is no
			// longer healthy.
			if channel == nil && personalSupported && personalPolicy != model.PersonalRoutePolicyPersonalOnly {
				route, routeErr := model.GetCommunityOfferModelRoute(userId, requestModel)
				if routeErr == nil {
					channel, err = model.ResolveCommunityOfferChannel(route.OfferID, requestModel, c.Request.URL.Path)
					if err != nil {
						logger.RelayWarnf(ctx, "DISTRIBUTE decision=abort reason=community_offer_unavailable user_id=%s offer_id=%s model=%s endpoint=%s error=%q", userId, route.OfferID, requestModel, c.Request.URL.Path, err.Error())
						abortWithMessage(c, http.StatusServiceUnavailable, "已选择的社区模型服务当前不可用，请在社区模型服务中重新选择")
						return
					}
					c.Set(ctxkey.CommunityOfferID, route.OfferID)
					recordRouteDecision(c, "community_offer", "", requestModel, c.Request.URL.Path, []*model.Channel{channel}, nil, channel, "explicit_offer")
				} else if !errors.Is(routeErr, gorm.ErrRecordNotFound) {
					logger.RelayWarnf(ctx, "DISTRIBUTE decision=community_offer_route_lookup_failed user_id=%s model=%s endpoint=%s error=%q", userId, requestModel, c.Request.URL.Path, routeErr.Error())
					abortWithMessage(c, http.StatusInternalServerError, "读取社区模型路由失败")
					return
				}
			}
			if channel == nil && personalSupported && (personalPolicy == model.PersonalRoutePolicyPersonalFirst || personalPolicy == model.PersonalRoutePolicyPersonalOnly) {
				channel, err = selectPersonalChannel("personal_provider", "personal_priority")
				if err != nil && personalPolicy == model.PersonalRoutePolicyPersonalOnly {
					abortWithMessage(c, http.StatusServiceUnavailable, "个人供应商凭据不可用，请在我的供应商中轮换凭据")
					return
				}
				if channel == nil && personalPolicy == model.PersonalRoutePolicyPersonalOnly {
					abortWithMessage(c, http.StatusServiceUnavailable, "该模型没有可用的个人供应商连接")
					return
				}
			}
			if channel == nil {
				if groupErr := resolveEntitlement(); groupErr != nil {
					if personalSupported && personalPolicy == model.PersonalRoutePolicyCommunityFirst {
						channel, err = selectPersonalChannel("personal_provider_fallback", "community_entitlement_unavailable")
					}
					if channel == nil {
						abortEntitlementResolution(groupErr)
						return
					}
				} else if channel, userGroup, entitlementSource, err = SelectEntitlementChannelForRequest(ctx, c, userId, userGroup, entitlementSource, requestModel); err != nil {
					if personalPolicy == model.PersonalRoutePolicyCommunityFirst && personalSupported {
						channel, err = selectPersonalChannel("personal_provider_fallback", "community_first_fallback")
					}
					if err != nil {
						statusCode := http.StatusServiceUnavailable
						message := err.Error()
						if strings.HasPrefix(message, "state_incompatible: ") {
							statusCode = http.StatusBadRequest
							message = strings.TrimPrefix(message, "state_incompatible: ")
							c.Set(ctxkey.RelayErrorType, "state_incompatible_error")
							c.Set(ctxkey.RelayErrorCode, "state_incompatible")
						}
						abortWithMessage(c, statusCode, message)
						return
					}
				}
			}
			c.Set(ctxkey.Group, userGroup)
			if entitlementSource != nil {
				c.Set(ctxkey.EntitlementSourceType, entitlementSource.SourceType)
				c.Set(ctxkey.EntitlementSourceId, entitlementSource.SourceID)
				c.Set(ctxkey.EntitlementSourceName, entitlementSource.SourceName)
			}
		}
		if !model.IsPersonalProviderChannelID(channel.Id) && tokenMonetaryQuotaExhausted(c) {
			c.Set(ctxkey.RelayErrorCode, "token_quota_exhausted")
			abortWithMessage(c, http.StatusForbidden, "该令牌额度已用尽")
			return
		}
		logger.Debugf(ctx, "user id %s, user group: %s, request model: %s, using channel #%s", userId, userGroup, requestModel, channel.Id)
		SetupContextForSelectedChannel(c, channel, requestModel)
		c.Next()
	}
}

// TokenAuth stores this snapshot before distribution. Enforce the monetary
// limit only after a community channel has been selected, because private
// provider requests intentionally do not spend Router monetary quota.
func tokenMonetaryQuotaExhausted(c *gin.Context) bool {
	if c == nil {
		return false
	}
	remaining, exists := c.Get(ctxkey.TokenRemainQuota)
	if !exists || c.GetBool(ctxkey.TokenUnlimitedQuota) {
		return false
	}
	value, ok := remaining.(int64)
	return ok && value <= 0
}

func SetupContextForSelectedChannel(c *gin.Context, channel *model.Channel, modelName string) {
	channelProtocol := channel.GetChannelProtocol()
	c.Set(ctxkey.Channel, channelProtocol)
	c.Set(ctxkey.ChannelId, channel.Id)
	c.Set(ctxkey.ChannelName, channel.DisplayName())
	if model.IsPersonalProviderChannelID(channel.Id) {
		c.Set(ctxkey.PersonalProviderID, model.PersonalProviderIDFromChannelID(channel.Id))
		c.Set(ctxkey.PersonalProviderName, channel.PersonalProviderName)
	} else {
		c.Set(ctxkey.PersonalProviderID, "")
		c.Set(ctxkey.PersonalProviderName, "")
	}
	if model.IsCommunityOfferChannelID(channel.Id) {
		c.Set(ctxkey.CommunityOfferID, model.CommunityOfferIDFromChannelID(channel.Id))
	} else {
		c.Set(ctxkey.CommunityOfferID, "")
	}
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

// Personal connections currently expose text-capable upstream protocols only.
// Keeping media, realtime, and async APIs on community channels prevents an
// in-flight resource from being transferred between unrelated provider accounts.
func supportsPersonalProviderRoute(requestPath string) bool {
	path := strings.TrimSpace(requestPath)
	return strings.HasSuffix(path, "/completions") ||
		strings.HasSuffix(path, "/messages") ||
		strings.HasSuffix(path, "/responses") ||
		strings.HasSuffix(path, "/embeddings") ||
		strings.HasSuffix(path, "/moderations")
}
