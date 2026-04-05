package model

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/common/random"
)

const (
	topupOrderClientTypePC     = "pc"
	topupOrderClientTypeMobile = "mobile"
)

type externalPayCreateResponse struct {
	Code int                           `json:"code"`
	Msg  string                        `json:"msg"`
	Data externalPayCreateResponseData `json:"data"`
}

type externalPayCreateResponseData struct {
	TradeNo         string                        `json:"trade_no"`
	CashierURL      string                        `json:"cashier_url"`
	StatusURL       string                        `json:"status_url"`
	ProviderName    string                        `json:"provider_name"`
	ProviderPayload externalPayCreateProviderData `json:"provider_payload"`
}

type externalPayCreateProviderData struct {
	TradeType string `json:"trade_type"`
	CodeURL   string `json:"code_url"`
	MwebURL   string `json:"mweb_url"`
	PrepayID  string `json:"prepay_id"`
}

type topupExternalPayCreateResult struct {
	RedirectURL     string
	ProviderOrderID string
	ProviderName    string
}

func normalizeTopupOrderClientType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case topupOrderClientTypeMobile:
		return topupOrderClientTypeMobile
	default:
		return topupOrderClientTypePC
	}
}

func buildExternalPayCreateURL() (string, error) {
	baseURL := strings.TrimSpace(config.ResolvedTopUpAPICreateURL())
	if baseURL == "" {
		return "", fmt.Errorf("超级管理员未设置支付 API 地址")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("支付 API 地址配置无效")
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("支付 API 地址配置无效")
	}
	query := parsed.Query()
	query.Set("uniacid", strconv.Itoa(config.TopUpAPIUniacidValue()))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func buildExternalPayCreatePayload(order TopupOrder, clientType string) map[string]string {
	payload := map[string]string{
		"merchant_app":   config.TopUpMerchantAppValue(),
		"order_id":       strings.TrimSpace(order.Id),
		"transaction_id": strings.TrimSpace(order.TransactionID),
		"user_id":        strings.TrimSpace(order.UserID),
		"username":       strings.TrimSpace(order.Username),
		"business_type":  strings.TrimSpace(order.BusinessType),
		"title":          strings.TrimSpace(order.Title),
		"amount":         fmt.Sprintf("%.2f", order.Amount),
		"currency":       strings.TrimSpace(order.Currency),
		"client_type":    normalizeTopupOrderClientType(clientType),
		"callback_url":   strings.TrimSpace(order.CallbackURL),
		"return_url":     strings.TrimSpace(order.ReturnURL),
		"timestamp":      strconv.FormatInt(helper.GetTimestamp(), 10),
		"nonce":          random.GetUUID(),
	}
	if order.Quota > 0 {
		payload["quota"] = strconv.FormatInt(order.Quota, 10)
	}
	if strings.TrimSpace(order.PackageID) != "" {
		payload["package_id"] = strings.TrimSpace(order.PackageID)
	}
	if strings.TrimSpace(order.PackageName) != "" {
		payload["package_name"] = strings.TrimSpace(order.PackageName)
	}
	payload["sign"] = signTopupOrderPayload(payload, config.TopUpSignSecret)
	return payload
}

func previewExternalPayResponse(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) <= 320 {
		return trimmed
	}
	return trimmed[:320] + "..."
}

func topupSignSecretFingerprint(secret string) string {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trimmed))
	fingerprint := hex.EncodeToString(sum[:])
	if len(fingerprint) > 12 {
		return fingerprint[:12]
	}
	return fingerprint
}

func marshalExternalPaySignFields(payload map[string]string) string {
	fields := make(map[string]string, len(payload))
	for key, value := range payload {
		if strings.EqualFold(strings.TrimSpace(key), "sign") {
			continue
		}
		fields[key] = value
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func externalPayUniacidFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Query().Get("uniacid"))
}

func logExternalPayCreateFailure(
	order TopupOrder,
	requestURL string,
	payload map[string]string,
	httpStatus int,
	upstreamCode int,
	upstreamMessage string,
	responseBody []byte,
	requestErr error,
) {
	requestError := ""
	if requestErr != nil {
		requestError = requestErr.Error()
	}
	logger.SysWarnf(
		"[topup.external_pay] create_failed order_id=%q transaction_id=%q business_type=%q merchant_app=%q uniacid=%q url=%q sign_fields=%s sign_base=%q sign=%q secret_fp=%q http_status=%d upstream_code=%d upstream_msg=%q response=%q err=%q",
		strings.TrimSpace(order.Id),
		strings.TrimSpace(order.TransactionID),
		strings.TrimSpace(order.BusinessType),
		strings.TrimSpace(payload["merchant_app"]),
		externalPayUniacidFromURL(requestURL),
		strings.TrimSpace(requestURL),
		marshalExternalPaySignFields(payload),
		topupOrderSigningBaseString(payload),
		strings.TrimSpace(payload["sign"]),
		topupSignSecretFingerprint(config.TopUpSignSecret),
		httpStatus,
		upstreamCode,
		strings.TrimSpace(upstreamMessage),
		previewExternalPayResponse(responseBody),
		requestError,
	)
}

func createTopupOrderByExternalPayAPI(order TopupOrder, clientType string) (topupExternalPayCreateResult, error) {
	requestURL, err := buildExternalPayCreateURL()
	if err != nil {
		return topupExternalPayCreateResult{}, err
	}
	payload := buildExternalPayCreatePayload(order, clientType)
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return topupExternalPayCreateResult{}, fmt.Errorf("构造支付请求失败")
	}
	request, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return topupExternalPayCreateResult{}, fmt.Errorf("构造支付请求失败")
	}
	request.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{
		Timeout: time.Duration(config.TopUpAPITimeoutSecondsValue()) * time.Second,
	}
	response, err := httpClient.Do(request)
	if err != nil {
		logExternalPayCreateFailure(order, requestURL, payload, 0, 0, "", nil, err)
		return topupExternalPayCreateResult{}, fmt.Errorf("调用支付 API 失败: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return topupExternalPayCreateResult{}, fmt.Errorf("读取支付 API 响应失败")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		logExternalPayCreateFailure(order, requestURL, payload, response.StatusCode, 0, "", responseBody, nil)
		return topupExternalPayCreateResult{}, fmt.Errorf(
			"支付 API 请求失败: status=%d body=%s",
			response.StatusCode,
			previewExternalPayResponse(responseBody),
		)
	}

	var payloadResponse externalPayCreateResponse
	if err := json.Unmarshal(responseBody, &payloadResponse); err != nil {
		logExternalPayCreateFailure(order, requestURL, payload, response.StatusCode, 0, "invalid_json", responseBody, err)
		return topupExternalPayCreateResult{}, fmt.Errorf("解析支付 API 响应失败: %s", previewExternalPayResponse(responseBody))
	}
	if payloadResponse.Code != 1 {
		message := strings.TrimSpace(payloadResponse.Msg)
		if message == "" {
			message = "支付 API 返回失败"
		}
		logExternalPayCreateFailure(order, requestURL, payload, response.StatusCode, payloadResponse.Code, message, responseBody, nil)
		return topupExternalPayCreateResult{}, fmt.Errorf(message)
	}

	redirectURL := strings.TrimSpace(payloadResponse.Data.CashierURL)
	if redirectURL == "" {
		return topupExternalPayCreateResult{}, fmt.Errorf("支付 API 未返回 cashier_url")
	}
	providerOrderID := strings.TrimSpace(payloadResponse.Data.TradeNo)
	if providerOrderID == "" {
		return topupExternalPayCreateResult{}, fmt.Errorf("支付 API 未返回 trade_no")
	}

	providerName := strings.TrimSpace(payloadResponse.Data.ProviderName)
	if providerName == "" {
		providerName = "yeying-room"
	}

	return topupExternalPayCreateResult{
		RedirectURL:     redirectURL,
		ProviderOrderID: providerOrderID,
		ProviderName:    providerName,
	}, nil
}
