package models

import (
	"time"

	"gorm.io/gorm"
)

type Reservation struct {
	gorm.Model
	BeautyCenterID uint `gorm:"not null"`
	ClientID       uint `gorm:"not null"`
	PersonnelID    uint `gorm:"not null"`
	ServiceID      uint `gorm:"not null"`

	Date              time.Time  `gorm:"type:date;not null" json:"date"`
	StartTime         string     `gorm:"size:5;not null" json:"start_time"`
	EndTime           string     `gorm:"size:5;not null" json:"end_time"`
	TotalPrice        float64    `gorm:"type:decimal(10,2)" json:"total_price"`
	Status            string     `gorm:"size:20;default:PENDING" json:"status"` // PENDING, CONFIRMED, CANCELLED, COMPLETED
	VerificationToken string     `gorm:"size:255;uniqueIndex" json:"verification_token,omitempty"`
	TokenExpirateAt   *time.Time `json:"token_expires_at,omitempty"`
}
