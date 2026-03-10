package channel

import (
	"bytes"
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

type previewModelsRequest struct {
	Protocol     string               `json:"protocol"`
	Key          string               `json:"key"`
	BaseURL      string               `json:"base_url"`
	ChannelID    string               `json:"channel_id"`
	Config       json.RawMessage      `json:"config"`
	ModelConfigs []model.ChannelModel `json:"model_configs"`
}

type previewModelTestsRequest struct {
	Protocol     string               `json:"protocol"`
	Key          string               `json:"key"`
	BaseURL      string               `json:"base_url"`
	ChannelID    string               `json:"channel_id"`
	Config       json.RawMessage      `json:"config"`
	Models       []string             `json:"models"`
	ModelConfigs []model.ChannelModel `json:"model_configs"`
	TestModel    string               `json:"test_model"`
	TargetModels []string             `json:"target_models"`
}

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

type previewModelFetchTrace struct {
	ModelsURL       string
	RequestPayload  string
	ResponsePayload string
}

type channelModelTestsRequest struct {
	TargetModels []string `json:"target_models"`
	TestModel    string   `json:"test_model,omitempty"`
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

type previewModelTestExecution struct {
	LatencyMs     int64
	Message       string
	InputPayload  string
	OutputPayload string
	Err           error
}

const (
	previewChannelTestModeBatch  = "batch"
	previewChannelTestModeSingle = "model"

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

func normalizePreviewModelType(raw string) string {
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
		if normalized := normalizePreviewModelType(candidate); normalized != "" {
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
			if normalized := normalizePreviewModelType(value); normalized != "" {
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
		if normalized := normalizePreviewModelType(key); normalized != "" {
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

func fetchModelsByConfiguredChannelDetailed(key, baseURL, providerFilter string) ([]model.ChannelModel, previewModelFetchTrace, error) {
	trace := previewModelFetchTrace{}
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
	httpReq.Header.Set("Authorization", "Bearer "+trimmedKey)
	trace.RequestPayload = buildHTTPRequestPayloadForLog(httpReq.Method, modelsURL, httpReq.Header, nil)

	resp, err := client.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, trace, fmt.Errorf("请求模型列表失败")
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

func resolvePreviewBaseURL(protocol string, baseURL string) string {
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

func loadPreviewChannel(protocol string, key string, baseURL string, channelID string, configRaw json.RawMessage, selectedModels []string, modelConfigs []model.ChannelModel, testModel string) (*model.Channel, string, error) {
	normalizedProtocol := relaychannel.NormalizeProtocolName(protocol)
	trimmedKey := strings.TrimSpace(key)
	trimmedBaseURL := strings.TrimSpace(baseURL)
	trimmedChannelID := strings.TrimSpace(channelID)
	normalizedModels := model.NormalizeChannelModelIDsPreserveOrder(selectedModels)
	normalizedModelConfigs := model.NormalizeChannelModelConfigsPreserveOrder(modelConfigs)
	keySource := "request"

	previewChannel := &model.Channel{
		Protocol: normalizedProtocol,
		Key:      trimmedKey,
	}

	if trimmedChannelID != "" {
		savedChannel, err := channelsvc.GetByID(trimmedChannelID)
		if err != nil {
			return nil, keySource, fmt.Errorf("渠道不存在或已删除")
		}
		previewChannel = savedChannel
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
		normalizedProtocol = previewChannel.GetProtocol()
	}
	previewChannel.Protocol = normalizedProtocol
	previewChannel.NormalizeProtocol()
	previewChannel.Key = trimmedKey
	if trimmedBaseURL != "" {
		previewChannel.BaseURL = &trimmedBaseURL
	} else {
		resolvedBaseURL := resolvePreviewBaseURL(previewChannel.GetProtocol(), previewChannel.GetBaseURL())
		if resolvedBaseURL != "" {
			previewChannel.BaseURL = &resolvedBaseURL
		}
	}
	if len(configRaw) > 0 && string(configRaw) != "null" {
		previewChannel.Config = string(configRaw)
	}
	if len(normalizedModelConfigs) > 0 {
		previewChannel.SetModelConfigs(normalizedModelConfigs)
	} else if len(normalizedModels) > 0 {
		previewChannel.SetSelectedModelIDs(normalizedModels)
	}
	if strings.TrimSpace(testModel) != "" {
		previewChannel.TestModel = strings.TrimSpace(testModel)
	}
	return previewChannel, keySource, nil
}

func normalizePreviewModelTestMode(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case previewChannelTestModeSingle:
		return previewChannelTestModeSingle
	case previewChannelTestModeBatch:
		return previewChannelTestModeBatch
	default:
		return previewChannelTestModeBatch
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

func resolvePreviewTargetModels(channel *model.Channel, mode string, requestedModel string, requestedModels []string) []model.ChannelModel {
	if channel == nil {
		return nil
	}
	allRows := channel.GetModelConfigs()
	if len(allRows) == 0 {
		return nil
	}
	selectedRows := selectedChannelModelConfigs(channel)

	targets := model.NormalizeChannelModelIDsPreserveOrder(requestedModels)
	if len(targets) == 0 && normalizePreviewModelTestMode(mode) == previewChannelTestModeSingle {
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

func buildPreviewChannelTestResult(row model.ChannelModel, execution previewModelTestExecution) model.ChannelTest {
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

func runSingleChannelModelTest(channel *model.Channel, row model.ChannelModel) (model.ChannelTest, previewModelTestExecution) {
	modelType := resolveSelectionModelType(row)
	endpoint := model.NormalizeChannelModelEndpoint(modelType, row.Endpoint)

	switch modelType {
	case model.ProviderModelTypeImage:
		execution := executePreviewImageModelTest(channel, row.Model)
		return buildPreviewChannelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      model.ChannelModelEndpointImages,
		}, execution), execution
	case model.ProviderModelTypeAudio:
		execution := executePreviewAudioModelTest(channel, row.Model)
		return buildPreviewChannelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      model.ChannelModelEndpointAudio,
		}, execution), execution
	case model.ProviderModelTypeVideo:
		execution := executePreviewVideoModelTest(channel, row.Model)
		return buildPreviewChannelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      model.ChannelModelEndpointVideos,
		}, execution), execution
	default:
		if endpoint == model.ChannelModelEndpointChat {
			execution := executePreviewTextModelTest(channel, endpoint, &relaymodel.GeneralOpenAIRequest{
				Model: row.Model,
				Messages: []relaymodel.Message{{
					Role:    "user",
					Content: config.TestPrompt,
				}},
			})
			return buildPreviewChannelTestResult(model.ChannelModel{
				Model:         row.Model,
				UpstreamModel: row.UpstreamModel,
				Type:          modelType,
				Endpoint:      endpoint,
			}, execution), execution
		}
		execution := executePreviewTextModelTest(channel, model.ChannelModelEndpointResponses, &relaymodel.GeneralOpenAIRequest{
			Model: row.Model,
			Input: []relaymodel.Message{{
				Role:    "user",
				Content: config.TestPrompt,
			}},
		})
		return buildPreviewChannelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      model.ChannelModelEndpointResponses,
		}, execution), execution
	}
}

func logChannelModelTestExecution(c *gin.Context, channelID string, result model.ChannelTest, execution previewModelTestExecution) {
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
	targetRows := resolvePreviewTargetModels(channel, mode, requestedModel, requestedModels)
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
		logChannelModelTestExecution(c, channelID, testResult, execution)
		results = append(results, testResult)
	}
	return model.NormalizeChannelTestRows(results), nil
}

func resolvePreviewModelName(channel *model.Channel, requestedModel string) string {
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

func newPreviewRelayContext(path string, channel *model.Channel) (*gin.Context, *meta.Meta, error) {
	if channel == nil {
		return nil, nil, fmt.Errorf("渠道不能为空")
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	requestURL := &url.URL{Path: path}
	c.Request = &http.Request{
		Method: "POST",
		URL:    requestURL,
		Body:   io.NopCloser(bytes.NewBuffer(nil)),
		Header: make(http.Header),
	}
	c.Request.Header.Set("Content-Type", "application/json")
	middleware.SetupContextForSelectedChannel(c, channel, "")
	return c, meta.GetByContext(c), nil
}

func resolvePreviewEndpointURL(baseURL string, path string) string {
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

func parsePreviewUpstreamError(statusCode int, body []byte) error {
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

func executePreviewTextModelTest(channel *model.Channel, path string, request *relaymodel.GeneralOpenAIRequest) previewModelTestExecution {
	execution := previewModelTestExecution{}
	if request == nil {
		execution.Err = fmt.Errorf("请求不能为空")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	c, relayMeta, err := newPreviewRelayContext(path, channel)
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
	request.Model = resolvePreviewModelName(channel, request.Model)
	if request.Model == "" {
		execution.Err = fmt.Errorf("未找到可用于测试的模型")
		execution.OutputPayload = marshalJSONForLog(map[string]any{"error": execution.Err.Error()})
		return execution
	}
	relayMeta.OriginModelName = request.Model
	relayMeta.ActualModelName = request.Model
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
	requestURL := resolvePreviewEndpointURL(resolvePreviewBaseURL(channel.GetProtocol(), channel.GetBaseURL()), path)
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
	execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		execution.Err = parsePreviewUpstreamError(resp.StatusCode, body)
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

func executePreviewImageModelTest(channel *model.Channel, modelName string) previewModelTestExecution {
	execution := previewModelTestExecution{}
	c, relayMeta, err := newPreviewRelayContext("/v1/images/generations", channel)
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
	actualModelName := resolvePreviewModelName(channel, modelName)
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
	requestURL := resolvePreviewEndpointURL(resolvePreviewBaseURL(channel.GetProtocol(), channel.GetBaseURL()), "/v1/images/generations")
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
	execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		execution.Err = parsePreviewUpstreamError(resp.StatusCode, body)
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

func executePreviewAudioModelTest(channel *model.Channel, modelName string) previewModelTestExecution {
	execution := previewModelTestExecution{}
	actualModelName := resolvePreviewModelName(channel, modelName)
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
	c, relayMeta, err := newPreviewRelayContext("/v1/audio/speech", channel)
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
	requestURL := resolvePreviewEndpointURL(resolvePreviewBaseURL(channel.GetProtocol(), channel.GetBaseURL()), "/v1/audio/speech")
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
	execution.OutputPayload = buildHTTPResponsePayloadForLog(resp.StatusCode, resp.Header, body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		execution.Err = parsePreviewUpstreamError(resp.StatusCode, body)
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

func executePreviewVideoModelTest(channel *model.Channel, modelName string) previewModelTestExecution {
	execution := previewModelTestExecution{}
	actualModelName := resolvePreviewModelName(channel, modelName)
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
	baseURL := resolvePreviewBaseURL(channel.GetProtocol(), channel.GetBaseURL())
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

	requestURL := resolvePreviewEndpointURL(baseURL, "/v1/videos")
	httpReq, err := http.NewRequest(http.MethodPost, requestURL, bodyBuffer)
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
		execution.Err = parsePreviewUpstreamError(resp.StatusCode, body)
		return execution
	}

	type previewVideoResponse struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	parsed := previewVideoResponse{}
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

func persistPreviewChannelTests(channelID string, rows []model.ChannelModel, results []model.ChannelTest) error {
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
		insertedResults, err := model.AppendChannelTestsForModelsWithDB(tx, normalizedChannelID, targetModels, results)
		if err != nil {
			return err
		}
		if err := model.ReplaceChannelModelConfigsWithDB(tx, normalizedChannelID, model.ApplyChannelTestResultsToModelConfigs(rows, insertedResults)); err != nil {
			return err
		}
		return model.EnsureChannelTestModelWithDB(tx, normalizedChannelID)
	})
}

func parseChannelModelPageParams(c *gin.Context) (page int, pageSize int, keyword string) {
	page = 0
	if raw := strings.TrimSpace(c.Query("p")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
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
	rows, total, err := model.ListChannelModelRowsPageWithDB(model.DB, channelID, page, pageSize, keyword)
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
// @Param p query int false "Page index"
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
// @Summary Refresh channel models from upstream (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Channel ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/{id}/models/refresh [post]
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
	previewChannel, keySource, err := loadPreviewChannel("", "", "", channelID, nil, nil, nil, "")
	if err != nil {
		logChannelAdminWarn(c, "refresh_models", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	baseURL := resolvePreviewBaseURL(previewChannel.GetProtocol(), previewChannel.GetBaseURL())
	fetchedRows, fetchTrace, err := fetchModelsByConfiguredChannelDetailed(previewChannel.Key, baseURL, "")
	if err != nil {
		logChannelAdminWarn(
			c,
			"refresh_models",
			stringField("source", keySource),
			stringField("channel_id", channelID),
			stringField("models_url", fetchTrace.ModelsURL),
			structuredPayloadField("request_payload", fetchTrace.RequestPayload),
			structuredPayloadField("response_payload", fetchTrace.ResponsePayload),
			stringField("reason", err.Error()),
		)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logChannelAdminInfo(
		c,
		"refresh_models",
		stringField("source", keySource),
		stringField("channel_id", channelID),
		stringField("models_url", fetchTrace.ModelsURL),
		structuredPayloadField("request_payload", fetchTrace.RequestPayload),
		structuredPayloadField("response_payload", fetchTrace.ResponsePayload),
		intField("count", len(fetchedRows)),
	)
	if err := model.SyncFetchedChannelModelConfigsFromBaseWithDB(model.DB, channelID, previewChannel.GetModelConfigs(), fetchedRows); err != nil {
		logChannelAdminWarn(c, "refresh_models_save", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "保存渠道模型失败",
		})
		return
	}
	if err := model.EnsureChannelTestModelWithDB(model.DB, channelID); err != nil {
		logChannelAdminWarn(c, "refresh_models_test_model_sync", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "保存测试模型失败",
		})
		return
	}
	data, err := buildChannelModelListData(channelID, 0, defaultChannelModelPageSize, "")
	if err != nil {
		logChannelAdminWarn(c, "refresh_models_reload", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "读取渠道模型失败",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
		"meta": gin.H{
			"source":     "channel",
			"key_source": keySource,
			"channel_id": channelID,
			"models_url": fetchTrace.ModelsURL,
			"count":      len(fetchedRows),
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
	previewChannel, keySource, err := loadPreviewChannel("", "", "", channelID, nil, nil, nil, strings.TrimSpace(req.TestModel))
	if err != nil {
		logChannelAdminWarn(c, "test_models", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	testMode := previewChannelTestModeBatch
	if len(req.TargetModels) == 1 || strings.TrimSpace(req.TestModel) != "" {
		testMode = previewChannelTestModeSingle
	}
	results, err := runChannelModelTests(c, previewChannel, testMode, req.TestModel, req.TargetModels)
	if err != nil {
		logChannelAdminWarn(c, "test_models", stringField("source", keySource), stringField("channel_id", channelID), stringField("base_url", previewChannel.GetBaseURL()), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if err := persistPreviewChannelTests(channelID, previewChannel.GetModelConfigs(), results); err != nil {
		logChannelAdminWarn(c, "test_models_save", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "保存模型测试结果失败",
		})
		return
	}
	savedChannel, getErr := channelsvc.GetByID(channelID)
	if getErr != nil {
		logChannelAdminWarn(c, "test_models_reload", stringField("channel_id", channelID), stringField("reason", getErr.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "读取渠道测试结果失败",
		})
		return
	}
	results = savedChannel.Tests
	logChannelAdminInfo(c, "test_models", stringField("source", keySource), stringField("channel_id", channelID), stringField("base_url", previewChannel.GetBaseURL()), intField("results", len(results)))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"results": results,
		},
	})
}
