package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
)

type ReservationService interface {
	Create(req request.CreateReservationRequest) error
	Update(reservationID uint, req request.CreateReservationRequest) error
	Delete(reservationID uint) error
	UpdateStatus(reservationID uint, status string) error
	Cancel(reservationID uint, clientID uint, reason string) error
	FindAll(page, limit int) ([]models.Reservation, int64, error)
	FindById(categoryID uint) (*models.Reservation, error)
	FindByClientID(clientID uint) ([]models.Reservation, error)
	FindUpcomingByClientID(clientID uint) (*models.Reservation, error)
	FindByBeautyCenterID(beautyCenterID uint) ([]models.Reservation, error)
	GenerateVerificationToken(reservationID uint) (string, error)
	VerifyQR(token string, authBeautyCenterID uint) error
	AssignPersonnel(reservationID uint, personnelID uint) error
}
