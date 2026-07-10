package model

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
)

const (
	providerTextCachePricingOpenAIURL    = "https://platform.openai.com/docs/pricing"
	providerTextCachePricingAnthropicURL = "https://docs.anthropic.com/en/docs/about-claude/pricing"
	providerTextCachePricingGoogleURL    = "https://ai.google.dev/gemini-api/docs/pricing"
	providerTextCachePricingQwenURL      = "https://help.aliyun.com/zh/model-studio/model-pricing"
	providerTextCachePricingDeepSeekURL  = "https://api-docs.deepseek.com/quick_start/pricing"
	providerTextCachePricingZhipuURL     = "https://open.bigmodel.cn/pricing"
)

type providerTextCachePricingRule struct {
	Provider        string
	Model           string
	Condition       string
	CacheReadRate   float64
	CacheWriteRate  float64
	CacheReadPrice  float64
	CacheWritePrice float64
	SourceURL       string
}

var providerTextCachePricingRules = []providerTextCachePricingRule{
	// OpenAI pricing exposes cached input as 10% of regular text input for the
	// current OpenAI text models where the catalog's base price matches the
	// standard official price row. GPT-5.6 family models additionally publish
	// explicit cache write pricing at 125% of the uncached input rate.
	{Provider: "openai", Model: "gpt-5.3-codex", CacheReadRate: 0.1, SourceURL: providerTextCachePricingOpenAIURL},
	{Provider: "openai", Model: "gpt-5.4", CacheReadRate: 0.1, SourceURL: providerTextCachePricingOpenAIURL},
	{Provider: "openai", Model: "gpt-5.4-mini", CacheReadRate: 0.1, SourceURL: providerTextCachePricingOpenAIURL},
	{Provider: "openai", Model: "gpt-5.5", CacheReadRate: 0.1, SourceURL: providerTextCachePricingOpenAIURL},
	{Provider: "openai", Model: "gpt-5.6", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingOpenAIURL},
	{Provider: "openai", Model: "gpt-5.6-sol", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingOpenAIURL},
	{Provider: "openai", Model: "gpt-5.6-terra", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingOpenAIURL},
	{Provider: "openai", Model: "gpt-5.6-luna", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingOpenAIURL},

	// Anthropic prompt caching prices 5-minute cache writes at 1.25x input and
	// cache hits at 0.1x input for the Claude models tracked here.
	{Provider: "anthropic", Model: "claude-3-5-haiku-20241022", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingAnthropicURL},
	{Provider: "anthropic", Model: "claude-haiku-4-5", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingAnthropicURL},
	{Provider: "anthropic", Model: "claude-haiku-4-5-20251001", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingAnthropicURL},
	{Provider: "anthropic", Model: "claude-opus-4-1", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingAnthropicURL},
	{Provider: "anthropic", Model: "claude-opus-4-1-20250805", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingAnthropicURL},
	{Provider: "anthropic", Model: "claude-opus-4-5", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingAnthropicURL},
	{Provider: "anthropic", Model: "claude-opus-4-5-20251101", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingAnthropicURL},
	{Provider: "anthropic", Model: "claude-opus-4-6", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingAnthropicURL},
	{Provider: "anthropic", Model: "claude-opus-4-6-thinking", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingAnthropicURL},
	{Provider: "anthropic", Model: "claude-opus-4-7", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingAnthropicURL},
	{Provider: "anthropic", Model: "claude-opus-4-8", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingAnthropicURL},
	{Provider: "anthropic", Model: "claude-sonnet-4-5", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingAnthropicURL},
	{Provider: "anthropic", Model: "claude-sonnet-4-5-20250929", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingAnthropicURL},
	{Provider: "anthropic", Model: "claude-sonnet-4-6", CacheReadRate: 0.1, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingAnthropicURL},

	// Gemini context cache read prices are already expressed in the provider
	// catalog as context_cache. Mirror text/image/video cache reads into the
	// text cache component so runtime token billing can consume them directly.
	{Provider: "google", Model: "gemini-2.5-flash", Condition: "mode=standard;input_type=text_image_video", CacheReadPrice: 0.00003, SourceURL: providerTextCachePricingGoogleURL},
	{Provider: "google", Model: "gemini-2.5-flash-lite", Condition: "mode=standard;input_type=text_image_video", CacheReadPrice: 0.00001, SourceURL: providerTextCachePricingGoogleURL},
	{Provider: "google", Model: "gemini-2.5-pro", Condition: "mode=standard;prompt_tokens_lte=200000", CacheReadPrice: 0.000125, SourceURL: providerTextCachePricingGoogleURL},
	{Provider: "google", Model: "gemini-2.5-pro", Condition: "mode=standard;prompt_tokens_gt=200000", CacheReadPrice: 0.00025, SourceURL: providerTextCachePricingGoogleURL},

	// DashScope implicit cache hit prices are 20% of regular input for the
	// non-tiered Qwen text models currently present in the catalog. Tiered Qwen
	// rows are expressed explicitly by prompt token range. Explicit cache writes
	// are priced at 125% of the matched regular input price.
	{Provider: "qwen", Model: "qwen3.7-max", CacheReadRate: 0.2, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingQwenURL},
	{Provider: "qwen", Model: "qwen3.7-plus", Condition: "mode=standard;prompt_tokens_lte=256000", CacheReadPrice: 0.0004, CacheWritePrice: 0.0025, SourceURL: providerTextCachePricingQwenURL},
	{Provider: "qwen", Model: "qwen3.7-plus", Condition: "mode=standard;prompt_tokens_gt=256000;prompt_tokens_lte=1000000", CacheReadPrice: 0.0012, CacheWritePrice: 0.0075, SourceURL: providerTextCachePricingQwenURL},
	{Provider: "qwen", Model: "qwen3.6-plus", CacheReadRate: 0.2, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingQwenURL},
	{Provider: "qwen", Model: "qwen3.6-flash", CacheReadRate: 0.2, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingQwenURL},
	{Provider: "qwen", Model: "qwen3.5-plus", CacheReadRate: 0.2, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingQwenURL},
	{Provider: "qwen", Model: "qwen3.5-flash", CacheReadRate: 0.2, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingQwenURL},
	{Provider: "qwen", Model: "qwen3-max", CacheReadRate: 0.2, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingQwenURL},
	{Provider: "qwen", Model: "qwen3-max-2026-01-23", CacheReadRate: 0.2, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingQwenURL},
	{Provider: "qwen", Model: "qwen3-coder-next", CacheReadRate: 0.2, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingQwenURL},
	{Provider: "qwen", Model: "qwen3-coder-plus", CacheReadRate: 0.2, CacheWriteRate: 1.25, SourceURL: providerTextCachePricingQwenURL},

	// DeepSeek publishes explicit cache-hit input prices. Cache misses are the
	// regular input price already stored on the provider model rows.
	{Provider: "deepseek", Model: "deepseek-v4-flash", CacheReadRate: 0.02, SourceURL: providerTextCachePricingDeepSeekURL},
	{Provider: "deepseek", Model: "deepseek-chat", CacheReadRate: 0.02, SourceURL: providerTextCachePricingDeepSeekURL},
	{Provider: "deepseek", Model: "deepseek-reasoner", CacheReadRate: 0.02, SourceURL: providerTextCachePricingDeepSeekURL},
	{Provider: "deepseek", Model: "deepseek-v4-pro", CacheReadRate: 0.008333333333333333, SourceURL: providerTextCachePricingDeepSeekURL},

	// Zhipu publishes context cache read pricing for the GLM text models below.
	{Provider: "zhipu", Model: "glm-5.2", Condition: "mode=standard", CacheReadPrice: 0.002, SourceURL: providerTextCachePricingZhipuURL},
}

func upsertProviderTextCachePricingComponentsWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if err := db.AutoMigrate(&ProviderModelPriceComponent{}); err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		return upsertProviderTextCachePricingComponentsInTransaction(tx)
	})
}

func upsertProviderTextCachePricingComponentsInTransaction(db *gorm.DB) error {
	for _, rule := range providerTextCachePricingRules {
		if err := upsertProviderTextCachePricingRuleWithDB(db, rule); err != nil {
			return err
		}
	}
	return nil
}

func upsertProviderTextCachePricingRuleWithDB(db *gorm.DB, rule providerTextCachePricingRule) error {
	provider := strings.TrimSpace(strings.ToLower(rule.Provider))
	modelName := canonicalizeModelNameForProvider(provider, rule.Model)
	if provider == "" || modelName == "" {
		return nil
	}
	providerModel := ProviderModel{}
	if err := db.First(&providerModel, "provider = ? AND model = ?", provider, modelName).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if providerModel.InputPrice <= 0 {
		return nil
	}
	now := helper.GetTimestamp()
	rows := make([]ProviderModelPriceComponent, 0, 2)
	if rule.CacheReadRate > 0 {
		rows = append(rows, buildProviderTextCachePricingComponent(providerModel, ProviderModelPriceComponentTextCacheRead, rule.CacheReadRate, rule.SourceURL, now, 910))
	} else if rule.CacheReadPrice > 0 {
		rows = append(rows, buildProviderTextCachePricingComponentWithPrice(providerModel, ProviderModelPriceComponentTextCacheRead, rule.Condition, rule.CacheReadPrice, rule.SourceURL, now, 910))
	}
	if rule.CacheWriteRate > 0 {
		rows = append(rows, buildProviderTextCachePricingComponent(providerModel, ProviderModelPriceComponentTextCacheWrite, rule.CacheWriteRate, rule.SourceURL, now, 920))
	} else if rule.CacheWritePrice > 0 {
		rows = append(rows, buildProviderTextCachePricingComponentWithPrice(providerModel, ProviderModelPriceComponentTextCacheWrite, rule.Condition, rule.CacheWritePrice, rule.SourceURL, now, 920))
	}
	if len(rows) == 0 {
		return nil
	}
	for _, row := range rows {
		if err := upsertProviderTextCachePricingComponentWithDB(db, row); err != nil {
			return err
		}
	}
	return nil
}

func upsertProviderTextCachePricingComponentWithDB(db *gorm.DB, row ProviderModelPriceComponent) error {
	existing := ProviderModelPriceComponent{}
	result := db.Where(
		"provider = ? AND model = ? AND component = ? AND condition = ?",
		row.Provider,
		row.Model,
		row.Component,
		row.Condition,
	).Limit(1).Find(&existing)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return db.Create(&row).Error
	}
	if strings.TrimSpace(strings.ToLower(existing.Source)) != "migration" {
		return nil
	}
	return db.Model(&ProviderModelPriceComponent{}).
		Where(
			"provider = ? AND model = ? AND component = ? AND condition = ?",
			row.Provider,
			row.Model,
			row.Component,
			row.Condition,
		).
		Updates(map[string]any{
			"input_price":  row.InputPrice,
			"output_price": row.OutputPrice,
			"price_unit":   row.PriceUnit,
			"currency":     row.Currency,
			"source":       row.Source,
			"source_url":   row.SourceURL,
			"sort_order":   row.SortOrder,
			"updated_at":   row.UpdatedAt,
		}).Error
}

func buildProviderTextCachePricingComponent(providerModel ProviderModel, component string, rate float64, sourceURL string, now int64, sortOrder int) ProviderModelPriceComponent {
	inputPrice := providerModel.InputPrice * rate
	return buildProviderTextCachePricingComponentWithPrice(providerModel, component, "", inputPrice, sourceURL, now, sortOrder)
}

func buildProviderTextCachePricingComponentWithPrice(providerModel ProviderModel, component string, condition string, inputPrice float64, sourceURL string, now int64, sortOrder int) ProviderModelPriceComponent {
	return ProviderModelPriceComponent{
		Provider:   strings.TrimSpace(strings.ToLower(providerModel.Provider)),
		Model:      strings.TrimSpace(providerModel.Model),
		Component:  strings.TrimSpace(strings.ToLower(component)),
		Condition:  strings.TrimSpace(condition),
		InputPrice: roundProviderTextCachePrice(inputPrice),
		PriceUnit:  firstNonEmpty(providerModel.PriceUnit, ProviderPriceUnitPer1KTokens),
		Currency:   firstNonEmpty(providerModel.Currency, ProviderPriceCurrencyUSD),
		Source:     "migration",
		SourceURL:  strings.TrimSpace(sourceURL),
		SortOrder:  sortOrder,
		UpdatedAt:  now,
	}
}

func roundProviderTextCachePrice(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return math.Round(value*1_000_000_000_000) / 1_000_000_000_000
}
