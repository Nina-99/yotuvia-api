package repository

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
)

// VisitNoteRepository persists internal per-visit notes authored by a
// professional (or the owner).
//
// Every read is AUTHOR-scoped: a caller only ever sees their own notes, so the
// repository never exposes another professional's notes.
type VisitNoteRepository interface {
	// Upsert inserts or updates the author's note for one reservation, keyed by
	// (reservation, role, actor). It is race-safe: the unique index makes a
	// concurrent double-insert collapse to one row.
	Upsert(note *models.VisitNote) error
	// FindByAuthor returns the author's note for one reservation, or nil when
	// none exists.
	FindByAuthor(reservationID uint, role string, actorID uint) (*models.VisitNote, error)
	// ListByAuthorRange returns the author's notes whose reservation Date falls
	// within [from, to].
	ListByAuthorRange(role string, actorID uint, from, to time.Time) ([]models.VisitNote, error)
	// DeleteByAuthor removes the author's note for one reservation. A missing
	// row is not an error.
	DeleteByAuthor(reservationID uint, role string, actorID uint) error
}
