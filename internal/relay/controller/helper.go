package controller

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/yeying-community/router/common/helper"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/adaptor/anthropic"
	"github.com/yeying-community/router/internal/relay/adaptor/openai"
	"github.com/yeying-community/router/internal/relay/billing"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/controller/validator"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
	"github.com/yeying-community/router/internal/tokenestimate"
)

func getAndValidateTextRequest(c *gin.Context, relayMode int) (*relaymodel.GeneralOpenAIRequest, []byte, error) {
	var (
		textRequest *relaymodel.GeneralOpenAIRequest
		rawBody     []byte
		err         error
	)
	if relayMode == relaymode.Messages {
		requestBody, getErr := common.GetRequestBody(c)
		if getErr != nil {
			return nil, nil, getErr
		}
		rawBody = append([]byte(nil), requestBody...)
		textRequest, err = anthropic.ParseMessagesRequestToRelayRequest(requestBody)
		if err != nil {
			return nil, rawBody, err
		}
	} else {
		textRequest = &relaymodel.GeneralOpenAIRequest{}
		err = common.UnmarshalBodyReusable(c, textRequest)
		if err != nil {
			return nil, nil, err
		}
	}
	if relayMode == relaymode.Moderations && textRequest.Model == "" {
		textRequest.Model = "text-moderation-latest"
	}
	if relayMode == relaymode.Embeddings && textRequest.Model == "" {
		textRequest.Model = c.Param("model")
	}
	if relayMode != relaymode.Messages {
		err = validator.ValidateTextRequest(textRequest, relayMode)
		if err != nil {
			return nil, rawBody, err
		}
	}
	return textRequest, rawBody, nil
}

func getPreConsumedQuota(textRequest *relaymodel.GeneralOpenAIRequest, promptTokens int, ratio float64) int64 {
	preConsumedTokens := config.PreConsumedQuota + int64(promptTokens)
	if textRequest.MaxTokens != 0 {
		preConsumedTokens += int64(textRequest.MaxTokens)
	}
	return int64(float64(preConsumedTokens) * ratio)
}

func preConsumeQuota(ctx context.Context, textRequest *relaymodel.GeneralOpenAIRequest, promptTokens int, pricing model.ResolvedModelPricing, groupRatio float64, meta *meta.Meta, chargeUserBalance bool) (int64, *relaymodel.ErrorWithStatusCode) {
	preConsumedQuota, err := billing.ComputeTextPreConsumedQuota(promptTokens, textRequest.MaxTokens, pricing, groupRatio)
	if err != nil {
		return 0, openai.ErrorWrapper(err, "calculate_text_quota_failed", http.StatusInternalServerError)
	}
	if !chargeUserBalance {
		if strings.TrimSpace(meta.TokenId) == "" {
			return 0, nil
		}
		err = model.PreConsumeTokenRemainQuota(meta.TokenId, preConsumedQuota)
		if err != nil {
			return preConsumedQuota, openai.ErrorWrapper(err, "pre_consume_token_quota_failed", http.StatusForbidden)
		}
		return preConsumedQuota, nil
	}
	if _, expireErr := model.ExpireUserBalanceLots(meta.UserId); expireErr != nil {
		logger.Error(ctx, "expire user balance lots failed: "+expireErr.Error())
	}

	userQuota, err := model.CacheGetUserQuota(ctx, meta.UserId)
	if err != nil {
		return preConsumedQuota, openai.ErrorWrapper(err, "get_user_quota_failed", http.StatusInternalServerError)
	}
	if userQuota-preConsumedQuota < 0 {
		return preConsumedQuota, openai.ErrorWrapper(errors.New("user quota is not enough"), "insufficient_user_quota", http.StatusForbidden)
	}
	err = model.CacheDecreaseUserQuota(meta.UserId, preConsumedQuota)
	if err != nil {
		return preConsumedQuota, openai.ErrorWrapper(err, "decrease_user_quota_failed", http.StatusInternalServerError)
	}
	if userQuota > 100*preConsumedQuota {
		// in this case, we do not pre-consume quota
		// because the user has enough quota
		preConsumedQuota = 0
		logger.Debugf(ctx, "user %s has enough quota %d, trusted and no need to pre-consume", meta.UserId, userQuota)
	}
	if preConsumedQuota > 0 {
		if strings.TrimSpace(meta.TokenId) != "" {
			err := model.PreConsumeTokenQuota(meta.TokenId, preConsumedQuota)
			if err != nil {
				return preConsumedQuota, openai.ErrorWrapper(err, "pre_consume_token_quota_failed", http.StatusForbidden)
			}
		} else {
			err := model.DecreaseUserQuota(meta.UserId, preConsumedQuota)
			if err != nil {
				return preConsumedQuota, openai.ErrorWrapper(err, "pre_consume_user_quota_failed", http.StatusForbidden)
			}
		}
	}
	return preConsumedQuota, nil
}

func postConsumeQuota(ctx context.Context, usage *relaymodel.Usage, meta *meta.Meta, textRequest *relaymodel.GeneralOpenAIRequest, pricing model.ResolvedModelPricing, preConsumedQuota int64, groupRatio float64, estimateResult tokenestimate.EstimateResult, responsesImageTools []responsesImageToolSpec, systemPromptReset bool, chargeUserBalance bool, packageReservation model.PackageQuotaReservation) {
	if usage == nil {
		logger.Error(ctx, "usage is nil, which is unexpected")
		releasePackageQuotaReservation(ctx, packageReservation)
		return
	}
	promptTokens := usage.PromptTokens
	completionTokens := usage.CompletionTokens
	quota, err := billing.ComputeTextQuota(promptTokens, completionTokens, pricing, groupRatio)
	billingSnapshot, snapshotErr := billing.ComputeTextBillingSnapshot(promptTokens, completionTokens, pricing, groupRatio)
	if snapshotErr != nil {
		logger.Error(ctx, "calculate text billing snapshot failed: "+snapshotErr.Error())
	}
	annotateTextBillingSnapshot(&billingSnapshot, pricing.Source, resolveTextEstimateSourceLabel(estimateResult), meta.UpstreamRequestPath, textRequest)
	imageFeeNote := ""
	imageFeeDetail, imageFeeNote, imageFeeErr := maybeApplyResponsesImageToolBilling(&billingSnapshot, usage, meta.ChannelProtocol, meta.ChannelModelConfigs, groupRatio, responsesImageTools)
	if imageFeeErr != nil {
		logger.Error(ctx, "calculate responses image tool billing failed: "+imageFeeErr.Error())
	}
	if err != nil {
		logger.Error(ctx, "calculate text quota failed: "+err.Error())
		quota = preConsumedQuota
	}
	if imageFeeDetail.Applied {
		quota = billingSnapshot.YYCAmount
	}
	totalTokens := promptTokens + completionTokens
	if totalTokens == 0 {
		// in this case, must be some error happened
		// we cannot just return, because we may have to return the pre-consumed quota
		quota = 0
	}
	quotaDelta := quota - preConsumedQuota
	if strings.TrimSpace(meta.TokenId) != "" {
		if chargeUserBalance {
			err = model.PostConsumeTokenQuota(meta.TokenId, quotaDelta)
		} else {
			err = model.PostConsumeTokenRemainQuota(meta.TokenId, quotaDelta)
		}
		if err != nil {
			logger.Error(ctx, "error consuming token remain quota: "+err.Error())
		}
	} else if chargeUserBalance {
		if quotaDelta > 0 {
			err = model.DecreaseUserQuota(meta.UserId, quotaDelta)
		} else if quotaDelta < 0 {
			err = model.IncreaseUserQuota(meta.UserId, -quotaDelta)
		}
		if err != nil {
			logger.Error(ctx, "error consuming user quota: "+err.Error())
		}
	}
	if chargeUserBalance {
		err = model.CacheUpdateUserQuota(ctx, meta.UserId)
		if err != nil {
			logger.Error(ctx, "error update user quota cache: "+err.Error())
		}
		if quota > 0 {
			consumedFromLots, consumeErr := model.ConsumeUserBalanceLots(meta.UserId, quota)
			if consumeErr != nil {
				logger.Error(ctx, "error consuming user balance lots: "+consumeErr.Error())
			} else if consumedFromLots < quota {
				logger.Warnf(ctx, "user balance lot coverage partial user=%s consumed=%d requested=%d", strings.TrimSpace(meta.UserId), consumedFromLots, quota)
			}
		}
	}
	userDailyQuota := 0
	userEmergencyQuota := 0
	if !chargeUserBalance {
		dailyConsumed, emergencyConsumed := settlePackageQuotaReservation(ctx, packageReservation, quota)
		userDailyQuota = int(dailyConsumed)
		userEmergencyQuota = int(emergencyConsumed)
	}
	billingSnapshot.YYCAmount = quota
	entry := &model.Log{
		UserId:             meta.UserId,
		GroupId:            meta.Group,
		ChannelId:          meta.ChannelId,
		PromptTokens:       promptTokens,
		CompletionTokens:   completionTokens,
		ModelName:          textRequest.Model,
		TokenName:          meta.TokenName,
		Quota:              int(quota),
		BillingSource:      model.ResolveConsumeLogBillingSource(chargeUserBalance),
		UserDailyQuota:     userDailyQuota,
		UserEmergencyQuota: userEmergencyQuota,
		Content:            buildTextBillingLogContent(pricing, groupRatio, imageFeeNote),
		IsStream:           meta.IsStream,
		ElapsedTime:        helper.CalcElapsedTime(meta.StartTime),
	}
	billingSnapshot.ApplyToLog(entry)
	annotateTextEstimateLogFields(entry, estimateResult)
	model.RecordConsumeLog(ctx, entry)
	model.UpdateUserUsedQuotaAndRequestCount(meta.UserId, quota)
	model.UpdateChannelUsedQuota(meta.ChannelId, quota)
}

func getMappedModelName(modelName string, mapping map[string]string) (string, bool) {
	if mapping == nil {
		return modelName, false
	}
	mappedModelName := mapping[modelName]
	if mappedModelName != "" {
		return mappedModelName, true
	}
	return modelName, false
}

func isErrorHappened(meta *meta.Meta, resp *http.Response) bool {
	if resp == nil {
		if meta.ChannelProtocol == relaychannel.AwsClaude {
			return false
		}
		return true
	}
	if resp.StatusCode != http.StatusOK &&
		// replicate return 201 to create a task
		resp.StatusCode != http.StatusCreated {
		return true
	}
	if meta.ChannelProtocol == relaychannel.DeepL {
		// skip stream check for deepl
		return false
	}

	if meta.IsStream && strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") &&
		// Even if stream mode is enabled, replicate will first return a task info in JSON format,
		// requiring the client to request the stream endpoint in the task info
		meta.ChannelProtocol != relaychannel.Replicate {
		return true
	}
	return false
}
