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
	defaultModelProviderPageSize = 10
	maxModelProviderPageSize     = 100
)

type modelProviderCatalogItem struct {
	ID           string                           `json:"id"`
	Provider     string                           `json:"provider,omitempty"`
	Name         string                           `json:"name,omitempty"`
	Models       []string                         `json:"models"`
	ModelDetails []model.ModelProviderModelDetail `json:"model_details,omitempty"`
	BaseURL      string                           `json:"base_url,omitempty"`
	SortOrder    int                              `json:"sort_order,omitempty"`
	Source       string                           `json:"source,omitempty"`
	UpdatedAt    int64                            `json:"updated_at,omitempty"`
}

type modelProviderCatalogListData struct {
	Items    []modelProviderCatalogItem `json:"items"`
	Total    int64                      `json:"total"`
	Page     int                        `json:"page"`
	PageSize int                        `json:"page_size"`
}

func normalizeModelProviderCatalogID(item modelProviderCatalogItem) string {
	id := commonutils.NormalizeModelProvider(item.ID)
	if id == "" {
		id = commonutils.NormalizeModelProvider(item.Provider)
	}
	if id == "" {
		id = commonutils.NormalizeModelProvider(item.Name)
	}
	return id
}

func normalizeCatalogSortOrder(sortOrder int) int {
	if sortOrder > 0 {
		return sortOrder
	}
	return 0
}

func parseModelProviderPageParams(c *gin.Context) (page int, pageSize int) {
	pageSize = defaultModelProviderPageSize
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	if pageSize > maxModelProviderPageSize {
		pageSize = maxModelProviderPageSize
	}
	page = 0
	if raw := strings.TrimSpace(c.Query("p")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			page = parsed
		}
	}
	return page, pageSize
}

func buildModelProviderCatalogItems(rows []model.ModelProvider) ([]modelProviderCatalogItem, error) {
	if len(rows) == 0 {
		return []modelProviderCatalogItem{}, nil
	}
	providers := make([]string, 0, len(rows))
	for _, row := range rows {
		provider := commonutils.NormalizeModelProvider(row.Id)
		if provider == "" {
			continue
		}
		providers = append(providers, provider)
	}
	detailsByProvider, err := model.LoadModelProviderModelDetailsMapForProviders(model.DB, providers)
	if err != nil {
		return nil, err
	}
	items := make([]modelProviderCatalogItem, 0, len(rows))
	now := helper.GetTimestamp()
	for _, row := range rows {
		provider := commonutils.NormalizeModelProvider(row.Id)
		if provider == "" {
			continue
		}
		details := model.MergeModelProviderDetails(provider, detailsByProvider[provider], nil, false, now)
		items = append(items, modelProviderCatalogItem{
			ID:           provider,
			Name:         strings.TrimSpace(row.Name),
			Models:       model.ModelProviderModelNames(details),
			ModelDetails: details,
			BaseURL:      strings.TrimSpace(row.BaseURL),
			SortOrder:    normalizeCatalogSortOrder(row.SortOrder),
			Source:       strings.TrimSpace(strings.ToLower(row.Source)),
			UpdatedAt:    row.UpdatedAt,
		})
	}
	return items, nil
}

func buildModelProviderListQuery(keyword string) *gorm.DB {
	query := model.DB.Model(&model.ModelProvider{})
	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if normalizedKeyword == "" {
		return query
	}
	likeKeyword := "%" + normalizedKeyword + "%"
	return query.Where(
		`LOWER(id) LIKE ? OR LOWER(name) LIKE ? OR LOWER(COALESCE(base_url, '')) LIKE ? OR LOWER(source) LIKE ? OR EXISTS (SELECT 1 FROM `+model.ModelProviderModelsTableName+` pm WHERE pm.provider = providers.id AND LOWER(pm.model) LIKE ?)`,
		likeKeyword,
		likeKeyword,
		likeKeyword,
		likeKeyword,
		likeKeyword,
	)
}

func listModelProviderCatalog(page int, pageSize int, keyword string) (modelProviderCatalogListData, error) {
	total := int64(0)
	if err := buildModelProviderListQuery(keyword).Count(&total).Error; err != nil {
		return modelProviderCatalogListData{}, err
	}
	rows := make([]model.ModelProvider, 0)
	if err := buildModelProviderListQuery(keyword).
		Order("sort_order asc, id asc").
		Limit(pageSize).
		Offset(page * pageSize).
		Find(&rows).Error; err != nil {
		return modelProviderCatalogListData{}, err
	}
	items, err := buildModelProviderCatalogItems(rows)
	if err != nil {
		return modelProviderCatalogListData{}, err
	}
	return modelProviderCatalogListData{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func getModelProviderCatalogItemByID(id string) (modelProviderCatalogItem, error) {
	provider := commonutils.NormalizeModelProvider(id)
	if provider == "" {
		return modelProviderCatalogItem{}, gorm.ErrRecordNotFound
	}
	row := model.ModelProvider{}
	if err := model.DB.First(&row, "id = ?", provider).Error; err != nil {
		return modelProviderCatalogItem{}, err
	}
	items, err := buildModelProviderCatalogItems([]model.ModelProvider{row})
	if err != nil {
		return modelProviderCatalogItem{}, err
	}
	if len(items) == 0 {
		return modelProviderCatalogItem{}, gorm.ErrRecordNotFound
	}
	return items[0], nil
}

func nextModelProviderSortOrder(tx *gorm.DB) (int, error) {
	nextOrder := 10
	if err := tx.Model(&model.ModelProvider{}).Select("COALESCE(MAX(sort_order), 0) + 10").Scan(&nextOrder).Error; err != nil {
		return 0, err
	}
	if nextOrder <= 0 {
		nextOrder = 10
	}
	return nextOrder, nil
}

func normalizeModelProviderUpsertItem(providerID string, item modelProviderCatalogItem, existing *modelProviderCatalogItem, defaultSortOrder int) (modelProviderCatalogItem, error) {
	provider := commonutils.NormalizeModelProvider(providerID)
	bodyProvider := normalizeModelProviderCatalogID(item)
	if provider == "" {
		provider = bodyProvider
	}
	if provider == "" {
		return modelProviderCatalogItem{}, errors.New("供应商标识不能为空")
	}
	if bodyProvider != "" && bodyProvider != provider {
		return modelProviderCatalogItem{}, errors.New("供应商标识不匹配")
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

	detailInput := make([]model.ModelProviderModelDetail, 0, len(item.ModelDetails)+len(item.Models))
	detailInput = append(detailInput, item.ModelDetails...)
	for _, modelName := range item.Models {
		detailInput = append(detailInput, model.ModelProviderModelDetail{Model: strings.TrimSpace(modelName)})
	}

	details := model.MergeModelProviderDetails(provider, detailInput, item.Models, false, now)
	if len(details) == 0 && existing != nil {
		details = model.MergeModelProviderDetails(provider, existing.ModelDetails, existing.Models, false, now)
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

	return modelProviderCatalogItem{
		ID:           provider,
		Name:         name,
		Models:       model.ModelProviderModelNames(details),
		ModelDetails: details,
		BaseURL:      baseURL,
		SortOrder:    sortOrder,
		Source:       source,
		UpdatedAt:    now,
	}, nil
}

func saveModelProviderCatalogItem(item modelProviderCatalogItem, create bool) (modelProviderCatalogItem, error) {
	resolvedID := normalizeModelProviderCatalogID(item)
	existing, err := getModelProviderCatalogItemByID(resolvedID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return modelProviderCatalogItem{}, err
	}
	existingFound := err == nil
	if create && existingFound {
		return modelProviderCatalogItem{}, errors.New("该供应商已存在，请直接编辑")
	}
	if !create && !existingFound {
		return modelProviderCatalogItem{}, gorm.ErrRecordNotFound
	}

	tx := model.DB.Begin()
	if tx.Error != nil {
		return modelProviderCatalogItem{}, tx.Error
	}

	defaultSortOrder := 0
	if !existingFound {
		defaultSortOrder, err = nextModelProviderSortOrder(tx)
		if err != nil {
			_ = tx.Rollback()
			return modelProviderCatalogItem{}, err
		}
	}

	var existingPtr *modelProviderCatalogItem
	if existingFound {
		existingCopy := existing
		existingPtr = &existingCopy
	}
	normalized, err := normalizeModelProviderUpsertItem(resolvedID, item, existingPtr, defaultSortOrder)
	if err != nil {
		_ = tx.Rollback()
		return modelProviderCatalogItem{}, err
	}

	providerRow := model.ModelProvider{
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
			return modelProviderCatalogItem{}, err
		}
	} else {
		result := tx.Model(&model.ModelProvider{}).
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
			return modelProviderCatalogItem{}, result.Error
		}
		if result.RowsAffected == 0 {
			_ = tx.Rollback()
			return modelProviderCatalogItem{}, gorm.ErrRecordNotFound
		}
	}

	if err := tx.Where("provider = ?", normalized.ID).Delete(&model.ModelProviderModel{}).Error; err != nil {
		_ = tx.Rollback()
		return modelProviderCatalogItem{}, err
	}
	modelRows := model.BuildModelProviderModelRows(normalized.ID, normalized.ModelDetails, normalized.UpdatedAt)
	if len(modelRows) > 0 {
		if err := tx.Create(&modelRows).Error; err != nil {
			_ = tx.Rollback()
			return modelProviderCatalogItem{}, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return modelProviderCatalogItem{}, err
	}
	if err := model.SyncModelPricingCatalogWithDB(model.DB); err != nil {
		return modelProviderCatalogItem{}, err
	}
	return getModelProviderCatalogItemByID(normalized.ID)
}

func deleteModelProviderCatalogItem(id string) error {
	provider := commonutils.NormalizeModelProvider(id)
	if provider == "" {
		return errors.New("供应商标识不能为空")
	}
	tx := model.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	if err := tx.Where("provider = ?", provider).Delete(&model.ModelProviderModel{}).Error; err != nil {
		_ = tx.Rollback()
		return err
	}
	result := tx.Where("id = ?", provider).Delete(&model.ModelProvider{})
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

// GetModelProviders godoc
// @Summary Get paged model provider catalog (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/provider [get]
func GetModelProviders(c *gin.Context) {
	page, pageSize := parseModelProviderPageParams(c)
	data, err := listModelProviderCatalog(page, pageSize, c.Query("keyword"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "加载模型供应商列表失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

// GetModelProvider godoc
// @Summary Get model provider detail (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Provider ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/provider/{id} [get]
func GetModelProvider(c *gin.Context) {
	item, err := getModelProviderCatalogItemByID(c.Param("id"))
	if err != nil {
		message := "加载模型供应商详情失败: " + err.Error()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			message = "模型供应商不存在"
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

// CreateModelProvider godoc
// @Summary Create model provider (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/provider [post]
func CreateModelProvider(c *gin.Context) {
	req := modelProviderCatalogItem{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	saved, err := saveModelProviderCatalogItem(req, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "新增模型供应商失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    saved,
	})
}

// UpdateModelProvider godoc
// @Summary Update model provider (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Provider ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/provider/{id} [put]
func UpdateModelProvider(c *gin.Context) {
	req := modelProviderCatalogItem{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	req.ID = strings.TrimSpace(c.Param("id"))
	saved, err := saveModelProviderCatalogItem(req, false)
	if err != nil {
		message := "保存模型供应商失败: " + err.Error()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			message = "模型供应商不存在"
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

// DeleteModelProvider godoc
// @Summary Delete model provider (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Provider ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/provider/{id} [delete]
func DeleteModelProvider(c *gin.Context) {
	if err := deleteModelProviderCatalogItem(c.Param("id")); err != nil {
		message := "删除模型供应商失败: " + err.Error()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			message = "模型供应商不存在"
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
