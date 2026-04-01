package controller

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/internal/admin/model"
)

type topupCallbackRequest struct {
	OrderID         string `json:"order_id"`
	TransactionID   string `json:"transaction_id"`
	ProviderOrderID string `json:"provider_order_id"`
	Status          string `json:"status"`
	ProviderName    string `json:"provider_name"`
	RedemptionID    string `json:"redemption_id"`
	RedemptionCode  string `json:"redemption_code"`
	StatusMessage   string `json:"status_message"`
	PaidAt          int64  `json:"paid_at"`
	RedeemedAt      int64  `json:"redeemed_at"`
}

func configuredTopupCallbackToken() string {
	if token := strings.TrimSpace(config.TopUpCallbackToken); token != "" {
		return token
	}
	return strings.TrimSpace(os.Getenv("TOPUP_CALLBACK_TOKEN"))
}

func extractTopupCallbackToken(c *gin.Context) string {
	if token := strings.TrimSpace(c.GetHeader("X-Topup-Callback-Token")); token != "" {
		return token
	}
	rawAuthorization := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(rawAuthorization), "bearer ") {
		return strings.TrimSpace(rawAuthorization[7:])
	}
	return ""
}

// ProcessTopupCallback godoc
// @Summary Process external top up callback
// @Tags public
// @Accept json
// @Produce json
// @Param body body docs.TopupOrderCallbackRequest true "Top up callback payload"
// @Success 200 {object} docs.UserTopUpOrderDetailResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/topup/callback [post]
func ProcessTopupCallback(c *gin.Context) {
	expectedToken := configuredTopupCallbackToken()
	if expectedToken == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "充值回调未启用",
		})
		return
	}
	receivedToken := extractTopupCallbackToken(c)
	if receivedToken == "" || subtle.ConstantTimeCompare([]byte(receivedToken), []byte(expectedToken)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "无效的充值回调凭证",
		})
		return
	}

	req := topupCallbackRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	order, err := model.ApplyTopupOrderCallbackWithDB(model.DB, model.TopupOrderCallbackInput{
		OrderID:         req.OrderID,
		TransactionID:   req.TransactionID,
		ProviderOrderID: req.ProviderOrderID,
		Status:          req.Status,
		ProviderName:    req.ProviderName,
		RedemptionID:    req.RedemptionID,
		RedemptionCode:  req.RedemptionCode,
		StatusMessage:   req.StatusMessage,
		PaidAt:          req.PaidAt,
		RedeemedAt:      req.RedeemedAt,
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
		"data":    order,
	})
}
