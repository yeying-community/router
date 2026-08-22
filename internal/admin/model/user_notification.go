package model

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	UserNotificationEventTypeTopupFulfilled     = "topup_fulfilled"
	UserNotificationEventTypeSubscriptionActive = "subscription_activated"
	UserNotificationEventTypeBalanceLow         = "balance_low"

	UserNotificationEventStatusPending    = "pending"
	UserNotificationEventStatusProcessing = "processing"
	UserNotificationEventStatusSent       = "sent"
	UserNotificationEventStatusFailed     = "failed"
	UserNotificationEventStatusSkipped    = "skipped"
)

type UserNotificationEvent struct {
	ID              string `gorm:"type:char(36);primaryKey"`
	EventType       string `gorm:"type:varchar(64);not null;uniqueIndex:idx_user_notification_event_dedupe,priority:1"`
	BusinessID      string `gorm:"type:varchar(128);not null;uniqueIndex:idx_user_notification_event_dedupe,priority:2"`
	UserID          string `gorm:"type:char(36);not null;index"`
	RecipientEmail  string `gorm:"type:varchar(320);not null;default:''"`
	RecipientSource string `gorm:"type:varchar(32);not null;default:'user'"`
	Status          string `gorm:"type:varchar(32);not null;index"`
	Payload         string `gorm:"type:text;not null;default:''"`
	LastError       string `gorm:"type:text;not null;default:''"`
	AttemptCount    int    `gorm:"not null;default:0"`
	NextAttemptAt   int64  `gorm:"bigint;not null;default:0;index"`
	SentAt          int64  `gorm:"bigint;not null;default:0"`
	CreatedAt       int64  `gorm:"bigint;not null;index"`
	UpdatedAt       int64  `gorm:"bigint;not null;index"`
}

func (UserNotificationEvent) TableName() string { return "user_notification_events" }

type UserBalanceNotificationState struct {
	UserID      string `gorm:"type:char(36);primaryKey"`
	IsBelow     bool   `gorm:"not null;default:false;index"`
	Cycle       int    `gorm:"not null;default:0"`
	LastBalance int64  `gorm:"bigint;not null;default:0"`
	Threshold   int64  `gorm:"bigint;not null;default:0"`
	UpdatedAt   int64  `gorm:"bigint;not null;index"`
}

func (UserBalanceNotificationState) TableName() string {
	return "user_balance_notification_states"
}

type userOrderNotificationPayload struct {
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
}

func CreateUserOrderNotificationEventWithDB(db *gorm.DB, order TopupOrder) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	eventType := ""
	switch strings.TrimSpace(order.BusinessType) {
	case TopupOrderBusinessBalance:
		eventType = UserNotificationEventTypeTopupFulfilled
	case TopupOrderBusinessPackage:
		operationType := resolveTopupOrderOperationType(TopupOrderBusinessPackage, order.OperationType)
		if operationType != TopupOrderOperationNew {
			return nil
		}
		eventType = UserNotificationEventTypeSubscriptionActive
	default:
		return nil
	}
	if strings.TrimSpace(order.CreditOrigin) != "" && order.CreditOrigin != TopupOrderCreditOriginPaid {
		return nil
	}
	now := helper.GetTimestamp()
	payloadValue := userOrderNotificationPayload{
		OrderID: order.Id, Title: order.Title, Amount: order.Amount, Currency: order.Currency,
		Quota: order.Quota, PackageName: order.PackageName, OperationType: order.OperationType,
		FulfilledAt: order.RedeemedAt,
	}
	if eventType == UserNotificationEventTypeSubscriptionActive {
		subscription := UserPackageSubscription{}
		if err := db.Where("user_id = ? AND package_id = ?", order.UserID, order.PackageID).
			Order("created_at desc, updated_at desc, id desc").First(&subscription).Error; err == nil {
			payloadValue.StartedAt = subscription.StartedAt
			payloadValue.ExpiresAt = subscription.ExpiresAt
			payloadValue.GroupID = subscription.GroupID
		}
	}
	payload, err := json.Marshal(payloadValue)
	if err != nil {
		return err
	}
	type userEmailRow struct{ Email string }
	var emailRow userEmailRow
	lookupErr := db.Model(&User{}).Select("email").Where("id = ?", order.UserID).Scan(&emailRow).Error
	event := UserNotificationEvent{
		ID: random.GetUUID(), EventType: eventType, BusinessID: order.Id, UserID: order.UserID,
		Status: UserNotificationEventStatusPending, Payload: string(payload), NextAttemptAt: now,
		RecipientSource: "user", CreatedAt: now, UpdatedAt: now,
	}
	if lookupErr == nil && strings.TrimSpace(emailRow.Email) != "" {
		event.RecipientEmail = strings.ToLower(strings.TrimSpace(emailRow.Email))
	} else if lookupErr == gorm.ErrRecordNotFound || (lookupErr == nil && strings.TrimSpace(emailRow.Email) == "") {
		event.Status = UserNotificationEventStatusSkipped
		event.LastError = "recipient_email_unavailable"
	} else {
		return lookupErr
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_type"}, {Name: "business_id"}},
		DoNothing: true,
	}).Create(&event).Error
}

func ListUserNotificationCandidatesWithDB(db *gorm.DB, limit int, now int64) ([]UserNotificationEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	rows := make([]UserNotificationEvent, 0)
	err := db.Where("status IN ? AND next_attempt_at <= ? AND attempt_count < ?", []string{UserNotificationEventStatusPending, UserNotificationEventStatusFailed}, now, 5).
		Order("next_attempt_at asc, created_at asc").Limit(limit).Find(&rows).Error
	return rows, err
}

func ClaimUserNotificationEventWithDB(db *gorm.DB, id string, now int64) (bool, error) {
	result := db.Model(&UserNotificationEvent{}).
		Where("id = ? AND status IN ? AND next_attempt_at <= ?", strings.TrimSpace(id), []string{UserNotificationEventStatusPending, UserNotificationEventStatusFailed}, now).
		Updates(map[string]any{"status": UserNotificationEventStatusProcessing, "attempt_count": gorm.Expr("attempt_count + 1"), "updated_at": now})
	return result.RowsAffected == 1, result.Error
}

func CompleteUserNotificationEventWithDB(db *gorm.DB, id string, sentAt int64) error {
	return db.Model(&UserNotificationEvent{}).Where("id = ? AND status = ?", id, UserNotificationEventStatusProcessing).
		Updates(map[string]any{"status": UserNotificationEventStatusSent, "sent_at": sentAt, "last_error": "", "updated_at": sentAt}).Error
}

func SkipUserNotificationEventWithDB(db *gorm.DB, id string, reason string) error {
	now := helper.GetTimestamp()
	return db.Model(&UserNotificationEvent{}).Where("id = ? AND status = ?", id, UserNotificationEventStatusProcessing).
		Updates(map[string]any{"status": UserNotificationEventStatusSkipped, "last_error": strings.TrimSpace(reason), "updated_at": now}).Error
}

func FailUserNotificationEventWithDB(db *gorm.DB, id string, message string, nextAttemptAt int64) error {
	return db.Model(&UserNotificationEvent{}).Where("id = ? AND status = ?", id, UserNotificationEventStatusProcessing).
		Updates(map[string]any{"status": UserNotificationEventStatusFailed, "last_error": strings.TrimSpace(message), "next_attempt_at": nextAttemptAt, "updated_at": helper.GetTimestamp()}).Error
}

type userVerifiedNotificationBalance struct {
	UserID           string `gorm:"column:user_id"`
	Email            string `gorm:"column:email"`
	AvailableBalance int64  `gorm:"column:available_balance"`
}

func RefreshUserBalanceLowNotificationEventsWithDB(db *gorm.DB, threshold int64, now int64) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if threshold <= 0 {
		return nil
	}
	if now <= 0 {
		now = helper.GetTimestamp()
	}
	rows := make([]userVerifiedNotificationBalance, 0)
	err := db.Raw(`
		SELECT u.id AS user_id, u.email,
		       COALESCE(SUM(CASE
		           WHEN l.status = ? AND l.remaining_amount > 0 AND (l.expires_at = 0 OR l.expires_at > ?)
		           THEN l.remaining_amount ELSE 0 END), 0) AS available_balance
		FROM users u
		LEFT JOIN user_balance_lots l ON l.user_id = u.id
		WHERE COALESCE(TRIM(u.email), '') <> ''
		GROUP BY u.id, u.email
	`, UserBalanceLotStatusActive, now).Scan(&rows).Error
	if err != nil {
		return err
	}
	for _, row := range rows {
		row := row
		if err := db.Transaction(func(tx *gorm.DB) error {
			state := UserBalanceNotificationState{UserID: row.UserID}
			lookupErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&state, "user_id = ?", row.UserID).Error
			if lookupErr != nil && lookupErr != gorm.ErrRecordNotFound {
				return lookupErr
			}
			below := row.AvailableBalance < threshold
			if below && (lookupErr == gorm.ErrRecordNotFound || !state.IsBelow) {
				state.Cycle++
				payload, err := json.Marshal(map[string]int64{"balance": row.AvailableBalance, "threshold": threshold})
				if err != nil {
					return err
				}
				event := UserNotificationEvent{
					ID: random.GetUUID(), EventType: UserNotificationEventTypeBalanceLow,
					BusinessID: fmt.Sprintf("%s:%d", row.UserID, state.Cycle), UserID: row.UserID,
					RecipientEmail:  strings.ToLower(strings.TrimSpace(row.Email)),
					RecipientSource: "user", Status: UserNotificationEventStatusPending,
					Payload: string(payload), NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
				}
				if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_type"}, {Name: "business_id"}}, DoNothing: true}).Create(&event).Error; err != nil {
					return err
				}
			}
			state.IsBelow = below
			state.LastBalance = row.AvailableBalance
			state.Threshold = threshold
			state.UpdatedAt = now
			return tx.Save(&state).Error
		}); err != nil {
			return err
		}
	}
	return nil
}
