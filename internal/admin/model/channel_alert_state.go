package model

import (
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
)

const (
	ChannelAlertStatesTableName = "channel_alert_states"

	ChannelAlertTypeBilling          = "billing"
	ChannelAlertTypeCircuitBreaker   = "circuit"
	ChannelAlertTypeModelDisabled    = "model_disabled"
	ChannelAlertTypeEndpointDisabled = "endpoint_disabled"

	ChannelAlertStatusActive       = "active"
	ChannelAlertStatusAcknowledged = "acknowledged"
	ChannelAlertStatusResolved     = "resolved"
)

type ChannelAlertState struct {
	AlertType        string `json:"alert_type" gorm:"primaryKey;type:varchar(64)"`
	AlertKey         string `json:"alert_key" gorm:"primaryKey;type:varchar(191)"`
	ChannelId        string `json:"channel_id" gorm:"type:char(36);not null;default:'';index"`
	Status           string `json:"status" gorm:"type:varchar(32);not null;default:'active';index"`
	AcknowledgedAt   int64  `json:"acknowledged_at" gorm:"bigint;not null;default:0;index"`
	AcknowledgedBy   string `json:"acknowledged_by" gorm:"type:char(36);not null;default:'';index"`
	ResolvedAt       int64  `json:"resolved_at" gorm:"bigint;not null;default:0;index"`
	ResolvedBy       string `json:"resolved_by" gorm:"type:char(36);not null;default:'';index"`
	LastOperatorNote string `json:"last_operator_note" gorm:"type:text"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;not null;default:0;index"`
	UpdatedAt        int64  `json:"updated_at" gorm:"bigint;not null;default:0;index"`
}

func (ChannelAlertState) TableName() string {
	return ChannelAlertStatesTableName
}

type ChannelAlertStateRef struct {
	AlertType string
	AlertKey  string
}

func normalizeChannelAlertType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case ChannelAlertTypeBilling:
		return ChannelAlertTypeBilling
	case ChannelAlertTypeCircuitBreaker:
		return ChannelAlertTypeCircuitBreaker
	case ChannelAlertTypeModelDisabled:
		return ChannelAlertTypeModelDisabled
	case ChannelAlertTypeEndpointDisabled:
		return ChannelAlertTypeEndpointDisabled
	default:
		return ""
	}
}

func normalizeChannelAlertStatus(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case ChannelAlertStatusAcknowledged:
		return ChannelAlertStatusAcknowledged
	case ChannelAlertStatusResolved:
		return ChannelAlertStatusResolved
	default:
		return ChannelAlertStatusActive
	}
}

func isChannelAlertStatesTableReady(db *gorm.DB) bool {
	return db != nil && db.Migrator().HasTable(&ChannelAlertState{})
}

func GetChannelAlertStatesByRefsWithDB(db *gorm.DB, refs []ChannelAlertStateRef) (map[string]ChannelAlertState, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	if !isChannelAlertStatesTableReady(db) {
		return map[string]ChannelAlertState{}, nil
	}
	typeCandidates := make([]string, 0, len(refs))
	keyCandidates := make([]string, 0, len(refs))
	allowed := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		alertType := normalizeChannelAlertType(ref.AlertType)
		alertKey := strings.TrimSpace(ref.AlertKey)
		if alertType == "" || alertKey == "" {
			continue
		}
		typeCandidates = append(typeCandidates, alertType)
		keyCandidates = append(keyCandidates, alertKey)
		allowed[alertType+"::"+alertKey] = struct{}{}
	}
	if len(allowed) == 0 {
		return map[string]ChannelAlertState{}, nil
	}
	rows := make([]ChannelAlertState, 0, len(allowed))
	if err := db.Where("alert_type IN ? AND alert_key IN ?", typeCandidates, keyCandidates).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]ChannelAlertState, len(rows))
	for _, row := range rows {
		key := normalizeChannelAlertType(row.AlertType) + "::" + strings.TrimSpace(row.AlertKey)
		if _, ok := allowed[key]; !ok {
			continue
		}
		row.AlertType = normalizeChannelAlertType(row.AlertType)
		row.AlertKey = strings.TrimSpace(row.AlertKey)
		row.ChannelId = strings.TrimSpace(row.ChannelId)
		row.Status = normalizeChannelAlertStatus(row.Status)
		row.AcknowledgedBy = strings.TrimSpace(row.AcknowledgedBy)
		row.ResolvedBy = strings.TrimSpace(row.ResolvedBy)
		row.LastOperatorNote = strings.TrimSpace(row.LastOperatorNote)
		result[key] = row
	}
	return result, nil
}

func AcknowledgeChannelAlertStateWithDB(db *gorm.DB, ref ChannelAlertStateRef, channelID string, operatorUserID string, note string) (ChannelAlertState, error) {
	if db == nil {
		return ChannelAlertState{}, fmt.Errorf("database handle is nil")
	}
	if !isChannelAlertStatesTableReady(db) {
		return ChannelAlertState{}, fmt.Errorf("channel alert state table is not ready")
	}
	alertType := normalizeChannelAlertType(ref.AlertType)
	alertKey := strings.TrimSpace(ref.AlertKey)
	if alertType == "" || alertKey == "" {
		return ChannelAlertState{}, fmt.Errorf("invalid alert reference")
	}
	now := helper.GetTimestamp()
	row := ChannelAlertState{}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("alert_type = ? AND alert_key = ?", alertType, alertKey).First(&row).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				return err
			}
			row = ChannelAlertState{
				AlertType: alertType,
				AlertKey:  alertKey,
				ChannelId: strings.TrimSpace(channelID),
				Status:    ChannelAlertStatusAcknowledged,
				CreatedAt: now,
				UpdatedAt: now,
			}
		}
		row.ChannelId = strings.TrimSpace(channelID)
		row.Status = ChannelAlertStatusAcknowledged
		row.AcknowledgedAt = now
		row.AcknowledgedBy = strings.TrimSpace(operatorUserID)
		row.LastOperatorNote = strings.TrimSpace(note)
		row.UpdatedAt = now
		return tx.Save(&row).Error
	})
	if err != nil {
		return ChannelAlertState{}, err
	}
	return row, nil
}

func ResolveChannelAlertStateWithDB(db *gorm.DB, ref ChannelAlertStateRef, channelID string, operatorUserID string, note string) (ChannelAlertState, error) {
	if db == nil {
		return ChannelAlertState{}, fmt.Errorf("database handle is nil")
	}
	if !isChannelAlertStatesTableReady(db) {
		return ChannelAlertState{}, fmt.Errorf("channel alert state table is not ready")
	}
	alertType := normalizeChannelAlertType(ref.AlertType)
	alertKey := strings.TrimSpace(ref.AlertKey)
	if alertType == "" || alertKey == "" {
		return ChannelAlertState{}, fmt.Errorf("invalid alert reference")
	}
	now := helper.GetTimestamp()
	row := ChannelAlertState{}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("alert_type = ? AND alert_key = ?", alertType, alertKey).First(&row).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				return err
			}
			row = ChannelAlertState{
				AlertType: alertType,
				AlertKey:  alertKey,
				ChannelId: strings.TrimSpace(channelID),
				CreatedAt: now,
			}
		}
		row.ChannelId = strings.TrimSpace(channelID)
		row.Status = ChannelAlertStatusResolved
		if row.AcknowledgedAt <= 0 {
			row.AcknowledgedAt = now
		}
		if strings.TrimSpace(row.AcknowledgedBy) == "" {
			row.AcknowledgedBy = strings.TrimSpace(operatorUserID)
		}
		row.ResolvedAt = now
		row.ResolvedBy = strings.TrimSpace(operatorUserID)
		row.LastOperatorNote = strings.TrimSpace(note)
		row.UpdatedAt = now
		return tx.Save(&row).Error
	})
	if err != nil {
		return ChannelAlertState{}, err
	}
	return row, nil
}
