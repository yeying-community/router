package channel

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
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
	"github.com/yeying-community/router/internal/relay"
	openaiadaptor "github.com/yeying-community/router/internal/relay/adaptor/openai"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/transport/http/middleware"
	"gorm.io/gorm"
)

type channelModelTestTargetItem struct {
	Model    string `json:"model"`
	Endpoint string `json:"endpoint,omitempty"`
	IsStream *bool  `json:"is_stream,omitempty"`
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
	return model.DB.Transaction(func(tx *gorm.DB) error {
		if _, err := model.AppendChannelTestsForModelsWithDB(tx, normalizedChannelID, targetModels, results); err != nil {
			return err
		}
		if err := model.EnsureChannelTestModelWithDB(tx, normalizedChannelID); err != nil {
			return err
		}
		return model.UpsertChannelModelEndpointTestResultsWithDB(tx, normalizedChannelID, taskID, results)
	})
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
	return runSingleChannelModelTestWithContextAndStream(context.Background(), channel, row, nil, "")
}

func runSingleChannelModelTestWithContext(ctx context.Context, channel *model.Channel, row model.ChannelModel) (model.ChannelTest, channelModelTestExecution) {
	return runSingleChannelModelTestWithContextAndStream(ctx, channel, row, nil, "")
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

func runSingleChannelModelTestWithContextAndStream(ctx context.Context, channel *model.Channel, row model.ChannelModel, requestedStream *bool, requestedAudioLanguage string) (model.ChannelTest, channelModelTestExecution) {
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

	switch modelType {
	case model.ProviderModelTypeImage:
		var execution channelModelTestExecution
		switch endpoint {
		case model.ChannelModelEndpointResponses:
			execution = executeChannelImageResponsesModelTest(ctx, channel, row.Model)
		case model.ChannelModelEndpointImageEdit:
			execution = executeChannelImageEditModelTest(ctx, channel, row.Model)
		case model.ChannelModelEndpointBatches:
			execution = channelModelTestExecution{
				Message:       "Batch API 需要先上传 JSONL 文件，暂不自动探测",
				Err:           fmt.Errorf("Batch API 需要先上传 JSONL 文件，暂不自动探测"),
				OutputPayload: marshalJSONForLog(map[string]any{"error": "Batch API 需要先上传 JSONL 文件，暂不自动探测"}),
			}
		default:
			execution = executeChannelImageModelTest(ctx, channel, row.Model)
		}
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      endpoint,
		}, execution), execution
	case model.ProviderModelTypeAudio:
		var execution channelModelTestExecution
		switch endpoint {
		case model.ChannelModelEndpointRealtime:
			execution = executeChannelRealtimeModelTest(ctx, channel, row.Model)
		default:
			execution = executeChannelAudioModelTest(ctx, channel, row.Model, requestedAudioLanguage)
		}
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      endpoint,
		}, execution), execution
	case model.ProviderModelTypeVideo:
		execution := executeChannelVideoModelTest(ctx, channel, row.Model)
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      model.ChannelModelEndpointVideos,
		}, execution), execution
	default:
		if endpoint == model.ChannelModelEndpointChat || endpoint == model.ChannelModelEndpointMessages {
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
		}
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
	request.Model = resolveChannelUpstreamModelName(channel, request.Model)
	if request.Model == "" {
		execution.Err = fmt.Errorf("未找到可用于测试的模型")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	relayMeta.OriginModelName = request.Model
	relayMeta.ActualModelName = request.Model
	relayMeta.IsStream = request.Stream
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
	baseURL := channel.ResolveAPIBaseURLForModel(path, request.Model)
	requestURL := resolveChannelEndpointURL(baseURL, path)
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
	if stream {
		c.Request.Header.Set("Accept", "text/event-stream")
	}
	baseURL := channel.ResolveAPIBaseURLForModel(path, requestedModel, actualModel)
	requestURL := resolveChannelEndpointURL(baseURL, path)
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

const tinyPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9WnSUs8AAAAASUVORK5CYII="

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
	requestURL := resolveChannelEndpointURL(baseURL, model.ChannelModelEndpointResponses)
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
		Size:   "1024x1024",
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
	requestURL := resolveChannelEndpointURL(baseURL, "/v1/images/generations")
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

func executeChannelImageEditModelTest(ctx context.Context, channel *model.Channel, modelName string) channelModelTestExecution {
	execution := channelModelTestExecution{}
	actualModelName := resolveChannelUpstreamModelName(channel, modelName)
	if actualModelName == "" {
		execution.Err = fmt.Errorf("未找到可用于图片编辑测试的模型")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	imageBytes, err := base64.StdEncoding.DecodeString(tinyPNGBase64)
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
	part, err := writer.CreateFormFile("image", "test.png")
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
	requestURL := resolveChannelEndpointURL(baseURL, "/v1/audio/speech")
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
	query := parsedURL.Query()
	query.Set("model", actualModelName)
	parsedURL.RawQuery = query.Encode()

	upstreamURL, err := normalizeChannelTestRealtimeWebSocketURL(parsedURL.String())
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}

	requestHeader := http.Header{}
	requestHeader.Set("OpenAI-Beta", "realtime=v1")
	switch relayMeta.ChannelProtocol {
	case relaychannel.Azure:
		requestHeader.Set("api-key", strings.TrimSpace(channel.Key))
	default:
		requestHeader.Set("Authorization", "Bearer "+strings.TrimSpace(channel.Key))
	}
	execution.InputPayload = buildHTTPRequestPayloadForLog(http.MethodGet, parsedURL.String(), requestHeader, nil)

	dialer := websocket.Dialer{
		Subprotocols: []string{"realtime"},
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
