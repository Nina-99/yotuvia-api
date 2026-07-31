package service

import (
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
	FindByBeautyCenterID(beautyCenterID, actorCenterID uint) ([]models.Reservation, error)
	GenerateVerificationToken(reservationID uint, clientID, centerID uint, role string) (string, error)
	VerifyQR(token string, authBeautyCenterID uint) error
	AssignPersonnel(reservationID uint, personnelID uint, centerID uint) error
}
