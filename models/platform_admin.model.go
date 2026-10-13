package models

import "gorm.io/gorm"

// PlatformAdmin is the platform-level principal: it belongs to NO beauty centre
// and oversees all of them (validating token top-ups across centres and managing
// each centre's payment QR).
//
// Why a dedicated table instead of an AuthAccount branch:
//   - AuthAccount is a provider/social link record whose identity is keyed by the
//     polymorphic Client/Personnel/BeautyCenter FKs. A fourth identity there
//     would need another nullable FK and would thread a new branch through every
//     login/social/verification/recovery flow that resolves "who is this".
//   - The platform admin is an OPERATIONAL principal, never a self-registered
//     user: it has no social login, no public registration and no email-recovery
//     surface. Keeping it out of AuthAccount means the register endpoints and
//     their `oneof` validation stay untouched, and the account can ONLY be
//     created by the idempotent env-driven seed.
//
// The table is additive: AutoMigrate creates it without touching existing
// tables.
type PlatformAdmin struct {
	gorm.Model
	Email string `gorm:"size:70;not null;unique_index" json:"email"`
	// Password is the bcrypt hash. Never serialised: the hash must not reach the
	// wire on any platform response.
	Password string `gorm:"size:255;not null" json:"-"`
	Name     string `gorm:"size:100;not null" json:"name"`
	IsActive bool   `gorm:"default:true" json:"is_active"`
}
