package service

import "github.com/Nina-99/yotuvia-api/models"

// VisitNoteService manages the internal per-visit notes a professional OR the
// centre owner records about their appointments (see models.VisitNote).
//
// Every operation is authorized from the JWT: a professional may note only the
// visits they SERVED (lead or a member of one of its service lines — the same
// rule C1 uses), while the owner may note any visit of its own centre. The note
// is keyed by (role, actor), so the two never overwrite each other.
type VisitNoteService interface {
	// Upsert stores the caller's note for the visit, replacing any previous
	// note of theirs on the same visit.
	Upsert(reservationID uint, role string, actorID, centerID uint, body string) (*models.VisitNote, error)
	// Delete removes the caller's note for the visit. A missing note is a no-op.
	Delete(reservationID uint, role string, actorID, centerID uint) error
	// List returns the caller's notes on visits whose date falls within
	// [from, to], so a client view can merge them by reservation id. Scoped to
	// the CALLER: a professional sees their own, the owner theirs.
	List(role string, actorID, centerID uint, from, to string) ([]models.VisitNote, error)
}
