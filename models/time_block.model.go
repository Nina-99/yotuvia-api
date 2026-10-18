package models

import (
	"time"

	"gorm.io/gorm"
)

// TimeBlock is a stretch of time a professional or the whole centre is
// UNAVAILABLE (a break, a holiday, training…). It subtracts from availability
// exactly like a busy reservation does.
//
// Scope:
//   - PersonnelID == nil → the WHOLE centre is blocked (owner-level: holidays,
//     closed days). Applies to every professional.
//   - PersonnelID != nil → only that professional is blocked (their break). It
//     binds to a personnel of the SAME BeautyCenterID.
//
// A "whole day" block is simply a wide range (00:00–23:59); no special case.
type TimeBlock struct {
	gorm.Model
	BeautyCenterID uint  `gorm:"index;not null"`
	PersonnelID    *uint `gorm:"index"`

	Date      time.Time `gorm:"index;not null"`
	StartTime string    `gorm:"size:5;not null"`
	EndTime   string    `gorm:"size:5;not null"`
	Reason    string    `gorm:"size:255"`

	// CreatedByRole records who made the block ("personnel" | "beauty_center")
	// so ownership can be enforced without a second table.
	CreatedByRole string `gorm:"size:20"`
	// CreatedByID is the actor's id (personnel id or centre id) in that role.
	CreatedByID uint
}

// AppliesToPersonnel reports whether this block constrains the given
// professional. `personnelID == 0` means "any professional" (a centre-wide
// search): only a centre-wide block applies, because one professional's break
// must not hide the centre for the others.
func (b TimeBlock) AppliesToPersonnel(personnelID uint) bool {
	if b.PersonnelID == nil {
		return true
	}
	if personnelID == 0 {
		return false
	}
	return *b.PersonnelID == personnelID
}
