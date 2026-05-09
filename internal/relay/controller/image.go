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
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/imagerule"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

func validateImageBillingPricing(pricing adminmodel.ResolvedModelPricing) error {
	switch strings.TrimSpace(strings.ToLower(pricing.PriceUnit)) {
	case adminmodel.ProviderPriceUnitPer1KTokens, adminmodel.ProviderPriceUnitPer1KChars:
		return fmt.Errorf("image billing strategy is not supported for model %s with price_unit %s on traditional image endpoints", strings.TrimSpace(pricing.Model), strings.TrimSpace(pricing.PriceUnit))
	default:
		return nil
	}
}

func getImageRequest(c *gin.Context, _ int) (*relaymodel.ImageRequest, error) {
	imageRequest := &relaymodel.ImageRequest{}
	err := common.UnmarshalBodyReusable(c, imageRequest)
	if err != nil {
		return nil, err
	}
	if imageRequest.N == 0 {
		imageRequest.N = 1
	}
	if imageRequest.Size == "" {
		imageRequest.Size = "1024x1024"
	}
	if imageRequest.Model == "" {
		imageRequest.Model = "dall-e-2"
	}
	return imageRequest, nil
}

func getImageEditRequest(c *gin.Context) (*relaymodel.ImageRequest, *multipart.Form, error) {
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		return nil, nil, err
	}
	form := c.Request.MultipartForm
	if form == nil {
		return nil, nil, errors.New("multipart form is required")
	}
	imageRequest := &relaymodel.ImageRequest{
		Model:          strings.TrimSpace(c.Request.FormValue("model")),
		Prompt:         strings.TrimSpace(c.Request.FormValue("prompt")),
		Size:           strings.TrimSpace(c.Request.FormValue("size")),
		Quality:        strings.TrimSpace(c.Request.FormValue("quality")),
		ResponseFormat: strings.TrimSpace(c.Request.FormValue("response_format")),
		Style:          strings.TrimSpace(c.Request.FormValue("style")),
		User:           strings.TrimSpace(c.Request.FormValue("user")),
	}
	if rawN := strings.TrimSpace(c.Request.FormValue("n")); rawN != "" {
		n, err := strconv.Atoi(rawN)
		if err != nil {
			return nil, nil, err
		}
		imageRequest.N = n
	}
	if imageRequest.N == 0 {
		imageRequest.N = 1
	}
	if len(form.File["image"]) == 0 {
		return nil, nil, errors.New("image file is required")
	}
	return imageRequest, form, nil
}

func buildMultipartImageEditBody(form *multipart.Form, imageRequest *relaymodel.ImageRequest) (*bytes.Buffer, string, error) {
	if form == nil {
		return nil, "", errors.New("multipart form is required")
	}
	if imageRequest == nil {
		return nil, "", errors.New("image request is nil")
	}
	bodyBuffer := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuffer)
	knownFields := map[string]struct{}{
		"model":           {},
		"prompt":          {},
		"n":               {},
		"size":            {},
		"quality":         {},
		"response_format": {},
		"style":           {},
		"user":            {},
	}
	writeField := func(key string, value string) error {
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return writer.WriteField(key, value)
	}
	if err := writeField("model", imageRequest.Model); err != nil {
		return nil, "", err
	}
	if err := writeField("prompt", imageRequest.Prompt); err != nil {
		return nil, "", err
	}
	if imageRequest.N > 0 {
		if err := writer.WriteField("n", strconv.Itoa(imageRequest.N)); err != nil {
			return nil, "", err
		}
	}
	if err := writeField("size", imageRequest.Size); err != nil {
		return nil, "", err
	}
	if err := writeField("quality", imageRequest.Quality); err != nil {
		return nil, "", err
	}
	if err := writeField("response_format", imageRequest.ResponseFormat); err != nil {
		return nil, "", err
	}
	if err := writeField("style", imageRequest.Style); err != nil {
		return nil, "", err
	}
	if err := writeField("user", imageRequest.User); err != nil {
		return nil, "", err
	}
	for key, values := range form.Value {
		if _, known := knownFields[key]; known {
			continue
		}
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				return nil, "", err
			}
		}
	}
	for fieldName, files := range form.File {
		for _, header := range files {
			src, err := header.Open()
			if err != nil {
				return nil, "", err
			}
			part, err := writer.CreateFormFile(fieldName, header.Filename)
			if err != nil {
				_ = src.Close()
				return nil, "", err
			}
			if _, err := io.Copy(part, src); err != nil {
				_ = src.Close()
				return nil, "", err
			}
			if err := src.Close(); err != nil {
				return nil, "", err
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return bodyBuffer, writer.FormDataContentType(), nil
}

func isValidImageSize(model string, size string) bool {
	if model == "cogview-3" || imagerule.ImageSizeRatios[model] == nil {
		return true
	}
	_, ok := imagerule.ImageSizeRatios[model][size]
	return ok
}

func isValidImagePromptLength(model string, promptLength int) bool {
	maxPromptLength, ok := imagerule.ImagePromptLengthLimitations[model]
	return !ok || promptLength <= maxPromptLength
}

func isWithinRange(element string, value int) bool {
	amounts, ok := imagerule.ImageGenerationAmounts[element]
	return !ok || (value >= amounts[0] && value <= amounts[1])
}

func getImageSizeRatio(model string, size string) float64 {
	if ratio, ok := imagerule.ImageSizeRatios[model][size]; ok {
		return ratio
	}
	return 1
}

func validateImageRequest(imageRequest *relaymodel.ImageRequest, _ *meta.Meta) *relaymodel.ErrorWithStatusCode {
	// check prompt length
	if imageRequest.Prompt == "" {
		return openai.ErrorWrapper(errors.New("prompt is required"), "prompt_missing", http.StatusBadRequest)
	}

	// model validation
	if !isValidImageSize(imageRequest.Model, imageRequest.Size) {
		return openai.ErrorWrapper(errors.New("size not supported for this image model"), "size_not_supported", http.StatusBadRequest)
	}

	if !isValidImagePromptLength(imageRequest.Model, len(imageRequest.Prompt)) {
		return openai.ErrorWrapper(errors.New("prompt is too long"), "prompt_too_long", http.StatusBadRequest)
	}

	// Number of generated images validation
	if !isWithinRange(imageRequest.Model, imageRequest.N) {
		return openai.ErrorWrapper(errors.New("invalid value of n"), "n_not_within_range", http.StatusBadRequest)
	}
	return nil
}

func getImageCostRatio(imageRequest *relaymodel.ImageRequest) (float64, error) {
	if imageRequest == nil {
		return 0, errors.New("imageRequest is nil")
	}
	imageCostRatio := getImageSizeRatio(imageRequest.Model, imageRequest.Size)
	if imageRequest.Quality == "hd" && imageRequest.Model == "dall-e-3" {
		if imageRequest.Size == "1024x1024" {
			imageCostRatio *= 2
		} else {
			imageCostRatio *= 1.5
		}
	}
	return imageCostRatio, nil
}

func RelayImageHelper(c *gin.Context, relayMode int) *relaymodel.ErrorWithStatusCode {
	ctx := c.Request.Context()
	meta := meta.GetByContext(c)
	var (
		imageRequest *relaymodel.ImageRequest
		form         *multipart.Form
		err          error
	)
	if relayMode == relaymode.ImagesEdits {
		imageRequest, form, err = getImageEditRequest(c)
	} else {
		imageRequest, err = getImageRequest(c, meta.Mode)
	}
	if err != nil {
		logger.Errorf(ctx, "image relay get request failed user_id=%s group=%s channel_id=%s endpoint=%s err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), c.Request.URL.Path, err.Error())
		return openai.ErrorWrapper(err, "invalid_image_request", http.StatusBadRequest)
	}

	// map model name
	var isModelMapped bool
	meta.OriginModelName = imageRequest.Model
	imageRequest.Model, isModelMapped = getMappedModelName(imageRequest.Model, meta.ModelMapping)
	meta.ActualModelName = imageRequest.Model

	// model validation
	bizErr := validateImageRequest(imageRequest, meta)
	if bizErr != nil {
		return bizErr
	}

	imageCostRatio, err := getImageCostRatio(imageRequest)
	if err != nil {
		return openai.ErrorWrapper(err, "get_image_cost_ratio_failed", http.StatusInternalServerError)
	}

	imageModel := imageRequest.Model
	// Convert the original image model
	imageRequest.Model, _ = getMappedModelName(imageRequest.Model, imagerule.ImageOriginModelName)
	c.Set("response_format", imageRequest.ResponseFormat)
	groupRatio := adminmodel.GetGroupBillingRatio(meta.Group)
	pricing, pricingErr := adminmodel.ResolveChannelModelPricing(meta.ChannelProtocol, meta.ChannelModelConfigs, imageModel)
	if pricingErr != nil {
		if groupRatio == 0 {
			pricing = adminmodel.ResolvedModelPricing{
				Model:     imageModel,
				Type:      adminmodel.InferModelType(imageModel),
				PriceUnit: adminmodel.ProviderPriceUnitPerImage,
				Currency:  adminmodel.ProviderPriceCurrencyUSD,
				Source:    "group_free",
			}
		} else {
			return openai.ErrorWrapper(pricingErr, "model_pricing_not_configured", http.StatusServiceUnavailable)
		}
	}
	pricing = adminmodel.ResolveImageRequestPricing(pricing, imageRequest.Size, imageRequest.Quality)
	if pricing.MatchedComponent != "" {
		imageCostRatio = 1
	}
	if billingErr := validateImageBillingPricing(pricing); billingErr != nil {
		return openai.ErrorWrapper(billingErr, "unsupported_image_billing", http.StatusServiceUnavailable)
	}

	var requestBody io.Reader
	if relayMode == relaymode.ImagesEdits {
		requestBodyBuffer, contentType, buildErr := buildMultipartImageEditBody(form, imageRequest)
		if buildErr != nil {
			return openai.ErrorWrapper(buildErr, "marshal_image_request_failed", http.StatusInternalServerError)
		}
		c.Request.Header.Set("Content-Type", contentType)
		requestBody = bytes.NewBuffer(requestBodyBuffer.Bytes())
	} else if isModelMapped || meta.ChannelProtocol == relaychannel.Azure { // make Azure channel request body
		jsonStr, err := json.Marshal(imageRequest)
		if err != nil {
			return openai.ErrorWrapper(err, "marshal_image_request_failed", http.StatusInternalServerError)
		}
		requestBody = bytes.NewBuffer(jsonStr)
	} else {
		requestBody = c.Request.Body
	}

	adaptor := relay.GetAdaptor(meta.APIType)
	if adaptor == nil {
		return openai.ErrorWrapper(fmt.Errorf("invalid api type: %d", meta.APIType), "invalid_api_type", http.StatusBadRequest)
	}
	adaptor.Init(meta)

	// these adaptors need to convert the request
	if relayMode != relaymode.ImagesEdits {
		switch meta.ChannelProtocol {
		case relaychannel.Zhipu,
			relaychannel.Ali,
			relaychannel.Replicate,
			relaychannel.Baidu:
			finalRequest, err := adaptor.ConvertImageRequest(imageRequest)
			if err != nil {
				return openai.ErrorWrapper(err, "convert_image_request_failed", http.StatusInternalServerError)
			}
			jsonStr, err := json.Marshal(finalRequest)
			if err != nil {
				return openai.ErrorWrapper(err, "marshal_image_request_failed", http.StatusInternalServerError)
			}
			requestBody = bytes.NewBuffer(jsonStr)
		}
	}

	imageCount := imageRequest.N
	if meta.ChannelProtocol == relaychannel.Replicate {
		imageCount = 1
	}
	quota, err := billing.ComputeImageQuota(imageCount, imageCostRatio, pricing, groupRatio)
	if err != nil {
		return openai.ErrorWrapper(err, "calculate_image_quota_failed", http.StatusInternalServerError)
	}
	billingSnapshot, snapshotErr := billing.ComputeImageBillingSnapshot(imageCount, imageCostRatio, pricing, groupRatio)
	if snapshotErr != nil {
		logger.Errorf(ctx, "image billing snapshot failed user_id=%s group=%s channel_id=%s model=%s image_count=%d err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), imageCount, snapshotErr.Error())
	}
	billingPlan, quotaErr := reserveRelayQuota(ctx, meta.Group, meta.UserId, quota)
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

	if billingPlan.ChargeUserBalance() {
		if _, expireErr := model.ExpireUserBalanceLots(meta.UserId); expireErr != nil {
			logger.Errorf(ctx, "image billing expire lots failed user_id=%s group=%s channel_id=%s model=%s err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), expireErr.Error())
		}
		userQuota, err := model.CacheGetUserQuota(ctx, meta.UserId)
		if err != nil {
			return openai.ErrorWrapper(err, "get_user_quota_failed", http.StatusInternalServerError)
		}
		if userQuota-quota < 0 {
			return openai.ErrorWrapper(errors.New("user quota is not enough"), "insufficient_user_quota", http.StatusForbidden)
		}
	}

	// do request
	resp, err := adaptor.DoRequest(c, meta, requestBody)
	if err != nil {
		return openai.ErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
	}

	defer func(ctx context.Context) {
		if resp != nil &&
			resp.StatusCode != http.StatusCreated && // replicate returns 201
			resp.StatusCode != http.StatusOK {
			releasePackageQuotaReservation(ctx, packageReservation)
			return
		}
		userDailyQuota := 0
		userEmergencyQuota := 0
		if !billingPlan.ChargeUserBalance() {
			dailyConsumed, emergencyConsumed := settlePackageQuotaReservation(ctx, packageReservation, quota)
			userDailyQuota = int(dailyConsumed)
			userEmergencyQuota = int(emergencyConsumed)
		}

		if strings.TrimSpace(meta.TokenId) != "" {
			if billingPlan.ChargeUserBalance() {
				err := model.PostConsumeTokenQuota(meta.TokenId, quota)
				if err != nil {
					logger.Errorf(ctx, "image billing failed code=post_consume_token_quota_failed user_id=%s group=%s channel_id=%s model=%s token_id=%s quota=%d charge_user_balance=%t err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), strings.TrimSpace(meta.TokenId), quota, billingPlan.ChargeUserBalance(), err.Error())
				}
			} else {
				err := model.PostConsumeTokenRemainQuota(meta.TokenId, quota)
				if err != nil {
					logger.Errorf(ctx, "image billing failed code=post_consume_token_remain_quota_failed user_id=%s group=%s channel_id=%s model=%s token_id=%s quota=%d charge_user_balance=%t err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), strings.TrimSpace(meta.TokenId), quota, billingPlan.ChargeUserBalance(), err.Error())
				}
			}
		} else if billingPlan.ChargeUserBalance() && quota != 0 {
			if err := model.DecreaseUserQuota(meta.UserId, quota); err != nil {
				logger.Errorf(ctx, "image billing failed code=post_consume_user_quota_failed user_id=%s group=%s channel_id=%s model=%s quota=%d charge_user_balance=%t err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), quota, billingPlan.ChargeUserBalance(), err.Error())
			}
		}
		if billingPlan.ChargeUserBalance() {
			if err := model.CacheUpdateUserQuota(ctx, meta.UserId); err != nil {
				logger.Errorf(ctx, "image billing failed code=update_user_quota_cache_failed user_id=%s group=%s channel_id=%s model=%s quota=%d charge_user_balance=%t err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), quota, billingPlan.ChargeUserBalance(), err.Error())
			}
			if quota > 0 {
				consumedFromLots, consumeErr := model.ConsumeUserBalanceLots(meta.UserId, quota)
				if consumeErr != nil {
					logger.Errorf(ctx, "image billing lots consume failed user_id=%s group=%s channel_id=%s model=%s quota=%d err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), quota, consumeErr.Error())
				} else if consumedFromLots < quota {
					logger.Warnf(ctx, "image billing lots consume partial user_id=%s group=%s channel_id=%s model=%s consumed=%d requested=%d", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), consumedFromLots, quota)
				}
			}
		}
		if quota != 0 {
			tokenName := c.GetString(ctxkey.TokenName)
			billingSnapshot.YYCAmount = quota
			entry := &model.Log{
				UserId:             meta.UserId,
				GroupId:            meta.Group,
				ChannelId:          meta.ChannelId,
				PromptTokens:       0,
				CompletionTokens:   0,
				ModelName:          imageRequest.Model,
				TokenName:          tokenName,
				Quota:              int(quota),
				BillingSource:      model.ResolveConsumeLogBillingSource(billingPlan.ChargeUserBalance()),
				UserDailyQuota:     userDailyQuota,
				UserEmergencyQuota: userEmergencyQuota,
				Content:            billing.FormatPricingLog(pricing, groupRatio),
			}
			billingSnapshot.ApplyToLog(entry)
			model.RecordConsumeLog(ctx, entry)
			model.UpdateUserUsedQuotaAndRequestCount(meta.UserId, quota)
			channelId := c.GetString(ctxkey.ChannelId)
			model.UpdateChannelUsedQuota(channelId, quota)
		}
	}(c.Request.Context())
	groupQuotaSettled = true

	// do response
	_, respErr := adaptor.DoResponse(c, resp, meta)
	if respErr != nil {
		logger.Errorf(ctx, "image relay response failed user_id=%s group=%s channel_id=%s model=%s endpoint=%s err=%+v", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), c.Request.URL.Path, respErr)
		return respErr
	}

	return nil
}
