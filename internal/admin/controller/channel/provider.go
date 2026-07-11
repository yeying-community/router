package channel

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/helper"
	commonutils "github.com/yeying-community/router/common/utils"
	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/gorm"
)

const (
	defaultProviderPageSize = 10
	maxProviderPageSize     = 100
)

type providerItem struct {
	ID           string                      `json:"id"`
	Provider     string                      `json:"provider,omitempty"`
	Name         string                      `json:"name,omitempty"`
	Models       []string                    `json:"models"`
	ModelDetails []model.ProviderModelDetail `json:"model_details,omitempty"`
	BaseURL      string                      `json:"base_url,omitempty"`
	OfficialURL  string                      `json:"official_url,omitempty"`
	SortOrder    int                         `json:"sort_order,omitempty"`
	Source       string                      `json:"source,omitempty"`
	CreatedAt    int64                       `json:"created_at,omitempty"`
	UpdatedAt    int64                       `json:"updated_at,omitempty"`
}

type providerListData struct {
	Items    []providerItem `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

type publicProviderModelDetail struct {
	Model              string                            `json:"model"`
	Tags               []string                          `json:"tags,omitempty"`
	Status             string                            `json:"status,omitempty"`
	Description        string                            `json:"description,omitempty"`
	Specification      *model.ProviderModelSpecification `json:"specification,omitempty"`
	SupportedEndpoints []string                          `json:"supported_endpoints,omitempty"`
}

type publicProviderModelItem struct {
	ID        string                      `json:"id"`
	Name      string                      `json:"name,omitempty"`
	Models    []publicProviderModelDetail `json:"models"`
	SortOrder int                         `json:"sort_order,omitempty"`
	UpdatedAt int64                       `json:"updated_at,omitempty"`
}

type appendProviderModelRequest struct {
	Model              string   `json:"model"`
	Tags               []string `json:"tags,omitempty"`
	Status             string   `json:"status,omitempty"`
	Description        string   `json:"description,omitempty"`
	IsDeleted          bool     `json:"is_deleted,omitempty"`
	SupportedEndpoints []string `json:"supported_endpoints,omitempty"`
	InputPrice         float64  `json:"input_price,omitempty"`
	OutputPrice        float64  `json:"output_price,omitempty"`
	PriceUnit          string   `json:"price_unit,omitempty"`
	Currency           string   `json:"currency,omitempty"`
	Source             string   `json:"source,omitempty"`
}

func providerModelNames(details []model.ProviderModelDetail) []string {
	normalized := model.FilterActiveProviderModelDetails(details)
	names := make([]string, 0, len(normalized))
	for _, item := range normalized {
		if strings.TrimSpace(item.Model) == "" {
			continue
		}
		names = append(names, item.Model)
	}
	return names
}

func mergeProviderDetailInputs(current []model.ProviderModelDetail, fallbackModels []string, now int64) []model.ProviderModelDetail {
	merged := make([]model.ProviderModelDetail, 0, len(current)+len(fallbackModels))
	merged = append(merged, current...)
	seen := make(map[string]struct{}, len(current))
	for _, detail := range current {
		modelName := strings.TrimSpace(detail.Model)
		if modelName == "" {
			continue
		}
		seen[modelName] = struct{}{}
	}
	for _, modelName := range fallbackModels {
		name := strings.TrimSpace(modelName)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		merged = append(merged, model.ProviderModelDetail{
			Model:     name,
			Source:    "manual",
			UpdatedAt: now,
		})
	}
	return model.NormalizeProviderModelDetails(merged)
}

func applyProviderModelEndpointDefaults(provider string, details []model.ProviderModelDetail) []model.ProviderModelDetail {
	normalizedProvider := commonutils.NormalizeProvider(provider)
	normalizedDetails := model.NormalizeProviderModelDetails(details)
	for i := range normalizedDetails {
		if len(normalizedDetails[i].SupportedEndpoints) > 0 {
			continue
		}
		normalizedDetails[i].SupportedEndpoints = model.DefaultProviderModelSupportedEndpoints(
			normalizedProvider,
			normalizedDetails[i].Type,
			normalizedDetails[i].Model,
		)
	}
	return model.NormalizeProviderModelDetails(normalizedDetails)
}

func validateProviderModelTags(details []model.ProviderModelDetail) error {
	required := make(map[string]bool, len(details))
	valid := make(map[string]bool, len(details))
	for _, detail := range details {
		if detail.IsDeleted {
			continue
		}
		modelName := strings.TrimSpace(detail.Model)
		if modelName == "" {
			continue
		}
		required[modelName] = true
		tags := model.NormalizeProviderModelTags(detail.Tags)
		if len(tags) > 0 && model.ProviderModelTypeFromTags(tags) != "" {
			valid[modelName] = true
		}
	}
	for modelName := range required {
		if !valid[modelName] {
			return fmt.Errorf("模型 %s 的 tags 必须包含 text、image、audio、video、embedding 之一", modelName)
		}
	}
	return nil
}

func mergeMissingProviderDetailsAsDeleted(current []model.ProviderModelDetail, existing []model.ProviderModelDetail) []model.ProviderModelDetail {
	if len(existing) == 0 {
		return model.NormalizeProviderModelDetails(current)
	}
	merged := make([]model.ProviderModelDetail, 0, len(current)+len(existing))
	merged = append(merged, current...)
	currentByModel := make(map[string]struct{}, len(current))
	for _, detail := range current {
		modelName := strings.TrimSpace(detail.Model)
		if modelName == "" {
			continue
		}
		currentByModel[modelName] = struct{}{}
	}
	for _, detail := range existing {
		modelName := strings.TrimSpace(detail.Model)
		if modelName == "" {
			continue
		}
		if _, ok := currentByModel[modelName]; ok {
			continue
		}
		deletedDetail := detail
		deletedDetail.IsDeleted = true
		merged = append(merged, deletedDetail)
	}
	return model.NormalizeProviderModelDetails(merged)
}

type providerModelUsageSummary struct {
	ChannelModels      []string
	GroupModels        []string
	GroupModelChannels []string
}

func (summary providerModelUsageSummary) InUse() bool {
	return len(summary.ChannelModels) > 0 || len(summary.GroupModels) > 0 || len(summary.GroupModelChannels) > 0
}

func (summary providerModelUsageSummary) Error(provider string, modelName string) error {
	parts := make([]string, 0, 3)
	if len(summary.ChannelModels) > 0 {
		parts = append(parts, fmt.Sprintf("channels=%s", strings.Join(summary.ChannelModels, ",")))
	}
	if len(summary.GroupModels) > 0 {
		parts = append(parts, fmt.Sprintf("group_models=%s", strings.Join(summary.GroupModels, ",")))
	}
	if len(summary.GroupModelChannels) > 0 {
		parts = append(parts, fmt.Sprintf("group_model_channels=%s", strings.Join(summary.GroupModelChannels, ",")))
	}
	return fmt.Errorf("provider model %s/%s is still in use: %s", provider, modelName, strings.Join(parts, "; "))
}

func formatChannelUsageLabelsWithDB(db *gorm.DB, channelIDs []string) ([]string, error) {
	if len(channelIDs) == 0 {
		return []string{}, nil
	}
	normalizedIDs := make([]string, 0, len(channelIDs))
	seen := make(map[string]struct{}, len(channelIDs))
	for _, rawID := range channelIDs {
		channelID := strings.TrimSpace(rawID)
		if channelID == "" {
			continue
		}
		if _, exists := seen[channelID]; exists {
			continue
		}
		seen[channelID] = struct{}{}
		normalizedIDs = append(normalizedIDs, channelID)
	}
	if len(normalizedIDs) == 0 {
		return []string{}, nil
	}

	channels := make([]model.Channel, 0, len(normalizedIDs))
	if err := db.Model(&model.Channel{}).
		Select("id", "name").
		Where("id IN ?", normalizedIDs).
		Find(&channels).Error; err != nil {
		return nil, err
	}

	displayNames := make(map[string]string, len(channels))
	for i := range channels {
		channelID := strings.TrimSpace(channels[i].Id)
		if channelID == "" {
			continue
		}
		displayNames[channelID] = strings.TrimSpace(channels[i].DisplayName())
	}

	labels := make([]string, 0, len(normalizedIDs))
	for _, channelID := range normalizedIDs {
		displayName := displayNames[channelID]
		if displayName == "" || displayName == channelID {
			labels = append(labels, channelID)
			continue
		}
		labels = append(labels, fmt.Sprintf("%s (%s)", displayName, channelID))
	}
	return labels, nil
}

func collectProviderModelUsageWithDB(db *gorm.DB, provider string, modelName string) (providerModelUsageSummary, error) {
	if db == nil {
		return providerModelUsageSummary{}, fmt.Errorf("database handle is nil")
	}
	normalizedProvider := commonutils.NormalizeProvider(provider)
	normalizedModel := strings.TrimSpace(modelName)
	if normalizedProvider == "" || normalizedModel == "" {
		return providerModelUsageSummary{}, nil
	}

	type channelModelRef struct {
		ChannelID string `gorm:"column:channel_id"`
	}
	channelRefs := make([]channelModelRef, 0)
	if err := db.Model(&model.ChannelModel{}).
		Select("DISTINCT channel_id").
		Where("provider = ? AND publish_enabled = ? AND (model = ? OR upstream_model = ?)", normalizedProvider, true, normalizedModel, normalizedModel).
		Order("channel_id asc").
		Find(&channelRefs).Error; err != nil {
		return providerModelUsageSummary{}, err
	}

	type groupModelRef struct {
		Group string `gorm:"column:group"`
	}
	groupRefs := make([]groupModelRef, 0)
	groupCol := `"group"`
	if err := db.Model(&model.GroupModel{}).
		Select("DISTINCT "+groupCol).
		Where("provider = ? AND enabled = ? AND model = ?", normalizedProvider, true, normalizedModel).
		Order(groupCol + " asc").
		Find(&groupRefs).Error; err != nil {
		return providerModelUsageSummary{}, err
	}

	type routeRef struct {
		Group string `gorm:"column:group"`
	}
	routeRefs := make([]routeRef, 0)
	if err := db.Model(&model.GroupModelChannel{}).
		Select("DISTINCT "+groupCol).
		Where("provider = ? AND (model = ? OR upstream_model = ?)", normalizedProvider, normalizedModel, normalizedModel).
		Order(groupCol + " asc").
		Find(&routeRefs).Error; err != nil {
		return providerModelUsageSummary{}, err
	}

	summary := providerModelUsageSummary{
		ChannelModels:      make([]string, 0, len(channelRefs)),
		GroupModels:        make([]string, 0, len(groupRefs)),
		GroupModelChannels: make([]string, 0, len(routeRefs)),
	}
	for _, item := range channelRefs {
		channelID := strings.TrimSpace(item.ChannelID)
		if channelID != "" {
			summary.ChannelModels = append(summary.ChannelModels, channelID)
		}
	}
	channelLabels, err := formatChannelUsageLabelsWithDB(db, summary.ChannelModels)
	if err != nil {
		return providerModelUsageSummary{}, err
	}
	summary.ChannelModels = channelLabels
	for _, item := range groupRefs {
		groupID := strings.TrimSpace(item.Group)
		if groupID != "" {
			summary.GroupModels = append(summary.GroupModels, groupID)
		}
	}
	for _, item := range routeRefs {
		groupID := strings.TrimSpace(item.Group)
		if groupID != "" {
			summary.GroupModelChannels = append(summary.GroupModelChannels, groupID)
		}
	}
	return summary, nil
}

func ensureProviderModelsCanSoftDeleteWithDB(db *gorm.DB, provider string, current []model.ProviderModelDetail, existing []model.ProviderModelDetail) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	currentByModel := make(map[string]struct{}, len(current))
	for _, detail := range current {
		modelName := strings.TrimSpace(detail.Model)
		if modelName == "" {
			continue
		}
		currentByModel[modelName] = struct{}{}
	}
	for _, detail := range existing {
		modelName := strings.TrimSpace(detail.Model)
		if modelName == "" {
			continue
		}
		if _, ok := currentByModel[modelName]; ok {
			continue
		}
		usage, err := collectProviderModelUsageWithDB(db, provider, modelName)
		if err != nil {
			return err
		}
		if usage.InUse() {
			return usage.Error(provider, modelName)
		}
	}
	return nil
}

func normalizeProviderID(item providerItem) string {
	id := commonutils.NormalizeProvider(item.ID)
	if id == "" {
		id = commonutils.NormalizeProvider(item.Provider)
	}
	if id == "" {
		id = commonutils.NormalizeProvider(item.Name)
	}
	return id
}

func normalizeProviderSortOrder(sortOrder int) int {
	if sortOrder > 0 {
		return sortOrder
	}
	return 0
}

func parseProviderPageParams(c *gin.Context) (page int, pageSize int) {
	pageSize = defaultProviderPageSize
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	if pageSize > maxProviderPageSize {
		pageSize = maxProviderPageSize
	}
	page = 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	return page, pageSize
}

func buildProviderItems(rows []model.Provider) ([]providerItem, error) {
	if len(rows) == 0 {
		return []providerItem{}, nil
	}
	providers := make([]string, 0, len(rows))
	for _, row := range rows {
		provider := commonutils.NormalizeProvider(row.Id)
		if provider == "" {
			continue
		}
		providers = append(providers, provider)
	}
	detailsByProvider, err := model.LoadProviderModelDetailsMapForProviders(model.DB, providers)
	if err != nil {
		return nil, err
	}
	items := make([]providerItem, 0, len(rows))
	for _, row := range rows {
		provider := commonutils.NormalizeProvider(row.Id)
		if provider == "" {
			continue
		}
		details := model.FilterActiveProviderModelDetails(detailsByProvider[provider])
		items = append(items, providerItem{
			ID:           provider,
			Name:         strings.TrimSpace(row.Name),
			Models:       providerModelNames(details),
			ModelDetails: details,
			BaseURL:      strings.TrimSpace(row.BaseURL),
			OfficialURL:  strings.TrimSpace(row.OfficialURL),
			SortOrder:    normalizeProviderSortOrder(row.SortOrder),
			Source:       strings.TrimSpace(strings.ToLower(row.Source)),
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
		})
	}
	return items, nil
}

func buildProviderListQuery(keyword string) *gorm.DB {
	query := model.DB.Model(&model.Provider{})
	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if normalizedKeyword == "" {
		return query
	}
	likeKeyword := "%" + normalizedKeyword + "%"
	return query.Where(
		`LOWER(id) LIKE ? OR LOWER(name) LIKE ? OR LOWER(COALESCE(base_url, '')) LIKE ? OR LOWER(COALESCE(official_url, '')) LIKE ? OR LOWER(source) LIKE ? OR EXISTS (SELECT 1 FROM `+model.ProviderModelsTableName+` pm WHERE pm.provider = providers.id AND pm.is_deleted = false AND LOWER(pm.model) LIKE ?)`,
		likeKeyword,
		likeKeyword,
		likeKeyword,
		likeKeyword,
		likeKeyword,
		likeKeyword,
	)
}

func listProvidersPage(page int, pageSize int, keyword string) (providerListData, error) {
	if page < 1 {
		page = 1
	}
	total := int64(0)
	if err := buildProviderListQuery(keyword).Count(&total).Error; err != nil {
		return providerListData{}, err
	}
	rows := make([]model.Provider, 0)
	if err := buildProviderListQuery(keyword).
		Order("sort_order asc, id asc").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&rows).Error; err != nil {
		return providerListData{}, err
	}
	items, err := buildProviderItems(rows)
	if err != nil {
		return providerListData{}, err
	}
	return providerListData{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func listPublicProviderModels() ([]publicProviderModelItem, error) {
	rows := make([]model.Provider, 0)
	if err := model.DB.Model(&model.Provider{}).
		Order("sort_order asc, id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items, err := buildProviderItems(rows)
	if err != nil {
		return nil, err
	}
	result := make([]publicProviderModelItem, 0, len(items))
	for _, item := range items {
		details := make([]publicProviderModelDetail, 0, len(item.ModelDetails))
		for _, detail := range item.ModelDetails {
			details = append(details, publicProviderModelDetail{
				Model:              strings.TrimSpace(detail.Model),
				Tags:               detail.Tags,
				Status:             strings.TrimSpace(detail.Status),
				Description:        strings.TrimSpace(detail.Description),
				Specification:      detail.Specification,
				SupportedEndpoints: detail.SupportedEndpoints,
			})
		}
		result = append(result, publicProviderModelItem{
			ID:        item.ID,
			Name:      item.Name,
			Models:    details,
			SortOrder: item.SortOrder,
			UpdatedAt: item.UpdatedAt,
		})
	}
	return result, nil
}

func getProviderItemByID(id string) (providerItem, error) {
	provider := commonutils.NormalizeProvider(id)
	if provider == "" {
		return providerItem{}, gorm.ErrRecordNotFound
	}
	row := model.Provider{}
	if err := model.DB.First(&row, "id = ?", provider).Error; err != nil {
		return providerItem{}, err
	}
	items, err := buildProviderItems([]model.Provider{row})
	if err != nil {
		return providerItem{}, err
	}
	if len(items) == 0 {
		return providerItem{}, gorm.ErrRecordNotFound
	}
	return items[0], nil
}

func nextProviderSortOrder(tx *gorm.DB) (int, error) {
	nextOrder := 10
	if err := tx.Model(&model.Provider{}).Select("COALESCE(MAX(sort_order), 0) + 10").Scan(&nextOrder).Error; err != nil {
		return 0, err
	}
	if nextOrder <= 0 {
		nextOrder = 10
	}
	return nextOrder, nil
}

func normalizeProviderUpsertItem(db *gorm.DB, providerID string, item providerItem, existing *providerItem, defaultSortOrder int) (providerItem, error) {
	provider := commonutils.NormalizeProvider(providerID)
	bodyProvider := normalizeProviderID(item)
	if provider == "" {
		provider = bodyProvider
	}
	if provider == "" {
		return providerItem{}, errors.New("供应商标识不能为空")
	}
	if bodyProvider != "" && bodyProvider != provider {
		return providerItem{}, errors.New("供应商标识不匹配")
	}

	now := helper.GetTimestamp()
	name := strings.TrimSpace(item.Name)
	if name == "" {
		if existing != nil && strings.TrimSpace(existing.Name) != "" {
			name = strings.TrimSpace(existing.Name)
		} else {
			name = provider
		}
	}

	source := strings.TrimSpace(strings.ToLower(item.Source))
	if source == "" {
		if existing != nil && strings.TrimSpace(existing.Source) != "" {
			source = strings.TrimSpace(strings.ToLower(existing.Source))
		} else {
			source = "manual"
		}
	}

	baseURL := strings.TrimSpace(item.BaseURL)
	if baseURL == "" && existing != nil {
		baseURL = strings.TrimSpace(existing.BaseURL)
	}
	officialURL := strings.TrimSpace(item.OfficialURL)
	if officialURL == "" && existing != nil {
		officialURL = strings.TrimSpace(existing.OfficialURL)
	}

	detailInput := make([]model.ProviderModelDetail, 0, len(item.ModelDetails)+len(item.Models))
	detailInput = append(detailInput, item.ModelDetails...)
	for _, modelName := range item.Models {
		detailInput = append(detailInput, model.ProviderModelDetail{Model: strings.TrimSpace(modelName)})
	}
	if err := validateProviderModelTags(detailInput); err != nil {
		return providerItem{}, err
	}

	details := mergeProviderDetailInputs(detailInput, item.Models, now)
	if len(details) == 0 && existing != nil {
		details = mergeProviderDetailInputs(existing.ModelDetails, existing.Models, now)
	}
	if existing != nil {
		if err := ensureProviderModelsCanSoftDeleteWithDB(db, provider, details, existing.ModelDetails); err != nil {
			return providerItem{}, err
		}
	}
	if existing != nil {
		details = mergeMissingProviderDetailsAsDeleted(details, existing.ModelDetails)
	}
	details = applyProviderModelEndpointDefaults(provider, details)

	sortOrder := normalizeProviderSortOrder(item.SortOrder)
	if sortOrder <= 0 && existing != nil {
		sortOrder = normalizeProviderSortOrder(existing.SortOrder)
	}
	if sortOrder <= 0 {
		sortOrder = normalizeProviderSortOrder(defaultSortOrder)
	}
	if sortOrder <= 0 {
		sortOrder = 10
	}

	return providerItem{
		ID:           provider,
		Name:         name,
		Models:       providerModelNames(details),
		ModelDetails: details,
		BaseURL:      baseURL,
		OfficialURL:  officialURL,
		SortOrder:    sortOrder,
		Source:       source,
		CreatedAt: func() int64 {
			if existing != nil && existing.CreatedAt > 0 {
				return existing.CreatedAt
			}
			return now
		}(),
		UpdatedAt: now,
	}, nil
}

func saveProviderItem(item providerItem, create bool) (providerItem, error) {
	resolvedID := normalizeProviderID(item)
	existing, err := getProviderItemByID(resolvedID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return providerItem{}, err
	}
	existingFound := err == nil
	if create && existingFound {
		return providerItem{}, errors.New("该供应商已存在，请直接编辑")
	}
	if !create && !existingFound {
		return providerItem{}, gorm.ErrRecordNotFound
	}

	tx := model.DB.Begin()
	if tx.Error != nil {
		return providerItem{}, tx.Error
	}

	defaultSortOrder := 0
	if !existingFound {
		defaultSortOrder, err = nextProviderSortOrder(tx)
		if err != nil {
			_ = tx.Rollback()
			return providerItem{}, err
		}
	}

	var existingPtr *providerItem
	if existingFound {
		existingCopy := existing
		existingPtr = &existingCopy
	}
	normalized, err := normalizeProviderUpsertItem(tx, resolvedID, item, existingPtr, defaultSortOrder)
	if err != nil {
		_ = tx.Rollback()
		return providerItem{}, err
	}

	providerRow := model.Provider{
		Id:          normalized.ID,
		Name:        strings.TrimSpace(normalized.Name),
		BaseURL:     strings.TrimSpace(normalized.BaseURL),
		OfficialURL: strings.TrimSpace(normalized.OfficialURL),
		SortOrder:   normalized.SortOrder,
		Source:      strings.TrimSpace(strings.ToLower(normalized.Source)),
		CreatedAt:   normalized.CreatedAt,
		UpdatedAt:   normalized.UpdatedAt,
	}
	if create {
		if err := tx.Create(&providerRow).Error; err != nil {
			_ = tx.Rollback()
			return providerItem{}, err
		}
	} else {
		result := tx.Model(&model.Provider{}).
			Where("id = ?", normalized.ID).
			Updates(map[string]any{
				"name":         providerRow.Name,
				"base_url":     providerRow.BaseURL,
				"official_url": providerRow.OfficialURL,
				"sort_order":   providerRow.SortOrder,
				"source":       providerRow.Source,
				"updated_at":   providerRow.UpdatedAt,
			})
		if result.Error != nil {
			_ = tx.Rollback()
			return providerItem{}, result.Error
		}
		if result.RowsAffected == 0 {
			_ = tx.Rollback()
			return providerItem{}, gorm.ErrRecordNotFound
		}
	}

	if err := tx.Where("provider = ?", normalized.ID).Delete(&model.ProviderModelPriceComponent{}).Error; err != nil {
		_ = tx.Rollback()
		return providerItem{}, err
	}
	if err := tx.Where("provider = ?", normalized.ID).Delete(&model.ProviderModel{}).Error; err != nil {
		_ = tx.Rollback()
		return providerItem{}, err
	}
	storeRows := model.BuildProviderModelStoreRows(normalized.ID, normalized.ModelDetails, normalized.UpdatedAt)
	if len(storeRows.Models) > 0 {
		if err := tx.Create(&storeRows.Models).Error; err != nil {
			_ = tx.Rollback()
			return providerItem{}, err
		}
	}
	if len(storeRows.PriceComponents) > 0 {
		if err := tx.Create(&storeRows.PriceComponents).Error; err != nil {
			_ = tx.Rollback()
			return providerItem{}, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return providerItem{}, err
	}
	if err := model.SyncModelPricingCatalogWithDB(model.DB); err != nil {
		return providerItem{}, err
	}
	return getProviderItemByID(normalized.ID)
}

func deleteProviderItem(id string) error {
	provider := commonutils.NormalizeProvider(id)
	if provider == "" {
		return errors.New("供应商标识不能为空")
	}
	tx := model.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	if err := tx.Where("provider = ?", provider).Delete(&model.ProviderModelPriceComponent{}).Error; err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Where("provider = ?", provider).Delete(&model.ProviderModel{}).Error; err != nil {
		_ = tx.Rollback()
		return err
	}
	result := tx.Where("id = ?", provider).Delete(&model.Provider{})
	if result.Error != nil {
		_ = tx.Rollback()
		return result.Error
	}
	if result.RowsAffected == 0 {
		_ = tx.Rollback()
		return gorm.ErrRecordNotFound
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	return model.SyncModelPricingCatalogWithDB(model.DB)
}

func appendModelToProviderItem(id string, req appendProviderModelRequest) (providerItem, error) {
	existing, err := getProviderItemByID(id)
	if err != nil {
		return providerItem{}, err
	}

	now := helper.GetTimestamp()
	detail := model.ProviderModelDetail{
		Model:              strings.TrimSpace(req.Model),
		Tags:               req.Tags,
		Status:             req.Status,
		Description:        strings.TrimSpace(req.Description),
		IsDeleted:          req.IsDeleted,
		SupportedEndpoints: req.SupportedEndpoints,
		InputPrice:         req.InputPrice,
		OutputPrice:        req.OutputPrice,
		PriceUnit:          strings.TrimSpace(strings.ToLower(req.PriceUnit)),
		Currency:           strings.TrimSpace(strings.ToUpper(req.Currency)),
		Source:             strings.TrimSpace(strings.ToLower(req.Source)),
		UpdatedAt:          now,
	}
	if detail.Model == "" {
		return providerItem{}, errors.New("模型名称不能为空")
	}
	if err := validateProviderModelTags([]model.ProviderModelDetail{detail}); err != nil {
		return providerItem{}, err
	}
	if detail.Source == "" {
		detail.Source = "manual"
	}

	existing.ModelDetails = mergeProviderDetailInputs(append(existing.ModelDetails, detail), nil, now)
	existing.UpdatedAt = now
	return saveProviderItem(existing, false)
}

func GetPublicProviderModels(c *gin.Context) {
	items, err := listPublicProviderModels()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "加载供应商模型目录失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    items,
	})
}

func GetProviders(c *gin.Context) {
	page, pageSize := parseProviderPageParams(c)
	data, err := listProvidersPage(page, pageSize, c.Query("keyword"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "加载供应商列表失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

func GetProvider(c *gin.Context) {
	item, err := getProviderItemByID(c.Param("id"))
	if err != nil {
		message := "加载供应商详情失败: " + err.Error()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			message = "供应商不存在"
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": message,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    item,
	})
}

func CreateProvider(c *gin.Context) {
	req := providerItem{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	saved, err := saveProviderItem(req, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "新增供应商失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    saved,
	})
}

func UpdateProvider(c *gin.Context) {
	req := providerItem{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	req.ID = strings.TrimSpace(c.Param("id"))
	saved, err := saveProviderItem(req, false)
	if err != nil {
		message := "保存供应商失败: " + err.Error()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			message = "供应商不存在"
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": message,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    saved,
	})
}

func AppendProviderModel(c *gin.Context) {
	req := appendProviderModelRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	saved, err := appendModelToProviderItem(c.Param("id"), req)
	if err != nil {
		message := "录入供应商模型失败: " + err.Error()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			message = "供应商不存在"
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": message,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    saved,
	})
}

func DeleteProvider(c *gin.Context) {
	if err := deleteProviderItem(c.Param("id")); err != nil {
		message := "删除供应商失败: " + err.Error()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			message = "供应商不存在"
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": message,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
