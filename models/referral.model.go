package models

import (
	"time"

	"gorm.io/gorm"
)

type Referral struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	ReferrerID uint         `gorm:"not null;index" json:"referrer_id"`
	Referrer   BeautyCenter `gorm:"foreignKey:ReferrerID" json:"referrer,omitempty"`
	ReferredID uint         `gorm:"not null;uniqueIndex" json:"referred_id"`
	Referred   BeautyCenter `gorm:"foreignKey:ReferredID" json:"referred,omitempty"`

	Status         string     `gorm:"size:20;default:pending" json:"status"` // pending, credited, expired
	RewardReferrer float64    `gorm:"type:decimal(12,2);default:0" json:"reward_referrer"`
	RewardReferred float64    `gorm:"type:decimal(12,2);default:0" json:"reward_referred"`
	CreditedAt     *time.Time `json:"credited_at,omitempty"`
}
