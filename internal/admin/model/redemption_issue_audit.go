package model

import (
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
)

type RedemptionIssueAuditLog struct {
	ID                 uint    `json:"id" gorm:"primaryKey"`
	BatchID            string  `json:"batch_id" gorm:"type:char(36);not null;index"`
	CreatedByUserID    string  `json:"created_by_user_id" gorm:"type:char(36);default:'';index"`
	Name               string  `json:"name" gorm:"type:varchar(255);default:''"`
	GroupID            string  `json:"group_id" gorm:"type:char(36);default:'';index"`
	Count              int     `json:"count" gorm:"type:int;not null;default:0"`
	FaceValueAmount    float64 `json:"face_value_amount" gorm:"type:numeric(30,8);not null;default:0"`
	FaceValueUnit      string  `json:"face_value_unit" gorm:"type:varchar(16);not null;default:'YYC'"`
	Quota              int64   `json:"quota" gorm:"bigint;default:0"`
	CodeValidityDays   int     `json:"code_validity_days" gorm:"type:int;not null;default:0"`
	CreditValidityDays int     `json:"credit_validity_days" gorm:"type:int;not null;default:0"`
	FirstCode          string  `json:"first_code" gorm:"type:varchar(64);default:''"`
	LastCode           string  `json:"last_code" gorm:"type:varchar(64);default:''"`
	CreatedAt          int64   `json:"created_at" gorm:"bigint;index"`
}

func (RedemptionIssueAuditLog) TableName() string {
	return "redemption_issue_audit_logs"
}

func RecordRedemptionIssueAuditLogWithDB(db *gorm.DB, row RedemptionIssueAuditLog) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	row.BatchID = strings.TrimSpace(row.BatchID)
	row.CreatedByUserID = strings.TrimSpace(row.CreatedByUserID)
	row.Name = strings.TrimSpace(row.Name)
	row.GroupID = strings.TrimSpace(row.GroupID)
	row.FaceValueUnit = normalizeRedemptionFaceValueUnit(row.FaceValueUnit)
	row.FirstCode = strings.TrimSpace(row.FirstCode)
	row.LastCode = strings.TrimSpace(row.LastCode)
	if row.BatchID == "" {
		return fmt.Errorf("redemption issue batch id is empty")
	}
	if row.Count <= 0 {
		return fmt.Errorf("redemption issue count must be positive")
	}
	if row.CreatedAt <= 0 {
		row.CreatedAt = helper.GetTimestamp()
	}
	return db.Create(&row).Error
}
