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
	BeautyCenterID uint  `gorm:"index;not null" json:"beauty_center_id"`
	PersonnelID    *uint `gorm:"index" json:"personnel_id"`

	Date      time.Time `gorm:"index;not null" json:"date"`
	StartTime string    `gorm:"size:5;not null" json:"start_time"`
	EndTime   string    `gorm:"size:5;not null" json:"end_time"`
	Reason    string    `gorm:"size:255" json:"reason"`

	// CreatedByRole records who made the block ("personnel" | "beauty_center")
	// so ownership can be enforced without a second table.
	CreatedByRole string `gorm:"size:20" json:"created_by_role"`
	// CreatedByID is the actor's id (personnel id or centre id) in that role.
	CreatedByID uint `json:"created_by_id"`
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
