package plan

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/internal/admin/model"
	plansvc "github.com/yeying-community/router/internal/admin/service/plan"
	"gorm.io/gorm"
)

const maxPackageListPageSize = 100

type packageListPageData struct {
	Items    []model.ServicePackage `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

type upsertServicePackageRequest struct {
	Id                         string   `json:"id"`
	Name                       *string  `json:"name"`
	Description                *string  `json:"description"`
	GroupID                    *string  `json:"group_id"`
	PackageType                *string  `json:"package_type"`
	QuotaMetric                *string  `json:"quota_metric"`
	PeriodType                 *string  `json:"period_type"`
	PeriodLimit                *int64   `json:"period_limit"`
	MaxConcurrencyPerUser      *int     `json:"max_concurrency_per_user"`
	MaxConcurrencyPerPackage   *int     `json:"max_concurrency_per_package"`
	AllowBalanceFallback       *bool    `json:"allow_balance_fallback"`
	VisibilityScope            *string  `json:"visibility_scope"`
	VisibleUserIDs             []string `json:"visible_user_ids"`
	SalePrice                  *float64 `json:"sale_price"`
	SaleCurrency               *string  `json:"sale_currency"`
	DailyQuotaLimit            *int64   `json:"daily_quota_limit"`
	PackageEmergencyQuotaLimit *int64   `json:"package_emergency_quota_limit"`
	DurationDays               *int     `json:"duration_days"`
	QuotaResetTimezone         *string  `json:"quota_reset_timezone"`
	Enabled                    *bool    `json:"enabled"`
	SortOrder                  *int     `json:"sort_order"`
	Source                     *string  `json:"source"`
}

type assignServicePackageRequest struct {
	UserID  string `json:"user_id"`
	StartAt int64  `json:"start_at"`
}

func parsePackageListPageParams(c *gin.Context) (page int, pageSize int, keyword string) {
	page = 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	pageSize = config.ItemsPerPage
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	if pageSize > maxPackageListPageSize {
		pageSize = maxPackageListPageSize
	}
	keyword = strings.TrimSpace(c.Query("keyword"))
	return page, pageSize, keyword
}

func optionalStringValue(ptr *string, fallback string) string {
	if ptr == nil {
		return strings.TrimSpace(fallback)
	}
	return strings.TrimSpace(*ptr)
}

func optionalInt64Value(ptr *int64, fallback int64) int64 {
	if ptr == nil {
		return fallback
	}
	return *ptr
}

func optionalIntValue(ptr *int, fallback int) int {
	if ptr == nil {
		return fallback
	}
	return *ptr
}

func optionalBoolValue(ptr *bool, fallback bool) bool {
	if ptr == nil {
		return fallback
	}
	return *ptr
}

func optionalFloat64Value(ptr *float64, fallback float64) float64 {
	if ptr == nil {
		return fallback
	}
	return *ptr
}

func GetPackages(c *gin.Context) {
	page, pageSize, keyword := parsePackageListPageParams(c)
	rows, total, err := plansvc.ListPage(page, pageSize, keyword)
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
		"data": packageListPageData{
			Items:    rows,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

func GetPublicPackages(c *gin.Context) {
	rows, err := model.ListEnabledServicePackagesForUser(strings.TrimSpace(c.GetString(ctxkey.Id)))
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
		"data":    rows,
	})
}

func GetPackage(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "套餐 ID 不能为空",
		})
		return
	}
	row, err := plansvc.Get(id)
	if err != nil {
		message := err.Error()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			message = "套餐不存在"
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
		"data":    row,
	})
}

func CreatePackage(c *gin.Context) {
	req := upsertServicePackageRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	item := model.ServicePackage{
		Id:                         strings.TrimSpace(req.Id),
		Name:                       optionalStringValue(req.Name, ""),
		Description:                optionalStringValue(req.Description, ""),
		GroupID:                    optionalStringValue(req.GroupID, ""),
		PackageType:                optionalStringValue(req.PackageType, ""),
		ScopeType:                  model.ServicePackageScopeAll,
		QuotaMetric:                optionalStringValue(req.QuotaMetric, ""),
		PeriodType:                 optionalStringValue(req.PeriodType, ""),
		PeriodLimit:                optionalInt64Value(req.PeriodLimit, 0),
		MaxConcurrencyPerUser:      optionalIntValue(req.MaxConcurrencyPerUser, 0),
		MaxConcurrencyPerPackage:   optionalIntValue(req.MaxConcurrencyPerPackage, 0),
		AllowBalanceFallback:       optionalBoolValue(req.AllowBalanceFallback, false),
		VisibilityScope:            optionalStringValue(req.VisibilityScope, model.ServicePackageVisibilityScopeAll),
		VisibleUserIDs:             req.VisibleUserIDs,
		SalePrice:                  optionalFloat64Value(req.SalePrice, 0),
		SaleCurrency:               optionalStringValue(req.SaleCurrency, model.BillingCurrencyCodeCNY),
		DailyQuotaLimit:            optionalInt64Value(req.DailyQuotaLimit, 0),
		PackageEmergencyQuotaLimit: optionalInt64Value(req.PackageEmergencyQuotaLimit, 0),
		DurationDays:               optionalIntValue(req.DurationDays, model.DefaultServicePackageDurationDays),
		QuotaResetTimezone:         optionalStringValue(req.QuotaResetTimezone, model.DefaultGroupQuotaResetTimezone),
		Enabled:                    optionalBoolValue(req.Enabled, true),
		SortOrder:                  optionalIntValue(req.SortOrder, 0),
		Source:                     optionalStringValue(req.Source, "manual"),
	}
	row, err := plansvc.Create(item)
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
		"data":    row,
	})
}

func UpdatePackage(c *gin.Context) {
	req := upsertServicePackageRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	id := strings.TrimSpace(req.Id)
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "套餐 ID 不能为空",
		})
		return
	}
	current, err := plansvc.Get(id)
	if err != nil {
		message := err.Error()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			message = "套餐不存在"
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": message,
		})
		return
	}
	item := model.ServicePackage{
		Id:                         id,
		Name:                       optionalStringValue(req.Name, current.Name),
		Description:                optionalStringValue(req.Description, current.Description),
		GroupID:                    optionalStringValue(req.GroupID, current.GroupID),
		PackageType:                optionalStringValue(req.PackageType, current.PackageType),
		ScopeType:                  model.ServicePackageScopeAll,
		QuotaMetric:                optionalStringValue(req.QuotaMetric, current.QuotaMetric),
		PeriodType:                 optionalStringValue(req.PeriodType, current.PeriodType),
		PeriodLimit:                optionalInt64Value(req.PeriodLimit, current.PeriodLimit),
		MaxConcurrencyPerUser:      optionalIntValue(req.MaxConcurrencyPerUser, current.MaxConcurrencyPerUser),
		MaxConcurrencyPerPackage:   optionalIntValue(req.MaxConcurrencyPerPackage, current.MaxConcurrencyPerPackage),
		AllowBalanceFallback:       optionalBoolValue(req.AllowBalanceFallback, current.AllowBalanceFallback),
		VisibilityScope:            optionalStringValue(req.VisibilityScope, current.VisibilityScope),
		VisibleUserIDs:             req.VisibleUserIDs,
		SalePrice:                  optionalFloat64Value(req.SalePrice, current.SalePrice),
		SaleCurrency:               optionalStringValue(req.SaleCurrency, current.SaleCurrency),
		DailyQuotaLimit:            optionalInt64Value(req.DailyQuotaLimit, current.DailyQuotaLimit),
		PackageEmergencyQuotaLimit: optionalInt64Value(req.PackageEmergencyQuotaLimit, current.PackageEmergencyQuotaLimit),
		DurationDays:               optionalIntValue(req.DurationDays, current.DurationDays),
		QuotaResetTimezone:         optionalStringValue(req.QuotaResetTimezone, current.QuotaResetTimezone),
		Enabled:                    optionalBoolValue(req.Enabled, current.Enabled),
		SortOrder:                  optionalIntValue(req.SortOrder, current.SortOrder),
		Source:                     optionalStringValue(req.Source, current.Source),
	}
	if req.VisibleUserIDs == nil {
		item.VisibleUserIDs = current.VisibleUserIDs
	}
	row, err := plansvc.Update(item)
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
		"data":    row,
	})
}

func DeletePackage(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "套餐 ID 不能为空",
		})
		return
	}
	if err := plansvc.Delete(id); err != nil {
		message := err.Error()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			message = "套餐不存在"
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

func AssignPackageToUser(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "套餐 ID 不能为空",
		})
		return
	}
	req := assignServicePackageRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	row, err := plansvc.AssignToUser(id, strings.TrimSpace(req.UserID), req.StartAt)
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
		"data":    row,
	})
}
