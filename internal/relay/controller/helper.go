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

func preConsumeQuota(ctx context.Context, preConsumedQuota int64, meta *meta.Meta, billingPlan relayBillingPlan) (int64, *relaymodel.ErrorWithStatusCode) {
	var err error
	chargeUserBalance := billingPlan.ChargeUserBalance()
	chargeTokenQuota := billingPlan.ChargeTokenQuota()
	if !chargeUserBalance {
		if !chargeTokenQuota {
			return 0, nil
		}
		if strings.TrimSpace(meta.TokenId) == "" {
			return 0, nil
		}
		err = model.PreConsumeTokenRemainQuota(meta.TokenId, preConsumedQuota)
		if err != nil {
			logTokenPreConsumeFailure(ctx, meta, preConsumedQuota, chargeUserBalance, err)
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
				logTokenPreConsumeFailure(ctx, meta, preConsumedQuota, chargeUserBalance, err)
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

func logTokenPreConsumeFailure(ctx context.Context, meta *meta.Meta, quota int64, chargeUserBalance bool, cause error) {
	if meta == nil {
		logger.Warnf(ctx, "token pre-consume failed quota=%d charge_user_balance=%t error=%v", quota, chargeUserBalance, cause)
		return
	}
	tokenID := strings.TrimSpace(meta.TokenId)
	if tokenID == "" {
		logger.Warnf(
			ctx,
			"token pre-consume failed token_id= user_id=%s group=%s model=%s channel_id=%s quota=%d charge_user_balance=%t error=%v",
			strings.TrimSpace(meta.UserId),
			strings.TrimSpace(meta.Group),
			strings.TrimSpace(meta.OriginModelName),
			strings.TrimSpace(meta.ChannelId),
			quota,
			chargeUserBalance,
			cause,
		)
		return
	}
	token, err := model.GetTokenById(tokenID)
	if err != nil {
		logger.Warnf(
			ctx,
			"token pre-consume failed token_id=%s user_id=%s group=%s model=%s channel_id=%s quota=%d charge_user_balance=%t token_lookup_error=%v error=%v",
			tokenID,
			strings.TrimSpace(meta.UserId),
			strings.TrimSpace(meta.Group),
			strings.TrimSpace(meta.OriginModelName),
			strings.TrimSpace(meta.ChannelId),
			quota,
			chargeUserBalance,
			err,
			cause,
		)
		return
	}
	logger.Warnf(
		ctx,
		"token pre-consume failed token_id=%s token_name=%s user_id=%s group=%s model=%s channel_id=%s quota=%d token_remain_quota=%d token_unlimited=%t charge_user_balance=%t error=%v",
		tokenID,
		strings.TrimSpace(token.Name),
		strings.TrimSpace(meta.UserId),
		strings.TrimSpace(meta.Group),
		strings.TrimSpace(meta.OriginModelName),
		strings.TrimSpace(meta.ChannelId),
		quota,
		token.RemainQuota,
		token.UnlimitedQuota,
		chargeUserBalance,
		cause,
	)
}

func postConsumeQuota(ctx context.Context, usage *relaymodel.Usage, meta *meta.Meta, textRequest *relaymodel.GeneralOpenAIRequest, pricing model.ResolvedModelPricing, preConsumedQuota int64, groupRatio float64, estimateResult tokenestimate.EstimateResult, responsesImageTools []responsesImageToolSpec, systemPromptReset bool, billingPlan relayBillingPlan) {
	if usage == nil {
		logger.Error(ctx, "usage is nil, which is unexpected")
		releaseRelayBillingPlan(ctx, billingPlan)
		return
	}
	chargeUserBalance := billingPlan.ChargeUserBalance()
	chargeTokenQuota := billingPlan.ChargeTokenQuota()
	promptTokens := usage.PromptTokens
	completionTokens := usage.CompletionTokens
	quota := preConsumedQuota
	billingSnapshot, snapshotErr := billing.ComputeTextBillingSnapshot(promptTokens, completionTokens, pricing, groupRatio)
	if snapshotErr != nil {
		logger.Error(ctx, "calculate text billing snapshot failed: "+snapshotErr.Error())
	}
	annotateTextBillingSnapshot(&billingSnapshot, pricing.Source, resolveTextEstimateSourceLabel(estimateResult), meta.UpstreamRequestPath, textRequest)
	imageFeeNote := ""
	_, imageFeeNote, imageFeeErr := maybeApplyResponsesImageToolBilling(&billingSnapshot, usage, meta.ChannelProtocol, meta.ChannelModelConfigs, groupRatio, responsesImageTools)
	if imageFeeErr != nil {
		logger.Error(ctx, "calculate responses image tool billing failed: "+imageFeeErr.Error())
	}
	if snapshotErr == nil {
		if err := billing.ApplyEstimatedProcurementCostFloor(&billingSnapshot, meta.ChannelId, meta.ActualModelName); err != nil {
			logger.Error(ctx, "estimate procurement cost for text settlement failed: "+err.Error())
		}
		quota = billingSnapshot.ChargeAmount
	}
	totalTokens := promptTokens + completionTokens
	if totalTokens == 0 {
		// in this case, must be some error happened
		// we cannot just return, because we may have to return the pre-consumed quota
		quota = 0
	}
	var err error
	quotaDelta := quota - preConsumedQuota
	if strings.TrimSpace(meta.TokenId) != "" && chargeTokenQuota {
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
			consumedFromLots, consumeErr := model.ConsumeUserBalanceLotsForGroup(meta.UserId, meta.Group, quota)
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
		dailyConsumed, emergencyConsumed := settleRelayBillingPlan(ctx, billingPlan, quota)
		userDailyQuota = int(dailyConsumed)
		userEmergencyQuota = int(emergencyConsumed)
	}
	billingSnapshot.ChargeAmount = quota
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
	applyRouteObservabilityToLog(entry, meta, textRequest.Model)
	billingSnapshot.ApplyToLog(entry)
	annotateTextEstimateLogFields(entry, estimateResult)
	billing.ApplyProcurementCostObservation(entry)
	model.RecordConsumeLog(ctx, entry)
	billing.RecordProcurementConsumptionObservation(ctx, entry)
	model.UpdateUserUsedQuotaAndRequestCount(meta.UserId, quota)
	model.UpdateChannelUsedQuota(meta.ChannelId, quota)
}

func settleRequestPackageOnlyConsumption(ctx context.Context, meta *meta.Meta, modelName string, tokenName string, promptTokens int, completionTokens int, elapsedTime int64, isStream bool, billingPlan relayBillingPlan) {
	if !billingPlan.UsesRequestPackage() {
		return
	}
	settleRelayBillingPlan(ctx, billingPlan, 0)
	entry := &model.Log{
		UserId:                meta.UserId,
		GroupId:               meta.Group,
		ChannelId:             meta.ChannelId,
		PromptTokens:          promptTokens,
		CompletionTokens:      completionTokens,
		ModelName:             strings.TrimSpace(modelName),
		TokenName:             strings.TrimSpace(tokenName),
		Quota:                 0,
		BillingSource:         model.LogBillingSourcePackage,
		Content:               "request_count package",
		IsStream:              isStream,
		ElapsedTime:           elapsedTime,
		BillingUsageSource:    billingUsageSourceUpstreamUsage,
		BillingSettlementMode: "request_count_final",
		BillingChargeAmount:   0,
	}
	applyRouteObservabilityToLog(entry, meta, modelName)
	model.RecordConsumeLog(ctx, entry)
	model.UpdateUserUsedQuotaAndRequestCount(meta.UserId, 0)
	model.UpdateChannelUsedQuota(meta.ChannelId, 0)
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
