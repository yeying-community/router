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
	OutTradeNo      string `json:"out_trade_no"`
	TradeNo         string `json:"trade_no"`
	Status          string `json:"status"`
	TradeStatus     string `json:"trade_status"`
	PayStatus       string `json:"pay_status"`
	CallbackStatus  string `json:"callback_status"`
	ProviderName    string `json:"provider_name"`
	StatusMessage   string `json:"status_message"`
	Message         string `json:"message"`
	PaidAt          int64  `json:"paid_at"`
	PayTime         int64  `json:"pay_time"`
	RedeemedAt      int64  `json:"redeemed_at"`
}

func normalizeTopupCallbackStatus(values ...string) string {
	for _, value := range values {
		normalized := strings.TrimSpace(strings.ToLower(value))
		switch normalized {
		case model.TopupOrderStatusCreated, "0", "unpaid":
			return model.TopupOrderStatusCreated
		case model.TopupOrderStatusPending, "1", "processing", "paying":
			return model.TopupOrderStatusPending
		case model.TopupOrderStatusPaid, "2", "success", "succeeded", "paid_success":
			return model.TopupOrderStatusPaid
		case model.TopupOrderStatusFulfilled, "fulfilled_success":
			return model.TopupOrderStatusFulfilled
		case model.TopupOrderStatusFailed, "4", "fail", "failure":
			return model.TopupOrderStatusFailed
		case model.TopupOrderStatusCanceled, "3", "cancel", "closed", "cancelled":
			return model.TopupOrderStatusCanceled
		}
	}
	return ""
}

func normalizeTopupCallbackInput(req topupCallbackRequest) model.TopupOrderCallbackInput {
	transactionID := strings.TrimSpace(req.TransactionID)
	if transactionID == "" {
		transactionID = strings.TrimSpace(req.OutTradeNo)
	}
	providerOrderID := strings.TrimSpace(req.ProviderOrderID)
	if providerOrderID == "" {
		providerOrderID = strings.TrimSpace(req.TradeNo)
	}
	statusMessage := strings.TrimSpace(req.StatusMessage)
	if statusMessage == "" {
		statusMessage = strings.TrimSpace(req.Message)
	}
	paidAt := req.PaidAt
	if paidAt <= 0 {
		paidAt = req.PayTime
	}
	return model.TopupOrderCallbackInput{
		OrderID:         req.OrderID,
		TransactionID:   transactionID,
		ProviderOrderID: providerOrderID,
		Status: normalizeTopupCallbackStatus(
			req.Status,
			req.TradeStatus,
			req.PayStatus,
			req.CallbackStatus,
		),
		ProviderName:  req.ProviderName,
		StatusMessage: statusMessage,
		PaidAt:        paidAt,
		RedeemedAt:    req.RedeemedAt,
	}
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

func ProcessTopupCallback(c *gin.Context) {
	expectedToken := configuredTopupCallbackToken()
	if expectedToken == "" {
		logTopupCallbackFailure(c, true, "callback_not_enabled", http.StatusServiceUnavailable, nil, nil)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "充值回调未启用",
		})
		return
	}
	receivedToken := extractTopupCallbackToken(c)
	if receivedToken == "" || subtle.ConstantTimeCompare([]byte(receivedToken), []byte(expectedToken)) != 1 {
		logTopupCallbackFailure(c, true, "payment_signature_invalid", http.StatusUnauthorized, nil, nil)
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "无效的充值回调凭证",
		})
		return
	}

	req := topupCallbackRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		logTopupCallbackFailure(c, true, "callback_payload_invalid", http.StatusBadRequest, nil, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	callbackInput := normalizeTopupCallbackInput(req)
	order, err := model.ApplyTopupOrderCallbackWithDB(model.DB, callbackInput)
	if err != nil {
		logTopupCallbackFailure(c, true, "payment_callback_apply_failed", http.StatusOK, &req, err)
		response := gin.H{
			"success": false,
			"message": err.Error(),
		}
		if code := model.TopupErrorCode(err); code != "" {
			response["data"] = gin.H{"code": code}
		}
		c.JSON(http.StatusOK, response)
		return
	}
	if order.Status == model.TopupOrderStatusPaid {
		fulfilledOrder, fulfilledNow, err := model.FulfillTopupOrderWithDB(model.DB, order.Id)
		if err != nil {
			logTopupCallbackFailure(c, true, "payment_callback_fulfill_failed", http.StatusOK, &topupCallbackRequest{
				OrderID:         order.Id,
				TransactionID:   order.TransactionID,
				ProviderOrderID: order.ProviderOrderID,
				ProviderName:    order.ProviderName,
				Status:          order.Status,
			}, err)
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

func logTopupCallbackFailure(c *gin.Context, asError bool, reason string, httpStatus int, req *topupCallbackRequest, err error) {
	if c == nil || c.Request == nil {
		return
	}
	orderID := ""
	transactionID := ""
	providerOrderID := ""
	providerName := ""
	status := ""
	statusMessage := ""
	paidAt := int64(0)
	redeemedAt := int64(0)
	if req != nil {
		orderID = strings.TrimSpace(req.OrderID)
		transactionID = strings.TrimSpace(req.TransactionID)
		providerOrderID = strings.TrimSpace(req.ProviderOrderID)
		providerName = strings.TrimSpace(req.ProviderName)
		status = strings.TrimSpace(req.Status)
		statusMessage = strings.TrimSpace(req.StatusMessage)
		paidAt = req.PaidAt
		redeemedAt = req.RedeemedAt
	}
	errorMessage := ""
	if err != nil {
		errorMessage = strings.TrimSpace(err.Error())
	}
	msg := "[topup.callback] failed reason=%s http_status=%d endpoint=%s ip=%s user_agent=%q order_id=%q transaction_id=%q provider_order_id=%q provider_name=%q status=%q status_message=%q paid_at=%d redeemed_at=%d err=%q"
	if asError {
		logger.Errorf(c.Request.Context(), msg, strings.TrimSpace(reason), httpStatus, c.Request.URL.Path, c.ClientIP(), c.GetHeader("User-Agent"), orderID, transactionID, providerOrderID, providerName, status, statusMessage, paidAt, redeemedAt, errorMessage)
		return
	}
	logger.Warnf(c.Request.Context(), msg, strings.TrimSpace(reason), httpStatus, c.Request.URL.Path, c.ClientIP(), c.GetHeader("User-Agent"), orderID, transactionID, providerOrderID, providerName, status, statusMessage, paidAt, redeemedAt, errorMessage)
}
