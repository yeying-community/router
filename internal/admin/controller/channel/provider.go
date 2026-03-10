package channel

import (
	"errors"
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

type providerCatalogItem struct {
	ID           string                      `json:"id"`
	Provider     string                      `json:"provider,omitempty"`
	Name         string                      `json:"name,omitempty"`
	Models       []string                    `json:"models"`
	ModelDetails []model.ProviderModelDetail `json:"model_details,omitempty"`
	BaseURL      string                      `json:"base_url,omitempty"`
	SortOrder    int                         `json:"sort_order,omitempty"`
	Source       string                      `json:"source,omitempty"`
	UpdatedAt    int64                       `json:"updated_at,omitempty"`
}

type providerCatalogListData struct {
	Items    []providerCatalogItem `json:"items"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

type appendProviderModelRequest struct {
	Model       string  `json:"model"`
	Type        string  `json:"type,omitempty"`
	InputPrice  float64 `json:"input_price,omitempty"`
	OutputPrice float64 `json:"output_price,omitempty"`
	PriceUnit   string  `json:"price_unit,omitempty"`
	Currency    string  `json:"currency,omitempty"`
	Source      string  `json:"source,omitempty"`
}

func providerModelNames(details []model.ProviderModelDetail) []string {
	normalized := model.NormalizeProviderModelDetails(details)
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

func normalizeProviderCatalogID(item providerCatalogItem) string {
	id := commonutils.NormalizeProvider(item.ID)
	if id == "" {
		id = commonutils.NormalizeProvider(item.Provider)
	}
	if id == "" {
		id = commonutils.NormalizeProvider(item.Name)
	}
	return id
}

func normalizeCatalogSortOrder(sortOrder int) int {
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

func buildProviderCatalogItems(rows []model.Provider) ([]providerCatalogItem, error) {
	if len(rows) == 0 {
		return []providerCatalogItem{}, nil
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
	items := make([]providerCatalogItem, 0, len(rows))
	for _, row := range rows {
		provider := commonutils.NormalizeProvider(row.Id)
		if provider == "" {
			continue
		}
		details := model.NormalizeProviderModelDetails(detailsByProvider[provider])
		items = append(items, providerCatalogItem{
			ID:           provider,
			Name:         strings.TrimSpace(row.Name),
			Models:       providerModelNames(details),
			ModelDetails: details,
			BaseURL:      strings.TrimSpace(row.BaseURL),
			SortOrder:    normalizeCatalogSortOrder(row.SortOrder),
			Source:       strings.TrimSpace(strings.ToLower(row.Source)),
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
		`LOWER(id) LIKE ? OR LOWER(name) LIKE ? OR LOWER(COALESCE(base_url, '')) LIKE ? OR LOWER(source) LIKE ? OR EXISTS (SELECT 1 FROM `+model.ProviderModelsTableName+` pm WHERE pm.provider = providers.id AND LOWER(pm.model) LIKE ?)`,
		likeKeyword,
		likeKeyword,
		likeKeyword,
		likeKeyword,
		likeKeyword,
	)
}

func listProviderCatalog(page int, pageSize int, keyword string) (providerCatalogListData, error) {
	if page < 1 {
		page = 1
	}
	total := int64(0)
	if err := buildProviderListQuery(keyword).Count(&total).Error; err != nil {
		return providerCatalogListData{}, err
	}
	rows := make([]model.Provider, 0)
	if err := buildProviderListQuery(keyword).
		Order("sort_order asc, id asc").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&rows).Error; err != nil {
		return providerCatalogListData{}, err
	}
	items, err := buildProviderCatalogItems(rows)
	if err != nil {
		return providerCatalogListData{}, err
	}
	return providerCatalogListData{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func getProviderCatalogItemByID(id string) (providerCatalogItem, error) {
	provider := commonutils.NormalizeProvider(id)
	if provider == "" {
		return providerCatalogItem{}, gorm.ErrRecordNotFound
	}
	row := model.Provider{}
	if err := model.DB.First(&row, "id = ?", provider).Error; err != nil {
		return providerCatalogItem{}, err
	}
	items, err := buildProviderCatalogItems([]model.Provider{row})
	if err != nil {
		return providerCatalogItem{}, err
	}
	if len(items) == 0 {
		return providerCatalogItem{}, gorm.ErrRecordNotFound
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

func normalizeProviderUpsertItem(providerID string, item providerCatalogItem, existing *providerCatalogItem, defaultSortOrder int) (providerCatalogItem, error) {
	provider := commonutils.NormalizeProvider(providerID)
	bodyProvider := normalizeProviderCatalogID(item)
	if provider == "" {
		provider = bodyProvider
	}
	if provider == "" {
		return providerCatalogItem{}, errors.New("供应商标识不能为空")
	}
	if bodyProvider != "" && bodyProvider != provider {
		return providerCatalogItem{}, errors.New("供应商标识不匹配")
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

	detailInput := make([]model.ProviderModelDetail, 0, len(item.ModelDetails)+len(item.Models))
	detailInput = append(detailInput, item.ModelDetails...)
	for _, modelName := range item.Models {
		detailInput = append(detailInput, model.ProviderModelDetail{Model: strings.TrimSpace(modelName)})
	}

	details := mergeProviderDetailInputs(detailInput, item.Models, now)
	if len(details) == 0 && existing != nil {
		details = mergeProviderDetailInputs(existing.ModelDetails, existing.Models, now)
	}

	sortOrder := normalizeCatalogSortOrder(item.SortOrder)
	if sortOrder <= 0 && existing != nil {
		sortOrder = normalizeCatalogSortOrder(existing.SortOrder)
	}
	if sortOrder <= 0 {
		sortOrder = normalizeCatalogSortOrder(defaultSortOrder)
	}
	if sortOrder <= 0 {
		sortOrder = 10
	}

	return providerCatalogItem{
		ID:           provider,
		Name:         name,
		Models:       providerModelNames(details),
		ModelDetails: details,
		BaseURL:      baseURL,
		SortOrder:    sortOrder,
		Source:       source,
		UpdatedAt:    now,
	}, nil
}

func saveProviderCatalogItem(item providerCatalogItem, create bool) (providerCatalogItem, error) {
	resolvedID := normalizeProviderCatalogID(item)
	existing, err := getProviderCatalogItemByID(resolvedID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return providerCatalogItem{}, err
	}
	existingFound := err == nil
	if create && existingFound {
		return providerCatalogItem{}, errors.New("该供应商已存在，请直接编辑")
	}
	if !create && !existingFound {
		return providerCatalogItem{}, gorm.ErrRecordNotFound
	}

	tx := model.DB.Begin()
	if tx.Error != nil {
		return providerCatalogItem{}, tx.Error
	}

	defaultSortOrder := 0
	if !existingFound {
		defaultSortOrder, err = nextProviderSortOrder(tx)
		if err != nil {
			_ = tx.Rollback()
			return providerCatalogItem{}, err
		}
	}

	var existingPtr *providerCatalogItem
	if existingFound {
		existingCopy := existing
		existingPtr = &existingCopy
	}
	normalized, err := normalizeProviderUpsertItem(resolvedID, item, existingPtr, defaultSortOrder)
	if err != nil {
		_ = tx.Rollback()
		return providerCatalogItem{}, err
	}

	providerRow := model.Provider{
		Id:        normalized.ID,
		Name:      strings.TrimSpace(normalized.Name),
		BaseURL:   strings.TrimSpace(normalized.BaseURL),
		SortOrder: normalized.SortOrder,
		Source:    strings.TrimSpace(strings.ToLower(normalized.Source)),
		UpdatedAt: normalized.UpdatedAt,
	}
	if create {
		if err := tx.Create(&providerRow).Error; err != nil {
			_ = tx.Rollback()
			return providerCatalogItem{}, err
		}
	} else {
		result := tx.Model(&model.Provider{}).
			Where("id = ?", normalized.ID).
			Updates(map[string]any{
				"name":       providerRow.Name,
				"base_url":   providerRow.BaseURL,
				"sort_order": providerRow.SortOrder,
				"source":     providerRow.Source,
				"updated_at": providerRow.UpdatedAt,
			})
		if result.Error != nil {
			_ = tx.Rollback()
			return providerCatalogItem{}, result.Error
		}
		if result.RowsAffected == 0 {
			_ = tx.Rollback()
			return providerCatalogItem{}, gorm.ErrRecordNotFound
		}
	}

	if err := tx.Where("provider = ?", normalized.ID).Delete(&model.ProviderModel{}).Error; err != nil {
		_ = tx.Rollback()
		return providerCatalogItem{}, err
	}
	modelRows := model.BuildProviderModelRows(normalized.ID, normalized.ModelDetails, normalized.UpdatedAt)
	if len(modelRows) > 0 {
		if err := tx.Create(&modelRows).Error; err != nil {
			_ = tx.Rollback()
			return providerCatalogItem{}, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return providerCatalogItem{}, err
	}
	if err := model.SyncModelPricingCatalogWithDB(model.DB); err != nil {
		return providerCatalogItem{}, err
	}
	return getProviderCatalogItemByID(normalized.ID)
}

func deleteProviderCatalogItem(id string) error {
	provider := commonutils.NormalizeProvider(id)
	if provider == "" {
		return errors.New("供应商标识不能为空")
	}
	tx := model.DB.Begin()
	if tx.Error != nil {
		return tx.Error
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

func appendModelToProviderItem(id string, req appendProviderModelRequest) (providerCatalogItem, error) {
	existing, err := getProviderCatalogItemByID(id)
	if err != nil {
		return providerCatalogItem{}, err
	}

	now := helper.GetTimestamp()
	detail := model.ProviderModelDetail{
		Model:       strings.TrimSpace(req.Model),
		Type:        strings.TrimSpace(strings.ToLower(req.Type)),
		InputPrice:  req.InputPrice,
		OutputPrice: req.OutputPrice,
		PriceUnit:   strings.TrimSpace(strings.ToLower(req.PriceUnit)),
		Currency:    strings.TrimSpace(strings.ToUpper(req.Currency)),
		Source:      strings.TrimSpace(strings.ToLower(req.Source)),
		UpdatedAt:   now,
	}
	if detail.Model == "" {
		return providerCatalogItem{}, errors.New("模型名称不能为空")
	}
	if detail.Source == "" {
		detail.Source = "manual"
	}

	existing.ModelDetails = mergeProviderDetailInputs(append(existing.ModelDetails, detail), nil, now)
	existing.UpdatedAt = now
	return saveProviderCatalogItem(existing, false)
}

// GetProviders godoc
// @Summary Get paged provider catalog (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page (1-based)"
// @Param page_size query int false "Page size"
// @Param keyword query string false "Keyword"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/providers [get]
func GetProviders(c *gin.Context) {
	page, pageSize := parseProviderPageParams(c)
	data, err := listProviderCatalog(page, pageSize, c.Query("keyword"))
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

// GetProvider godoc
// @Summary Get provider detail (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Provider ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/providers/{id} [get]
func GetProvider(c *gin.Context) {
	item, err := getProviderCatalogItemByID(c.Param("id"))
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

// CreateProvider godoc
// @Summary Create provider (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/providers [post]
func CreateProvider(c *gin.Context) {
	req := providerCatalogItem{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	saved, err := saveProviderCatalogItem(req, true)
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

// UpdateProvider godoc
// @Summary Update provider (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Provider ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/providers/{id} [put]
func UpdateProvider(c *gin.Context) {
	req := providerCatalogItem{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	req.ID = strings.TrimSpace(c.Param("id"))
	saved, err := saveProviderCatalogItem(req, false)
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

// AppendProviderModel godoc
// @Summary Append one model detail into provider catalog (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Provider ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/providers/{id}/model [post]
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

// DeleteProvider godoc
// @Summary Delete provider (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Provider ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/providers/{id} [delete]
func DeleteProvider(c *gin.Context) {
	if err := deleteProviderCatalogItem(c.Param("id")); err != nil {
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
