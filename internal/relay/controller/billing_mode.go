package controller

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/adaptor/openai"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"gorm.io/gorm"
)

type relayBillingSource string

const (
	relayBillingSourceBalance                relayBillingSource = "balance"
	relayBillingSourcePackage                relayBillingSource = "package"
	relayBillingSourcePackageFallbackBalance relayBillingSource = "package_fallback_balance"
)

type relayBillingPlan struct {
	Source           relayBillingSource
	GroupReservation model.GroupDailyQuotaReservation
	UserReservation  model.UserQuotaReservation
}

func (plan relayBillingPlan) ChargeUserBalance() bool {
	return plan.Source != relayBillingSourcePackage
}

func hasActivePackageForGroup(userID string, groupID string) (bool, error) {
	normalizedUserID := strings.TrimSpace(userID)
	normalizedGroupID := strings.TrimSpace(groupID)
	if normalizedUserID == "" || normalizedGroupID == "" {
		return false, nil
	}
	_, err := model.GetActiveUserPackageSubscriptionForGroup(normalizedUserID, normalizedGroupID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func reserveRelayQuota(ctx context.Context, groupID string, userID string, quota int64) (relayBillingPlan, *relaymodel.ErrorWithStatusCode) {
	packageActive, err := hasActivePackageForGroup(userID, groupID)
	if err != nil {
		return relayBillingPlan{}, openai.ErrorWrapper(err, "resolve_billing_source_failed", http.StatusInternalServerError)
	}
	if packageActive {
		groupReservation, groupQuotaErr := reserveGroupDailyQuota(ctx, groupID, userID, quota)
		if groupQuotaErr == nil {
			return relayBillingPlan{
				Source:           relayBillingSourcePackage,
				GroupReservation: groupReservation,
			}, nil
		}
		if !IsGroupDailyQuotaExceededError(groupQuotaErr) {
			return relayBillingPlan{}, groupQuotaErr
		}
		logger.Infof(
			ctx,
			"package quota exhausted, fallback to balance group=%s user=%s requested=%d",
			strings.TrimSpace(groupID),
			strings.TrimSpace(userID),
			quota,
		)
	}
	userReservation, userQuotaErr := reserveUserQuota(userID, quota)
	if userQuotaErr != nil {
		return relayBillingPlan{}, userQuotaErr
	}
	source := relayBillingSourceBalance
	if packageActive {
		source = relayBillingSourcePackageFallbackBalance
	}
	return relayBillingPlan{
		Source:          source,
		UserReservation: userReservation,
	}, nil
}
