package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	commonutils "github.com/yeying-community/router/common/utils"
	"github.com/yeying-community/router/internal/admin/model"
	relay "github.com/yeying-community/router/internal/relay"
	"github.com/yeying-community/router/internal/relay/adaptor/openai"
	"github.com/yeying-community/router/internal/relay/apitype"
	"github.com/yeying-community/router/internal/relay/channeltype"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

// coarse provider matcher to avoid misclassification when channels share the same API style
func modelBelongsToProvider(provider string, model string) bool {
	p := strings.ToLower(provider)
	m := strings.ToLower(model)
	switch p {
	case "openai", "openai-compatible", "openai-sb":
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

func normalizeModelProviderFilter(provider string) string {
	trimmed := strings.TrimSpace(provider)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	switch lower {
	case "gpt", "openai":
		return "openai"
	case "gemini", "google":
		return "google"
	case "claude", "anthropic":
		return "anthropic"
	case "deepseek":
		return "deepseek"
	case "qwen", "qwq", "qvq", "千问":
		return "qwen"
	default:
		return lower
	}
}

func filterModelsByProvider(models []string, provider string) []string {
	if provider == "" {
		return models
	}
	filtered := make([]string, 0, len(models))
	for _, modelName := range models {
		if commonutils.ResolveModelProvider(modelName) == provider {
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
	Id         string                  `json:"id"`
	Object     string                  `json:"object"`
	Created    int                     `json:"created"`
	OwnedBy    string                  `json:"owned_by"`
	Permission []OpenAIModelPermission `json:"permission"`
	Root       string                  `json:"root"`
	Parent     *string                 `json:"parent"`
}

var models []OpenAIModels
var modelsMap map[string]OpenAIModels
var channelId2Models map[int][]string

func init() {
	var permission []OpenAIModelPermission
	permission = append(permission, OpenAIModelPermission{
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
	})
	// https://platform.openai.com/docs/models/model-endpoint-compatibility
	for i := 0; i < apitype.Dummy; i++ {
		if i == apitype.AIProxyLibrary {
			continue
		}
		adaptor := relay.GetAdaptor(i)
		channelName := adaptor.GetChannelName()
		modelNames := adaptor.GetModelList()
		for _, modelName := range modelNames {
			models = append(models, OpenAIModels{
				Id:         modelName,
				Object:     "model",
				Created:    1626777600,
				OwnedBy:    channelName,
				Permission: permission,
				Root:       modelName,
				Parent:     nil,
			})
		}
	}
	for _, channelType := range openai.CompatibleChannels {
		if channelType == channeltype.Azure {
			continue
		}
		channelName, channelModelList := openai.GetCompatibleChannelMeta(channelType)
		for _, modelName := range channelModelList {
			models = append(models, OpenAIModels{
				Id:         modelName,
				Object:     "model",
				Created:    1626777600,
				OwnedBy:    channelName,
				Permission: permission,
				Root:       modelName,
				Parent:     nil,
			})
		}
	}
	modelsMap = make(map[string]OpenAIModels)
	for _, model := range models {
		modelsMap[model.Id] = model
	}
	channelId2Models = make(map[int][]string)
	for i := 1; i < channeltype.Dummy; i++ {
		adaptor := relay.GetAdaptor(channeltype.ToAPIType(i))
		meta := &meta.Meta{
			ChannelType: i,
		}
		adaptor.Init(meta)
		channelId2Models[i] = adaptor.GetModelList()
	}
}

// DashboardListModels godoc
// @Summary List channel models for UI
// @Description When provider is specified, the response shape becomes docs.ChannelModelsProviderResponse (data is string[] and meta is an object). model_provider filters by model naming rules.
// @Tags public
// @Security BearerAuth
// @Produce json
// @Param provider query string false "Provider name"
// @Param model_provider query string false "Model provider filter (gpt/gemini/claude/deepseek/qwen)"
// @Success 200 {object} docs.ChannelModelsResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/channel/models [get]
func DashboardListModels(c *gin.Context) {
	// optional filter: provider (channel) name, case-insensitive
	provider := strings.ToLower(strings.TrimSpace(c.Query("provider")))
	modelProvider := normalizeModelProviderFilter(c.Query("model_provider"))

	// backward compatibility: keep original map response, and add metadata list for UI friendliness
	metaList := make([]gin.H, 0, len(channelId2Models))
	filteredMap := make(map[int][]string, len(channelId2Models))
	for id, models := range channelId2Models {
		name := ""
		if id >= 0 && id < len(channeltype.ChannelTypeNames) {
			name = channeltype.ChannelTypeNames[id]
		}
		filteredModels := filterModelsByProvider(models, modelProvider)
		metaList = append(metaList, gin.H{
			"id":     id,
			"name":   name,
			"models": filteredModels,
		})
		// if provider is specified and matches, short‑circuit with filtered payload
		if provider != "" && strings.ToLower(name) == provider {
			c.JSON(http.StatusOK, gin.H{
				"success":  true,
				"message":  "",
				"provider": name,
				"id":       id,
				"data":     filteredModels,
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

	if provider != "" {
		// provider specified but not found
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("provider '%s' not found", provider),
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

// ListAllModels godoc
// @Summary List all models (non OpenAI-compatible)
// @Tags public
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.OpenAIModelListResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/models-all [get]
func ListAllModels(c *gin.Context) {
	c.JSON(200, gin.H{
		"object": "list",
		"data":   models,
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
	ctx := c.Request.Context()
	var availableModels []string
	if c.GetString(ctxkey.AvailableModels) != "" {
		availableModels = strings.Split(c.GetString(ctxkey.AvailableModels), ",")
	} else {
		userId := c.GetInt(ctxkey.Id)
		userGroup, _ := model.CacheGetUserGroup(userId)
		availableModels, _ = model.CacheGetGroupModels(ctx, userGroup)
	}
	modelSet := make(map[string]bool)
	for _, availableModel := range availableModels {
		modelSet[availableModel] = true
	}
	availableOpenAIModels := make([]OpenAIModels, 0)
	for _, model := range models {
		if _, ok := modelSet[model.Id]; ok {
			modelSet[model.Id] = false
			availableOpenAIModels = append(availableOpenAIModels, model)
		}
	}
	for modelName, ok := range modelSet {
		if ok {
			availableOpenAIModels = append(availableOpenAIModels, OpenAIModels{
				Id:      modelName,
				Object:  "model",
				Created: 1626777600,
				OwnedBy: "custom",
				Root:    modelName,
				Parent:  nil,
			})
		}
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
	if model, ok := modelsMap[modelId]; ok {
		c.JSON(200, model)
	} else {
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
	id := c.GetInt(ctxkey.Id)
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
		// backward compatibility: keep original payload
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
		name := ""
		if ch.Type >= 0 && ch.Type < len(channeltype.ChannelTypeNames) {
			name = channeltype.ChannelTypeNames[ch.Type]
		}
		if strings.ToLower(name) == provider && modelBelongsToProvider(provider, m) {
			filtered = append(filtered, m)
			continue
		}
		// in case channel type被配置为通用openai但实际是其他供应商，允许通过模型名归类
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
