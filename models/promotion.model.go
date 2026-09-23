package models

import (
	"time"

	"gorm.io/gorm"
)

// Promotion represents a discount or special offer on one or more services.
// It can be time-bound, hour-restricted (off-peak), and targeted at specific
// client segments.
type Promotion struct {
	gorm.Model
	BeautyCenterID uint         `gorm:"not null;index" json:"beauty_center_id"`
	BeautyCenter   BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"-"`

	Name        string  `gorm:"size:100;not null" json:"name"`
	Description string  `gorm:"type:text" json:"description"`

	// Type determines how the discount is calculated:
	//   "percentage"      — DiscountValue% off the service price
	//   "fixed_amount"    — DiscountValue currency units off
	//   "buy_x_get_y"     — Buy X services, get Y free (BuyXGetYCount = Y)
	Type        string  `gorm:"size:30;not null" json:"type"`
	DiscountValue float64 `gorm:"type:decimal(10,2);default:0" json:"discount_value"`
	BuyXCount   int     `gorm:"default:1" json:"buy_x_count"`
	BuyYCount   int     `gorm:"default:1" json:"buy_y_count"`

	// ServiceID is optional. When set, the promo applies only to this service.
	// When nil, the promo applies to ALL services in the beauty center.
	ServiceID *uint  `json:"service_id,omitempty"`
	Service   Service `gorm:"foreignKey:ServiceID" json:"service,omitempty"`

	// Validity window (optional — nil means always valid)
	ValidFrom *time.Time `json:"valid_from,omitempty"`
	ValidTo   *time.Time `json:"valid_to,omitempty"`

	// ApplicableHours is a JSON array of time ranges when the promo is active.
	// Example: [{"start":"09:00","end":"12:00"},{"start":"14:00","end":"17:00"}]
	// Empty/nil means the promo applies at all hours.
	ApplicableHours string `gorm:"type:json" json:"applicable_hours,omitempty"`

	// TargetClients restricts which clients can use this promo:
	//   "all"      — everyone
	//   "new"      — clients with 0 completed reservations
	//   "inactive" — clients who haven't visited in 30+ days
	TargetClients string `gorm:"size:20;default:'all'" json:"target_clients"`

	// Redemption limits
	MaxRedemptions    int `gorm:"default:0" json:"max_redemptions"`     // 0 = unlimited
	CurrentRedemptions int `gorm:"default:0" json:"current_redemptions"`

	IsActive bool `gorm:"default:true" json:"is_active"`
}

// PromotionRedemption tracks each use of a promotion on a reservation.
type PromotionRedemption struct {
	gorm.Model
	PromotionID    uint        `gorm:"not null;index" json:"promotion_id"`
	Promotion      Promotion   `gorm:"foreignKey:PromotionID" json:"-"`
	ReservationID  uint        `gorm:"not null;index" json:"reservation_id"`
	Reservation    Reservation `gorm:"foreignKey:ReservationID" json:"-"`
	DiscountApplied float64   `gorm:"type:decimal(10,2);not null" json:"discount_applied"`
}
