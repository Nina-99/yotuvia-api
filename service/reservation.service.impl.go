package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
)

type ReservationServiceImpl struct {
	PersonnelRepository   repository.PersonnelRepository
	ReservationRepository repository.ReservationRepository
	ServiceRepository     repository.ServiceRepository
	NotificationService   NotificationService
	LoyaltyService        LoyaltyService
	TokenAccountService   TokenAccountService
	MissionService        TokenMissionService
	Validate              *validator.Validate
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

// ownsReservation enforces per-role ownership on the /:id family: a client may
// access only its own reservations (clientID claim == ClientID), while a center
// (or its staff) only its own center's (centerID claim == BeautyCenterID). The
// "owner client OR owner center" matrix rule is applied across roles, not
// within one actor — a client token can never ride a centerID claim.
func ownsReservation(r *models.Reservation, clientID, centerID uint, role string) bool {
	switch role {
	case "client":
		return clientID == r.ClientID
	case "beauty_center", "personnel":
		return centerID != 0 && centerID == r.BeautyCenterID
	default:
		return clientID == r.ClientID || (centerID != 0 && centerID == r.BeautyCenterID)
	}
}

// normalizeStatus canonicalizes status strings to UPPERCASE so legacy lowercase
// rows (e.g. "completed") compare and persist correctly.
func normalizeStatus(status string) string {
	return strings.ToUpper(status)
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

func (s *ReservationServiceImpl) Update(reservationID uint, req request.CreateReservationRequest, clientID, centerID uint, role string) error {
	// Ownership at entry: fetch the resource, then prove the caller may touch it.
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return apperrors.ErrNotFound
	}
	if !ownsReservation(reservation, clientID, centerID, role) {
		return apperrors.ErrForbidden
	}

	if err := s.Validate.Struct(req); err != nil {
		return apperrors.ErrValidation
	}

	// Parse date string into time.Time
	parsedDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return apperrors.ErrBadRequest
	}

	// Fetch services and recalculate
	services, err := s.ServiceRepository.FindByIDs(req.ServiceIDs)
	if err != nil || len(services) != len(req.ServiceIDs) {
		return apperrors.ErrBadRequest
	}

	var totalPrice float64
	var totalDuration int
	for _, svc := range services {
		if svc.BeautyCenterID != req.BeautyCenterID {
			return apperrors.ErrBadRequest
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
			return apperrors.ErrConflict
		}
	}

	// ClientID is never rewritten from the request: ownership was already proven
	// against the actor's claim, and the body may carry an arbitrary client_id
	// that must not reassign the reservation to someone else.
	reservation.BeautyCenterID = req.BeautyCenterID
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

func (s *ReservationServiceImpl) AssignPersonnel(reservationID uint, personnelID uint, centerID uint) error {
	// Find the reservation
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return apperrors.ErrNotFound
	}

	// Admin-only: the acting center must own the reservation.
	if centerID != reservation.BeautyCenterID {
		return apperrors.ErrForbidden
	}

	// Cannot reassign completed or cancelled
	if reservation.Status == "COMPLETED" || reservation.Status == "CANCELLED" {
		return apperrors.ErrConflict
	}

	// Verify personnel exists and belongs to the same beauty center
	personnel, err := s.PersonnelRepository.FindByID(personnelID)
	if err != nil || personnel == nil {
		return apperrors.ErrNotFound
	}
	if personnel.BeautyCenterID != reservation.BeautyCenterID {
		return apperrors.ErrBadRequest
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
		return apperrors.ErrConflict
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
		// Push mirrors the REST {type, data} shape so the mobile mapper can
		// resolve the payload on cold start (design D3). Row data stays clean.
		pushData := fmt.Sprintf(`{"type":"personnel_changed","data":%q}`, dataStr)
		_ = s.NotificationService.SendPushToClient(reservation.ClientID, title, message, pushData)
	}

	// Personnel-scoped notifications (design D5): in-app only, no push.
	// The losing staff member is notified only when a staff was actually
	// assigned before; the gaining staff member always is (personnelID is
	// validated above and belongs to the right center).
	if reservation.PersonnelID != 0 && reservation.PersonnelID != personnelID {
		_ = s.NotifyPersonnelReassigned(reservation, personnelID)
	}
	_ = s.NotifyPersonnelAssigned(reservation, personnelID)

	return nil
}

// ──────────────────────────────────────────────
//  UPDATE STATUS
// ──────────────────────────────────────────────

func (s *ReservationServiceImpl) UpdateStatus(reservationID uint, status string, centerID uint) error {
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return apperrors.ErrNotFound
	}

	// Admin-only: the acting center must own the reservation.
	if centerID != reservation.BeautyCenterID {
		return apperrors.ErrForbidden
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
		return apperrors.ErrConflict
	}
	if !allowed[status] {
		return apperrors.ErrConflict
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
		return apperrors.ErrNotFound
	}

	if reservation.ClientID != clientID {
		return apperrors.ErrForbidden
	}

	if reservation.Status != "PENDING" && reservation.Status != "CONFIRMED" {
		return apperrors.ErrConflict
	}

	reservation.Status = "CANCELLED"
	if reason != "" {
		reservation.CancelReason = &reason
	}

	if err := s.ReservationRepository.Update(reservation); err != nil {
		return err
	}

	// Personnel-scoped notification (design D5): in-app only, no push. The
	// assigned staff member is told the reservation was cancelled. Guarded
	// against reservations created without personnel (PersonnelID = 0).
	if reservation.PersonnelID != 0 {
		_ = s.NotifyPersonnelCancelled(reservation, reason)
	}

	return nil
}

// ──────────────────────────────────────────────
//  PERSONNEL-SCOPED NOTIFICATION HELPERS (design D5)
// ──────────────────────────────────────────────

// NotifyPersonnelAssigned tells the gaining staff member about a new
// reservation (or reassignment). Data carries the stable key pair used by the
// mobile mapper: reservation_id + personnel_id.
func (s *ReservationServiceImpl) NotifyPersonnelAssigned(reservation *models.Reservation, personnelID uint) error {
	dataStr := fmt.Sprintf(`{"reservation_id":%d,"personnel_id":%d}`, reservation.ID, personnelID)
	title := "Nuevo turno asignado"
	message := fmt.Sprintf("Tenés un turno el %s a las %s",
		reservation.Date.Format("02/01"),
		reservation.StartTime,
	)
	return s.NotificationService.CreateForPersonnel(personnelID, "personnel_changed", title, message, dataStr)
}

// NotifyPersonnelReassigned tells the losing staff member their reservation
// was reassigned to another professional.
func (s *ReservationServiceImpl) NotifyPersonnelReassigned(reservation *models.Reservation, personnelID uint) error {
	dataStr := fmt.Sprintf(`{"reservation_id":%d,"personnel_id":%d}`, reservation.ID, personnelID)
	title := "Turno reasignado"
	message := fmt.Sprintf("El turno del %s a las %s fue reasignado a otro profesional",
		reservation.Date.Format("02/01"),
		reservation.StartTime,
	)
	return s.NotificationService.CreateForPersonnel(reservation.PersonnelID, "personnel_changed", title, message, dataStr)
}

// NotifyPersonnelCancelled tells the assigned staff member their reservation
// was cancelled by the client.
func (s *ReservationServiceImpl) NotifyPersonnelCancelled(reservation *models.Reservation, reason string) error {
	dataStr := fmt.Sprintf(`{"reservation_id":%d}`, reservation.ID)
	title := "Turno cancelado"
	message := fmt.Sprintf("El turno del %s a las %s fue cancelado",
		reservation.Date.Format("02/01"),
		reservation.StartTime,
	)
	if reason != "" {
		message += fmt.Sprintf(" — Motivo: %s", reason)
	}
	return s.NotificationService.CreateForPersonnel(reservation.PersonnelID, "reservation_cancelled", title, message, dataStr)
}

// ──────────────────────────────────────────────
//  DELETE
// ──────────────────────────────────────────────

func (s *ReservationServiceImpl) Delete(reservationID uint, clientID, centerID uint, role string) error {
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return apperrors.ErrNotFound
	}
	if !ownsReservation(reservation, clientID, centerID, role) {
		return apperrors.ErrForbidden
	}
	return s.ReservationRepository.Delete(reservation)
}

// ──────────────────────────────────────────────
//  FIND
// ──────────────────────────────────────────────

func (s *ReservationServiceImpl) FindAll(page, limit int, centerID uint) ([]models.Reservation, int64, error) {
	return s.ReservationRepository.FindAll(page, limit, centerID)
}

func (s *ReservationServiceImpl) FindById(reservationID uint, clientID, centerID uint, role string) (*models.Reservation, error) {
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return nil, apperrors.ErrNotFound
	}
	if !ownsReservation(reservation, clientID, centerID, role) {
		return nil, apperrors.ErrForbidden
	}
	return reservation, nil
}

func (s *ReservationServiceImpl) FindByClientID(clientID, actorClientID uint) ([]models.Reservation, error) {
	if actorClientID != clientID {
		return nil, apperrors.ErrForbidden
	}
	return s.ReservationRepository.FindByClientID(clientID)
}

func (s *ReservationServiceImpl) FindUpcomingByClientID(clientID, actorClientID uint) (*models.Reservation, error) {
	if actorClientID != clientID {
		return nil, apperrors.ErrForbidden
	}
	reservation, err := s.ReservationRepository.FindUpcomingByClientID(clientID)
	if err != nil {
		// No upcoming reservation (or a lookup failure) is surfaced as NotFound
		// to keep the endpoint's 404 contract stable.
		return nil, apperrors.ErrNotFound
	}
	return reservation, nil
}

func (s *ReservationServiceImpl) FindByBeautyCenterID(beautyCenterID, actorCenterID uint) ([]models.Reservation, error) {
	if actorCenterID != beautyCenterID {
		return nil, apperrors.ErrForbidden
	}
	return s.ReservationRepository.FindByBeautyCenterId(beautyCenterID)
}

// FindByPersonnelID returns the caller's own agenda. The personnelID arrives as
// the caller's claim from the controller (never from the request path), so no
// cross-staff surface exists; from/to bound the optional date range.
func (s *ReservationServiceImpl) FindByPersonnelID(personnelID uint, from, to *time.Time) ([]models.Reservation, error) {
	return s.ReservationRepository.FindByPersonnelId(personnelID, from, to)
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

func (s *ReservationServiceImpl) GenerateVerificationToken(reservationID uint, clientID, centerID uint, role string) (string, error) {
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return "", apperrors.ErrNotFound
	}

	// Owner client or owner center only.
	if !ownsReservation(reservation, clientID, centerID, role) {
		return "", apperrors.ErrForbidden
	}

	// Terminal states must never issue a token. normalizeStatus covers legacy
	// lowercase rows written by the old code.
	if status := normalizeStatus(reservation.Status); status == "COMPLETED" || status == "CANCELLED" {
		return "", apperrors.ErrConflict
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

func (s *ReservationServiceImpl) VerifyQR(token string, authBeautyCenterID uint) (*models.Reservation, bool, error) {
	reservation, err := s.ReservationRepository.FindByToken(token)
	if err != nil || reservation == nil {
		return nil, false, apperrors.ErrNotFound
	}

	if reservation.BeautyCenterID != authBeautyCenterID {
		utils.Log.Warnf("[SECURITY ALERT] Intrusion attempt! Salon ID %d tried to verify QR for Reservation ID %d (owned by Salon %d)", authBeautyCenterID, reservation.ID, reservation.BeautyCenterID)
		return nil, false, apperrors.ErrForbidden
	}

	if reservation.TokenExpiresAt != nil && time.Now().After(*reservation.TokenExpiresAt) {
		return nil, false, apperrors.ErrBadRequest
	}

	// Idempotency: a COMPLETED reservation (including legacy lowercase rows) is
	// a no-op success — never a second charge, punch, or mission trigger. A
	// CANCELLED reservation is terminal and must not complete.
	switch normalizeStatus(reservation.Status) {
	case "COMPLETED":
		return reservation, true, nil
	case "CANCELLED":
		return nil, false, apperrors.ErrConflict
	}

	// ── Token consumption check ──
	charge := reservation.TotalPrice * 0.035
	canConsume, err := s.TokenAccountService.CanConsume(authBeautyCenterID, utils.ToMilli(charge))
	if err != nil {
		return nil, false, err
	}
	if !canConsume {
		return nil, false, apperrors.ErrInsufficientBalance
	}

	// ── Exactly-once persistence ──
	// The conditional update only completes reservations still in a non-terminal
	// state; a concurrent scan that already completed this one hits 0 rows and
	// becomes a no-op success. Status is persisted UPPERCASE.
	reservation.Status = "COMPLETED"
	reservation.VerificationToken = nil
	reservation.TokenExpiresAt = nil

	completed, err := s.ReservationRepository.CompleteVerification(reservation)
	if err != nil {
		return nil, false, err
	}
	if !completed {
		utils.Log.Infof("[QR] Reservation ID %d already completed by another scan — no-op", reservation.ID)
		return reservation, true, nil
	}

	// ── Deduct YOTTUs ──
	// The charge is converted to milli and deducted atomically (balance + ledger
	// row in one transaction). A failure propagates to the caller — there is no
	// grace mechanism and no partial state.
	consumeErr := s.TokenAccountService.Consume(authBeautyCenterID, utils.ToMilli(charge), reservation.ID,
		fmt.Sprintf("Reservation #%d completed — 3.5%% of %.2f", reservation.ID, reservation.TotalPrice))
	if consumeErr != nil {
		utils.Log.Errorf("[TOKEN] Failed to deduct tokens for reservation %d: %v", reservation.ID, consumeErr)
		return nil, false, consumeErr
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
	return reservation, false, nil
}
