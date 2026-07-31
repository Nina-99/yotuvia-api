package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
)

type ReservationServiceImpl struct {
	PersonnelRepository    repository.PersonnelRepository
	ReservationRepository  repository.ReservationRepository
	ServiceRepository      repository.ServiceRepository
	NotificationService    NotificationService
	LoyaltyService         LoyaltyService
	TokenAccountService    TokenAccountService
	MissionService         TokenMissionService
	Validate               *validator.Validate
}

func NewReservationServiceImpl(
	reservationRepository repository.ReservationRepository,
	serviceRepository repository.ServiceRepository,
	personnelRepository repository.PersonnelRepository,
	notificationService NotificationService,
	loyaltyService LoyaltyService,
	tokenAccountService TokenAccountService,
	missionService TokenMissionService,
	validate *validator.Validate,
) ReservationService {
	return &ReservationServiceImpl{
		ReservationRepository: reservationRepository,
		PersonnelRepository:   personnelRepository,
		ServiceRepository:     serviceRepository,
		NotificationService:   notificationService,
		LoyaltyService:        loyaltyService,
		TokenAccountService:   tokenAccountService,
		MissionService:        missionService,
		Validate:              validate,
	}
}

// ──────────────────────────────────────────────
//  CREATE — Multi-servicio
// ──────────────────────────────────────────────

func (s *ReservationServiceImpl) Create(req request.CreateReservationRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}

	// Parse date string into time.Time
	parsedDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return errors.New("invalid date format, use YYYY-MM-DD")
	}

	// Fetch all services and validate
	services, err := s.ServiceRepository.FindByIDs(req.ServiceIDs)
	if err != nil {
		return errors.New("one or more services not found")
	}
	if len(services) != len(req.ServiceIDs) {
		return errors.New("one or more services not found")
	}

	// All services must belong to the same beauty center
	for _, svc := range services {
		if svc.BeautyCenterID != req.BeautyCenterID {
			return errors.New("all services must belong to the same beauty center")
		}
	}

	// Calculate totals
	var totalPrice float64
	var totalDuration int
	for _, svc := range services {
		totalPrice += svc.Price
		totalDuration += svc.Duration
	}

	// Calculate end time from start time + total duration
	startParsed, err := time.Parse("15:04", req.StartTime)
	if err != nil {
		return errors.New("invalid start_time format, use HH:MM")
	}
	endTime := startParsed.Add(time.Duration(totalDuration) * time.Minute)
	endTimeStr := endTime.Format("15:04")

	// Check for overlaps
	overlap, err := s.ReservationRepository.CheckOverlap(req.BeautyCenterID, parsedDate, req.StartTime, endTimeStr, req.PersonnelID, 0)
	if err != nil {
		return err
	}
	if overlap {
		return errors.New("overlapping reservation exists")
	}

	// Build reservation with services in a transaction
	reservation := models.Reservation{
		BeautyCenterID: req.BeautyCenterID,
		ClientID:       req.ClientID,
		PersonnelID:    req.PersonnelID,
		Date:           parsedDate,
		StartTime:      req.StartTime,
		EndTime:        endTimeStr,
		TotalPrice:     totalPrice,
		Status:         "PENDING",
	}

	for _, svc := range services {
		reservation.ReservationServices = append(reservation.ReservationServices, models.ReservationService{
			ServiceID:   svc.ID,
			ServiceName: svc.Name,
			Price:       svc.Price,
			Duration:    svc.Duration,
		})
	}

	return s.ReservationRepository.Create(&reservation)
}

// ──────────────────────────────────────────────
//  UPDATE
// ──────────────────────────────────────────────

func (s *ReservationServiceImpl) Update(reservationID uint, req request.CreateReservationRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}

	// Parse date string into time.Time
	parsedDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return errors.New("invalid date format, use YYYY-MM-DD")
	}

	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return errors.New("reservation not found")
	}

	// Fetch services and recalculate
	services, err := s.ServiceRepository.FindByIDs(req.ServiceIDs)
	if err != nil || len(services) != len(req.ServiceIDs) {
		return errors.New("one or more services not found")
	}

	var totalPrice float64
	var totalDuration int
	for _, svc := range services {
		if svc.BeautyCenterID != req.BeautyCenterID {
			return errors.New("all services must belong to the same beauty center")
		}
		totalPrice += svc.Price
		totalDuration += svc.Duration
	}

	startParsed, _ := time.Parse("15:04", req.StartTime)
	endTimeStr := startParsed.Add(time.Duration(totalDuration) * time.Minute).Format("15:04")

	// Check overlap excluding current reservation
	if !reservation.Date.Equal(parsedDate) || reservation.StartTime != req.StartTime || endTimeStr != reservation.EndTime || reservation.BeautyCenterID != req.BeautyCenterID {
		overlap, err := s.ReservationRepository.CheckOverlap(req.BeautyCenterID, parsedDate, req.StartTime, endTimeStr, req.PersonnelID, reservationID)
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
	reservation.Date = parsedDate
	reservation.StartTime = req.StartTime
	reservation.EndTime = endTimeStr
	reservation.TotalPrice = totalPrice

	// Replace services: clear old, add new
	if err := s.ReservationRepository.DeleteReservationServices(reservationID); err != nil {
		return err
	}
	reservation.ReservationServices = nil
	for _, svc := range services {
		reservation.ReservationServices = append(reservation.ReservationServices, models.ReservationService{
			ServiceID:   svc.ID,
			ServiceName: svc.Name,
			Price:       svc.Price,
			Duration:    svc.Duration,
		})
	}

	return s.ReservationRepository.Update(reservation)
}

// ──────────────────────────────────────────────
//  ASSIGN PERSONNEL
// ──────────────────────────────────────────────

func (s *ReservationServiceImpl) AssignPersonnel(reservationID uint, personnelID uint) error {
	// Find the reservation
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return errors.New("reservation not found")
	}

	// Cannot reassign completed or cancelled
	if reservation.Status == "COMPLETED" || reservation.Status == "CANCELLED" {
		return errors.New("cannot reassign personnel for " + reservation.Status + " reservations")
	}

	// Verify personnel exists and belongs to the same beauty center
	personnel, err := s.PersonnelRepository.FindByID(personnelID)
	if err != nil || personnel == nil {
		return errors.New("personnel not found")
	}
	if personnel.BeautyCenterID != reservation.BeautyCenterID {
		return errors.New("personnel does not belong to the same beauty center")
	}

	// Check for overlaps with the new personnel
	overlap, err := s.ReservationRepository.CheckOverlap(
		reservation.BeautyCenterID,
		reservation.Date,
		reservation.StartTime,
		reservation.EndTime,
		personnelID,
		reservationID,
	)
	if err != nil {
		return err
	}
	if overlap {
		return errors.New("the new personnel has an overlapping reservation at the same time")
	}

	// Update personnel directly
	if err := s.ReservationRepository.UpdatePersonnelID(reservationID, personnelID); err != nil {
		return err
	}

	// Create notification for the client
	personnelNew, _ := s.PersonnelRepository.FindByID(personnelID)
	if personnelNew != nil {
		dataStr := fmt.Sprintf(`{"reservation_id":%d,"personnel_id":%d}`, reservationID, personnelID)
		title := "Cambio de profesional"
		message := fmt.Sprintf("Tu turno del %s a las %s ahora será con %s %s",
			reservation.Date.Format("02/01"),
			reservation.StartTime,
			personnelNew.FirstName,
			personnelNew.LastName,
		)
		_ = s.NotificationService.Create(reservation.ClientID, "personnel_changed", title, message, dataStr)
		_ = s.NotificationService.SendPushToClient(reservation.ClientID, title, message, dataStr)
	}

	return nil
}

// ──────────────────────────────────────────────
//  UPDATE STATUS
// ──────────────────────────────────────────────

func (s *ReservationServiceImpl) UpdateStatus(reservationID uint, status string) error {
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return errors.New("reservation not found")
	}

	// State machine: valid transitions
	// PENDING   → CONFIRMED, CANCELLED
	// CONFIRMED → COMPLETED, CANCELLED
	// COMPLETED → (terminal)
	// CANCELLED → (terminal)
	validTransitions := map[string]map[string]bool{
		"PENDING":   {"CONFIRMED": true, "CANCELLED": true},
		"CONFIRMED": {"COMPLETED": true, "CANCELLED": true},
	}

	allowed, ok := validTransitions[reservation.Status]
	if !ok {
		return errors.New("reservation is already " + reservation.Status + " and cannot be changed")
	}
	if !allowed[status] {
		return errors.New("cannot transition from " + reservation.Status + " to " + status)
	}

	reservation.Status = status
	return s.ReservationRepository.Update(reservation)
}

// ──────────────────────────────────────────────
//  CANCEL — Client cancels own reservation
// ──────────────────────────────────────────────

func (s *ReservationServiceImpl) Cancel(reservationID uint, clientID uint, reason string) error {
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return errors.New("reservation not found")
	}

	if reservation.ClientID != clientID {
		return errors.New("you can only cancel your own reservations")
	}

	if reservation.Status != "PENDING" && reservation.Status != "CONFIRMED" {
		return errors.New("only PENDING or CONFIRMED reservations can be cancelled")
	}

	reservation.Status = "CANCELLED"
	if reason != "" {
		reservation.CancelReason = &reason
	}

	return s.ReservationRepository.Update(reservation)
}

// ──────────────────────────────────────────────
//  DELETE
// ──────────────────────────────────────────────

func (s *ReservationServiceImpl) Delete(reservationID uint) error {
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return errors.New("reservation not found")
	}
	return s.ReservationRepository.Delete(reservation)
}

// ──────────────────────────────────────────────
//  FIND
// ──────────────────────────────────────────────

func (s *ReservationServiceImpl) FindAll(page, limit int) ([]models.Reservation, int64, error) {
	return s.ReservationRepository.FindAll(page, limit)
}

func (s *ReservationServiceImpl) FindById(reservationID uint) (*models.Reservation, error) {
	return s.ReservationRepository.FindById(reservationID)
}

func (s *ReservationServiceImpl) FindByClientID(clientID uint) ([]models.Reservation, error) {
	return s.ReservationRepository.FindByClientID(clientID)
}

func (s *ReservationServiceImpl) FindUpcomingByClientID(clientID uint) (*models.Reservation, error) {
	return s.ReservationRepository.FindUpcomingByClientID(clientID)
}

func (s *ReservationServiceImpl) FindByBeautyCenterID(beautyCenterID uint) ([]models.Reservation, error) {
	return s.ReservationRepository.FindByBeautyCenterId(beautyCenterID)
}

// ──────────────────────────────────────────────
//  QR / VERIFICATION
// ──────────────────────────────────────────────

func generateSecureToken() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ23456789"
	result := make([]byte, 6)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}

func (s *ReservationServiceImpl) GenerateVerificationToken(reservationID uint) (string, error) {
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
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
	reservation.VerificationToken = &token
	reservation.TokenExpiresAt = &expiration

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
		return errors.New("unauthorized: this reservation does not belong to your beauty center")
	}

	if reservation.TokenExpiresAt != nil && time.Now().After(*reservation.TokenExpiresAt) {
		return errors.New("QR code has expired. Please refresh the app to generate a new one")
	}

	if reservation.Status == "completed" {
		return errors.New("reservation is already completed")
	}

	// ── Token consumption check ──
	charge := reservation.TotalPrice * 0.035
	canConsume, err := s.TokenAccountService.CanConsume(authBeautyCenterID, charge)
	if err != nil {
		return errors.New("failed to check token balance: " + err.Error())
	}
	if !canConsume {
		return errors.New("insufficient YOTTU balance. Please top up to complete this reservation")
	}

	reservation.Status = "completed"
	reservation.VerificationToken = nil
	reservation.TokenExpiresAt = nil

	if err := s.ReservationRepository.Update(reservation); err != nil {
		return err
	}

	// ── Deduct YOTTUs ──
	consumeErr := s.TokenAccountService.Consume(authBeautyCenterID, charge, reservation.ID,
		fmt.Sprintf("Reservation #%d completed — 3.5%% of %.2f", reservation.ID, reservation.TotalPrice))
	if consumeErr != nil {
		utils.Log.Errorf("[TOKEN GRACE] Failed to deduct tokens for reservation %d: %v", reservation.ID, consumeErr)
		// Reservation is already completed — grace tx was logged inside Consume
	}

	// ── Loyalty stamps ──
	for _, svc := range reservation.ReservationServices {
		err = s.LoyaltyService.AddPunch(reservation.ID, reservation.ClientID, reservation.BeautyCenterID, svc.ServiceID)
		if err != nil {
			utils.Log.Errorf("[Loyalty Error] failed to add punch for service %d via QR for Client %d; %v", svc.ServiceID, reservation.ClientID, err)
		}
	}

	// ── Trigger missions ──
	if s.MissionService != nil {
		if err := s.MissionService.TriggerCheck(authBeautyCenterID, "reservation_completed"); err != nil {
			utils.Log.Errorf("[Mission Error] failed to trigger mission check for center %d: %v", authBeautyCenterID, err)
		}
	}

	utils.Log.Infof("[QR VERIFIED] Reservation ID %d marked as COMPLETED. Charged %.2f YOTTUs. Added stamps for %d services",
		reservation.ID, charge, len(reservation.ReservationServices))
	return nil
}