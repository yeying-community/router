package controller

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
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

// https://platform.openai.com/docs/api-reference/models/list

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
	Id                 string                  `json:"id"`
	Object             string                  `json:"object"`
	Created            int                     `json:"created"`
	OwnedBy            string                  `json:"owned_by"`
	SupportedEndpoints []string                `json:"supported_endpoints"`
	Permission         []OpenAIModelPermission `json:"permission"`
	Root               string                  `json:"root"`
	Parent             *string                 `json:"parent"`
}

var channelId2Models map[int][]string
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

func init() {
	channelId2Models = make(map[int][]string)
	for i := 1; i < relaychannel.Dummy; i++ {
		if i == relaychannel.OpenAICompatible {
			continue
		}
		adaptor := relay.GetAdaptor(relaychannel.ToAPIType(i))
		meta := &meta.Meta{
			ChannelProtocol: i,
		}
		adaptor.Init(meta)
		channelId2Models[i] = adaptor.GetModelList()
	}
}

var loadGroupModelProvidersFn = model.ListGroupModelProviderMapByModels
var loadGroupModelSupportedEndpointsFn = listGroupModelSupportedEndpoints

var endpointSortOrder = map[string]int{
	model.ChannelModelEndpointChat:      10,
	model.ChannelModelEndpointResponses: 20,
	model.ChannelModelEndpointMessages:  30,
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
		channelIDs := make([]string, 0, len(channels))
		for _, channel := range channels {
			if channel == nil {
				continue
			}
			channelID := strings.TrimSpace(channel.Id)
			if channelID == "" {
				continue
			}
			channelIDs = append(channelIDs, channelID)
		}
		supportedByChannelID, err := model.ListLatestChannelTestSupportByChannelIDsWithDB(
			model.DB,
			channelIDs,
			[]string{modelName},
		)
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
			channelModelSupportMap := supportedByChannelID[channelID][modelName]
			for endpoint, enabled := range enabledMap {
				if !enabled {
					continue
				}
				normalizedEndpoint := model.NormalizeRequestedChannelModelEndpoint(endpoint)
				if normalizedEndpoint == "" {
					continue
				}
				// /v1/models exposure rule:
				// - If latest test exists for an endpoint, it must be supported.
				// - If no latest test exists, fallback to enabled endpoint config.
				if supported, ok := channelModelSupportMap[normalizedEndpoint]; ok && !supported {
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

// DashboardListModels godoc
// @Summary List channel models for UI
// @Description When channel is specified, the response shape becomes docs.ChannelModelsProviderResponse (data is string[] and meta is an object). provider filters by model naming rules.
// @Tags public
// @Security BearerAuth
// @Produce json
// @Param channel query string false "Channel name"
// @Param provider query string false "Provider filter (gpt/gemini/claude/deepseek/qwen)"
// @Success 200 {object} docs.ChannelModelsResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/channel/models [get]
func DashboardListModels(c *gin.Context) {
	// optional filter: channel name, case-insensitive
	channelFilter := strings.ToLower(strings.TrimSpace(c.Query("channel")))
	providerFilter := commonutils.NormalizeProvider(c.Query("provider"))

	// Keep the established map-shaped payload and include metadata for the admin UI.
	metaList := make([]gin.H, 0, len(channelId2Models))
	filteredMap := make(map[int][]string, len(channelId2Models))
	for id, models := range channelId2Models {
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

// ListModels godoc
// @Summary List available OpenAI-compatible models
// @Tags public
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.OpenAIModelListResponse
// @Failure 401 {object} docs.OpenAIErrorResponse
// @Router /api/v1/public/models [get]
func ListModels(c *gin.Context) {
	availableOpenAIModels, _, err := buildOpenAIModelsForRequest(c)
	if err != nil {
		errorBody := relaymodel.Error{
			Message: err.Error(),
			Type:    "invalid_request_error",
			Param:   "group_model_providers",
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

// RetrieveModel godoc
// @Summary Retrieve model detail (OpenAI compatible)
// @Tags public
// @Security BearerAuth
// @Produce json
// @Param model path string true "Model ID"
// @Success 200 {object} docs.OpenAIModel
// @Failure 404 {object} docs.OpenAIErrorResponse
// @Router /api/v1/public/models/{model} [get]
func RetrieveModel(c *gin.Context) {
	modelId := c.Param("model")
	_, modelMap, err := buildOpenAIModelsForRequest(c)
	if err != nil {
		errorBody := relaymodel.Error{
			Message: err.Error(),
			Type:    "invalid_request_error",
			Param:   "group_model_providers",
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

// GetUserAvailableModels godoc
// @Summary List available models for current user
// @Tags public
// @Security BearerAuth
// @Produce json
// @Param provider query string false "Provider name"
// @Success 200 {object} docs.UserAvailableModelsResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/user/available_models [get]
func GetUserAvailableModels(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.GetString(ctxkey.Id)
	userGroup, err := model.CacheGetUserGroup(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	models, err := model.CacheGetGroupModels(ctx, userGroup)
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
			"data":    models,
		})
		return
	}

	filtered := make([]string, 0)
	for _, m := range models {
		ch, err := model.GetTopChannelByModel(userGroup, m)
		if err != nil {
			continue
		}
		name := ch.GetProtocol()
		if strings.ToLower(name) == provider && modelBelongsToProvider(provider, m) {
			filtered = append(filtered, m)
			continue
		}
		// If the channel protocol is configured as generic OpenAI,
		// still allow provider classification by model name.
		if modelBelongsToProvider(provider, m) {
			filtered = append(filtered, m)
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
