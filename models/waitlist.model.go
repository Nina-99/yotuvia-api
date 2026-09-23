package models

import (
	"time"

	"gorm.io/gorm"
)

// WaitlistEntry is one client's place in line for a concrete full slot
// (center + date + start time). Phase 3 waitlist with automatic backfill.
//
// Lifecycle: WAITING → OFFERED → BOOKED (claimed) | EXPIRED (window lapsed,
// the offer passes to the next waiter) | CANCELLED (client leaves).
// FIFO is CreatedAt order within (center, date, start).
type WaitlistEntry struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	BeautyCenterID uint         `gorm:"not null;index" json:"beauty_center_id"`
	BeautyCenter   BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"beauty_center,omitempty"`
	ClientID       uint         `gorm:"not null;index" json:"client_id"`
	Client         Client       `gorm:"foreignKey:ClientID" json:"client,omitempty"`

	Date        time.Time `gorm:"type:date;not null" json:"date"`
	StartTime   string    `gorm:"size:5;not null" json:"start_time"`
	EndTime     string    `gorm:"size:5;not null" json:"end_time"`
	PersonnelID uint      `gorm:"not null" json:"personnel_id"`
	// ServiceIDs is the CSV of service IDs to book on claim (e.g. "4,7").
	ServiceIDs string `gorm:"size:100;not null" json:"service_ids"`

	Status string `gorm:"size:20;default:WAITING" json:"status"`
	// OfferExpiresAt bounds the 15-minute claim window while OFFERED.
	OfferedAt      *time.Time `json:"offered_at,omitempty"`
	OfferExpiresAt *time.Time `json:"offer_expires_at,omitempty"`
}

// Waitlist statuses.
const (
	WaitlistWaiting   = "WAITING"
	WaitlistOffered   = "OFFERED"
	WaitlistBooked    = "BOOKED"
	WaitlistExpired   = "EXPIRED"
	WaitlistCancelled = "CANCELLED"
)

// WaitlistOfferMinutes is the claim window for an offered slot.
const WaitlistOfferMinutes = 15
