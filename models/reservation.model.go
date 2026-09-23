package models

import (
	"time"

	"gorm.io/gorm"
)

type Reservation struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	BeautyCenterID uint         `gorm:"not null" json:"beauty_center_id"`
	BeautyCenter   BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"beauty_center,omitempty"`
	ClientID       uint         `gorm:"not null" json:"client_id"`
	Client         Client       `gorm:"foreignKey:ClientID" json:"client,omitempty"`
	PersonnelID    uint         `gorm:"not null" json:"personnel_id"`
	Personnel      Personnel    `gorm:"foreignKey:PersonnelID" json:"personnel,omitempty"`

	Date       time.Time `gorm:"type:date;not null" json:"date"`
	StartTime  string    `gorm:"size:5;not null" json:"start_time"`
	EndTime    string    `gorm:"size:5;not null" json:"end_time"`
	TotalPrice float64 `gorm:"type:decimal(10,2)" json:"total_price"`
	// LoyaltyDiscount is the amount subtracted from TotalPrice due to a
	// loyalty program reward redemption at booking time.
	LoyaltyDiscount float64 `gorm:"type:decimal(10,2);default:0" json:"loyalty_discount"`
	// PromotionDiscount is the amount subtracted from TotalPrice due to an
	// active promotion applied at booking time.
	PromotionDiscount float64 `gorm:"type:decimal(10,2);default:0" json:"promotion_discount"`
	Status            string  `gorm:"size:20;default:PENDING" json:"status"`
	// PaymentStatus tracks the zero-cost deposit ("seña") flow: NONE when no
	// deposit is required, PENDING_PROOF after the client uploads the payment
	// receipt, APPROVED/REJECTED after center review. See Phase 1 plan.
	PaymentStatus string `gorm:"size:20;default:NONE" json:"payment_status"`
	// ProofImageURL is the uploaded bank receipt photo for the deposit.
	ProofImageURL *string `gorm:"size:500" json:"proof_image_url,omitempty"`
	// DepositAmount snapshots the seña due at booking time (fixed or computed
	// from percent), immune to later config changes. 0 = no deposit required.
	DepositAmount float64 `gorm:"type:decimal(10,2);default:0" json:"deposit_amount"`
	// HoldExpiresAt is the deadline of the unconfirmed slot hold. A nil value
	// means no active hold. Expiry is evaluated lazily on read plus a ticker.
	HoldExpiresAt *time.Time `json:"hold_expires_at,omitempty"`
	// Reminder24HSentAt / Reminder2HSentAt mark the one-shot push reminders
	// (Phase 3). Nil = not sent yet. Columns instead of a table: two flags
	// are cheaper than a join and idempotency is a NULL check.
	Reminder24HSentAt *time.Time `json:"reminder_24h_sent_at,omitempty"`
	Reminder2HSentAt  *time.Time `json:"reminder_2h_sent_at,omitempty"`

	CancelReason *string `gorm:"type:text" json:"cancel_reason,omitempty"`

	VerificationToken *string    `gorm:"size:255;uniqueIndex" json:"verification_token,omitempty"`
	TokenExpiresAt    *time.Time `json:"token_expires_at,omitempty"`

	ReservationServices []ReservationService `gorm:"foreignKey:ReservationID" json:"services,omitempty"`
}
