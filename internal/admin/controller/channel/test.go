package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/common/message"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/monitor"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
	"github.com/yeying-community/router/internal/relay/adaptor/openai"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

func parseTestResponse(resp string) (*openai.TextResponse, string, error) {
	var response openai.TextResponse
	err := json.Unmarshal([]byte(resp), &response)
	if err != nil {
		return nil, "", err
	}
	if len(response.Choices) == 0 {
		return nil, "", errors.New("response has no choices")
	}
	stringContent, ok := response.Choices[0].Content.(string)
	if !ok {
		return nil, "", errors.New("response content is not string")
	}
	return &response, stringContent, nil
}

type responsesEnvelope struct {
	Output []struct {
		Content []struct {
			Type       string `json:"type"`
			Text       string `json:"text"`
			OutputText string `json:"output_text"`
		} `json:"content"`
	} `json:"output"`
}

func parseResponsesTestResponse(resp string) (string, error) {
	var env responsesEnvelope
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		return "", err
	}
	contentTypes := make([]string, 0)
	for _, output := range env.Output {
		for _, content := range output.Content {
			if content.Type != "" {
				contentTypes = append(contentTypes, content.Type)
			} else {
				contentTypes = append(contentTypes, "<empty>")
			}
			if content.Text != "" {
				return content.Text, nil
			}
			if content.OutputText != "" {
				return content.OutputText, nil
			}
		}
	}
	return "", errors.New("response has no output text, content types: " + strings.Join(contentTypes, ","))
}

func parseChannelTestResponse(resp string) (string, error) {
	_, chatText, chatErr := parseTestResponse(resp)
	if chatErr == nil {
		return chatText, nil
	}
	responsesText, responsesErr := parseResponsesTestResponse(resp)
	if responsesErr == nil {
		return responsesText, nil
	}
	return "", fmt.Errorf("parse as chat failed: %v; parse as responses failed: %v", chatErr, responsesErr)
}

func summarizeCapabilityTestResults(results []previewCapabilityResult) (bool, string, int64, string) {
	if len(results) == 0 {
		return false, "未返回能力测试结果", 0, ""
	}
	supportedLabels := make([]string, 0, len(results))
	messageParts := make([]string, 0, len(results))
	var latencyMs int64
	modelName := ""
	for _, item := range results {
		if modelName == "" && strings.TrimSpace(item.Model) != "" {
			modelName = strings.TrimSpace(item.Model)
		}
		if item.Supported {
			label := strings.TrimSpace(item.Label)
			if label == "" {
				label = strings.TrimSpace(item.Capability)
			}
			if label != "" {
				supportedLabels = append(supportedLabels, label)
			}
			if latencyMs == 0 && item.LatencyMs > 0 {
				latencyMs = item.LatencyMs
			}
			continue
		}
		if item.Status == previewCapabilityStatusSkipped {
			continue
		}
		if msg := strings.TrimSpace(item.Message); msg != "" {
			messageParts = append(messageParts, msg)
		}
	}
	if len(supportedLabels) > 0 {
		return true, "支持能力: " + strings.Join(supportedLabels, ", "), latencyMs, modelName
	}
	if len(messageParts) > 0 {
		return false, messageParts[0], 0, modelName
	}
	return false, "未检测到可用能力", 0, modelName
}

func parseHTTPStatusCodeFromMessage(message string) int {
	normalized := strings.TrimSpace(strings.ToLower(message))
	const prefix = "http status code:"
	if !strings.HasPrefix(normalized, prefix) {
		return 0
	}
	var statusCode int
	if _, err := fmt.Sscanf(strings.TrimSpace(normalized[len(prefix):]), "%d", &statusCode); err != nil {
		return 0
	}
	return statusCode
}

func extractFailureSignalFromCapabilityResults(results []previewCapabilityResult) (*relaymodel.Error, int) {
	for _, item := range results {
		if item.Supported || item.Status == previewCapabilityStatusSkipped {
			continue
		}
		message := strings.TrimSpace(item.Message)
		if message == "" {
			continue
		}
		return &relaymodel.Error{Message: message}, parseHTTPStatusCodeFromMessage(message)
	}
	return nil, 0
}

func recordCapabilityTestLog(ctx context.Context, channel *model.Channel, modelName string, success bool, responseMessage string, startedAt time.Time) {
	if channel == nil {
		return
	}
	logContent := fmt.Sprintf("渠道 %s 能力测试成功，结果：%s", channel.Name, responseMessage)
	if !success {
		logContent = fmt.Sprintf("渠道 %s 能力测试失败，错误：%s", channel.Name, responseMessage)
	}
	model.RecordTestLog(ctx, &model.Log{
		ChannelId:   channel.Id,
		ModelName:   modelName,
		Content:     logContent,
		ElapsedTime: helper.CalcElapsedTime(startedAt),
	})
}

func executeChannelCapabilityTest(ctx context.Context, channel *model.Channel, overrideTestModel string, userAgent string) ([]previewCapabilityResult, bool, string, int64, string, *relaymodel.Error, int, error) {
	if channel == nil {
		return nil, false, "", 0, "", nil, 0, fmt.Errorf("渠道不存在")
	}
	targetChannel := *channel
	if strings.TrimSpace(overrideTestModel) != "" {
		targetChannel.TestModel = strings.TrimSpace(overrideTestModel)
	}
	startedAt := time.Now()
	results, err := runChannelCapabilityTests(&targetChannel, userAgent)
	if err != nil {
		modelName := strings.TrimSpace(targetChannel.TestModel)
		recordCapabilityTestLog(ctx, channel, modelName, false, err.Error(), startedAt)
		return nil, false, err.Error(), 0, modelName, nil, 0, err
	}
	if err := persistPreviewCapabilityResults(channel.Id, results); err != nil {
		modelName := strings.TrimSpace(targetChannel.TestModel)
		message := "保存能力测试结果失败: " + err.Error()
		recordCapabilityTestLog(ctx, channel, modelName, false, message, startedAt)
		return nil, false, message, 0, modelName, nil, 0, err
	}
	success, responseMessage, latencyMs, modelName := summarizeCapabilityTestResults(results)
	if strings.TrimSpace(modelName) == "" {
		modelName = strings.TrimSpace(targetChannel.TestModel)
	}
	relayErr, statusCode := extractFailureSignalFromCapabilityResults(results)
	recordCapabilityTestLog(ctx, channel, modelName, success, responseMessage, startedAt)
	return results, success, responseMessage, latencyMs, modelName, relayErr, statusCode, nil
}

// TestChannel godoc
// @Summary Test channel (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "Channel ID"
// @Param model query string false "Model name"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/test/{id} [get]
func TestChannel(c *gin.Context) {
	ctx := c.Request.Context()
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "id 为空",
		})
		return
	}
	var err error
	channel, err := channelsvc.GetByID(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	testModel := strings.TrimSpace(c.Query("model"))
	results, success, responseMessage, milliseconds, modelName, _, _, err := executeChannelCapabilityTest(ctx, channel, testModel, c.Request.UserAgent())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": responseMessage,
		})
		return
	}
	go channel.UpdateResponseTime(milliseconds)
	consumedTime := float64(milliseconds) / 1000.0
	if !success {
		c.JSON(http.StatusOK, gin.H{
			"success":   false,
			"message":   responseMessage,
			"time":      consumedTime,
			"modelName": modelName,
			"data": gin.H{
				"results": results,
			},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   responseMessage,
		"time":      consumedTime,
		"modelName": modelName,
		"data": gin.H{
			"results": results,
		},
	})
	return
}

var testAllChannelsLock sync.Mutex
var testAllChannelsRunning bool = false

func testChannels(ctx context.Context, notify bool, scope string, userAgent string) error {
	if config.RootUserEmail == "" {
		config.RootUserEmail = model.GetRootUserEmail()
	}
	testAllChannelsLock.Lock()
	if testAllChannelsRunning {
		testAllChannelsLock.Unlock()
		return errors.New("测试已在运行中")
	}
	testAllChannelsRunning = true
	testAllChannelsLock.Unlock()
	channels, err := channelsvc.GetAll(0, 0, scope)
	if err != nil {
		return err
	}
	var disableThreshold = int64(config.ChannelDisableThreshold * 1000)
	if disableThreshold == 0 {
		disableThreshold = 10000000 // a impossible value
	}
	go func() {
		defer func() {
			testAllChannelsLock.Lock()
			testAllChannelsRunning = false
			testAllChannelsLock.Unlock()
			if notify {
				err := message.Notify(message.ByAll, "渠道测试完成", "", "渠道测试完成，如果没有收到禁用通知，说明所有渠道都正常")
				if err != nil {
					logger.SysError(fmt.Sprintf("failed to send email: %s", err.Error()))
				}
			}
		}()
		for _, channel := range channels {
			isChannelEnabled := channel.Status == model.ChannelStatusEnabled
			_, success, responseMessage, milliseconds, _, relayErr, statusCode, _ := executeChannelCapabilityTest(ctx, channel, strings.TrimSpace(channel.TestModel), userAgent)
			if isChannelEnabled && success && milliseconds > disableThreshold {
				timeoutErr := fmt.Errorf("响应时间 %.2fs 超过阈值 %.2fs", float64(milliseconds)/1000.0, float64(disableThreshold)/1000.0)
				if config.AutomaticDisableChannelEnabled {
					monitor.DisableChannel(channel.Id, channel.Name, timeoutErr.Error())
				} else {
					_ = message.Notify(message.ByAll, fmt.Sprintf("渠道 %s （%s）测试超时", channel.Name, channel.Id), "", timeoutErr.Error())
				}
			}
			if isChannelEnabled && !success && monitor.ShouldDisableChannel(relayErr, statusCode) {
				monitor.DisableChannel(channel.Id, channel.Name, responseMessage)
			}
			if !isChannelEnabled && success && monitor.ShouldEnableChannel(nil, nil) {
				monitor.EnableChannel(channel.Id, channel.Name)
			}
			channel.UpdateResponseTime(milliseconds)
			time.Sleep(config.RequestInterval)
		}
	}()
	return nil
}

// TestChannels godoc
// @Summary Test all channels (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param scope query string false "Scope (all|enabled|disabled)"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/test [get]
func TestChannels(c *gin.Context) {
	ctx := c.Request.Context()
	scope := c.Query("scope")
	if scope == "" {
		scope = "all"
	}
	err := testChannels(ctx, true, scope, c.Request.UserAgent())
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

func AutomaticallyTestChannels(frequency int) {
	ctx := context.Background()
	for {
		time.Sleep(time.Duration(frequency) * time.Minute)
		logger.SysLog("testing all channels")
		_ = testChannels(ctx, false, "all", "")
		logger.SysLog("channel test finished")
	}
}
