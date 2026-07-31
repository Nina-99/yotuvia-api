package models

import (
	"time"

	"gorm.io/gorm"
)

type Reservation struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	BeautyCenterID uint         `gorm:"not null" json:"beauty_center_id"`
	BeautyCenter   BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"beauty_center,omitempty"`
	ClientID       uint         `gorm:"not null" json:"client_id"`
	Client         Client       `gorm:"foreignKey:ClientID" json:"client,omitempty"`
	PersonnelID    uint         `gorm:"not null" json:"personnel_id"`
	Personnel      Personnel    `gorm:"foreignKey:PersonnelID" json:"personnel,omitempty"`

	Date       time.Time `gorm:"type:date;not null" json:"date"`
	StartTime  string    `gorm:"size:5;not null" json:"start_time"`
	EndTime    string    `gorm:"size:5;not null" json:"end_time"`
	TotalPrice float64   `gorm:"type:decimal(10,2)" json:"total_price"`
	Status     string    `gorm:"size:20;default:PENDING" json:"status"`

	CancelReason *string `gorm:"type:text" json:"cancel_reason,omitempty"`

	VerificationToken *string    `gorm:"size:255;uniqueIndex" json:"verification_token,omitempty"`
	TokenExpiresAt    *time.Time `json:"token_expires_at,omitempty"`

	ReservationServices []ReservationService `gorm:"foreignKey:ReservationID" json:"services,omitempty"`
}
