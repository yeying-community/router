package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/adaptor/openai"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

const groupDailyQuotaExceededCode = "group_daily_quota_exceeded"

func formatGroupDailyQuotaExceededMessage(requested int64, snapshot model.GroupDailyQuotaSnapshot) string {
	requestedYYC := requested
	if requestedYYC < 0 {
		requestedYYC = 0
	}
	return fmt.Sprintf(
		"当前分组套餐每日额度不足：本次预估消耗 %d YYC，今日剩余 %d YYC（已用 %d，预占 %d，日上限 %d）",
		requestedYYC,
		snapshot.RemainingQuota,
		snapshot.ConsumedQuota,
		snapshot.ReservedQuota,
		snapshot.Limit,
	)
}

func reserveGroupDailyQuota(ctx context.Context, groupID string, userID string, quota int64) (model.GroupDailyQuotaReservation, *relaymodel.ErrorWithStatusCode) {
	reservation, allowed, err := model.ReserveGroupDailyQuota(groupID, userID, quota)
	if err != nil {
		return model.GroupDailyQuotaReservation{}, openai.ErrorWrapper(err, "reserve_group_daily_quota_failed", http.StatusInternalServerError)
	}
	if !allowed {
		message := "当前分组套餐每日额度已达上限，请明日再试"
		snapshot, snapshotErr := model.GetGroupDailyQuotaSnapshot(groupID, userID, "")
		if snapshotErr != nil {
			logger.Warnf(ctx, "group daily quota denied group=%s user=%s requested=%d snapshot_err=%v", strings.TrimSpace(groupID), strings.TrimSpace(userID), quota, snapshotErr)
		} else {
			logger.Warnf(
				ctx,
				"group daily quota denied group=%s user=%s biz_date=%s requested=%d limit=%d consumed=%d reserved=%d remaining=%d unlimited=%t",
				snapshot.GroupID,
				snapshot.UserID,
				snapshot.BizDate,
				quota,
				snapshot.Limit,
				snapshot.ConsumedQuota,
				snapshot.ReservedQuota,
				snapshot.RemainingQuota,
				snapshot.Unlimited,
			)
			message = formatGroupDailyQuotaExceededMessage(quota, snapshot)
		}
		return model.GroupDailyQuotaReservation{}, openai.ErrorWrapper(errors.New(message), groupDailyQuotaExceededCode, http.StatusForbidden)
	}
	return reservation, nil
}

func releaseGroupDailyQuotaReservation(ctx context.Context, reservation model.GroupDailyQuotaReservation) {
	if !reservation.Active() {
		return
	}
	if err := model.ReleaseGroupDailyQuotaReservation(reservation); err != nil {
		logger.Error(ctx, "release group daily quota reservation failed: "+err.Error())
	}
}

func settleGroupDailyQuotaReservation(ctx context.Context, reservation model.GroupDailyQuotaReservation, consumedQuota int64) {
	if !reservation.Active() {
		return
	}
	if err := model.SettleGroupDailyQuotaReservation(reservation, consumedQuota); err != nil {
		logger.Error(ctx, "settle group daily quota reservation failed: "+err.Error())
	}
}

func IsGroupDailyQuotaExceededError(err *relaymodel.ErrorWithStatusCode) bool {
	if err == nil {
		return false
	}
	code := strings.TrimSpace(fmt.Sprint(err.Code))
	return code == groupDailyQuotaExceededCode
}
