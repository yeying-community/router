package model

import (
	"strings"
)

const (
	IdentityPasskeyLoginSessionStatusPending  = "pending"
	IdentityPasskeyLoginSessionStatusApproved = "approved"
	IdentityPasskeyLoginSessionStatusComplete = "complete"
	IdentityPasskeyLoginSessionStatusFailed   = "failed"
)

// IdentityPasskeyLoginSession contains server-side PKCE material for a short-lived
// no-wallet-plugin login attempt via Node identity authorization. The verifier
// must never be sent to the browser.
type IdentityPasskeyLoginSession struct {
	SessionID    string `gorm:"type:char(64);primaryKey"`
	State        string `gorm:"type:char(64);not null;uniqueIndex"`
	RequestID    string `gorm:"type:varchar(128);not null;uniqueIndex"`
	CodeVerifier string `gorm:"type:varchar(128);not null"`
	Status       string `gorm:"type:varchar(32);not null;index"`
	Code         string `gorm:"type:varchar(256);not null;default:''"`
	UserID       string `gorm:"type:char(36);not null;default:'';index"`
	ErrorMessage string `gorm:"type:varchar(255);not null;default:''"`
	ExpiresAt    int64  `gorm:"bigint;not null;index"`
	CreatedAt    int64  `gorm:"bigint;index"`
	UpdatedAt    int64  `gorm:"bigint;index"`
}

func (IdentityPasskeyLoginSession) TableName() string { return "identity_passkey_login_sessions" }

func FindIdentityPasskeyLoginSessionByID(sessionID string) (*IdentityPasskeyLoginSession, error) {
	row := &IdentityPasskeyLoginSession{}
	if err := DB.Where("session_id = ?", strings.TrimSpace(sessionID)).First(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

func FindIdentityPasskeyLoginSessionByState(state string) (*IdentityPasskeyLoginSession, error) {
	row := &IdentityPasskeyLoginSession{}
	if err := DB.Where("state = ?", strings.TrimSpace(state)).First(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}
