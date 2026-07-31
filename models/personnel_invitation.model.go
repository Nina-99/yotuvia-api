package models

import (
	"time"

	"gorm.io/gorm"
)

type PersonnelInvitation struct {
	gorm.Model
	BeautyCenterID uint       `gorm:"not null;index" json:"beauty_center_id"`
	Email          string     `gorm:"size:70;not null" json:"email"`
	Token          string     `gorm:"size:64;uniqueIndex;not null" json:"token"`
	ExpiresAt      time.Time  `gorm:"not null" json:"expires_at"`
	UsedAt         *time.Time `json:"used_at,omitempty"`
	Status         string     `gorm:"size:20;default:'pending'" json:"status"` // pending, accepted, expired, cancelled
}
