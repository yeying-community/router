package channel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/client"
	"github.com/yeying-community/router/common/logger"
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
	Protocol string          `json:"protocol"`
	Key      string          `json:"key"`
	BaseURL  string          `json:"base_url"`
	DraftID  string          `json:"draft_id"`
	Config   json.RawMessage `json:"config"`
}

type previewCapabilitiesRequest struct {
	Protocol           string                                `json:"protocol"`
	Key                string                                `json:"key"`
	BaseURL            string                                `json:"base_url"`
	DraftID            string                                `json:"draft_id"`
	Config             json.RawMessage                       `json:"config"`
	Models             []string                              `json:"models"`
	TestModel          string                                `json:"test_model"`
	CapabilityProfiles *[]model.ChannelCapabilityProfileRule `json:"capability_profiles"`
}

type openAIModelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type previewCapabilityResult struct {
	Capability    string `json:"capability"`
	Label         string `json:"label"`
	Endpoint      string `json:"endpoint"`
	Model         string `json:"model,omitempty"`
	ClientProfile string `json:"client_profile,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
	Status        string `json:"status"`
	Supported     bool   `json:"supported"`
	Message       string `json:"message,omitempty"`
	LatencyMs     int64  `json:"latency_ms,omitempty"`
}

const (
	previewCapabilityStatusSupported   = "supported"
	previewCapabilityStatusUnsupported = "unsupported"
	previewCapabilityStatusSkipped     = "skipped"
)

func persistPreviewCapabilityResults(channelID string, results []previewCapabilityResult) error {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	rows := make([]model.ChannelCapabilityResult, 0, len(results))
	for idx, item := range results {
		rows = append(rows, model.ChannelCapabilityResult{
			ChannelId:     normalizedChannelID,
			Capability:    strings.TrimSpace(item.Capability),
			ClientProfile: model.NormalizeClientProfileName(item.ClientProfile),
			Label:         strings.TrimSpace(item.Label),
			Endpoint:      strings.TrimSpace(item.Endpoint),
			Model:         strings.TrimSpace(item.Model),
			UserAgent:     strings.TrimSpace(item.UserAgent),
			Status:        model.NormalizeChannelCapabilityStatus(item.Status),
			Supported:     item.Supported && item.Status == previewCapabilityStatusSupported,
			Message:       strings.TrimSpace(item.Message),
			LatencyMs:     item.LatencyMs,
			SortOrder:     int64(idx),
		})
	}
	return model.ReplaceChannelCapabilityResultsWithDB(model.DB, normalizedChannelID, rows)
}

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

func fetchModelsByConfiguredChannelDetailed(key, baseURL, modelProvider string) ([]string, string, error) {
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

	provider := commonutils.NormalizeModelProvider(modelProvider)
	seen := make(map[string]struct{}, len(parsed.Data))
	modelIDs := make([]string, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if provider != "" && !commonutils.MatchModelProvider(id, item.OwnedBy, provider) {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		modelIDs = append(modelIDs, id)
	}
	if len(modelIDs) == 0 {
		if provider != "" {
			return nil, modelsURL, fmt.Errorf("未找到符合所选模型供应商的模型")
		}
		return nil, modelsURL, fmt.Errorf("未返回可用模型")
	}
	return modelIDs, modelsURL, nil
}

func fetchModelsByConfiguredChannel(key, baseURL, modelProvider string) ([]string, error) {
	modelIDs, _, err := fetchModelsByConfiguredChannelDetailed(key, baseURL, modelProvider)
	return modelIDs, err
}

func fetchOpenAICompatibleModelIDsByBaseURL(key, baseURL, modelProvider string) ([]string, error) {
	return fetchModelsByConfiguredChannel(key, baseURL, modelProvider)
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

func loadPreviewChannel(protocol string, key string, baseURL string, draftID string, configRaw json.RawMessage, selectedModels []string, testModel string, capabilityProfiles *[]model.ChannelCapabilityProfileRule) (*model.Channel, string, error) {
	normalizedProtocol := relaychannel.NormalizeProtocolName(protocol)
	trimmedKey := strings.TrimSpace(key)
	trimmedBaseURL := strings.TrimSpace(baseURL)
	trimmedDraftID := strings.TrimSpace(draftID)
	normalizedModels := model.NormalizeChannelModelIDsPreserveOrder(selectedModels)
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
		if len(normalizedModels) == 0 {
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
	if len(normalizedModels) > 0 {
		previewChannel.SetSelectedModelIDs(normalizedModels)
	}
	if capabilityProfiles != nil {
		previewChannel.SetCapabilityProfiles(*capabilityProfiles)
	}
	if strings.TrimSpace(testModel) != "" {
		previewChannel.TestModel = strings.TrimSpace(testModel)
	}
	return previewChannel, keySource, nil
}

func buildClientProfileDisplayNames(profiles []model.ClientProfile) map[string]string {
	result := make(map[string]string, len(profiles))
	for _, profile := range profiles {
		name := model.NormalizeClientProfileName(profile.Name)
		if name == "" {
			continue
		}
		displayName := strings.TrimSpace(profile.DisplayName)
		if displayName == "" {
			displayName = name
		}
		result[name] = displayName
	}
	return result
}

func runChannelCapabilityTests(channel *model.Channel, userAgent string) ([]previewCapabilityResult, error) {
	textModel, imageModel, audioModel := pickCapabilityModels(channel)
	clientProfiles, err := model.ListEnabledClientProfilesWithDB(model.DB)
	if err != nil {
		return nil, fmt.Errorf("读取客户端画像失败: %w", err)
	}
	clientProfileNames := buildClientProfileDisplayNames(clientProfiles)
	results := make([]previewCapabilityResult, 0, 4)

	if strings.TrimSpace(textModel) == "" {
		results = append(results, previewCapabilityResult{
			Capability: "chat",
			Label:      "Chat",
			Endpoint:   "/v1/chat/completions",
			Status:     previewCapabilityStatusSkipped,
			Message:    "未找到可用于文本能力测试的模型",
		})
		results = append(results, previewCapabilityResult{
			Capability: "responses",
			Label:      "Responses",
			Endpoint:   "/v1/responses",
			Status:     previewCapabilityStatusSkipped,
			Message:    "未找到可用于文本能力测试的模型",
		})
	} else {
		latencyMs, message, execErr := executePreviewTextCapability(channel, "/v1/chat/completions", &relaymodel.GeneralOpenAIRequest{
			Model: textModel,
			Messages: []relaymodel.Message{{
				Role:    "user",
				Content: "Reply with pong.",
			}},
		}, userAgent)
		results = append(results, buildPreviewCapabilityResult("chat", "Chat", "/v1/chat/completions", textModel, latencyMs, message, execErr))

		responseRules := channel.CapabilityProfilesByCapability(model.ChannelCapabilityResponses)
		if len(responseRules) == 0 {
			results = append(results, previewCapabilityResult{
				Capability: model.ChannelCapabilityResponses,
				Label:      "Responses",
				Endpoint:   "/v1/responses",
				Model:      textModel,
				Status:     previewCapabilityStatusSkipped,
				Message:    "未选择 responses 客户端白名单，已跳过 responses 测试",
			})
		} else {
			for _, rule := range responseRules {
				resolvedUserAgent := channel.ResolveCapabilityUpstreamUserAgent(model.ChannelCapabilityResponses, rule.ClientProfile, clientProfiles, userAgent)
				latencyMs, message, execErr = executePreviewTextCapability(channel, "/v1/responses", &relaymodel.GeneralOpenAIRequest{
					Model: textModel,
					Input: "Reply with pong.",
				}, resolvedUserAgent)
				label := "Responses"
				if displayName := clientProfileNames[rule.ClientProfile]; displayName != "" {
					label = "Responses / " + displayName
				}
				result := buildPreviewCapabilityResult("responses:"+rule.ClientProfile, label, "/v1/responses", textModel, latencyMs, message, execErr)
				result.ClientProfile = rule.ClientProfile
				result.UserAgent = resolvedUserAgent
				if resolvedUserAgent != "" {
					if strings.TrimSpace(result.Message) == "" {
						result.Message = "User-Agent: " + resolvedUserAgent
					} else {
						result.Message = "User-Agent: " + resolvedUserAgent + " | " + result.Message
					}
				}
				results = append(results, result)
			}
		}
	}

	if strings.TrimSpace(imageModel) == "" {
		results = append(results, previewCapabilityResult{
			Capability: "images",
			Label:      "Images",
			Endpoint:   "/v1/images/generations",
			Status:     previewCapabilityStatusSkipped,
			Message:    "未选择图片模型，已跳过图片能力测试",
		})
	} else {
		latencyMs, message, execErr := executePreviewImageCapability(channel, imageModel, userAgent)
		results = append(results, buildPreviewCapabilityResult("images", "Images", "/v1/images/generations", imageModel, latencyMs, message, execErr))
	}

	if strings.TrimSpace(audioModel) == "" {
		results = append(results, previewCapabilityResult{
			Capability: "audio",
			Label:      "Audio",
			Endpoint:   "/v1/audio/speech",
			Status:     previewCapabilityStatusSkipped,
			Message:    "未选择音频模型，已跳过音频能力测试",
		})
	} else {
		latencyMs, message, execErr := executePreviewAudioCapability(channel, audioModel, userAgent)
		result := buildPreviewCapabilityResult("audio", "Audio", "/v1/audio/speech", audioModel, latencyMs, message, execErr)
		if execErr != nil && strings.Contains(strings.ToLower(execErr.Error()), "暂不自动探测") {
			result.Status = previewCapabilityStatusSkipped
			result.Message = execErr.Error()
		}
		results = append(results, result)
	}

	return results, nil
}

func pickCapabilityModels(channel *model.Channel) (string, string, string) {
	if channel == nil {
		return "", "", ""
	}
	selectedModels := channel.SelectedModelIDs()
	textModel := strings.TrimSpace(channel.TestModel)
	imageModel := ""
	audioModel := ""
	if textModel != "" {
		switch model.InferModelType(textModel) {
		case model.ModelProviderModelTypeImage:
			imageModel = textModel
			textModel = ""
		case model.ModelProviderModelTypeAudio:
			audioModel = textModel
			textModel = ""
		}
	}
	for _, modelName := range selectedModels {
		switch model.InferModelType(modelName) {
		case model.ModelProviderModelTypeImage:
			if imageModel == "" {
				imageModel = modelName
			}
		case model.ModelProviderModelTypeAudio:
			if audioModel == "" {
				audioModel = modelName
			}
		default:
			if textModel == "" {
				textModel = modelName
			}
		}
	}
	if textModel == "" && len(selectedModels) > 0 {
		textModel = selectedModels[0]
	}
	return textModel, imageModel, audioModel
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

func newPreviewRelayContext(path string, channel *model.Channel, userAgent string) (*gin.Context, *meta.Meta, error) {
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
	if strings.TrimSpace(userAgent) != "" {
		c.Request.Header.Set("User-Agent", strings.TrimSpace(userAgent))
	}
	middleware.SetupContextForSelectedChannel(c, channel, "")
	return c, meta.GetByContext(c), nil
}

func executePreviewTextCapability(channel *model.Channel, path string, request *relaymodel.GeneralOpenAIRequest, userAgent string) (int64, string, error) {
	if request == nil {
		return 0, "", fmt.Errorf("请求不能为空")
	}
	c, relayMeta, err := newPreviewRelayContext(path, channel, userAgent)
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
	message, err := parseChannelTestResponse(string(body))
	if err != nil {
		return latencyMs, "", err
	}
	return latencyMs, message, nil
}

func executePreviewImageCapability(channel *model.Channel, modelName string, userAgent string) (int64, string, error) {
	c, relayMeta, err := newPreviewRelayContext("/v1/images/generations", channel, userAgent)
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
		return 0, "", fmt.Errorf("未找到可用于图片能力测试的模型")
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

func executePreviewAudioCapability(channel *model.Channel, modelName string, userAgent string) (int64, string, error) {
	actualModelName := resolvePreviewModelName(channel, modelName)
	if actualModelName == "" {
		return 0, "", fmt.Errorf("未找到可用于音频能力测试的模型")
	}
	if strings.Contains(strings.ToLower(actualModelName), "whisper") {
		return 0, "", fmt.Errorf("当前音频模型更像转录模型，暂不自动探测")
	}
	c, relayMeta, err := newPreviewRelayContext("/v1/audio/speech", channel, userAgent)
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
		Input:          "Capability test from Router.",
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

func buildPreviewCapabilityResult(capability string, label string, endpoint string, modelName string, latencyMs int64, message string, err error) previewCapabilityResult {
	result := previewCapabilityResult{
		Capability: capability,
		Label:      label,
		Endpoint:   endpoint,
		Model:      strings.TrimSpace(modelName),
		LatencyMs:  latencyMs,
	}
	if err == nil {
		result.Status = previewCapabilityStatusSupported
		result.Supported = true
		result.Message = strings.TrimSpace(message)
		return result
	}
	result.Message = strings.TrimSpace(err.Error())
	if result.Message == "" {
		result.Message = "能力测试失败"
	}
	result.Status = previewCapabilityStatusUnsupported
	return result
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
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	previewChannel, keySource, err := loadPreviewChannel(req.Protocol, req.Key, req.BaseURL, req.DraftID, req.Config, nil, "", nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	baseURL := resolvePreviewBaseURL(previewChannel.GetProtocol(), previewChannel.GetBaseURL())
	modelIDs, modelsURL, err := fetchModelsByConfiguredChannelDetailed(previewChannel.Key, baseURL, "")
	if err != nil {
		logger.SysWarnf("channel preview models failed: source=%s draft_id=%s models_url=%s err=%v", keySource, strings.TrimSpace(req.DraftID), modelsURL, err)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	draftID := strings.TrimSpace(req.DraftID)
	logger.SysLogf("channel preview models fetched: source=%s draft_id=%s models_url=%s count=%d", keySource, draftID, modelsURL, len(modelIDs))
	if draftID != "" {
		if err := model.SyncFetchedChannelModelsWithDB(model.DB, draftID, modelIDs); err != nil {
			logger.SysWarnf("channel preview models save failed: draft_id=%s err=%v", draftID, err)
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "保存渠道模型失败",
			})
			return
		}
		if err := model.EnsureChannelTestModelWithDB(model.DB, draftID); err != nil {
			logger.SysWarnf("channel preview test model sync failed: draft_id=%s err=%v", draftID, err)
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "保存测试模型失败",
			})
			return
		}
		if err := model.DeleteChannelCapabilityResultsByChannelIDWithDB(model.DB, draftID); err != nil {
			logger.SysWarnf("channel preview capability results reset failed: draft_id=%s err=%v", draftID, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    modelIDs,
		"meta": gin.H{
			"source":     "channel",
			"key_source": keySource,
			"draft_id":   draftID,
			"models_url": modelsURL,
		},
	})
}

// PreviewChannelCapabilities godoc
// @Summary Preview channel capabilities (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.ChannelPreviewCapabilitiesRequest true "Preview payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/preview/capabilities [post]
func PreviewChannelCapabilities(c *gin.Context) {
	var req previewCapabilitiesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	previewChannel, keySource, err := loadPreviewChannel(req.Protocol, req.Key, req.BaseURL, req.DraftID, req.Config, req.Models, req.TestModel, req.CapabilityProfiles)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if strings.TrimSpace(previewChannel.Key) == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请先填写 Key",
		})
		return
	}
	if strings.TrimSpace(previewChannel.GetBaseURL()) == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请先填写 Base URL",
		})
		return
	}

	userAgent := strings.TrimSpace(c.Request.UserAgent())
	results, err := runChannelCapabilityTests(previewChannel, userAgent)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if draftID := strings.TrimSpace(req.DraftID); draftID != "" {
		if err := persistPreviewCapabilityResults(draftID, results); err != nil {
			logger.SysWarnf("channel preview capability results save failed: draft_id=%s err=%v", draftID, err)
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "保存能力测试结果失败",
			})
			return
		}
	}

	logger.SysLogf("channel preview capabilities fetched: source=%s draft_id=%s base_url=%s results=%d", keySource, strings.TrimSpace(req.DraftID), previewChannel.GetBaseURL(), len(results))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"results": results,
		},
		"meta": gin.H{
			"source":     "channel",
			"key_source": keySource,
			"draft_id":   strings.TrimSpace(req.DraftID),
		},
	})
}
