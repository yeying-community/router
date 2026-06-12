package channel

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yeying-community/router/common/client"
	commonutils "github.com/yeying-community/router/common/utils"
	"github.com/yeying-community/router/internal/admin/model"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
)

type openAIModelCard struct {
	ID               string         `json:"id"`
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

func resolveModelsURL(baseURL string, protocol string) string {
	resolvedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	switch relaychannel.NormalizeProtocolName(protocol) {
	case "ali":
		lower := strings.ToLower(resolvedBaseURL)
		if strings.HasSuffix(lower, "/compatible-mode/v1") {
			return resolvedBaseURL + "/models"
		}
		return resolvedBaseURL + "/compatible-mode/v1/models"
	case "deepseek":
		return resolvedBaseURL + "/models"
	}
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
	case strings.Contains(lower, "embedding"),
		strings.Contains(lower, "embeddings"),
		strings.Contains(lower, "embed"):
		return model.ProviderModelTypeEmbedding
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
	if usesProviderOfficialModelsForSync(protocol) {
		return fetchChannelModelsFromProviderOfficialData(protocol, providerFilter, trace)
	}
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return nil, trace, fmt.Errorf("请先填写 Key")
	}
	trimmedBaseURL := strings.TrimSpace(baseURL)
	if trimmedBaseURL == "" {
		return nil, trace, fmt.Errorf("请先填写 Base URL")
	}

	modelsURL := resolveModelsURL(trimmedBaseURL, protocol)
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

	modelCandidates := make([]string, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		modelCandidates = append(modelCandidates, id)
	}
	providerByModel, err := model.LoadUniqueProviderMapByModels(modelCandidates)
	if err != nil {
		return nil, trace, fmt.Errorf("加载供应商模型信息失败: %w", err)
	}

	provider := commonutils.NormalizeProvider(providerFilter)
	seen := make(map[string]struct{}, len(parsed.Data))
	modelRows := make([]model.ChannelModel, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		resolvedProvider := model.ResolveProviderFromModelMap(providerByModel, id)
		if provider != "" {
			if resolvedProvider != "" {
				if commonutils.NormalizeProvider(resolvedProvider) != provider {
					continue
				}
			} else {
				continue
			}
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		modelRows = append(modelRows, model.ChannelModel{
			Model:         id,
			UpstreamModel: id,
			Provider:      resolvedProvider,
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

func usesProviderOfficialModelsForSync(protocol string) bool {
	switch relaychannel.NormalizeProtocolName(protocol) {
	case "doubao", "zhipu":
		return true
	default:
		return false
	}
}

func fetchChannelModelsFromProviderOfficialData(protocol string, providerFilter string, trace channelModelFetchTrace) ([]model.ChannelModel, channelModelFetchTrace, error) {
	provider := commonutils.NormalizeProvider(providerFilter)
	if provider == "" {
		provider = commonutils.NormalizeProvider(relaychannel.NormalizeProtocolName(protocol))
	}
	if provider == "" {
		return nil, trace, fmt.Errorf("未识别供应商")
	}
	details, err := model.ListActiveProviderModelDetailsWithDB(model.DB, provider)
	if err != nil {
		return nil, trace, fmt.Errorf("加载供应商模型信息失败: %w", err)
	}
	if len(details) == 0 {
		return nil, trace, fmt.Errorf("未找到供应商官方模型，请先执行供应商模型迁移")
	}
	rows := make([]model.ChannelModel, 0, len(details))
	seen := make(map[string]struct{}, len(details))
	for _, detail := range details {
		modelName := strings.TrimSpace(detail.Model)
		if modelName == "" {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		rows = append(rows, model.ChannelModel{
			Model:         modelName,
			UpstreamModel: providerOfficialUpstreamModel(provider, modelName),
			Provider:      provider,
			Type:          detail.Type,
			Selected:      false,
		})
	}
	if len(rows) == 0 {
		return nil, trace, fmt.Errorf("未找到供应商官方模型")
	}
	return rows, trace, nil
}

func providerOfficialUpstreamModel(provider string, modelName string) string {
	normalizedModel := strings.TrimSpace(modelName)
	return normalizedModel
}

func loadChannelSyncState(protocol string, key string, baseURL string, channelID string, configRaw json.RawMessage, selectedModels []string, channelModels []model.ChannelModel, testModel string) (*model.Channel, string, error) {
	normalizedProtocol := relaychannel.NormalizeProtocolName(protocol)
	trimmedKey := strings.TrimSpace(key)
	trimmedBaseURL := strings.TrimSpace(baseURL)
	trimmedChannelID := strings.TrimSpace(channelID)
	normalizedModels := model.NormalizeChannelModelIDsPreserveOrder(selectedModels)
	normalizedChannelModels := model.NormalizeChannelModelsPreserveOrder(channelModels)
	keySource := "request"

	resolvedChannel := &model.Channel{
		Protocol: normalizedProtocol,
		Key:      trimmedKey,
	}

	if trimmedChannelID != "" {
		savedChannel, err := channelsvc.GetByID(trimmedChannelID)
		if err != nil {
			return nil, keySource, fmt.Errorf("渠道不存在或已删除")
		}
		resolvedChannel = savedChannel
		if trimmedKey == "" {
			trimmedKey = strings.TrimSpace(savedChannel.Key)
			keySource = "channel"
		}
		if normalizedProtocol == "" {
			normalizedProtocol = savedChannel.GetProtocol()
		}
		if trimmedBaseURL == "" {
			trimmedBaseURL = strings.TrimSpace(savedChannel.ResolveAPIBaseURL(""))
		}
		if len(normalizedChannelModels) == 0 && len(normalizedModels) == 0 {
			normalizedModels = savedChannel.SelectedModelIDs()
		}
		if strings.TrimSpace(testModel) == "" {
			testModel = strings.TrimSpace(savedChannel.TestModel)
		}
	}

	if normalizedProtocol == "" {
		normalizedProtocol = resolvedChannel.GetProtocol()
	}
	resolvedChannel.Protocol = normalizedProtocol
	resolvedChannel.NormalizeProtocol()
	resolvedChannel.Key = trimmedKey
	if trimmedBaseURL != "" {
		resolvedChannel.BaseURL = &trimmedBaseURL
	} else {
		resolvedBaseURL := resolvedChannel.ResolveAPIBaseURL("")
		if resolvedBaseURL != "" {
			resolvedChannel.BaseURL = &resolvedBaseURL
		}
	}
	if len(configRaw) > 0 && string(configRaw) != "null" {
		resolvedChannel.Config = string(configRaw)
	}
	if len(normalizedChannelModels) > 0 {
		resolvedChannel.SetChannelModels(normalizedChannelModels)
	} else if len(normalizedModels) > 0 {
		resolvedChannel.SetSelectedModelIDs(normalizedModels)
	}
	if strings.TrimSpace(testModel) != "" {
		resolvedChannel.TestModel = strings.TrimSpace(testModel)
	}
	return resolvedChannel, keySource, nil
}
