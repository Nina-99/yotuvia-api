package models

import (
	"time"

	"gorm.io/gorm"
)

// OtpChallenge is a single-use email login code.
//
// It is keyed by email rather than by user on purpose: the same flow must serve
// an unknown address without revealing that it is unknown, so there is no user
// row to hang the challenge from. Only the SHA-256 digest of the code is stored;
// the raw value exists only in the email.
type OtpChallenge struct {
	gorm.Model
	Email     string     `gorm:"size:70;index;not null" json:"email"`
	CodeHash  string     `gorm:"size:64;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"not null" json:"expires_at"`
	Attempts  int        `gorm:"default:0" json:"attempts"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
}
