package service

import "github.com/Nina-99/yotuvia-api/models"

// VisitNoteService manages the internal per-visit notes a professional records
// about their own appointments (see models.VisitNote).
//
// Every operation is authorized by MEMBERSHIP, not by centre ownership: the
// caller must be the visit's lead or a member of one of its service lines. The
// same rule C1 (reschedule) uses, so a professional can never note a visit they
// did not serve.
type VisitNoteService interface {
	// Upsert stores the caller's note for the visit, replacing any previous
	// note of theirs on the same visit.
	Upsert(reservationID uint, role string, actorID, centerID uint, body string) (*models.VisitNote, error)
	// Delete removes the caller's note for the visit. A missing note is a no-op.
	Delete(reservationID uint, role string, actorID, centerID uint) error
	// ListMine returns the caller's notes on visits whose date falls within
	// [from, to], so a client view can merge them by reservation id.
	ListMine(role string, actorID, centerID uint, from, to string) ([]models.VisitNote, error)
}
