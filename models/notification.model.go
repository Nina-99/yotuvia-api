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

	// PersonnelID is nullable: staff notifications target a personnel row and
	// use a ClientID sentinel of 0, leaving client rows untouched (design D5).
	// AutoMigrate (configs/db.config.go) adds the column — no migration file.
	PersonnelID *uint `gorm:"index"`

	// BeautyCenterID is nullable: center notifications target a beauty center
	// row and combine the ClientID=0 sentinel with PersonnelID=nil, leaving
	// client and staff rows untouched (design D1). AutoMigrate adds the column
	// — no migration file.
	BeautyCenterID *uint `gorm:"index"`

	Client Client `gorm:"foreignKey:ClientID"`
}
