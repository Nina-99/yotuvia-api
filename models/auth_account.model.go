package models

import (
	"time"

	"gorm.io/gorm"
)

type AuthAccount struct {
	gorm.Model
	Provider   string `gorm:"size:20;not null" json:"provider"`
	ProviderID string `gorm:"size:255" json:"provider_id"`

	UserType string `gorm:"size:255;not null" json:"user_type"`

	// TermsAcceptedAt records the moment the account holder accepted the
	// platform Terms. NULL means acceptance was never captured. It is stamped
	// by the server at registration, never supplied by the client.
	TermsAcceptedAt *time.Time `json:"terms_accepted_at,omitempty"`
	// TermsVersion is the server-authoritative version of the Terms the user
	// accepted (TERMS_VERSION env). The client cannot dictate it.
	TermsVersion string `gorm:"size:20" json:"terms_version,omitempty"`

	ClientID       *uint         `json:"client_id,omitempty"`
	Client         *Client       `gorm:"foreignKey:ClientID" json:"-"`
	PersonnelID    *uint         `json:"personnel_id,omitempty"`
	Personnel      *Personnel    `gorm:"foreignKey:PersonnelID" json:"-"`
	BeautyCenterID *uint         `json:"beauty_center_id,omitempty"`
	BeautyCenter   *BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"-"`
}
