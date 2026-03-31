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

func reserveUserQuota(userID string, quota int64) (model.UserQuotaReservation, *relaymodel.ErrorWithStatusCode) {
	reservation, allowed, denyMessage, err := model.ReserveUserQuota(userID, quota)
	if err != nil {
		return model.UserQuotaReservation{}, openai.ErrorWrapper(err, "reserve_user_quota_failed", http.StatusInternalServerError)
	}
	if !allowed {
		message := strings.TrimSpace(denyMessage)
		if message == "" {
			message = "当前用户今日额度及本月应急额度已达上限"
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
