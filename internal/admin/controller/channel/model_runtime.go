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
	"gorm.io/gorm"

	"github.com/yeying-community/router/common/client"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	commonutils "github.com/yeying-community/router/common/utils"
	"github.com/yeying-community/router/internal/admin/model"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
	"github.com/yeying-community/router/internal/relay"
	openaiadaptor "github.com/yeying-community/router/internal/relay/adaptor/openai"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/transport/http/middleware"
)

type openAIModelCard struct {
	ID               string         `json:"id"`
	OwnedBy          string         `json:"owned_by"`
	Type             string         `json:"type"`
	Modality         string         `json:"modality"`
	Modalities       []string       `json:"modalities"`
	InputModalities  []string       `json:"input_modalities"`
	OutputModalities []string       `json:"output_modalities"`
	Capabilities     map[string]any `json:"capabilities"`
	Architecture     struct {
		Type             string   `json:"type"`
		Modality         string   `json:"modality"`
		Modalities       []string `json:"modalities"`
		InputModalities  []string `json:"input_modalities"`
		OutputModalities []string `json:"output_modalities"`
	} `json:"architecture"`
}

type openAIModelsResponse struct {
	Data  []openAIModelCard `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type channelModelFetchTrace struct {
	ModelsURL       string
	RequestPayload  string
	ResponsePayload string
}

type channelModelTestsRequest struct {
	TargetModels  []string                     `json:"target_models"`
	TargetConfigs []channelModelTestTargetItem `json:"target_configs"`
	TestModel     string                       `json:"test_model,omitempty"`
}

type channelModelTestTargetItem struct {
	Model    string `json:"model"`
	Endpoint string `json:"endpoint,omitempty"`
}

type channelModelListData struct {
	Items         []model.ChannelModel `json:"items"`
	Total         int64                `json:"total"`
	Page          int                  `json:"page"`
	PageSize      int                  `json:"page_size"`
	SelectedCount int                  `json:"selected_count"`
	ActiveCount   int                  `json:"active_count"`
	InactiveCount int                  `json:"inactive_count"`
}

type channelTestListData struct {
	Items        []model.ChannelTest `json:"items"`
	LastTestedAt int64               `json:"last_tested_at"`
}

type channelModelTestExecution struct {
	LatencyMs          int64
	Message            string
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

	defaultChannelModelPageSize = 10
	maxChannelModelPageSize     = 100
)

func resolveModelsURL(baseURL string) string {
	resolvedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	lower := strings.ToLower(resolvedBaseURL)
	if strings.HasSuffix(lower, "/v1") ||
		strings.HasSuffix(lower, "/openai") ||
		strings.HasSuffix(lower, "/v1beta/openai") {
		return resolvedBaseURL + "/models"
	}
	return resolvedBaseURL + "/v1/models"
}

func normalizeChannelModelTypeHint(raw string) string {
	lower := strings.TrimSpace(strings.ToLower(raw))
	switch {
	case lower == "":
		return ""
	case strings.Contains(lower, "text-to-video"),
		strings.Contains(lower, "video_generation"),
		strings.Contains(lower, "video-generation"),
		strings.Contains(lower, "video"):
		return model.ProviderModelTypeVideo
	case strings.Contains(lower, "image"),
		strings.Contains(lower, "vision"),
		strings.Contains(lower, "diffusion"):
		return model.ProviderModelTypeImage
	case strings.Contains(lower, "audio"),
		strings.Contains(lower, "speech"),
		strings.Contains(lower, "tts"),
		strings.Contains(lower, "transcription"):
		return model.ProviderModelTypeAudio
	case strings.Contains(lower, "text"),
		strings.Contains(lower, "chat"),
		strings.Contains(lower, "completion"),
		strings.Contains(lower, "response"),
		strings.Contains(lower, "reason"):
		return model.ProviderModelTypeText
	default:
		return ""
	}
}

func inferUpstreamModelCardType(item openAIModelCard) string {
	candidates := []string{
		item.Type,
		item.Modality,
		item.Architecture.Type,
		item.Architecture.Modality,
	}
	for _, candidate := range candidates {
		if normalized := normalizeChannelModelTypeHint(candidate); normalized != "" {
			return normalized
		}
	}

	fallback := ""
	multiValueCandidates := [][]string{
		item.OutputModalities,
		item.Architecture.OutputModalities,
		item.Modalities,
		item.Architecture.Modalities,
		item.InputModalities,
		item.Architecture.InputModalities,
	}
	for _, values := range multiValueCandidates {
		for _, value := range values {
			if normalized := normalizeChannelModelTypeHint(value); normalized != "" {
				switch normalized {
				case model.ProviderModelTypeVideo:
					return normalized
				case model.ProviderModelTypeImage, model.ProviderModelTypeAudio:
					if fallback == "" || fallback == model.ProviderModelTypeText {
						fallback = normalized
					}
				case model.ProviderModelTypeText:
					if fallback == "" {
						fallback = normalized
					}
				}
			}
		}
	}
	if fallback != "" {
		return fallback
	}

	fallback = ""
	for key, raw := range item.Capabilities {
		enabled, ok := raw.(bool)
		if !ok || !enabled {
			continue
		}
		if normalized := normalizeChannelModelTypeHint(key); normalized != "" {
			switch normalized {
			case model.ProviderModelTypeVideo:
				return normalized
			case model.ProviderModelTypeImage, model.ProviderModelTypeAudio:
				if fallback == "" || fallback == model.ProviderModelTypeText {
					fallback = normalized
				}
			case model.ProviderModelTypeText:
				if fallback == "" {
					fallback = normalized
				}
			}
		}
	}
	return fallback
}

func fetchChannelModelsDetailed(protocol, key, baseURL, providerFilter string) ([]model.ChannelModel, channelModelFetchTrace, error) {
	trace := channelModelFetchTrace{}
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return nil, trace, fmt.Errorf("请先填写 Key")
	}
	trimmedBaseURL := strings.TrimSpace(baseURL)
	if trimmedBaseURL == "" {
		return nil, trace, fmt.Errorf("请先填写 Base URL")
	}

	modelsURL := resolveModelsURL(trimmedBaseURL)
	trace.ModelsURL = modelsURL
	httpReq, err := http.NewRequest(http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, trace, fmt.Errorf("创建请求失败")
	}
	switch relaychannel.NormalizeProtocolName(protocol) {
	case "anthropic":
		httpReq.Header.Set("x-api-key", trimmedKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
	default:
		httpReq.Header.Set("Authorization", "Bearer "+trimmedKey)
	}
	trace.RequestPayload = buildHTTPRequestPayloadForLog(httpReq.Method, modelsURL, httpReq.Header, nil)

	resp, err := client.HTTPClient.Do(httpReq)
	if err != nil {
		trace.ResponsePayload = marshalJSONForLog(map[string]any{
			"error": err.Error(),
		})
		return nil, trace, fmt.Errorf("请求模型列表失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		trace.ResponsePayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, nil)
		return nil, trace, fmt.Errorf("读取模型列表失败")
	}
	trace.ResponsePayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)

	var parsed openAIModelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, trace, fmt.Errorf("解析模型列表失败")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message := fmt.Sprintf("模型列表请求失败（HTTP %d）", resp.StatusCode)
		if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
			message = parsed.Error.Message
		}
		return nil, trace, fmt.Errorf("%s", message)
	}

	provider := commonutils.NormalizeProvider(providerFilter)
	seen := make(map[string]struct{}, len(parsed.Data))
	modelRows := make([]model.ChannelModel, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if provider != "" && !commonutils.MatchProvider(id, item.OwnedBy, provider) {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		modelRows = append(modelRows, model.ChannelModel{
			Model:         id,
			UpstreamModel: id,
			Type:          inferUpstreamModelCardType(item),
			Selected:      false,
		})
	}
	if len(modelRows) == 0 {
		if provider != "" {
			return nil, trace, fmt.Errorf("未找到符合所选供应商的模型")
		}
		return nil, trace, fmt.Errorf("未返回可用模型")
	}
	return modelRows, trace, nil
}

func resolveChannelBaseURL(protocol string, baseURL string) string {
	trimmedBaseURL := strings.TrimSpace(baseURL)
	if trimmedBaseURL != "" {
		return trimmedBaseURL
	}
	normalized := relaychannel.NormalizeProtocolName(protocol)
	if normalized == "" {
		return ""
	}
	return relaychannel.BaseURLByProtocol(normalized)
}

func loadChannelRuntimeState(protocol string, key string, baseURL string, channelID string, configRaw json.RawMessage, selectedModels []string, modelConfigs []model.ChannelModel, testModel string) (*model.Channel, string, error) {
	normalizedProtocol := relaychannel.NormalizeProtocolName(protocol)
	trimmedKey := strings.TrimSpace(key)
	trimmedBaseURL := strings.TrimSpace(baseURL)
	trimmedChannelID := strings.TrimSpace(channelID)
	normalizedModels := model.NormalizeChannelModelIDsPreserveOrder(selectedModels)
	normalizedModelConfigs := model.NormalizeChannelModelConfigsPreserveOrder(modelConfigs)
	keySource := "request"

	runtimeChannel := &model.Channel{
		Protocol: normalizedProtocol,
		Key:      trimmedKey,
	}

	if trimmedChannelID != "" {
		savedChannel, err := channelsvc.GetByID(trimmedChannelID)
		if err != nil {
			return nil, keySource, fmt.Errorf("渠道不存在或已删除")
		}
		runtimeChannel = savedChannel
		if trimmedKey == "" {
			trimmedKey = strings.TrimSpace(savedChannel.Key)
			keySource = "channel"
		}
		if normalizedProtocol == "" {
			normalizedProtocol = savedChannel.GetProtocol()
		}
		if trimmedBaseURL == "" {
			trimmedBaseURL = strings.TrimSpace(savedChannel.GetBaseURL())
		}
		if len(normalizedModelConfigs) == 0 && len(normalizedModels) == 0 {
			normalizedModels = savedChannel.SelectedModelIDs()
		}
		if strings.TrimSpace(testModel) == "" {
			testModel = strings.TrimSpace(savedChannel.TestModel)
		}
	}

	if normalizedProtocol == "" {
		normalizedProtocol = runtimeChannel.GetProtocol()
	}
	runtimeChannel.Protocol = normalizedProtocol
	runtimeChannel.NormalizeProtocol()
	runtimeChannel.Key = trimmedKey
	if trimmedBaseURL != "" {
		runtimeChannel.BaseURL = &trimmedBaseURL
	} else {
		resolvedBaseURL := resolveChannelBaseURL(runtimeChannel.GetProtocol(), runtimeChannel.GetBaseURL())
		if resolvedBaseURL != "" {
			runtimeChannel.BaseURL = &resolvedBaseURL
		}
	}
	if len(configRaw) > 0 && string(configRaw) != "null" {
		runtimeChannel.Config = string(configRaw)
	}
	if len(normalizedModelConfigs) > 0 {
		runtimeChannel.SetModelConfigs(normalizedModelConfigs)
	} else if len(normalizedModels) > 0 {
		runtimeChannel.SetSelectedModelIDs(normalizedModels)
	}
	if strings.TrimSpace(testModel) != "" {
		runtimeChannel.TestModel = strings.TrimSpace(testModel)
	}
	return runtimeChannel, keySource, nil
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

func selectedChannelModelConfigs(channel *model.Channel) []model.ChannelModel {
	if channel == nil {
		return nil
	}
	rows := channel.GetModelConfigs()
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
	allRows := channel.GetModelConfigs()
	if len(allRows) == 0 {
		return nil
	}
	selectedRows := selectedChannelModelConfigs(channel)

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
	endpoint := model.NormalizeChannelModelEndpoint(modelType, row.Endpoint)
	result := model.ChannelTest{
		Model:         strings.TrimSpace(row.Model),
		UpstreamModel: strings.TrimSpace(row.UpstreamModel),
		Type:          modelType,
		Endpoint:      endpoint,
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
	return runSingleChannelModelTestWithContext(context.Background(), channel, row)
}

func runSingleChannelModelTestWithContext(ctx context.Context, channel *model.Channel, row model.ChannelModel) (model.ChannelTest, channelModelTestExecution) {
	modelType := resolveSelectionModelType(row)
	endpoint := model.NormalizeChannelModelEndpoint(modelType, row.Endpoint)

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
		execution := executeChannelAudioModelTest(ctx, channel, row.Model)
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      model.ChannelModelEndpointAudio,
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
			execution := executeChannelTextModelTest(ctx, channel, endpoint, &relaymodel.GeneralOpenAIRequest{
				Model: row.Model,
				Messages: []relaymodel.Message{{
					Role:    "user",
					Content: config.TestPrompt,
				}},
			})
			return buildChannelModelTestResult(model.ChannelModel{
				Model:         row.Model,
				UpstreamModel: row.UpstreamModel,
				Type:          modelType,
				Endpoint:      endpoint,
			}, execution), execution
		}
		execution := executeChannelTextModelTest(ctx, channel, model.ChannelModelEndpointResponses, &relaymodel.GeneralOpenAIRequest{
			Model: row.Model,
			Input: []relaymodel.Message{{
				Role:    "user",
				Content: config.TestPrompt,
			}},
		})
		return buildChannelModelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      model.ChannelModelEndpointResponses,
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

func normalizeResponsesTestInput(request *relaymodel.GeneralOpenAIRequest) {
	if request == nil {
		return
	}
	switch value := request.Input.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return
		}
		request.Input = []relaymodel.Message{{
			Role:    "user",
			Content: value,
		}}
	case []string:
		if len(value) == 0 {
			return
		}
		messages := make([]relaymodel.Message, 0, len(value))
		for _, item := range value {
			if strings.TrimSpace(item) == "" {
				continue
			}
			messages = append(messages, relaymodel.Message{
				Role:    "user",
				Content: item,
			})
		}
		if len(messages) > 0 {
			request.Input = messages
		}
	case []any:
		if len(value) == 0 {
			return
		}
		messages := make([]relaymodel.Message, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				return
			}
			if strings.TrimSpace(text) == "" {
				continue
			}
			messages = append(messages, relaymodel.Message{
				Role:    "user",
				Content: text,
			})
		}
		if len(messages) > 0 {
			request.Input = messages
		}
	}
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
	if path == model.ChannelModelEndpointResponses {
		if request.Input == nil && len(request.Messages) > 0 {
			request.Input = request.Messages
			request.Messages = nil
		}
		normalizeResponsesTestInput(request)
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
	requestURL := resolveChannelEndpointURL(resolveChannelBaseURL(channel.GetProtocol(), channel.GetBaseURL()), path)
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
	message, parseErr := parseTextModelTestResponse(string(body))
	if parseErr != nil {
		execution.Err = parseErr
		return execution
	}
	execution.Message = message
	return execution
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
	requestBody, err := json.Marshal(request)
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	requestURL := resolveChannelEndpointURL(resolveChannelBaseURL(channel.GetProtocol(), channel.GetBaseURL()), model.ChannelModelEndpointResponses)
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
	requestURL := resolveChannelEndpointURL(resolveChannelBaseURL(channel.GetProtocol(), channel.GetBaseURL()), "/v1/images/generations")
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

	requestURL := resolveChannelEndpointURL(resolveChannelBaseURL(channel.GetProtocol(), channel.GetBaseURL()), model.ChannelModelEndpointImageEdit)
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

func executeChannelAudioModelTest(ctx context.Context, channel *model.Channel, modelName string) channelModelTestExecution {
	execution := channelModelTestExecution{}
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
	requestBody, err := json.Marshal(openaiadaptor.TextToSpeechRequest{
		Model:          actualModelName,
		Input:          "Model test from Router.",
		Voice:          "alloy",
		ResponseFormat: "mp3",
	})
	if err != nil {
		execution.Err = err
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": err.Error()})
		return execution
	}
	requestURL := resolveChannelEndpointURL(resolveChannelBaseURL(channel.GetProtocol(), channel.GetBaseURL()), "/v1/audio/speech")
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
	baseURL := resolveChannelBaseURL(channel.GetProtocol(), channel.GetBaseURL())
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

func persistChannelModelTests(channelID string, results []model.ChannelTest) error {
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
		currentRows, err := model.ListChannelModelRowsByChannelIDWithDB(tx, normalizedChannelID)
		if err != nil {
			return err
		}
		insertedResults, err := model.AppendChannelTestsForModelsWithDB(tx, normalizedChannelID, targetModels, results)
		if err != nil {
			return err
		}
		if err := model.ReplaceChannelModelConfigsWithDB(tx, normalizedChannelID, model.ApplyChannelTestResultsToModelConfigs(currentRows, insertedResults)); err != nil {
			return err
		}
		return model.EnsureChannelTestModelWithDB(tx, normalizedChannelID)
	})
}

func parseChannelModelPageParams(c *gin.Context) (page int, pageSize int, keyword string) {
	page = 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	pageSize = defaultChannelModelPageSize
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	if pageSize > maxChannelModelPageSize {
		pageSize = maxChannelModelPageSize
	}
	keyword = strings.TrimSpace(c.Query("keyword"))
	return page, pageSize, keyword
}

func buildChannelModelListData(channelID string, page int, pageSize int, keyword string) (channelModelListData, error) {
	rows, total, err := model.ListChannelModelRowsPageWithDB(model.DB, channelID, page-1, pageSize, keyword)
	if err != nil {
		return channelModelListData{}, err
	}
	channelRow, err := channelsvc.GetByID(channelID)
	if err != nil {
		return channelModelListData{}, err
	}
	allRows := channelRow.GetModelConfigs()
	selectedCount := 0
	activeCount := 0
	inactiveCount := 0
	for _, row := range allRows {
		if row.Inactive {
			inactiveCount++
			continue
		}
		activeCount++
		if row.Selected {
			selectedCount++
		}
	}
	return channelModelListData{
		Items:         rows,
		Total:         total,
		Page:          page,
		PageSize:      pageSize,
		SelectedCount: selectedCount,
		ActiveCount:   activeCount,
		InactiveCount: inactiveCount,
	}, nil
}

func buildChannelTestListData(channelID string) (channelTestListData, error) {
	rows, err := model.ListLatestChannelTestsByChannelIDWithDB(model.DB, channelID)
	if err != nil {
		return channelTestListData{}, err
	}
	return channelTestListData{
		Items:        rows,
		LastTestedAt: model.CalcChannelTestsLastTestedAt(rows),
	}, nil
}

// GetChannelModels godoc
// @Summary List channel models (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Channel ID"
// @Param page query int false "Page (1-based)"
// @Param page_size query int false "Page size"
// @Param keyword query string false "Keyword"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/{id}/models [get]
func GetChannelModels(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "渠道 ID 无效",
		})
		return
	}
	page, pageSize, keyword := parseChannelModelPageParams(c)
	data, err := buildChannelModelListData(channelID, page, pageSize, keyword)
	if err != nil {
		logChannelAdminWarn(c, "list_models", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

// GetChannelTests godoc
// @Summary List latest channel tests (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Channel ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/{id}/tests [get]
func GetChannelTests(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "渠道 ID 无效",
		})
		return
	}
	data, err := buildChannelTestListData(channelID)
	if err != nil {
		logChannelAdminWarn(c, "list_tests", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

// RefreshChannelModels godoc
// @Summary Refresh channel runtime data from upstream (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Channel ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/{id}/refresh [post]
func RefreshChannelModels(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		logChannelAdminWarn(c, "refresh_models", stringField("reason", "渠道 ID 无效"))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "渠道 ID 无效",
		})
		return
	}
	taskRow, reused, err := CreateChannelRefreshModelsTask(channelID, c.GetString(ctxkey.Id), c.GetString(helper.TraceIDKey))
	if err != nil {
		logChannelAdminWarn(c, "refresh_models", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logChannelAdminInfo(c, "refresh_models", stringField("channel_id", channelID), stringField("task_id", taskRow.Id), stringField("status", taskRow.Status))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"task": taskRow,
		},
		"meta": gin.H{
			"channel_id": channelID,
			"reused":     reused,
		},
	})
}

// TestChannelModels godoc
// @Summary Test channel models (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Channel ID"
// @Param body body docs.ChannelModelTestsRequest true "Channel model test payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/{id}/tests [post]
func TestChannelModels(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		logChannelAdminWarn(c, "test_models", stringField("reason", "渠道 ID 无效"))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "渠道 ID 无效",
		})
		return
	}
	var req channelModelTestsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logChannelAdminWarn(c, "test_models", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	tasks, createdCount, reusedCount, err := CreateChannelModelTestTasks(
		channelID,
		c.GetString(ctxkey.Id),
		strings.TrimSpace(req.TestModel),
		req.TargetModels,
		req.TargetConfigs,
		c.GetString(helper.TraceIDKey),
	)
	if err != nil {
		logChannelAdminWarn(c, "test_models", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logChannelAdminInfo(c, "test_models", stringField("channel_id", channelID), intField("created", createdCount), intField("reused", reusedCount))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"tasks": tasks,
		},
		"meta": gin.H{
			"channel_id": channelID,
			"created":    createdCount,
			"reused":     reusedCount,
		},
	})
}
