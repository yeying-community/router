package topup

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/internal/admin/model"
)

type upsertTopupPlanRequest struct {
	Id             string  `json:"id"`
	Name           string  `json:"name"`
	GroupID        string  `json:"group_id"`
	Amount         float64 `json:"amount"`
	AmountCurrency string  `json:"amount_currency"`
	QuotaAmount    float64 `json:"quota_amount"`
	QuotaCurrency  string  `json:"quota_currency"`
	Enabled        bool    `json:"enabled"`
	SortOrder      int     `json:"sort_order"`
}

func GetAdminTopupPlans(c *gin.Context) {
	items, err := model.ListTopupPlans()
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
		"data":    items,
	})
}

func CreateAdminTopupPlan(c *gin.Context) {
	request := upsertTopupPlanRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	row, err := model.CreateTopupPlan(model.TopupPlan{
		Name:           request.Name,
		GroupID:        request.GroupID,
		Amount:         request.Amount,
		AmountCurrency: request.AmountCurrency,
		QuotaAmount:    request.QuotaAmount,
		QuotaCurrency:  request.QuotaCurrency,
		Enabled:        request.Enabled,
		SortOrder:      request.SortOrder,
	})
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

func UpdateAdminTopupPlan(c *gin.Context) {
	request := upsertTopupPlanRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	row, err := model.UpdateTopupPlan(model.TopupPlan{
		Id:             request.Id,
		Name:           request.Name,
		GroupID:        request.GroupID,
		Amount:         request.Amount,
		AmountCurrency: request.AmountCurrency,
		QuotaAmount:    request.QuotaAmount,
		QuotaCurrency:  request.QuotaCurrency,
		Enabled:        request.Enabled,
		SortOrder:      request.SortOrder,
	})
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

func DeleteAdminTopupPlan(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if err := model.DeleteTopupPlan(id); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func GetPublicTopupPlans(c *gin.Context) {
	items, err := model.ListPublicTopupPlans()
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
		"data":    items,
	})
}
