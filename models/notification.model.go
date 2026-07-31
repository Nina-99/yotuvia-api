package models

import (
	"gorm.io/gorm"
	"time"
)

type Notification struct {
	gorm.Model
	ClientID uint   `gorm:"not null;index"`
	Type     string `gorm:"size:50;not null"` // personnel_changed, appointment_reminder, etc.
	Title    string `gorm:"size:255;not null"`
	Message  string `gorm:"size:1000;not null"`
	Data     string `gorm:"type:jsonb"` // JSON payload, e.g. {"reservation_id": 42}
	IsRead   bool   `gorm:"default:false"`
	ReadAt   *time.Time

	Client Client `gorm:"foreignKey:ClientID"`
}
