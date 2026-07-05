package channel

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/yeying-community/router/common/client"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/monitor"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
	"github.com/yeying-community/router/internal/relay"
	relayadaptor "github.com/yeying-community/router/internal/relay/adaptor"
	aliadaptor "github.com/yeying-community/router/internal/relay/adaptor/ali"
	openaiadaptor "github.com/yeying-community/router/internal/relay/adaptor/openai"
	volcenginerealtime "github.com/yeying-community/router/internal/relay/adaptor/volcengine/realtime"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	relaycontroller "github.com/yeying-community/router/internal/relay/controller"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/transport/http/middleware"
	"gorm.io/gorm"
)

const defaultChannelImageEditTestURL = "https://webdav.yeying.pub/api/v1/public/share/03fed01d-6f6b-4ffc-9eb0-d53f21fc17d2/blue_blank.png"

type imageEditTestInput struct {
	URL     string
	DataURI string
}

type channelModelTestTargetItem struct {
	Model             string `json:"model"`
	Endpoint          string `json:"endpoint,omitempty"`
	IsStream          *bool  `json:"is_stream,omitempty"`
	ResponsesTestMode string `json:"responses_test_mode,omitempty"`
}

const (
	channelModelResponsesTestModeText            = "text"
	channelModelResponsesTestModeImageGeneration = "image_generation"
)

func normalizeResponsesTestMode(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case channelModelResponsesTestModeImageGeneration:
		return channelModelResponsesTestModeImageGeneration
	default:
		return channelModelResponsesTestModeText
	}
}

func normalizeAudioTestLanguage(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "zh", "zh-cn", "zh_hans":
		return "zh-CN"
	case "en", "en-us", "en_us":
		return "en-US"
	default:
		return "zh-CN"
	}
}

func buildAudioModelTestInput(language string) string {
	switch normalizeAudioTestLanguage(language) {
	case "en-US":
		return "This is Router's voice test."
	default:
		return "这是 Router 的语音测试。"
	}
}

type channelModelTestExecution struct {
	LatencyMs          int64
	IsStream           bool
	Message            string
	BaseURL            string
	RequestURL         string
	InputPayload       string
	OutputPayload      string
	ResponseStatusCode int
	ResponseHeader     http.Header
	ResponseBody       []byte
	Err                error
}

const (
	channelModelTestModeBatch  = "batch"
	channelModelTestModeSingle = "model"

	channelModelTestRetryMax = 3
)

func persistChannelModelTests(channelID string, taskID string, results []model.ChannelTest) error {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	targetModels := make([]string, 0, len(results))
	for _, item := range results {
		if strings.TrimSpace(item.Model) == "" {
			continue
		}
		targetModels = append(targetModels, item.Model)
	}
	targetModels = model.NormalizeChannelModelIDsPreserveOrder(targetModels)
	restoredModels := make([]string, 0)
	restoredEndpoints := make([]channelModelEndpointRestore, 0)
	shouldRestoreChannel, err := shouldRestoreInsufficientBalanceChannelAfterSuccessfulTests(normalizedChannelID, results)
	if err != nil {
		return err
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if _, err := model.AppendChannelTestsForModelsWithDB(tx, normalizedChannelID, targetModels, results); err != nil {
			return err
		}
		if err := model.EnsureChannelTestModelWithDB(tx, normalizedChannelID); err != nil {
			return err
		}
		if err := model.UpsertChannelModelEndpointTestResultsWithDB(tx, normalizedChannelID, taskID, results); err != nil {
			return err
		}
		models, endpoints, err := restoreRuntimeDisabledCapabilitiesAfterSuccessfulTests(tx, normalizedChannelID, results)
		if err != nil {
			return err
		}
		restoredModels = append(restoredModels, models...)
		restoredEndpoints = append(restoredEndpoints, endpoints...)
		return nil
	})
	if err != nil {
		return err
	}
	if len(restoredModels) > 0 || len(restoredEndpoints) > 0 {
		notifyAutoRestoredCapabilities(normalizedChannelID, restoredModels, restoredEndpoints)
	}
	if err := model.RefreshGroupModelChannelsForChannelWithDB(model.DB, normalizedChannelID); err != nil {
		return err
	}
	if shouldRestoreChannel {
		channelRow, err := channelsvc.GetByID(normalizedChannelID)
		if err != nil {
			return err
		}
		if err := model.RecordInsufficientBalanceChannelCircuitBreakerRecovered(normalizedChannelID); err != nil {
			return err
		}
		return monitor.EnableChannel(normalizedChannelID, channelRow.DisplayName())
	}
	return nil
}

func shouldRestoreInsufficientBalanceChannelAfterSuccessfulTests(channelID string, results []model.ChannelTest) (bool, error) {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return false, nil
	}
	if !hasSuccessfulChannelModelTest(results) {
		return false, nil
	}
	channelRow, err := channelsvc.GetByID(normalizedChannelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if channelRow.Status != model.ChannelStatusAutoDisabled {
		return false, nil
	}
	state, err := model.GetChannelCircuitBreakerState(normalizedChannelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return model.IsInsufficientBalanceCircuitBreakerState(state), nil
}

func hasSuccessfulChannelModelTest(results []model.ChannelTest) bool {
	for _, result := range model.NormalizeChannelTestRows(results) {
		if result.Supported && model.NormalizeChannelTestStatus(result.Status) == model.ChannelTestStatusSupported {
			return true
		}
	}
	return false
}

type channelModelEndpointRestore struct {
	Model    string
	Endpoint string
}

func restoreRuntimeDisabledCapabilitiesAfterSuccessfulTests(tx *gorm.DB, channelID string, results []model.ChannelTest) ([]string, []channelModelEndpointRestore, error) {
	restoredModels := make([]string, 0)
	restoredEndpoints := make([]channelModelEndpointRestore, 0)
	seenModels := make(map[string]struct{})
	seenEndpoints := make(map[string]struct{})
	for _, result := range model.NormalizeChannelTestRows(results) {
		if !result.Supported || model.NormalizeChannelTestStatus(result.Status) != model.ChannelTestStatusSupported {
			continue
		}
		modelName := strings.TrimSpace(result.Model)
		endpoint := model.NormalizeRequestedChannelModelEndpoint(result.Endpoint)
		if modelName == "" || endpoint == "" {
			continue
		}
		if _, ok := seenModels[modelName]; !ok {
			restored, err := model.RestoreRuntimeDisabledChannelModelCapabilityWithDB(tx, channelID, modelName)
			if err != nil {
				return nil, nil, err
			}
			if restored {
				restoredModels = append(restoredModels, modelName)
			}
			seenModels[modelName] = struct{}{}
		}
		endpointKey := modelName + "::" + endpoint
		if _, ok := seenEndpoints[endpointKey]; ok {
			continue
		}
		restored, err := model.RestoreRuntimeDisabledChannelModelEndpointCapabilityWithDB(tx, channelID, modelName, endpoint)
		if err != nil {
			return nil, nil, err
		}
		if restored {
			restoredEndpoints = append(restoredEndpoints, channelModelEndpointRestore{
				Model:    modelName,
				Endpoint: endpoint,
			})
		}
		seenEndpoints[endpointKey] = struct{}{}
	}
	return restoredModels, restoredEndpoints, nil
}

func notifyAutoRestoredCapabilities(channelID string, restoredModels []string, restoredEndpoints []channelModelEndpointRestore) {
	channelRow, err := channelsvc.GetByID(channelID)
	if err != nil {
		return
	}
	if len(restoredModels) > 0 {
		if err := channelRow.UpdateGroupModelChannels(); err != nil {
			return
		}
	}
	channelName := channelRow.DisplayName()
	for _, modelName := range model.NormalizeChannelModelIDsPreserveOrder(restoredModels) {
		monitor.NotifyChannelModelCapabilityRestored(channelID, channelName, modelName, "auto-test")
	}
	for _, item := range restoredEndpoints {
		monitor.NotifyChannelModelEndpointCapabilityRestored(channelID, channelName, item.Model, item.Endpoint, "auto-test")
	}
}

func normalizeChannelModelTestMode(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case channelModelTestModeSingle:
		return channelModelTestModeSingle
	case channelModelTestModeBatch:
		return channelModelTestModeBatch
	default:
		return channelModelTestModeBatch
	}
}

func selectedChannelModels(channel *model.Channel) []model.ChannelModel {
	if channel == nil {
		return nil
	}
	rows := channel.GetChannelModels()
	if len(rows) == 0 {
		return nil
	}
	selected := make([]model.ChannelModel, 0, len(rows))
	for _, row := range rows {
		if !row.Selected {
			continue
		}
		selected = append(selected, row)
	}
	return selected
}

func resolveSelectionModelType(row model.ChannelModel) string {
	resolved := strings.TrimSpace(row.Type)
	if resolved != "" {
		return resolved
	}
	referenceModel := strings.TrimSpace(row.UpstreamModel)
	if referenceModel == "" {
		referenceModel = strings.TrimSpace(row.Model)
	}
	return model.InferModelType(referenceModel)
}

func resolveChannelTestTargetModels(channel *model.Channel, mode string, requestedModel string, requestedModels []string) []model.ChannelModel {
	if channel == nil {
		return nil
	}
	allRows := channel.GetChannelModels()
	if len(allRows) == 0 {
		return nil
	}
	selectedRows := selectedChannelModels(channel)

	targets := model.NormalizeChannelModelIDsPreserveOrder(requestedModels)
	if len(targets) == 0 && normalizeChannelModelTestMode(mode) == channelModelTestModeSingle {
		targetModel := strings.TrimSpace(requestedModel)
		if targetModel == "" && channel != nil {
			targetModel = strings.TrimSpace(channel.TestModel)
		}
		if targetModel != "" {
			targets = []string{targetModel}
		}
	}
	if len(targets) == 0 {
		if len(selectedRows) == 0 {
			return nil
		}
		return selectedRows
	}

	result := make([]model.ChannelModel, 0, len(targets))
	targetSet := make(map[string]struct{}, len(targets))
	for _, item := range targets {
		targetSet[item] = struct{}{}
	}
	for _, row := range allRows {
		if _, ok := targetSet[strings.TrimSpace(row.Model)]; ok {
			result = append(result, row)
			continue
		}
		if _, ok := targetSet[strings.TrimSpace(row.UpstreamModel)]; ok {
			result = append(result, row)
		}
	}
	return result
}

func buildChannelModelTestResult(row model.ChannelModel, execution channelModelTestExecution) model.ChannelTest {
	modelType := resolveSelectionModelType(row)
	endpoint := model.NormalizeRequestedChannelModelEndpoint(strings.TrimSpace(row.Endpoint))
	if endpoint == "" {
		endpoint = strings.TrimSpace(row.Endpoint)
	}
	result := model.ChannelTest{
		Model:         strings.TrimSpace(row.Model),
		UpstreamModel: strings.TrimSpace(row.UpstreamModel),
		Type:          modelType,
		Endpoint:      endpoint,
		IsStream:      execution.IsStream,
		LatencyMs:     execution.LatencyMs,
		Message:       strings.TrimSpace(execution.Message),
	}
	if result.UpstreamModel == "" {
		result.UpstreamModel = result.Model
	}
	if execution.Err == nil {
		result.Status = model.ChannelTestStatusSupported
		result.Supported = true
		return result
	}
	errMessage := strings.TrimSpace(execution.Err.Error())
	if errMessage == "" {
		errMessage = "模型测试失败"
	}
	result.Message = errMessage
	result.Status = model.ChannelTestStatusUnsupported
	if strings.Contains(strings.ToLower(errMessage), "暂不自动探测") {
		result.Status = model.ChannelTestStatusSkipped
	}
	return result
}

func runSingleChannelModelTest(channel *model.Channel, row model.ChannelModel) (model.ChannelTest, channelModelTestExecution) {
	return runSingleChannelModelTestWithContextAndStream(context.Background(), channel, row, nil, "", imageEditTestInput{}, "")
}

func runSingleChannelModelTestWithContext(ctx context.Context, channel *model.Channel, row model.ChannelModel) (model.ChannelTest, channelModelTestExecution) {
	return runSingleChannelModelTestWithContextAndStream(ctx, channel, row, nil, "", imageEditTestInput{}, "")
}

func resolveChannelModelTestRequestURL(baseURL string, path string, adaptor relayadaptor.Adaptor, relayMeta *meta.Meta) string {
	requestURL := resolveChannelEndpointURL(baseURL, path)
	if adaptor == nil || relayMeta == nil {
		return requestURL
	}
	if resolvedRequestURL, err := adaptor.GetRequestURL(relayMeta); err == nil && strings.TrimSpace(resolvedRequestURL) != "" {
		return resolvedRequestURL
	}
	return requestURL
}

type channelModelTestKind string

const (
	channelModelTestKindText           channelModelTestKind = "text"
	channelModelTestKindTextResponses  channelModelTestKind = "text_responses"
	channelModelTestKindImage          channelModelTestKind = "image"
	channelModelTestKindImageResponses channelModelTestKind = "image_responses"
	channelModelTestKindImageEdit      channelModelTestKind = "image_edit"
	channelModelTestKindBatch          channelModelTestKind = "batch"
	channelModelTestKindAudio          channelModelTestKind = "audio"
	channelModelTestKindRealtime       channelModelTestKind = "realtime"
	channelModelTestKindVideo          channelModelTestKind = "video"
	channelModelTestKindEmbedding      channelModelTestKind = "embedding"
)

func resolveChannelModelTestKind(modelType string, endpoint string, responsesTestMode string) channelModelTestKind {
	switch model.NormalizeRequestedChannelModelEndpoint(endpoint) {
	case model.ChannelModelEndpointChat, model.ChannelModelEndpointMessages:
		return channelModelTestKindText
	case model.ChannelModelEndpointResponses:
		if normalizeResponsesTestMode(responsesTestMode) == channelModelResponsesTestModeImageGeneration {
			return channelModelTestKindImageResponses
		}
		if strings.EqualFold(strings.TrimSpace(modelType), model.ProviderModelTypeImage) {
			return channelModelTestKindImageResponses
		}
		return channelModelTestKindTextResponses
	case model.ChannelModelEndpointImageEdit:
		return channelModelTestKindImageEdit
	case model.ChannelModelEndpointBatches:
		return channelModelTestKindBatch
	case model.ChannelModelEndpointImages:
		return channelModelTestKindImage
	case model.ChannelModelEndpointAudio:
		return channelModelTestKindAudio
	case model.ChannelModelEndpointRealtime:
		return channelModelTestKindRealtime
	case model.ChannelModelEndpointVideos:
		return channelModelTestKindVideo
	case model.ChannelModelEndpointEmbeddings:
		return channelModelTestKindEmbedding
	default:
		return channelModelTestKindText
	}
}

func isChannelModelTestEndpointAllowed(modelType string, endpoint string) bool {
	normalizedEndpoint := model.NormalizeRequestedChannelModelEndpoint(endpoint)
	if normalizedEndpoint == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(modelType)) {
	case model.ProviderModelTypeImage:
		return normalizedEndpoint == model.ChannelModelEndpointResponses ||
			normalizedEndpoint == model.ChannelModelEndpointBatches ||
			normalizedEndpoint == model.ChannelModelEndpointImageEdit ||
			normalizedEndpoint == model.ChannelModelEndpointImages
	case model.ProviderModelTypeAudio:
		return normalizedEndpoint == model.ChannelModelEndpointAudio ||
			normalizedEndpoint == model.ChannelModelEndpointRealtime
	case model.ProviderModelTypeVideo:
		return normalizedEndpoint == model.ChannelModelEndpointVideos
	case model.ProviderModelTypeEmbedding:
		return normalizedEndpoint == model.ChannelModelEndpointEmbeddings
	default:
		return normalizedEndpoint == model.ChannelModelEndpointChat ||
			normalizedEndpoint == model.ChannelModelEndpointMessages ||
			normalizedEndpoint == model.ChannelModelEndpointResponses
	}
}

func resolveChannelModelTestEndpoint(modelType string, endpoint string) (string, error) {
	raw := strings.TrimSpace(endpoint)
	if raw == "" {
		return "", fmt.Errorf("模型测试端点未配置，请先为模型明确选择端点")
	}
	normalized := model.NormalizeRequestedChannelModelEndpoint(raw)
	if normalized == "" {
		return "", fmt.Errorf("模型测试端点无效: %s", raw)
	}
	if !isChannelModelTestEndpointAllowed(modelType, normalized) {
		return "", fmt.Errorf("模型类型 %s 不支持测试端点 %s", strings.TrimSpace(modelType), normalized)
	}
	return normalized, nil
}

func resolveChannelModelTestEndpointForRow(row model.ChannelModel) (string, error) {
	modelType := resolveSelectionModelType(row)
	endpoint, err := resolveChannelModelTestEndpoint(modelType, row.Endpoint)
	if err != nil {
		return "", err
	}
	if len(row.Endpoints) == 0 {
		return endpoint, nil
	}
	for _, candidate := range row.Endpoints {
		if model.NormalizeRequestedChannelModelEndpoint(candidate) == endpoint {
			return endpoint, nil
		}
	}
	return "", fmt.Errorf("模型 %s 未声明支持测试端点 %s", strings.TrimSpace(row.Model), endpoint)
}

func runSingleChannelModelTestWithContextAndStream(ctx context.Context, channel *model.Channel, row model.ChannelModel, requestedStream *bool, requestedAudioLanguage string, imageEditInput imageEditTestInput, responsesTestMode string) (model.ChannelTest, channelModelTestExecution) {
	modelType := resolveSelectionModelType(row)
	endpoint, endpointErr := resolveChannelModelTestEndpointForRow(row)
	if endpointErr != nil {
		execution := channelModelTestExecution{
			Err: endpointErr,
			OutputPayload: marshalJSONForLog(map[string]any{
				"error": endpointErr.Error(),
			}),
		}
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      model.NormalizeRequestedChannelModelEndpoint(strings.TrimSpace(row.Endpoint)),
		}, execution), execution
	}

	switch resolveChannelModelTestKind(modelType, endpoint, responsesTestMode) {
	case channelModelTestKindText:
		stream := false
		if requestedStream != nil {
			stream = *requestedStream
		}
		execution := executeChannelTextModelTest(ctx, channel, endpoint, &relaymodel.GeneralOpenAIRequest{
			Model: row.Model,
			Messages: []relaymodel.Message{{
				Role:    "user",
				Content: config.TestPrompt,
			}},
			Stream: stream,
		})
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      endpoint,
		}, execution), execution
	case channelModelTestKindTextResponses:
		stream := false
		if requestedStream != nil {
			stream = *requestedStream
		}
		requestBody := buildResponsesTextModelTestRequestBody(row.Model, stream)
		execution := executeChannelTextModelTestRawBodyWithRetry(
			ctx,
			channel,
			endpoint,
			requestBody,
			row.Model,
			channelModelTestRetryMax,
		)
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      endpoint,
		}, execution), execution
	case channelModelTestKindImageResponses:
		execution := executeChannelImageResponsesModelTest(ctx, channel, row.Model)
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      endpoint,
		}, execution), execution
	case channelModelTestKindImageEdit:
		execution := executeChannelImageEditModelTest(ctx, channel, row.Model, imageEditInput)
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      endpoint,
		}, execution), execution
	case channelModelTestKindBatch:
		execution := channelModelTestExecution{
			Message:       "Batch API 需要先上传 JSONL 文件，暂不自动探测",
			Err:           fmt.Errorf("Batch API 需要先上传 JSONL 文件，暂不自动探测"),
			OutputPayload: marshalJSONForLog(map[string]any{"error": "Batch API 需要先上传 JSONL 文件，暂不自动探测"}),
		}
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      endpoint,
		}, execution), execution
	case channelModelTestKindImage:
		execution := executeChannelImageModelTest(ctx, channel, row.Model)
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      endpoint,
		}, execution), execution
	case channelModelTestKindAudio:
		execution := executeChannelAudioModelTest(ctx, channel, row.Model, requestedAudioLanguage)
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      endpoint,
		}, execution), execution
	case channelModelTestKindRealtime:
		execution := executeChannelRealtimeModelTest(ctx, channel, row.Model)
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      endpoint,
		}, execution), execution
	case channelModelTestKindVideo:
		execution := executeChannelVideoModelTest(ctx, channel, row.Model)
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      model.ChannelModelEndpointVideos,
		}, execution), execution
	case channelModelTestKindEmbedding:
		execution := executeChannelEmbeddingModelTest(ctx, channel, row.Model)
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      model.ChannelModelEndpointEmbeddings,
		}, execution), execution
	default:
		execution := channelModelTestExecution{
			Err:           fmt.Errorf("模型测试端点不支持自动探测: %s", endpoint),
			OutputPayload: marshalJSONForLog(map[string]any{"error": fmt.Sprintf("模型测试端点不支持自动探测: %s", endpoint)}),
		}
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      endpoint,
		}, execution), execution
	}
}

func logChannelModelTestExecution(c *gin.Context, channelID string, result model.ChannelTest, execution channelModelTestExecution) {
	if c == nil {
		return
	}
	fields := []string{
		stringField("channel_id", channelID),
		stringField("model", result.Model),
		stringField("upstream_model", result.UpstreamModel),
		stringField("type", result.Type),
		stringField("endpoint", result.Endpoint),
		stringField("is_stream", strconv.FormatBool(result.IsStream)),
		stringField("status", result.Status),
		int64Field("latency_ms", result.LatencyMs),
		stringField("message", result.Message),
		structuredPayloadField("request_payload", execution.InputPayload),
		structuredPayloadField("response_payload", execution.OutputPayload),
	}
	if result.Supported {
		logChannelAdminInfo(c, "test_model_result", fields...)
		return
	}
	logChannelAdminWarn(c, "test_model_result", fields...)
}

func runChannelModelTests(c *gin.Context, channel *model.Channel, mode string, requestedModel string, requestedModels []string) ([]model.ChannelTest, error) {
	targetRows := resolveChannelTestTargetModels(channel, mode, requestedModel, requestedModels)
	if len(targetRows) == 0 {
		return nil, fmt.Errorf("未找到可用于测试的模型")
	}
	channelID := ""
	if channel != nil {
		channelID = strings.TrimSpace(channel.Id)
	}
	results := make([]model.ChannelTest, 0, len(targetRows))
	for _, row := range targetRows {
		testResult, execution := runSingleChannelModelTest(channel, row)
		if strings.TrimSpace(testResult.ChannelId) == "" {
			testResult.ChannelId = channelID
		}
		persistChannelTestArtifactForExecution(
			context.Background(),
			fmt.Sprintf("manual-%s-%s", sanitizeArtifactFilenamePart(channelID), sanitizeArtifactFilenamePart(testResult.Model)),
			&testResult,
			&execution,
		)
		logChannelModelTestExecution(c, channelID, testResult, execution)
		results = append(results, testResult)
	}
	return model.NormalizeChannelTestRows(results), nil
}

func resolveChannelUpstreamModelName(channel *model.Channel, requestedModel string) string {
	modelName := strings.TrimSpace(requestedModel)
	if channel == nil {
		return modelName
	}
	if modelName == "" {
		selected := channel.SelectedModelIDs()
		if len(selected) > 0 {
			modelName = selected[0]
		}
	}
	if mapped := channel.GetModelMapping()[modelName]; mapped != "" {
		return mapped
	}
	return modelName
}

func newChannelRelayRuntimeContext(path string, channel *model.Channel, requestCtx context.Context) (*gin.Context, *meta.Meta, error) {
	if channel == nil {
		return nil, nil, fmt.Errorf("渠道不能为空")
	}
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	requestURL := &url.URL{Path: path}
	req := &http.Request{
		Method: "POST",
		URL:    requestURL,
		Body:   io.NopCloser(bytes.NewBuffer(nil)),
		Header: make(http.Header),
	}
	c.Request = req.WithContext(requestCtx)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "router-channel-model-tester/1.0")
	middleware.SetupContextForSelectedChannel(c, channel, "")
	return c, meta.GetByContext(c), nil
}

func resolveChannelEndpointURL(baseURL string, path string) string {
	trimmedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	normalizedPath := "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	if trimmedBaseURL == "" {
		return normalizedPath
	}
	lowerBaseURL := strings.ToLower(trimmedBaseURL)
	lowerPath := strings.ToLower(normalizedPath)
	if strings.HasSuffix(lowerBaseURL, "/v1") && strings.HasPrefix(lowerPath, "/v1/") {
		return trimmedBaseURL + normalizedPath[len("/v1"):]
	}
	if strings.HasSuffix(lowerBaseURL, "/openai") && strings.HasPrefix(lowerPath, "/v1/") {
		return trimmedBaseURL + normalizedPath[len("/v1"):]
	}
	if strings.HasSuffix(lowerBaseURL, "/v1beta/openai") && strings.HasPrefix(lowerPath, "/v1/") {
		return trimmedBaseURL + normalizedPath[len("/v1"):]
	}
	return trimmedBaseURL + normalizedPath
}

func parseChannelUpstreamError(statusCode int, body []byte) error {
	type upstreamErrorEnvelope struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
		Message string `json:"message,omitempty"`
	}
	message := ""
	parsed := upstreamErrorEnvelope{}
	if err := json.Unmarshal(body, &parsed); err == nil {
		if parsed.Error != nil {
			message = strings.TrimSpace(parsed.Error.Message)
		}
		if message == "" {
			message = strings.TrimSpace(parsed.Message)
		}
	}
	if message == "" {
		message = strings.TrimSpace(string(body))
	}
	if message == "" {
		return fmt.Errorf("http status code: %d", statusCode)
	}
	return fmt.Errorf("http status code: %d, error message: %s", statusCode, message)
}

func executeChannelTextModelTest(ctx context.Context, channel *model.Channel, path string, request *relaymodel.GeneralOpenAIRequest) channelModelTestExecution {
	execution := channelModelTestExecution{}
	if request == nil {
		execution.Err = fmt.Errorf("请求不能为空")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	c, relayMeta, err := newChannelRelayRuntimeContext(path, channel, ctx)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	adaptor := relay.GetAdaptor(relayMeta.APIType)
	if adaptor == nil {
		execution.Err = fmt.Errorf("invalid api type: %d", relayMeta.APIType)
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	adaptor.Init(relayMeta)
	originModelName := strings.TrimSpace(request.Model)
	actualModelName := resolveChannelUpstreamModelName(channel, originModelName)
	request.Model = actualModelName
	if request.Model == "" {
		execution.Err = fmt.Errorf("未找到可用于测试的模型")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	relayMeta.OriginModelName = originModelName
	relayMeta.ActualModelName = actualModelName
	relayMeta.IsStream = request.Stream
	relayMeta.EndpointPolicies = model.CacheGetChannelModelEndpointPolicies(relayMeta.ChannelId, path, relayMeta.OriginModelName, relayMeta.ActualModelName)
	relayMeta.EndpointPolicy = model.CacheGetChannelModelEndpointPolicy(relayMeta.ChannelId, path, relayMeta.OriginModelName, relayMeta.ActualModelName)
	if err := relaycontroller.ApplyEndpointAccessPolicies(c, relayMeta); err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	execution.IsStream = request.Stream
	if request.Stream {
		c.Request.Header.Set("Accept", "text/event-stream")
	}
	if path == model.ChannelModelEndpointResponses {
		if request.Input == nil && len(request.Messages) > 0 {
			request.Input = request.Messages
			request.Messages = nil
		}
	}
	convertedRequest, err := adaptor.ConvertRequest(c, relayMeta.Mode, request)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	requestBody, err := json.Marshal(convertedRequest)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	requestBody, err = relaycontroller.ApplyEndpointRequestPolicy(c, relayMeta, requestBody)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	baseURL := channel.ResolveAPIBaseURLForModel(path, originModelName, actualModelName)
	requestURL := resolveChannelEndpointURL(baseURL, path)
	if resolvedRequestURL, urlErr := adaptor.GetRequestURL(relayMeta); urlErr == nil && strings.TrimSpace(resolvedRequestURL) != "" {
		requestURL = resolvedRequestURL
	}
	execution.BaseURL = baseURL
	execution.RequestURL = requestURL
	requestHeader := http.Header{}
	requestHeader.Set("Content-Type", "application/json")
	requestHeader.Set("User-Agent", "router-channel-model-tester/1.0")
	if request.Stream {
		requestHeader.Set("Accept", "text/event-stream")
	}
	execution.InputPayload = buildHTTPRequestPayloadForLog(http.MethodPost, requestURL, requestHeader, requestBody)

	startedAt := time.Now()
	resp, err := adaptor.DoRequest(c, relayMeta, bytes.NewBuffer(requestBody))
	execution.LatencyMs = time.Since(startedAt).Milliseconds()
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, nil)
		return execution
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		execution.Err = closeErr
		execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
		return execution
	}
	execution.ResponseStatusCode = resp.StatusCode
	execution.ResponseHeader = resp.Header.Clone()
	execution.ResponseBody = append([]byte(nil), body...)
	execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		execution.Err = parseChannelUpstreamError(resp.StatusCode, body)
		return execution
	}
	message, parseErr := parseTextModelTestResponseByEndpoint(path, string(body))
	if parseErr != nil {
		execution.Err = parseErr
		return execution
	}
	execution.Message = message
	return execution
}

func replaceModelNameInRawTextRequest(body []byte, modelName string) ([]byte, bool, error) {
	trimmedModel := strings.TrimSpace(modelName)
	if len(body) == 0 {
		return nil, false, fmt.Errorf("请求不能为空")
	}
	if trimmedModel == "" {
		return body, false, nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false, fmt.Errorf("解析模板请求失败: %w", err)
	}
	payload["model"] = trimmedModel
	updatedBody, err := json.Marshal(payload)
	if err != nil {
		return nil, false, fmt.Errorf("序列化模板请求失败: %w", err)
	}
	stream := false
	if rawStream, exists := payload["stream"]; exists {
		if parsed, ok := rawStream.(bool); ok {
			stream = parsed
		}
	}
	return updatedBody, stream, nil
}

func executeChannelTextModelTestRawBody(ctx context.Context, channel *model.Channel, path string, requestBody []byte, requestedModel string) channelModelTestExecution {
	execution := channelModelTestExecution{}
	if len(requestBody) == 0 {
		execution.Err = fmt.Errorf("请求不能为空")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	c, relayMeta, err := newChannelRelayRuntimeContext(path, channel, ctx)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	adaptor := relay.GetAdaptor(relayMeta.APIType)
	if adaptor == nil {
		execution.Err = fmt.Errorf("invalid api type: %d", relayMeta.APIType)
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	adaptor.Init(relayMeta)
	actualModel := resolveChannelUpstreamModelName(channel, requestedModel)
	if strings.TrimSpace(actualModel) == "" {
		execution.Err = fmt.Errorf("未找到可用于测试的模型")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	updatedBody, stream, err := replaceModelNameInRawTextRequest(requestBody, actualModel)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	execution.IsStream = stream
	relayMeta.OriginModelName = strings.TrimSpace(requestedModel)
	relayMeta.ActualModelName = strings.TrimSpace(actualModel)
	relayMeta.IsStream = stream
	relayMeta.EndpointPolicies = model.CacheGetChannelModelEndpointPolicies(relayMeta.ChannelId, path, relayMeta.OriginModelName, relayMeta.ActualModelName)
	relayMeta.EndpointPolicy = model.CacheGetChannelModelEndpointPolicy(relayMeta.ChannelId, path, relayMeta.OriginModelName, relayMeta.ActualModelName)
	if err := relaycontroller.ApplyEndpointAccessPolicies(c, relayMeta); err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	if stream {
		c.Request.Header.Set("Accept", "text/event-stream")
	}
	updatedBody, err = relaycontroller.ApplyEndpointRequestPolicy(c, relayMeta, updatedBody)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	baseURL := channel.ResolveAPIBaseURLForModel(path, requestedModel, actualModel)
	requestURL := resolveChannelEndpointURL(baseURL, path)
	if resolvedRequestURL, urlErr := adaptor.GetRequestURL(relayMeta); urlErr == nil && strings.TrimSpace(resolvedRequestURL) != "" {
		requestURL = resolvedRequestURL
	}
	execution.BaseURL = baseURL
	execution.RequestURL = requestURL
	requestHeader := http.Header{}
	requestHeader.Set("Content-Type", "application/json")
	requestHeader.Set("User-Agent", "router-channel-model-tester/1.0")
	if stream {
		requestHeader.Set("Accept", "text/event-stream")
	}
	execution.InputPayload = buildHTTPRequestPayloadForLog(http.MethodPost, requestURL, requestHeader, updatedBody)

	startedAt := time.Now()
	resp, err := adaptor.DoRequest(c, relayMeta, bytes.NewBuffer(updatedBody))
	execution.LatencyMs = time.Since(startedAt).Milliseconds()
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, nil)
		return execution
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		execution.Err = closeErr
		execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
		return execution
	}
	execution.ResponseStatusCode = resp.StatusCode
	execution.ResponseHeader = resp.Header.Clone()
	execution.ResponseBody = append([]byte(nil), body...)
	execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		execution.Err = parseChannelUpstreamError(resp.StatusCode, body)
		return execution
	}
	message, parseErr := parseTextModelTestResponseByEndpoint(path, string(body))
	if parseErr != nil {
		execution.Err = parseErr
		return execution
	}
	execution.Message = message
	return execution
}

func executeChannelTextModelTestRawBodyWithRetry(ctx context.Context, channel *model.Channel, path string, requestBody []byte, requestedModel string, maxAttempts int) channelModelTestExecution {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var last channelModelTestExecution
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		last = executeChannelTextModelTestRawBody(ctx, channel, path, requestBody, requestedModel)
		if last.Err == nil || !shouldRetryChannelTextModelTest(last) || attempt == maxAttempts {
			return last
		}
		time.Sleep(time.Duration(attempt) * 250 * time.Millisecond)
	}
	return last
}

func shouldRetryChannelTextModelTest(execution channelModelTestExecution) bool {
	if execution.Err == nil {
		return false
	}
	switch execution.ResponseStatusCode {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	message := strings.ToLower(strings.TrimSpace(execution.Err.Error()))
	if message == "" {
		return false
	}
	keywords := []string{
		"http status code: 429",
		"http status code: 500",
		"http status code: 502",
		"http status code: 503",
		"http status code: 504",
		"temporarily unavailable",
		"timeout",
		"i/o timeout",
		"connection reset",
		"暂时不可用",
		"稍后重试",
	}
	for _, keyword := range keywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}

func buildResponsesTextModelTestRequest(modelName string, stream bool) *relaymodel.GeneralOpenAIRequest {
	return &relaymodel.GeneralOpenAIRequest{
		Model: modelName,
		Input: []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type": "input_text",
						"text": config.TestPrompt,
					},
				},
			},
		},
		Stream: stream,
	}
}

func buildResponsesTextModelTestRequestBody(modelName string, stream bool) []byte {
	request := buildResponsesTextModelTestRequest(modelName, stream)
	body, err := json.Marshal(request)
	if err != nil {
		return nil
	}
	return body
}

func parseTextModelTestResponseByEndpoint(path string, resp string) (string, error) {
	endpoint := strings.TrimSpace(path)
	switch endpoint {
	case model.ChannelModelEndpointResponses:
		responsesText, responsesErr := parseResponsesModelTestResponse(resp)
		if responsesErr == nil {
			return responsesText, nil
		}
		if isLikelySSEPayload(resp) {
			streamText, streamErr := parseTextModelTestStreamResponse(resp)
			if streamErr == nil {
				return streamText, nil
			}
			return "", fmt.Errorf("parse as responses failed: %v; parse as stream failed: %v", responsesErr, streamErr)
		}
		return "", fmt.Errorf("parse as responses failed: %v", responsesErr)
	case model.ChannelModelEndpointMessages:
		messagesText, messagesErr := parseMessagesModelTestResponse(resp)
		if messagesErr == nil {
			return messagesText, nil
		}
		if isLikelySSEPayload(resp) {
			streamText, streamErr := parseTextModelTestStreamResponse(resp)
			if streamErr == nil {
				return streamText, nil
			}
			return "", fmt.Errorf("parse as messages failed: %v; parse as stream failed: %v", messagesErr, streamErr)
		}
		return "", fmt.Errorf("parse as messages failed: %v", messagesErr)
	case model.ChannelModelEndpointChat:
		_, chatText, chatErr := parseChatModelTestResponse(resp)
		if chatErr == nil {
			return chatText, nil
		}
		if isLikelySSEPayload(resp) {
			streamText, streamErr := parseTextModelTestStreamResponse(resp)
			if streamErr == nil {
				return streamText, nil
			}
			return "", fmt.Errorf("parse as chat failed: %v; parse as stream failed: %v", chatErr, streamErr)
		}
		return "", fmt.Errorf("parse as chat failed: %v", chatErr)
	default:
		return parseTextModelTestResponse(resp)
	}
}

func buildEmbeddingModelTestRequestBody(modelName string) []byte {
	body, err := json.Marshal(map[string]any{
		"model": strings.TrimSpace(modelName),
		"input": "Router embedding test.",
	})
	if err != nil {
		return nil
	}
	return body
}

func parseEmbeddingModelTestResponse(resp string) (string, error) {
	payload := make(map[string]any)
	if err := json.Unmarshal([]byte(resp), &payload); err != nil {
		return "", fmt.Errorf("parse embedding response: %w", err)
	}
	data, ok := payload["data"].([]any)
	if !ok || len(data) == 0 {
		return "", fmt.Errorf("embedding response missing data")
	}
	first, ok := data[0].(map[string]any)
	if !ok {
		return "", fmt.Errorf("embedding response data[0] is not object")
	}
	embedding, ok := first["embedding"].([]any)
	if !ok || len(embedding) == 0 {
		return "", fmt.Errorf("embedding response missing embedding vector")
	}
	return fmt.Sprintf("embedding dimensions: %d", len(embedding)), nil
}

func executeChannelEmbeddingModelTest(ctx context.Context, channel *model.Channel, modelName string) channelModelTestExecution {
	execution := channelModelTestExecution{}
	actualModel := resolveChannelUpstreamModelName(channel, modelName)
	if strings.TrimSpace(actualModel) == "" {
		execution.Err = fmt.Errorf("未找到可用于向量模型测试的模型")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	requestBody := buildEmbeddingModelTestRequestBody(actualModel)
	if len(requestBody) == 0 {
		execution.Err = fmt.Errorf("构建向量模型测试请求失败")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	c, relayMeta, err := newChannelRelayRuntimeContext(model.ChannelModelEndpointEmbeddings, channel, ctx)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	adaptor := relay.GetAdaptor(relayMeta.APIType)
	if adaptor == nil {
		execution.Err = fmt.Errorf("invalid api type: %d", relayMeta.APIType)
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	adaptor.Init(relayMeta)
	relayMeta.OriginModelName = strings.TrimSpace(modelName)
	relayMeta.ActualModelName = strings.TrimSpace(actualModel)
	baseURL := channel.ResolveAPIBaseURLForModel(model.ChannelModelEndpointEmbeddings, modelName, actualModel)
	requestURL := resolveChannelModelTestRequestURL(baseURL, model.ChannelModelEndpointEmbeddings, adaptor, relayMeta)
	execution.BaseURL = baseURL
	execution.RequestURL = requestURL
	requestHeader := http.Header{}
	requestHeader.Set("Content-Type", "application/json")
	execution.InputPayload = buildHTTPRequestPayloadForLog(http.MethodPost, requestURL, requestHeader, requestBody)
	startedAt := time.Now()
	resp, err := adaptor.DoRequest(c, relayMeta, bytes.NewBuffer(requestBody))
	execution.LatencyMs = time.Since(startedAt).Milliseconds()
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, nil)
		return execution
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		execution.Err = closeErr
		execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
		return execution
	}
	execution.ResponseStatusCode = resp.StatusCode
	execution.ResponseHeader = resp.Header.Clone()
	execution.ResponseBody = append([]byte(nil), body...)
	execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		execution.Err = parseChannelUpstreamError(resp.StatusCode, body)
		return execution
	}
	message, parseErr := parseEmbeddingModelTestResponse(string(body))
	if parseErr != nil {
		execution.Err = parseErr
		return execution
	}
	execution.Message = message
	return execution
}

func executeChannelImageResponsesModelTest(ctx context.Context, channel *model.Channel, modelName string) channelModelTestExecution {
	execution := channelModelTestExecution{}
	request := map[string]any{
		"model": resolveChannelUpstreamModelName(channel, modelName),
		"input": "Generate a simple blue square on a white background.",
		"tools": []map[string]any{
			{
				"type": "image_generation",
			},
		},
	}
	if strings.TrimSpace(request["model"].(string)) == "" {
		execution.Err = fmt.Errorf("未找到可用于图片模型测试的模型")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	c, relayMeta, err := newChannelRelayRuntimeContext(model.ChannelModelEndpointResponses, channel, ctx)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	adaptor := relay.GetAdaptor(relayMeta.APIType)
	if adaptor == nil {
		execution.Err = fmt.Errorf("invalid api type: %d", relayMeta.APIType)
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	adaptor.Init(relayMeta)
	relayMeta.OriginModelName = strings.TrimSpace(modelName)
	relayMeta.ActualModelName = request["model"].(string)
	actualModelName := relayMeta.ActualModelName
	requestBody, err := json.Marshal(request)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	baseURL := channel.ResolveAPIBaseURLForModel(model.ChannelModelEndpointResponses, modelName, actualModelName)
	requestURL := resolveChannelModelTestRequestURL(baseURL, model.ChannelModelEndpointResponses, adaptor, relayMeta)
	execution.BaseURL = baseURL
	execution.RequestURL = requestURL
	requestHeader := http.Header{}
	requestHeader.Set("Content-Type", "application/json")
	execution.InputPayload = buildHTTPRequestPayloadForLog(http.MethodPost, requestURL, requestHeader, requestBody)
	startedAt := time.Now()
	resp, err := adaptor.DoRequest(c, relayMeta, bytes.NewBuffer(requestBody))
	execution.LatencyMs = time.Since(startedAt).Milliseconds()
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, nil)
		return execution
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		execution.Err = closeErr
		execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
		return execution
	}
	execution.ResponseStatusCode = resp.StatusCode
	execution.ResponseHeader = resp.Header.Clone()
	execution.ResponseBody = append([]byte(nil), body...)
	execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		execution.Err = parseChannelUpstreamError(resp.StatusCode, body)
		return execution
	}
	message, parseErr := parseResponsesImageTestResponse(string(body))
	if parseErr != nil {
		execution.Err = parseErr
		return execution
	}
	execution.Message = message
	return execution
}

func executeChannelImageModelTest(ctx context.Context, channel *model.Channel, modelName string) channelModelTestExecution {
	execution := channelModelTestExecution{}
	c, relayMeta, err := newChannelRelayRuntimeContext("/v1/images/generations", channel, ctx)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	adaptor := relay.GetAdaptor(relayMeta.APIType)
	if adaptor == nil {
		execution.Err = fmt.Errorf("invalid api type: %d", relayMeta.APIType)
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	adaptor.Init(relayMeta)
	actualModelName := resolveChannelUpstreamModelName(channel, modelName)
	if actualModelName == "" {
		execution.Err = fmt.Errorf("未找到可用于图片模型测试的模型")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	relayMeta.OriginModelName = strings.TrimSpace(modelName)
	relayMeta.ActualModelName = actualModelName
	imageRequest := &relaymodel.ImageRequest{
		Model:  actualModelName,
		Prompt: "A blue square on a white background.",
		N:      1,
		Size:   resolveChannelImageModelTestSize(channel, actualModelName),
	}
	convertedRequest, err := adaptor.ConvertImageRequest(imageRequest)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	requestBody, err := json.Marshal(convertedRequest)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	baseURL := channel.ResolveAPIBaseURLForModel("/v1/images/generations", modelName, actualModelName)
	requestURL := resolveChannelModelTestRequestURL(baseURL, "/v1/images/generations", adaptor, relayMeta)
	execution.BaseURL = baseURL
	execution.RequestURL = requestURL
	requestHeader := http.Header{}
	requestHeader.Set("Content-Type", "application/json")
	execution.InputPayload = buildHTTPRequestPayloadForLog(http.MethodPost, requestURL, requestHeader, requestBody)
	startedAt := time.Now()
	resp, err := adaptor.DoRequest(c, relayMeta, bytes.NewBuffer(requestBody))
	execution.LatencyMs = time.Since(startedAt).Milliseconds()
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, nil)
		return execution
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		execution.Err = closeErr
		execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
		return execution
	}
	execution.ResponseStatusCode = resp.StatusCode
	execution.ResponseHeader = resp.Header.Clone()
	execution.ResponseBody = append([]byte(nil), body...)
	execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		execution.Err = parseChannelUpstreamError(resp.StatusCode, body)
		return execution
	}
	preview := "图片接口返回成功"
	imageResponse := openaiadaptor.ImageResponse{}
	if err := json.Unmarshal(body, &imageResponse); err == nil && len(imageResponse.Data) > 0 {
		preview = fmt.Sprintf("返回 %d 个图片结果", len(imageResponse.Data))
	}
	execution.Message = preview
	return execution
}

func resolveChannelImageModelTestSize(channel *model.Channel, actualModelName string) string {
	if channel != nil &&
		channel.GetChannelProtocol() == relaychannel.VolcEngine &&
		strings.Contains(strings.ToLower(strings.TrimSpace(actualModelName)), "seedream") {
		return "2048x2048"
	}
	return "1024x1024"
}

func resolveChannelImageEditTestImage(ctx context.Context, input imageEditTestInput) ([]byte, string, error) {
	dataURI := strings.TrimSpace(input.DataURI)
	if dataURI != "" {
		comma := strings.Index(dataURI, ",")
		if comma < 0 {
			return nil, "", fmt.Errorf("图片测试上传数据无效")
		}
		header := strings.ToLower(strings.TrimSpace(dataURI[:comma]))
		payload := strings.TrimSpace(dataURI[comma+1:])
		if !strings.Contains(header, ";base64") {
			return nil, "", fmt.Errorf("图片测试上传数据必须是 base64 data URL")
		}
		imageBytes, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, "", err
		}
		return imageBytes, "uploaded-image.png", nil
	}

	imageURL := strings.TrimSpace(input.URL)
	if imageURL == "" {
		imageURL = defaultChannelImageEditTestURL
	}
	parsedURL, err := url.Parse(imageURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, "", fmt.Errorf("图片测试原图地址无效")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("图片测试原图下载失败: http status %d", resp.StatusCode)
	}
	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	filename := strings.TrimSpace(parsedURL.Path)
	if idx := strings.LastIndex(filename, "/"); idx >= 0 {
		filename = filename[idx+1:]
	}
	if filename == "" {
		filename = "source-image.png"
	}
	return imageBytes, filename, nil
}

func executeChannelImageEditModelTest(ctx context.Context, channel *model.Channel, modelName string, imageEditInput imageEditTestInput) channelModelTestExecution {
	execution := channelModelTestExecution{}
	actualModelName := resolveChannelUpstreamModelName(channel, modelName)
	if actualModelName == "" {
		execution.Err = fmt.Errorf("未找到可用于图片编辑测试的模型")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	imageBytes, imageFilename, err := resolveChannelImageEditTestImage(ctx, imageEditInput)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	bodyBuffer := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuffer)
	if err := writer.WriteField("model", actualModelName); err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	if err := writer.WriteField("prompt", "Replace the image with a simple blue square on a white background."); err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	part, err := writer.CreateFormFile("image", imageFilename)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	if _, err := part.Write(imageBytes); err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	if err := writer.Close(); err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	if channel.GetChannelProtocol() == relaychannel.Ali && aliadaptor.IsQwenImageModel(actualModelName) {
		form, err := multipart.NewReader(bytes.NewReader(bodyBuffer.Bytes()), writer.Boundary()).ReadForm(32 << 20)
		if err != nil {
			execution.Err = err
			execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
			return execution
		}
		defer form.RemoveAll()
		imageRequest := relaymodel.ImageRequest{
			Model:  actualModelName,
			Prompt: "Replace the image with a simple blue square on a white background.",
		}
		convertedRequest, err := aliadaptor.ConvertQwenImageEditRequest(imageRequest, form)
		if err != nil {
			execution.Err = err
			execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
			return execution
		}
		requestBody, err := json.Marshal(convertedRequest)
		if err != nil {
			execution.Err = err
			execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
			return execution
		}
		c, relayMeta, err := newChannelRelayRuntimeContext(model.ChannelModelEndpointImageEdit, channel, ctx)
		if err != nil {
			execution.Err = err
			execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
			return execution
		}
		adaptor := relay.GetAdaptor(relayMeta.APIType)
		if adaptor == nil {
			execution.Err = fmt.Errorf("invalid api type: %d", relayMeta.APIType)
			execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
			return execution
		}
		adaptor.Init(relayMeta)
		relayMeta.OriginModelName = strings.TrimSpace(modelName)
		relayMeta.ActualModelName = actualModelName
		baseURL := channel.ResolveAPIBaseURLForModel(model.ChannelModelEndpointImageEdit, modelName, actualModelName)
		requestURL := resolveChannelModelTestRequestURL(baseURL, model.ChannelModelEndpointImageEdit, adaptor, relayMeta)
		execution.BaseURL = baseURL
		execution.RequestURL = requestURL
		c.Request.Header.Set("Content-Type", "application/json")
		c.Request.Header.Set("Accept", "application/json")
		execution.InputPayload = buildHTTPRequestPayloadForLog(http.MethodPost, requestURL, c.Request.Header, requestBody)
		startedAt := time.Now()
		resp, err := adaptor.DoRequest(c, relayMeta, bytes.NewBuffer(requestBody))
		execution.LatencyMs = time.Since(startedAt).Milliseconds()
		if err != nil {
			execution.Err = err
			execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
			return execution
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			execution.Err = err
			execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, nil)
			return execution
		}
		execution.ResponseStatusCode = resp.StatusCode
		execution.ResponseHeader = resp.Header.Clone()
		execution.ResponseBody = append([]byte(nil), body...)
		execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			execution.Err = parseChannelUpstreamError(resp.StatusCode, body)
			return execution
		}
		preview := "图片编辑接口返回成功"
		imageResponse := openaiadaptor.ImageResponse{}
		if err := json.Unmarshal(body, &imageResponse); err == nil && len(imageResponse.Data) > 0 {
			preview = fmt.Sprintf("返回 %d 个图片结果", len(imageResponse.Data))
		}
		execution.Message = preview
		return execution
	}

	baseURL := channel.ResolveAPIBaseURLForModel(model.ChannelModelEndpointImageEdit, modelName, actualModelName)
	requestURL := resolveChannelEndpointURL(baseURL, model.ChannelModelEndpointImageEdit)
	execution.BaseURL = baseURL
	execution.RequestURL = requestURL
	if ctx == nil {
		ctx = context.Background()
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bodyBuffer)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(channel.Key))
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Accept", "application/json")
	execution.InputPayload = buildHTTPRequestPayloadForLog(httpReq.Method, requestURL, httpReq.Header, bodyBuffer.Bytes())
	startedAt := time.Now()
	resp, err := client.HTTPClient.Do(httpReq)
	execution.LatencyMs = time.Since(startedAt).Milliseconds()
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, nil)
		return execution
	}
	execution.ResponseStatusCode = resp.StatusCode
	execution.ResponseHeader = resp.Header.Clone()
	execution.ResponseBody = append([]byte(nil), body...)
	execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		execution.Err = parseChannelUpstreamError(resp.StatusCode, body)
		return execution
	}
	preview := "图片编辑接口返回成功"
	imageResponse := openaiadaptor.ImageResponse{}
	if err := json.Unmarshal(body, &imageResponse); err == nil && len(imageResponse.Data) > 0 {
		preview = fmt.Sprintf("返回 %d 个图片结果", len(imageResponse.Data))
	}
	execution.Message = preview
	return execution
}

func executeChannelAudioModelTest(ctx context.Context, channel *model.Channel, modelName string, language string) channelModelTestExecution {
	execution := channelModelTestExecution{}
	execution.IsStream = false
	actualModelName := resolveChannelUpstreamModelName(channel, modelName)
	if actualModelName == "" {
		execution.Err = fmt.Errorf("未找到可用于音频模型测试的模型")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	if strings.Contains(strings.ToLower(actualModelName), "whisper") {
		execution.Err = fmt.Errorf("当前音频模型更像转录模型，暂不自动探测")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	c, relayMeta, err := newChannelRelayRuntimeContext("/v1/audio/speech", channel, ctx)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	c.Request.Header.Set("Accept", "audio/mpeg")
	adaptor := relay.GetAdaptor(relayMeta.APIType)
	if adaptor == nil {
		execution.Err = fmt.Errorf("invalid api type: %d", relayMeta.APIType)
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	adaptor.Init(relayMeta)
	relayMeta.OriginModelName = strings.TrimSpace(modelName)
	relayMeta.ActualModelName = actualModelName
	requestBody, err := json.Marshal(map[string]any{
		"model": actualModelName,
		"input": buildAudioModelTestInput(language),
		"voice": "alloy",
	})
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	baseURL := channel.ResolveAPIBaseURLForModel("/v1/audio/speech", modelName, actualModelName)
	requestURL := resolveChannelModelTestRequestURL(baseURL, "/v1/audio/speech", adaptor, relayMeta)
	execution.BaseURL = baseURL
	execution.RequestURL = requestURL
	requestHeader := http.Header{}
	requestHeader.Set("Content-Type", "application/json")
	requestHeader.Set("Accept", "audio/mpeg")
	execution.InputPayload = buildHTTPRequestPayloadForLog(http.MethodPost, requestURL, requestHeader, requestBody)
	startedAt := time.Now()
	resp, err := adaptor.DoRequest(c, relayMeta, bytes.NewBuffer(requestBody))
	execution.LatencyMs = time.Since(startedAt).Milliseconds()
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, nil)
		return execution
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		execution.Err = closeErr
		execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
		return execution
	}
	execution.ResponseStatusCode = resp.StatusCode
	execution.ResponseHeader = resp.Header.Clone()
	execution.ResponseBody = append([]byte(nil), body...)
	execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		execution.Err = parseChannelUpstreamError(resp.StatusCode, body)
		return execution
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "audio payload"
	}
	if len(body) == 0 {
		execution.Err = fmt.Errorf("响应为空")
		return execution
	}
	execution.Message = fmt.Sprintf("返回 %d bytes (%s)", len(body), contentType)
	return execution
}

func normalizeChannelTestRealtimeWebSocketURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "https":
		parsed.Scheme = "wss"
	case "http":
		parsed.Scheme = "ws"
	case "wss", "ws":
	default:
		return "", fmt.Errorf("unsupported upstream scheme: %s", parsed.Scheme)
	}
	return parsed.String(), nil
}

type realtimeServerEventError struct {
	Type    string `json:"type"`
	EventID string `json:"event_id,omitempty"`
	Error   struct {
		Type    string `json:"type,omitempty"`
		Code    any    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"error,omitempty"`
	Response *struct {
		Status string `json:"status,omitempty"`
	} `json:"response,omitempty"`
}

func wrapRealtimeHandshakeError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("WebSocket 握手失败: %w", err)
}

func wrapRealtimeSessionError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("WebSocket 握手成功，但会话失败: %w", err)
}

func buildRealtimeSessionSuccessMessage(subprotocol string, responseText string) string {
	base := "WebSocket 会话成功"
	if strings.TrimSpace(subprotocol) != "" {
		base = fmt.Sprintf("%s（subprotocol=%s）", base, strings.TrimSpace(subprotocol))
	}
	trimmedResponseText := strings.TrimSpace(responseText)
	if trimmedResponseText == "" {
		return base + "，未返回文本"
	}
	return fmt.Sprintf("%s，返回文本：%s", base, trimmedResponseText)
}

func writeRealtimeTestEvent(conn *websocket.Conn, payload map[string]any) error {
	if conn == nil {
		return fmt.Errorf("realtime connection is nil")
	}
	return conn.WriteJSON(payload)
}

func waitRealtimeTestCompletion(conn *websocket.Conn) (string, error) {
	if conn == nil {
		return "", fmt.Errorf("realtime connection is nil")
	}
	deadline := time.Now().Add(15 * time.Second)
	_ = conn.SetReadDeadline(deadline)
	textParts := make([]string, 0, 4)
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return strings.TrimSpace(strings.Join(textParts, "")), err
		}
		event := map[string]any{}
		if err := json.Unmarshal(message, &event); err != nil {
			continue
		}
		eventType := strings.TrimSpace(fmt.Sprintf("%v", event["type"]))
		switch eventType {
		case "response.output_text.delta":
			if delta := strings.TrimSpace(fmt.Sprintf("%v", event["delta"])); delta != "" && delta != "<nil>" {
				textParts = append(textParts, delta)
			}
		case "response.text.delta":
			if delta := strings.TrimSpace(fmt.Sprintf("%v", event["delta"])); delta != "" && delta != "<nil>" {
				textParts = append(textParts, delta)
			}
		case "response.done":
			rawResponse, ok := event["response"].(map[string]any)
			if ok {
				status := strings.TrimSpace(fmt.Sprintf("%v", rawResponse["status"]))
				if status != "" && status != "completed" {
					return strings.TrimSpace(strings.Join(textParts, "")), fmt.Errorf("realtime response finished with status %s", status)
				}
			}
			return strings.TrimSpace(strings.Join(textParts, "")), nil
		case "error":
			realtimeErr := realtimeServerEventError{}
			if err := json.Unmarshal(message, &realtimeErr); err == nil {
				msg := strings.TrimSpace(realtimeErr.Error.Message)
				if msg == "" {
					msg = strings.TrimSpace(string(message))
				}
				return strings.TrimSpace(strings.Join(textParts, "")), fmt.Errorf("realtime server error: %s", msg)
			}
			return strings.TrimSpace(strings.Join(textParts, "")), fmt.Errorf("realtime server error: %s", strings.TrimSpace(string(message)))
		}
	}
}

func waitRealtimeSessionUpdated(conn *websocket.Conn) error {
	if conn == nil {
		return fmt.Errorf("realtime connection is nil")
	}
	deadline := time.Now().Add(15 * time.Second)
	_ = conn.SetReadDeadline(deadline)
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		event := map[string]any{}
		if err := json.Unmarshal(message, &event); err != nil {
			continue
		}
		eventType := strings.TrimSpace(fmt.Sprintf("%v", event["type"]))
		switch eventType {
		case "session.updated":
			return nil
		case "error":
			realtimeErr := realtimeServerEventError{}
			if err := json.Unmarshal(message, &realtimeErr); err == nil {
				msg := strings.TrimSpace(realtimeErr.Error.Message)
				if msg == "" {
					msg = strings.TrimSpace(string(message))
				}
				return fmt.Errorf("realtime server error: %s", msg)
			}
			return fmt.Errorf("realtime server error: %s", strings.TrimSpace(string(message)))
		}
	}
}

func waitRealtimeSessionCreated(conn *websocket.Conn) error {
	if conn == nil {
		return fmt.Errorf("realtime connection is nil")
	}
	deadline := time.Now().Add(15 * time.Second)
	_ = conn.SetReadDeadline(deadline)
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		event := map[string]any{}
		if err := json.Unmarshal(message, &event); err != nil {
			continue
		}
		eventType := strings.TrimSpace(fmt.Sprintf("%v", event["type"]))
		switch eventType {
		case "session.created":
			return nil
		case "error":
			realtimeErr := realtimeServerEventError{}
			if err := json.Unmarshal(message, &realtimeErr); err == nil {
				msg := strings.TrimSpace(realtimeErr.Error.Message)
				if msg == "" {
					msg = strings.TrimSpace(string(message))
				}
				return fmt.Errorf("realtime server error: %s", msg)
			}
			return fmt.Errorf("realtime server error: %s", strings.TrimSpace(string(message)))
		}
	}
}

func isQwenLiveTranslateRealtimeModel(modelName string) bool {
	normalized := strings.TrimSpace(strings.ToLower(modelName))
	return strings.Contains(normalized, "livetranslate") && strings.HasSuffix(normalized, "-realtime")
}

func buildZhipuRealtimeSessionUpdate(modelName string) map[string]any {
	return map[string]any{
		"event_id":         fmt.Sprintf("router-test-%d", time.Now().UnixMilli()),
		"client_timestamp": time.Now().UnixMilli(),
		"type":             "session.update",
		"session": map[string]any{
			"model":                       strings.TrimSpace(modelName),
			"modalities":                  []string{"audio", "text"},
			"instructions":                "请简短回答。",
			"voice":                       "tongtong",
			"input_audio_format":          "wav",
			"output_audio_format":         "pcm",
			"input_audio_noise_reduction": map[string]any{"type": "far_field"},
			"temperature":                 0.7,
			"max_response_output_tokens":  "inf",
			"beta_fields": map[string]any{
				"chat_mode":  "audio",
				"tts_source": "e2e",
			},
		},
	}
}

func executeChannelRealtimeModelTest(ctx context.Context, channel *model.Channel, modelName string) channelModelTestExecution {
	execution := channelModelTestExecution{}
	actualModelName := resolveChannelUpstreamModelName(channel, modelName)
	if actualModelName == "" {
		execution.Err = fmt.Errorf("未找到可用于实时模型测试的模型")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	_, relayMeta, err := newChannelRelayRuntimeContext(model.ChannelModelEndpointRealtime, channel, ctx)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	adaptor := relay.GetAdaptor(relayMeta.APIType)
	if adaptor == nil {
		execution.Err = fmt.Errorf("invalid api type: %d", relayMeta.APIType)
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	adaptor.Init(relayMeta)
	relayMeta.OriginModelName = strings.TrimSpace(modelName)
	relayMeta.ActualModelName = actualModelName

	baseURL := channel.ResolveAPIBaseURLForModel(model.ChannelModelEndpointRealtime, modelName, actualModelName)
	requestURL := resolveChannelEndpointURL(baseURL, model.ChannelModelEndpointRealtime)
	if resolvedRequestURL, urlErr := adaptor.GetRequestURL(relayMeta); urlErr == nil && strings.TrimSpace(resolvedRequestURL) != "" {
		requestURL = resolvedRequestURL
	}
	execution.BaseURL = baseURL
	execution.RequestURL = requestURL
	if requestURL == "" {
		execution.Err = fmt.Errorf("未找到 realtime 测试地址")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	if !relaychannel.IsVolcengineRealtimeRequest(
		relayMeta.ChannelProtocol,
		relayMeta.UpstreamRequestPath,
		relayMeta.RequestURLPath,
	) {
		query := parsedURL.Query()
		query.Set("model", actualModelName)
		parsedURL.RawQuery = query.Encode()
	}

	upstreamURL, err := normalizeChannelTestRealtimeWebSocketURL(parsedURL.String())
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}

	requestHeader := http.Header{}
	switch {
	case relayMeta.ChannelProtocol == relaychannel.Azure:
		requestHeader.Set("OpenAI-Beta", "realtime=v1")
		requestHeader.Set("api-key", strings.TrimSpace(channel.Key))
	case relaychannel.IsVolcengineRealtimeRequest(
		relayMeta.ChannelProtocol,
		relayMeta.UpstreamRequestPath,
		relayMeta.RequestURLPath,
	):
		volcenginerealtime.ApplyRealtimeHeaders(
			requestHeader,
			relayMeta.Config.AppID,
			strings.TrimSpace(channel.Key),
			relayMeta.Config.ResourceID,
		)
	case relayMeta.ChannelProtocol == relaychannel.Zhipu:
		requestHeader.Set("Authorization", "Bearer "+strings.TrimSpace(channel.Key))
	default:
		requestHeader.Set("OpenAI-Beta", "realtime=v1")
		requestHeader.Set("Authorization", "Bearer "+strings.TrimSpace(channel.Key))
	}
	execution.InputPayload = buildHTTPRequestPayloadForLog(http.MethodGet, parsedURL.String(), requestHeader, nil)

	dialer := websocket.Dialer{
		Subprotocols: []string{"realtime"},
	}
	if relayMeta.ChannelProtocol == relaychannel.Zhipu {
		dialer.Subprotocols = nil
	}
	startedAt := time.Now()
	conn, resp, err := dialer.DialContext(ctx, upstreamURL, requestHeader)
	execution.LatencyMs = time.Since(startedAt).Milliseconds()
	if resp != nil {
		execution.ResponseStatusCode = resp.StatusCode
		execution.ResponseHeader = resp.Header.Clone()
	}
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			if resp.Body != nil {
				_ = resp.Body.Close()
			}
			execution.ResponseBody = append([]byte(nil), body...)
			execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
			execution.Err = wrapRealtimeHandshakeError(parseChannelUpstreamError(resp.StatusCode, body))
			return execution
		}
		execution.Err = wrapRealtimeHandshakeError(err)
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	subprotocol := strings.TrimSpace(conn.Subprotocol())
	if relaychannel.IsVolcengineRealtimeRequest(
		relayMeta.ChannelProtocol,
		relayMeta.UpstreamRequestPath,
		relayMeta.RequestURLPath,
	) {
		execution.Message = "WebSocket 握手成功，Volcengine Realtime 未执行 OpenAI 风格会话测试"
		outputMessage := execution.Message
		if subprotocol != "" {
			outputMessage = fmt.Sprintf("%s（subprotocol=%s）", execution.Message, subprotocol)
		}
		execution.OutputPayload = buildHTTPResponsePayloadForLog(http.StatusSwitchingProtocols, execution.ResponseHeader, []byte(outputMessage))
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "channel test complete"), time.Now().Add(2*time.Second))
		_ = conn.Close()
		return execution
	}
	if relayMeta.ChannelProtocol == relaychannel.Zhipu {
		if err := waitRealtimeSessionCreated(conn); err != nil {
			execution.Err = wrapRealtimeSessionError(err)
			execution.OutputPayload = buildHTTPResponsePayloadForLog(http.StatusSwitchingProtocols, execution.ResponseHeader, []byte(execution.Err.Error()))
			_ = conn.Close()
			return execution
		}
		if err := writeRealtimeTestEvent(conn, buildZhipuRealtimeSessionUpdate(actualModelName)); err != nil {
			execution.Err = err
			execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
			_ = conn.Close()
			return execution
		}
		if err := waitRealtimeSessionUpdated(conn); err != nil {
			execution.Err = wrapRealtimeSessionError(err)
			execution.OutputPayload = buildHTTPResponsePayloadForLog(http.StatusSwitchingProtocols, execution.ResponseHeader, []byte(execution.Err.Error()))
			_ = conn.Close()
			return execution
		}
		execution.Message = "WebSocket 会话成功，Zhipu Realtime 会话参数校验通过"
		outputMessage := execution.Message
		if subprotocol != "" {
			outputMessage = fmt.Sprintf("%s（subprotocol=%s）", execution.Message, subprotocol)
		}
		execution.OutputPayload = buildHTTPResponsePayloadForLog(http.StatusSwitchingProtocols, execution.ResponseHeader, []byte(outputMessage))
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "channel test complete"), time.Now().Add(2*time.Second))
		_ = conn.Close()
		return execution
	}
	if isQwenLiveTranslateRealtimeModel(actualModelName) {
		if err := writeRealtimeTestEvent(conn, map[string]any{
			"type": "session.update",
			"session": map[string]any{
				"modalities":         []string{"text"},
				"input_audio_format": "pcm",
				"sample_rate":        16000,
				"translation": map[string]any{
					"language": "en",
				},
			},
		}); err != nil {
			execution.Err = err
			execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
			_ = conn.Close()
			return execution
		}
		if err := waitRealtimeSessionUpdated(conn); err != nil {
			execution.Err = wrapRealtimeSessionError(err)
			execution.OutputPayload = buildHTTPResponsePayloadForLog(http.StatusSwitchingProtocols, execution.ResponseHeader, []byte(execution.Err.Error()))
			_ = conn.Close()
			return execution
		}
		execution.Message = "WebSocket 会话成功，实时翻译模型会话参数校验通过"
		outputMessage := execution.Message
		if subprotocol != "" {
			outputMessage = fmt.Sprintf("%s（subprotocol=%s）", execution.Message, subprotocol)
		}
		execution.OutputPayload = buildHTTPResponsePayloadForLog(http.StatusSwitchingProtocols, execution.ResponseHeader, []byte(outputMessage))
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "channel test complete"), time.Now().Add(2*time.Second))
		_ = conn.Close()
		return execution
	}
	if err := writeRealtimeTestEvent(conn, map[string]any{
		"type": "session.update",
		"session": map[string]any{
			"type":         "realtime",
			"instructions": "Reply with exactly OK.",
		},
	}); err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		_ = conn.Close()
		return execution
	}
	if err := writeRealtimeTestEvent(conn, map[string]any{
		"type": "conversation.item.create",
		"item": map[string]any{
			"type": "message",
			"role": "user",
			"content": []map[string]any{
				{
					"type": "input_text",
					"text": "Reply with exactly OK.",
				},
			},
		},
	}); err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		_ = conn.Close()
		return execution
	}
	if err := writeRealtimeTestEvent(conn, map[string]any{
		"type": "response.create",
		"response": map[string]any{
			"modalities":   []string{"text"},
			"instructions": "Reply with exactly OK.",
		},
	}); err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		_ = conn.Close()
		return execution
	}
	responseText, waitErr := waitRealtimeTestCompletion(conn)
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "channel test complete"), time.Now().Add(2*time.Second))
	_ = conn.Close()
	if waitErr != nil {
		execution.Err = wrapRealtimeSessionError(waitErr)
		execution.OutputPayload = buildHTTPResponsePayloadForLog(http.StatusSwitchingProtocols, execution.ResponseHeader, []byte(execution.Err.Error()))
		return execution
	}
	outputMessage := buildRealtimeSessionSuccessMessage(subprotocol, responseText)
	execution.OutputPayload = buildHTTPResponsePayloadForLog(http.StatusSwitchingProtocols, execution.ResponseHeader, []byte(outputMessage))
	execution.Message = outputMessage
	return execution
}

func executeChannelVideoModelTest(ctx context.Context, channel *model.Channel, modelName string) channelModelTestExecution {
	execution := channelModelTestExecution{}
	actualModelName := resolveChannelUpstreamModelName(channel, modelName)
	if actualModelName == "" {
		execution.Err = fmt.Errorf("未找到可用于视频模型测试的模型")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	if channel == nil {
		execution.Err = fmt.Errorf("渠道不能为空")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	baseURL := channel.ResolveAPIBaseURLForModel("/v1/videos", modelName, actualModelName)
	execution.BaseURL = baseURL
	if strings.TrimSpace(baseURL) == "" {
		execution.Err = fmt.Errorf("未找到可用于视频模型测试的 Base URL")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}

	bodyBuffer := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuffer)
	if err := writer.WriteField("model", actualModelName); err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	if err := writer.WriteField("prompt", "A short blue sphere morphing into a cube."); err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	if err := writer.Close(); err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}

	requestURL := resolveChannelEndpointURL(baseURL, "/v1/videos")
	execution.RequestURL = requestURL
	if ctx == nil {
		ctx = context.Background()
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bodyBuffer)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(channel.Key))
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Accept", "application/json")
	execution.InputPayload = buildHTTPRequestPayloadForLog(httpReq.Method, requestURL, httpReq.Header, bodyBuffer.Bytes())

	startedAt := time.Now()
	resp, err := client.HTTPClient.Do(httpReq)
	execution.LatencyMs = time.Since(startedAt).Milliseconds()
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, nil)
		return execution
	}
	execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		execution.Err = parseChannelUpstreamError(resp.StatusCode, body)
		return execution
	}

	type channelVideoResponse struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	parsed := channelVideoResponse{}
	if err := json.Unmarshal(body, &parsed); err == nil {
		if strings.TrimSpace(parsed.ID) != "" && strings.TrimSpace(parsed.Status) != "" {
			execution.Message = fmt.Sprintf("返回任务 %s（%s）", strings.TrimSpace(parsed.ID), strings.TrimSpace(parsed.Status))
			return execution
		}
		if strings.TrimSpace(parsed.ID) != "" {
			execution.Message = fmt.Sprintf("返回任务 %s", strings.TrimSpace(parsed.ID))
			return execution
		}
		if strings.TrimSpace(parsed.Status) != "" {
			execution.Message = fmt.Sprintf("视频任务状态：%s", strings.TrimSpace(parsed.Status))
			return execution
		}
	}
	execution.Message = "视频接口返回成功"
	return execution
}
