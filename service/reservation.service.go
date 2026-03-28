package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
)

type ReservationService interface {
	Create(req request.CreateReservationRequest) error
	Update(reservationID uint, req request.CreateReservationRequest) error
	Delete(reservationID uint) error
	FindAll() ([]models.Reservation, error)
	FindById(categoryID uint) (*models.Reservation, error)
}
