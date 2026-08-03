package models

import (
	"time"

	"gorm.io/gorm"
)

type TokenTopUp struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	BeautyCenterID uint         `gorm:"not null;index" json:"beauty_center_id"`
	BeautyCenter   BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"beauty_center,omitempty"`

	AmountBs       float64 `gorm:"type:decimal(12,2);not null" json:"amount_bs"`
	TokensReceived int64   `gorm:"type:bigint;not null" json:"tokens_received"`
	BonusTokens    int64   `gorm:"type:bigint;default:0" json:"bonus_tokens"`
	ReceiptURL     string  `gorm:"type:text;not null" json:"receipt_url"`
	Status         string  `gorm:"size:20;default:pending" json:"status"` // pending, approved, rejected

	ValidatedBy *uint      `gorm:"index" json:"validated_by,omitempty"`
	ValidatedAt *time.Time `json:"validated_at,omitempty"`
	Notes       *string    `gorm:"type:text" json:"notes,omitempty"`
}
