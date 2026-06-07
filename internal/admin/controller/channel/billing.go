package channel

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yeying-community/router/common/client"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type OpenAISubscriptionResponse struct {
	Object             string  `json:"object"`
	HasPaymentMethod   bool    `json:"has_payment_method"`
	SoftLimitUSD       float64 `json:"soft_limit_usd"`
	HardLimitUSD       float64 `json:"hard_limit_usd"`
	SystemHardLimitUSD float64 `json:"system_hard_limit_usd"`
	AccessUntil        int64   `json:"access_until"`
}

type OpenAIUsageDailyCost struct {
	Timestamp float64 `json:"timestamp"`
	LineItems []struct {
		Name string  `json:"name"`
		Cost float64 `json:"cost"`
	}
}

type OpenAICreditGrants struct {
	Object         string  `json:"object"`
	TotalGranted   float64 `json:"total_granted"`
	TotalUsed      float64 `json:"total_used"`
	TotalAvailable float64 `json:"total_available"`
}

type OpenAIUsageResponse struct {
	Object string `json:"object"`
	//DailyCosts []OpenAIUsageDailyCost `json:"daily_costs"`
	TotalUsage float64 `json:"total_usage"` // unit: 0.01 dollar
}

type OpenAISBUsageResponse struct {
	Msg  string `json:"msg"`
	Data *struct {
		Credit string `json:"credit"`
	} `json:"data"`
}

type AIProxyUserOverviewResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	ErrorCode int    `json:"error_code"`
	Data      struct {
		TotalPoints float64 `json:"totalPoints"`
	} `json:"data"`
}

type API2GPTUsageResponse struct {
	Object         string  `json:"object"`
	TotalGranted   float64 `json:"total_granted"`
	TotalUsed      float64 `json:"total_used"`
	TotalRemaining float64 `json:"total_remaining"`
}

type APGC2DGPTUsageResponse struct {
	//Grants         interface{} `json:"grants"`
	Object         string  `json:"object"`
	TotalAvailable float64 `json:"total_available"`
	TotalGranted   float64 `json:"total_granted"`
	TotalUsed      float64 `json:"total_used"`
}

type SiliconFlowUsageResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  bool   `json:"status"`
	Data    struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Image         string `json:"image"`
		Email         string `json:"email"`
		IsAdmin       bool   `json:"isAdmin"`
		Balance       string `json:"balance"`
		Status        string `json:"status"`
		Introduction  string `json:"introduction"`
		Role          string `json:"role"`
		ChargeBalance string `json:"chargeBalance"`
		TotalBalance  string `json:"totalBalance"`
		Category      string `json:"category"`
	} `json:"data"`
}

type DeepSeekUsageResponse struct {
	IsAvailable  bool `json:"is_available"`
	BalanceInfos []struct {
		Currency        string `json:"currency"`
		TotalBalance    string `json:"total_balance"`
		GrantedBalance  string `json:"granted_balance"`
		ToppedUpBalance string `json:"topped_up_balance"`
	} `json:"balance_infos"`
}

type OpenRouterResponse struct {
	Data struct {
		TotalCredits float64 `json:"total_credits"`
		TotalUsage   float64 `json:"total_usage"`
	} `json:"data"`
}

type CDKUsageStatsResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Consumed        float64 `json:"consumed"`
		Remaining       float64 `json:"remaining"`
		Requests        int     `json:"requests"`
		ActiveSessions  int     `json:"activeSessions"`
		QuotaScope      string  `json:"quotaScope"`
		ResetAt         string  `json:"resetAt"`
		WeeklyConsumed  float64 `json:"weeklyConsumed"`
		WeeklyRemaining float64 `json:"weeklyRemaining"`
		WeeklyLimit     float64 `json:"weeklyLimit"`
		WeeklyResetAt   string  `json:"weeklyResetAt"`
		WeeklyResetMode string  `json:"weeklyResetMode"`
		TotalConsumed   float64 `json:"totalConsumed"`
		TotalRemaining  float64 `json:"totalRemaining"`
		TotalLimit      float64 `json:"totalLimit"`
	} `json:"data"`
}

type CDKCardInfoResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		OK                             bool    `json:"ok"`
		Status                         string  `json:"status"`
		CanRenew                       bool    `json:"canRenew"`
		MaskedCDK                      string  `json:"maskedCdk"`
		ProductName                    string  `json:"productName"`
		CategoryName                   string  `json:"categoryName"`
		CategoryPool                   string  `json:"categoryPool"`
		BillingMode                    string  `json:"billingMode"`
		ExpiresAt                      string  `json:"expiresAt"`
		ExpireTime                     string  `json:"expireTime"`
		ExpireDays                     int     `json:"expireDays"`
		DailyQuota                     float64 `json:"dailyQuota"`
		AllowRefundRequest             bool    `json:"allowRefundRequest"`
		AllowDailyLimitReset           bool    `json:"allowDailyLimitReset"`
		CanResetDailyLimit             bool    `json:"canResetDailyLimit"`
		DailyLimitResetMaxTimes        int     `json:"dailyLimitResetMaxTimes"`
		DailyLimitResetRemainingTimes  int     `json:"dailyLimitResetRemainingTimes"`
		DailyLimitResetCostDays        int     `json:"dailyLimitResetCostDays"`
		AllowWeeklyLimitReset          bool    `json:"allowWeeklyLimitReset"`
		CanResetWeeklyLimit            bool    `json:"canResetWeeklyLimit"`
		WeeklyLimitResetMaxTimes       int     `json:"weeklyLimitResetMaxTimes"`
		WeeklyLimitResetRemainingTimes int     `json:"weeklyLimitResetRemainingTimes"`
		WeeklyLimitResetCostDays       int     `json:"weeklyLimitResetCostDays"`
		LimitConcurrentSessions        int     `json:"limitConcurrentSessions"`
		Key                            string  `json:"key"`
		Nickname                       string  `json:"nickname"`
		UpstreamUserName               string  `json:"upstreamUserName"`
		CanConvertToUsage              bool    `json:"canConvertToUsage"`
		CanPlanConvert                 bool    `json:"canPlanConvert"`
	} `json:"data"`
}

// buildBearerAuthHeader get auth header
func buildBearerAuthHeader(token string) http.Header {
	h := http.Header{}
	h.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	return h
}

func resolveChannelBillingAPIBaseURL(channel *model.Channel, profile model.ChannelBillingProfile) string {
	if channel == nil {
		return ""
	}
	return normalizeChannelBillingAPIBaseURL(profile.ParseBillingConfig().APIBaseURL)
}

func normalizeChannelBillingAPIBaseURL(raw string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return trimmed
}

func fetchChannelBillingResponseBody(method, url string, channel *model.Channel, headers http.Header) ([]byte, error) {
	if client.HTTPClient == nil {
		return nil, errors.New("HTTP client is not initialized")
	}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	for k := range headers {
		req.Header.Add(k, headers.Get(k))
	}
	channelID := ""
	channelName := ""
	if channel != nil {
		channelID = strings.TrimSpace(channel.Id)
		channelName = strings.TrimSpace(channel.DisplayName())
	}
	requestPayload := buildHTTPRequestPayloadForLog(method, url, req.Header, nil)
	res, err := client.HTTPClient.Do(req)
	if err != nil {
		logger.Info(req.Context(), strings.Join([]string{
			fmt.Sprintf("[channel-billing] stage=request channel_id=%s name=%s", channelID, channelName),
			structuredPayloadField("request_payload", requestPayload),
			quotedField("error", err.Error()),
		}, " "))
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		logger.Info(req.Context(), strings.Join([]string{
			fmt.Sprintf("[channel-billing] stage=response channel_id=%s name=%s", channelID, channelName),
			structuredPayloadField("request_payload", requestPayload),
			structuredPayloadField("response_payload", buildHTTPResponsePayloadForLog(res.StatusCode, res.Header, nil)),
			quotedField("error", err.Error()),
		}, " "))
		return nil, err
	}
	responsePayload := buildHTTPResponsePayloadForLog(res.StatusCode, res.Header, body)
	logger.Info(req.Context(), strings.Join([]string{
		fmt.Sprintf("[channel-billing] stage=response channel_id=%s name=%s", channelID, channelName),
		structuredPayloadField("request_payload", requestPayload),
		structuredPayloadField("response_payload", responsePayload),
	}, " "))
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", res.StatusCode)
	}
	return body, nil
}

func fetchChannelCloseAIBillingAmount(channel *model.Channel, profile model.ChannelBillingProfile) (float64, error) {
	baseURL := resolveChannelBillingAPIBaseURL(channel, profile)
	if baseURL == "" {
		return 0, errors.New("渠道账务未配置账务 API 地址")
	}
	url := fmt.Sprintf("%s/dashboard/billing/credit_grants", baseURL)
	body, err := fetchChannelBillingResponseBody("GET", url, channel, buildBearerAuthHeader(channel.Key))

	if err != nil {
		return 0, err
	}
	response := OpenAICreditGrants{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	return response.TotalAvailable, nil
}

func fetchChannelOpenAISBBillingAmount(channel *model.Channel) (float64, error) {
	url := fmt.Sprintf("https://api.openai-sb.com/sb-api/user/status?api_key=%s", channel.Key)
	body, err := fetchChannelBillingResponseBody("GET", url, channel, buildBearerAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := OpenAISBUsageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	if response.Data == nil {
		return 0, errors.New(response.Msg)
	}
	balance, err := strconv.ParseFloat(response.Data.Credit, 64)
	if err != nil {
		return 0, err
	}
	return balance, nil
}

func fetchChannelAIProxyBillingAmount(channel *model.Channel) (float64, error) {
	url := "https://aiproxy.io/api/report/getUserOverview"
	headers := http.Header{}
	headers.Add("Api-Key", channel.Key)
	body, err := fetchChannelBillingResponseBody("GET", url, channel, headers)
	if err != nil {
		return 0, err
	}
	response := AIProxyUserOverviewResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	if !response.Success {
		return 0, fmt.Errorf("code: %d, message: %s", response.ErrorCode, response.Message)
	}
	return response.Data.TotalPoints, nil
}

func fetchChannelAPI2GPTBillingAmount(channel *model.Channel) (float64, error) {
	url := "https://api.api2gpt.com/dashboard/billing/credit_grants"
	body, err := fetchChannelBillingResponseBody("GET", url, channel, buildBearerAuthHeader(channel.Key))

	if err != nil {
		return 0, err
	}
	response := API2GPTUsageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	return response.TotalRemaining, nil
}

func fetchChannelAIGC2DBillingAmount(channel *model.Channel) (float64, error) {
	url := "https://api.aigc2d.com/dashboard/billing/credit_grants"
	body, err := fetchChannelBillingResponseBody("GET", url, channel, buildBearerAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := APGC2DGPTUsageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	return response.TotalAvailable, nil
}

func fetchChannelSiliconFlowBillingAmount(channel *model.Channel) (float64, error) {
	url := "https://api.siliconflow.cn/v1/user/info"
	body, err := fetchChannelBillingResponseBody("GET", url, channel, buildBearerAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := SiliconFlowUsageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	if response.Code != 20000 {
		return 0, fmt.Errorf("code: %d, message: %s", response.Code, response.Message)
	}
	balance, err := strconv.ParseFloat(response.Data.TotalBalance, 64)
	if err != nil {
		return 0, err
	}
	return balance, nil
}

func fetchChannelDeepSeekBillingAmount(channel *model.Channel) (float64, error) {
	url := "https://api.deepseek.com/user/balance"
	body, err := fetchChannelBillingResponseBody("GET", url, channel, buildBearerAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := DeepSeekUsageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	index := -1
	for i, balanceInfo := range response.BalanceInfos {
		if balanceInfo.Currency == "CNY" {
			index = i
			break
		}
	}
	if index == -1 {
		return 0, errors.New("currency CNY not found")
	}
	balance, err := strconv.ParseFloat(response.BalanceInfos[index].TotalBalance, 64)
	if err != nil {
		return 0, err
	}
	return balance, nil
}

func fetchChannelOpenRouterBillingAmount(channel *model.Channel) (float64, error) {
	url := "https://openrouter.ai/api/v1/credits"
	body, err := fetchChannelBillingResponseBody("GET", url, channel, buildBearerAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := OpenRouterResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	balance := response.Data.TotalCredits - response.Data.TotalUsage
	return balance, nil
}

func resolveChannelCDKKey(channel *model.Channel, profile model.ChannelBillingProfile) string {
	if cdk := profile.ParseBillingConfig().CDK; cdk != "" {
		return cdk
	}
	if channel != nil {
		cfg, _ := channel.LoadConfig()
		if cdk := model.ExtractCDKFromAccountURL(cfg.GetAccountBaseURL()); cdk != "" {
			return cdk
		}
	}
	return strings.TrimSpace(channel.Key)
}

func resolveChannelCDKBillingCurrency(profile model.ChannelBillingProfile) string {
	if currency := profile.ParseBillingConfig().Currency; currency != "" {
		return currency
	}
	return "USD"
}

func fetchChannelCDKBillingStats(channel *model.Channel, profile model.ChannelBillingProfile) (*CDKUsageStatsResponse, error) {
	baseURL := resolveChannelBillingAPIBaseURL(channel, profile)
	if baseURL == "" {
		return nil, errors.New("渠道账务未配置 CDK API 地址")
	}
	cdkKey := resolveChannelCDKKey(channel, profile)
	if cdkKey == "" {
		return nil, errors.New("渠道未配置 CDK 密钥")
	}
	statsURL := fmt.Sprintf("%s/api/public/usage/stats?cdk=%s", baseURL, cdkKey)
	body, err := fetchChannelBillingResponseBody("GET", statsURL, channel, http.Header{})
	if err != nil {
		return nil, err
	}
	response := CDKUsageStatsResponse{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if response.Code != 0 {
		return nil, fmt.Errorf("CDK 余额查询失败: %s", response.Message)
	}
	return &response, nil
}

func fetchChannelCDKCardInfo(channel *model.Channel, profile model.ChannelBillingProfile) (*CDKCardInfoResponse, error) {
	baseURL := resolveChannelBillingAPIBaseURL(channel, profile)
	if baseURL == "" {
		return nil, errors.New("渠道账务未配置 CDK API 地址")
	}
	cdkKey := resolveChannelCDKKey(channel, profile)
	if cdkKey == "" {
		return nil, errors.New("渠道未配置 CDK 密钥")
	}
	cardInfoURL := fmt.Sprintf("%s/api/public/card-info?cdk=%s", baseURL, cdkKey)
	body, err := fetchChannelBillingResponseBody("GET", cardInfoURL, channel, http.Header{})
	if err != nil {
		return nil, err
	}
	response := CDKCardInfoResponse{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if response.Code != 0 {
		return nil, fmt.Errorf("CDK 套餐信息查询失败: %s", response.Message)
	}
	return &response, nil
}

func buildCDKBillingSnapshotItems(stats *CDKUsageStatsResponse, currency string) []model.ChannelBillingSnapshotItem {
	data := stats.Data
	items := make([]model.ChannelBillingSnapshotItem, 0, 3)

	dailyResetAt := int64(0)
	if t, err := time.Parse(time.RFC3339Nano, data.ResetAt); err == nil {
		dailyResetAt = t.Unix()
	}
	weeklyResetAt := int64(0)
	if t, err := time.Parse(time.RFC3339Nano, data.WeeklyResetAt); err == nil {
		weeklyResetAt = t.Unix()
	}

	items = append(items, model.ChannelBillingSnapshotItem{
		QuotaType:  "daily",
		QuotaLabel: "日额度",
		Amount:     data.Remaining,
		Currency:   currency,
		ExpiresAt:  dailyResetAt,
		SortOrder:  1,
	})
	items = append(items, model.ChannelBillingSnapshotItem{
		QuotaType:  "weekly",
		QuotaLabel: "周额度",
		Amount:     data.WeeklyRemaining,
		Currency:   currency,
		ExpiresAt:  weeklyResetAt,
		SortOrder:  2,
	})
	items = append(items, model.ChannelBillingSnapshotItem{
		QuotaType:  "total",
		QuotaLabel: "总额度",
		Amount:     data.TotalRemaining,
		Currency:   currency,
		SortOrder:  3,
	})
	return items
}

func resolveChannelCDKBillingRequestURL(channel *model.Channel, profile model.ChannelBillingProfile) string {
	baseURL := resolveChannelBillingAPIBaseURL(channel, profile)
	if baseURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/api/public/usage/stats?cdk=***", baseURL)
}

func resolveChannelCDKCardInfoRequestURL(channel *model.Channel, profile model.ChannelBillingProfile) string {
	baseURL := resolveChannelBillingAPIBaseURL(channel, profile)
	if baseURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/api/public/card-info?cdk=***", baseURL)
}

func refreshAndPersistChannelCDKBilling(channel *model.Channel, profile model.ChannelBillingProfile, message string) (float64, error) {
	stats, err := fetchChannelCDKBillingStats(channel, profile)
	if err != nil {
		return 0, err
	}

	currency := resolveChannelCDKBillingCurrency(profile)
	items := buildCDKBillingSnapshotItems(stats, currency)
	now := helper.GetTimestamp()
	requestURL := resolveChannelCDKBillingRequestURL(channel, profile)

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		snapshotRow, err := model.CreateChannelBillingSnapshotWithDB(tx, model.ChannelBillingSnapshot{
			ChannelId:  strings.TrimSpace(channel.Id),
			SourceType: model.ChannelBillingSnapshotSourceAPI,
			Balance:    stats.Data.TotalRemaining,
			RawStatus:  "ok",
			Message:    strings.TrimSpace(message),
			RequestURL: requestURL,
			CreatedAt:  now,
		})
		if err != nil {
			return err
		}
		_, err = model.CreateChannelBillingSnapshotItemsWithDB(tx, snapshotRow.Id, channel.Id, items)
		return err
	})

	return stats.Data.TotalRemaining, err
}

func resolveChannelBillingRequestURLs(channel *model.Channel) []string {
	if channel == nil {
		return nil
	}
	profile, err := model.GetChannelBillingProfileByChannelIDWithDB(model.DB, channel.Id)
	if err != nil {
		return nil
	}
	baseURL := resolveChannelBillingAPIBaseURL(channel, profile)
	switch channel.GetChannelProtocol() {
	case relaychannel.CloseAI:
		if baseURL == "" {
			return nil
		}
		return []string{
			fmt.Sprintf("%s/dashboard/billing/credit_grants", baseURL),
		}
	case relaychannel.OpenAISB:
		return []string{
			"https://api.openai-sb.com/sb-api/user/status?api_key=***",
		}
	case relaychannel.AIProxy:
		return []string{"https://aiproxy.io/api/report/getUserOverview"}
	case relaychannel.API2GPT:
		return []string{"https://api.api2gpt.com/dashboard/billing/credit_grants"}
	case relaychannel.AIGC2D:
		return []string{"https://api.aigc2d.com/dashboard/billing/credit_grants"}
	case relaychannel.SiliconFlow:
		return []string{"https://api.siliconflow.cn/v1/user/info"}
	case relaychannel.DeepSeek:
		return []string{"https://api.deepseek.com/user/balance"}
	case relaychannel.OpenRouter:
		return []string{"https://openrouter.ai/api/v1/credits"}
	}
	if baseURL == "" {
		return nil
	}
	now := time.Now()
	startDate := fmt.Sprintf("%s-01", now.Format("2006-01"))
	endDate := now.Format("2006-01-02")
	return []string{
		fmt.Sprintf("%s/v1/dashboard/billing/subscription", baseURL),
		fmt.Sprintf("%s/v1/dashboard/billing/usage?start_date=%s&end_date=%s", baseURL, startDate, endDate),
	}
}

func resolveChannelBillingSnapshotCurrency(channel *model.Channel) string {
	switch strings.TrimSpace(strings.ToLower(channel.GetProtocol())) {
	case "closeai", "openai-sb", "api2gpt", "deepseek", "siliconflow":
		return "CNY"
	default:
		return "USD"
	}
}

func persistChannelAutoBillingSnapshot(channel *model.Channel, amount float64, message string) error {
	if channel == nil {
		return errors.New("渠道不存在")
	}
	now := helper.GetTimestamp()
	return model.DB.Transaction(func(tx *gorm.DB) error {
		snapshotRow, err := model.CreateChannelBillingSnapshotWithDB(tx, model.ChannelBillingSnapshot{
			ChannelId:  strings.TrimSpace(channel.Id),
			SourceType: model.ChannelBillingSnapshotSourceAPI,
			RawStatus:  "ok",
			Message:    strings.TrimSpace(message),
			RequestURL: strings.Join(resolveChannelBillingRequestURLs(channel), "\n"),
			CreatedAt:  now,
		})
		if err != nil {
			return err
		}
		_, err = model.CreateChannelBillingSnapshotItemsWithDB(tx, snapshotRow.Id, channel.Id, []model.ChannelBillingSnapshotItem{
			{
				QuotaType:  "total",
				QuotaLabel: "总额度",
				Amount:     amount,
				Currency:   resolveChannelBillingSnapshotCurrency(channel),
				SortOrder:  1,
				CreatedAt:  now,
			},
		})
		return err
	})
}

func refreshChannelBillingAmount(channel *model.Channel) (float64, error) {
	if channel == nil {
		return 0, errors.New("渠道不存在")
	}
	profile, err := model.GetChannelBillingProfileByChannelIDWithDB(model.DB, channel.Id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, errors.New("渠道账务未配置")
		}
		return 0, err
	}
	switch strings.TrimSpace(profile.BillingMode) {
	case model.ChannelBillingModeBuiltinCloseAI:
		return fetchChannelCloseAIBillingAmount(channel, profile)
	case model.ChannelBillingModeBuiltinOpenAISB:
		return fetchChannelOpenAISBBillingAmount(channel)
	case model.ChannelBillingModeBuiltinAIProxy:
		return fetchChannelAIProxyBillingAmount(channel)
	case model.ChannelBillingModeBuiltinAPI2GPT:
		return fetchChannelAPI2GPTBillingAmount(channel)
	case model.ChannelBillingModeBuiltinAIGC2D:
		return fetchChannelAIGC2DBillingAmount(channel)
	case model.ChannelBillingModeBuiltinSiliconFlow:
		return fetchChannelSiliconFlowBillingAmount(channel)
	case model.ChannelBillingModeBuiltinDeepSeek:
		return fetchChannelDeepSeekBillingAmount(channel)
	case model.ChannelBillingModeBuiltinOpenRouter:
		return fetchChannelOpenRouterBillingAmount(channel)
	case model.ChannelBillingModeBuiltinCDK:
		stats, err := fetchChannelCDKBillingStats(channel, profile)
		if err != nil {
			return 0, err
		}
		return stats.Data.TotalRemaining, nil
	case model.ChannelBillingModeBuiltinOpenAI:
		// Continue below with OpenAI-style subscription + usage billing API.
	default:
		return 0, errors.New("当前渠道不支持自动刷新账务")
	}
	baseURL := resolveChannelBillingAPIBaseURL(channel, profile)
	if baseURL == "" {
		return 0, errors.New("渠道账务未配置账务 API 地址")
	}
	url := fmt.Sprintf("%s/v1/dashboard/billing/subscription", baseURL)

	body, err := fetchChannelBillingResponseBody("GET", url, channel, buildBearerAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	subscription := OpenAISubscriptionResponse{}
	err = json.Unmarshal(body, &subscription)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	startDate := fmt.Sprintf("%s-01", now.Format("2006-01"))
	endDate := now.Format("2006-01-02")
	if !subscription.HasPaymentMethod {
		startDate = now.AddDate(0, 0, -100).Format("2006-01-02")
	}
	url = fmt.Sprintf("%s/v1/dashboard/billing/usage?start_date=%s&end_date=%s", baseURL, startDate, endDate)
	body, err = fetchChannelBillingResponseBody("GET", url, channel, buildBearerAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	usage := OpenAIUsageResponse{}
	err = json.Unmarshal(body, &usage)
	if err != nil {
		return 0, err
	}
	balance := subscription.HardLimitUSD - usage.TotalUsage/100
	return balance, nil
}

// UpdateChannelBilling submits a single-channel billing refresh task.
// The admin HTTP route is unified under POST /api/v1/admin/channel/{id}/refresh with action=billing.
func UpdateChannelBilling(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		logChannelAdminWarn(c, "refresh_billing", stringField("reason", "id 为空"))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "id 为空",
		})
		return
	}
	taskRow, reused, err := CreateChannelRefreshBillingTask(id, c.GetString(ctxkey.Id), c.GetString(helper.TraceIDKey))
	if err != nil {
		logChannelAdminWarn(c, "refresh_billing", stringField("channel_id", id), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logChannelAdminInfo(c, "refresh_billing", stringField("channel_id", taskRow.ChannelId), stringField("task_id", taskRow.Id), stringField("status", taskRow.Status))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"task": taskRow,
		},
		"meta": gin.H{
			"reused": reused,
		},
	})
	return
}

func refreshAllChannelsBilling() error {
	channels, err := channelsvc.GetAllBasic(0, 0, "all", true)
	if err != nil {
		return err
	}
	circuitStateByChannelID := loadChannelBillingRefreshCircuitStateMap(channels)
	for _, channel := range channels {
		if !shouldRefreshChannelBillingInBatch(channel, circuitStateByChannelID) {
			continue
		}
		profile, err := model.GetChannelBillingProfileByChannelIDWithDB(model.DB, channel.Id)
		if err != nil {
			time.Sleep(config.RequestInterval)
			continue
		}
		if _, err := refreshAndPersistChannelBillingEntitlements(channel, profile, "批量自动刷新账务"); err != nil {
			time.Sleep(config.RequestInterval)
			continue
		}
		time.Sleep(config.RequestInterval)
	}
	return nil
}

func loadChannelBillingRefreshCircuitStateMap(channels []*model.Channel) map[string]model.ChannelCircuitBreakerState {
	result := map[string]model.ChannelCircuitBreakerState{}
	channelIDs := make([]string, 0, len(channels))
	for _, channel := range channels {
		if channel == nil || channel.Status != model.ChannelStatusAutoDisabled {
			continue
		}
		if channelID := strings.TrimSpace(channel.Id); channelID != "" {
			channelIDs = append(channelIDs, channelID)
		}
	}
	if len(channelIDs) == 0 {
		return result
	}
	rows, err := model.ListChannelCircuitBreakerStatesByChannelIDsWithDB(model.DB, channelIDs)
	if err != nil {
		return result
	}
	for _, row := range rows {
		if channelID := strings.TrimSpace(row.ChannelId); channelID != "" {
			result[channelID] = row
		}
	}
	return result
}

func shouldRefreshChannelBillingInBatch(channel *model.Channel, circuitStateByChannelID map[string]model.ChannelCircuitBreakerState) bool {
	if channel == nil {
		return false
	}
	switch channel.Status {
	case model.ChannelStatusEnabled:
		return true
	case model.ChannelStatusAutoDisabled:
		state := circuitStateByChannelID[strings.TrimSpace(channel.Id)]
		return model.IsInsufficientBalanceCircuitBreakerState(state)
	default:
		return false
	}
}

func AutomaticallyRefreshChannelBilling(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Minute)
		logger.SysLog("refreshing channel billing")
		_ = refreshAllChannelsBilling()
		logger.SysLog("channel billing refresh done")
	}
}
