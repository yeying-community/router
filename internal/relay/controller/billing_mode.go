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
	"github.com/yeying-community/router/internal/relay/meta"
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
	Source                    relayBillingSource
	PackageReservation        model.PackageQuotaReservation
	RequestPackageReservation model.RequestPackageReservation
}

func (plan relayBillingPlan) ChargeUserBalance() bool {
	return plan.Source != relayBillingSourcePackage
}

func (plan relayBillingPlan) UsesPackage() bool {
	return plan.Source == relayBillingSourcePackage
}

func (plan relayBillingPlan) UsesRequestPackage() bool {
	return plan.RequestPackageReservation.Active()
}

func (plan relayBillingPlan) ChargeTokenQuota() bool {
	return !plan.UsesRequestPackage()
}

func buildBalanceRelayBillingPlan(packageActive bool) relayBillingPlan {
	source := relayBillingSourceBalance
	if packageActive {
		source = relayBillingSourcePackageFallbackBalance
	}
	return relayBillingPlan{Source: source}
}

func tryBuildRequestPackageBillingPlan(ctx context.Context, meta *meta.Meta) (relayBillingPlan, bool, *relaymodel.ErrorWithStatusCode) {
	return tryReserveRequestPackage(ctx, meta)
}

func hasActivePackageForGroup(userID string, groupID string) (bool, error) {
	normalizedUserID := strings.TrimSpace(userID)
	normalizedGroupID := strings.TrimSpace(groupID)
	if normalizedUserID == "" || normalizedGroupID == "" {
		return false, nil
	}
	_, err := model.GetActiveYYCUserPackageSubscriptionForGroup(normalizedUserID, normalizedGroupID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func formatRequestPackageDeniedMessage(result model.RequestPackageReserveResult) string {
	packageName := strings.TrimSpace(result.Subscription.PackageName)
	if packageName == "" {
		packageName = strings.TrimSpace(result.Subscription.PackageID)
	}
	if packageName == "" {
		packageName = "当前套餐"
	}
	switch strings.TrimSpace(result.Reason) {
	case "request_concurrency_per_user_exceeded":
		return fmt.Sprintf("%s 当前用户并发请求数已达上限", packageName)
	case "request_concurrency_per_package_exceeded":
		return fmt.Sprintf("%s 当前套餐总并发请求数已达上限", packageName)
	case "request_quota_limit_unconfigured":
		return fmt.Sprintf("%s 请求次数额度未配置", packageName)
	default:
		return fmt.Sprintf("%s 请求次数额度不足，本周期剩余 %d 次", packageName, result.Remaining)
	}
}

func tryReserveRequestPackage(ctx context.Context, meta *meta.Meta) (relayBillingPlan, bool, *relaymodel.ErrorWithStatusCode) {
	if meta == nil || strings.TrimSpace(meta.UserId) == "" {
		return relayBillingPlan{}, false, nil
	}
	result, err := model.ReserveRequestPackage(model.PackageScopeRequest{
		UserID:        meta.UserId,
		GroupID:       meta.Group,
		RequestAmount: 1,
	})
	if err != nil {
		logger.Errorf(ctx, "request package reserve failed code=reserve_request_package_failed user_id=%s group=%s err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), err.Error())
		return relayBillingPlan{}, true, openai.ErrorWrapper(err, "reserve_request_package_failed", http.StatusInternalServerError)
	}
	if !result.Matched {
		return relayBillingPlan{}, false, nil
	}
	if !result.Allowed {
		if result.Subscription.AllowBalanceFallback {
			logger.Infof(ctx, "request package denied with balance fallback user_id=%s group=%s package_id=%s reason=%s remaining=%d", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(result.Subscription.PackageID), strings.TrimSpace(result.Reason), result.Remaining)
			return buildBalanceRelayBillingPlan(true), true, nil
		}
		message := formatRequestPackageDeniedMessage(result)
		logger.Warnf(ctx, "request package denied user_id=%s group=%s package_id=%s reason=%s remaining=%d", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(result.Subscription.PackageID), strings.TrimSpace(result.Reason), result.Remaining)
		return relayBillingPlan{}, true, openai.ErrorWrapper(errors.New(message), "request_package_quota_exceeded", http.StatusForbidden)
	}
	return relayBillingPlan{
		Source:                    relayBillingSourcePackage,
		RequestPackageReservation: result.Reservation,
	}, true, nil
}

func reserveRelayQuota(ctx context.Context, meta *meta.Meta, quota int64) (relayBillingPlan, *relaymodel.ErrorWithStatusCode) {
	if plan, matched, err := tryReserveRequestPackage(ctx, meta); matched || err != nil {
		return plan, err
	}
	groupID := ""
	userID := ""
	if meta != nil {
		groupID = meta.Group
		userID = meta.UserId
	}
	packageActive, err := hasActivePackageForGroup(userID, groupID)
	if err != nil {
		return relayBillingPlan{}, openai.ErrorWrapper(err, "resolve_billing_source_failed", http.StatusInternalServerError)
	}
	if packageActive {
		packageReservation, groupQuotaErr := reservePackageQuota(ctx, groupID, userID, quota)
		if groupQuotaErr == nil {
			return relayBillingPlan{
				Source:             relayBillingSourcePackage,
				PackageReservation: packageReservation,
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
	// Balance-mode requests are admitted by main balance and token quota only.
	return buildBalanceRelayBillingPlan(packageActive), nil
}
