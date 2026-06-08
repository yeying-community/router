package model

import (
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
)

const (
	ChannelCircuitBreakerStateOpen      = "open"
	ChannelCircuitBreakerStateHalfOpen  = "half_open"
	ChannelCircuitBreakerStateRecovered = "recovered"
	ChannelCircuitBreakerStateCanceled  = "canceled"

	ChannelCircuitBreakerReasonInsufficientBalance = "insufficient balance"
)

type ChannelCircuitBreakerState struct {
	ChannelId    string  `json:"channel_id" gorm:"type:varchar(64);primaryKey;autoIncrement:false"`
	State        string  `json:"state" gorm:"type:varchar(32);not null;default:'open';index"`
	Reason       string  `json:"reason" gorm:"type:text"`
	SuccessRate  float64 `json:"success_rate" gorm:"type:double precision;default:0"`
	DisabledAt   int64   `json:"disabled_at" gorm:"bigint;index"`
	RecoverAfter int64   `json:"recover_after" gorm:"bigint;index"`
	RecoveredAt  int64   `json:"recovered_at" gorm:"bigint;index"`
	UpdatedAt    int64   `json:"updated_at" gorm:"bigint;index"`
}

type ChannelCircuitBreakerEvent struct {
	ID           uint    `json:"id" gorm:"primaryKey"`
	ChannelId    string  `json:"channel_id" gorm:"type:varchar(64);not null;index"`
	Event        string  `json:"event" gorm:"type:varchar(32);not null;index"`
	State        string  `json:"state" gorm:"type:varchar(32);not null;default:'';index"`
	Reason       string  `json:"reason" gorm:"type:text"`
	SuccessRate  float64 `json:"success_rate" gorm:"type:double precision;default:0"`
	RecoverAfter int64   `json:"recover_after" gorm:"bigint;index"`
	CreatedAt    int64   `json:"created_at" gorm:"bigint;index"`
}

func (ChannelCircuitBreakerState) TableName() string {
	return "channel_circuit_breaker_states"
}

func (ChannelCircuitBreakerEvent) TableName() string {
	return "channel_circuit_breaker_events"
}

func RecordChannelCircuitBreakerOpen(channelID string, reason string, successRate float64, recoverAfter int64) error {
	return recordChannelCircuitBreakerOpenWithDB(DB, channelID, reason, successRate, recoverAfter)
}

func RecordChannelCircuitBreakerRecovered(channelID string) error {
	return updateChannelCircuitBreakerStateWithDB(DB, channelID, ChannelCircuitBreakerStateRecovered, "")
}

func RecordChannelCircuitBreakerHalfOpen(channelID string) error {
	return updateChannelCircuitBreakerStateWithDB(DB, channelID, ChannelCircuitBreakerStateHalfOpen, "")
}

func RecordChannelCircuitBreakerCanceled(channelID string, reason string) error {
	return updateChannelCircuitBreakerStateWithDB(DB, channelID, ChannelCircuitBreakerStateCanceled, reason)
}

func IsInsufficientBalanceCircuitBreakerState(row ChannelCircuitBreakerState) bool {
	return strings.TrimSpace(strings.ToLower(row.State)) == ChannelCircuitBreakerStateCanceled &&
		strings.TrimSpace(strings.ToLower(row.Reason)) == ChannelCircuitBreakerReasonInsufficientBalance
}

func GetChannelCircuitBreakerState(channelID string) (ChannelCircuitBreakerState, error) {
	return getChannelCircuitBreakerStateWithDB(DB, channelID)
}

func ListOpenChannelCircuitBreakerStates() ([]ChannelCircuitBreakerState, error) {
	return listOpenChannelCircuitBreakerStatesWithDB(DB)
}

func ListHalfOpenChannelCircuitBreakerStates() ([]ChannelCircuitBreakerState, error) {
	return listHalfOpenChannelCircuitBreakerStatesWithDB(DB)
}

func ListChannelCircuitBreakerStatesByChannelIDsWithDB(db *gorm.DB, channelIDs []string) ([]ChannelCircuitBreakerState, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedIDs := make([]string, 0, len(channelIDs))
	seen := make(map[string]struct{}, len(channelIDs))
	for _, channelID := range channelIDs {
		normalizedID := strings.TrimSpace(channelID)
		if normalizedID == "" {
			continue
		}
		if _, ok := seen[normalizedID]; ok {
			continue
		}
		seen[normalizedID] = struct{}{}
		normalizedIDs = append(normalizedIDs, normalizedID)
	}
	if len(normalizedIDs) == 0 {
		return []ChannelCircuitBreakerState{}, nil
	}
	rows := make([]ChannelCircuitBreakerState, 0, len(normalizedIDs))
	err := db.Where("channel_id IN ?", normalizedIDs).Find(&rows).Error
	return rows, err
}

func ListChannelCircuitBreakerEventsWithDB(db *gorm.DB, channelID string, limit int) ([]ChannelCircuitBreakerEvent, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil, fmt.Errorf("channel id is empty")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	rows := make([]ChannelCircuitBreakerEvent, 0, limit)
	err := db.Where("channel_id = ?", normalizedChannelID).
		Order("created_at desc, id desc").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func ListRecentChannelCircuitBreakerEventsWithDB(db *gorm.DB, limit int) ([]ChannelCircuitBreakerEvent, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	rows := make([]ChannelCircuitBreakerEvent, 0, limit)
	err := db.Order("created_at desc, id desc").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func recordChannelCircuitBreakerOpenWithDB(db *gorm.DB, channelID string, reason string, successRate float64, recoverAfter int64) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	now := helper.GetTimestamp()
	row := ChannelCircuitBreakerState{
		ChannelId:    normalizedChannelID,
		State:        ChannelCircuitBreakerStateOpen,
		Reason:       strings.TrimSpace(reason),
		SuccessRate:  successRate,
		DisabledAt:   now,
		RecoverAfter: recoverAfter,
		RecoveredAt:  0,
		UpdatedAt:    now,
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		return createChannelCircuitBreakerEventWithDB(tx, ChannelCircuitBreakerEvent{
			ChannelId:    normalizedChannelID,
			Event:        ChannelCircuitBreakerStateOpen,
			State:        ChannelCircuitBreakerStateOpen,
			Reason:       row.Reason,
			SuccessRate:  successRate,
			RecoverAfter: recoverAfter,
			CreatedAt:    now,
		})
	})
}

func updateChannelCircuitBreakerStateWithDB(db *gorm.DB, channelID string, state string, reason string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	normalizedState := strings.TrimSpace(state)
	if normalizedChannelID == "" || normalizedState == "" {
		return nil
	}
	now := helper.GetTimestamp()
	if normalizedState == ChannelCircuitBreakerStateCanceled {
		normalizedReason := strings.TrimSpace(reason)
		row := ChannelCircuitBreakerState{
			ChannelId:   normalizedChannelID,
			State:       normalizedState,
			Reason:      normalizedReason,
			DisabledAt:  now,
			RecoveredAt: 0,
			UpdatedAt:   now,
		}
		return db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Save(&row).Error; err != nil {
				return err
			}
			return createChannelCircuitBreakerEventWithDB(tx, ChannelCircuitBreakerEvent{
				ChannelId: normalizedChannelID,
				Event:     normalizedState,
				State:     normalizedState,
				Reason:    normalizedReason,
				CreatedAt: now,
			})
		})
	}
	updates := map[string]any{
		"state":      normalizedState,
		"updated_at": now,
	}
	if normalizedState == ChannelCircuitBreakerStateRecovered {
		updates["recovered_at"] = now
	}
	if normalizedReason := strings.TrimSpace(reason); normalizedReason != "" {
		updates["reason"] = normalizedReason
	}
	return db.Transaction(func(tx *gorm.DB) error {
		updateResult := tx.Model(&ChannelCircuitBreakerState{}).
			Where("channel_id = ? AND state IN ?", normalizedChannelID, []string{ChannelCircuitBreakerStateOpen, ChannelCircuitBreakerStateHalfOpen}).
			Updates(updates)
		if updateResult.Error != nil {
			return updateResult.Error
		}
		if updateResult.RowsAffected == 0 {
			return nil
		}
		return createChannelCircuitBreakerEventWithDB(tx, ChannelCircuitBreakerEvent{
			ChannelId: normalizedChannelID,
			Event:     normalizedState,
			State:     normalizedState,
			Reason:    strings.TrimSpace(reason),
			CreatedAt: now,
		})
	})
}

func createChannelCircuitBreakerEventWithDB(db *gorm.DB, event ChannelCircuitBreakerEvent) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	event.ChannelId = strings.TrimSpace(event.ChannelId)
	event.Event = strings.TrimSpace(event.Event)
	event.State = strings.TrimSpace(event.State)
	event.Reason = strings.TrimSpace(event.Reason)
	if event.ChannelId == "" || event.Event == "" {
		return nil
	}
	if event.CreatedAt <= 0 {
		event.CreatedAt = helper.GetTimestamp()
	}
	return db.Create(&event).Error
}

func getChannelCircuitBreakerStateWithDB(db *gorm.DB, channelID string) (ChannelCircuitBreakerState, error) {
	if db == nil {
		return ChannelCircuitBreakerState{}, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return ChannelCircuitBreakerState{}, fmt.Errorf("channel id is empty")
	}
	row := ChannelCircuitBreakerState{}
	err := db.First(&row, "channel_id = ?", normalizedChannelID).Error
	return row, err
}

func listOpenChannelCircuitBreakerStatesWithDB(db *gorm.DB) ([]ChannelCircuitBreakerState, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	rows := make([]ChannelCircuitBreakerState, 0)
	err := db.Where("state = ?", ChannelCircuitBreakerStateOpen).Find(&rows).Error
	return rows, err
}

func listHalfOpenChannelCircuitBreakerStatesWithDB(db *gorm.DB) ([]ChannelCircuitBreakerState, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	rows := make([]ChannelCircuitBreakerState, 0)
	err := db.Where("state = ?", ChannelCircuitBreakerStateHalfOpen).Find(&rows).Error
	return rows, err
}
