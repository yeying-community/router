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
	requestedChargeAmount := requested
	if requestedChargeAmount < 0 {
		requestedChargeAmount = 0
	}
	return fmt.Sprintf(
		"当前分组套餐每日额度不足：本次预估消耗 %d YYC，今日剩余 %d YYC（已用 %d，预占 %d，日上限 %d）",
		requestedChargeAmount,
		snapshot.RemainingQuota,
		snapshot.ConsumedQuota,
		snapshot.ReservedQuota,
		snapshot.Limit,
	)
}

func formatPackageQuotaExceededMessage(requested int64, daily model.GroupDailyQuotaSnapshot, emergency model.UserPackageEmergencyQuotaSnapshot) string {
	requestedChargeAmount := requested
	if requestedChargeAmount < 0 {
		requestedChargeAmount = 0
	}
	return fmt.Sprintf(
		"当前分组套餐额度不足：本次预估消耗 %d YYC，每日剩余 %d YYC（已用 %d，预占 %d，日上限 %d），应急剩余 %d YYC（已用 %d，预占 %d，应急上限 %d）",
		requestedChargeAmount,
		daily.RemainingQuota,
		daily.ConsumedQuota,
		daily.ReservedQuota,
		daily.Limit,
		emergency.RemainingQuota,
		emergency.ConsumedQuota,
		emergency.ReservedQuota,
		emergency.Limit,
	)
}

func reservePackageQuota(ctx context.Context, groupID string, userID string, quota int64) (model.PackageQuotaReservation, *relaymodel.ErrorWithStatusCode) {
	reservation, allowed, err := model.ReservePackageQuota(groupID, userID, quota)
	if err != nil {
		logger.Errorf(ctx, "group quota reserve failed code=reserve_group_daily_quota_failed group=%s user=%s requested_quota=%d err=%q", strings.TrimSpace(groupID), strings.TrimSpace(userID), quota, err.Error())
		return model.PackageQuotaReservation{}, openai.ErrorWrapper(err, "reserve_group_daily_quota_failed", http.StatusInternalServerError)
	}
	if !allowed {
		message := "当前分组套餐每日额度已达上限，请明日再试"
		dailySnapshot, dailyErr := model.GetGroupDailyQuotaSnapshot(groupID, userID, "")
		emergencySnapshot, emergencyErr := model.GetUserPackageEmergencyQuotaSnapshot(userID, "")
		if dailyErr != nil || emergencyErr != nil {
			logger.Warnf(ctx, "package quota denied group=%s user=%s requested=%d daily_snapshot_err=%v emergency_snapshot_err=%v", strings.TrimSpace(groupID), strings.TrimSpace(userID), quota, dailyErr, emergencyErr)
		} else {
			logger.Warnf(
				ctx,
				"package quota denied group=%s user=%s biz_date=%s biz_month=%s requested=%d daily_limit=%d daily_consumed=%d daily_reserved=%d daily_remaining=%d emergency_limit=%d emergency_consumed=%d emergency_reserved=%d emergency_remaining=%d",
				dailySnapshot.GroupID,
				dailySnapshot.UserID,
				dailySnapshot.BizDate,
				emergencySnapshot.BizMonth,
				quota,
				dailySnapshot.Limit,
				dailySnapshot.ConsumedQuota,
				dailySnapshot.ReservedQuota,
				dailySnapshot.RemainingQuota,
				emergencySnapshot.Limit,
				emergencySnapshot.ConsumedQuota,
				emergencySnapshot.ReservedQuota,
				emergencySnapshot.RemainingQuota,
			)
			message = formatPackageQuotaExceededMessage(quota, dailySnapshot, emergencySnapshot)
		}
		return model.PackageQuotaReservation{}, openai.ErrorWrapper(errors.New(message), groupDailyQuotaExceededCode, http.StatusForbidden)
	}
	return reservation, nil
}

func releasePackageQuotaReservation(ctx context.Context, reservation model.PackageQuotaReservation) {
	if !reservation.Active() {
		return
	}
	if err := model.ReleasePackageQuotaReservation(reservation); err != nil {
		logger.Errorf(ctx, "group quota release failed code=release_package_quota_reservation_failed group=%s user=%s daily_reserved=%d emergency_reserved=%d err=%q", strings.TrimSpace(reservation.GroupDaily.GroupID), strings.TrimSpace(reservation.GroupDaily.UserID), reservation.GroupDaily.ReservedQuota, reservation.PackageEmergency.ReservedQuota, err.Error())
	}
}

func settlePackageQuotaReservation(ctx context.Context, reservation model.PackageQuotaReservation, consumedQuota int64) (int64, int64) {
	if !reservation.Active() {
		return 0, 0
	}
	dailyConsumed, emergencyConsumed, err := model.SettlePackageQuotaReservation(reservation, consumedQuota)
	if err != nil {
		logger.Errorf(ctx, "group quota settle failed code=settle_package_quota_reservation_failed group=%s user=%s consumed_quota=%d daily_reserved=%d emergency_reserved=%d err=%q", strings.TrimSpace(reservation.GroupDaily.GroupID), strings.TrimSpace(reservation.GroupDaily.UserID), consumedQuota, reservation.GroupDaily.ReservedQuota, reservation.PackageEmergency.ReservedQuota, err.Error())
		return 0, 0
	}
	return dailyConsumed, emergencyConsumed
}

func IsGroupDailyQuotaExceededError(err *relaymodel.ErrorWithStatusCode) bool {
	if err == nil {
		return false
	}
	code := strings.TrimSpace(fmt.Sprint(err.Code))
	return code == groupDailyQuotaExceededCode
}
