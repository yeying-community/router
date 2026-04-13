package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/presenter"
)

// GetAllRedemptions godoc
// @Summary List redemptions (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page (1-based)"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/redemption [get]
func GetAllRedemptions(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	redemptions, err := model.GetAllRedemptions((page-1)*config.ItemsPerPage, config.ItemsPerPage)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	var total int64
	if err := model.DB.Model(&model.Redemption{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    presenter.NewRedemptions(redemptions),
		"meta": gin.H{
			"total":     total,
			"page":      page,
			"page_size": config.ItemsPerPage,
		},
	})
	return
}

// SearchRedemptions godoc
// @Summary Search redemptions (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param keyword query string false "Keyword"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/redemption/search [get]
func SearchRedemptions(c *gin.Context) {
	keyword := c.Query("keyword")
	redemptions, err := model.SearchRedemptions(keyword)
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
		"data":    presenter.NewRedemptions(redemptions),
	})
	return
}

// GetRedemption godoc
// @Summary Get redemption by ID (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Redemption ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/redemption/{id} [get]
func GetRedemption(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "id 为空",
		})
		return
	}
	var err error
	redemption, err := model.GetRedemptionById(id)
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
		"data":    presenter.NewRedemption(redemption),
	})
	return
}

// AddRedemption godoc
// @Summary Create redemption codes (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.RedemptionCreateRequest true "Redemption payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/redemption [post]
func AddRedemption(c *gin.Context) {
	redemption := model.Redemption{}
	err := c.ShouldBindJSON(&redemption)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if len(redemption.Name) == 0 || len(redemption.Name) > 20 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "兑换码名称长度必须在1-20之间",
		})
		return
	}
	if redemption.Count <= 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "兑换码个数必须大于0",
		})
		return
	}
	if redemption.Count > 100 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "一次兑换码批量生成的个数不能大于 100",
		})
		return
	}
	resolvedGroup, err := model.ResolveRedemptionGroupWithDB(model.DB, redemption.GroupID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	redemption.GroupID = resolvedGroup.Id
	redemption.GroupName = resolvedGroup.Name
	if err := model.NormalizeRedemptionFaceValueFieldsWithDB(model.DB, &redemption); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	codeValidityDays := redemption.CodeValidityDays
	if codeValidityDays < 0 || codeValidityDays > model.UserBalanceLotMaxValidityDay {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "兑换码有效期天数超出范围",
		})
		return
	}
	creditValidityDays := redemption.CreditValidityDays
	if creditValidityDays < 0 || creditValidityDays > model.UserBalanceLotMaxValidityDay {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "兑换到账有效期天数超出范围",
		})
		return
	}
	var codes []string
	for i := 0; i < redemption.Count; i++ {
		code := random.GetUUID()
		createdAt := helper.GetTimestamp()
		cleanRedemption := model.Redemption{
			UserId:             c.GetString(ctxkey.Id),
			Name:               redemption.Name,
			GroupID:            redemption.GroupID,
			Code:               code,
			CreatedTime:        createdAt,
			FaceValueAmount:    redemption.FaceValueAmount,
			FaceValueUnit:      redemption.FaceValueUnit,
			Quota:              redemption.Quota,
			CodeValidityDays:   codeValidityDays,
			CodeExpiresAt:      model.ResolveBalanceCreditExpiresAt(createdAt, codeValidityDays),
			CreditValidityDays: creditValidityDays,
		}
		err = cleanRedemption.Insert()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
				"data":    codes,
			})
			return
		}
		codes = append(codes, code)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    codes,
	})
	return
}

// DeleteRedemption godoc
// @Summary Delete redemption (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Redemption ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/redemption/{id} [delete]
func DeleteRedemption(c *gin.Context) {
	id := c.Param("id")
	err := model.DeleteRedemptionById(id)
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
	})
	return
}

// UpdateRedemption godoc
// @Summary Update redemption (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.RedemptionUpdateRequest true "Redemption update payload"
// @Param status_only query string false "Update status only if set"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/redemption [put]
func UpdateRedemption(c *gin.Context) {
	statusOnly := c.Query("status_only")
	redemption := model.Redemption{}
	err := c.ShouldBindJSON(&redemption)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	cleanRedemption, err := model.GetRedemptionById(redemption.Id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if statusOnly != "" {
		cleanRedemption.Status = redemption.Status
	} else {
		resolvedGroup, err := model.ResolveRedemptionGroupWithDB(model.DB, redemption.GroupID)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		cleanRedemption.Name = redemption.Name
		cleanRedemption.GroupID = resolvedGroup.Id
		cleanRedemption.GroupName = resolvedGroup.Name
		cleanRedemption.FaceValueAmount = redemption.FaceValueAmount
		cleanRedemption.FaceValueUnit = redemption.FaceValueUnit
		if redemption.CodeValidityDays < 0 || redemption.CodeValidityDays > model.UserBalanceLotMaxValidityDay {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "兑换码有效期天数超出范围",
			})
			return
		}
		if redemption.CreditValidityDays < 0 || redemption.CreditValidityDays > model.UserBalanceLotMaxValidityDay {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "兑换到账有效期天数超出范围",
			})
			return
		}
		cleanRedemption.CodeValidityDays = redemption.CodeValidityDays
		cleanRedemption.CodeExpiresAt = model.ResolveBalanceCreditExpiresAt(cleanRedemption.CreatedTime, cleanRedemption.CodeValidityDays)
		cleanRedemption.CreditValidityDays = redemption.CreditValidityDays
		if err := model.NormalizeRedemptionFaceValueFieldsWithDB(model.DB, cleanRedemption); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	err = cleanRedemption.Update()
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
		"data":    presenter.NewRedemption(cleanRedemption),
	})
	return
}
