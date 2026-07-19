package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/presenter"
	"gorm.io/gorm"
)

var generateRedemptionCode = random.GetUUID

func applyRedemptionEntitlementProduct(productID string, redemption *model.Redemption) error {
	if redemption == nil {
		return gorm.ErrInvalidData
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return fmt.Errorf("必须绑定充值型权益")
	}
	product, err := model.GetEntitlementProductByID(productID)
	if err != nil {
		return fmt.Errorf("权益不存在: %w", err)
	}
	if product.Kind != model.EntitlementProductKindBalance {
		return fmt.Errorf("兑换码目前只能绑定充值型权益")
	}
	if !product.Enabled {
		return fmt.Errorf("权益已禁用")
	}
	redemption.EntitlementProductID = product.Id
	redemption.ProductKind = product.Kind
	redemption.ProductNameSnapshot = product.Name
	redemption.QuotaAmountSnapshot = product.QuotaAmount
	redemption.QuotaCurrencySnapshot = product.QuotaCurrency
	redemption.ValidityDaysSnapshot = product.ValidityDays
	redemption.GroupIDSnapshot = product.GroupID
	redemption.GroupID = product.GroupID
	return nil
}

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
	if redemption.Count > 99 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "一次兑换码批量生成的个数不能大于 99",
		})
		return
	}
	if err := applyRedemptionEntitlementProduct(redemption.EntitlementProductID, &redemption); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
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
	codeValidityDays := redemption.CodeValidityDays
	if codeValidityDays < 0 || codeValidityDays > model.UserBalanceLotMaxValidityDay {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "兑换码有效期天数超出范围",
		})
		return
	}
	codes := make([]string, 0, redemption.Count)
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		generatedCodes, err := createRedemptionsWithDB(tx, redemption, c.GetString(ctxkey.Id), generateRedemptionCode)
		if err != nil {
			return err
		}
		codes = generatedCodes
		return nil
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"data":    codes,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    codes,
	})
	return
}

func createRedemptionsWithDB(tx *gorm.DB, template model.Redemption, creatorID string, codeGenerator func() string) ([]string, error) {
	if tx == nil {
		return nil, gorm.ErrInvalidDB
	}
	if codeGenerator == nil {
		codeGenerator = random.GetUUID
	}
	batchID := random.GetUUID()
	codes := make([]string, 0, template.Count)
	for i := 0; i < template.Count; i++ {
		code := codeGenerator()
		createdAt := helper.GetTimestamp()
		cleanRedemption := model.Redemption{
			Id:                    random.GetUUID(),
			UserId:                creatorID,
			Name:                  template.Name,
			GroupID:               template.GroupID,
			EntitlementProductID:  template.EntitlementProductID,
			ProductKind:           template.ProductKind,
			ProductNameSnapshot:   template.ProductNameSnapshot,
			QuotaAmountSnapshot:   template.QuotaAmountSnapshot,
			QuotaCurrencySnapshot: template.QuotaCurrencySnapshot,
			ValidityDaysSnapshot:  template.ValidityDaysSnapshot,
			GroupIDSnapshot:       template.GroupIDSnapshot,
			Code:                  code,
			CreatedTime:           createdAt,
			CodeValidityDays:      template.CodeValidityDays,
			CodeExpiresAt:         model.ResolveBalanceCreditExpiresAt(createdAt, template.CodeValidityDays),
		}
		if template.Count > 1 {
			cleanRedemption.Name = fmt.Sprintf("%s-%02d", template.Name, i+1)
		}
		if err := tx.Create(&cleanRedemption).Error; err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	firstCode := ""
	lastCode := ""
	if len(codes) > 0 {
		firstCode = codes[0]
		lastCode = codes[len(codes)-1]
	}
	if err := model.RecordRedemptionIssueAuditLogWithDB(tx, model.RedemptionIssueAuditLog{
		BatchID:               batchID,
		CreatedByUserID:       creatorID,
		Name:                  template.Name,
		EntitlementProductID:  template.EntitlementProductID,
		ProductNameSnapshot:   template.ProductNameSnapshot,
		QuotaAmountSnapshot:   template.QuotaAmountSnapshot,
		QuotaCurrencySnapshot: template.QuotaCurrencySnapshot,
		ValidityDaysSnapshot:  template.ValidityDaysSnapshot,
		GroupID:               template.GroupID,
		Count:                 len(codes),
		CodeValidityDays:      template.CodeValidityDays,
		FirstCode:             firstCode,
		LastCode:              lastCode,
	}); err != nil {
		return nil, err
	}
	return codes, nil
}

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
		cleanRedemption.Name = redemption.Name
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
