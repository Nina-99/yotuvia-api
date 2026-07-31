package models

import "gorm.io/gorm"

type PushDevice struct {
	gorm.Model
	ClientID  uint   `gorm:"not null;index"`
	PushToken string `gorm:"size:255;not null;uniqueIndex"`
	Platform  string `gorm:"size:20"` // ios, android

	Client Client `gorm:"foreignKey:ClientID"`
}
