package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
)

type WaitlistServiceImpl struct {
	WaitlistRepository    repository.WaitlistRepository
	ReservationRepository repository.ReservationRepository
	ReservationService    ReservationService
	ServiceRepository     repository.ServiceRepository
	ClientRepository      repository.ClientRepository
	NotificationService   NotificationService
	Validate              *validator.Validate
}

func NewWaitlistServiceImpl(
	waitlistRepo repository.WaitlistRepository,
	reservationRepo repository.ReservationRepository,
	reservationSvc ReservationService,
	serviceRepo repository.ServiceRepository,
	clientRepo repository.ClientRepository,
	notificationSvc NotificationService,
	validate *validator.Validate,
) WaitlistService {
	return &WaitlistServiceImpl{
		WaitlistRepository:    waitlistRepo,
		ReservationRepository: reservationRepo,
		ReservationService:    reservationSvc,
		ServiceRepository:     serviceRepo,
		ClientRepository:      clientRepo,
		NotificationService:   notificationSvc,
		Validate:              validate,
	}
}

// encodeServiceIDs serializes service IDs for the CSV column.
func encodeServiceIDs(ids []uint) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatUint(uint64(id), 10))
	}
	return strings.Join(parts, ",")
}

// decodeServiceIDs parses the CSV column back. Malformed entries abort the
// claim with BadRequest instead of booking a wrong bundle.
func decodeServiceIDs(csv string) ([]uint, error) {
	if csv == "" {
		return nil, apperrors.ErrBadRequest
	}
	var out []uint
	for _, p := range strings.Split(csv, ",") {
		n, err := strconv.ParseUint(strings.TrimSpace(p), 10, 64)
		if err != nil || n == 0 {
			return nil, apperrors.ErrBadRequest
		}
		out = append(out, uint(n))
	}
	return out, nil
}

// clientHoldsSlot reports whether the client already owns a live reservation
// for the exact slot — joining the line for your own turn is rejected.
func (s *WaitlistServiceImpl) clientHoldsSlot(clientID, centerID uint, date time.Time, start string) (bool, error) {
	rows, err := s.ReservationRepository.FindByClientID(clientID)
	if err != nil {
		return false, err
	}
	for _, r := range rows {
		if r.BeautyCenterID == centerID && r.Date.Format("2006-01-02") == date.Format("2006-01-02") &&
			r.StartTime == start && (r.Status == "PENDING" || r.Status == "CONFIRMED") {
			return true, nil
		}
	}
	return false, nil
}

// Join implements WaitlistService.
func (s *WaitlistServiceImpl) Join(req request.JoinWaitlistRequest, clientID uint) (*models.WaitlistEntry, error) {
	if err := s.Validate.Struct(req); err != nil {
		return nil, apperrors.ErrValidation
	}
	parsedDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, apperrors.ErrBadRequest
	}
	if _, err := time.Parse("15:04", req.StartTime); err != nil {
		return nil, apperrors.ErrBadRequest
	}
	slotStart, err := time.Parse("2006-01-02 15:04", req.Date+" "+req.StartTime)
	if err != nil || !slotStart.After(time.Now()) {
		return nil, apperrors.ErrBadRequest
	}
	client, err := s.ClientRepository.FindById(clientID)
	if err != nil || client == nil || !client.IsActive {
		return nil, apperrors.ErrBadRequest
	}
	dup, err := s.WaitlistRepository.ExistsLive(req.BeautyCenterID, clientID, parsedDate, req.StartTime)
	if err != nil {
		return nil, err
	}
	if dup {
		return nil, apperrors.ErrConflict
	}
	holds, err := s.clientHoldsSlot(clientID, req.BeautyCenterID, parsedDate, req.StartTime)
	if err != nil {
		return nil, err
	}
	if holds {
		return nil, apperrors.ErrConflict
	}
	// Compute the end time from the service bundle so the claim rebooks the
	// exact window (validates the services belong to the center, too).
	services, err := s.ServiceRepository.FindByIDs(req.ServiceIDs)
	if err != nil || len(services) != len(req.ServiceIDs) {
		return nil, apperrors.ErrBadRequest
	}
	totalDuration := 0
	for _, svc := range services {
		if svc.BeautyCenterID != req.BeautyCenterID {
			return nil, apperrors.ErrBadRequest
		}
		totalDuration += svc.Duration
	}
	startParsed, _ := time.Parse("15:04", req.StartTime)
	endTime := startParsed.Add(time.Duration(totalDuration) * time.Minute).Format("15:04")

	entry := &models.WaitlistEntry{
		BeautyCenterID: req.BeautyCenterID,
		ClientID:       clientID,
		Date:           parsedDate,
		StartTime:      req.StartTime,
		EndTime:        endTime,
		PersonnelID:    req.PersonnelID,
		ServiceIDs:     encodeServiceIDs(req.ServiceIDs),
		Status:         models.WaitlistWaiting,
	}
	if err := s.WaitlistRepository.Create(entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// Leave implements WaitlistService.
func (s *WaitlistServiceImpl) Leave(entryID, clientID uint) error {
	entry, err := s.WaitlistRepository.FindByID(entryID)
	if err != nil || entry == nil {
		return apperrors.ErrNotFound
	}
	if entry.ClientID != clientID {
		return apperrors.ErrForbidden
	}
	if entry.Status != models.WaitlistWaiting && entry.Status != models.WaitlistOffered {
		return apperrors.ErrConflict
	}
	entry.Status = models.WaitlistCancelled
	return s.WaitlistRepository.Update(entry)
}

// ListMine implements WaitlistService.
func (s *WaitlistServiceImpl) ListMine(clientID uint) ([]models.WaitlistEntry, error) {
	return s.WaitlistRepository.FindLive(clientID)
}

// offerRow moves one WAITING row to OFFERED with a fresh claim window and
// notifies the client. In-app is the record, push the nudge (best-effort).
func (s *WaitlistServiceImpl) offerRow(entry *models.WaitlistEntry, now time.Time) error {
	expires := now.Add(models.WaitlistOfferMinutes * time.Minute)
	entry.Status = models.WaitlistOffered
	entry.OfferedAt = &now
	entry.OfferExpiresAt = &expires
	if err := s.WaitlistRepository.Update(entry); err != nil {
		return err
	}
	dataStr := fmt.Sprintf(`{"waitlist_id":%d,"reservation_date":%q}`, entry.ID, entry.Date.Format("2006-01-02"))
	_ = s.NotificationService.Create(entry.ClientID, "waitlist_offer",
		"Se liberó un turno",
		fmt.Sprintf("Tenés %d minutos para reclamar el turno del %s a las %s",
			models.WaitlistOfferMinutes, entry.Date.Format("02/01"), entry.StartTime),
		dataStr)
	pushData := fmt.Sprintf(`{"type":"waitlist_offer","data":%q}`, dataStr)
	_ = s.NotificationService.SendPushToClient(entry.ClientID, "Se liberó un turno",
		fmt.Sprintf("Turno del %s a las %s disponible", entry.Date.Format("02/01"), entry.StartTime), pushData)
	return nil
}

// OfferForFreedSlot implements WaitlistService.
func (s *WaitlistServiceImpl) OfferForFreedSlot(centerID uint, date time.Time, startTime string) (*models.WaitlistEntry, error) {
	waiters, err := s.WaitlistRepository.FindWaiting(centerID, date, startTime)
	if err != nil {
		return nil, err
	}
	if len(waiters) == 0 {
		return nil, nil
	}
	first := waiters[0]
	if err := s.offerRow(&first, time.Now()); err != nil {
		return nil, err
	}
	return &first, nil
}

// Claim implements WaitlistService.
func (s *WaitlistServiceImpl) Claim(entryID, clientID uint) (*models.Reservation, error) {
	entry, err := s.WaitlistRepository.FindByID(entryID)
	if err != nil || entry == nil {
		return nil, apperrors.ErrNotFound
	}
	if entry.ClientID != clientID {
		return nil, apperrors.ErrForbidden
	}
	if entry.Status != models.WaitlistOffered {
		return nil, apperrors.ErrConflict
	}
	now := time.Now()
	if entry.OfferExpiresAt == nil || !now.Before(*entry.OfferExpiresAt) {
		// Window lapsed: expire this offer and pass it on, then report.
		entry.Status = models.WaitlistExpired
		_ = s.WaitlistRepository.Update(entry)
		_, _ = s.OfferForFreedSlot(entry.BeautyCenterID, entry.Date, entry.StartTime)
		return nil, apperrors.ErrConflict
	}
	serviceIDs, err := decodeServiceIDs(entry.ServiceIDs)
	if err != nil {
		return nil, err
	}
	cerr := s.ReservationService.Create(request.CreateReservationRequest{
		BeautyCenterID: entry.BeautyCenterID,
		ClientID:       entry.ClientID,
		PersonnelID:    entry.PersonnelID,
		ServiceIDs:     serviceIDs,
		Date:           entry.Date.Format("2006-01-02"),
		StartTime:      entry.StartTime,
	})
	if cerr != nil {
		if cerr == apperrors.ErrConflict {
			// Slot was taken meanwhile: expire and pass the offer on.
			entry.Status = models.WaitlistExpired
			_ = s.WaitlistRepository.Update(entry)
			_, _ = s.OfferForFreedSlot(entry.BeautyCenterID, entry.Date, entry.StartTime)
		}
		return nil, cerr
	}
	entry.Status = models.WaitlistBooked
	if err := s.WaitlistRepository.Update(entry); err != nil {
		utils.Log.Errorf("[WAITLIST] Booked but failed to mark entry %d: %v", entry.ID, err)
	}
	// Return the fresh booking for immediate UI use.
	rows, err := s.ReservationRepository.FindByClientID(clientID)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		r := rows[i]
		if r.BeautyCenterID == entry.BeautyCenterID &&
			r.Date.Format("2006-01-02") == entry.Date.Format("2006-01-02") &&
			r.StartTime == entry.StartTime &&
			(r.Status == "PENDING" || r.Status == "CONFIRMED") {
			return &rows[i], nil
		}
	}
	return nil, apperrors.ErrNotFound
}

// SweepExpiredOffers implements WaitlistService.
func (s *WaitlistServiceImpl) SweepExpiredOffers(now time.Time) (int, error) {
	stale, err := s.WaitlistRepository.FindOfferedExpired(now)
	if err != nil {
		return 0, err
	}
	offered := 0
	for i := range stale {
		e := stale[i]
		e.Status = models.WaitlistExpired
		if err := s.WaitlistRepository.Update(&e); err != nil {
			utils.Log.Errorf("[WAITLIST] Failed to expire offer %d: %v", e.ID, err)
			continue
		}
		if next, err := s.OfferForFreedSlot(e.BeautyCenterID, e.Date, e.StartTime); err != nil {
			utils.Log.Errorf("[WAITLIST] Failed to pass on slot after expiry %d: %v", e.ID, err)
		} else if next != nil {
			offered++
		}
	}
	return offered, nil
}
