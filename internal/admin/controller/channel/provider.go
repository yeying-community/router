package channel

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/helper"
	commonutils "github.com/yeying-community/router/common/utils"
	"github.com/yeying-community/router/internal/admin/model"
)

type modelProviderCatalogItem struct {
	Provider  string   `json:"provider"`
	Name      string   `json:"name,omitempty"`
	Models    []string `json:"models"`
	BaseURL   string   `json:"base_url,omitempty"`
	APIKey    string   `json:"api_key,omitempty"`
	Source    string   `json:"source,omitempty"`
	UpdatedAt int64    `json:"updated_at,omitempty"`
}

type modelProviderCatalogUpdateRequest struct {
	Providers []modelProviderCatalogItem `json:"providers"`
}

type modelProviderFetchRequest struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
	BaseURL  string `json:"base_url"`
}

type modelProviderSeed struct {
	Provider string
	Name     string
	BaseURL  string
	Models   []string
}

var mainstreamProviderSeeds = []modelProviderSeed{
	{
		Provider: "anthropic",
		Name:     "Anthropic Claude",
		BaseURL:  "https://api.anthropic.com",
		Models:   []string{"claude-opus-4-1", "claude-sonnet-4-5", "claude-3-7-sonnet-latest"},
	},
	{
		Provider: "cohere",
		Name:     "Cohere",
		BaseURL:  "https://api.cohere.com/compatibility/v1",
		Models:   []string{"command-r-plus", "command-r"},
	},
	{
		Provider: "deepseek",
		Name:     "DeepSeek",
		BaseURL:  "https://api.deepseek.com",
		Models:   []string{"deepseek-chat", "deepseek-reasoner"},
	},
	{
		Provider: "google",
		Name:     "Google Gemini",
		BaseURL:  "https://generativelanguage.googleapis.com/v1beta/openai",
		Models:   []string{"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.0-flash"},
	},
	{
		Provider: "hunyuan",
		Name:     "Tencent Hunyuan",
		BaseURL:  "https://api.hunyuan.cloud.tencent.com/v1",
		Models:   []string{"hunyuan-large", "hunyuan-turbo", "hunyuan-lite"},
	},
	{
		Provider: "minimax",
		Name:     "MiniMax",
		BaseURL:  "https://api.minimax.chat/v1",
		Models:   []string{"minimax-m1", "abab6.5s-chat"},
	},
	{
		Provider: "mistral",
		Name:     "Mistral",
		BaseURL:  "https://api.mistral.ai",
		Models:   []string{"mistral-large-latest", "mistral-small-latest"},
	},
	{
		Provider: "openai",
		Name:     "OpenAI",
		BaseURL:  "https://api.openai.com",
		Models:   []string{"gpt-5", "gpt-5-mini", "gpt-4.1", "gpt-4.1-mini", "gpt-4o", "gpt-4o-mini", "o3", "o4-mini"},
	},
	{
		Provider: "qwen",
		Name:     "Qwen",
		BaseURL:  "https://dashscope.aliyuncs.com/compatible-mode",
		Models:   []string{"qwen-max", "qwen-plus", "qwen-turbo"},
	},
	{
		Provider: "volcengine",
		Name:     "Volcengine Doubao",
		BaseURL:  "https://ark.cn-beijing.volces.com/api/v3",
		Models:   []string{"doubao-1.5-pro-32k", "doubao-1.5-lite-32k"},
	},
	{
		Provider: "xai",
		Name:     "xAI Grok",
		BaseURL:  "https://api.x.ai",
		Models:   []string{"grok-4", "grok-3", "grok-3-mini"},
	},
	{
		Provider: "zhipu",
		Name:     "Zhipu GLM",
		BaseURL:  "https://open.bigmodel.cn/api/paas/v4",
		Models:   []string{"glm-4-plus", "glm-4-air", "glm-4-flash"},
	},
}

var providerDefaultBaseURLs = map[string]string{
	"openai":     "https://api.openai.com",
	"google":     "https://generativelanguage.googleapis.com/v1beta/openai",
	"anthropic":  "https://api.anthropic.com",
	"xai":        "https://api.x.ai",
	"mistral":    "https://api.mistral.ai",
	"cohere":     "https://api.cohere.com/compatibility/v1",
	"deepseek":   "https://api.deepseek.com",
	"qwen":       "https://dashscope.aliyuncs.com/compatible-mode",
	"zhipu":      "https://open.bigmodel.cn/api/paas/v4",
	"hunyuan":    "https://api.hunyuan.cloud.tencent.com/v1",
	"volcengine": "https://ark.cn-beijing.volces.com/api/v3",
	"minimax":    "https://api.minimax.chat/v1",
}

func normalizeAndSortModels(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	normalized := make([]string, 0, len(models))
	for _, modelName := range models {
		name := strings.TrimSpace(modelName)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	sort.Strings(normalized)
	return normalized
}

func mergeAndSortModels(first, second []string) []string {
	merged := make([]string, 0, len(first)+len(second))
	merged = append(merged, first...)
	merged = append(merged, second...)
	return normalizeAndSortModels(merged)
}

func normalizeModelProviderCatalog(items []modelProviderCatalogItem) []modelProviderCatalogItem {
	indexByProvider := make(map[string]int, len(items))
	normalized := make([]modelProviderCatalogItem, 0, len(items))
	for _, item := range items {
		provider := commonutils.NormalizeModelProvider(item.Provider)
		if provider == "" {
			provider = commonutils.NormalizeModelProvider(item.Name)
		}
		if provider == "" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = provider
		}
		source := strings.TrimSpace(strings.ToLower(item.Source))
		if source == "" {
			source = "manual"
		}
		baseURL := strings.TrimSpace(item.BaseURL)
		apiKey := strings.TrimSpace(item.APIKey)
		entry := modelProviderCatalogItem{
			Provider:  provider,
			Name:      name,
			Models:    normalizeAndSortModels(item.Models),
			BaseURL:   baseURL,
			APIKey:    apiKey,
			Source:    source,
			UpdatedAt: item.UpdatedAt,
		}
		if idx, ok := indexByProvider[provider]; ok {
			existing := normalized[idx]
			existing.Models = mergeAndSortModels(existing.Models, entry.Models)
			if existing.Name == existing.Provider && entry.Name != entry.Provider {
				existing.Name = entry.Name
			}
			if existing.BaseURL == "" && entry.BaseURL != "" {
				existing.BaseURL = entry.BaseURL
			}
			if entry.BaseURL != "" && entry.Source != "default" {
				existing.BaseURL = entry.BaseURL
			}
			if existing.APIKey == "" && entry.APIKey != "" {
				existing.APIKey = entry.APIKey
			}
			if entry.APIKey != "" && entry.Source != "default" {
				existing.APIKey = entry.APIKey
			}
			if entry.UpdatedAt > existing.UpdatedAt {
				existing.UpdatedAt = entry.UpdatedAt
			}
			existing.Source = entry.Source
			normalized[idx] = existing
			continue
		}
		indexByProvider[provider] = len(normalized)
		normalized = append(normalized, entry)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].Provider < normalized[j].Provider
	})
	return normalized
}

func parseModelProviderModelsRaw(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return make([]string, 0)
	}
	models := make([]string, 0)
	if err := json.Unmarshal([]byte(trimmed), &models); err == nil {
		return normalizeAndSortModels(models)
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	return normalizeAndSortModels(parts)
}

func loadModelProviderCatalog() ([]modelProviderCatalogItem, error) {
	rows := make([]model.ModelProvider, 0)
	if err := model.DB.Order("provider asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]modelProviderCatalogItem, 0, len(rows))
	for _, row := range rows {
		provider := commonutils.NormalizeModelProvider(row.Provider)
		if provider == "" {
			continue
		}
		items = append(items, modelProviderCatalogItem{
			Provider:  provider,
			Name:      strings.TrimSpace(row.Name),
			Models:    parseModelProviderModelsRaw(row.Models),
			BaseURL:   strings.TrimSpace(row.BaseURL),
			APIKey:    strings.TrimSpace(row.APIKey),
			Source:    strings.TrimSpace(strings.ToLower(row.Source)),
			UpdatedAt: row.UpdatedAt,
		})
	}
	return normalizeModelProviderCatalog(items), nil
}

func saveModelProviderCatalog(items []modelProviderCatalogItem) ([]modelProviderCatalogItem, error) {
	now := helper.GetTimestamp()
	normalized := normalizeModelProviderCatalog(items)
	for i := range normalized {
		if normalized[i].UpdatedAt == 0 {
			normalized[i].UpdatedAt = now
		}
	}
	rows := make([]model.ModelProvider, 0, len(normalized))
	for _, item := range normalized {
		modelsRaw, err := json.Marshal(item.Models)
		if err != nil {
			return nil, err
		}
		rows = append(rows, model.ModelProvider{
			Provider:  item.Provider,
			Name:      strings.TrimSpace(item.Name),
			Models:    string(modelsRaw),
			BaseURL:   strings.TrimSpace(item.BaseURL),
			APIKey:    strings.TrimSpace(item.APIKey),
			Source:    strings.TrimSpace(strings.ToLower(item.Source)),
			UpdatedAt: item.UpdatedAt,
		})
	}
	tx := model.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	if err := tx.Where("1 = 1").Delete(&model.ModelProvider{}).Error; err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if len(rows) > 0 {
		if err := tx.Create(&rows).Error; err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return normalized, nil
}

func buildDefaultModelProviderCatalog() []modelProviderCatalogItem {
	entries := make([]modelProviderCatalogItem, 0, len(mainstreamProviderSeeds))
	now := helper.GetTimestamp()
	for _, seed := range mainstreamProviderSeeds {
		list := normalizeAndSortModels(seed.Models)
		entries = append(entries, modelProviderCatalogItem{
			Provider:  seed.Provider,
			Name:      seed.Name,
			Models:    list,
			BaseURL:   seed.BaseURL,
			Source:    "default",
			UpdatedAt: now,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Provider < entries[j].Provider
	})
	return entries
}

// GetModelProviders godoc
// @Summary Get model provider catalog (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.ModelProviderCatalogResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/model-provider [get]
func GetModelProviders(c *gin.Context) {
	items, err := loadModelProviderCatalog()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "模型供应商配置解析失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    items,
	})
}

// UpdateModelProviders godoc
// @Summary Update model provider catalog (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.ModelProviderCatalogUpdateRequest true "Model provider catalog payload"
// @Success 200 {object} docs.ModelProviderCatalogResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/model-provider [put]
func UpdateModelProviders(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "读取请求失败",
		})
		return
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请求体不能为空",
		})
		return
	}
	providers := make([]modelProviderCatalogItem, 0)
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &providers); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "请求体格式错误",
			})
			return
		}
	} else {
		req := modelProviderCatalogUpdateRequest{}
		if err := json.Unmarshal(trimmed, &req); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "请求体格式错误",
			})
			return
		}
		providers = req.Providers
	}

	saved, err := saveModelProviderCatalog(providers)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "保存模型供应商配置失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    saved,
	})
}

// GetDefaultModelProviders godoc
// @Summary Get default mainstream model provider catalog (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.ModelProviderCatalogResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/model-provider/defaults [get]
func GetDefaultModelProviders(c *gin.Context) {
	defaults := buildDefaultModelProviderCatalog()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    defaults,
	})
}

// FetchModelProviderModels godoc
// @Summary Fetch models from provider API (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.ModelProviderFetchRequest true "Provider fetch payload"
// @Success 200 {object} docs.ModelProviderFetchResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/model-provider/fetch [post]
func FetchModelProviderModels(c *gin.Context) {
	req := modelProviderFetchRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	provider := commonutils.NormalizeModelProvider(req.Provider)
	if provider == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请先选择模型供应商",
		})
		return
	}

	baseURL := strings.TrimSpace(req.BaseURL)
	catalogItems, loadErr := loadModelProviderCatalog()
	if loadErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "读取模型供应商配置失败: " + loadErr.Error(),
		})
		return
	}
	savedProvider := modelProviderCatalogItem{}
	for _, item := range catalogItems {
		if commonutils.NormalizeModelProvider(item.Provider) == provider {
			savedProvider = item
			break
		}
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(savedProvider.BaseURL)
	}
	if baseURL == "" {
		baseURL = providerDefaultBaseURLs[provider]
	}
	if baseURL == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "该供应商未配置默认 Base URL，请手动填写",
		})
		return
	}
	apiKey := strings.TrimSpace(req.Key)
	if apiKey == "" {
		apiKey = strings.TrimSpace(savedProvider.APIKey)
	}
	if apiKey == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请先配置该供应商 API Key",
		})
		return
	}

	models, err := fetchOpenAICompatibleModelIDsByBaseURL(apiKey, baseURL, provider)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "",
		"provider": provider,
		"data":     models,
	})
}
