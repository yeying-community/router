package model

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	commonutils "github.com/yeying-community/router/common/utils"
	"gorm.io/gorm"
)

const optionKeyModelProviderCatalog = "ModelProviderCatalog"

type modelProviderCatalogMigrationItem struct {
	Provider  string   `json:"provider"`
	Name      string   `json:"name,omitempty"`
	Models    []string `json:"models"`
	BaseURL   string   `json:"base_url,omitempty"`
	APIKey    string   `json:"api_key,omitempty"`
	Source    string   `json:"source,omitempty"`
	UpdatedAt int64    `json:"updated_at,omitempty"`
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

func runModelProviderMigrations() error {
	if err := normalizeChannelModelProviders(); err != nil {
		return err
	}
	if err := backfillChannelModelProviderFromModels(); err != nil {
		return err
	}
	if err := ensureModelProviderCatalogTable(); err != nil {
		return err
	}
	return nil
}

func normalizeChannelModelProviders() error {
	channels := make([]Channel, 0)
	if err := DB.Select("id", "model_provider").
		Where("COALESCE(model_provider, '') <> ''").
		Find(&channels).Error; err != nil {
		return err
	}
	updated := 0
	for _, channel := range channels {
		normalized := commonutils.NormalizeModelProvider(channel.ModelProvider)
		if normalized == "" || normalized == channel.ModelProvider {
			continue
		}
		if err := DB.Model(&Channel{}).
			Where("id = ?", channel.Id).
			Update("model_provider", normalized).Error; err != nil {
			return err
		}
		updated++
	}
	if updated > 0 {
		logger.SysLogf("migration: normalized model_provider for %d channels", updated)
	}
	return nil
}

func backfillChannelModelProviderFromModels() error {
	channels := make([]Channel, 0)
	if err := DB.Select("id", "models", "model_provider").
		Where("COALESCE(model_provider, '') = ''").
		Find(&channels).Error; err != nil {
		return err
	}
	updated := 0
	for _, channel := range channels {
		provider := inferModelProviderFromModelList(channel.Models)
		if provider == "" {
			continue
		}
		if err := DB.Model(&Channel{}).
			Where("id = ? AND COALESCE(model_provider, '') = ''", channel.Id).
			Update("model_provider", provider).Error; err != nil {
			return err
		}
		updated++
	}
	if updated > 0 {
		logger.SysLogf("migration: backfilled model_provider for %d channels", updated)
	}
	return nil
}

func inferModelProviderFromModelList(modelList string) string {
	models := strings.Split(modelList, ",")
	counts := make(map[string]int)
	for _, modelName := range models {
		provider := commonutils.NormalizeModelProvider(commonutils.ResolveModelProvider(modelName))
		if provider == "" || provider == "unknown" {
			continue
		}
		counts[provider]++
	}
	if len(counts) == 0 {
		return ""
	}
	// Deterministic selection: highest frequency, then lexical order.
	type item struct {
		provider string
		count    int
	}
	items := make([]item, 0, len(counts))
	for provider, count := range counts {
		items = append(items, item{provider: provider, count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count == items[j].count {
			return items[i].provider < items[j].provider
		}
		return items[i].count > items[j].count
	})
	return items[0].provider
}

func ensureModelProviderCatalogTable() error {
	tableItems, err := loadModelProviderCatalogFromTable()
	if err != nil {
		return err
	}

	if len(tableItems) == 0 {
		legacyItems, legacyErr := loadModelProviderCatalogFromLegacyOption()
		if legacyErr != nil {
			return legacyErr
		}
		if len(legacyItems) > 0 {
			tableItems = legacyItems
			logger.SysLog("migration: imported model providers from options.ModelProviderCatalog")
		} else {
			tableItems = buildMainstreamModelProviderCatalog(helper.GetTimestamp())
			logger.SysLog("migration: initialized model providers from mainstream defaults")
		}
	}

	normalizedItems, normalizeErr := normalizeModelProviderCatalogItems(tableItems)
	if normalizeErr != nil {
		logger.SysError("migration: normalize model providers failed, fallback to mainstream defaults: " + normalizeErr.Error())
		normalizedItems = buildMainstreamModelProviderCatalog(helper.GetTimestamp())
	}

	if err := saveModelProviderCatalogToTable(normalizedItems); err != nil {
		return err
	}

	// Legacy fallback source is no longer used after table migration.
	if err := DB.Where("key = ?", optionKeyModelProviderCatalog).Delete(&Option{}).Error; err != nil {
		return err
	}
	return nil
}

func normalizeModelProviderCatalogItems(items []modelProviderCatalogMigrationItem) ([]modelProviderCatalogMigrationItem, error) {
	raw, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	normalizedRaw, err := normalizeModelProviderCatalogRaw(string(raw))
	if err != nil {
		return nil, err
	}
	normalized := make([]modelProviderCatalogMigrationItem, 0)
	if err := json.Unmarshal([]byte(normalizedRaw), &normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func loadModelProviderCatalogFromLegacyOption() ([]modelProviderCatalogMigrationItem, error) {
	var option Option
	err := DB.Where("key = ?", optionKeyModelProviderCatalog).First(&option).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	raw := strings.TrimSpace(option.Value)
	if raw == "" {
		return nil, nil
	}
	normalizedRaw, normalizeErr := normalizeModelProviderCatalogRaw(raw)
	if normalizeErr != nil {
		logger.SysError("migration: failed to parse options.ModelProviderCatalog, fallback to defaults: " + normalizeErr.Error())
		return nil, nil
	}
	items := make([]modelProviderCatalogMigrationItem, 0)
	if err := json.Unmarshal([]byte(normalizedRaw), &items); err != nil {
		return nil, err
	}
	return items, nil
}

func parseModelProviderModels(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return make([]string, 0)
	}
	modelSet := make(map[string]struct{})
	jsonModels := make([]string, 0)
	if err := json.Unmarshal([]byte(trimmed), &jsonModels); err == nil {
		for _, modelName := range jsonModels {
			name := strings.TrimSpace(modelName)
			if name == "" {
				continue
			}
			modelSet[name] = struct{}{}
		}
		models := make([]string, 0, len(modelSet))
		for name := range modelSet {
			models = append(models, name)
		}
		sort.Strings(models)
		return models
	}

	for _, modelName := range strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	}) {
		name := strings.TrimSpace(modelName)
		if name == "" {
			continue
		}
		modelSet[name] = struct{}{}
	}
	models := make([]string, 0, len(modelSet))
	for name := range modelSet {
		models = append(models, name)
	}
	sort.Strings(models)
	return models
}

func loadModelProviderCatalogFromTable() ([]modelProviderCatalogMigrationItem, error) {
	rows := make([]ModelProvider, 0)
	if err := DB.Order("provider asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]modelProviderCatalogMigrationItem, 0, len(rows))
	for _, row := range rows {
		provider := commonutils.NormalizeModelProvider(row.Provider)
		if provider == "" {
			continue
		}
		items = append(items, modelProviderCatalogMigrationItem{
			Provider:  provider,
			Name:      strings.TrimSpace(row.Name),
			Models:    parseModelProviderModels(row.Models),
			BaseURL:   strings.TrimSpace(row.BaseURL),
			APIKey:    strings.TrimSpace(row.APIKey),
			Source:    strings.TrimSpace(strings.ToLower(row.Source)),
			UpdatedAt: row.UpdatedAt,
		})
	}
	return items, nil
}

func saveModelProviderCatalogToTable(items []modelProviderCatalogMigrationItem) error {
	now := helper.GetTimestamp()
	rows := make([]ModelProvider, 0, len(items))
	for _, item := range items {
		provider := commonutils.NormalizeModelProvider(item.Provider)
		if provider == "" {
			continue
		}
		modelSet := make(map[string]struct{}, len(item.Models))
		for _, modelName := range item.Models {
			name := strings.TrimSpace(modelName)
			if name == "" {
				continue
			}
			modelSet[name] = struct{}{}
		}
		models := make([]string, 0, len(modelSet))
		for name := range modelSet {
			models = append(models, name)
		}
		sort.Strings(models)
		modelsRaw, err := json.Marshal(models)
		if err != nil {
			return err
		}
		updatedAt := item.UpdatedAt
		if updatedAt == 0 {
			updatedAt = now
		}
		source := strings.TrimSpace(strings.ToLower(item.Source))
		if source == "" {
			source = "manual"
		}
		rows = append(rows, ModelProvider{
			Provider:  provider,
			Name:      strings.TrimSpace(item.Name),
			Models:    string(modelsRaw),
			BaseURL:   strings.TrimSpace(item.BaseURL),
			APIKey:    strings.TrimSpace(item.APIKey),
			Source:    source,
			UpdatedAt: updatedAt,
		})
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&ModelProvider{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func buildMainstreamModelProviderCatalog(now int64) []modelProviderCatalogMigrationItem {
	items := make([]modelProviderCatalogMigrationItem, 0, len(mainstreamProviderSeeds))
	for _, seed := range mainstreamProviderSeeds {
		modelSet := make(map[string]struct{}, len(seed.Models))
		for _, modelName := range seed.Models {
			name := strings.TrimSpace(modelName)
			if name == "" {
				continue
			}
			modelSet[name] = struct{}{}
		}
		models := make([]string, 0, len(modelSet))
		for modelName := range modelSet {
			models = append(models, modelName)
		}
		sort.Strings(models)
		items = append(items, modelProviderCatalogMigrationItem{
			Provider:  seed.Provider,
			Name:      seed.Name,
			Models:    models,
			BaseURL:   seed.BaseURL,
			Source:    "default",
			UpdatedAt: now,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Provider < items[j].Provider
	})
	return items
}

func buildDefaultModelProviderCatalogRaw() (string, error) {
	items := buildMainstreamModelProviderCatalog(helper.GetTimestamp())
	raw, err := json.Marshal(items)
	return string(raw), err
}

func normalizeModelProviderCatalogRaw(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return buildDefaultModelProviderCatalogRaw()
	}
	items := make([]modelProviderCatalogMigrationItem, 0)
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return "", err
	}

	indexByProvider := make(map[string]int, len(items))
	normalized := make([]modelProviderCatalogMigrationItem, 0, len(items))
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
		modelSet := make(map[string]struct{}, len(item.Models))
		for _, modelName := range item.Models {
			name := strings.TrimSpace(modelName)
			if name == "" {
				continue
			}
			modelSet[name] = struct{}{}
		}
		models := make([]string, 0, len(modelSet))
		for name := range modelSet {
			models = append(models, name)
		}
		sort.Strings(models)

		entry := modelProviderCatalogMigrationItem{
			Provider:  provider,
			Name:      name,
			Models:    models,
			BaseURL:   baseURL,
			APIKey:    apiKey,
			Source:    source,
			UpdatedAt: item.UpdatedAt,
		}
		if idx, ok := indexByProvider[provider]; ok {
			existing := normalized[idx]
			modelUnion := make(map[string]struct{}, len(existing.Models)+len(entry.Models))
			for _, m := range existing.Models {
				modelUnion[m] = struct{}{}
			}
			for _, m := range entry.Models {
				modelUnion[m] = struct{}{}
			}
			mergedModels := make([]string, 0, len(modelUnion))
			for m := range modelUnion {
				mergedModels = append(mergedModels, m)
			}
			sort.Strings(mergedModels)
			existing.Models = mergedModels
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
	normalized = reconcileWithMainstreamDefaults(normalized)

	normalizedRaw, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(normalizedRaw), nil
}

func reconcileWithMainstreamDefaults(items []modelProviderCatalogMigrationItem) []modelProviderCatalogMigrationItem {
	seeded := buildMainstreamModelProviderCatalog(helper.GetTimestamp())
	seedByProvider := make(map[string]modelProviderCatalogMigrationItem, len(seeded))
	for _, item := range seeded {
		seedByProvider[item.Provider] = item
	}

	result := make(map[string]modelProviderCatalogMigrationItem, len(items)+len(seeded))
	for _, item := range seeded {
		result[item.Provider] = item
	}

	for _, item := range items {
		provider := commonutils.NormalizeModelProvider(item.Provider)
		if provider == "" {
			continue
		}
		item.Provider = provider
		if seededItem, ok := seedByProvider[provider]; ok {
			merged := seededItem
			if strings.TrimSpace(item.Name) != "" && item.Name != provider {
				merged.Name = strings.TrimSpace(item.Name)
			}
			if strings.TrimSpace(item.BaseURL) != "" {
				merged.BaseURL = strings.TrimSpace(item.BaseURL)
			}
			if strings.TrimSpace(item.APIKey) != "" {
				merged.APIKey = strings.TrimSpace(item.APIKey)
			}
			if item.UpdatedAt > 0 {
				merged.UpdatedAt = item.UpdatedAt
			}
			if item.Source != "default" {
				if len(item.Models) > 0 {
					merged.Models = item.Models
				}
				merged.Source = item.Source
			}
			result[provider] = merged
			continue
		}

		// Drop legacy default providers outside the curated mainstream set.
		if item.Source == "default" {
			continue
		}
		result[provider] = item
	}

	mergedItems := make([]modelProviderCatalogMigrationItem, 0, len(result))
	for _, item := range result {
		modelSet := make(map[string]struct{}, len(item.Models))
		for _, modelName := range item.Models {
			name := strings.TrimSpace(modelName)
			if name == "" {
				continue
			}
			modelSet[name] = struct{}{}
		}
		item.Models = make([]string, 0, len(modelSet))
		for modelName := range modelSet {
			item.Models = append(item.Models, modelName)
		}
		sort.Strings(item.Models)
		if item.Name == "" {
			item.Name = item.Provider
		}
		if item.Source == "" {
			item.Source = "manual"
		}
		mergedItems = append(mergedItems, item)
	}
	sort.Slice(mergedItems, func(i, j int) bool {
		return mergedItems[i].Provider < mergedItems[j].Provider
	})
	return mergedItems
}
