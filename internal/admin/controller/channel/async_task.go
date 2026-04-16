package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
)

type channelModelTestTaskPayload struct {
	ChannelID string `json:"channel_id"`
	Model     string `json:"model"`
	Endpoint  string `json:"endpoint"`
	IsStream  *bool  `json:"is_stream,omitempty"`
}

type channelRefreshModelsTaskPayload struct {
	ChannelID string `json:"channel_id"`
}

type channelRefreshBalanceTaskPayload struct {
	ChannelID string `json:"channel_id"`
}

func buildChannelModelTestTaskDedupeKey(channelID string, row model.ChannelModel, streamOverride *bool) string {
	modelID := strings.TrimSpace(row.Model)
	endpoint := model.NormalizeChannelModelEndpoint(resolveSelectionModelType(row), row.Endpoint)
	if streamOverride == nil {
		return fmt.Sprintf("%s:%s:%s:%s", model.AsyncTaskTypeChannelModelTest, strings.TrimSpace(channelID), modelID, endpoint)
	}
	return fmt.Sprintf(
		"%s:%s:%s:%s:%t",
		model.AsyncTaskTypeChannelModelTest,
		strings.TrimSpace(channelID),
		modelID,
		endpoint,
		*streamOverride,
	)
}

func buildChannelRefreshModelsTaskDedupeKey(channelID string) string {
	return fmt.Sprintf("%s:%s", model.AsyncTaskTypeChannelRefreshModels, strings.TrimSpace(channelID))
}

func buildChannelRefreshBalanceTaskDedupeKey(channelID string) string {
	return fmt.Sprintf("%s:%s", model.AsyncTaskTypeChannelRefreshBalance, strings.TrimSpace(channelID))
}

func buildChannelModelTestTaskPayload(row model.ChannelModel, channelID string, endpointOverride string, streamOverride *bool) string {
	endpoint := model.NormalizeChannelModelEndpoint(resolveSelectionModelType(row), endpointOverride)
	if strings.TrimSpace(endpointOverride) == "" {
		endpoint = model.NormalizeChannelModelEndpoint(resolveSelectionModelType(row), row.Endpoint)
	}
	return marshalJSONForLog(channelModelTestTaskPayload{
		ChannelID: strings.TrimSpace(channelID),
		Model:     strings.TrimSpace(row.Model),
		Endpoint:  endpoint,
		IsStream:  streamOverride,
	})
}

func CreateChannelModelTestTasks(channelID string, createdBy string, requestedTestModel string, requestedModels []string, requestedConfigs []channelModelTestTargetItem, traceID string) ([]model.AsyncTask, int, int, error) {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil, 0, 0, fmt.Errorf("渠道 ID 无效")
	}
	channelRow, err := channelsvc.GetByID(normalizedChannelID)
	if err != nil {
		return nil, 0, 0, err
	}
	testMode := channelModelTestModeBatch
	if len(requestedModels) == 1 || strings.TrimSpace(requestedTestModel) != "" {
		testMode = channelModelTestModeSingle
	}
	targetRows := resolveChannelTestTargetModels(channelRow, testMode, requestedTestModel, requestedModels)
	if len(targetRows) == 0 {
		return nil, 0, 0, fmt.Errorf("未找到可用于测试的模型")
	}
	tasks := make([]model.AsyncTask, 0, len(targetRows))
	createdCount := 0
	reusedCount := 0
	endpointOverrides := make(map[string]string, len(requestedConfigs))
	streamOverrides := make(map[string]*bool, len(requestedConfigs))
	for _, item := range requestedConfigs {
		modelID := strings.TrimSpace(item.Model)
		if modelID == "" {
			continue
		}
		endpointOverrides[modelID] = strings.TrimSpace(item.Endpoint)
		streamOverrides[modelID] = item.IsStream
	}
	for _, row := range targetRows {
		endpoint := endpointOverrides[strings.TrimSpace(row.Model)]
		if endpoint != "" {
			row.Endpoint = endpoint
		}
		stream := streamOverrides[strings.TrimSpace(row.Model)]
		normalizedEndpoint := model.NormalizeChannelModelEndpoint(resolveSelectionModelType(row), row.Endpoint)
		task, reused, err := model.CreateOrReuseAsyncTaskWithDB(model.DB, model.AsyncTask{
			Type:      model.AsyncTaskTypeChannelModelTest,
			DedupeKey: buildChannelModelTestTaskDedupeKey(normalizedChannelID, row, stream),
			ChannelId: normalizedChannelID,
			Model:     strings.TrimSpace(row.Model),
			Endpoint:  normalizedEndpoint,
			Payload:   buildChannelModelTestTaskPayload(row, normalizedChannelID, endpoint, stream),
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

func CreateChannelRefreshBalanceTask(channelID string, createdBy string, traceID string) (model.AsyncTask, bool, error) {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return model.AsyncTask{}, false, fmt.Errorf("渠道 ID 无效")
	}
	if _, err := channelsvc.GetByID(normalizedChannelID); err != nil {
		return model.AsyncTask{}, false, err
	}
	task, reused, err := model.CreateOrReuseAsyncTaskWithDB(model.DB, model.AsyncTask{
		Type:      model.AsyncTaskTypeChannelRefreshBalance,
		DedupeKey: buildChannelRefreshBalanceTaskDedupeKey(normalizedChannelID),
		ChannelId: normalizedChannelID,
		Payload: marshalJSONForLog(channelRefreshBalanceTaskPayload{
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
	case model.AsyncTaskTypeChannelRefreshBalance:
		return executeChannelRefreshBalanceTask(task)
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
	channelRow, _, err := loadChannelRuntimeState("", "", "", channelID, nil, nil, nil, modelID)
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
	testResult, execution := runSingleChannelModelTestWithContextAndStream(ctx, channelRow, row, payload.IsStream)
	testResult.ChannelId = channelID
	persistChannelTestArtifactForExecution(ctx, task.Id, &testResult, &execution)
	logChannelAsyncTestExecution(task, testResult, execution)
	if err := persistChannelModelTests(channelID, []model.ChannelTest{testResult}); err != nil {
		return "", err
	}
	resultPayload := map[string]any{
		"channel_id": channelID,
		"model":      testResult.Model,
		"endpoint":   testResult.Endpoint,
		"is_stream":  testResult.IsStream,
		"status":     testResult.Status,
		"supported":  testResult.Supported,
		"message":    testResult.Message,
		"latency_ms": testResult.LatencyMs,
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
	runtimeChannel, keySource, err := loadChannelRuntimeState("", "", "", channelID, nil, nil, nil, "")
	if err != nil {
		return "", err
	}
	baseURL := resolveChannelBaseURL(runtimeChannel.GetProtocol(), runtimeChannel.GetBaseURL())
	fetchedRows, fetchTrace, err := fetchChannelModelsDetailed(runtimeChannel.GetProtocol(), runtimeChannel.Key, baseURL, "")
	logChannelAsyncRefresh(task, keySource, fetchTrace, len(fetchedRows), err)
	if err != nil {
		return "", err
	}
	if err := model.SyncFetchedChannelModelConfigsFromBaseWithDB(model.DB, channelID, runtimeChannel.GetModelConfigs(), fetchedRows); err != nil {
		return "", err
	}
	if err := model.EnsureChannelTestModelWithDB(model.DB, channelID); err != nil {
		return "", err
	}
	return marshalJSONForLog(map[string]any{
		"channel_id": channelID,
		"models_url": fetchTrace.ModelsURL,
		"count":      len(fetchedRows),
	}), nil
}

func executeChannelRefreshBalanceTask(task *model.AsyncTask) (string, error) {
	payload := channelRefreshBalanceTaskPayload{}
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
	balance, err := updateChannelBalance(channelRow)
	if err != nil {
		return "", err
	}
	return marshalJSONForLog(map[string]any{
		"channel_id": channelID,
		"balance":    balance,
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
