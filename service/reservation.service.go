package service

import (
	"time"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
)

type ReservationService interface {
	Create(req request.CreateReservationRequest) error
	// Update/Delete/FindById/GenerateVerificationToken take actor params
	// (clientID, centerID, role claims) and enforce ownership: a client may act
	// on its own reservation, a center on its own center's reservation.
	Update(reservationID uint, req request.CreateReservationRequest, clientID, centerID uint, role string) error
	Delete(reservationID uint, clientID, centerID uint, role string) error
	UpdateStatus(reservationID uint, status string, centerID uint) error
	Cancel(reservationID uint, clientID uint, reason string) error
	// FindAll is scoped to the caller's centerID claim (beauty_center only).
	FindAll(page, limit int, centerID uint) ([]models.Reservation, int64, error)
	FindById(reservationID uint, clientID, centerID uint, role string) (*models.Reservation, error)
	// FindByClientID/FindUpcomingByClientID reject a path clientID that does not
	// match the actor's clientID claim.
	FindByClientID(clientID, actorClientID uint) ([]models.Reservation, error)
	FindUpcomingByClientID(clientID, actorClientID uint) (*models.Reservation, error)
	// FindByBeautyCenterID allows center staff only for their own centerID.
	// The optional date (parsed YYYY-MM-DD) scopes the agenda to one day; a
	// nil date returns all reservations unfiltered.
	FindByBeautyCenterID(beautyCenterID, actorCenterID uint, date *time.Time) ([]models.Reservation, error)
	// FindByPersonnelID returns a staff member's agenda. The personnelID is the
	// caller's own claim (never a path parameter); from/to are an optional
	// inclusive date range (nil = open-ended).
	FindByPersonnelID(personnelID uint, from, to *time.Time) ([]models.Reservation, error)
	GenerateVerificationToken(reservationID uint, clientID, centerID uint, role string) (string, error)
	// VerifyQR completes a reservation exactly once and reports whether the
	// reservation was ALREADY COMPLETED before this call: false when this call
	// performed the completion, true for an idempotent re-scan or a lost
	// exactly-once race.
	VerifyQR(token string, authBeautyCenterID uint) (*models.Reservation, bool, error)
	AssignPersonnel(reservationID uint, personnelID uint, centerID uint) error
}
