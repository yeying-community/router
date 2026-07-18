package entitlement

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/gorm"
)

const maxEntitlementProductListPageSize = 100

type entitlementProductListPageData struct {
	Items    []model.EntitlementProduct `json:"items"`
	Total    int64                      `json:"total"`
	Page     int                        `json:"page"`
	PageSize int                        `json:"page_size"`
}

func parseEntitlementProductListPageParams(c *gin.Context) (kind string, page int, pageSize int, keyword string) {
	kind = strings.TrimSpace(c.Query("kind"))
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
	if pageSize > maxEntitlementProductListPageSize {
		pageSize = maxEntitlementProductListPageSize
	}
	keyword = strings.TrimSpace(c.Query("keyword"))
	return kind, page, pageSize, keyword
}

func GetProducts(c *gin.Context) {
	kind, page, pageSize, keyword := parseEntitlementProductListPageParams(c)
	rows, total, err := model.ListEntitlementProductsPage(kind, page, pageSize, keyword)
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
		"data": entitlementProductListPageData{
			Items:    rows,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

func GetProduct(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "权益商品 ID 不能为空",
		})
		return
	}
	row, err := model.GetEntitlementProductByID(id)
	if err != nil {
		message := err.Error()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			message = "权益商品不存在"
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

func CreateProduct(c *gin.Context) {
	req := model.EntitlementProduct{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	row, err := model.CreateEntitlementProduct(req)
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

func UpdateProduct(c *gin.Context) {
	req := model.EntitlementProduct{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if strings.TrimSpace(req.Id) == "" {
		req.Id = strings.TrimSpace(c.Param("id"))
	}
	row, err := model.UpdateEntitlementProduct(req)
	if err != nil {
		message := err.Error()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			message = "权益商品不存在"
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

func DeleteProduct(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "权益商品 ID 不能为空",
		})
		return
	}
	if err := model.DeleteEntitlementProduct(id); err != nil {
		message := err.Error()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			message = "权益商品不存在"
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
