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

const userQuotaExceededCode = "user_quota_limit_exceeded"

func formatUserQuotaExceededMessage(requested int64, summary model.UserQuotaSummary, fallback string) string {
	requestedYYC := requested
	if requestedYYC < 0 {
		requestedYYC = 0
	}
	dailyRemaining := summary.Daily.RemainingQuota
	emergencyRemaining := summary.MonthlyEmergency.RemainingQuota
	if summary.MonthlyEmergency.Enabled {
		return fmt.Sprintf(
			"当前用户额度不足：本次预估消耗 %d YYC，今日剩余 %d YYC，套餐应急剩余 %d YYC",
			requestedYYC,
			dailyRemaining,
			emergencyRemaining,
		)
	}
	if strings.TrimSpace(fallback) != "" {
		return fmt.Sprintf(
			"%s（本次预估消耗 %d YYC，今日剩余 %d YYC）",
			strings.TrimSpace(fallback),
			requestedYYC,
			dailyRemaining,
		)
	}
	return fmt.Sprintf(
		"当前用户今日额度不足：本次预估消耗 %d YYC，今日剩余 %d YYC",
		requestedYYC,
		dailyRemaining,
	)
}

func reserveUserQuota(userID string, quota int64) (model.UserQuotaReservation, *relaymodel.ErrorWithStatusCode) {
	reservation, allowed, denyMessage, err := model.ReserveUserQuota(userID, quota)
	if err != nil {
		return model.UserQuotaReservation{}, openai.ErrorWrapper(err, "reserve_user_quota_failed", http.StatusInternalServerError)
	}
	if !allowed {
		message := strings.TrimSpace(denyMessage)
		if message == "" {
			message = "当前用户今日额度及套餐应急额度已达上限"
		}
		summary, summaryErr := model.GetUserQuotaSummary(userID, "", "")
		if summaryErr != nil {
			logger.Warnf(context.Background(), "user quota denied user=%s requested=%d summary_err=%v", strings.TrimSpace(userID), quota, summaryErr)
		} else {
			message = formatUserQuotaExceededMessage(quota, summary, message)
		}
		return model.UserQuotaReservation{}, openai.ErrorWrapper(errors.New(message), userQuotaExceededCode, http.StatusForbidden)
	}
	return reservation, nil
}

func releaseUserQuotaReservation(ctx context.Context, reservation model.UserQuotaReservation) {
	if !reservation.Active() {
		return
	}
	if err := model.ReleaseUserQuotaReservation(reservation); err != nil {
		logger.Error(ctx, "release user quota reservation failed: "+err.Error())
	}
}

func settleUserQuotaReservation(ctx context.Context, reservation model.UserQuotaReservation, consumedQuota int64) model.UserQuotaUsage {
	if !reservation.Active() {
		return model.UserQuotaUsage{}
	}
	usage, err := model.SettleUserQuotaReservation(reservation, consumedQuota)
	if err != nil {
		logger.Error(ctx, "settle user quota reservation failed: "+err.Error())
		return model.UserQuotaUsage{}
	}
	return usage
}

func IsUserQuotaExceededError(err *relaymodel.ErrorWithStatusCode) bool {
	if err == nil {
		return false
	}
	code := strings.TrimSpace(fmt.Sprint(err.Code))
	return code == userQuotaExceededCode
}
