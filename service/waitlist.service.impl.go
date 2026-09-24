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

// WaitlistServiceImpl dependency contract.
//
// WaitlistRepository is REQUIRED by every method and is enforced by
// requireDeps in the constructor.
//
// The others are needed only by SPECIFIC methods, so a nil is a legitimate
// configuration and must never be enforced at construction:
//
//	Validate              — Join
//	ReservationRepository — clientHoldsSlot (Join), Claim
//	ReservationService    — Claim
//	ServiceRepository     — Join
//	ClientRepository      — Join
//	NotificationService   — notifyOffer (OfferForFreedSlot, SweepExpiredOffers)
//
// The Phase 3 ticker calls only SweepExpiredOffers, which needs
// WaitlistRepository + NotificationService; scheduler.go wires both.
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
	requireDeps("NewWaitlistServiceImpl",
		dep("WaitlistRepository", waitlistRepo),
	)
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

// ListMine implements WaitlistService. Live listing runs read-time expiry
// first so a lapsed offer never reaches the client as claimable; both modes
// then attach the queue Position.
func (s *WaitlistServiceImpl) ListMine(clientID uint, history bool) ([]models.WaitlistEntry, error) {
	if history {
		entries, err := s.WaitlistRepository.FindAllByClient(clientID)
		if err != nil {
			return nil, err
		}
		s.attachPositions(entries)
		return entries, nil
	}
	entries, err := s.WaitlistRepository.FindLive(clientID)
	if err != nil {
		return nil, err
	}
	entries = s.expireLapsedOffers(entries)
	s.attachPositions(entries)
	return entries, nil
}

// expireLapsedOffers implements read-time expiry (A3). It walks only the
// caller's live page and touches just the OFFERED rows whose window already
// lapsed: each is flipped to EXPIRED and its slot is passed to the next
// waiter. Every failure is logged and swallowed so a stale-offer problem can
// never break the listing. Expired rows are dropped from the returned page.
func (s *WaitlistServiceImpl) expireLapsedOffers(entries []models.WaitlistEntry) []models.WaitlistEntry {
	now := time.Now()
	out := make([]models.WaitlistEntry, 0, len(entries))
	for i := range entries {
		e := entries[i]
		if e.Status == models.WaitlistOffered && e.OfferExpiresAt != nil && e.OfferExpiresAt.Before(now) {
			e.Status = models.WaitlistExpired
			if err := s.WaitlistRepository.Update(&e); err != nil {
				utils.Log.Errorf("[WAITLIST] read-time expiry failed for entry %d: %v", e.ID, err)
			}
			if _, err := s.OfferForFreedSlot(e.BeautyCenterID, e.Date, e.StartTime); err != nil {
				utils.Log.Errorf("[WAITLIST] read-time cascade failed after expiring entry %d: %v", e.ID, err)
			}
			continue // no longer live: never ship it as claimable
		}
		out = append(out, e)
	}
	return out
}

// attachPositions fills Position for the page: a 1-based FIFO rank among the
// live entries of the same slot, ordered by created_at ASC (OFFERED holds 1).
// Closed entries keep 0. The rank comes from one batched query over the
// distinct slots of the page — no COUNT per row. A lookup failure logs and
// leaves positions at 0 rather than failing the listing.
func (s *WaitlistServiceImpl) attachPositions(entries []models.WaitlistEntry) {
	if len(entries) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(entries))
	slots := make([]repository.WaitlistSlot, 0, len(entries))
	for i := range entries {
		if entries[i].Status != models.WaitlistWaiting && entries[i].Status != models.WaitlistOffered {
			continue
		}
		key := waitlistSlotKey(entries[i].BeautyCenterID, entries[i].Date, entries[i].StartTime)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		slots = append(slots, repository.WaitlistSlot{
			BeautyCenterID: entries[i].BeautyCenterID,
			Date:           entries[i].Date,
			StartTime:      entries[i].StartTime,
		})
	}
	if len(slots) == 0 {
		return
	}
	live, err := s.WaitlistRepository.FindLiveForSlots(slots)
	if err != nil {
		utils.Log.Errorf("[WAITLIST] position lookup failed: %v", err)
		return
	}
	counts := make(map[string]int, len(slots))
	positions := make(map[uint]int, len(live))
	for i := range live {
		key := waitlistSlotKey(live[i].BeautyCenterID, live[i].Date, live[i].StartTime)
		counts[key]++
		p := counts[key]
		if live[i].Status == models.WaitlistOffered {
			p = 1 // the offer holder is always at the front
		}
		positions[live[i].ID] = p
	}
	for i := range entries {
		if p, ok := positions[entries[i].ID]; ok {
			entries[i].Position = p
		}
	}
}

// waitlistSlotKey identifies a queue slot for in-memory grouping.
func waitlistSlotKey(centerID uint, date time.Time, startTime string) string {
	return fmt.Sprintf("%d|%s|%s", centerID, date.Format("2006-01-02"), startTime)
}

// notifyOffer tells the client a slot is theirs: an in-app record plus a push
// nudge (best-effort). Both carry the SAME flat JSON payload so the app can
// deep-link from either surface:
// {"type":"waitlist_offer","waitlist_id":N,"reservation_date":"YYYY-MM-DD","start_time":"HH:MM"}.
func (s *WaitlistServiceImpl) notifyOffer(entry *models.WaitlistEntry) {
	payload := fmt.Sprintf(`{"type":"waitlist_offer","waitlist_id":%d,"reservation_date":%q,"start_time":%q}`,
		entry.ID, entry.Date.Format("2006-01-02"), entry.StartTime)
	_ = s.NotificationService.Create(entry.ClientID, "waitlist_offer",
		"Se liberó un turno",
		fmt.Sprintf("Tenés %d minutos para reclamar el turno del %s a las %s",
			models.WaitlistOfferMinutes, entry.Date.Format("02/01"), entry.StartTime),
		payload)
	_ = s.NotificationService.SendPushToClient(entry.ClientID, "Se liberó un turno",
		fmt.Sprintf("Turno del %s a las %s disponible", entry.Date.Format("02/01"), entry.StartTime), payload)
}

// OfferForFreedSlot implements WaitlistService. The select-and-mark is atomic
// (repository row lock), so concurrent cancellations cannot double-offer; the
// notification stays best-effort after the row is committed.
func (s *WaitlistServiceImpl) OfferForFreedSlot(centerID uint, date time.Time, startTime string) (*models.WaitlistEntry, error) {
	now := time.Now()
	expires := now.Add(models.WaitlistOfferMinutes * time.Minute)
	entry, err := s.WaitlistRepository.OfferNextWaiting(centerID, date, startTime, now, expires)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}
	s.notifyOffer(entry)
	return entry, nil
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
