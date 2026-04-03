package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
)

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
	imageRequest, err := getImageRequest(c, meta.Mode)
	if err != nil {
		logger.Errorf(ctx, "getImageRequest failed: %s", err.Error())
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

	var requestBody io.Reader
	if isModelMapped || meta.ChannelProtocol == relaychannel.Azure { // make Azure channel request body
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
		logger.Error(ctx, "calculate image billing snapshot failed: "+snapshotErr.Error())
	}
	billingPlan, quotaErr := reserveRelayQuota(ctx, meta.Group, meta.UserId, quota)
	if quotaErr != nil {
		return quotaErr
	}
	groupReservation := billingPlan.GroupReservation
	userReservation := billingPlan.UserReservation
	groupQuotaSettled := false
	userQuotaSettled := false
	defer func() {
		if !groupQuotaSettled {
			releaseGroupDailyQuotaReservation(ctx, groupReservation)
		}
		if !userQuotaSettled {
			releaseUserQuotaReservation(ctx, userReservation)
		}
	}()

	if billingPlan.ChargeUserBalance() {
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
		logger.Errorf(ctx, "DoRequest failed: %s", err.Error())
		return openai.ErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
	}

	defer func(ctx context.Context) {
		if resp != nil &&
			resp.StatusCode != http.StatusCreated && // replicate returns 201
			resp.StatusCode != http.StatusOK {
			releaseGroupDailyQuotaReservation(ctx, groupReservation)
			releaseUserQuotaReservation(ctx, userReservation)
			return
		}

		if strings.TrimSpace(meta.TokenId) != "" {
			if billingPlan.ChargeUserBalance() {
				err := model.PostConsumeTokenQuota(meta.TokenId, quota)
				if err != nil {
					logger.SysError("error consuming token remain quota: " + err.Error())
				}
			} else {
				err := model.PostConsumeTokenRemainQuota(meta.TokenId, quota)
				if err != nil {
					logger.SysError("error consuming token remain quota: " + err.Error())
				}
			}
		} else if billingPlan.ChargeUserBalance() && quota != 0 {
			if err := model.DecreaseUserQuota(meta.UserId, quota); err != nil {
				logger.SysError("error consuming user quota: " + err.Error())
			}
		}
		if billingPlan.ChargeUserBalance() {
			if err := model.CacheUpdateUserQuota(ctx, meta.UserId); err != nil {
				logger.SysError("error update user quota cache: " + err.Error())
			}
		}
		userQuotaUsage := settleUserQuotaReservation(ctx, userReservation, quota)
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
				UserDailyQuota:     int(userQuotaUsage.DailyQuotaUsed),
				UserEmergencyQuota: int(userQuotaUsage.EmergencyQuotaUsed),
				Content:            billing.FormatPricingLog(pricing, groupRatio),
			}
			billingSnapshot.ApplyToLog(entry)
			model.RecordConsumeLog(ctx, entry)
			model.UpdateUserUsedQuotaAndRequestCount(meta.UserId, quota)
			channelId := c.GetString(ctxkey.ChannelId)
			model.UpdateChannelUsedQuota(channelId, quota)
		}
		settleGroupDailyQuotaReservation(ctx, groupReservation, quota)
	}(c.Request.Context())
	groupQuotaSettled = true
	userQuotaSettled = true

	// do response
	_, respErr := adaptor.DoResponse(c, resp, meta)
	if respErr != nil {
		logger.Errorf(ctx, "respErr is not nil: %+v", respErr)
		return respErr
	}

	return nil
}
