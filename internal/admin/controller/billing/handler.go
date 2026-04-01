package billing

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/internal/admin/model"
	billingsvc "github.com/yeying-community/router/internal/admin/service/billing"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

func usdYYCPerUnit() float64 {
	value, err := model.GetBillingCurrencyYYCPerUnit(model.BillingCurrencyCodeUSD)
	if err != nil || value <= 0 {
		return config.QuotaPerUnit
	}
	return value
}

type upsertBillingCurrencyRequest struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Symbol     string  `json:"symbol"`
	MinorUnit  int     `json:"minor_unit"`
	YYCPerUnit float64 `json:"yyc_per_unit"`
	Status     int     `json:"status"`
	Source     string  `json:"source"`
}

// GetBillingCurrencies godoc
// @Summary List billing currencies (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/billing/currencies [get]
func GetBillingCurrencies(c *gin.Context) {
	rows, err := model.ListBillingCurrencies()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "加载计费币种失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
}

// CreateBillingCurrency godoc
// @Summary Create billing currency (root)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/billing/currencies [post]
func CreateBillingCurrency(c *gin.Context) {
	req := upsertBillingCurrencyRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	row, err := model.CreateBillingCurrencyWithDB(model.DB, model.BillingCurrency{
		Code:       req.Code,
		Name:       req.Name,
		Symbol:     req.Symbol,
		MinorUnit:  req.MinorUnit,
		YYCPerUnit: req.YYCPerUnit,
		Status:     req.Status,
		Source:     req.Source,
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

// UpdateBillingCurrency godoc
// @Summary Update billing currency (root)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param code path string true "Currency code"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/billing/currencies/{code} [put]
func UpdateBillingCurrency(c *gin.Context) {
	code := strings.TrimSpace(c.Param("code"))
	if code == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "币种代码不能为空",
		})
		return
	}
	req := upsertBillingCurrencyRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	row, err := model.UpdateBillingCurrencyWithDB(model.DB, code, func(current model.BillingCurrency) (model.BillingCurrency, error) {
		next := current
		next.Name = req.Name
		next.Symbol = req.Symbol
		next.MinorUnit = req.MinorUnit
		next.YYCPerUnit = req.YYCPerUnit
		next.Status = req.Status
		if strings.TrimSpace(req.Source) != "" {
			next.Source = req.Source
		} else if strings.TrimSpace(strings.ToLower(current.Source)) == model.BillingCurrencySourceSystemDefault {
			next.Source = "manual"
		}
		return next, nil
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

func GetSubscription(c *gin.Context) {
	var remainQuota int64
	var usedQuota int64
	var err error
	var token *model.Token
	var expiredTime int64
	if config.DisplayTokenStatEnabled {
		tokenId := c.GetString(ctxkey.TokenId)
		if tokenId != "" {
			token, err = billingsvc.GetTokenByID(tokenId)
			if err == nil {
				expiredTime = token.ExpiredTime
				remainQuota = token.RemainQuota
				usedQuota = token.UsedQuota
			}
		}
	}
	if token == nil {
		userId := c.GetString(ctxkey.Id)
		remainQuota, err = billingsvc.GetUserQuota(userId)
		if err != nil {
			usedQuota, err = billingsvc.GetUserUsedQuota(userId)
		}
	}
	if expiredTime <= 0 {
		expiredTime = 0
	}
	if err != nil {
		Error := relaymodel.Error{
			Message: err.Error(),
			Type:    "upstream_error",
		}
		c.JSON(200, gin.H{
			"error": Error,
		})
		return
	}
	quota := remainQuota + usedQuota
	amount := float64(quota)
	if config.DisplayInCurrencyEnabled {
		amount /= usdYYCPerUnit()
	}
	if token != nil && token.UnlimitedQuota {
		amount = 100000000
	}
	subscription := billingsvc.OpenAISubscriptionResponse{
		Object:             "billing_subscription",
		HasPaymentMethod:   true,
		SoftLimitUSD:       amount,
		HardLimitUSD:       amount,
		SystemHardLimitUSD: amount,
		AccessUntil:        expiredTime,
	}
	c.JSON(200, subscription)
	return
}

func GetUsage(c *gin.Context) {
	var quota int64
	var err error
	var token *model.Token
	if config.DisplayTokenStatEnabled {
		tokenId := c.GetString(ctxkey.TokenId)
		if tokenId != "" {
			token, err = billingsvc.GetTokenByID(tokenId)
			if err == nil {
				quota = token.UsedQuota
			}
		}
	}
	if token == nil {
		userId := c.GetString(ctxkey.Id)
		quota, err = billingsvc.GetUserUsedQuota(userId)
	}
	if err != nil {
		Error := relaymodel.Error{
			Message: err.Error(),
			Type:    "one_api_error",
		}
		c.JSON(200, gin.H{
			"error": Error,
		})
		return
	}
	amount := float64(quota)
	if config.DisplayInCurrencyEnabled {
		amount /= usdYYCPerUnit()
	}
	usage := billingsvc.OpenAIUsageResponse{
		Object:     "list",
		TotalUsage: amount * 100,
	}
	c.JSON(200, usage)
	return
}
