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
				logger.Error(ctx, "error return pre-consumed quota: "+err.Error())
			}
			return
		}
		if !chargeUserBalance {
			return
		}
		// JWT 场景：只需要归还用户额度
		err := model.IncreaseUserQuota(userId, preConsumedQuota)
		if err != nil {
			logger.Error(ctx, "error return pre-consumed user quota: "+err.Error())
			return
		}
		_ = model.CacheUpdateUserQuota(ctx, userId)
	}(ctx)
}

func PostConsumeQuota(ctx context.Context, tokenId string, quotaDelta int64, totalQuota int64, userId string, groupID string, channelId string, pricing model.ResolvedModelPricing, groupRatio float64, modelName string, tokenName string, chargeUserBalance bool, groupReservation model.GroupDailyQuotaReservation, userReservation model.UserQuotaReservation, snapshot BillingSnapshot) {
	// quotaDelta is remaining quota to be consumed
	var err error
	if strings.TrimSpace(tokenId) != "" {
		if chargeUserBalance {
			err = model.PostConsumeTokenQuota(tokenId, quotaDelta)
		} else {
			err = model.PostConsumeTokenRemainQuota(tokenId, quotaDelta)
		}
		if err != nil {
			logger.SysError("error consuming token remain quota: " + err.Error())
		}
	} else if chargeUserBalance {
		if quotaDelta > 0 {
			err = model.DecreaseUserQuota(userId, quotaDelta)
		} else if quotaDelta < 0 {
			err = model.IncreaseUserQuota(userId, -quotaDelta)
		}
		if err != nil {
			logger.SysError("error consuming user quota: " + err.Error())
		}
	}
	if chargeUserBalance {
		err = model.CacheUpdateUserQuota(ctx, userId)
		if err != nil {
			logger.SysError("error update user quota cache: " + err.Error())
		}
	}
	userQuotaUsage, err := model.SettleUserQuotaReservation(userReservation, totalQuota)
	if err != nil {
		logger.Error(ctx, "settle user quota reservation failed: "+err.Error())
	}
	// totalQuota is total quota consumed
	if totalQuota != 0 {
		snapshot.YYCAmount = totalQuota
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
			UserDailyQuota:     int(userQuotaUsage.DailyQuotaUsed),
			UserEmergencyQuota: int(userQuotaUsage.EmergencyQuotaUsed),
			Content:            FormatPricingLog(pricing, groupRatio),
		}
		snapshot.ApplyToLog(entry)
		model.RecordConsumeLog(ctx, entry)
		model.UpdateUserUsedQuotaAndRequestCount(userId, totalQuota)
		model.UpdateChannelUsedQuota(channelId, totalQuota)
	}
	if err := model.SettleGroupDailyQuotaReservation(groupReservation, totalQuota); err != nil {
		logger.Error(ctx, "settle group daily quota reservation failed: "+err.Error())
	}
}
