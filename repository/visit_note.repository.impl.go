package repository

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type VisitNoteRepositoryImpl struct {
	db *gorm.DB
}

func NewVisitNoteRepositoryImpl(db *gorm.DB) VisitNoteRepository {
	return &VisitNoteRepositoryImpl{db: db}
}

func (r *VisitNoteRepositoryImpl) Upsert(note *models.VisitNote) error {
	// OnConflict on the author key: a second write for the same
	// (reservation, role, actor) updates the body in place instead of failing on
	// the unique index, so the endpoint is idempotent and race-safe.
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "reservation_id"},
			{Name: "created_by_role"},
			{Name: "created_by_id"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"body":       note.Body,
			"updated_at": time.Now(),
		}),
	}).Create(note).Error
}

func (r *VisitNoteRepositoryImpl) FindByAuthor(reservationID uint, role string, actorID uint) (*models.VisitNote, error) {
	var note models.VisitNote
	err := r.db.
		Where("reservation_id = ? AND created_by_role = ? AND created_by_id = ?", reservationID, role, actorID).
		First(&note).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &note, nil
}

func (r *VisitNoteRepositoryImpl) ListByAuthorRange(role string, actorID uint, from, to time.Time) ([]models.VisitNote, error) {
	var notes []models.VisitNote
	err := r.db.
		Joins("JOIN reservations ON reservations.id = visit_notes.reservation_id AND reservations.deleted_at IS NULL").
		Where("visit_notes.created_by_role = ? AND visit_notes.created_by_id = ?", role, actorID).
		Where("reservations.date >= ? AND reservations.date <= ?", dateOnly(from), dateOnly(to)).
		Find(&notes).Error
	return notes, err
}

func (r *VisitNoteRepositoryImpl) DeleteByAuthor(reservationID uint, role string, actorID uint) error {
	// Hard delete: a soft-deleted row would keep occupying the author unique
	// index and block a later re-create for the same visit.
	return r.db.
		Unscoped().
		Where("reservation_id = ? AND created_by_role = ? AND created_by_id = ?", reservationID, role, actorID).
		Delete(&models.VisitNote{}).Error
}
