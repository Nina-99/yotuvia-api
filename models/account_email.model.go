package models

import "gorm.io/gorm"

// AccountEmail is the global email registry: exactly one row per email across
// every role (client, personnel, beauty centre, platform admin).
//
// The unique index on Email is the race-proof source of truth for "one account
// per email". The per-table unique indexes only protect a single role; this
// table protects the cross-role invariant that the login/registration flow
// depends on. Service-level checks remain as a fast, friendly pre-check.
type AccountEmail struct {
	gorm.Model
	Email    string `gorm:"size:70;not null;uniqueIndex" json:"email"`
	UserType string `gorm:"size:20;not null" json:"user_type"`
	// UserID is 0 between Reserve and BindUser; it is filled once the role row
	// exists. A row left at 0 means a creation failed and should have been
	// released.
	UserID uint `gorm:"not null;default:0" json:"user_id"`
}
