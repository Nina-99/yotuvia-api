package service

import (
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
)

type ReservationServiceImpl struct {
	ReservationRepository repository.ReservationRepository
	LoyaltyService        LoyaltyService
	Validate              *validator.Validate
}

func NewReservationServiceImpl(reservationRepository repository.ReservationRepository, loyaltyService LoyaltyService, validate *validator.Validate) ReservationService {
	return &ReservationServiceImpl{
		ReservationRepository: reservationRepository,
		LoyaltyService:        loyaltyService,
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

func (s *ReservationServiceImpl) FindAll(page, limit int) ([]models.Reservation, int64, error) {
	return s.ReservationRepository.FindAll(page, limit)
}

func (s *ReservationServiceImpl) FindById(reservationID uint) (*models.Reservation, error) {
	return s.ReservationRepository.FindById(reservationID)
}

// Helper to generate a secure and fast token
func generateSecureToken() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ23456789"
	result := make([]byte, 6)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}

// GenerateVerificationToken implements on-the-fly QR generaation with a 30-minute expiration
func (s *ReservationServiceImpl) GenerateVerificationToken(reservationID uint) (string, error) {
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil {
		return "", errors.New("reservation not found")
	}

	if reservation.Status == "completed" || reservation.Status == "cancelled" {
		return "", errors.New("cannot generate QR for a completed or cancelled reservation")
	}

	var token string
	for {
		token = generateSecureToken()

		existing, _ := s.ReservationRepository.FindByToken(token)
		if existing == nil {
			break
		}
	}

	expiration := time.Now().Add(30 * time.Minute)

	reservation.VerificationToken = token
	reservation.TokenExpirateAt = &expiration

	if err := s.ReservationRepository.Update(reservation); err != nil {
		return "", err
	}

	utils.Log.Infof("[QR] Dynamic QR generated for Reservation ID %d. Expires at: %s", reservation.ID, expiration.Format("15:04:05"))
	return token, nil
}

func (s *ReservationServiceImpl) VerifyQR(token string, authBeautyCenterID uint) error {
	reservation, err := s.ReservationRepository.FindByToken(token)
	if err != nil || reservation == nil {
		return errors.New("invalid or expired QR code")
	}

	if reservation.BeautyCenterID != authBeautyCenterID {
		utils.Log.Warnf("[SECURITY ALERT] Intrusion attempt! Salon ID %d tried to verify QR for Reservation ID %d (owned by Salon %d)", authBeautyCenterID, reservation.ID, reservation.BeautyCenterID)
		return errors.New("unauthorized: this reservation does not belong to your brauty center")
	}

	if reservation.TokenExpirateAt != nil && time.Now().After(*reservation.TokenExpirateAt) {
		return errors.New("QR code has expired. Please refresh the app to generate a new one")
	}

	if reservation.Status == "completed" {
		return errors.New("reservation is already completed")
	}

	reservation.Status = "completed"
	reservation.VerificationToken = ""
	reservation.TokenExpirateAt = nil

	if err := s.ReservationRepository.Update(reservation); err != nil {
		return err
	}

	err = s.LoyaltyService.AddPunch(reservation.ID, reservation.ClientID, reservation.BeautyCenterID, reservation.ServiceID)
	if err != nil {
		utils.Log.Errorf("[Loyalty Error] falied to add punch via QR for Client %d; %v", reservation.ClientID, err)
		// We are not blocking the response to the client, the service has already been successfully provide
	}

	utils.Log.Infof("[QR VERIFIED] Reservation ID %d marked as COMPLETED. Stamp added to Client", reservation.ID, reservation.ClientID)
	return nil
}
