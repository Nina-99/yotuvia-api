package models

import (
	"time"

	"gorm.io/gorm"
)

type ReservationService struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	ReservationID uint        `gorm:"not null;index" json:"reservation_id"`
	Reservation   Reservation `gorm:"foreignKey:ReservationID;constraint:OnDelete:CASCADE"`
	ServiceID     uint        `gorm:"not null" json:"service_id"`
	Service       Service     `gorm:"foreignKey:ServiceID" json:"service,omitempty"`
	ServiceName   string      `gorm:"size:100;not null" json:"service_name"`
	Price         float64     `gorm:"type:decimal(10,2);not null" json:"price"`
	Duration      int         `gorm:"not null" json:"duration"`
}
