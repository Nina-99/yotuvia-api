package models

import "gorm.io/gorm"

type AuthAccount struct {
	gorm.Model
	ClientID uint `gorm:"not null"`

	Provider   string `gorm:"size:20;not null" json:"provider"`
	ProviderID string `gorm:"size:255" json:"provider_id"`

	Client Client `gorm:"foreignKey:ClientID"`
}
