package controller

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	commonutils "github.com/yeying-community/router/common/utils"
	"github.com/yeying-community/router/internal/admin/model"
	relay "github.com/yeying-community/router/internal/relay"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

// coarse provider matcher to avoid misclassification when channels share the same API style
func modelBelongsToProvider(provider string, model string) bool {
	p := strings.ToLower(provider)
	m := strings.ToLower(model)
	switch p {
	case "openai", "openai-sb":
		return strings.HasPrefix(m, "gpt-") ||
			strings.HasPrefix(m, "o1") ||
			strings.HasPrefix(m, "o3") ||
			strings.HasPrefix(m, "chatgpt") ||
			strings.HasPrefix(m, "text-embedding") ||
			strings.HasPrefix(m, "whisper") ||
			strings.HasPrefix(m, "dall-") ||
			strings.HasPrefix(m, "tts-")
	case "anthropic":
		return strings.HasPrefix(m, "claude-")
	case "deepseek":
		return strings.HasPrefix(m, "deepseek-")
	case "gemini", "vertexai", "gemini-openai-compatible":
		return strings.HasPrefix(m, "gemini")
	default:
		// fallback: if provider name appears inside model string
		return strings.Contains(m, p)
	}
}

func filterModelsByProvider(models []string, provider string) []string {
	if provider == "" {
		return models
	}
	filtered := make([]string, 0, len(models))
	for _, modelName := range models {
		if commonutils.MatchProvider(modelName, "", provider) {
			filtered = append(filtered, modelName)
		}
	}
	return filtered
}

type OpenAIModelPermission struct {
	Id                 string  `json:"id"`
	Object             string  `json:"object"`
	Created            int     `json:"created"`
	AllowCreateEngine  bool    `json:"allow_create_engine"`
	AllowSampling      bool    `json:"allow_sampling"`
	AllowLogprobs      bool    `json:"allow_logprobs"`
	AllowSearchIndices bool    `json:"allow_search_indices"`
	AllowView          bool    `json:"allow_view"`
	AllowFineTuning    bool    `json:"allow_fine_tuning"`
	Organization       string  `json:"organization"`
	Group              *string `json:"group"`
	IsBlocking         bool    `json:"is_blocking"`
}

type OpenAIModels struct {
	Id                 string                            `json:"id"`
	Object             string                            `json:"object"`
	Created            int                               `json:"created"`
	OwnedBy            string                            `json:"owned_by"`
	Tags               []string                          `json:"tags"`
	Specification      *model.ProviderModelSpecification `json:"specification,omitempty"`
	SupportedEndpoints []string                          `json:"supported_endpoints"`
	Permission         []OpenAIModelPermission           `json:"permission"`
	Root               string                            `json:"root"`
	Parent             *string                           `json:"parent"`
}

type UserModelStatusPoint struct {
	State     string `json:"state"`
	ChannelID string `json:"channel_id,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
	TestedAt  int64  `json:"tested_at,omitempty"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Message   string `json:"message,omitempty"`
}

type UserModelStatusItem struct {
	Model               string                 `json:"model"`
	Provider            string                 `json:"provider"`
	Tags                []string               `json:"tags"`
	SupportedEndpoints  []string               `json:"supported_endpoints"`
	Status              string                 `json:"status"`
	HealthLevel         string                 `json:"health_level"`
	HealthScore         int                    `json:"health_score"`
	ChannelCount        int                    `json:"channel_count"`
	TestedChannelCount  int                    `json:"tested_channel_count"`
	TestedEndpointCount int                    `json:"tested_endpoint_count"`
	SupportedCount      int                    `json:"supported_count"`
	UnsupportedCount    int                    `json:"unsupported_count"`
	PassRate            float64                `json:"pass_rate"`
	AvgLatencyMs        int64                  `json:"avg_latency_ms"`
	LastTestedAt        int64                  `json:"last_tested_at"`
	HealthPoints        []UserModelStatusPoint `json:"health_points"`
}

type UserModelStatusSummary struct {
	ModelCount         int     `json:"model_count"`
	HealthyModelCount  int     `json:"healthy_model_count"`
	WarningModelCount  int     `json:"warning_model_count"`
	CriticalModelCount int     `json:"critical_model_count"`
	UnknownModelCount  int     `json:"unknown_model_count"`
	AvgPassRate        float64 `json:"avg_pass_rate"`
	AvgLatencyMs       int64   `json:"avg_latency_ms"`
}

type UserModelStatusPayload struct {
	Group       string                 `json:"group"`
	Summary     UserModelStatusSummary `json:"summary"`
	Models      []UserModelStatusItem  `json:"models"`
	GeneratedAt int64                  `json:"generated_at"`
}

var defaultModelPermissions = []OpenAIModelPermission{
	{
		Id:                 "modelperm-LwHkVFn8AcMItP432fKKDIKJ",
		Object:             "model_permission",
		Created:            1626777600,
		AllowCreateEngine:  true,
		AllowSampling:      true,
		AllowLogprobs:      true,
		AllowSearchIndices: false,
		AllowView:          true,
		AllowFineTuning:    false,
		Organization:       "*",
		Group:              nil,
		IsBlocking:         false,
	},
}

var loadGroupModelProvidersFn = model.ListGroupModelProviderMapByModels
var loadGroupModelSupportedEndpointsFn = listGroupModelSupportedEndpoints
var loadProviderModelTagsFn = model.LoadProviderModelTagMapByModelsWithDB
var loadProviderModelSpecificationsFn = model.LoadProviderModelSpecificationMapByModelsWithDB
var loadProviderProtocolModelsFn = loadDashboardProtocolModels
var loadSatisfiedChannelsFn = model.CacheListSatisfiedChannels

const (
	userModelHealthLevelHealthy  = "healthy"
	userModelHealthLevelWarning  = "warning"
	userModelHealthLevelCritical = "critical"
	userModelHealthLevelUnknown  = "unknown"
	userModelStatusNormal        = "normal"
	userModelStatusWarning       = "warning"
	userModelStatusUnsupported   = "unsupported"
	userModelStatusUnknown       = "unknown"
	userModelStatusPointSuccess  = "success"
	userModelStatusPointWarning  = "warning"
	userModelStatusPointFailure  = "failure"
	userModelStatusPointUnknown  = "unknown"
)

var endpointSortOrder = map[string]int{
	model.ChannelModelEndpointChat:      10,
	model.ChannelModelEndpointResponses: 20,
	model.ChannelModelEndpointMessages:  30,
	model.ChannelModelEndpointRealtime:  35,
	model.ChannelModelEndpointImages:    40,
	model.ChannelModelEndpointImageEdit: 50,
	model.ChannelModelEndpointBatches:   60,
	model.ChannelModelEndpointAudio:     70,
	model.ChannelModelEndpointVideos:    80,
}

func sortModelEndpoints(endpoints []string) []string {
	normalized := model.NormalizeChannelModelIDsPreserveOrder(endpoints)
	if len(normalized) == 0 {
		return []string{}
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		left := normalized[i]
		right := normalized[j]
		leftOrder, leftOk := endpointSortOrder[left]
		rightOrder, rightOk := endpointSortOrder[right]
		switch {
		case leftOk && rightOk && leftOrder != rightOrder:
			return leftOrder < rightOrder
		case leftOk && !rightOk:
			return true
		case !leftOk && rightOk:
			return false
		default:
			return left < right
		}
	})
	return normalized
}

func listGroupModelSupportedEndpoints(groupID string, modelNames []string) (map[string][]string, error) {
	normalizedGroupID := strings.TrimSpace(groupID)
	result := make(map[string][]string)
	if normalizedGroupID == "" {
		return result, nil
	}
	normalizedModels := model.NormalizeChannelModelIDsPreserveOrder(modelNames)
	for _, modelName := range normalizedModels {
		if strings.TrimSpace(modelName) == "" {
			continue
		}
		channels, err := model.CacheListSatisfiedChannels(normalizedGroupID, modelName)
		if err != nil {
			// Keep provider mapping strict, but endpoint metadata should not block /v1/models.
			continue
		}
		endpointSet := make(map[string]struct{})
		for _, channel := range channels {
			if channel == nil {
				continue
			}
			channelID := strings.TrimSpace(channel.Id)
			if channelID == "" {
				continue
			}
			enabledMap := model.CacheGetChannelModelEndpointSupport(channel.Id, modelName)
			if len(enabledMap) == 0 {
				continue
			}
			for endpoint, enabled := range enabledMap {
				if !enabled {
					continue
				}
				normalizedEndpoint := model.NormalizeRequestedChannelModelEndpoint(endpoint)
				if normalizedEndpoint == "" {
					continue
				}
				endpointSet[normalizedEndpoint] = struct{}{}
			}
		}
		if len(endpointSet) == 0 {
			continue
		}
		endpoints := make([]string, 0, len(endpointSet))
		for endpoint := range endpointSet {
			endpoints = append(endpoints, endpoint)
		}
		result[modelName] = sortModelEndpoints(endpoints)
	}
	return result, nil
}

func cloneDefaultModelPermissions() []OpenAIModelPermission {
	permissions := make([]OpenAIModelPermission, len(defaultModelPermissions))
	copy(permissions, defaultModelPermissions)
	return permissions
}

func resolveOwnedByFromProvider(provider string) (string, error) {
	normalizedProvider := commonutils.NormalizeProvider(provider)
	if normalizedProvider == "" || normalizedProvider == "custom" {
		return "", fmt.Errorf("missing canonical provider mapping")
	}
	return normalizedProvider, nil
}

func resolveProviderForDashboardProtocol(channelProtocol int) string {
	if channelProtocol <= 0 || channelProtocol >= len(relaychannel.ChannelProtocolNames) {
		return ""
	}
	switch relaychannel.NormalizeProtocolName(relaychannel.ChannelProtocolNames[channelProtocol]) {
	case "openai", "anthropic", "zhipu", "ali", "gemini", "moonshot", "baichuan", "minimax", "mistral", "groq", "lingyiwanwu", "stepfun", "cohere", "deepseek", "togetherai", "volcengine", "novita", "siliconflow", "xai", "baidu-v2", "gemini-openai-compatible":
		return commonutils.NormalizeProvider(relaychannel.ChannelProtocolNames[channelProtocol])
	default:
		return ""
	}
}

func loadDashboardProtocolModels(channelProtocol int) ([]string, error) {
	provider := resolveProviderForDashboardProtocol(channelProtocol)
	if provider != "" {
		return model.ListActiveProviderModelsWithDB(model.DB, provider)
	}
	adaptor := relay.GetAdaptor(relaychannel.ToAPIType(channelProtocol))
	meta := &meta.Meta{
		ChannelProtocol: channelProtocol,
	}
	adaptor.Init(meta)
	return adaptor.GetModelList(), nil
}

func buildDashboardChannelModelMap() map[int][]string {
	result := make(map[int][]string)
	for i := 1; i < relaychannel.Dummy; i++ {
		if i == relaychannel.OpenAICompatible {
			continue
		}
		models, err := loadProviderProtocolModelsFn(i)
		if err != nil {
			models = []string{}
		}
		result[i] = models
	}
	return result
}

func resolveRequestAvailableModels(c *gin.Context) ([]string, string, error) {
	userID := strings.TrimSpace(c.GetString(ctxkey.Id))
	availableModelsRaw := strings.TrimSpace(c.GetString(ctxkey.AvailableModels))
	if availableModelsRaw != "" {
		modelNames := model.NormalizeChannelModelIDsPreserveOrder(strings.Split(availableModelsRaw, ","))
		if userID == "" {
			return modelNames, "", nil
		}
		userGroup, err := model.CacheGetUserGroup(userID)
		if err != nil {
			return modelNames, "", nil
		}
		return modelNames, userGroup, nil
	}
	if userID == "" {
		return []string{}, "", nil
	}
	userGroup, err := model.CacheGetUserGroup(userID)
	if err != nil {
		return nil, "", err
	}
	availableModels, err := model.CacheGetGroupModels(c.Request.Context(), userGroup)
	if err != nil {
		return nil, "", err
	}
	return model.NormalizeChannelModelIDsPreserveOrder(availableModels), userGroup, nil
}

func buildOpenAIModelsForRequest(c *gin.Context) ([]OpenAIModels, map[string]OpenAIModels, error) {
	modelNames, userGroup, err := resolveRequestAvailableModels(c)
	if err != nil {
		return nil, nil, err
	}
	providerByModel, err := loadGroupModelProvidersFn(userGroup, modelNames)
	if err != nil {
		return nil, nil, err
	}
	endpointsByModel, err := loadGroupModelSupportedEndpointsFn(userGroup, modelNames)
	if err != nil {
		return nil, nil, err
	}
	tagsByModel, err := loadProviderModelTagsFn(model.DB, providerByModel, modelNames)
	if err != nil {
		return nil, nil, err
	}
	specificationsByModel, err := loadProviderModelSpecificationsFn(model.DB, providerByModel, modelNames)
	if err != nil {
		return nil, nil, err
	}
	items := make([]OpenAIModels, 0, len(modelNames))
	itemMap := make(map[string]OpenAIModels, len(modelNames))
	missingProviderModels := make([]string, 0)
	for _, modelName := range modelNames {
		ownedBy, resolveErr := resolveOwnedByFromProvider(providerByModel[modelName])
		if resolveErr != nil {
			missingProviderModels = append(missingProviderModels, modelName)
			continue
		}
		item := OpenAIModels{
			Id:                 modelName,
			Object:             "model",
			Created:            1626777600,
			OwnedBy:            ownedBy,
			Tags:               tagsByModel[modelName],
			Specification:      specificationsByModel[modelName],
			SupportedEndpoints: sortModelEndpoints(endpointsByModel[modelName]),
			Permission:         cloneDefaultModelPermissions(),
			Root:               modelName,
			Parent:             nil,
		}
		items = append(items, item)
		itemMap[modelName] = item
	}
	if len(missingProviderModels) > 0 {
		return nil, nil, fmt.Errorf("provider mapping missing for group=%s models=%s", strings.TrimSpace(userGroup), strings.Join(missingProviderModels, ","))
	}
	return items, itemMap, nil
}

func clampUserModelStatusRate(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func userModelHealthLevelByScore(score int) string {
	switch {
	case score >= 85:
		return userModelHealthLevelHealthy
	case score >= 65:
		return userModelHealthLevelWarning
	case score > 0:
		return userModelHealthLevelCritical
	default:
		return userModelHealthLevelUnknown
	}
}

func calcUserModelStatus(item *UserModelStatusItem) {
	if item == nil {
		return
	}
	score := 100.0
	if item.ChannelCount > 0 {
		coverageRate := float64(item.TestedChannelCount) / float64(item.ChannelCount)
		score -= (1 - clampUserModelStatusRate(coverageRate)) * 30
	} else {
		score -= 25
	}
	assertCount := item.SupportedCount + item.UnsupportedCount
	if assertCount > 0 {
		item.PassRate = clampUserModelStatusRate(float64(item.SupportedCount) / float64(assertCount))
		score -= (1 - item.PassRate) * 30
	} else {
		score -= 20
	}
	switch {
	case item.AvgLatencyMs >= 30000:
		score -= 20
	case item.AvgLatencyMs >= 15000:
		score -= 14
	case item.AvgLatencyMs >= 8000:
		score -= 8
	case item.AvgLatencyMs >= 3000:
		score -= 4
	default:
		if item.AvgLatencyMs <= 0 {
			score -= 6
		}
	}
	if item.LastTestedAt <= 0 {
		score -= 12
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	item.HealthScore = int(score + 0.5)
	item.HealthLevel = userModelHealthLevelByScore(item.HealthScore)
	switch item.HealthLevel {
	case userModelHealthLevelHealthy:
		item.Status = userModelStatusNormal
	case userModelHealthLevelWarning:
		item.Status = userModelStatusWarning
	case userModelHealthLevelCritical:
		item.Status = userModelStatusUnsupported
	default:
		item.Status = userModelStatusUnknown
	}
}

func normalizeUserModelStatusPoint(row model.ChannelTest) UserModelStatusPoint {
	state := userModelStatusPointFailure
	switch model.NormalizeChannelTestStatus(row.Status) {
	case model.ChannelTestStatusSupported:
		if row.Supported {
			state = userModelStatusPointSuccess
		}
	case model.ChannelTestStatusSkipped:
		state = userModelStatusPointWarning
	default:
		state = userModelStatusPointFailure
	}
	return UserModelStatusPoint{
		State:     state,
		ChannelID: strings.TrimSpace(row.ChannelId),
		Endpoint:  model.NormalizeRequestedChannelModelEndpoint(row.Endpoint),
		TestedAt:  row.TestedAt,
		LatencyMs: row.LatencyMs,
		Message:   strings.TrimSpace(row.Message),
	}
}

func loadUserModelStatusTestRows(channelIDs []string, modelNames []string) (map[string][]model.ChannelTest, error) {
	result := make(map[string][]model.ChannelTest)
	normalizedChannelIDs := model.NormalizeChannelModelIDsPreserveOrder(channelIDs)
	normalizedModels := model.NormalizeChannelModelIDsPreserveOrder(modelNames)
	if len(normalizedChannelIDs) == 0 || len(normalizedModels) == 0 {
		return result, nil
	}
	rows := make([]model.ChannelTest, 0)
	if err := model.DB.Model(&model.ChannelTest{}).
		Where("channel_id IN ? AND model IN ?", normalizedChannelIDs, normalizedModels).
		Order("tested_at desc, round desc, sort_order asc, endpoint asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	normalizedRows := model.NormalizeChannelTestRows(rows)
	sort.SliceStable(normalizedRows, func(i, j int) bool {
		if normalizedRows[i].Model != normalizedRows[j].Model {
			return normalizedRows[i].Model < normalizedRows[j].Model
		}
		if normalizedRows[i].TestedAt != normalizedRows[j].TestedAt {
			return normalizedRows[i].TestedAt > normalizedRows[j].TestedAt
		}
		if normalizedRows[i].Round != normalizedRows[j].Round {
			return normalizedRows[i].Round > normalizedRows[j].Round
		}
		return normalizedRows[i].Endpoint < normalizedRows[j].Endpoint
	})
	for _, row := range normalizedRows {
		modelName := strings.TrimSpace(row.Model)
		if modelName == "" {
			continue
		}
		result[modelName] = append(result[modelName], row)
	}
	return result, nil
}

func buildUserModelStatusPayload(c *gin.Context) (UserModelStatusPayload, error) {
	modelNames, userGroup, err := resolveRequestAvailableModels(c)
	if err != nil {
		return UserModelStatusPayload{}, err
	}
	providerByModel, err := loadGroupModelProvidersFn(userGroup, modelNames)
	if err != nil {
		return UserModelStatusPayload{}, err
	}
	endpointsByModel, err := loadGroupModelSupportedEndpointsFn(userGroup, modelNames)
	if err != nil {
		return UserModelStatusPayload{}, err
	}
	tagsByModel, err := loadProviderModelTagsFn(model.DB, providerByModel, modelNames)
	if err != nil {
		return UserModelStatusPayload{}, err
	}

	channelIDsByModel := make(map[string][]string, len(modelNames))
	channelIDSetByModel := make(map[string]map[string]struct{}, len(modelNames))
	testModelCandidatesByModel := make(map[string][]string, len(modelNames))
	channelIDs := make([]string, 0)
	seenChannelIDs := make(map[string]struct{})
	for _, modelName := range modelNames {
		testModelCandidatesByModel[modelName] = append(testModelCandidatesByModel[modelName], modelName)
		channelIDSetByModel[modelName] = make(map[string]struct{})
		channels, listErr := loadSatisfiedChannelsFn(userGroup, modelName)
		if listErr != nil {
			continue
		}
		for _, channel := range channels {
			if channel == nil {
				continue
			}
			channelID := strings.TrimSpace(channel.Id)
			if channelID == "" {
				continue
			}
			channelIDsByModel[modelName] = append(channelIDsByModel[modelName], channelID)
			channelIDSetByModel[modelName][channelID] = struct{}{}
			if mapping := model.CacheGetGroupModelMapping(userGroup, modelName, channelID); len(mapping) > 0 {
				if upstream := strings.TrimSpace(mapping[modelName]); upstream != "" {
					testModelCandidatesByModel[modelName] = append(testModelCandidatesByModel[modelName], upstream)
				}
			}
			if _, ok := seenChannelIDs[channelID]; !ok {
				seenChannelIDs[channelID] = struct{}{}
				channelIDs = append(channelIDs, channelID)
			}
		}
	}

	testModelNames := make([]string, 0, len(modelNames))
	for _, modelName := range modelNames {
		testModelNames = append(testModelNames, testModelCandidatesByModel[modelName]...)
	}
	testModelNames = model.NormalizeChannelModelIDsPreserveOrder(testModelNames)
	testsByModel, err := loadUserModelStatusTestRows(channelIDs, testModelNames)
	if err != nil {
		return UserModelStatusPayload{}, err
	}

	items := make([]UserModelStatusItem, 0, len(modelNames))
	for _, modelName := range modelNames {
		latencyTotal := int64(0)
		latencyCount := int64(0)
		testedChannels := make(map[string]struct{})
		testedEndpoints := make(map[string]struct{})
		item := UserModelStatusItem{
			Model:              modelName,
			Provider:           model.ResolveProviderFromModelMap(providerByModel, modelName),
			Tags:               tagsByModel[modelName],
			SupportedEndpoints: sortModelEndpoints(endpointsByModel[modelName]),
			ChannelCount:       len(model.NormalizeChannelModelIDsPreserveOrder(channelIDsByModel[modelName])),
			HealthLevel:        userModelHealthLevelUnknown,
			Status:             userModelStatusUnknown,
			HealthPoints:       []UserModelStatusPoint{},
		}
		modelTestRows := make([]model.ChannelTest, 0)
		for _, testModelName := range model.NormalizeChannelModelIDsPreserveOrder(testModelCandidatesByModel[modelName]) {
			for _, row := range testsByModel[testModelName] {
				if _, ok := channelIDSetByModel[modelName][strings.TrimSpace(row.ChannelId)]; !ok {
					continue
				}
				modelTestRows = append(modelTestRows, row)
			}
		}
		sort.SliceStable(modelTestRows, func(i, j int) bool {
			if modelTestRows[i].TestedAt != modelTestRows[j].TestedAt {
				return modelTestRows[i].TestedAt > modelTestRows[j].TestedAt
			}
			if modelTestRows[i].Round != modelTestRows[j].Round {
				return modelTestRows[i].Round > modelTestRows[j].Round
			}
			return modelTestRows[i].Endpoint < modelTestRows[j].Endpoint
		})
		for _, row := range modelTestRows {
			channelID := strings.TrimSpace(row.ChannelId)
			if channelID != "" {
				testedChannels[channelID] = struct{}{}
			}
			endpoint := model.NormalizeRequestedChannelModelEndpoint(row.Endpoint)
			if endpoint != "" {
				testedEndpoints[endpoint] = struct{}{}
			}
			item.TestedEndpointCount++
			if row.TestedAt > item.LastTestedAt {
				item.LastTestedAt = row.TestedAt
			}
			switch model.NormalizeChannelTestStatus(row.Status) {
			case model.ChannelTestStatusSupported:
				if row.Supported {
					item.SupportedCount++
				} else {
					item.UnsupportedCount++
				}
			case model.ChannelTestStatusSkipped:
				// Skipped tests are surfaced as warning points but should not count
				// as hard unsupported assertions.
			default:
				item.UnsupportedCount++
			}
			if row.LatencyMs > 0 {
				latencyTotal += row.LatencyMs
				latencyCount++
			}
			item.HealthPoints = append(item.HealthPoints, normalizeUserModelStatusPoint(row))
		}
		if latencyCount > 0 {
			item.AvgLatencyMs = latencyTotal / latencyCount
		}
		item.TestedChannelCount = len(testedChannels)
		item.TestedEndpointCount = len(testedEndpoints)
		if len(item.HealthPoints) > 60 {
			item.HealthPoints = item.HealthPoints[:60]
		}
		calcUserModelStatus(&item)
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].HealthScore != items[j].HealthScore {
			return items[i].HealthScore > items[j].HealthScore
		}
		if items[i].ChannelCount != items[j].ChannelCount {
			return items[i].ChannelCount > items[j].ChannelCount
		}
		return items[i].Model < items[j].Model
	})

	summary := UserModelStatusSummary{ModelCount: len(items)}
	passRateTotal := 0.0
	latencyTotal := int64(0)
	latencyCount := int64(0)
	for _, item := range items {
		passRateTotal += item.PassRate
		if item.AvgLatencyMs > 0 {
			latencyTotal += item.AvgLatencyMs
			latencyCount++
		}
		switch item.HealthLevel {
		case userModelHealthLevelHealthy:
			summary.HealthyModelCount++
		case userModelHealthLevelWarning:
			summary.WarningModelCount++
		case userModelHealthLevelCritical:
			summary.CriticalModelCount++
		default:
			summary.UnknownModelCount++
		}
	}
	if len(items) > 0 {
		summary.AvgPassRate = passRateTotal / float64(len(items))
	}
	if latencyCount > 0 {
		summary.AvgLatencyMs = latencyTotal / latencyCount
	}

	return UserModelStatusPayload{
		Group:       userGroup,
		Summary:     summary,
		Models:      items,
		GeneratedAt: helper.GetTimestamp(),
	}, nil
}

func DashboardListModels(c *gin.Context) {
	// optional filter: channel name, case-insensitive
	channelFilter := strings.ToLower(strings.TrimSpace(c.Query("channel")))
	providerFilter := commonutils.NormalizeProvider(c.Query("provider"))
	channelModelMap := buildDashboardChannelModelMap()

	// Keep the established map-shaped payload and include metadata for the admin UI.
	metaList := make([]gin.H, 0, len(channelModelMap))
	filteredMap := make(map[int][]string, len(channelModelMap))
	for id, models := range channelModelMap {
		name := ""
		if id >= 0 && id < len(relaychannel.ChannelProtocolNames) {
			name = relaychannel.ChannelProtocolNames[id]
		}
		filteredModels := filterModelsByProvider(models, providerFilter)
		metaList = append(metaList, gin.H{
			"id":     id,
			"name":   name,
			"models": filteredModels,
		})
		// if channel is specified and matches, short-circuit with filtered payload
		if channelFilter != "" && strings.ToLower(name) == channelFilter {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "",
				"channel": name,
				"id":      id,
				"data":    filteredModels,
				"meta": gin.H{
					"id":     id,
					"name":   name,
					"models": filteredModels,
				},
			})
			return
		}
		filteredMap[id] = filteredModels
	}

	if channelFilter != "" {
		// channel specified but not found
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("channel '%s' not found", channelFilter),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    filteredMap,
		"meta":    metaList,
	})
}

func ListModels(c *gin.Context) {
	availableOpenAIModels, _, err := buildOpenAIModelsForRequest(c)
	if err != nil {
		errorBody := relaymodel.Error{
			Message: err.Error(),
			Type:    "invalid_request_error",
			Param:   "group_models.provider",
			Code:    "provider_mapping_missing",
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": errorBody,
		})
		return
	}
	c.JSON(200, gin.H{
		"object": "list",
		"data":   availableOpenAIModels,
	})
}

func RetrieveModel(c *gin.Context) {
	modelId := c.Param("model")
	_, modelMap, err := buildOpenAIModelsForRequest(c)
	if err != nil {
		errorBody := relaymodel.Error{
			Message: err.Error(),
			Type:    "invalid_request_error",
			Param:   "group_models.provider",
			Code:    "provider_mapping_missing",
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": errorBody,
		})
		return
	}
	if item, ok := modelMap[modelId]; ok {
		c.JSON(200, item)
		return
	}
	Error := relaymodel.Error{
		Message: fmt.Sprintf("The model '%s' does not exist", modelId),
		Type:    "invalid_request_error",
		Param:   "model",
		Code:    "model_not_found",
	}
	c.JSON(200, gin.H{
		"error": Error,
	})
}

func GetUserAvailableModels(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.GetString(ctxkey.Id)
	payload, err := model.BuildUserEntitlementModels(ctx, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// optional filter by provider (channel) name, case-insensitive
	urlValues := c.Request.URL.Query()
	_, providerSpecified := urlValues["provider"]
	provider := strings.ToLower(strings.TrimSpace(urlValues.Get("provider")))
	if !providerSpecified {
		// Preserve the original list payload when no provider filter is specified.
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    payload.Models,
			"items":   payload.Items,
			"sources": payload.Sources,
		})
		return
	}

	filtered := make([]string, 0)
	for _, item := range payload.Items {
		modelName := strings.TrimSpace(item.Model)
		for _, source := range item.Sources {
			ch, err := model.GetTopChannelByModel(source.GroupID, modelName)
			if err != nil {
				continue
			}
			name := ch.GetProtocol()
			if strings.ToLower(name) == provider && modelBelongsToProvider(provider, modelName) {
				filtered = append(filtered, modelName)
				break
			}
			// If the channel protocol is configured as generic OpenAI,
			// still allow provider classification by model name.
			if modelBelongsToProvider(provider, modelName) {
				filtered = append(filtered, modelName)
				break
			}
		}
	}
	if len(filtered) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("provider '%s' not found or has no available models", provider),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "",
		"provider": provider,
		"data":     filtered,
	})
}

func GetUserModelStatus(c *gin.Context) {
	payload, err := buildUserModelStatusPayload(c)
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
		"data":    payload,
	})
}
