package models

import (
	"time"

	"gorm.io/gorm"
)

type TokenAccount struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	BeautyCenterID uint         `gorm:"not null;uniqueIndex" json:"beauty_center_id"`
	BeautyCenter   BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"beauty_center,omitempty"`

	Balance        int64   `gorm:"type:bigint;default:0" json:"balance"`
	TotalPurchased int64   `gorm:"type:bigint;default:0" json:"total_purchased"`
	TotalConsumed  int64   `gorm:"type:bigint;default:0" json:"total_consumed"`
	OverdraftLimit float64 `gorm:"-" json:"-"` // computed dynamically
	Currency       string  `gorm:"size:3;default:USD" json:"currency"`
}
