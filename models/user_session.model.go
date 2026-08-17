package models

import (
	"time"

	"gorm.io/gorm"
)

// UserSession is one device session (design D1). Sessions key on the
// polymorphic JWT user_id claim (client/personnel/center ID) — NOT
// auth_account_id, which is NULL for classic logins. Rotation creates a new
// generation row inside the same FamilyID; only the active generation has
// IsRevoked=false, so a replayed (already-rotated) token stays findable and
// triggers family revocation.
type UserSession struct {
	gorm.Model
	UserID           uint      `gorm:"index;not null" json:"user_id"`
	FamilyID         string    `gorm:"index;not null" json:"family_id"`
	RefreshTokenHash string    `gorm:"index;not null" json:"-"`
	ExpiresAt        time.Time `json:"expires_at"`
	IsRevoked        bool      `json:"is_revoked"`
	DeviceInfo       string    `json:"device_info,omitempty"`

	// Claim snapshot (D1 extension): Refresh re-issues the access token with
	// the same identity as the original login. A polymorphic user_id alone
	// cannot be resolved to role/center_id without a discriminator.
	Role     string `gorm:"size:50;not null" json:"-"`
	CenterID uint   `gorm:"default:0" json:"-"`
	Email    string `gorm:"size:70" json:"-"`
	Username string `gorm:"size:50" json:"-"`
}
