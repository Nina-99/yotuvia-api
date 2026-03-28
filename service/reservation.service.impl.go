package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/go-playground/validator/v10"
)

type ReservationServiceImpl struct {
	ReservationRepository repository.ReservationRepository
	Validate              *validator.Validate
}

func NewReservationServiceImpl(reservationRepository repository.ReservationRepository, validate *validator.Validate) ReservationService {
	return &ReservationServiceImpl{
		ReservationRepository: reservationRepository,
		Validate:              validate,
	}
}

// Create implements ReservationService
func (s *ReservationServiceImpl) Create(req request.CreateReservationRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}
	// Check for overlap
	overlap, err := s.ReservationRepository.CheckOverlap(req.BeautyCenterID, req.Date, req.StartTime, req.EndTime, 0)
	if err != nil {
		return err
	}
	if overlap {
		return errors.New("overlapping reservation exists")
	}

	reservation := models.Reservation{
		BeautyCenterID: req.BeautyCenterID,
		ClientID:       req.ClientID,
		PersonnelID:    req.PersonnelID,
		ServiceID:      req.ServiceID,
		Date:           req.Date,
		StartTime:      req.StartTime,
		EndTime:        req.EndTime,
		Status:         req.Status,
	}

	return s.ReservationRepository.Create(&reservation)
}

// Update implements ReservationService
func (s *ReservationServiceImpl) Update(reservationID uint, req request.CreateReservationRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}

	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return errors.New("reservation not found")
	}

	// Check for overlap if date or time changed
	if reservation.Date != req.Date || reservation.StartTime != req.StartTime || reservation.EndTime != req.EndTime || reservation.BeautyCenterID != req.BeautyCenterID {
		overlap, err := s.ReservationRepository.CheckOverlap(req.BeautyCenterID, req.Date, req.StartTime, req.EndTime, reservationID)
		if err != nil {
			return err
		}
		if overlap {
			return errors.New("overlapping reservation exists")
		}
	}

	reservation.BeautyCenterID = req.BeautyCenterID
	reservation.ClientID = req.ClientID
	reservation.PersonnelID = req.PersonnelID
	reservation.ServiceID = req.ServiceID
	reservation.Date = req.Date
	reservation.StartTime = req.StartTime
	reservation.EndTime = req.EndTime
	reservation.Status = req.Status

	return s.ReservationRepository.Update(reservation)
}

// Delete reservation implements ReservationService
func (s *ReservationServiceImpl) Delete(reservationID uint) error {
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return errors.New("reservation not found")
	}

	return s.ReservationRepository.Delete(reservation)
}

func (s *ReservationServiceImpl) FindAll() ([]models.Reservation, error) {
	return s.ReservationRepository.FindAll()
}

func (s *ReservationServiceImpl) FindById(reservationID uint) (*models.Reservation, error) {
	return s.ReservationRepository.FindById(reservationID)
}
