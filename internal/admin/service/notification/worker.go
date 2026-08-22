package notification

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/common/message"
	"github.com/yeying-community/router/internal/admin/model"
)

const userNotificationWorkerInterval = 30 * time.Second

var startUserNotificationWorkerOnce sync.Once

func StartUserNotificationWorker() {
	startUserNotificationWorkerOnce.Do(func() { go runUserNotificationWorker() })
}

func runUserNotificationWorker() {
	logger.SysLog("[user.notification] worker started")
	ticker := time.NewTicker(userNotificationWorkerInterval)
	defer ticker.Stop()
	for {
		runUserNotificationOnce()
		<-ticker.C
	}
}

func runUserNotificationOnce() {
	now := helper.GetTimestamp()
	if err := model.RefreshUserBalanceLowNotificationEventsWithDB(model.DB, config.UserBalanceLowNotificationThreshold, now); err != nil {
		logger.SysWarnf("[user.notification] refresh balance alerts failed: %s", err.Error())
	}
	rows, err := model.ListUserNotificationCandidatesWithDB(model.DB, 20, now)
	if err != nil {
		logger.SysWarnf("[user.notification] list candidates failed: %s", err.Error())
		return
	}
	for _, row := range rows {
		claimed, err := model.ClaimUserNotificationEventWithDB(model.DB, row.ID, now)
		if err != nil || !claimed {
			continue
		}
		if !message.EmailConfigured() {
			_ = model.SkipUserNotificationEventWithDB(model.DB, row.ID, "smtp_not_configured")
			continue
		}
		subject, body, err := renderUserNotification(row)
		if err == nil {
			err = message.SendEmail(subject, row.RecipientEmail, message.EmailTemplate(subject, body))
		}
		if err == nil {
			_ = model.CompleteUserNotificationEventWithDB(model.DB, row.ID, helper.GetTimestamp())
			continue
		}
		delay := int64(60 * (row.AttemptCount + 1))
		_ = model.FailUserNotificationEventWithDB(model.DB, row.ID, err.Error(), helper.GetTimestamp()+delay)
		logger.SysWarnf("[user.notification] send failed event_id=%s type=%s err=%s", row.ID, row.EventType, err.Error())
	}
}

func renderUserNotification(row model.UserNotificationEvent) (string, string, error) {
	payload := struct {
		OrderID       string  `json:"order_id"`
		Title         string  `json:"title"`
		Amount        float64 `json:"amount"`
		Currency      string  `json:"currency"`
		Quota         int64   `json:"quota"`
		PackageName   string  `json:"package_name"`
		OperationType string  `json:"operation_type"`
		FulfilledAt   int64   `json:"fulfilled_at"`
		StartedAt     int64   `json:"started_at"`
		ExpiresAt     int64   `json:"expires_at"`
		GroupID       string  `json:"group_id"`
	}{}
	if err := json.Unmarshal([]byte(row.Payload), &payload); err != nil {
		return "", "", err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.ServerAddress), "/")
	recordURL := baseURL + "/workspace/service/pricing/history"
	switch row.EventType {
	case model.UserNotificationEventTypeTopupFulfilled:
		subject := "充值到账通知"
		body := fmt.Sprintf("<p>您的充值已经到账。</p><p>订单号：%s</p><p>支付金额：%.2f %s</p><p>到账额度：%d</p><p><a href=\"%s\">查看购买记录</a></p>", html.EscapeString(payload.OrderID), payload.Amount, html.EscapeString(payload.Currency), payload.Quota, html.EscapeString(recordURL))
		return subject, body, nil
	case model.UserNotificationEventTypeSubscriptionActive:
		subject := "订阅生效通知"
		name := payload.PackageName
		if strings.TrimSpace(name) == "" {
			name = payload.Title
		}
		startedAt := "-"
		if payload.StartedAt > 0 {
			startedAt = time.Unix(payload.StartedAt, 0).Format("2006-01-02 15:04:05")
		}
		expiresAt := "长期有效"
		if payload.ExpiresAt > 0 {
			expiresAt = time.Unix(payload.ExpiresAt, 0).Format("2006-01-02 15:04:05")
		}
		body := fmt.Sprintf("<p>您的订阅已经生效。</p><p>套餐：%s</p><p>订单号：%s</p><p>支付金额：%.2f %s</p><p>生效时间：%s</p><p>到期时间：%s</p><p><a href=\"%s\">查看购买记录</a></p>", html.EscapeString(name), html.EscapeString(payload.OrderID), payload.Amount, html.EscapeString(payload.Currency), startedAt, expiresAt, html.EscapeString(recordURL))
		return subject, body, nil
	case model.UserNotificationEventTypeBalanceLow:
		balancePayload := struct {
			Balance   int64 `json:"balance"`
			Threshold int64 `json:"threshold"`
		}{}
		if err := json.Unmarshal([]byte(row.Payload), &balancePayload); err != nil {
			return "", "", err
		}
		subject := "余额不足提醒"
		topupURL := baseURL + "/workspace/service/pricing"
		body := fmt.Sprintf("<p>您的可用余额已低于提醒阈值。</p><p>当前余额：%d</p><p>提醒阈值：%d</p><p><a href=\"%s\">前往充值</a></p>", balancePayload.Balance, balancePayload.Threshold, html.EscapeString(topupURL))
		return subject, body, nil
	default:
		return "", "", fmt.Errorf("unsupported user notification type: %s", row.EventType)
	}
}
