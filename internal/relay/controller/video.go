package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	adminmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay"
	"github.com/yeying-community/router/internal/relay/adaptor/openai"
	"github.com/yeying-community/router/internal/relay/billing"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

func getVideoRequest(c *gin.Context, _ int) (*relaymodel.VideoRequest, error) {
	videoRequest := &relaymodel.VideoRequest{}
	if err := common.UnmarshalBodyReusable(c, videoRequest); err != nil {
		return nil, err
	}
	return videoRequest, nil
}

func validateVideoRequest(request *relaymodel.VideoRequest) *relaymodel.ErrorWithStatusCode {
	if request == nil {
		return openai.ErrorWrapper(errors.New("video request is nil"), "invalid_video_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(request.Model) == "" {
		return openai.ErrorWrapper(errors.New("model is required"), "model_missing", http.StatusBadRequest)
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return openai.ErrorWrapper(errors.New("prompt is required"), "prompt_missing", http.StatusBadRequest)
	}
	return nil
}

func validateVideoStatusRequest(request *relaymodel.VideoRequest) *relaymodel.ErrorWithStatusCode {
	if request == nil {
		return openai.ErrorWrapper(errors.New("video request is nil"), "invalid_video_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(request.Model) == "" {
		return openai.ErrorWrapper(errors.New("model is required"), "model_missing", http.StatusBadRequest)
	}
	return nil
}

func videoRequestPricingAttrs(request *relaymodel.VideoRequest) map[string]string {
	attrs := make(map[string]string, 3)
	if request == nil {
		return attrs
	}
	if value := strings.TrimSpace(strings.ToLower(request.Resolution)); value != "" {
		attrs["resolution"] = value
	}
	if value := strings.TrimSpace(strings.ToLower(request.Size)); value != "" {
		attrs["size"] = value
	}
	if value := strings.TrimSpace(strings.ToLower(request.Quality)); value != "" {
		attrs["quality"] = value
	}
	return attrs
}

func getVideoBillingQuantity(request *relaymodel.VideoRequest, priceUnit string) float64 {
	normalizedUnit := strings.TrimSpace(strings.ToLower(priceUnit))
	switch normalizedUnit {
	case adminmodel.ProviderPriceUnitPerSecond:
		if request != nil && request.Seconds > 0 {
			return float64(request.Seconds)
		}
		if request != nil {
			duration := strings.TrimSpace(strings.ToLower(request.Duration))
			if strings.HasSuffix(duration, "s") {
				if seconds, err := strconv.ParseFloat(strings.TrimSuffix(duration, "s"), 64); err == nil && seconds > 0 {
					return seconds
				}
			}
		}
		return 1
	case adminmodel.ProviderPriceUnitPerMinute:
		if request != nil && request.Seconds > 0 {
			return float64(request.Seconds) / 60
		}
		if request != nil {
			duration := strings.TrimSpace(strings.ToLower(request.Duration))
			if strings.HasSuffix(duration, "s") {
				if seconds, err := strconv.ParseFloat(strings.TrimSuffix(duration, "s"), 64); err == nil && seconds > 0 {
					return seconds / 60
				}
			}
		}
		return 1
	default:
		return 1
	}
}

func resetMultipartRequestBody(c *gin.Context) error {
	requestBody, err := common.GetRequestBody(c)
	if err != nil {
		return err
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	contentType := c.Request.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		boundaryIndex := strings.Index(contentType, "boundary=")
		if boundaryIndex > 0 {
			boundary := strings.TrimSpace(contentType[boundaryIndex+len("boundary="):])
			c.Request.MultipartForm = nil
			c.Request.Form = nil
			c.Request.PostForm = nil
			reader := multipart.NewReader(bytes.NewReader(requestBody), strings.Trim(boundary, `"`))
			form, parseErr := reader.ReadForm(32 << 20)
			if parseErr == nil {
				c.Request.MultipartForm = form
			}
		}
	}
	return nil
}

type videoResponseSummary struct {
	TaskID    string
	Status    string
	ResultURL string
	RequestID string
}

func extractVideoResponseSummary(responseBody []byte, header http.Header) videoResponseSummary {
	summary := videoResponseSummary{
		RequestID: strings.TrimSpace(header.Get("x-request-id")),
	}
	if len(responseBody) == 0 {
		return summary
	}

	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return summary
	}

	summary.TaskID = firstNonEmptyString(
		asString(payload["id"]),
		asString(payload["task_id"]),
		asString(payload["video_id"]),
	)
	summary.Status = firstNonEmptyString(
		asString(payload["status"]),
		asString(payload["state"]),
	)
	summary.RequestID = firstNonEmptyString(
		summary.RequestID,
		asString(payload["request_id"]),
		asString(payload["requestId"]),
	)
	summary.ResultURL = firstNonEmptyString(
		asString(payload["url"]),
		asString(payload["result_url"]),
		asString(payload["resultUrl"]),
		extractFirstURL(payload["data"]),
		extractFirstURL(payload["output"]),
		extractFirstURL(payload["results"]),
	)
	return summary
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func extractFirstURL(value any) string {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return ""
	}
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		url := firstNonEmptyString(
			asString(obj["url"]),
			asString(obj["result_url"]),
			asString(obj["resultUrl"]),
		)
		if url != "" {
			return url
		}
	}
	return ""
}

func appendVideoSummaryToLogContent(content string, summary videoResponseSummary) string {
	parts := []string{strings.TrimSpace(content)}
	if summary.TaskID != "" {
		parts = append(parts, "video_task_id="+summary.TaskID)
	}
	if summary.Status != "" {
		parts = append(parts, "video_status="+summary.Status)
	}
	if summary.ResultURL != "" {
		parts = append(parts, "video_result_url="+summary.ResultURL)
	}
	if summary.RequestID != "" {
		parts = append(parts, "upstream_request_id="+summary.RequestID)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func persistVideoTaskMeta(meta *meta.Meta, channelName string, provider string, modelName string, summary videoResponseSummary, source string) {
	if meta == nil {
		return
	}
	taskID := strings.TrimSpace(summary.TaskID)
	if taskID == "" {
		return
	}
	_ = model.UpsertUserTaskWithDB(model.DB, model.UserTask{
		TaskID:      taskID,
		Type:        model.UserTaskTypeVideo,
		UserID:      strings.TrimSpace(meta.UserId),
		GroupID:     strings.TrimSpace(meta.Group),
		ChannelID:   strings.TrimSpace(meta.ChannelId),
		ChannelName: strings.TrimSpace(channelName),
		Model:       strings.TrimSpace(modelName),
		Provider:    strings.TrimSpace(provider),
		Status:      strings.TrimSpace(summary.Status),
		RequestID:   strings.TrimSpace(summary.RequestID),
		ResultURL:   strings.TrimSpace(summary.ResultURL),
		Source:      strings.TrimSpace(source),
	})
}

func relayVideoRawResponse(c *gin.Context, resp *http.Response, responseBody []byte) *relaymodel.ErrorWithStatusCode {
	for k, v := range resp.Header {
		if len(v) == 0 {
			continue
		}
		c.Writer.Header().Set(k, v[0])
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(c.Writer, bytes.NewReader(responseBody)); err != nil {
		return openai.ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
	}
	return nil
}

func RelayVideoHelper(c *gin.Context, relayMode int) *relaymodel.ErrorWithStatusCode {
	ctx := c.Request.Context()
	meta := meta.GetByContext(c)
	videoRequest, err := getVideoRequest(c, relayMode)
	if err != nil {
		logger.Errorf(ctx, "getVideoRequest failed: %s", err.Error())
		return openai.ErrorWrapper(err, "invalid_video_request", http.StatusBadRequest)
	}

	if strings.TrimSpace(videoRequest.Model) == "" {
		videoRequest.Model = strings.TrimSpace(c.GetString(ctxkey.RequestModel))
	}
	meta.OriginModelName = videoRequest.Model
	videoRequest.Model, _ = getMappedModelName(videoRequest.Model, meta.ModelMapping)
	meta.ActualModelName = videoRequest.Model

	if c.Request.Method == http.MethodGet {
		if bizErr := validateVideoStatusRequest(videoRequest); bizErr != nil {
			return bizErr
		}
	} else {
		if bizErr := validateVideoRequest(videoRequest); bizErr != nil {
			return bizErr
		}
	}

	adaptor := relay.GetAdaptor(meta.APIType)
	if adaptor == nil {
		return openai.ErrorWrapper(errors.New("invalid api type"), "invalid_api_type", http.StatusBadRequest)
	}
	adaptor.Init(meta)
	if requestURL, requestURLErr := adaptor.GetRequestURL(meta); requestURLErr == nil {
		c.Set(ctxkey.UpstreamURL, requestURL)
	}

	if c.Request.Method == http.MethodGet {
		resp, err := adaptor.DoRequest(c, meta, nil)
		if err != nil {
			logger.Errorf(ctx, "video status DoRequest failed: %s", err.Error())
			return openai.ErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
		}
		defer resp.Body.Close()

		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))
			return RelayErrorHandler(meta, resp)
		}
		responseSummary := extractVideoResponseSummary(responseBody, resp.Header)
		logger.Infof(
			ctx,
			"video relay status model=%s channel=%s task_id=%s status=%s result_url=%s request_id=%s",
			strings.TrimSpace(videoRequest.Model),
			strings.TrimSpace(meta.ChannelId),
			responseSummary.TaskID,
			responseSummary.Status,
			responseSummary.ResultURL,
			responseSummary.RequestID,
		)
		persistVideoTaskMeta(meta, c.GetString(ctxkey.ChannelName), "", videoRequest.Model, responseSummary, "relay_video_status")
		c.Set(ctxkey.UpstreamStatus, resp.StatusCode)
		return relayVideoRawResponse(c, resp, responseBody)
	}

	groupRatio := adminmodel.GetGroupBillingRatio(meta.Group)
	pricing, pricingErr := adminmodel.ResolveChannelModelPricing(meta.ChannelProtocol, meta.ChannelModelConfigs, videoRequest.Model)
	if pricingErr != nil {
		if groupRatio == 0 {
			pricing = adminmodel.ResolvedModelPricing{
				Model:     videoRequest.Model,
				Type:      adminmodel.InferModelType(videoRequest.Model),
				PriceUnit: adminmodel.ProviderPriceUnitPerVideo,
				Currency:  adminmodel.ProviderPriceCurrencyUSD,
				Source:    "group_free",
			}
		} else {
			return openai.ErrorWrapper(pricingErr, "model_pricing_not_configured", http.StatusServiceUnavailable)
		}
	}
	pricing = adminmodel.ResolveVideoRequestPricing(pricing, videoRequestPricingAttrs(videoRequest))

	quantity := getVideoBillingQuantity(videoRequest, pricing.PriceUnit)
	quota, err := billing.ComputeVideoQuota(quantity, pricing, groupRatio)
	if err != nil {
		return openai.ErrorWrapper(err, "calculate_video_quota_failed", http.StatusInternalServerError)
	}

	userQuota, err := model.CacheGetUserQuota(ctx, meta.UserId)
	if err != nil {
		return openai.ErrorWrapper(err, "get_user_quota_failed", http.StatusInternalServerError)
	}
	if userQuota-quota < 0 {
		return openai.ErrorWrapper(errors.New("user quota is not enough"), "insufficient_user_quota", http.StatusForbidden)
	}

	if err := resetMultipartRequestBody(c); err != nil {
		return openai.ErrorWrapper(err, "reset_video_request_body_failed", http.StatusInternalServerError)
	}
	requestBody, err := common.GetRequestBody(c)
	if err != nil {
		return openai.ErrorWrapper(err, "get_video_request_body_failed", http.StatusInternalServerError)
	}

	resp, err := adaptor.DoRequest(c, meta, bytes.NewReader(requestBody))
	if err != nil {
		logger.Errorf(ctx, "video DoRequest failed: %s", err.Error())
		return openai.ErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))
		return RelayErrorHandler(meta, resp)
	}
	responseSummary := extractVideoResponseSummary(responseBody, resp.Header)
	logger.Infof(
		ctx,
		"video relay success model=%s channel=%s task_id=%s status=%s result_url=%s request_id=%s",
		strings.TrimSpace(videoRequest.Model),
		strings.TrimSpace(meta.ChannelId),
		responseSummary.TaskID,
		responseSummary.Status,
		responseSummary.ResultURL,
		responseSummary.RequestID,
	)
	persistVideoTaskMeta(meta, c.GetString(ctxkey.ChannelName), pricing.Provider, videoRequest.Model, responseSummary, "relay_video_create")

	defer func(ctx context.Context) {
		if quota == 0 {
			return
		}
		if strings.TrimSpace(meta.TokenId) != "" {
			if err := model.PostConsumeTokenQuota(meta.TokenId, quota); err != nil {
				logger.SysError("error consuming token remain quota: " + err.Error())
			}
		} else {
			if err := model.DecreaseUserQuota(meta.UserId, quota); err != nil {
				logger.SysError("error consuming user quota: " + err.Error())
			}
		}
		if err := model.CacheUpdateUserQuota(ctx, meta.UserId); err != nil {
			logger.SysError("error update user quota cache: " + err.Error())
		}
		tokenName := c.GetString(ctxkey.TokenName)
		model.RecordConsumeLog(ctx, &model.Log{
			UserId:           meta.UserId,
			GroupId:          meta.Group,
			ChannelId:        meta.ChannelId,
			PromptTokens:     0,
			CompletionTokens: 0,
			ModelName:        videoRequest.Model,
			TokenName:        tokenName,
			Quota:            int(quota),
			Content:          appendVideoSummaryToLogContent(billing.FormatPricingLog(pricing, groupRatio), responseSummary),
		})
		model.UpdateUserUsedQuotaAndRequestCount(meta.UserId, quota)
		model.UpdateChannelUsedQuota(meta.ChannelId, quota)
	}(c.Request.Context())

	for k, v := range resp.Header {
		if len(v) == 0 {
			continue
		}
		c.Writer.Header().Set(k, v[0])
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(c.Writer, bytes.NewReader(responseBody)); err != nil {
		return openai.ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
	}
	c.Set(ctxkey.UpstreamStatus, resp.StatusCode)
	return relayVideoRawResponse(c, resp, responseBody)
}
