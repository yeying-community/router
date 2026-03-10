package channel

import (
	"context"
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
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

func summarizeModelTestResults(results []model.ChannelTest) (bool, string, int64, string) {
	if len(results) == 0 {
		return false, "未返回模型测试结果", 0, ""
	}
	passedModels := make([]string, 0, len(results))
	messageParts := make([]string, 0, len(results))
	var latencyMs int64
	modelName := ""
	for _, item := range results {
		if modelName == "" && strings.TrimSpace(item.Model) != "" {
			modelName = strings.TrimSpace(item.Model)
		}
		if item.Supported {
			if modelID := strings.TrimSpace(item.Model); modelID != "" {
				passedModels = append(passedModels, modelID)
			}
			if latencyMs == 0 && item.LatencyMs > 0 {
				latencyMs = item.LatencyMs
			}
			continue
		}
		if item.Status == model.ChannelTestStatusSkipped {
			continue
		}
		if msg := strings.TrimSpace(item.Message); msg != "" {
			messageParts = append(messageParts, msg)
		}
	}
	if len(passedModels) > 0 {
		return true, fmt.Sprintf("通过 %d/%d 个模型测试", len(passedModels), len(results)), latencyMs, modelName
	}
	if len(messageParts) > 0 {
		return false, messageParts[0], 0, modelName
	}
	return false, "未检测到可用模型", 0, modelName
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

func extractFailureSignalFromModelTests(results []model.ChannelTest) (*relaymodel.Error, int) {
	for _, item := range results {
		if item.Supported || item.Status == model.ChannelTestStatusSkipped {
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

func recordModelTestLog(ctx context.Context, channel *model.Channel, modelName string, success bool, responseMessage string, startedAt time.Time) {
	if channel == nil {
		return
	}
	logContent := fmt.Sprintf("渠道 %s 模型测试成功，结果：%s", channel.DisplayName(), responseMessage)
	if !success {
		logContent = fmt.Sprintf("渠道 %s 模型测试失败，错误：%s", channel.DisplayName(), responseMessage)
	}
	model.RecordTestLog(ctx, &model.Log{
		ChannelId:   channel.Id,
		ModelName:   modelName,
		Content:     logContent,
		ElapsedTime: helper.CalcElapsedTime(startedAt),
	})
}

type channelTestOptions struct {
	Mode           string
	Model          string
	PersistResults bool
}

func executeChannelModelTest(ctx context.Context, channel *model.Channel, options channelTestOptions) ([]model.ChannelTest, bool, string, int64, string, *relaymodel.Error, int, error) {
	if channel == nil {
		return nil, false, "", 0, "", nil, 0, fmt.Errorf("渠道不存在")
	}
	targetChannel := *channel
	testMode := normalizePreviewModelTestMode(options.Mode)
	testModel := strings.TrimSpace(options.Model)
	if testModel != "" {
		targetChannel.TestModel = testModel
	}
	startedAt := time.Now()
	results, err := runChannelModelTests(nil, &targetChannel, testMode, testModel, nil)
	if err != nil {
		modelName := testModel
		if modelName == "" {
			modelName = strings.TrimSpace(targetChannel.TestModel)
		}
		recordModelTestLog(ctx, channel, modelName, false, err.Error(), startedAt)
		return nil, false, err.Error(), 0, modelName, nil, 0, err
	}
	if options.PersistResults {
		if err := persistPreviewChannelTests(channel.Id, channel.GetModelConfigs(), results); err != nil {
			modelName := testModel
			if modelName == "" {
				modelName = strings.TrimSpace(targetChannel.TestModel)
			}
			message := "保存模型测试结果失败: " + err.Error()
			recordModelTestLog(ctx, channel, modelName, false, message, startedAt)
			return nil, false, message, 0, modelName, nil, 0, err
		}
		if savedChannel, getErr := channelsvc.GetByID(channel.Id, true); getErr == nil {
			_ = savedChannel.UpdateAbilities()
		}
	}
	success, responseMessage, latencyMs, modelName := summarizeModelTestResults(results)
	if strings.TrimSpace(modelName) == "" {
		modelName = strings.TrimSpace(targetChannel.TestModel)
		if strings.TrimSpace(testModel) != "" {
			modelName = strings.TrimSpace(testModel)
		}
	}
	relayErr, statusCode := extractFailureSignalFromModelTests(results)
	recordModelTestLog(ctx, channel, modelName, success, responseMessage, startedAt)
	return results, success, responseMessage, latencyMs, modelName, relayErr, statusCode, nil
}

// TestChannel godoc
// @Summary Test channel (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Channel ID"
// @Param model query string false "Model name"
// @Param mode query string false "Test mode (batch|model)"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/test/{id} [get]
func TestChannel(c *gin.Context) {
	ctx := c.Request.Context()
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		logChannelAdminWarn(c, "test", stringField("reason", "id 为空"))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "id 为空",
		})
		return
	}
	var err error
	channel, err := channelsvc.GetByID(id, true)
	if err != nil {
		logChannelAdminWarn(c, "test", stringField("channel_id", id), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	testModel := strings.TrimSpace(c.Query("model"))
	testMode := normalizePreviewModelTestMode(c.Query("mode"))
	results, success, responseMessage, milliseconds, modelName, _, _, err := executeChannelModelTest(ctx, channel, channelTestOptions{
		Mode:           testMode,
		Model:          testModel,
		PersistResults: true,
	})
	if err != nil {
		logChannelAdminWarn(c, "test", stringField("channel_id", channel.Id), stringField("name", channel.DisplayName()), stringField("model", modelName), stringField("reason", responseMessage))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": responseMessage,
		})
		return
	}
	go channel.UpdateResponseTime(milliseconds)
	consumedTime := float64(milliseconds) / 1000.0
	if !success {
		logChannelAdminWarn(c, "test", stringField("channel_id", channel.Id), stringField("name", channel.DisplayName()), stringField("model", modelName), int64Field("latency_ms", milliseconds), stringField("result", responseMessage))
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
	logChannelAdminInfo(c, "test", stringField("channel_id", channel.Id), stringField("name", channel.DisplayName()), stringField("model", modelName), int64Field("latency_ms", milliseconds), intField("result_count", len(results)))
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

func testChannels(ctx context.Context, notify bool, scope string) error {
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
			_, success, responseMessage, milliseconds, _, relayErr, statusCode, _ := executeChannelModelTest(ctx, channel, channelTestOptions{
				Mode:           previewChannelTestModeBatch,
				PersistResults: true,
			})
			if isChannelEnabled && success && milliseconds > disableThreshold {
				timeoutErr := fmt.Errorf("响应时间 %.2fs 超过阈值 %.2fs", float64(milliseconds)/1000.0, float64(disableThreshold)/1000.0)
				if config.AutomaticDisableChannelEnabled {
					monitor.DisableChannel(channel.Id, channel.DisplayName(), timeoutErr.Error())
				} else {
					_ = message.Notify(message.ByAll, fmt.Sprintf("渠道 %s （%s）测试超时", channel.DisplayName(), channel.Id), "", timeoutErr.Error())
				}
			}
			if isChannelEnabled && !success && monitor.ShouldDisableChannel(relayErr, statusCode) {
				monitor.DisableChannel(channel.Id, channel.DisplayName(), responseMessage)
			}
			if !isChannelEnabled && success && monitor.ShouldEnableChannel(nil, nil) {
				monitor.EnableChannel(channel.Id, channel.DisplayName())
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
	err := testChannels(ctx, true, scope)
	if err != nil {
		logChannelAdminWarn(c, "test_all", stringField("scope", scope), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logChannelAdminInfo(c, "test_all", stringField("scope", scope))
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
		_ = testChannels(ctx, false, "all")
		logger.SysLog("channel test finished")
	}
}
