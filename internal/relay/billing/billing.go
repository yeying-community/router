package billing

import (
	"context"
	"strings"

	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
)

func ReturnPreConsumedQuota(ctx context.Context, preConsumedQuota int64, tokenId string, userId string, chargeUserBalance bool) {
	if preConsumedQuota == 0 {
		return
	}
	go func(ctx context.Context) {
		if strings.TrimSpace(tokenId) != "" {
			var err error
			if chargeUserBalance {
				err = model.PostConsumeTokenQuota(tokenId, -preConsumedQuota)
			} else {
				err = model.PostConsumeTokenRemainQuota(tokenId, -preConsumedQuota)
			}
			if err != nil {
				logger.Errorf(ctx, "billing rollback failed code=return_pre_consumed_quota_failed user_id=%s token_id=%s charge_user_balance=%t rollback_quota=%d err=%q", strings.TrimSpace(userId), strings.TrimSpace(tokenId), chargeUserBalance, preConsumedQuota, err.Error())
			}
			return
		}
		if !chargeUserBalance {
			return
		}
		// JWT 场景：只需要归还用户额度
		err := model.IncreaseUserQuota(userId, preConsumedQuota)
		if err != nil {
			logger.Errorf(ctx, "billing rollback failed code=return_pre_consumed_user_quota_failed user_id=%s charge_user_balance=%t rollback_quota=%d err=%q", strings.TrimSpace(userId), chargeUserBalance, preConsumedQuota, err.Error())
			return
		}
		_ = model.CacheUpdateUserQuota(ctx, userId)
	}(ctx)
}

type LogRouteObserver func(entry *model.Log)

func PostConsumeQuota(ctx context.Context, tokenId string, quotaDelta int64, totalQuota int64, userId string, groupID string, channelId string, pricing model.ResolvedModelPricing, groupRatio float64, modelName string, tokenName string, chargeUserBalance bool, chargeTokenQuota bool, packageReservation model.PackageQuotaReservation, snapshot BillingSnapshot, routeObservers ...LogRouteObserver) {
	// quotaDelta is remaining quota to be consumed
	var err error
	if strings.TrimSpace(tokenId) != "" && chargeTokenQuota {
		if chargeUserBalance {
			err = model.PostConsumeTokenQuota(tokenId, quotaDelta)
		} else {
			err = model.PostConsumeTokenRemainQuota(tokenId, quotaDelta)
		}
		if err != nil {
			logger.Errorf(ctx, "billing post_consume failed code=post_consume_token_quota_failed user_id=%s group=%s channel_id=%s model=%s token_id=%s quota_delta=%d total_quota=%d charge_user_balance=%t err=%q", strings.TrimSpace(userId), strings.TrimSpace(groupID), strings.TrimSpace(channelId), strings.TrimSpace(modelName), strings.TrimSpace(tokenId), quotaDelta, totalQuota, chargeUserBalance, err.Error())
		}
	} else if chargeUserBalance {
		if quotaDelta > 0 {
			err = model.DecreaseUserQuota(userId, quotaDelta)
		} else if quotaDelta < 0 {
			err = model.IncreaseUserQuota(userId, -quotaDelta)
		}
		if err != nil {
			logger.Errorf(ctx, "billing post_consume failed code=post_consume_user_quota_failed user_id=%s group=%s channel_id=%s model=%s quota_delta=%d total_quota=%d charge_user_balance=%t err=%q", strings.TrimSpace(userId), strings.TrimSpace(groupID), strings.TrimSpace(channelId), strings.TrimSpace(modelName), quotaDelta, totalQuota, chargeUserBalance, err.Error())
		}
	}
	if chargeUserBalance {
		err = model.CacheUpdateUserQuota(ctx, userId)
		if err != nil {
			logger.Errorf(ctx, "billing cache update failed code=update_user_quota_cache_failed user_id=%s group=%s channel_id=%s model=%s quota_delta=%d total_quota=%d charge_user_balance=%t err=%q", strings.TrimSpace(userId), strings.TrimSpace(groupID), strings.TrimSpace(channelId), strings.TrimSpace(modelName), quotaDelta, totalQuota, chargeUserBalance, err.Error())
		}
		if totalQuota > 0 {
			consumedFromLots, consumeErr := model.ConsumeUserBalanceLotsForGroup(userId, groupID, totalQuota)
			if consumeErr != nil {
				logger.Errorf(ctx, "billing lots consume failed code=consume_user_balance_lots_failed user_id=%s group=%s channel_id=%s model=%s total_quota=%d err=%q", strings.TrimSpace(userId), strings.TrimSpace(groupID), strings.TrimSpace(channelId), strings.TrimSpace(modelName), totalQuota, consumeErr.Error())
			} else if consumedFromLots < totalQuota {
				logger.Warnf(ctx, "billing lots consume partial user_id=%s group=%s channel_id=%s model=%s consumed=%d requested=%d", strings.TrimSpace(userId), strings.TrimSpace(groupID), strings.TrimSpace(channelId), strings.TrimSpace(modelName), consumedFromLots, totalQuota)
			}
		}
	}
	userDailyQuota := 0
	userEmergencyQuota := 0
	if !chargeUserBalance {
		dailyConsumed, emergencyConsumed, settleErr := model.SettlePackageQuotaReservation(packageReservation, totalQuota)
		if settleErr != nil {
			logger.Errorf(ctx, "billing settle failed code=settle_package_quota_reservation_failed user_id=%s group=%s channel_id=%s model=%s token_id=%s quota_delta=%d total_quota=%d charge_user_balance=%t err=%q", strings.TrimSpace(userId), strings.TrimSpace(groupID), strings.TrimSpace(channelId), strings.TrimSpace(modelName), strings.TrimSpace(tokenId), quotaDelta, totalQuota, chargeUserBalance, settleErr.Error())
		} else {
			userDailyQuota = int(dailyConsumed)
			userEmergencyQuota = int(emergencyConsumed)
		}
	}
	// totalQuota is total quota consumed
	if totalQuota != 0 {
		snapshot.ChargeAmount = totalQuota
		entry := &model.Log{
			UserId:             userId,
			GroupId:            groupID,
			ChannelId:          channelId,
			PromptTokens:       int(totalQuota),
			CompletionTokens:   0,
			ModelName:          modelName,
			TokenName:          tokenName,
			Quota:              int(totalQuota),
			BillingSource:      model.ResolveConsumeLogBillingSource(chargeUserBalance),
			UserDailyQuota:     userDailyQuota,
			UserEmergencyQuota: userEmergencyQuota,
			Content:            FormatPricingLog(pricing, groupRatio),
		}
		for _, observer := range routeObservers {
			if observer != nil {
				observer(entry)
			}
		}
		snapshot.ApplyToLog(entry)
		ApplyProcurementCostObservation(entry)
		model.RecordConsumeLog(ctx, entry)
		RecordProcurementConsumptionObservation(ctx, entry)
		model.UpdateUserUsedQuotaAndRequestCount(userId, totalQuota)
		model.UpdateChannelUsedQuota(channelId, totalQuota)
	}
}
