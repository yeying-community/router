package model

import (
	"strings"

	"gorm.io/gorm"
)

const (
	IdentityLoginSessionStatusPending  = "pending"
	IdentityLoginSessionStatusComplete = "complete"
	IdentityLoginSessionStatusFailed   = "failed"
)

// IdentityLoginSession is a short-lived server-side session for wallet
// identity login. It stores the challenge nonce and audience that the
// wallet signs against. No PKCE code exchange is needed because the
// presentation is verified locally.
type IdentityLoginSession struct {
	SessionID string `gorm:"type:char(64);primaryKey" json:"session_id"`
	Nonce     string `gorm:"type:varchar(128);not null" json:"nonce"`
	Audience  string `gorm:"type:varchar(512);not null;default:''" json:"audience"`
	AppID     string `gorm:"type:varchar(128);not null;default:''" json:"app_id"`
	Scopes    string `gorm:"type:varchar(256);not null;default:''" json:"scopes"`
	Status    string `gorm:"type:varchar(32);not null;index" json:"status"`
	UserID    string `gorm:"type:char(36);not null;default:'';index" json:"user_id"`
	ExpiresAt int64  `gorm:"bigint;not null;index" json:"expires_at"`
	CreatedAt int64  `gorm:"bigint;index" json:"created_at"`
	UpdatedAt int64  `gorm:"bigint;index" json:"updated_at"`
}

func (IdentityLoginSession) TableName() string { return "identity_login_sessions" }

func FindIdentityLoginSessionByID(sessionID string) (*IdentityLoginSession, error) {
	row := &IdentityLoginSession{}
	if err := DB.Where("session_id = ?", strings.TrimSpace(sessionID)).First(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// SyncIdentityEmail updates the user's email from a verified identity credential.
// This stores the verified email from wallet identity credentials on the user.
func SyncIdentityEmailWithDB(db *gorm.DB, userID string, email string) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return gorm.ErrInvalidData
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	return db.Model(&User{}).Where("id = ?", userID).
		Update("email", normalizedEmail).Error
}

func SyncIdentityEmail(userID string, email string) error {
	return SyncIdentityEmailWithDB(DB, userID, email)
}
