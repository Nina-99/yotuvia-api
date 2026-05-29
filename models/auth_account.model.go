package models

import "gorm.io/gorm"

type AuthAccount struct {
	gorm.Model
	Provider   string `gorm:"size:20;not null" json:"provider"`
	ProviderID string `gorm:"size:255" json:"provider_id"`

	UserType string `gorm:"size:255;not null" json:"user_type"`

	ClientID       *uint         `json:"client_id,omitempty"`
	Client         *Client       `gorm:"foreignKey:ClientID" json:"-"`
	PersonnelID    *uint         `json:"personnel_id,omitempty"`
	Personnel      *Personnel    `gorm:"foreignKey:PersonnelID" json:"-"`
	BeautyCenterID *uint         `json:"beauty_center_id,omitempty"`
	BeautyCenter   *BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"-"`
}
