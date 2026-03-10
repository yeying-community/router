package channel

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yeying-community/router/common/client"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/monitor"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"

	"github.com/gin-gonic/gin"
)

// https://github.com/yeying-community/router/issues/79

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

// GetAuthHeader get auth header
func GetAuthHeader(token string) http.Header {
	h := http.Header{}
	h.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	return h
}

func GetResponseBody(method, url string, channel *model.Channel, headers http.Header) ([]byte, error) {
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

func updateChannelCloseAIBalance(channel *model.Channel) (float64, error) {
	url := fmt.Sprintf("%s/dashboard/billing/credit_grants", channel.GetBaseURL())
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))

	if err != nil {
		return 0, err
	}
	response := OpenAICreditGrants{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(response.TotalAvailable)
	return response.TotalAvailable, nil
}

func updateChannelOpenAISBBalance(channel *model.Channel) (float64, error) {
	url := fmt.Sprintf("https://api.openai-sb.com/sb-api/user/status?api_key=%s", channel.Key)
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
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
	channel.UpdateBalance(balance)
	return balance, nil
}

func updateChannelAIProxyBalance(channel *model.Channel) (float64, error) {
	url := "https://aiproxy.io/api/report/getUserOverview"
	headers := http.Header{}
	headers.Add("Api-Key", channel.Key)
	body, err := GetResponseBody("GET", url, channel, headers)
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
	channel.UpdateBalance(response.Data.TotalPoints)
	return response.Data.TotalPoints, nil
}

func updateChannelAPI2GPTBalance(channel *model.Channel) (float64, error) {
	url := "https://api.api2gpt.com/dashboard/billing/credit_grants"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))

	if err != nil {
		return 0, err
	}
	response := API2GPTUsageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(response.TotalRemaining)
	return response.TotalRemaining, nil
}

func updateChannelAIGC2DBalance(channel *model.Channel) (float64, error) {
	url := "https://api.aigc2d.com/dashboard/billing/credit_grants"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := APGC2DGPTUsageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(response.TotalAvailable)
	return response.TotalAvailable, nil
}

func updateChannelSiliconFlowBalance(channel *model.Channel) (float64, error) {
	url := "https://api.siliconflow.cn/v1/user/info"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
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
	channel.UpdateBalance(balance)
	return balance, nil
}

func updateChannelDeepSeekBalance(channel *model.Channel) (float64, error) {
	url := "https://api.deepseek.com/user/balance"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
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
	channel.UpdateBalance(balance)
	return balance, nil
}

func updateChannelOpenRouterBalance(channel *model.Channel) (float64, error) {
	url := "https://openrouter.ai/api/v1/credits"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := OpenRouterResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	balance := response.Data.TotalCredits - response.Data.TotalUsage
	channel.UpdateBalance(balance)
	return balance, nil
}

func updateChannelBalance(channel *model.Channel) (float64, error) {
	channelProtocol := channel.GetChannelProtocol()
	baseURL := relaychannel.BaseURLByProtocol(channel.GetProtocol())
	if channel.GetBaseURL() == "" {
		channel.BaseURL = &baseURL
	}
	switch channelProtocol {
	case relaychannel.OpenAI:
		if channel.GetBaseURL() != "" {
			baseURL = channel.GetBaseURL()
		}
	case relaychannel.Azure:
		return 0, errors.New("尚未实现")
	case relaychannel.Custom:
		baseURL = channel.GetBaseURL()
	case relaychannel.CloseAI:
		return updateChannelCloseAIBalance(channel)
	case relaychannel.OpenAISB:
		return updateChannelOpenAISBBalance(channel)
	case relaychannel.AIProxy:
		return updateChannelAIProxyBalance(channel)
	case relaychannel.API2GPT:
		return updateChannelAPI2GPTBalance(channel)
	case relaychannel.AIGC2D:
		return updateChannelAIGC2DBalance(channel)
	case relaychannel.SiliconFlow:
		return updateChannelSiliconFlowBalance(channel)
	case relaychannel.DeepSeek:
		return updateChannelDeepSeekBalance(channel)
	case relaychannel.OpenRouter:
		return updateChannelOpenRouterBalance(channel)
	default:
		return 0, errors.New("尚未实现")
	}
	url := fmt.Sprintf("%s/v1/dashboard/billing/subscription", baseURL)

	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
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
	body, err = GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	usage := OpenAIUsageResponse{}
	err = json.Unmarshal(body, &usage)
	if err != nil {
		return 0, err
	}
	balance := subscription.HardLimitUSD - usage.TotalUsage/100
	channel.UpdateBalance(balance)
	return balance, nil
}

// UpdateChannelBalance godoc
// @Summary Update channel balance (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Channel ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/update_balance/{id} [get]
func UpdateChannelBalance(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		logChannelAdminWarn(c, "refresh_balance", stringField("reason", "id 为空"))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "id 为空",
		})
		return
	}
	var err error
	channel, err := channelsvc.GetByID(id)
	if err != nil {
		logChannelAdminWarn(c, "refresh_balance", stringField("channel_id", id), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	balance, err := updateChannelBalance(channel)
	if err != nil {
		logChannelAdminWarn(c, "refresh_balance", stringField("channel_id", channel.Id), stringField("name", channel.DisplayName()), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logChannelAdminInfo(c, "refresh_balance", stringField("channel_id", channel.Id), stringField("name", channel.DisplayName()), floatField("balance", balance))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"balance": balance,
	})
	return
}

func updateAllChannelsBalance() error {
	channels, err := channelsvc.GetAllBasic(0, 0, "all", true)
	if err != nil {
		return err
	}
	for _, channel := range channels {
		if channel.Status != model.ChannelStatusEnabled {
			continue
		}
		// TODO: support Azure
		channelProtocol := channel.GetChannelProtocol()
		if channelProtocol != relaychannel.OpenAI && channelProtocol != relaychannel.Custom {
			continue
		}
		balance, err := updateChannelBalance(channel)
		if err != nil {
			continue
		} else {
			// err is nil & balance <= 0 means quota is used up
			if balance <= 0 {
				monitor.DisableChannel(channel.Id, channel.DisplayName(), "余额不足")
			}
		}
		time.Sleep(config.RequestInterval)
	}
	return nil
}

// UpdateAllChannelsBalance godoc
// @Summary Update all channels balance (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/update_balance [get]
func UpdateAllChannelsBalance(c *gin.Context) {
	//err := updateAllChannelsBalance()
	//if err != nil {
	//	c.JSON(http.StatusOK, gin.H{
	//		"success": false,
	//		"message": err.Error(),
	//	})
	//	return
	//}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func AutomaticallyUpdateChannels(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Minute)
		logger.SysLog("updating all channels")
		_ = updateAllChannelsBalance()
		logger.SysLog("channels update done")
	}
}
