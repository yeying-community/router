package channel

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
	"gorm.io/gorm"
)

type channelModelTestTaskPayload struct {
	ChannelID         string `json:"channel_id"`
	Model             string `json:"model"`
	Endpoint          string `json:"endpoint"`
	IsStream          *bool  `json:"is_stream,omitempty"`
	AudioLanguage     string `json:"audio_language,omitempty"`
	ImageEditURL      string `json:"image_edit_url,omitempty"`
	ImageEditData     string `json:"image_edit_data,omitempty"`
	ResponsesTestMode string `json:"responses_test_mode,omitempty"`
}

type channelRefreshModelsTaskPayload struct {
	ChannelID string `json:"channel_id"`
}

type channelRefreshBillingTaskPayload struct {
	ChannelID string `json:"channel_id"`
}

type channelModelTestTaskTarget struct {
	Row               model.ChannelModel
	Stream            *bool
	ResponsesTestMode string
}

func buildChannelModelTestTaskDedupeKey(channelID string, modelID string, endpoint string, streamOverride *bool, audioLanguage string, imageEditURL string, imageEditData string, responsesTestMode string) string {
	normalizedModelID := strings.TrimSpace(modelID)
	normalizedEndpoint := model.NormalizeRequestedChannelModelEndpoint(endpoint)
	normalizedAudioLanguage := normalizeAudioTestLanguage(audioLanguage)
	imageEditSignature := channelModelTestImageEditSignature(normalizedEndpoint, imageEditURL, imageEditData)
	normalizedResponsesTestMode := ""
	if normalizedEndpoint == model.ChannelModelEndpointResponses {
		normalizedResponsesTestMode = normalizeResponsesTestMode(responsesTestMode)
	}
	if streamOverride == nil {
		if normalizedAudioLanguage == "zh-CN" && imageEditSignature == "" && normalizedResponsesTestMode == "" {
			return fmt.Sprintf("%s:%s:%s:%s", model.AsyncTaskTypeChannelModelTest, strings.TrimSpace(channelID), normalizedModelID, normalizedEndpoint)
		}
		return fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s", model.AsyncTaskTypeChannelModelTest, strings.TrimSpace(channelID), normalizedModelID, normalizedEndpoint, normalizedAudioLanguage, imageEditSignature, normalizedResponsesTestMode)
	}
	key := fmt.Sprintf(
		"%s:%s:%s:%s:%t",
		model.AsyncTaskTypeChannelModelTest,
		strings.TrimSpace(channelID),
		normalizedModelID,
		normalizedEndpoint,
		*streamOverride,
	)
	if normalizedAudioLanguage != "zh-CN" {
		key = fmt.Sprintf("%s:%s", key, normalizedAudioLanguage)
	}
	if imageEditSignature != "" {
		key = fmt.Sprintf("%s:%s", key, imageEditSignature)
	}
	if normalizedResponsesTestMode != "" {
		key = fmt.Sprintf("%s:%s", key, normalizedResponsesTestMode)
	}
	return key
}

func channelModelTestImageEditSignature(endpoint string, imageEditURL string, imageEditData string) string {
	if model.NormalizeRequestedChannelModelEndpoint(endpoint) != model.ChannelModelEndpointImageEdit {
		return ""
	}
	source := strings.TrimSpace(imageEditData)
	if source == "" {
		source = strings.TrimSpace(imageEditURL)
	}
	if source == "" {
		source = defaultChannelImageEditTestURL
	}
	sum := sha256.Sum256([]byte(source))
	return fmt.Sprintf("image:%x", sum[:8])
}

func buildChannelRefreshModelsTaskDedupeKey(channelID string) string {
	return fmt.Sprintf("%s:%s", model.AsyncTaskTypeChannelRefreshModels, strings.TrimSpace(channelID))
}

func buildChannelRefreshBillingTaskDedupeKey(channelID string) string {
	return fmt.Sprintf("%s:%s", model.AsyncTaskTypeChannelRefreshBilling, strings.TrimSpace(channelID))
}

func buildChannelModelTestTaskPayload(modelID string, channelID string, endpoint string, streamOverride *bool, audioLanguage string, imageEditURL string, imageEditData string, responsesTestMode string) string {
	return marshalJSONForLog(channelModelTestTaskPayload{
		ChannelID:         strings.TrimSpace(channelID),
		Model:             strings.TrimSpace(modelID),
		Endpoint:          model.NormalizeRequestedChannelModelEndpoint(endpoint),
		IsStream:          streamOverride,
		AudioLanguage:     normalizeAudioTestLanguage(audioLanguage),
		ImageEditURL:      strings.TrimSpace(imageEditURL),
		ImageEditData:     strings.TrimSpace(imageEditData),
		ResponsesTestMode: normalizeResponsesTestMode(responsesTestMode),
	})
}

func resolveChannelModelTestTaskTargets(channelRow *model.Channel, testMode string, requestedTestModel string, requestedModels []string, requestedConfigs []channelModelTestTargetItem) []channelModelTestTaskTarget {
	if len(requestedConfigs) == 0 {
		targetRows := resolveChannelTestTargetModels(channelRow, testMode, requestedTestModel, requestedModels)
		result := make([]channelModelTestTaskTarget, 0, len(targetRows))
		for _, row := range targetRows {
			result = append(result, channelModelTestTaskTarget{Row: row})
		}
		return result
	}

	targetRows := resolveChannelTestTargetModels(channelRow, channelModelTestModeBatch, "", nil)
	if len(targetRows) == 0 {
		return nil
	}
	rowByModel := make(map[string]model.ChannelModel, len(targetRows)*2)
	for _, row := range targetRows {
		if modelID := strings.TrimSpace(row.Model); modelID != "" {
			rowByModel[modelID] = row
		}
		if upstreamModel := strings.TrimSpace(row.UpstreamModel); upstreamModel != "" {
			rowByModel[upstreamModel] = row
		}
	}

	result := make([]channelModelTestTaskTarget, 0, len(requestedConfigs))
	seen := make(map[string]struct{}, len(requestedConfigs))
	for _, item := range requestedConfigs {
		modelID := strings.TrimSpace(item.Model)
		endpoint := model.NormalizeRequestedChannelModelEndpoint(item.Endpoint)
		if modelID == "" || endpoint == "" {
			continue
		}
		row, ok := rowByModel[modelID]
		if !ok {
			continue
		}
		row.Endpoint = endpoint
		hasDeclaredEndpoint := false
		for _, candidate := range row.Endpoints {
			if model.NormalizeRequestedChannelModelEndpoint(candidate) == endpoint {
				hasDeclaredEndpoint = true
				break
			}
		}
		if !hasDeclaredEndpoint {
			row.Endpoints = append(row.Endpoints, endpoint)
		}
		key := fmt.Sprintf("%s::%s", strings.TrimSpace(row.Model), endpoint)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, channelModelTestTaskTarget{
			Row:               row,
			Stream:            item.IsStream,
			ResponsesTestMode: normalizeResponsesTestMode(item.ResponsesTestMode),
		})
	}
	return result
}

func CreateChannelModelTestTasks(channelID string, createdBy string, requestedTestModel string, requestedModels []string, requestedConfigs []channelModelTestTargetItem, traceID string, requestedAudioLanguage string, requestedImageEditURL string, requestedImageEditData string) ([]model.AsyncTask, int, int, error) {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil, 0, 0, fmt.Errorf("渠道 ID 无效")
	}
	normalizedAudioLanguage := normalizeAudioTestLanguage(requestedAudioLanguage)
	channelRow, err := channelsvc.GetByID(normalizedChannelID)
	if err != nil {
		return nil, 0, 0, err
	}
	testMode := channelModelTestModeBatch
	if len(requestedModels) == 1 || strings.TrimSpace(requestedTestModel) != "" {
		testMode = channelModelTestModeSingle
	}
	targets := resolveChannelModelTestTaskTargets(channelRow, testMode, requestedTestModel, requestedModels, requestedConfigs)
	if len(targets) == 0 {
		return nil, 0, 0, fmt.Errorf("未找到可用于测试的模型")
	}
	tasks := make([]model.AsyncTask, 0, len(targets))
	createdCount := 0
	reusedCount := 0
	for _, target := range targets {
		row := target.Row
		stream := target.Stream
		normalizedEndpoint, endpointErr := resolveChannelModelTestEndpointForRow(row)
		if endpointErr != nil {
			return nil, createdCount, reusedCount, endpointErr
		}
		if err := validateChannelModelTestEndpointAgainstProvider(row, normalizedEndpoint); err != nil {
			return nil, createdCount, reusedCount, err
		}
		if resolveSelectionModelType(row) == model.ProviderModelTypeAudio {
			stream = nil
		}
		responsesTestMode := ""
		if normalizedEndpoint == model.ChannelModelEndpointResponses {
			responsesTestMode = target.ResponsesTestMode
		}
		imageEditURL := ""
		imageEditData := ""
		if normalizedEndpoint == model.ChannelModelEndpointImageEdit {
			imageEditURL = requestedImageEditURL
			imageEditData = requestedImageEditData
		}
		modelID := strings.TrimSpace(row.Model)
		task, reused, err := model.CreateOrReuseAsyncTaskWithDB(model.DB, model.AsyncTask{
			Type:      model.AsyncTaskTypeChannelModelTest,
			DedupeKey: buildChannelModelTestTaskDedupeKey(normalizedChannelID, modelID, normalizedEndpoint, stream, normalizedAudioLanguage, imageEditURL, imageEditData, responsesTestMode),
			ChannelId: normalizedChannelID,
			Model:     modelID,
			Endpoint:  normalizedEndpoint,
			Payload:   buildChannelModelTestTaskPayload(modelID, normalizedChannelID, normalizedEndpoint, stream, normalizedAudioLanguage, imageEditURL, imageEditData, responsesTestMode),
			CreatedBy: strings.TrimSpace(createdBy),
			TraceID:   strings.TrimSpace(traceID),
		})
		if err != nil {
			return nil, createdCount, reusedCount, err
		}
		if reused {
			reusedCount++
		} else {
			createdCount++
		}
		tasks = append(tasks, task)
	}
	return tasks, createdCount, reusedCount, nil
}

func EnqueueChannelModelEndpointRecoveryTest(channelID string, modelID string, endpoint string, traceID string) (bool, error) {
	normalizedModelID := strings.TrimSpace(modelID)
	normalizedEndpoint := model.NormalizeRequestedChannelModelEndpoint(endpoint)
	if strings.TrimSpace(channelID) == "" || normalizedModelID == "" || normalizedEndpoint == "" {
		return false, nil
	}
	_, createdCount, _, err := CreateChannelModelTestTasks(
		channelID,
		"runtime",
		normalizedModelID,
		[]string{normalizedModelID},
		[]channelModelTestTargetItem{
			{
				Model:    normalizedModelID,
				Endpoint: normalizedEndpoint,
			},
		},
		traceID,
		"",
		"",
		"",
	)
	if err != nil {
		return false, err
	}
	return createdCount > 0, nil
}

func enqueueInsufficientBalanceRecoveryTest(channelRow *model.Channel, traceID string) (bool, error) {
	if channelRow == nil || strings.TrimSpace(channelRow.Id) == "" {
		return false, nil
	}
	channelID := strings.TrimSpace(channelRow.Id)
	logInsufficientBalanceRecoveryProbe("load_channel", channelID, "", "", "")
	fullChannelRow, err := channelsvc.GetByID(channelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logInsufficientBalanceRecoveryProbe("skip_missing_channel", channelID, "", "channel not found", "")
			return false, nil
		}
		return false, err
	}
	if fullChannelRow.Status != model.ChannelStatusAutoDisabled {
		logInsufficientBalanceRecoveryProbe("skip_channel_status", channelID, "", "channel is no longer auto disabled", fmt.Sprintf("status=%d", fullChannelRow.Status))
		return false, nil
	}
	targetRows := resolveChannelTestTargetModels(fullChannelRow, channelModelTestModeSingle, "", nil)
	if len(targetRows) == 0 {
		logInsufficientBalanceRecoveryProbe("skip_no_test_target", channelID, "", "no selected testable channel model", fmt.Sprintf("channel_models=%d selected_models=%d test_model=%s", len(fullChannelRow.ChannelModels), len(fullChannelRow.SelectedModelIDs()), strings.TrimSpace(fullChannelRow.TestModel)))
		return false, nil
	}
	target := targetRows[0]
	modelID := strings.TrimSpace(target.Model)
	endpoint, err := resolveChannelModelTestEndpointForRow(target)
	if err != nil {
		logInsufficientBalanceRecoveryProbe("skip_invalid_endpoint", channelID, modelID, err.Error(), "")
		return false, err
	}
	logInsufficientBalanceRecoveryProbe("create_test_task", channelID, modelID, "", fmt.Sprintf("endpoint=%s", endpoint))
	_, createdCount, reusedCount, err := CreateChannelModelTestTasks(
		channelID,
		"channel_recovery",
		modelID,
		[]string{modelID},
		[]channelModelTestTargetItem{
			{
				Model:    modelID,
				Endpoint: endpoint,
			},
		},
		traceID,
		"",
		"",
		"",
	)
	if err != nil {
		logInsufficientBalanceRecoveryProbe("create_test_task_failed", channelID, modelID, err.Error(), fmt.Sprintf("endpoint=%s", endpoint))
		return false, err
	}
	logInsufficientBalanceRecoveryProbe("test_task_result", channelID, modelID, "", fmt.Sprintf("endpoint=%s created=%d reused=%d", endpoint, createdCount, reusedCount))
	return createdCount > 0, nil
}

func EnqueueRuntimeDisabledCapabilityRecoveryTests(limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	createdCount := 0
	modelRows, err := model.ListRecentDisabledChannelModelsWithDB(model.DB, limit)
	if err != nil {
		return createdCount, err
	}
	seen := make(map[string]struct{})
	for _, row := range modelRows {
		if strings.TrimSpace(row.DisabledBy) != "runtime" {
			continue
		}
		key := strings.TrimSpace(row.ChannelId) + "::" + strings.TrimSpace(row.Model) + "::" + model.NormalizeRequestedChannelModelEndpoint(row.Endpoint)
		if _, ok := seen[key]; ok {
			continue
		}
		created, err := EnqueueChannelModelEndpointRecoveryTest(row.ChannelId, row.Model, row.Endpoint, "")
		if err != nil {
			return createdCount, err
		}
		if created {
			createdCount++
		}
		seen[key] = struct{}{}
	}
	endpointRows, err := model.ListRecentDisabledChannelModelEndpointsWithDB(model.DB, limit)
	if err != nil {
		return createdCount, err
	}
	for _, row := range endpointRows {
		if strings.TrimSpace(row.DisabledBy) != "runtime" {
			continue
		}
		key := strings.TrimSpace(row.ChannelId) + "::" + strings.TrimSpace(row.Model) + "::" + model.NormalizeRequestedChannelModelEndpoint(row.Endpoint)
		if _, ok := seen[key]; ok {
			continue
		}
		created, err := EnqueueChannelModelEndpointRecoveryTest(row.ChannelId, row.Model, row.Endpoint, "")
		if err != nil {
			return createdCount, err
		}
		if created {
			createdCount++
		}
		seen[key] = struct{}{}
	}
	return createdCount, nil
}

func validateChannelModelTestEndpointAgainstProvider(row model.ChannelModel, endpoint string) error {
	normalizedEndpoint := model.NormalizeRequestedChannelModelEndpoint(endpoint)
	if normalizedEndpoint == "" {
		return fmt.Errorf("模型测试端点无效")
	}
	provider := model.NormalizeGroupModelProviderValue(row.Provider)
	if provider == "" {
		providerByModel, err := model.LoadUniqueProviderMapByModelsWithDB(model.DB, []string{row.Model, row.UpstreamModel})
		if err != nil {
			return err
		}
		provider = model.ResolveProviderFromModelMap(providerByModel, row.UpstreamModel, row.Model)
	}
	displayModel := strings.TrimSpace(row.UpstreamModel)
	if displayModel == "" {
		displayModel = strings.TrimSpace(row.Model)
	}
	if provider == "" {
		return fmt.Errorf("模型 %s 缺少供应商官方信息，不能测试端点 %s", displayModel, normalizedEndpoint)
	}
	endpointMap, err := model.LoadProviderModelEndpointMapByModelsWithDB(model.DB, provider, []string{row.Model, row.UpstreamModel})
	if err != nil {
		return err
	}
	candidates := model.NormalizeProviderLookupCandidates(row.Model, row.UpstreamModel)
	for _, candidate := range candidates {
		for _, allowedEndpoint := range endpointMap[candidate] {
			if model.NormalizeRequestedChannelModelEndpoint(allowedEndpoint) == normalizedEndpoint {
				return nil
			}
		}
	}
	return fmt.Errorf("模型 %s 的供应商官方端点范围不包含 %s", displayModel, normalizedEndpoint)
}

func CreateChannelRefreshModelsTask(channelID string, createdBy string, traceID string) (model.AsyncTask, bool, error) {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return model.AsyncTask{}, false, fmt.Errorf("渠道 ID 无效")
	}
	if _, err := channelsvc.GetByID(normalizedChannelID); err != nil {
		return model.AsyncTask{}, false, err
	}
	task, reused, err := model.CreateOrReuseAsyncTaskWithDB(model.DB, model.AsyncTask{
		Type:      model.AsyncTaskTypeChannelRefreshModels,
		DedupeKey: buildChannelRefreshModelsTaskDedupeKey(normalizedChannelID),
		ChannelId: normalizedChannelID,
		Payload: marshalJSONForLog(channelRefreshModelsTaskPayload{
			ChannelID: normalizedChannelID,
		}),
		CreatedBy: strings.TrimSpace(createdBy),
		TraceID:   strings.TrimSpace(traceID),
	})
	return task, reused, err
}

func CreateChannelRefreshBillingTask(channelID string, createdBy string, traceID string) (model.AsyncTask, bool, error) {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return model.AsyncTask{}, false, fmt.Errorf("渠道 ID 无效")
	}
	if _, err := channelsvc.GetByID(normalizedChannelID); err != nil {
		return model.AsyncTask{}, false, err
	}
	task, reused, err := model.CreateOrReuseAsyncTaskWithDB(model.DB, model.AsyncTask{
		Type:      model.AsyncTaskTypeChannelRefreshBilling,
		DedupeKey: buildChannelRefreshBillingTaskDedupeKey(normalizedChannelID),
		ChannelId: normalizedChannelID,
		Payload: marshalJSONForLog(channelRefreshBillingTaskPayload{
			ChannelID: normalizedChannelID,
		}),
		CreatedBy: strings.TrimSpace(createdBy),
		TraceID:   strings.TrimSpace(traceID),
	})
	return task, reused, err
}

func ExecuteAsyncTask(ctx context.Context, task *model.AsyncTask) (string, error) {
	if task == nil {
		return "", fmt.Errorf("任务不能为空")
	}
	switch model.NormalizeAsyncTaskType(task.Type) {
	case model.AsyncTaskTypeChannelModelTest:
		return executeChannelModelTestTask(ctx, task)
	case model.AsyncTaskTypeChannelRefreshModels:
		return executeChannelRefreshModelsTask(task)
	case model.AsyncTaskTypeChannelRefreshBilling:
		return executeChannelRefreshBillingTask(task)
	default:
		return "", fmt.Errorf("暂不支持的任务类型: %s", task.Type)
	}
}

func executeChannelModelTestTask(ctx context.Context, task *model.AsyncTask) (string, error) {
	payload := channelModelTestTaskPayload{}
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", err
	}
	channelID := strings.TrimSpace(payload.ChannelID)
	if channelID == "" {
		channelID = strings.TrimSpace(task.ChannelId)
	}
	modelID := strings.TrimSpace(payload.Model)
	if channelID == "" || modelID == "" {
		return "", fmt.Errorf("模型测试任务参数无效")
	}
	channelRow, _, err := loadChannelSyncState("", "", "", channelID, nil, nil, nil, modelID)
	if err != nil {
		return "", err
	}
	targetRows := resolveChannelTestTargetModels(channelRow, channelModelTestModeSingle, modelID, []string{modelID})
	if len(targetRows) == 0 {
		return "", fmt.Errorf("未找到待测试模型")
	}
	row := targetRows[0]
	if endpoint := strings.TrimSpace(payload.Endpoint); endpoint != "" {
		row.Endpoint = endpoint
	}
	testResult, execution := runSingleChannelModelTestWithContextAndStream(ctx, channelRow, row, payload.IsStream, payload.AudioLanguage, imageEditTestInput{
		URL:     payload.ImageEditURL,
		DataURI: payload.ImageEditData,
	}, payload.ResponsesTestMode)
	testResult.ChannelId = channelID
	persistChannelTestArtifactForExecution(ctx, task.Id, &testResult, &execution)
	logChannelAsyncTestExecution(task, testResult, execution)
	if err := persistChannelModelTests(channelID, task.Id, []model.ChannelTest{testResult}); err != nil {
		return "", err
	}
	resultPayload := map[string]any{
		"channel_id":  channelID,
		"model":       testResult.Model,
		"endpoint":    testResult.Endpoint,
		"is_stream":   testResult.IsStream,
		"status":      testResult.Status,
		"supported":   testResult.Supported,
		"message":     testResult.Message,
		"latency_ms":  testResult.LatencyMs,
		"base_url":    execution.BaseURL,
		"request_url": execution.RequestURL,
	}
	if strings.TrimSpace(testResult.ArtifactPath) != "" {
		resultPayload["artifact_name"] = testResult.ArtifactName
		resultPayload["artifact_content_type"] = testResult.ArtifactContentType
		resultPayload["artifact_size"] = testResult.ArtifactSize
	}
	return marshalJSONForLog(resultPayload), nil
}

func executeChannelRefreshModelsTask(task *model.AsyncTask) (string, error) {
	payload := channelRefreshModelsTaskPayload{}
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", err
	}
	channelID := strings.TrimSpace(payload.ChannelID)
	if channelID == "" {
		channelID = strings.TrimSpace(task.ChannelId)
	}
	resolvedChannel, keySource, err := loadChannelSyncState("", "", "", channelID, nil, nil, nil, "")
	if err != nil {
		return "", err
	}
	baseURL := resolvedChannel.ResolveAPIBaseURL("")
	fetchedRows, fetchTrace, err := fetchChannelModelsDetailed(resolvedChannel.GetProtocol(), resolvedChannel.Key, baseURL, "")
	logChannelAsyncRefresh(task, keySource, fetchTrace, len(fetchedRows), err)
	if err != nil {
		return "", err
	}
	if len(fetchedRows) > 0 {
		if err := model.AppendMissingFetchedChannelModelsWithDB(model.DB, channelID, fetchedRows); err != nil {
			return "", err
		}
	}
	if err := model.ReplaceChannelModelSyncResultsWithDB(model.DB, channelID, resolvedChannel.GetChannelModels(), fetchedRows, task.Id); err != nil {
		return "", err
	}
	return marshalJSONForLog(map[string]any{
		"channel_id":   channelID,
		"api_base_url": resolvedChannel.ResolveAPIBaseURL(""),
		"models_url":   fetchTrace.ModelsURL,
		"count":        len(fetchedRows),
	}), nil
}

func executeChannelRefreshBillingTask(task *model.AsyncTask) (string, error) {
	payload := channelRefreshBillingTaskPayload{}
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return "", err
	}
	channelID := strings.TrimSpace(payload.ChannelID)
	if channelID == "" {
		channelID = strings.TrimSpace(task.ChannelId)
	}
	channelRow, err := channelsvc.GetByID(channelID)
	if err != nil {
		return "", err
	}
	profile, err := model.GetChannelBillingProfileByChannelIDWithDB(model.DB, channelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("渠道账务未配置")
		}
		return "", err
	}
	primaryAmount, err := refreshAndPersistChannelBillingEntitlements(channelRow, profile, "自动刷新账务")
	if err != nil {
		return "", err
	}
	return marshalJSONForLog(map[string]any{
		"channel_id":           channelID,
		"billing_mode":         strings.TrimSpace(profile.BillingMode),
		"billing_api_base_url": resolveChannelBillingAPIBaseURL(channelRow, profile),
		"account_portal_url":   channelRow.ResolveAccountBaseURL(),
		"billing_request_urls": resolveChannelBillingRequestURLs(channelRow),
		"primary_amount":       primaryAmount,
	}), nil
}

func logChannelAsyncTestExecution(task *model.AsyncTask, result model.ChannelTest, execution channelModelTestExecution) {
	ctx := context.Background()
	if traceID := strings.TrimSpace(task.TraceID); traceID != "" {
		ctx = helper.SetTraceID(ctx, traceID)
	}
	fields := []string{
		"[channel-task]",
		"action=test_model",
		stringField("task_id", task.Id),
		stringField("channel_id", result.ChannelId),
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
	message := strings.Join(compactLogFields(fields), " ")
	if result.Supported {
		logger.Info(ctx, message)
		return
	}
	logger.Warn(ctx, message)
}

func logChannelAsyncRefresh(task *model.AsyncTask, keySource string, trace channelModelFetchTrace, count int, err error) {
	ctx := context.Background()
	if traceID := strings.TrimSpace(task.TraceID); traceID != "" {
		ctx = helper.SetTraceID(ctx, traceID)
	}
	fields := []string{
		"[channel-task]",
		"action=refresh_models",
		stringField("task_id", task.Id),
		stringField("source", keySource),
		stringField("channel_id", task.ChannelId),
		stringField("models_url", trace.ModelsURL),
		structuredPayloadField("request_payload", trace.RequestPayload),
		structuredPayloadField("response_payload", trace.ResponsePayload),
		intField("count", count),
	}
	if err != nil {
		fields = append(fields, stringField("reason", err.Error()))
		logger.Warn(ctx, strings.Join(compactLogFields(fields), " "))
		return
	}
	logger.Info(ctx, strings.Join(compactLogFields(fields), " "))
}

func compactLogFields(fields []string) []string {
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		if strings.TrimSpace(field) == "" {
			continue
		}
		result = append(result, field)
	}
	return result
}
