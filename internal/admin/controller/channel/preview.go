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
	relaycontroller "github.com/yeying-community/router/internal/relay/controller"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/transport/http/middleware"
)

type previewModelsRequest struct {
	Protocol     string               `json:"protocol"`
	Key          string               `json:"key"`
	BaseURL      string               `json:"base_url"`
	DraftID      string               `json:"draft_id"`
	Config       json.RawMessage      `json:"config"`
	ModelConfigs []model.ChannelModel `json:"model_configs"`
}

type previewModelTestsRequest struct {
	Protocol     string               `json:"protocol"`
	Key          string               `json:"key"`
	BaseURL      string               `json:"base_url"`
	DraftID      string               `json:"draft_id"`
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

const (
	previewChannelTestModeBatch  = "batch"
	previewChannelTestModeSingle = "model"
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

func fetchModelsByConfiguredChannelDetailed(key, baseURL, providerFilter string) ([]model.ChannelModel, string, error) {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return nil, "", fmt.Errorf("请先填写 Key")
	}
	trimmedBaseURL := strings.TrimSpace(baseURL)
	if trimmedBaseURL == "" {
		return nil, "", fmt.Errorf("请先填写 Base URL")
	}

	modelsURL := resolveModelsURL(trimmedBaseURL)
	httpReq, err := http.NewRequest(http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("创建请求失败")
	}
	httpReq.Header.Set("Authorization", "Bearer "+trimmedKey)

	resp, err := client.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, "", fmt.Errorf("请求模型列表失败")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取模型列表失败")
	}

	var parsed openAIModelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, "", fmt.Errorf("解析模型列表失败")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message := fmt.Sprintf("模型列表请求失败（HTTP %d）", resp.StatusCode)
		if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
			message = parsed.Error.Message
		}
		return nil, modelsURL, fmt.Errorf("%s", message)
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
			Selected:      true,
		})
	}
	if len(modelRows) == 0 {
		if provider != "" {
			return nil, modelsURL, fmt.Errorf("未找到符合所选供应商的模型")
		}
		return nil, modelsURL, fmt.Errorf("未返回可用模型")
	}
	return modelRows, modelsURL, nil
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

func loadPreviewChannel(protocol string, key string, baseURL string, draftID string, configRaw json.RawMessage, selectedModels []string, modelConfigs []model.ChannelModel, testModel string) (*model.Channel, string, error) {
	normalizedProtocol := relaychannel.NormalizeProtocolName(protocol)
	trimmedKey := strings.TrimSpace(key)
	trimmedBaseURL := strings.TrimSpace(baseURL)
	trimmedDraftID := strings.TrimSpace(draftID)
	normalizedModels := model.NormalizeChannelModelIDsPreserveOrder(selectedModels)
	normalizedModelConfigs := model.NormalizeChannelModelConfigsPreserveOrder(modelConfigs)
	keySource := "request"

	previewChannel := &model.Channel{
		Protocol: normalizedProtocol,
		Key:      trimmedKey,
	}

	if trimmedDraftID != "" {
		savedChannel, err := channelsvc.GetByID(trimmedDraftID, true)
		if err != nil {
			return nil, keySource, fmt.Errorf("渠道不存在或已删除")
		}
		previewChannel = savedChannel
		if trimmedKey == "" {
			trimmedKey = strings.TrimSpace(savedChannel.Key)
			keySource = "draft"
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
	selectedRows := selectedChannelModelConfigs(channel)
	if len(selectedRows) == 0 {
		return nil
	}

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
		return selectedRows
	}

	result := make([]model.ChannelModel, 0, len(targets))
	targetSet := make(map[string]struct{}, len(targets))
	for _, item := range targets {
		targetSet[item] = struct{}{}
	}
	for _, row := range selectedRows {
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

func buildPreviewChannelTestResult(row model.ChannelModel, latencyMs int64, message string, err error) model.ChannelTest {
	modelType := resolveSelectionModelType(row)
	endpoint := model.NormalizeChannelModelEndpoint(modelType, row.Endpoint)
	result := model.ChannelTest{
		Model:         strings.TrimSpace(row.Model),
		UpstreamModel: strings.TrimSpace(row.UpstreamModel),
		Type:          modelType,
		Endpoint:      endpoint,
		LatencyMs:     latencyMs,
		Message:       strings.TrimSpace(message),
	}
	if result.UpstreamModel == "" {
		result.UpstreamModel = result.Model
	}
	if err == nil {
		result.Status = model.ChannelTestStatusSupported
		result.Supported = true
		return result
	}
	errMessage := strings.TrimSpace(err.Error())
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

func runSingleChannelModelTest(channel *model.Channel, row model.ChannelModel) model.ChannelTest {
	modelType := resolveSelectionModelType(row)
	endpoint := model.NormalizeChannelModelEndpoint(modelType, row.Endpoint)

	switch modelType {
	case model.ProviderModelTypeImage:
		latencyMs, message, execErr := executePreviewImageModelTest(channel, row.Model)
		return buildPreviewChannelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      model.ChannelModelEndpointImages,
		}, latencyMs, message, execErr)
	case model.ProviderModelTypeAudio:
		latencyMs, message, execErr := executePreviewAudioModelTest(channel, row.Model)
		return buildPreviewChannelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      model.ChannelModelEndpointAudio,
		}, latencyMs, message, execErr)
	case model.ProviderModelTypeVideo:
		latencyMs, message, execErr := executePreviewVideoModelTest(channel, row.Model)
		return buildPreviewChannelTestResult(model.ChannelModel{
			Model:         row.Model,
			UpstreamModel: row.UpstreamModel,
			Type:          modelType,
			Endpoint:      model.ChannelModelEndpointVideos,
		}, latencyMs, message, execErr)
	default:
		if endpoint == model.ChannelModelEndpointChat {
			latencyMs, message, execErr := executePreviewTextModelTest(channel, endpoint, &relaymodel.GeneralOpenAIRequest{
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
			}, latencyMs, message, execErr)
		}
		latencyMs, message, execErr := executePreviewTextModelTest(channel, model.ChannelModelEndpointResponses, &relaymodel.GeneralOpenAIRequest{
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
		}, latencyMs, message, execErr)
	}
}

func runChannelModelTests(channel *model.Channel, mode string, requestedModel string, requestedModels []string) ([]model.ChannelTest, error) {
	targetRows := resolvePreviewTargetModels(channel, mode, requestedModel, requestedModels)
	if len(targetRows) == 0 {
		return nil, fmt.Errorf("未找到可用于测试的模型")
	}
	results := make([]model.ChannelTest, 0, len(targetRows))
	for _, row := range targetRows {
		results = append(results, runSingleChannelModelTest(channel, row))
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

func executePreviewTextModelTest(channel *model.Channel, path string, request *relaymodel.GeneralOpenAIRequest) (int64, string, error) {
	if request == nil {
		return 0, "", fmt.Errorf("请求不能为空")
	}
	c, relayMeta, err := newPreviewRelayContext(path, channel)
	if err != nil {
		return 0, "", err
	}
	adaptor := relay.GetAdaptor(relayMeta.APIType)
	if adaptor == nil {
		return 0, "", fmt.Errorf("invalid api type: %d", relayMeta.APIType)
	}
	adaptor.Init(relayMeta)
	request.Model = resolvePreviewModelName(channel, request.Model)
	if request.Model == "" {
		return 0, "", fmt.Errorf("未找到可用于测试的模型")
	}
	relayMeta.OriginModelName = request.Model
	relayMeta.ActualModelName = request.Model
	convertedRequest, err := adaptor.ConvertRequest(c, relayMeta.Mode, request)
	if err != nil {
		return 0, "", err
	}
	requestBody, err := json.Marshal(convertedRequest)
	if err != nil {
		return 0, "", err
	}
	startedAt := time.Now()
	resp, err := adaptor.DoRequest(c, relayMeta, bytes.NewBuffer(requestBody))
	latencyMs := time.Since(startedAt).Milliseconds()
	if err != nil {
		return latencyMs, "", err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		relayErr := relaycontroller.RelayErrorHandler(resp)
		if relayErr != nil && strings.TrimSpace(relayErr.Error.Message) != "" {
			return latencyMs, "", fmt.Errorf("http status code: %d, error message: %s", resp.StatusCode, relayErr.Error.Message)
		}
		return latencyMs, "", fmt.Errorf("http status code: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return latencyMs, "", err
	}
	if err := resp.Body.Close(); err != nil {
		return latencyMs, "", err
	}
	message, err := parseTextModelTestResponse(string(body))
	if err != nil {
		return latencyMs, "", err
	}
	return latencyMs, message, nil
}

func executePreviewImageModelTest(channel *model.Channel, modelName string) (int64, string, error) {
	c, relayMeta, err := newPreviewRelayContext("/v1/images/generations", channel)
	if err != nil {
		return 0, "", err
	}
	adaptor := relay.GetAdaptor(relayMeta.APIType)
	if adaptor == nil {
		return 0, "", fmt.Errorf("invalid api type: %d", relayMeta.APIType)
	}
	adaptor.Init(relayMeta)
	actualModelName := resolvePreviewModelName(channel, modelName)
	if actualModelName == "" {
		return 0, "", fmt.Errorf("未找到可用于图片模型测试的模型")
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
		return 0, "", err
	}
	requestBody, err := json.Marshal(convertedRequest)
	if err != nil {
		return 0, "", err
	}
	startedAt := time.Now()
	resp, err := adaptor.DoRequest(c, relayMeta, bytes.NewBuffer(requestBody))
	latencyMs := time.Since(startedAt).Milliseconds()
	if err != nil {
		return latencyMs, "", err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		relayErr := relaycontroller.RelayErrorHandler(resp)
		if relayErr != nil && strings.TrimSpace(relayErr.Error.Message) != "" {
			return latencyMs, "", fmt.Errorf("http status code: %d, error message: %s", resp.StatusCode, relayErr.Error.Message)
		}
		return latencyMs, "", fmt.Errorf("http status code: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return latencyMs, "", err
	}
	if err := resp.Body.Close(); err != nil {
		return latencyMs, "", err
	}
	preview := "图片接口返回成功"
	imageResponse := openaiadaptor.ImageResponse{}
	if err := json.Unmarshal(body, &imageResponse); err == nil && len(imageResponse.Data) > 0 {
		preview = fmt.Sprintf("返回 %d 个图片结果", len(imageResponse.Data))
	}
	return latencyMs, preview, nil
}

func executePreviewAudioModelTest(channel *model.Channel, modelName string) (int64, string, error) {
	actualModelName := resolvePreviewModelName(channel, modelName)
	if actualModelName == "" {
		return 0, "", fmt.Errorf("未找到可用于音频模型测试的模型")
	}
	if strings.Contains(strings.ToLower(actualModelName), "whisper") {
		return 0, "", fmt.Errorf("当前音频模型更像转录模型，暂不自动探测")
	}
	c, relayMeta, err := newPreviewRelayContext("/v1/audio/speech", channel)
	if err != nil {
		return 0, "", err
	}
	c.Request.Header.Set("Accept", "audio/mpeg")
	adaptor := relay.GetAdaptor(relayMeta.APIType)
	if adaptor == nil {
		return 0, "", fmt.Errorf("invalid api type: %d", relayMeta.APIType)
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
		return 0, "", err
	}
	startedAt := time.Now()
	resp, err := adaptor.DoRequest(c, relayMeta, bytes.NewBuffer(requestBody))
	latencyMs := time.Since(startedAt).Milliseconds()
	if err != nil {
		return latencyMs, "", err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		relayErr := relaycontroller.RelayErrorHandler(resp)
		if relayErr != nil && strings.TrimSpace(relayErr.Error.Message) != "" {
			return latencyMs, "", fmt.Errorf("http status code: %d, error message: %s", resp.StatusCode, relayErr.Error.Message)
		}
		return latencyMs, "", fmt.Errorf("http status code: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return latencyMs, "", err
	}
	if err := resp.Body.Close(); err != nil {
		return latencyMs, "", err
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "audio payload"
	}
	if len(body) == 0 {
		return latencyMs, "", fmt.Errorf("响应为空")
	}
	return latencyMs, fmt.Sprintf("返回 %d bytes (%s)", len(body), contentType), nil
}

func executePreviewVideoModelTest(channel *model.Channel, modelName string) (int64, string, error) {
	actualModelName := resolvePreviewModelName(channel, modelName)
	if actualModelName == "" {
		return 0, "", fmt.Errorf("未找到可用于视频模型测试的模型")
	}
	if channel == nil {
		return 0, "", fmt.Errorf("渠道不能为空")
	}
	baseURL := resolvePreviewBaseURL(channel.GetProtocol(), channel.GetBaseURL())
	if strings.TrimSpace(baseURL) == "" {
		return 0, "", fmt.Errorf("未找到可用于视频模型测试的 Base URL")
	}

	bodyBuffer := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuffer)
	if err := writer.WriteField("model", actualModelName); err != nil {
		return 0, "", err
	}
	if err := writer.WriteField("prompt", "A short blue sphere morphing into a cube."); err != nil {
		return 0, "", err
	}
	if err := writer.Close(); err != nil {
		return 0, "", err
	}

	requestURL := resolvePreviewEndpointURL(baseURL, "/v1/videos")
	httpReq, err := http.NewRequest(http.MethodPost, requestURL, bodyBuffer)
	if err != nil {
		return 0, "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(channel.Key))
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Accept", "application/json")

	startedAt := time.Now()
	resp, err := client.HTTPClient.Do(httpReq)
	latencyMs := time.Since(startedAt).Milliseconds()
	if err != nil {
		return latencyMs, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return latencyMs, "", err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return latencyMs, "", parsePreviewUpstreamError(resp.StatusCode, body)
	}

	type previewVideoResponse struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	parsed := previewVideoResponse{}
	if err := json.Unmarshal(body, &parsed); err == nil {
		if strings.TrimSpace(parsed.ID) != "" && strings.TrimSpace(parsed.Status) != "" {
			return latencyMs, fmt.Sprintf("返回任务 %s（%s）", strings.TrimSpace(parsed.ID), strings.TrimSpace(parsed.Status)), nil
		}
		if strings.TrimSpace(parsed.ID) != "" {
			return latencyMs, fmt.Sprintf("返回任务 %s", strings.TrimSpace(parsed.ID)), nil
		}
		if strings.TrimSpace(parsed.Status) != "" {
			return latencyMs, fmt.Sprintf("视频任务状态：%s", strings.TrimSpace(parsed.Status)), nil
		}
	}
	return latencyMs, "视频接口返回成功", nil
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

// PreviewChannelModels godoc
// @Summary Preview models for channel protocol (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.ChannelPreviewModelsRequest true "Preview payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/preview/models [post]
func PreviewChannelModels(c *gin.Context) {
	var req previewModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logChannelAdminWarn(c, "preview_models", stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	draftID := strings.TrimSpace(req.DraftID)
	if draftID == "" {
		logChannelAdminWarn(c, "preview_models", stringField("reason", "请先保存渠道后再刷新模型"))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请先保存渠道后再刷新模型",
		})
		return
	}
	previewChannel, keySource, err := loadPreviewChannel(req.Protocol, req.Key, req.BaseURL, draftID, req.Config, nil, req.ModelConfigs, "")
	if err != nil {
		logChannelAdminWarn(c, "preview_models", stringField("draft_id", strings.TrimSpace(req.DraftID)), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	baseURL := resolvePreviewBaseURL(previewChannel.GetProtocol(), previewChannel.GetBaseURL())
	fetchedRows, modelsURL, err := fetchModelsByConfiguredChannelDetailed(previewChannel.Key, baseURL, "")
	if err != nil {
		logChannelAdminWarn(c, "preview_models", stringField("source", keySource), stringField("draft_id", strings.TrimSpace(req.DraftID)), stringField("models_url", modelsURL), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logChannelAdminInfo(c, "preview_models", stringField("source", keySource), stringField("draft_id", draftID), stringField("models_url", modelsURL), intField("count", len(fetchedRows)))
	if err := model.SyncFetchedChannelModelConfigsFromBaseWithDB(model.DB, draftID, previewChannel.GetModelConfigs(), fetchedRows); err != nil {
		logChannelAdminWarn(c, "preview_models_save", stringField("draft_id", draftID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "保存渠道模型失败",
		})
		return
	}
	if err := model.EnsureChannelTestModelWithDB(model.DB, draftID); err != nil {
		logChannelAdminWarn(c, "preview_test_model_sync", stringField("draft_id", draftID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "保存测试模型失败",
		})
		return
	}
	if err := model.DeleteChannelTestsByChannelIDWithDB(model.DB, draftID); err != nil {
		logChannelAdminWarn(c, "preview_tests_reset", stringField("draft_id", draftID), stringField("reason", err.Error()))
	}
	if err := model.ResetChannelModelTestStateWithDB(model.DB, draftID, nil); err != nil {
		logChannelAdminWarn(c, "preview_tests_state_reset", stringField("draft_id", draftID), stringField("reason", err.Error()))
	}
	savedChannel, getErr := channelsvc.GetByID(draftID, true)
	if getErr != nil {
		logChannelAdminWarn(c, "preview_models_reload", stringField("draft_id", draftID), stringField("reason", getErr.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "读取渠道模型失败",
		})
		return
	}
	modelConfigs := savedChannel.GetModelConfigs()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"model_configs":    modelConfigs,
			"available_models": savedChannel.AvailableModels,
			"selected_models":  savedChannel.SelectedModelIDs(),
		},
		"meta": gin.H{
			"source":     "channel",
			"key_source": keySource,
			"draft_id":   draftID,
			"models_url": modelsURL,
			"count":      len(modelConfigs),
		},
	})
}

// PreviewChannelModelTests godoc
// @Summary Preview channel model tests (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.ChannelPreviewCapabilitiesRequest true "Preview payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/preview/model-tests [post]
func PreviewChannelModelTests(c *gin.Context) {
	var req previewModelTestsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logChannelAdminWarn(c, "preview_model_tests", stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	previewChannel, keySource, err := loadPreviewChannel(req.Protocol, req.Key, req.BaseURL, req.DraftID, req.Config, req.Models, req.ModelConfigs, req.TestModel)
	if err != nil {
		logChannelAdminWarn(c, "preview_model_tests", stringField("draft_id", strings.TrimSpace(req.DraftID)), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if strings.TrimSpace(previewChannel.Key) == "" {
		logChannelAdminWarn(c, "preview_model_tests", stringField("draft_id", strings.TrimSpace(req.DraftID)), stringField("reason", "请先填写 Key"))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请先填写 Key",
		})
		return
	}
	if strings.TrimSpace(previewChannel.GetBaseURL()) == "" {
		logChannelAdminWarn(c, "preview_model_tests", stringField("draft_id", strings.TrimSpace(req.DraftID)), stringField("reason", "请先填写 Base URL"))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请先填写 Base URL",
		})
		return
	}

	testMode := previewChannelTestModeBatch
	if len(req.TargetModels) == 1 || strings.TrimSpace(req.TestModel) != "" {
		testMode = previewChannelTestModeSingle
	}
	results, err := runChannelModelTests(previewChannel, testMode, req.TestModel, req.TargetModels)
	if err != nil {
		logChannelAdminWarn(c, "preview_model_tests", stringField("source", keySource), stringField("draft_id", strings.TrimSpace(req.DraftID)), stringField("base_url", previewChannel.GetBaseURL()), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	modelConfigs := previewChannel.GetModelConfigs()
	if draftID := strings.TrimSpace(req.DraftID); draftID != "" {
		if err := persistPreviewChannelTests(draftID, previewChannel.GetModelConfigs(), results); err != nil {
			logChannelAdminWarn(c, "preview_model_tests_save", stringField("draft_id", draftID), stringField("reason", err.Error()))
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "保存模型测试结果失败",
			})
			return
		}
		savedChannel, getErr := channelsvc.GetByID(draftID, true)
		if getErr != nil {
			logChannelAdminWarn(c, "preview_model_tests_reload", stringField("draft_id", draftID), stringField("reason", getErr.Error()))
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "读取渠道测试结果失败",
			})
			return
		}
		modelConfigs = savedChannel.GetModelConfigs()
		results = savedChannel.Tests
	}

	logChannelAdminInfo(c, "preview_model_tests", stringField("source", keySource), stringField("draft_id", strings.TrimSpace(req.DraftID)), stringField("base_url", previewChannel.GetBaseURL()), intField("results", len(results)))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"results":       results,
			"model_configs": modelConfigs,
		},
		"meta": gin.H{
			"source":     "channel",
			"key_source": keySource,
			"draft_id":   strings.TrimSpace(req.DraftID),
		},
	})
}
