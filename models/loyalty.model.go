package models

import "gorm.io/gorm"

type LoyaltyProgram struct {
	gorm.Model
	BeautyCenterID   uint         `gorm:"not null" json:"beauty_center_id"`
	BeautyCenter     BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"-"`
	ServiceID        *uint        `json:"service_id"`
	Service          Service      `gorm:"foreignKey:ServiceID" json:"-"`
	RewardServiceID  *uint        `json:"reward_service_id"`
	RewardService    Service      `gorm:"foreignKey:RewardServiceID" json:"-"`

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
	Status       string `gorm:"size:20;default:'active'" json:"status"`

	// PersonnelID is set when LoyaltyPerPersonnel is enabled. The card is
	// scoped to a specific staff member — the client gets a separate card per
	// stylist.
	PersonnelID *uint     `json:"personnel_id,omitempty"`
	Personnel   Personnel `gorm:"foreignKey:PersonnelID" json:"-"`
}

type PunchHistory struct {
	gorm.Model
	CustomerPunchCardID uint              `gorm:"not null" json:"customer_punch_card_id"`
	CustomerPunchCard   CustomerPunchCard `json:"-"`
	ReservationID       uint              `gorm:"not null" json:"reservation_id"`
	Reservation         Reservation       `json:"-"`
	// PersonnelID is set when LoyaltyPerPersonnel is enabled on the beauty
	// center. It tracks which staff member performed the service for the stamp.
	PersonnelID *uint     `json:"personnel_id,omitempty"`
	Personnel   Personnel `gorm:"foreignKey:PersonnelID" json:"-"`
}

// LoyaltyRedemption tracks when a completed punch card reward is redeemed.
// After redemption the card resets to "active" with CurrentSteps = 0 so the
// client can start collecting stamps again.
type LoyaltyRedemption struct {
	gorm.Model
	CustomerPunchCardID uint              `gorm:"not null" json:"customer_punch_card_id"`
	CustomerPunchCard   CustomerPunchCard `json:"-"`
	ReservationID       uint              `gorm:"not null" json:"reservation_id"`
	Reservation         Reservation       `gorm:"foreignKey:ReservationID" json:"-"`
	DiscountApplied     float64           `gorm:"type:decimal(10,2);not null" json:"discount_applied"`
}
