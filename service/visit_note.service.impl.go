package service

import (
	"strings"
	"time"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
)

type VisitNoteServiceImpl struct {
	repo            repository.VisitNoteRepository
	reservationRepo repository.ReservationRepository
}

func NewVisitNoteService(
	repo repository.VisitNoteRepository,
	reservationRepo repository.ReservationRepository,
) VisitNoteService {
	return &VisitNoteServiceImpl{repo: repo, reservationRepo: reservationRepo}
}

// authorize loads the visit and proves the caller may note it:
//
//   - BOTH roles require the reservation to belong to the caller's centre.
//   - A professional must additionally be the visit's LEAD or one of its service
//     lines, so they can never note a colleague's visit.
//   - The owner may note any visit of its own centre.
//
// The author key is (role, actorID), so a staff member and the owner keep
// separate notes on the same visit.
func (s *VisitNoteServiceImpl) authorize(reservationID uint, role string, actorID, centerID uint) error {
	reservation, err := s.reservationRepo.FindById(reservationID)
	if err != nil || reservation == nil {
		return apperrors.ErrNotFound
	}
	if reservation.BeautyCenterID != centerID {
		return apperrors.ErrForbidden
	}

	switch role {
	case "beauty_center":
		return nil
	case "personnel":
		if reservation.PersonnelID == actorID {
			return nil
		}
		member, err := s.reservationRepo.IsPersonnelMember(reservationID, actorID)
		if err != nil {
			return err
		}
		if !member {
			return apperrors.ErrForbidden
		}
		return nil
	default:
		return apperrors.ErrForbidden
	}
}

func (s *VisitNoteServiceImpl) Upsert(reservationID uint, role string, actorID, centerID uint, body string) (*models.VisitNote, error) {
	text := strings.TrimSpace(body)
	if text == "" {
		return nil, apperrors.ErrBadRequest
	}
	if err := s.authorize(reservationID, role, actorID, centerID); err != nil {
		return nil, err
	}

	note := &models.VisitNote{
		ReservationID: reservationID,
		Body:          text,
		CreatedByRole: role,
		CreatedByID:   actorID,
	}
	if err := s.repo.Upsert(note); err != nil {
		return nil, err
	}
	// Re-read so the response carries the persisted id/timestamps regardless of
	// whether the upsert inserted or updated.
	return s.repo.FindByAuthor(reservationID, role, actorID)
}

func (s *VisitNoteServiceImpl) Delete(reservationID uint, role string, actorID, centerID uint) error {
	if err := s.authorize(reservationID, role, actorID, centerID); err != nil {
		return err
	}
	return s.repo.DeleteByAuthor(reservationID, role, actorID)
}

func (s *VisitNoteServiceImpl) List(role string, actorID, centerID uint, from, to string) ([]models.VisitNote, error) {
	if role != "personnel" && role != "beauty_center" {
		return nil, apperrors.ErrForbidden
	}
	fromDay, err := time.Parse("2006-01-02", strings.TrimSpace(from))
	if err != nil {
		return nil, apperrors.ErrBadRequest
	}
	toDay, err := time.Parse("2006-01-02", strings.TrimSpace(to))
	if err != nil {
		return nil, apperrors.ErrBadRequest
	}
	if toDay.Before(fromDay) {
		return nil, apperrors.ErrBadRequest
	}
	return s.repo.ListByAuthorRange(role, actorID, fromDay, toDay)
}
