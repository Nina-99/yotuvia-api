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

	Balance        int64 `gorm:"type:bigint;default:0" json:"balance"`
	TotalPurchased int64 `gorm:"type:bigint;default:0" json:"total_purchased"`
	TotalConsumed  int64 `gorm:"type:bigint;default:0" json:"total_consumed"`
	// ReservedMilli is the part of Balance held for prepaid campaigns. It is not
	// money movement: Balance still equals the ledger sum, and available funds
	// are Balance - ReservedMilli.
	ReservedMilli  int64   `gorm:"type:bigint;default:0" json:"reserved"`
	OverdraftLimit float64 `gorm:"-" json:"-"` // computed dynamically
	Currency       string  `gorm:"size:3;default:USD" json:"currency"`

	// DeficitSince records when the centre ran out of REAL headroom (balance
	// minus holds, plus overdraft, at or below zero). It is the clock behind the
	// discovery grace window: a centre that stays in deficit past the grace is
	// removed from search/ranking/nearby, because it can no longer settle the
	// commission of a new visit. Cleared by the first top-up that restores a
	// positive available balance.
	DeficitSince *time.Time `gorm:"index" json:"-"`

	// LowBalanceNotifiedAt dedupes the "you are running out of Ytv" alert: it is
	// stamped when that notification is sent and cleared once the available
	// balance recovers, so a second dip notifies again but a single episode
	// never spams the owner on every completed visit.
	LowBalanceNotifiedAt *time.Time `json:"-"`
	// DeficitNotifiedAt dedupes the "your centre cannot take new reservations"
	// alert for one deficit episode.
	DeficitNotifiedAt *time.Time `json:"-"`
	// SuspendedAt marks a centre suspended for staying in deficit past the
	// suspension window. Unlike is_active (the owner's own switch), it is managed
	// by the platform and cleared automatically when a top-up restores funds.
	SuspendedAt *time.Time `gorm:"index" json:"-"`
}
