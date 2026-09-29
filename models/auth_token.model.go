package models

import (
	"time"

	"gorm.io/gorm"
)

// AuthToken purpose discriminators. A purpose is part of the lookup key so a
// token minted for one flow can never satisfy the other, even if an attacker
// mixes them up.
const (
	TokenPurposeEmailVerification = "email_verification"
	TokenPurposePasswordReset     = "password_reset"
)

// AuthToken is a single-use, short-lived, opaque token backing the
// email-verification and password-recovery flows.
//
// Why a dedicated table instead of columns on AuthAccount:
//   - A user can own multiple outstanding tokens (e.g. resend), and each must
//     be individually invalidated/consumed. Columns force a per-account single
//     slot and cannot express history.
//   - Password reset applies to social accounts too, which may have no local
//     AuthAccount row; keying on (user_type, user_id) covers both flows without
//     a foreign key that would be NULL half the time.
//   - Token state is separate lifecycle data (TTL, single-use, audit) that does
//     not belong on the account identity record.
//
// UserID is polymorphic (client / personnel / beauty_center id) exactly like
// UserSession.UserID, so it is always read together with UserType.
// AccountID optionally points at the local AuthAccount that owns the
// EmailVerifiedAt stamp; it is present for verification tokens and may be nil
// for reset tokens.
type AuthToken struct {
	gorm.Model
	UserType  string `gorm:"size:20;index;not null" json:"user_type"`
	UserID    uint   `gorm:"index;not null" json:"user_id"`
	AccountID *uint  `gorm:"index" json:"-"`
	Purpose   string `gorm:"size:30;index;not null" json:"purpose"`

	// TokenHash is the SHA-256 hex digest of the raw token. The raw value is
	// handed to the client exactly once (in the email link) and never stored.
	TokenHash string `gorm:"index;not null" json:"-"`

	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
}

// IsUsed reports whether the token was already redeemed.
func (t *AuthToken) IsUsed() bool { return t.UsedAt != nil }

// IsExpired reports whether the token is past its TTL.
func (t *AuthToken) IsExpired() bool { return time.Now().After(t.ExpiresAt) }
