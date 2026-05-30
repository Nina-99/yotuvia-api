package models

import (
	"time"

	"gorm.io/gorm"
)

type CenterMembership struct {
	gorm.Model
	BeautyCenterID uint         `gorm:"index" json:"beauty_center_id"`
	BeautyCenter   BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"beauty_center"`
	MembershipID   uint         `gorm:"not null" json:"membership_id"`
	Membership     Membership   `gorm:"foreignKey:MembershipID" json:"membership,omitempty"`

	Status     string `gorm:"type:varchar(20);default:'provisional'" json:"status"` // "provisional", "active", "rejected"
	ReceiptURL string `gorm:"type:text;not null" json:"receipt_url"`

	StartDate          time.Time `gorm:"not null" json:"start_date"`
	EndDate            time.Time `gorm:"not null" json:"end_date"`
	ProvisionalEndDate time.Time `gorm:"not null" json:"provisional_end_date"`
}
