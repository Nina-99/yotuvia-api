package models

import "gorm.io/gorm"

type LoyaltyProgram struct {
	gorm.Model
	BeautyCenterID uint         `gorm:"not null" json:"beauty_center_id"`
	BeautyCenter   BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"-"`
	ServiceID      uint         `gorm:"not null" json:"service_id"`
	Service        Service      `gorm:"foreignKey:ServiceID" json:"-"`

	RequiredSteps int     `gorm:"not null;default:10" json:"required_steps"`
	RewardType    string  `gorm:"not null;size:50" json:"reward_type"`
	DiscountValue float64 `gorm:"type:decimal(5,2);default:0.0" json:"discount_value"`
	IsActive      bool    `gorm:"default:true" json:"is_active"`
}

type CustomerPunchCard struct {
	gorm.Model
	ClientID         uint           `gorm:"not null" json:"client_id"`
	Client           Client         `gorm:"foreignKey:ClientID" json:"-"`
	LoyaltyProgramID uint           `gorm:"not null" json:"loyalty_program_id"`
	LoyaltyProgram   LoyaltyProgram `gorm:"foreignKey:LoyaltyProgramID" json:"loyalty_program"`

	CurrentSteps int    `gorm:"not null;default:0" json:"current_steps"`
	Status       string `gorm:"size:20;default:'active'" json:"status"` // "active", "completed", "claimed"
}

type PunchHistory struct {
	CustomerPunchCardID uint              `gorm:"not null" json:"customer_punch_card_id"`
	CustomerPunchCard   CustomerPunchCard `gorm:"foreignKey" json:"-"`
	ReservationID       uint              `gorm:"not null" json:"reservation_id"`
	Reservation         Reservation       `gorm:"foreignKey" json:"-"`
}
