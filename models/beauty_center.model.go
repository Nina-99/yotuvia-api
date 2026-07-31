package models

import (
	"time"

	"gorm.io/gorm"
)

type BeautyCenter struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	Name         string  `gorm:"size:50;not null" json:"name"`
	Address      string  `gorm:"size:255" json:"address"`
	Email        string  `gorm:"size:70;not null;uniqueIndex" json:"email"`
	Phone        string  `gorm:"size:20;not null" json:"phone"`
	Url          string  `gorm:"unique_index" json:"url"`
	Description  string  `gorm:"size:255" json:"description"`
	Latitude     float64 `gorm:"default:0;index:idx_bc_location" json:"latitude"`
	Longitude    float64 `gorm:"default:0;index:idx_bc_location,priority:2" json:"longitude"`
	Password     string  `gorm:"size:255" json:"-"`
	Currency     string  `gorm:"size:3;default:USD" json:"currency"`
	IsActive     bool    `gorm:"default:true" json:"is_active"`
	ReferralCode string  `gorm:"size:20;uniqueIndex" json:"referral_code"`

	AvgRating   float64 `gorm:"-" json:"avg_rating"`
	ReviewCount int64   `gorm:"-" json:"review_count"`

	BeautyCenterImage []BeautyCenterImage `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Personnel         []Personnel         `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Reservation       []Reservation       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	SocialMedia       []SocialMedia       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	WorkingHour       []WorkingHour       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}
