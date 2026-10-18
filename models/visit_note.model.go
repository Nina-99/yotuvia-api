package models

import "gorm.io/gorm"

// VisitNote is a short free-text note a professional records about ONE visit
// (a reservation). It is an INTERNAL staff note: it never reaches the client.
//
// Scope:
//   - One note per (reservation, author): the unique index lets the author
//     upsert their own note without a read-modify-write race against others.
//   - The author is identified by role + id (the same polymorphic shape
//     TimeBlock uses) so a staff member and the owner can each keep their own.
//
// Notes are author-scoped on purpose: two professionals who served the same
// visit never overwrite each other.
type VisitNote struct {
	gorm.Model

	ReservationID uint `gorm:"not null;index;uniqueIndex:idx_visit_note_author" json:"reservation_id"`

	Body string `gorm:"type:text" json:"body"`

	// CreatedByRole is "personnel" | "beauty_center"; CreatedByID is the actor's
	// id in that role. Together with ReservationID they form the uniqueness key.
	CreatedByRole string `gorm:"size:20;not null;uniqueIndex:idx_visit_note_author" json:"created_by_role"`
	CreatedByID   uint   `gorm:"not null;uniqueIndex:idx_visit_note_author" json:"created_by_id"`
}
