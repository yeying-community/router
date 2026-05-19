package model

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	commonutils "github.com/yeying-community/router/common/utils"
	"gorm.io/gorm"
)

type ResolvedModelPricing struct {
	Model                         string                              `json:"model"`
	Provider                      string                              `json:"provider,omitempty"`
	Type                          string                              `json:"type"`
	InputPrice                    float64                             `json:"input_price"`
	OutputPrice                   float64                             `json:"output_price"`
	PriceUnit                     string                              `json:"price_unit"`
	Currency                      string                              `json:"currency"`
	Source                        string                              `json:"source"`
	PriceComponents               []ProviderModelPriceComponentDetail `json:"price_components,omitempty"`
	MatchedComponent              string                              `json:"matched_component,omitempty"`
	MatchedCondition              string                              `json:"matched_condition,omitempty"`
	HasChannelOverride            bool                                `json:"has_channel_override"`
	HasChannelInputPriceOverride  bool                                `json:"has_channel_input_price_override,omitempty"`
	HasChannelOutputPriceOverride bool                                `json:"has_channel_output_price_override,omitempty"`
	HasChannelComponentOverride   bool                                `json:"has_channel_component_override,omitempty"`
}

func (pricing ResolvedModelPricing) IsConfigured() bool {
	return pricing.InputPrice > 0 || pricing.OutputPrice > 0
}

type providerModelPricingEntry struct {
	Provider string
	Detail   ProviderModelDetail
}

type providerModelPricingIndex struct {
	byProviderAndModel map[string]providerModelPricingEntry
	byModel            map[string][]providerModelPricingEntry
}

var (
	modelPricingIndexLock sync.RWMutex
	modelPricingIndex     = providerModelPricingIndex{
		byProviderAndModel: make(map[string]providerModelPricingEntry),
		byModel:            make(map[string][]providerModelPricingEntry),
	}
)

func SyncModelPricingCatalogWithDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	detailsMap, err := LoadProviderModelDetailsMap(db)
	if err != nil {
		return err
	}
	next := buildModelPricingIndexFromProviderDetailsMap(detailsMap)

	modelPricingIndexLock.Lock()
	modelPricingIndex = next
	modelPricingIndexLock.Unlock()
	return nil
}

func buildModelPricingIndexFromProviderDetailsMap(detailsMap map[string][]ProviderModelDetail) providerModelPricingIndex {
	estimatedSize := 0
	for _, details := range detailsMap {
		estimatedSize += len(details)
	}
	next := providerModelPricingIndex{
		byProviderAndModel: make(map[string]providerModelPricingEntry, estimatedSize),
		byModel:            make(map[string][]providerModelPricingEntry),
	}
	for providerKey, details := range detailsMap {
		provider := commonutils.NormalizeProvider(providerKey)
		if provider == "" {
			provider = strings.TrimSpace(strings.ToLower(providerKey))
		}
		if provider == "" {
			continue
		}
		for _, detail := range NormalizeProviderModelDetails(details) {
			modelName := normalizePricingLookupModelName(canonicalizeModelNameForProvider(provider, detail.Model))
			if modelName == "" {
				continue
			}
			normalizedDetail := detail
			normalizedDetail.Model = modelName
			normalizedDetail.Type = normalizeModelType(detail.Type, modelName)
			entry := providerModelPricingEntry{
				Provider: provider,
				Detail:   normalizedDetail,
			}
			next.byProviderAndModel[buildProviderModelPricingKey(provider, modelName)] = entry
			next.byModel[modelName] = append(next.byModel[modelName], entry)
		}
	}
	for modelName, entries := range next.byModel {
		sort.SliceStable(entries, func(i, j int) bool {
			return entries[i].Provider < entries[j].Provider
		})
		next.byModel[modelName] = entries
	}
	return next
}

func ResolveChannelModelPricing(channelProtocol int, channelModels []ChannelModel, modelName string) (ResolvedModelPricing, error) {
	normalizedModel := normalizePricingLookupModelName(modelName)
	if normalizedModel == "" {
		return ResolvedModelPricing{}, fmt.Errorf("model name is empty")
	}

	pricing, ok := lookupProviderDefaultModelPricing(normalizedModel, channelProtocol)
	if !ok {
		pricing = ResolvedModelPricing{}
	}

	if override, ok := findSelectedChannelModelPricingOverride(channelModels, normalizedModel); ok {
		hasOverride := false
		if override.InputPrice != nil && *override.InputPrice > 0 {
			pricing.InputPrice = *override.InputPrice
			hasOverride = true
			pricing.HasChannelInputPriceOverride = true
		}
		if override.OutputPrice != nil && *override.OutputPrice > 0 {
			pricing.OutputPrice = *override.OutputPrice
			hasOverride = true
			pricing.HasChannelOutputPriceOverride = true
		}
		if len(override.PriceComponents) > 0 {
			pricing.PriceComponents = mergeChannelModelPriceComponentOverrides(pricing.PriceComponents, override.PriceComponents)
			hasOverride = true
			pricing.HasChannelComponentOverride = true
		}
		if hasOverride {
			if override.PriceUnit != "" {
				pricing.PriceUnit = override.PriceUnit
			}
			if override.Currency != "" {
				pricing.Currency = override.Currency
			}
			pricing.HasChannelOverride = true
			pricing.Source = "channel_override"
		}
	}

	if pricing.Type == "" {
		pricing.Type = normalizeModelType("", normalizedModel)
	}
	if pricing.PriceUnit == "" {
		pricing.PriceUnit = defaultPriceUnitByType(pricing.Type, normalizedModel)
	}
	if pricing.Currency == "" {
		pricing.Currency = ProviderPriceCurrencyUSD
	}
	pricing.Model = normalizedModel
	if !pricing.IsConfigured() {
		return pricing, fmt.Errorf("model pricing not configured for %s", normalizedModel)
	}
	return pricing, nil
}

func buildProviderModelPricingKey(provider string, modelName string) string {
	return provider + ":" + modelName
}

func normalizePricingLookupModelName(modelName string) string {
	name := strings.TrimSpace(modelName)
	if strings.HasPrefix(name, "qwen-") && strings.HasSuffix(name, "-internet") {
		name = strings.TrimSuffix(name, "-internet")
	}
	if strings.HasPrefix(name, "command-") && strings.HasSuffix(name, "-internet") {
		name = strings.TrimSuffix(name, "-internet")
	}
	return strings.TrimSpace(name)
}

func lookupProviderDefaultModelPricing(modelName string, channelProtocol int) (ResolvedModelPricing, bool) {
	modelPricingIndexLock.RLock()
	index := modelPricingIndex
	modelPricingIndexLock.RUnlock()
	if len(index.byProviderAndModel) == 0 && DB != nil {
		_ = SyncModelPricingCatalogWithDB(DB)
		modelPricingIndexLock.RLock()
		index = modelPricingIndex
		modelPricingIndexLock.RUnlock()
	}

	preferredProvider := inferProviderByModel(modelName, channelProtocol, channelProtocol > 0)
	if preferredProvider != "" {
		canonicalModel := canonicalizeModelNameForProvider(preferredProvider, modelName)
		if entry, ok := index.byProviderAndModel[buildProviderModelPricingKey(preferredProvider, canonicalModel)]; ok {
			return resolvedModelPricingFromProviderEntry(modelName, entry), true
		}
	}

	candidates := []string{modelName}
	if strings.Contains(modelName, "/") {
		parts := strings.SplitN(modelName, "/", 2)
		if len(parts) == 2 {
			candidates = append(candidates, normalizePricingLookupModelName(parts[1]))
		}
	}
	if preferredProvider != "" {
		candidates = append(candidates, canonicalizeModelNameForProvider(preferredProvider, modelName))
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = normalizePricingLookupModelName(candidate)
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		entries := index.byModel[candidate]
		if len(entries) == 0 {
			continue
		}
		entry, ok := pickProviderModelPricingEntry(entries, preferredProvider)
		if !ok {
			continue
		}
		return resolvedModelPricingFromProviderEntry(modelName, entry), true
	}
	return ResolvedModelPricing{}, false
}

func pickProviderModelPricingEntry(entries []providerModelPricingEntry, preferredProvider string) (providerModelPricingEntry, bool) {
	if len(entries) == 0 {
		return providerModelPricingEntry{}, false
	}
	for _, entry := range entries {
		if entry.Provider == preferredProvider {
			return entry, true
		}
	}
	for _, entry := range entries {
		if entry.Provider != "other" {
			return entry, true
		}
	}
	return entries[0], true
}

func resolvedModelPricingFromProviderEntry(modelName string, entry providerModelPricingEntry) ResolvedModelPricing {
	return ResolvedModelPricing{
		Model:           modelName,
		Provider:        entry.Provider,
		Type:            normalizeModelType(entry.Detail.Type, entry.Detail.Model),
		InputPrice:      entry.Detail.InputPrice,
		OutputPrice:     entry.Detail.OutputPrice,
		PriceUnit:       entry.Detail.PriceUnit,
		Currency:        entry.Detail.Currency,
		Source:          "provider_default",
		PriceComponents: NormalizeProviderModelPriceComponents(entry.Detail.PriceComponents),
	}
}

func ResolveImageRequestPricing(pricing ResolvedModelPricing, size string, quality string) ResolvedModelPricing {
	component, ok := selectProviderPriceComponent(
		pricing.PriceComponents,
		ProviderModelPriceComponentImageGeneration,
		map[string]string{
			"size":    strings.TrimSpace(strings.ToLower(size)),
			"quality": strings.TrimSpace(strings.ToLower(quality)),
		},
	)
	if !ok {
		return pricing
	}
	if !pricing.HasChannelInputPriceOverride {
		pricing.InputPrice = component.InputPrice
	}
	if pricing.HasChannelOutputPriceOverride {
		// Keep the channel-specific output price above provider component defaults.
	} else if component.OutputPrice > 0 {
		pricing.OutputPrice = component.OutputPrice
	} else {
		pricing.OutputPrice = 0
	}
	if component.PriceUnit != "" && !pricing.HasChannelOverride {
		pricing.PriceUnit = component.PriceUnit
	}
	if component.Currency != "" && !pricing.HasChannelOverride {
		pricing.Currency = component.Currency
	}
	if pricing.HasChannelOverride || strings.TrimSpace(strings.ToLower(component.Source)) == "channel_override" {
		pricing.Source = "channel_override"
	} else {
		pricing.Source = "provider_component"
	}
	pricing.MatchedComponent = component.Component
	pricing.MatchedCondition = component.Condition
	return pricing
}

func ResolveTextRequestPricing(pricing ResolvedModelPricing, endpoint string) ResolvedModelPricing {
	componentType := ProviderModelPriceComponentText
	normalizedEndpoint := strings.TrimSpace(strings.ToLower(endpoint))
	if normalizedEndpoint == "" {
		return pricing
	}
	component, ok := selectProviderPriceComponent(
		pricing.PriceComponents,
		componentType,
		map[string]string{
			"endpoint": normalizedEndpoint,
		},
	)
	if !ok {
		return pricing
	}
	if component.InputPrice > 0 {
		pricing.InputPrice = component.InputPrice
	}
	if component.OutputPrice > 0 {
		pricing.OutputPrice = component.OutputPrice
	}
	if component.PriceUnit != "" {
		pricing.PriceUnit = component.PriceUnit
	}
	if component.Currency != "" {
		pricing.Currency = component.Currency
	}
	pricing.Source = "provider_component"
	pricing.MatchedComponent = component.Component
	pricing.MatchedCondition = component.Condition
	return pricing
}

func ResolveAudioRequestPricing(pricing ResolvedModelPricing, output bool) ResolvedModelPricing {
	componentType := ProviderModelPriceComponentAudioInput
	if output {
		componentType = ProviderModelPriceComponentAudioOutput
	}
	component, ok := selectProviderPriceComponent(
		pricing.PriceComponents,
		componentType,
		nil,
	)
	if !ok {
		return pricing
	}
	if component.InputPrice > 0 {
		pricing.InputPrice = component.InputPrice
	}
	if component.OutputPrice > 0 {
		pricing.OutputPrice = component.OutputPrice
	}
	if component.PriceUnit != "" {
		pricing.PriceUnit = component.PriceUnit
	}
	if component.Currency != "" {
		pricing.Currency = component.Currency
	}
	pricing.Source = "provider_component"
	pricing.MatchedComponent = component.Component
	pricing.MatchedCondition = component.Condition
	return pricing
}

func ResolveVideoRequestPricing(pricing ResolvedModelPricing, attrs map[string]string) ResolvedModelPricing {
	component, ok := selectProviderPriceComponent(
		pricing.PriceComponents,
		ProviderModelPriceComponentVideoGeneration,
		attrs,
	)
	if !ok {
		return pricing
	}
	if component.InputPrice > 0 {
		pricing.InputPrice = component.InputPrice
	}
	if component.OutputPrice > 0 {
		pricing.OutputPrice = component.OutputPrice
	}
	if component.PriceUnit != "" {
		pricing.PriceUnit = component.PriceUnit
	}
	if component.Currency != "" {
		pricing.Currency = component.Currency
	}
	pricing.Source = "provider_component"
	pricing.MatchedComponent = component.Component
	pricing.MatchedCondition = component.Condition
	return pricing
}

func selectProviderPriceComponent(components []ProviderModelPriceComponentDetail, componentType string, attrs map[string]string) (ProviderModelPriceComponentDetail, bool) {
	for _, component := range NormalizeProviderModelPriceComponents(components) {
		if component.Component != strings.TrimSpace(strings.ToLower(componentType)) {
			continue
		}
		if providerPriceComponentMatches(component.Condition, attrs) {
			return component, true
		}
	}
	return ProviderModelPriceComponentDetail{}, false
}

func mergeChannelModelPriceComponentOverrides(providerComponents []ProviderModelPriceComponentDetail, channelComponents []ProviderModelPriceComponentDetail) []ProviderModelPriceComponentDetail {
	merged := NormalizeProviderModelPriceComponents(providerComponents)
	indexByKey := make(map[string]int, len(merged))
	for idx, component := range merged {
		indexByKey[component.Component+"\x00"+component.Condition] = idx
	}
	for _, component := range NormalizeProviderModelPriceComponents(channelComponents) {
		if component.Component == "" {
			continue
		}
		if component.Source == "" || component.Source == "manual" || component.Source == "default" {
			component.Source = "channel_override"
		}
		key := component.Component + "\x00" + component.Condition
		if idx, ok := indexByKey[key]; ok {
			merged[idx] = component
			continue
		}
		indexByKey[key] = len(merged)
		merged = append(merged, component)
	}
	return NormalizeProviderModelPriceComponents(merged)
}

func providerPriceComponentMatches(condition string, attrs map[string]string) bool {
	normalizedCondition := strings.TrimSpace(condition)
	if normalizedCondition == "" {
		return true
	}
	parts := strings.Split(normalizedCondition, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pair := strings.SplitN(part, "=", 2)
		if len(pair) != 2 {
			return false
		}
		key := strings.TrimSpace(strings.ToLower(pair[0]))
		value := strings.TrimSpace(strings.ToLower(pair[1]))
		if key == "" {
			return false
		}
		if strings.TrimSpace(strings.ToLower(attrs[key])) != value {
			return false
		}
	}
	return true
}

func findSelectedChannelModelPricingOverride(rows []ChannelModel, modelName string) (ChannelModel, bool) {
	normalizedRows := NormalizeChannelModelsPreserveOrder(rows)
	normalizedModelName := normalizePricingLookupModelName(modelName)
	for _, row := range normalizedRows {
		if !row.Selected {
			continue
		}
		if !channelModelMatchesPricing(row, normalizedModelName) {
			continue
		}
		return row, true
	}
	return ChannelModel{}, false
}

func channelModelMatchesPricing(row ChannelModel, modelName string) bool {
	upstream := normalizePricingLookupModelName(row.UpstreamModel)
	alias := normalizePricingLookupModelName(row.Model)
	return upstream == modelName || alias == modelName
}
