package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
	adminmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay"
	"github.com/yeying-community/router/internal/relay/adaptor"
	"github.com/yeying-community/router/internal/relay/adaptor/openai"
	"github.com/yeying-community/router/internal/relay/apitype"
	"github.com/yeying-community/router/internal/relay/billing"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/meta"
	"github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
	"github.com/yeying-community/router/internal/tokenestimate"
)

func logTextStreamAcceptConflict(c *gin.Context, meta *meta.Meta) {
	if c == nil || c.Request == nil || meta == nil {
		return
	}
	accept := strings.ToLower(strings.TrimSpace(c.Request.Header.Get("Accept")))
	if accept == "" {
		return
	}
	const sse = "text/event-stream"
	hasSSEAccept := strings.Contains(accept, sse)
	if meta.IsStream == hasSSEAccept {
		return
	}
	logger.Warnf(
		c.Request.Context(),
		"[text_accept_conflict] path=%s stream=%t accept=%q resolved_accept=%q user_id=%s group=%s channel_id=%s origin_model=%s actual_model=%s",
		strings.TrimSpace(c.Request.URL.Path),
		meta.IsStream,
		strings.TrimSpace(c.Request.Header.Get("Accept")),
		map[bool]string{true: sse, false: "application/json"}[meta.IsStream],
		strings.TrimSpace(meta.UserId),
		strings.TrimSpace(meta.Group),
		strings.TrimSpace(meta.ChannelId),
		strings.TrimSpace(meta.OriginModelName),
		strings.TrimSpace(meta.ActualModelName),
	)
}

func RelayTextHelper(c *gin.Context) *model.ErrorWithStatusCode {
	ctx := c.Request.Context()
	meta := meta.GetByContext(c)
	// get & validate textRequest
	textRequest, validatedRawBody, err := getAndValidateTextRequest(c, meta.Mode)
	if err != nil {
		logger.Errorf(ctx, "getAndValidateTextRequest failed: %s", err.Error())
		return openai.ErrorWrapper(err, "invalid_text_request", http.StatusBadRequest)
	}
	if err := validateProviderSpecificTextRequest(meta, textRequest, validatedRawBody); err != nil {
		logger.Errorf(ctx, "validateProviderSpecificTextRequest failed: %s", err.Error())
		return openai.ErrorWrapper(err, "invalid_text_request", http.StatusBadRequest)
	}
	meta.IsStream = textRequest.Stream
	logTextStreamAcceptConflict(c, meta)

	// map model name
	meta.OriginModelName = textRequest.Model
	textRequest.Model, _ = getMappedModelName(textRequest.Model, meta.ModelMapping)
	meta.ActualModelName = textRequest.Model
	upstreamMode, upstreamPath, err := resolveChannelTextUpstream(meta, meta.OriginModelName, textRequest.Model)
	if err != nil {
		return openai.ErrorWrapper(err, "unsupported_channel_endpoint", http.StatusBadRequest)
	}
	meta.UpstreamMode = upstreamMode
	meta.UpstreamRequestPath = upstreamPath
	meta.EndpointPolicy = adminmodel.CacheGetChannelModelEndpointPolicy(meta.ChannelId, upstreamPath, meta.OriginModelName, textRequest.Model)
	if config.DebugEnabled {
		logger.Debugf(
			ctx,
			"[text_route] downstream=%s upstream=%s api_type=%d channel_protocol=%d origin_model=%s actual_model=%s",
			relayModeLabel(meta.Mode),
			relayModeLabel(upstreamMode),
			meta.APIType,
			meta.ChannelProtocol,
			strings.TrimSpace(meta.OriginModelName),
			strings.TrimSpace(meta.ActualModelName),
		)
	}
	groupRatio := adminmodel.GetGroupChannelBillingRatio(meta.Group, meta.ChannelId)
	pricing, err := adminmodel.ResolveChannelModelPricing(meta.ChannelProtocol, meta.ChannelModelConfigs, textRequest.Model)
	if err != nil {
		if groupRatio == 0 {
			pricing = adminmodel.ResolvedModelPricing{
				Model:     textRequest.Model,
				Type:      adminmodel.InferModelType(textRequest.Model),
				PriceUnit: adminmodel.ProviderPriceUnitPer1KTokens,
				Currency:  adminmodel.ProviderPriceCurrencyUSD,
				Source:    "group_free",
			}
		} else {
			logger.Errorf(ctx, "ResolveChannelModelPricing failed: %s", err.Error())
			return openai.ErrorWrapper(err, "model_pricing_not_configured", http.StatusServiceUnavailable)
		}
	}
	pricing = adminmodel.ResolveTextRequestPricing(pricing, upstreamPath)
	// pre-consume quota
	rawRequestBody := validatedRawBody
	if len(rawRequestBody) == 0 {
		var rawBodyErr error
		rawRequestBody, rawBodyErr = common.GetRequestBody(c)
		if rawBodyErr != nil {
			logger.Errorf(ctx, "get request body for token estimate failed: %s", rawBodyErr.Error())
			return openai.ErrorWrapper(rawBodyErr, "read_request_body_failed", http.StatusBadRequest)
		}
	}
	estimateResult, estimateErr := tokenestimate.Estimate(tokenestimate.EstimateRequest{
		RelayMode: meta.Mode,
		Model:     meta.OriginModelName,
		RawBody:   rawRequestBody,
		Request:   textRequest,
	})
	if estimateErr != nil {
		logger.Errorf(ctx, "estimate prompt tokens failed: %s", estimateErr.Error())
		return openai.ErrorWrapper(estimateErr, "estimate_prompt_tokens_failed", http.StatusInternalServerError)
	}
	promptTokens := estimateResult.PromptTokens
	meta.PromptTokens = promptTokens
	logger.Debugf(
		ctx,
		"[prompt_token_estimate] estimator=%s source=%s precision=%s tokens=%d model=%s path=%s",
		strings.TrimSpace(estimateResult.Estimator),
		strings.TrimSpace(estimateResult.Source),
		string(estimateResult.Precision),
		promptTokens,
		strings.TrimSpace(meta.OriginModelName),
		strings.TrimSpace(c.Request.URL.Path),
	)
	responsesImageTools, responsesImageToolsErr := parseResponsesImageToolSpecs(rawRequestBody)
	if responsesImageToolsErr != nil {
		logger.Errorf(ctx, "parse responses image tools failed: %s", responsesImageToolsErr.Error())
		return openai.ErrorWrapper(responsesImageToolsErr, "parse_responses_image_tools_failed", http.StatusBadRequest)
	}
	preConsumedSnapshot, err := billing.ComputeTextPreConsumedBillingSnapshot(promptTokens, textRequest.MaxTokens, pricing, groupRatio)
	if err != nil {
		logger.Errorf(ctx, "ComputeTextPreConsumedQuota failed: %s", err.Error())
		return openai.ErrorWrapper(err, "calculate_text_quota_failed", http.StatusInternalServerError)
	}
	if err := billing.ApplyEstimatedProcurementCostFloor(&preConsumedSnapshot, meta.ChannelId, meta.ActualModelName); err != nil {
		logger.Errorf(ctx, "estimate procurement cost for text pre-consume failed: %s", err.Error())
		return openai.ErrorWrapper(err, "calculate_text_quota_failed", http.StatusInternalServerError)
	}
	groupReservedQuota := preConsumedSnapshot.ChargeAmount
	billingPlan, quotaErr := reserveRelayQuota(ctx, meta.Group, meta.UserId, groupReservedQuota)
	if quotaErr != nil {
		return quotaErr
	}
	packageReservation := billingPlan.PackageReservation
	groupQuotaSettled := false
	defer func() {
		if !groupQuotaSettled {
			releasePackageQuotaReservation(ctx, packageReservation)
		}
	}()
	preConsumedQuota, bizErr := preConsumeQuota(ctx, groupReservedQuota, meta, billingPlan.ChargeUserBalance())
	if bizErr != nil {
		logger.Warnf(ctx, "preConsumeQuota failed: %+v", *bizErr)
		return bizErr
	}
	preConsumedQuotaSettled := false
	defer func() {
		if !preConsumedQuotaSettled && preConsumedQuota > 0 {
			billing.ReturnPreConsumedQuota(ctx, preConsumedQuota, meta.TokenId, meta.UserId, billingPlan.ChargeUserBalance())
		}
	}()

	upstreamRequest, err := convertTextRequestForUpstream(textRequest, meta.Mode, upstreamMode)
	if err != nil {
		return openai.ErrorWrapper(err, "convert_request_failed", http.StatusBadRequest)
	}

	adaptor := relay.GetAdaptor(meta.APIType)
	if adaptor == nil {
		return openai.ErrorWrapper(fmt.Errorf("invalid api type: %d", meta.APIType), "invalid_api_type", http.StatusBadRequest)
	}
	adaptor.Init(meta)

	// get request body
	requestBody, err := getRequestBody(c, meta, upstreamRequest, adaptor, rawRequestBody)
	if err != nil {
		var policyErr *endpointPolicyError
		if errors.As(err, &policyErr) {
			return openai.ErrorWrapper(policyErr, policyErr.ErrorCode(), policyErr.StatusCode())
		}
		return openai.ErrorWrapper(err, "convert_request_failed", http.StatusInternalServerError)
	}

	// do request
	resp, err := adaptor.DoRequest(c, meta, requestBody)
	if err != nil {
		return openai.ErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
	}
	if isErrorHappened(meta, resp) {
		return RelayErrorHandler(meta, resp)
	}

	// do response
	usage, respErr := adaptor.DoResponse(c, resp, meta)
	if respErr != nil {
		return respErr
	}
	// post-consume quota
	go postConsumeQuota(ctx, usage, meta, upstreamRequest, pricing, preConsumedQuota, groupRatio, estimateResult, responsesImageTools, false, billingPlan.ChargeUserBalance(), packageReservation)
	preConsumedQuotaSettled = true
	groupQuotaSettled = true
	return nil
}

func getRequestBody(c *gin.Context, meta *meta.Meta, textRequest *model.GeneralOpenAIRequest, adaptor adaptor.Adaptor, rawRequestBody []byte) (io.Reader, error) {
	upstreamMode := meta.Mode
	if meta.UpstreamMode != 0 {
		upstreamMode = meta.UpstreamMode
	}
	if meta.Mode == relaymode.Messages && upstreamMode == relaymode.Messages {
		rawBody := rawRequestBody
		if len(rawBody) == 0 {
			var err error
			rawBody, err = common.GetRequestBody(c)
			if err != nil {
				return nil, err
			}
		}
		jsonData, err := normalizeMessagesRequestBody(rawBody, meta.ActualModelName)
		if err != nil {
			return nil, err
		}
		jsonData, err = applyEndpointRequestPolicy(c, meta, jsonData)
		if err != nil {
			return nil, err
		}
		logger.Debugf(
			c.Request.Context(),
			"[messages_body] len=%d model=%s stream=%t",
			len(jsonData),
			strings.TrimSpace(meta.ActualModelName),
			meta.IsStream,
		)
		if config.DebugEnabled {
			logger.Debugf(
				c.Request.Context(),
				"[upstream_request_body] downstream=%s upstream=%s body=%s",
				relayModeLabel(meta.Mode),
				relayModeLabel(upstreamMode),
				sanitizePayloadForRelayDebug(jsonData),
			)
		}
		return bytes.NewBuffer(jsonData), nil
	}
	if meta.Mode == relaymode.Responses && upstreamMode == relaymode.Responses {
		rawBody, err := common.GetRequestBody(c)
		if err != nil {
			return nil, err
		}
		rawBody, err = applyEndpointRequestPolicy(c, meta, rawBody)
		if err != nil {
			return nil, err
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))
		if config.DebugEnabled {
			logger.Debugf(
				c.Request.Context(),
				"[responses_body] len=%d model=%s stream=%t",
				len(rawBody),
				strings.TrimSpace(meta.ActualModelName),
				meta.IsStream,
			)
			logger.Debugf(
				c.Request.Context(),
				"[upstream_request_body] downstream=%s upstream=%s body=%s",
				relayModeLabel(meta.Mode),
				relayModeLabel(upstreamMode),
				sanitizePayloadForRelayDebug(rawBody),
			)
		}
		return c.Request.Body, nil
	}
	if !config.EnforceIncludeUsage &&
		meta.APIType == apitype.OpenAI &&
		meta.OriginModelName == meta.ActualModelName &&
		meta.ChannelProtocol != relaychannel.Baichuan &&
		meta.Mode == upstreamMode &&
		meta.Mode != relaymode.Messages {
		// no need to convert request for openai
		rawBody, err := common.GetRequestBody(c)
		if err != nil {
			return nil, err
		}
		rawBody, err = applyEndpointRequestPolicy(c, meta, rawBody)
		if err != nil {
			return nil, err
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))
		return c.Request.Body, nil
	}

	// get request body
	var requestBody io.Reader
	convertedRequest, err := adaptor.ConvertRequest(c, upstreamMode, textRequest)
	if err != nil {
		logger.Debugf(c.Request.Context(), "converted request failed: %s\n", err.Error())
		return nil, err
	}
	jsonData, err := json.Marshal(convertedRequest)
	if err != nil {
		logger.Debugf(c.Request.Context(), "converted request json_marshal_failed: %s\n", err.Error())
		return nil, err
	}
	jsonData, err = applyEndpointRequestPolicy(c, meta, jsonData)
	if err != nil {
		return nil, err
	}
	if config.DebugEnabled {
		logger.Debugf(
			c.Request.Context(),
			"[converted_request_body] downstream=%s upstream=%s len=%d body=%s",
			relayModeLabel(meta.Mode),
			relayModeLabel(upstreamMode),
			len(jsonData),
			sanitizePayloadForRelayDebug(jsonData),
		)
	}
	requestBody = bytes.NewBuffer(jsonData)
	return requestBody, nil
}

func relayModeLabel(mode int) string {
	switch mode {
	case relaymode.ChatCompletions:
		return "chat_completions"
	case relaymode.Messages:
		return "messages"
	case relaymode.Responses:
		return "responses"
	case relaymode.Completions:
		return "completions"
	case relaymode.Embeddings:
		return "embeddings"
	case relaymode.Moderations:
		return "moderations"
	case relaymode.ImagesGenerations:
		return "images_generations"
	case relaymode.Edits:
		return "edits"
	case relaymode.AudioSpeech:
		return "audio_speech"
	case relaymode.AudioTranslation:
		return "audio_translation"
	case relaymode.AudioTranscription:
		return "audio_transcription"
	case relaymode.Realtime:
		return "realtime"
	case relaymode.Videos:
		return "videos"
	default:
		return "unknown"
	}
}
