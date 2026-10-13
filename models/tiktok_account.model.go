package models

import (
	"time"

	"gorm.io/gorm"
)

// TikTokAccount is a center's (or one staff member's) connected TikTok account,
// obtained through Login Kit OAuth. Tokens are stored ENCRYPTED at rest (see
// utils.EncryptSecret) and are NEVER returned by the API.
//
// PersonnelID is 0 for a center-owned connection and the staff id otherwise, so
// the (BeautyCenterID, PersonnelID) pair is a valid unique key (a nullable
// column would not dedupe in Postgres, where NULLs are distinct).
type TikTokAccount struct {
	gorm.Model
	BeautyCenterID uint   `gorm:"uniqueIndex:idx_tiktok_actor"`
	PersonnelID    uint   `gorm:"uniqueIndex:idx_tiktok_actor"` // 0 = center-owned
	OpenID         string `gorm:"size:128;index"`
	DisplayName    string `gorm:"size:128"`
	AvatarURL      string `gorm:"size:512"`
	AccessToken    string `gorm:"type:text"` // encrypted
	RefreshToken   string `gorm:"type:text"` // encrypted
	Scope          string `gorm:"size:255"`

	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

// TikTokConnectionState is a single-use, short-lived CSRF state for the OAuth
// redirect. Storing it server-side keeps the flow stateless for the client and
// safe across multiple API instances.
type TikTokConnectionState struct {
	gorm.Model
	State          string `gorm:"size:128;uniqueIndex"`
	BeautyCenterID uint
	PersonnelID    uint
	ExpiresAt      time.Time `gorm:"index"`
}
