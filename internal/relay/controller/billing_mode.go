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
	ConcurrencyReservation    model.EntitlementConcurrencyReservation
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
	return tryBuildRequestPackageBillingPlanWithAmount(ctx, meta, 1)
}

func tryBuildRequestPackageBillingPlanWithAmount(ctx context.Context, meta *meta.Meta, requestAmount int64) (relayBillingPlan, bool, *relaymodel.ErrorWithStatusCode) {
	if requestAmount <= 0 {
		requestAmount = 1
	}
	return tryReserveRequestPackage(ctx, meta, requestAmount)
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
		packageName = "生效套餐"
	}
	switch strings.TrimSpace(result.Reason) {
	case "request_concurrency_per_user_exceeded":
		return fmt.Sprintf("%s 当前用户并发请求数已达上限", packageName)
	case "request_concurrency_per_package_exceeded":
		return fmt.Sprintf("%s 套餐总并发请求数已达上限", packageName)
	case "request_quota_limit_unconfigured":
		return fmt.Sprintf("%s 请求次数额度未配置", packageName)
	default:
		return fmt.Sprintf("%s 请求次数额度不足，本周期剩余 %d 次", packageName, result.Remaining)
	}
}

func tryReserveRequestPackage(ctx context.Context, meta *meta.Meta, requestAmount int64) (relayBillingPlan, bool, *relaymodel.ErrorWithStatusCode) {
	if meta == nil || strings.TrimSpace(meta.UserId) == "" {
		return relayBillingPlan{}, false, nil
	}
	result, err := model.ReserveRequestPackage(model.PackageScopeRequest{
		UserID:        meta.UserId,
		GroupID:       meta.Group,
		RequestAmount: requestAmount,
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
		ConcurrencyReservation:    result.Concurrency,
	}, true, nil
}

func formatEntitlementConcurrencyDeniedMessage(sourceName string, sourceKind string, reason string) string {
	name := strings.TrimSpace(sourceName)
	if name == "" {
		switch sourceKind {
		case model.EntitlementConcurrencySourceTopupPlan:
			name = "当前充值额度"
		default:
			name = "生效套餐"
		}
	}
	switch strings.TrimSpace(reason) {
	case model.EntitlementConcurrencyReasonPerUserExceeded:
		return fmt.Sprintf("%s 当前用户并发请求数已达上限", name)
	case model.EntitlementConcurrencyReasonPerSourceExceeded:
		if sourceKind == model.EntitlementConcurrencySourceTopupPlan {
			return fmt.Sprintf("%s 当前充值方案总并发请求数已达上限", name)
		}
		return fmt.Sprintf("%s 套餐总并发请求数已达上限", name)
	default:
		return fmt.Sprintf("%s 并发请求数已达上限", name)
	}
}

func reserveBalanceConcurrency(ctx context.Context, meta *meta.Meta, packageActive bool) (model.EntitlementConcurrencyReservation, *relaymodel.ErrorWithStatusCode) {
	if meta == nil || strings.TrimSpace(meta.UserId) == "" {
		return model.EntitlementConcurrencyReservation{}, nil
	}
	entitlement, err := model.GetActiveTopupConcurrencyEntitlementForGroup(meta.UserId, meta.Group)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.EntitlementConcurrencyReservation{}, nil
		}
		logger.Errorf(ctx, "resolve topup concurrency entitlement failed user_id=%s group=%s err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), err.Error())
		return model.EntitlementConcurrencyReservation{}, openai.ErrorWrapper(err, "resolve_topup_concurrency_failed", http.StatusInternalServerError)
	}
	result, err := model.ReserveEntitlementConcurrency(model.EntitlementConcurrencyReserveInput{
		SourceType:               model.EntitlementConcurrencySourceTopupPlan,
		SourceID:                 strings.TrimSpace(entitlement.TopupPlanID),
		SourceName:               strings.TrimSpace(entitlement.Title),
		UserID:                   strings.TrimSpace(meta.UserId),
		RequestCount:             1,
		MaxConcurrencyPerUser:    entitlement.MaxConcurrencyPerUser,
		MaxConcurrencyPerPackage: entitlement.MaxConcurrencyPerPackage,
	})
	if err != nil {
		logger.Errorf(ctx, "reserve topup concurrency failed user_id=%s group=%s plan_id=%s err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(entitlement.TopupPlanID), err.Error())
		return model.EntitlementConcurrencyReservation{}, openai.ErrorWrapper(err, "reserve_topup_concurrency_failed", http.StatusInternalServerError)
	}
	if !result.Allowed {
		message := formatEntitlementConcurrencyDeniedMessage(entitlement.Title, model.EntitlementConcurrencySourceTopupPlan, result.Reason)
		logger.Warnf(ctx, "topup concurrency denied user_id=%s group=%s plan_id=%s reason=%s package_active=%t", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(entitlement.TopupPlanID), strings.TrimSpace(result.Reason), packageActive)
		return model.EntitlementConcurrencyReservation{}, openai.ErrorWrapper(errors.New(message), "topup_concurrency_exceeded", http.StatusForbidden)
	}
	return result.Reservation, nil
}

func reserveRelayQuota(ctx context.Context, meta *meta.Meta, quota int64) (relayBillingPlan, *relaymodel.ErrorWithStatusCode) {
	if plan, matched, err := tryReserveRequestPackage(ctx, meta, 1); matched || err != nil {
		if err == nil && matched && plan.Source != relayBillingSourcePackage {
			concurrencyReservation, concurrencyErr := reserveBalanceConcurrency(ctx, meta, true)
			if concurrencyErr != nil {
				return relayBillingPlan{}, concurrencyErr
			}
			plan.ConcurrencyReservation = concurrencyReservation
		}
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
		subscription, subscriptionErr := model.GetActiveYYCUserPackageSubscriptionForGroup(userID, groupID)
		if subscriptionErr != nil && !errors.Is(subscriptionErr, gorm.ErrRecordNotFound) {
			return relayBillingPlan{}, openai.ErrorWrapper(subscriptionErr, "resolve_package_concurrency_failed", http.StatusInternalServerError)
		}
		concurrencyReservation := model.EntitlementConcurrencyReservation{}
		if subscriptionErr == nil {
			concurrencyResult, concurrencyErr := model.ReserveEntitlementConcurrency(model.EntitlementConcurrencyReserveInput{
				SourceType:               model.EntitlementConcurrencySourceServicePackage,
				SourceID:                 strings.TrimSpace(subscription.PackageID),
				SourceName:               strings.TrimSpace(subscription.PackageName),
				UserID:                   strings.TrimSpace(subscription.UserID),
				RequestCount:             1,
				MaxConcurrencyPerUser:    subscription.MaxConcurrencyPerUser,
				MaxConcurrencyPerPackage: subscription.MaxConcurrencyPerPackage,
			})
			if concurrencyErr != nil {
				return relayBillingPlan{}, openai.ErrorWrapper(concurrencyErr, "reserve_package_concurrency_failed", http.StatusInternalServerError)
			}
			if !concurrencyResult.Allowed {
				message := formatEntitlementConcurrencyDeniedMessage(subscription.PackageName, model.EntitlementConcurrencySourceServicePackage, concurrencyResult.Reason)
				return relayBillingPlan{}, openai.ErrorWrapper(errors.New(message), "package_concurrency_exceeded", http.StatusForbidden)
			}
			concurrencyReservation = concurrencyResult.Reservation
		}
		packageReservation, groupQuotaErr := reservePackageQuota(ctx, groupID, userID, quota)
		if groupQuotaErr == nil {
			return relayBillingPlan{
				Source:                 relayBillingSourcePackage,
				PackageReservation:     packageReservation,
				ConcurrencyReservation: concurrencyReservation,
			}, nil
		}
		if concurrencyReservation.Active() {
			_ = model.ReleaseEntitlementConcurrencyReservation(concurrencyReservation)
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
	plan := buildBalanceRelayBillingPlan(packageActive)
	concurrencyReservation, concurrencyErr := reserveBalanceConcurrency(ctx, meta, packageActive)
	if concurrencyErr != nil {
		return relayBillingPlan{}, concurrencyErr
	}
	plan.ConcurrencyReservation = concurrencyReservation
	return plan, nil
}
