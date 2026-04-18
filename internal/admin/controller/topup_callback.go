package controller

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	usersvc "github.com/yeying-community/router/internal/admin/service/user"
)

type topupCallbackRequest struct {
	OrderID         string `json:"order_id"`
	TransactionID   string `json:"transaction_id"`
	ProviderOrderID string `json:"provider_order_id"`
	Status          string `json:"status"`
	ProviderName    string `json:"provider_name"`
	StatusMessage   string `json:"status_message"`
	PaidAt          int64  `json:"paid_at"`
	RedeemedAt      int64  `json:"redeemed_at"`
}

func configuredTopupCallbackToken() string {
	return config.ConfiguredTopUpCallbackToken()
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
		emitTopupCallbackErrorCard(
			c,
			"callback_not_enabled",
			"TOPUP_CALLBACK_NOT_ENABLED",
			"充值回调未启用",
			"支付回调入口被关闭，请求被拒绝",
			"system",
			"已支付订单可能无法自动发货，需要人工补偿处理",
			nil,
			http.StatusServiceUnavailable,
			nil,
		)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "充值回调未启用",
		})
		return
	}
	receivedToken := extractTopupCallbackToken(c)
	if receivedToken == "" || subtle.ConstantTimeCompare([]byte(receivedToken), []byte(expectedToken)) != 1 {
		emitTopupCallbackErrorCard(
			c,
			"payment_signature_invalid",
			"TOPUP_CALLBACK_TOKEN_INVALID",
			"充值回调凭证无效",
			"支付回调鉴权失败",
			"system",
			"本次支付回调未被接收，订单状态可能无法及时更新",
			nil,
			http.StatusUnauthorized,
			nil,
		)
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "无效的充值回调凭证",
		})
		return
	}

	req := topupCallbackRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		emitTopupCallbackErrorCard(
			c,
			"callback_payload_invalid",
			"TOPUP_CALLBACK_PAYLOAD_INVALID",
			"充值回调请求体无效",
			err.Error(),
			"system",
			"支付回调数据无法解析，订单状态未更新",
			nil,
			http.StatusBadRequest,
			err,
		)
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
		StatusMessage:   req.StatusMessage,
		PaidAt:          req.PaidAt,
		RedeemedAt:      req.RedeemedAt,
	})
	if err != nil {
		emitTopupCallbackErrorCard(
			c,
			"payment_callback_failed",
			"TOPUP_CALLBACK_APPLY_FAILED",
			"充值回调处理失败",
			err.Error(),
			"single_user",
			"支付状态未成功回写到订单，用户可能看到订单仍未支付",
			&req,
			http.StatusOK,
			err,
		)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if order.Status == model.TopupOrderStatusPaid {
		fulfilledOrder, fulfilledNow, err := model.FulfillTopupOrderWithDB(model.DB, order.Id)
		if err != nil {
			emitTopupCallbackErrorCard(
				c,
				"payment_callback_failed",
				"TOPUP_CALLBACK_FULFILL_FAILED",
				"充值回调发货失败",
				err.Error(),
				"single_user",
				"订单已支付但额度/套餐未发放，用户权益受影响",
				&topupCallbackRequest{
					OrderID:         order.Id,
					TransactionID:   order.TransactionID,
					ProviderOrderID: order.ProviderOrderID,
					ProviderName:    order.ProviderName,
					Status:          order.Status,
				},
				http.StatusOK,
				err,
			)
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		order = fulfilledOrder
		if fulfilledNow && order.BusinessType == model.TopupOrderBusinessBalance && order.Quota > 0 {
			quotaText := strconv.FormatInt(order.Quota, 10)
			usersvc.RecordTopupLog(
				c.Request.Context(),
				order.UserID,
				"外部支付充值 "+quotaText+" 额度",
				int(order.Quota),
			)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    order,
	})
}

func emitTopupCallbackErrorCard(c *gin.Context, subtype string, errorCode string, title string, summary string, impactScope string, impactSummary string, req *topupCallbackRequest, httpStatus int, err error) {
	tags := map[string]string{}
	if req != nil {
		if strings.TrimSpace(req.OrderID) != "" {
			tags["order_id"] = strings.TrimSpace(req.OrderID)
		}
		if strings.TrimSpace(req.TransactionID) != "" {
			tags["transaction_id"] = strings.TrimSpace(req.TransactionID)
		}
		if strings.TrimSpace(req.ProviderOrderID) != "" {
			tags["provider_order_id"] = strings.TrimSpace(req.ProviderOrderID)
		}
		if strings.TrimSpace(req.ProviderName) != "" {
			tags["provider_name"] = strings.TrimSpace(req.ProviderName)
		}
		if strings.TrimSpace(req.Status) != "" {
			tags["status"] = strings.TrimSpace(req.Status)
		}
	}
	errorMessage := strings.TrimSpace(summary)
	if err != nil {
		errorMessage = strings.TrimSpace(err.Error())
	}
	logger.EmitFeishuCardError(c.Request.Context(), logger.ErrorCardEvent{
		EventType:     "payment_topup_callback_error",
		Domain:        "payment",
		Subtype:       strings.TrimSpace(subtype),
		Severity:      "error",
		Title:         strings.TrimSpace(title),
		Summary:       strings.TrimSpace(summary),
		BizStatus:     "failed",
		ErrorCode:     strings.TrimSpace(errorCode),
		ErrorMessage:  errorMessage,
		ImpactScope:   strings.TrimSpace(impactScope),
		ImpactSummary: strings.TrimSpace(impactSummary),
		Endpoint:      c.Request.URL.Path,
		HTTPStatus:    httpStatus,
		Tags:          tags,
	})
}
