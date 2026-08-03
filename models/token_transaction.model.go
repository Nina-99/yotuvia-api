package models

import (
	"time"

	"gorm.io/gorm"
)

type TokenTransaction struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	BeautyCenterID uint         `gorm:"not null;index" json:"beauty_center_id"`
	BeautyCenter   BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"beauty_center,omitempty"`

	Amount      int64  `gorm:"type:bigint;not null" json:"amount"` // negative = consumption
	Type        string `gorm:"size:30;not null;index" json:"type"` // consumption, topup, bonus, adjustment, referral, mission
	ReferenceID *uint  `json:"reference_id,omitempty"`             // reservation_id, topup_id, mission_id, etc.

	BalanceBefore int64 `gorm:"type:bigint;not null" json:"balance_before"`
	BalanceAfter  int64 `gorm:"type:bigint;not null" json:"balance_after"`

	Description string `gorm:"size:255" json:"description"`
}
