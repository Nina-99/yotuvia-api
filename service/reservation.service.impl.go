package service

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

// ReservationServiceImpl dependency contract.
//
// ReservationRepository is REQUIRED by every method and is enforced by
// requireDeps in the constructor.
//
// The remaining dependencies are needed only by SPECIFIC methods, so a nil is
// a legitimate configuration (configs/scheduler.go wires several as nil) and
// must never be enforced at construction:
//
//	Validate             — Create, CreateAdminReservation, Update
//	ServiceRepository    — Create/CreateAdminReservation (createReservationCore), Update
//	PersonnelRepository  — CreateAdminReservation, AssignPersonnel
//	ClientRepository     — Create (trust), CreateAdminReservation, Cancel (late-cancel),
//	                       RecordNoShow, RequiresDeposit, VerifyQR (attendance reward)
//	NotificationService  — CreateAdminReservation, AssignPersonnel, UploadProof,
//	                       ReviewProof, SendDueReminders, NotifyPersonnel* helpers
//	CenterRepository     — Create (working-hours validation), Cancel + applyDepositDecision
//	                       (both nil-guarded)
//	TokenAccountRepo     — VerifyQR (unguarded)
//	TokenAccountService  — VerifyQR (unguarded)
//	LoyaltyService       — Create (nil-guarded), VerifyQR (unguarded)
//	PromotionService     — Create (nil-guarded)
//	MissionService       — VerifyQR (nil-guarded)
//	MarketConfigService  — VerifyQR (nil-guarded)
//	FeedPostRepository   — Create attribution only (nil-guarded): when nil, a
//	                       source_post_id hint is simply ignored.
//	PersonnelServiceRepo — Create (capability check, nil-guarded): when nil the
//	                       capability rule is skipped, which is correct for the
//	                       sweeps wiring where Create is never reachable.
//
// The Phase 3 ticker calls only SendDueReminders, SweepNoShows and
// ExpireHolds, which need ReservationRepository + NotificationService +
// ClientRepository — all of which scheduler.go wires. The nil deps above are
// never reachable from those three methods.
type ReservationServiceImpl struct {
	PersonnelRepository   repository.PersonnelRepository
	ReservationRepository repository.ReservationRepository
	ServiceRepository     repository.ServiceRepository
	ClientRepository      repository.ClientRepository
	CenterRepository      repository.BeautyCenterRepository
	TokenAccountRepo      repository.TokenAccountRepository
	NotificationService   NotificationService
	LoyaltyService        LoyaltyService
	PromotionService      PromotionService
	TokenAccountService   TokenAccountService
	MissionService        TokenMissionService
	MarketConfigService   MarketConfigService
	// FeedPostRepository resolves an optional source_post_id hint from the
	// client booking flow. It is OPTIONAL: nil simply disables attribution.
	FeedPostRepository repository.FeedPostRepository
	// PersonnelServiceRepository backs the capability check on the client
	// booking path (a professional with capability rows may only be booked for
	// those services). OPTIONAL: nil disables the check, which is the correct
	// configuration for the sweeps wiring, where Create is never reachable.
	PersonnelServiceRepository repository.PersonnelServiceRepository
	Validate                   *validator.Validate
}

func NewReservationServiceImpl(
	reservationRepository repository.ReservationRepository,
	serviceRepository repository.ServiceRepository,
	personnelRepository repository.PersonnelRepository,
	clientRepository repository.ClientRepository,
	tokenAccountRepository repository.TokenAccountRepository,
	notificationService NotificationService,
	loyaltyService LoyaltyService,
	promotionService PromotionService,
	tokenAccountService TokenAccountService,
	missionService TokenMissionService,
	marketConfigService MarketConfigService,
	validate *validator.Validate,
	centerRepository repository.BeautyCenterRepository,
	feedPostRepository repository.FeedPostRepository,
	personnelServiceRepository repository.PersonnelServiceRepository,
) ReservationService {
	requireDeps("NewReservationServiceImpl",
		dep("ReservationRepository", reservationRepository),
	)
	return &ReservationServiceImpl{
		ReservationRepository:      reservationRepository,
		PersonnelRepository:        personnelRepository,
		ServiceRepository:          serviceRepository,
		ClientRepository:           clientRepository,
		CenterRepository:           centerRepository,
		TokenAccountRepo:           tokenAccountRepository,
		NotificationService:        notificationService,
		LoyaltyService:             loyaltyService,
		PromotionService:           promotionService,
		TokenAccountService:        tokenAccountService,
		MissionService:             missionService,
		MarketConfigService:        marketConfigService,
		FeedPostRepository:         feedPostRepository,
		PersonnelServiceRepository: personnelServiceRepository,
		Validate:                   validate,
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

// resolveSourcePostID turns an optional client attribution hint into a value
// that is safe to persist. A tracking field must NEVER fail a real booking, so
// every non-resolving case collapses to nil and the booking proceeds:
//
//   - absent (nil) or zero id → nil, no query;
//   - the feed repository is not wired → nil, no query;
//   - the id does not match an existing post → nil;
//   - the existence probe itself errors → nil (the booking wins).
func (s *ReservationServiceImpl) resolveSourcePostID(sourcePostID *uint) *uint {
	if sourcePostID == nil || *sourcePostID == 0 {
		return nil
	}
	if s.FeedPostRepository == nil {
		return nil
	}
	exists, err := s.FeedPostRepository.Exists(*sourcePostID)
	if err != nil {
		utils.Log.Errorf("[ATTRIBUTION] post %d lookup failed, booking proceeds un-attributed: %v", *sourcePostID, err)
		return nil
	}
	if !exists {
		utils.Log.Warnf("[ATTRIBUTION] source_post_id %d does not resolve, booking proceeds un-attributed", *sourcePostID)
		return nil
	}
	id := *sourcePostID
	return &id
}

// createReservationCore is the single DRY creation path shared by the client
// flow (Create) and the admin walk-in flow (CreateAdminReservation, design
// D1). It is parameterized by fields — never by a request DTO — so both flows
// keep identical semantics for status tracking (PENDING), end_time/total_price
// computation, the overlap check, and the transactional insert. It returns the
// created reservation ID so callers can re-fetch the preloaded row for DTO
// responses. Errors are canonical apperrors sentinels so controllers map them
// to the spec contract: 400 for malformed input / unknown or cross-center
// services, 409 for a conflicting slot. sourcePostID is the ALREADY-RESOLVED
// attribution (nil when untracked); walk-ins always pass nil.
func (s *ReservationServiceImpl) createReservationCore(beautyCenterID, clientID, personnelID uint, serviceIDs []uint, dateStr, startTimeStr string, sourcePostID *uint) (uint, error) {
	// Parse date string into time.Time
	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0, apperrors.ErrBadRequest
	}

	// Fetch all services and validate
	services, err := s.ServiceRepository.FindByIDs(serviceIDs)
	if err != nil || len(services) != len(serviceIDs) {
		return 0, apperrors.ErrBadRequest
	}

	// All services must belong to the same beauty center
	var totalPrice float64
	for _, svc := range services {
		if svc.BeautyCenterID != beautyCenterID {
			return 0, apperrors.ErrBadRequest
		}
		totalPrice += svc.Price
	}
	// One shared duration definition (service/availability) so the booking
	// flow and the availability search can never compute different totals.
	totalDuration := TotalServicesDuration(services)

	// Calculate end time from start time + total duration
	startParsed, err := time.Parse("15:04", startTimeStr)
	if err != nil {
		return 0, apperrors.ErrBadRequest
	}
	endTimeStr := startParsed.Add(time.Duration(totalDuration) * time.Minute).Format("15:04")

	// Check for overlaps
	overlap, err := s.ReservationRepository.CheckOverlap(beautyCenterID, parsedDate, startTimeStr, endTimeStr, personnelID, 0)
	if err != nil {
		return 0, err
	}
	if overlap {
		return 0, apperrors.ErrConflict
	}

	// ── Loyalty reward redemption (only if center has loyalty enabled) ──
	// Check if the client has any completed punch cards for the services being
	// booked. Calculate the discount WITHOUT committing — we finalize after
	// reservation creation succeeds.
	type pendingRedemption struct {
		cardID   uint
		discount float64
	}
	var redemptions []pendingRedemption
	var loyaltyDiscount float64

	loyaltyEnabled := false
	if s.LoyaltyService != nil {
		settings, settingsErr := s.LoyaltyService.GetCenterSettings(beautyCenterID)
		if settingsErr == nil {
			loyaltyEnabled = settings.LoyaltyEnabled
		}
	}

	if loyaltyEnabled && s.LoyaltyService != nil {
		for _, svc := range services {
			if s.LoyaltyService.HasPendingRedemption(clientID, svc.ID, beautyCenterID) {
				discount, cardID, calcErr := s.LoyaltyService.CalculateRedemption(clientID, svc.ID, beautyCenterID)
				if calcErr != nil {
					utils.Log.Errorf("[LOYALTY] Failed to calculate redemption for client %d, service %d: %v", clientID, svc.ID, calcErr)
				} else if discount > 0 {
					redemptions = append(redemptions, pendingRedemption{cardID: cardID, discount: discount})
					loyaltyDiscount += discount
					utils.Log.Infof("[LOYALTY] Pending %.2f discount for client %d on service %d (card %d)", discount, clientID, svc.ID, cardID)
				}
			}
		}
	}

	// Ensure discount doesn't exceed total price
	if loyaltyDiscount > totalPrice {
		loyaltyDiscount = totalPrice
	}

	remainingAfterLoyalty := totalPrice - loyaltyDiscount

	// ── Promotion discount ──
	// Check for applicable promotions on each service. Only the BEST promo
	// per service is applied (highest discount).
	type pendingPromo struct {
		promoID  uint
		discount float64
	}
	var promoRedemptions []pendingPromo
	var promotionDiscount float64

	if s.PromotionService != nil {
		now := time.Now()
		hour := now.Hour()
		dayOfWeek := int(now.Weekday())

		for _, svc := range services {
			applicable, findErr := s.PromotionService.FindApplicable(beautyCenterID, svc.ID, hour, dayOfWeek)
			if findErr != nil || len(applicable) == 0 {
				continue
			}

			// Find the best discount among applicable promos
			bestDiscount := 0.0
			bestPromoID := uint(0)
			for _, p := range applicable {
				d, calcErr := s.PromotionService.CalculateDiscount(p.ID, svc.Price, len(services))
				if calcErr != nil {
					continue
				}
				if d > bestDiscount {
					bestDiscount = d
					bestPromoID = p.ID
				}
			}

			if bestDiscount > 0 {
				promoRedemptions = append(promoRedemptions, pendingPromo{promoID: bestPromoID, discount: bestDiscount})
				promotionDiscount += bestDiscount
				utils.Log.Infof("[PROMO] Best promo %d gives %.2f discount on service %d for center %d", bestPromoID, bestDiscount, svc.ID, beautyCenterID)
			}
		}
	}

	// Ensure combined discounts don't exceed remaining price
	if promotionDiscount > remainingAfterLoyalty {
		promotionDiscount = remainingAfterLoyalty
	}

	// Build reservation with services in a transaction
	reservation := models.Reservation{
		BeautyCenterID:    beautyCenterID,
		ClientID:          clientID,
		PersonnelID:       personnelID,
		Date:              parsedDate,
		StartTime:         startTimeStr,
		EndTime:           endTimeStr,
		TotalPrice:        totalPrice,
		LoyaltyDiscount:   loyaltyDiscount,
		PromotionDiscount: promotionDiscount,
		Status:            "PENDING",
		PaymentStatus:     PaymentNone,
		SourcePostID:      sourcePostID,
	}

	for _, svc := range services {
		reservation.ReservationServices = append(reservation.ReservationServices, models.ReservationService{
			ServiceID:   svc.ID,
			ServiceName: svc.Name,
			Price:       svc.Price,
			Duration:    svc.Duration,
			// Phase 1: the line's professional is the reservation's lead by
			// construction.
			PersonnelID: &personnelID,
		})
	}

	if err := s.ReservationRepository.Create(&reservation); err != nil {
		return 0, err
	}

	// ── Finalize loyalty redemptions ──
	// Now that the reservation exists, link the redemptions to it and reset cards.
	if len(redemptions) > 0 && s.LoyaltyService != nil {
		for _, r := range redemptions {
			if finErr := s.LoyaltyService.FinalizeRedemption(r.cardID, reservation.ID); finErr != nil {
				utils.Log.Errorf("[LOYALTY] Failed to finalize redemption for card %d on reservation %d: %v", r.cardID, reservation.ID, finErr)
			}
		}
	}

	// ── Finalize promotion redemptions ──
	if len(promoRedemptions) > 0 && s.PromotionService != nil {
		for _, pr := range promoRedemptions {
			if finErr := s.PromotionService.FinalizeRedemption(pr.promoID, reservation.ID, pr.discount); finErr != nil {
				utils.Log.Errorf("[PROMO] Failed to finalize redemption for promo %d on reservation %d: %v", pr.promoID, reservation.ID, finErr)
			}
		}
	}

	return reservation.ID, nil
}

func (s *ReservationServiceImpl) Create(req request.CreateReservationRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}
	// Client self-booking must obey the same working-hours rules the read path
	// applies when it builds the availability list (see service/availability):
	// the booking window must fit inside a single working block, must not start
	// in the past today, and a weekday with no configured hours is CLOSED. The
	// check lives HERE, in the client entry point, and deliberately NOT in
	// createReservationCore: a center booking its own walk-in
	// (CreateAdminReservation) is responsible for its manual bookings and may
	// legitimately book outside posted hours, so the admin and reassignment
	// paths stay literally untouched. The waitlist claim path calls this Create,
	// which is correct — claiming an offer books a real slot and must satisfy
	// the same client rules.
	//
	// An out-of-hours / closed-day / past request is invalid INPUT, so it maps
	// to 400 (apperrors.ErrBadRequest). ErrConflict (409) stays reserved for an
	// overlap on a slot the availability list actually offered.
	if err := s.validateClientBookingWindow(req.BeautyCenterID, req.Date, req.StartTime, req.ServiceIDs); err != nil {
		return err
	}
	// Capability rule (personnel↔service). Placed HERE, in the client entry
	// point, for the same reason as the working-hours check above: the
	// admin/walk-in path (CreateAdminReservation) is a centre taking
	// responsibility for its own manual bookings and stays exempt. The picker
	// filter (GET /api/personnel?service_ids=) is a client surface, so the
	// read/write agreement that matters is client-side. An unbookable pairing is
	// invalid INPUT, so it maps to 400 (apperrors.ErrBadRequest).
	if err := s.validateClientServiceCapability(req.PersonnelID, req.ServiceIDs); err != nil {
		return err
	}
	id, err := s.createReservationCore(req.BeautyCenterID, req.ClientID, req.PersonnelID, req.ServiceIDs, req.Date, req.StartTime, s.resolveSourcePostID(req.SourcePostID))
	if err != nil {
		return err
	}
	// Deposit decision (client flow only — walk-ins pay in person). Trusted
	// clients and opted-out centers keep PaymentStatus NONE.
	reservation, err := s.ReservationRepository.FindById(id)
	if err != nil || reservation == nil {
		return err
	}
	return s.applyDepositDecision(reservation)
}

// validateClientBookingWindow rejects a client booking whose [start, end)
// window is not fully inside one of the center's working blocks for that
// weekday, or that starts in the past today. A center with no blocks for the
// weekday is CLOSED and cannot receive client bookings — consistent with its
// availability list already coming back empty. The window rules come from the
// shared kernel predicates, so this path and the read path can never drift.
//
// It reads the services only to derive the same end time the creation core will
// persist. That extra SELECT is acceptable on this low-frequency write and is
// what lets the admin path avoid changing at all.
func (s *ReservationServiceImpl) validateClientBookingWindow(centerID uint, dateStr, startTimeStr string, serviceIDs []uint) error {
	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return apperrors.ErrBadRequest
	}
	startParsed, err := time.Parse("15:04", startTimeStr)
	if err != nil {
		return apperrors.ErrBadRequest
	}
	services, err := s.ServiceRepository.FindByIDs(serviceIDs)
	if err != nil || len(services) != len(serviceIDs) {
		return apperrors.ErrBadRequest
	}
	if s.CenterRepository == nil {
		// Fail closed: without the schedule the client window cannot be proven,
		// so the booking is refused rather than silently allowed out of hours.
		return fmt.Errorf("reservation: client booking requires CenterRepository to validate working hours")
	}
	schedules, err := s.CenterRepository.FindWorkingHours(centerID, int(parsedDate.Weekday()))
	if err != nil {
		return err
	}
	startMinutes := startParsed.Hour()*60 + startParsed.Minute()
	endMinutes := startMinutes + TotalServicesDuration(services)
	if !BookingWindowAllowed(parsedDate, startMinutes, endMinutes, schedules, time.Now()) {
		return apperrors.ErrBadRequest
	}
	return nil
}

// validateClientServiceCapability enforces the personnel↔service capability
// rule on the client booking path: a professional with capability rows may only
// be booked for services among them. A professional with NO rows is
// UNRESTRICTED, which keeps the seeded/demo database (zero rows) and any fresh
// install fully bookable — the backward-compatible guarantee.
//
// personnelID == 0 means "no professional preference" (the centre assigns one
// later), so there is nothing to constrain. A nil repository disables the rule,
// which only happens in the sweeps wiring where Create is unreachable. The exact
// predicate is shared with the picker read filter via PersonnelCoversServices.
func (s *ReservationServiceImpl) validateClientServiceCapability(personnelID uint, serviceIDs []uint) error {
	if personnelID == 0 || s.PersonnelServiceRepository == nil {
		return nil
	}
	capabilityIDs, err := s.PersonnelServiceRepository.FindServiceIDsByPersonnel(personnelID)
	if err != nil {
		return err
	}
	if !PersonnelCoversServices(capabilityIDs, serviceIDs) {
		return apperrors.ErrBadRequest
	}
	return nil
}

// ──────────────────────────────────────────────
//  CREATE ADMIN — Walk-in (beauty_center | personnel)
// ──────────────────────────────────────────────

// CreateAdminReservation implements ReservationService (design D2/D4/D6).
// The route gate already restricted the HTTP surface, but the service re-proves
// every binding from the actor claims so direct callers cannot bypass them:
// centerID claim must exist and match the body, a personnel actor must be a
// member of that center (Personnel row), and the target client must exist and
// be active. The actual persistence delegates to createReservationCore (D1).
func (s *ReservationServiceImpl) CreateAdminReservation(req request.CreateAdminReservationRequest, actor ReservationActor) (*models.Reservation, error) {
	if err := s.Validate.Struct(req); err != nil {
		return nil, apperrors.ErrValidation
	}

	// D2: only center admin and staff may book walk-ins.
	if actor.Role != "beauty_center" && actor.Role != "personnel" {
		return nil, apperrors.ErrForbidden
	}

	// D2: the JWT centerID claim is the source of truth; a missing claim can
	// never bind a center, and a body center never overrides the claim.
	if actor.CenterID == 0 || actor.CenterID != req.BeautyCenterID {
		return nil, apperrors.ErrForbidden
	}

	// D2: personnel membership is proven from the Personnel row, not trusted
	// from the claim alone.
	if actor.Role == "personnel" {
		personnel, err := s.PersonnelRepository.FindByID(actor.ClientID)
		if err != nil || personnel == nil || personnel.BeautyCenterID != req.BeautyCenterID {
			return nil, apperrors.ErrForbidden
		}
	}

	// D4: the walk-in target must exist and be active — a clear 400 before any
	// DB write, never a deferred foreign-key failure.
	client, err := s.ClientRepository.FindById(req.ClientID)
	if err != nil || client == nil || !client.IsActive {
		return nil, apperrors.ErrBadRequest
	}

	id, err := s.createReservationCore(req.BeautyCenterID, req.ClientID, req.PersonnelID, req.ServiceIDs, req.Date, req.StartTime, nil)
	if err != nil {
		return nil, err
	}

	// Best-effort trigger (design D4): the center is told about its new walk-in.
	// The service already proved actor.CenterID == req.BeautyCenterID for both
	// roles, so the target equals req.BeautyCenterID. Never fails the write.
	_ = s.NotificationService.CreateForCenter(req.BeautyCenterID, "walkin_created",
		"Nueva reserva walk-in",
		fmt.Sprintf("Nueva reserva walk-in el %s a las %s", req.Date, req.StartTime),
		fmt.Sprintf(`{"reservation_id":%d,"client_id":%d}`, id, req.ClientID))

	// D6: return the reservation with relations preloaded so the controller can
	// emit the full ReservationResponse DTO (names, services, prices).
	return s.ReservationRepository.FindById(id)
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
	for _, svc := range services {
		if svc.BeautyCenterID != req.BeautyCenterID {
			return apperrors.ErrBadRequest
		}
		totalPrice += svc.Price
	}
	totalDuration := TotalServicesDuration(services)

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
	// Phase 1: every replacement line carries the reservation's lead (the
	// professional chosen for this update) so line attribution stays defined.
	linePersonnelID := req.PersonnelID
	for _, svc := range services {
		reservation.ReservationServices = append(reservation.ReservationServices, models.ReservationService{
			ServiceID:   svc.ID,
			ServiceName: svc.Name,
			Price:       svc.Price,
			Duration:    svc.Duration,
			PersonnelID: &linePersonnelID,
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

	return s.updateStatus(reservation, status)
}

// UpdateStatusByPersonnel lets a staff member transition ONLY their own
// reservations (design D1: claim-scoped, never a path parameter). Same state
// machine as the admin UpdateStatus, plus ownership: the reservation must be
// assigned to the acting staff member and belong to the acting center.
func (s *ReservationServiceImpl) UpdateStatusByPersonnel(reservationID uint, status string, personnelID, centerID uint) error {
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return apperrors.ErrNotFound
	}

	if reservation.PersonnelID != personnelID || reservation.BeautyCenterID != centerID {
		return apperrors.ErrForbidden
	}

	return s.updateStatus(reservation, status)
}

// updateStatus applies the shared state machine:
//
//	PENDING   → CONFIRMED, CANCELLED
//	CONFIRMED → COMPLETED, CANCELLED
//	COMPLETED → (terminal)
//	CANCELLED → (terminal)
func (s *ReservationServiceImpl) updateStatus(reservation *models.Reservation, status string) error {
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

	// Late-cancellation policy (Phase 3, opt-in per center): cancelling
	// inside the deadline window still cancels, but costs half a strike.
	// Best-effort like the trust engine — the cancel itself never fails here.
	if s.CenterRepository != nil {
		if center, cerr := s.CenterRepository.FindByID(reservation.BeautyCenterID); cerr == nil && center != nil && center.CancelDeadlineHours > 0 {
			if start, perr := slotStart(reservation); perr == nil {
				deadline := start.Add(-time.Duration(center.CancelDeadlineHours) * time.Hour)
				if time.Now().After(deadline) {
					if _, terr := s.ClientRepository.AdjustTrustScore(reservation.ClientID, TrustLateCancelPenalty); terr != nil {
						utils.Log.Errorf("[TRUST] Failed late-cancel penalty for client %d: %v", reservation.ClientID, terr)
					} else if s.NotificationService != nil {
						_ = s.NotificationService.Create(reservation.ClientID, "late_cancel",
							"Cancelación tardía",
							fmt.Sprintf("Cancelaste dentro de la ventana de %d hs. Tu confianza bajó 10 puntos.", center.CancelDeadlineHours),
							fmt.Sprintf(`{"reservation_id":%d}`, reservation.ID))
					}
				}
			}
		}
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
//  REMINDERS + NO-SHOW SWEEP (Phase 3)
// ──────────────────────────────────────────────

// Reminder windows before slot start.
const (
	Reminder24H = 24 * time.Hour
	Reminder2H  = 2 * time.Hour
)

// slotStart combines the date-only field with the HH:MM start time.
func slotStart(r *models.Reservation) (time.Time, error) {
	return time.Parse("2006-01-02 15:04",
		r.Date.Format("2006-01-02")+" "+r.StartTime)
}

// slotEnd combines the date-only field with the HH:MM end time.
func slotEnd(r *models.Reservation) (time.Time, error) {
	return time.Parse("2006-01-02 15:04",
		r.Date.Format("2006-01-02")+" "+r.EndTime)
}

// sendReminder delivers one in-app + push reminder and stamps the flag.
// Best-effort on push: the in-app row is the record, push is the nudge.
func (s *ReservationServiceImpl) sendReminder(r *models.Reservation, kind string, when time.Time) error {
	start, err := slotStart(r)
	if err != nil {
		return err
	}
	title := "Recordatorio de turno"
	message := fmt.Sprintf("Tenés turno el %s a las %s en %s",
		start.Format("02/01"), r.StartTime, centerNameOrDefault(r))
	dataStr := fmt.Sprintf(`{"reservation_id":%d}`, r.ID)
	pushData := fmt.Sprintf(`{"type":"reminder_%s","data":%q}`, kind, dataStr)
	if err := s.NotificationService.Create(r.ClientID, "reminder_"+kind, title, message, dataStr); err != nil {
		return err
	}
	_ = s.NotificationService.SendPushToClient(r.ClientID, title, message, pushData)
	if kind == "24h" {
		r.Reminder24HSentAt = &when
	} else {
		r.Reminder2HSentAt = &when
	}
	return s.ReservationRepository.Update(r)
}

func centerNameOrDefault(r *models.Reservation) string {
	if r.BeautyCenter.Name != "" {
		return r.BeautyCenter.Name
	}
	return "tu centro"
}

// SendDueReminders implements ReservationService.
func (s *ReservationServiceImpl) SendDueReminders(now time.Time) (int, error) {
	// Only CONFIRMED reservations in the next 48h can need a reminder.
	rows, err := s.ReservationRepository.FindConfirmedInDateRange(now, now.Add(48*time.Hour))
	if err != nil {
		return 0, err
	}
	sent := 0
	for i := range rows {
		r := rows[i]
		start, err := slotStart(&r)
		if err != nil || !start.After(now) {
			continue
		}
		until := start.Sub(now)
		switch {
		case until <= Reminder2H && r.Reminder2HSentAt == nil:
			if err := s.sendReminder(&r, "2h", now); err != nil {
				utils.Log.Errorf("[REMINDER] Failed 2h reminder for reservation %d: %v", r.ID, err)
				continue
			}
			sent++
		case until > Reminder2H && until <= Reminder24H && r.Reminder24HSentAt == nil:
			// The 24h notice only fires while the slot is actually ~a day
			// out: a last-minute booking must never receive a "24h"
			// reminder 90 minutes before.
			if err := s.sendReminder(&r, "24h", now); err != nil {
				utils.Log.Errorf("[REMINDER] Failed 24h reminder for reservation %d: %v", r.ID, err)
				continue
			}
			sent++
		}
	}
	return sent, nil
}

// SweepNoShows implements ReservationService.
func (s *ReservationServiceImpl) SweepNoShows(now time.Time, graceMinutes int) (int, error) {
	// Look back 3 days: anything older was either swept or predates the engine.
	rows, err := s.ReservationRepository.FindConfirmedInDateRange(now.Add(-72*time.Hour), now)
	if err != nil {
		return 0, err
	}
	struck := 0
	for i := range rows {
		r := rows[i]
		end, err := slotEnd(&r)
		if err != nil {
			continue
		}
		if now.After(end.Add(time.Duration(graceMinutes) * time.Minute)) {
			if err := s.RecordNoShow(r.ID, r.BeautyCenterID); err != nil {
				utils.Log.Errorf("[NOSHOW] Sweep failed for reservation %d: %v", r.ID, err)
				continue
			}
			struck++
		}
	}
	return struck, nil
}

// ──────────────────────────────────────────────
//  TRUST ENGINE (Phase 1)
// ──────────────────────────────────────────────

// RecordNoShow implements ReservationService. It marks a past CONFIRMED
// reservation as NO_SHOW and applies the trust penalty. NO_SHOW is a system
// terminal state set directly — it deliberately bypasses the updateStatus
// machine, which only models user-driven transitions.
func (s *ReservationServiceImpl) RecordNoShow(reservationID uint, centerID uint) error {
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return apperrors.ErrNotFound
	}

	// System actor must still prove center ownership (staff button or ticker
	// acting for the center — never a client, never another center).
	if centerID == 0 || centerID != reservation.BeautyCenterID {
		return apperrors.ErrForbidden
	}

	if reservation.Status != "CONFIRMED" {
		return apperrors.ErrConflict
	}

	// A no-show can only happen in the past: future slots are rejected so a
	// misclick or a rogue ticker can never strike a client early.
	slotStart, err := time.Parse("2006-01-02 15:04",
		reservation.Date.Format("2006-01-02")+" "+reservation.StartTime)
	if err != nil {
		return apperrors.ErrBadRequest
	}
	if !time.Now().After(slotStart) {
		return apperrors.ErrConflict
	}

	reservation.Status = "NO_SHOW"
	if err := s.ReservationRepository.Update(reservation); err != nil {
		return err
	}

	// Best-effort like the loyalty block: the strike must land, but a trust
	// write failure must not roll back an already-persisted terminal state.
	// (Phase 3 adds the client-facing strike notice.)
	if _, trustErr := s.ClientRepository.AdjustTrustScore(reservation.ClientID, TrustNoShowPenalty); trustErr != nil {
		utils.Log.Errorf("[TRUST] Failed to penalize no-show for client %d: %v", reservation.ClientID, trustErr)
	}

	return nil
}

// RequiresDeposit implements ReservationService. A client whose trust score
// is below the threshold must place a deposit; trusted clients skip the seña.
func (s *ReservationServiceImpl) RequiresDeposit(clientID uint, threshold int) (bool, error) {
	client, err := s.ClientRepository.FindById(clientID)
	if err != nil || client == nil {
		return false, apperrors.ErrNotFound
	}
	return client.TrustScore < threshold, nil
}

// ComputeDepositQuote resolves the seña due: a fixed amount wins, else the
// percent of the total, capped at the total itself. Zero means no deposit.
func ComputeDepositQuote(required bool, fixed, percent, total float64) float64 {
	if !required {
		return 0
	}
	if fixed > 0 {
		if fixed > total {
			return total
		}
		return fixed
	}
	if percent > 0 {
		quote := total * percent / 100
		if quote > total {
			return total
		}
		return quote
	}
	return 0
}

// ──────────────────────────────────────────────
//  DEPOSIT ("SEÑA") FLOW — zero-cost, receipt based
// ──────────────────────────────────────────────

// maybeExpireHold cancels a PENDING_PROOF reservation whose hold deadline
// passed. It reports whether it expired the row. Payment touchpoints call it
// first so a stale hold can never accept proof or review.
func (s *ReservationServiceImpl) maybeExpireHold(reservation *models.Reservation, now time.Time) bool {
	if reservation.PaymentStatus != PaymentPendingProof {
		return false
	}
	if reservation.Status != "PENDING" && reservation.Status != "CONFIRMED" {
		return false
	}
	if reservation.HoldExpiresAt == nil || !now.After(*reservation.HoldExpiresAt) {
		return false
	}
	reason := "hold expired"
	reservation.Status = "CANCELLED"
	reservation.CancelReason = &reason
	if err := s.ReservationRepository.Update(reservation); err != nil {
		utils.Log.Errorf("[DEPOSIT] Failed to expire hold on reservation %d: %v", reservation.ID, err)
		return false
	}
	return true
}

// applyDepositDecision runs after client creation: trusted clients and
// opted-out centers skip the flow; everyone else gets a PENDING_PROOF hold
// with a snapshotted amount and deadline.
func (s *ReservationServiceImpl) applyDepositDecision(reservation *models.Reservation) error {
	if s.CenterRepository == nil {
		return nil
	}
	center, err := s.CenterRepository.FindByID(reservation.BeautyCenterID)
	if err != nil || center == nil || !center.DepositRequired {
		return nil
	}
	threshold := center.DepositTrustThreshold
	if threshold <= 0 {
		threshold = DefaultDepositThreshold
	}
	need, rerr := s.RequiresDeposit(reservation.ClientID, threshold)
	if rerr != nil {
		// Fail-closed: unknown trust cannot skip the seña.
		need = true
	}
	if !need {
		return nil
	}
	quote := ComputeDepositQuote(true, center.DepositAmount, center.DepositPercent, reservation.TotalPrice)
	holdMin := center.DepositHoldMinutes
	if holdMin <= 0 {
		holdMin = 30
	}
	expires := time.Now().Add(time.Duration(holdMin) * time.Minute)
	reservation.PaymentStatus = PaymentPendingProof
	reservation.DepositAmount = quote
	reservation.HoldExpiresAt = &expires
	return s.ReservationRepository.Update(reservation)
}

// UploadProof implements ReservationService.
func (s *ReservationServiceImpl) UploadProof(reservationID, clientID uint, imageURL string) error {
	if imageURL == "" || len(imageURL) > 500 {
		return apperrors.ErrBadRequest
	}
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return apperrors.ErrNotFound
	}
	if reservation.ClientID != clientID {
		return apperrors.ErrForbidden
	}
	if s.maybeExpireHold(reservation, time.Now()) {
		return apperrors.ErrConflict
	}
	if reservation.PaymentStatus != PaymentPendingProof && reservation.PaymentStatus != PaymentRejected {
		return apperrors.ErrConflict
	}
	reservation.ProofImageURL = &imageURL
	reservation.PaymentStatus = PaymentPendingProof
	if err := s.ReservationRepository.Update(reservation); err != nil {
		return err
	}
	// Best-effort: the center reviews proofs from the agenda it already opens.
	_ = s.NotificationService.CreateForCenter(reservation.BeautyCenterID, "proof_uploaded",
		"Comprobante recibido",
		fmt.Sprintf("Comprobante de seña para el turno del %s a las %s", reservation.Date.Format("02/01"), reservation.StartTime),
		fmt.Sprintf(`{"reservation_id":%d}`, reservation.ID))
	return nil
}

// ReviewProof implements ReservationService.
func (s *ReservationServiceImpl) ReviewProof(reservationID, centerID uint, approve bool) error {
	reservation, err := s.ReservationRepository.FindById(reservationID)
	if err != nil || reservation == nil {
		return apperrors.ErrNotFound
	}
	if centerID == 0 || centerID != reservation.BeautyCenterID {
		return apperrors.ErrForbidden
	}
	if s.maybeExpireHold(reservation, time.Now()) {
		return apperrors.ErrConflict
	}
	if reservation.PaymentStatus != PaymentPendingProof {
		return apperrors.ErrConflict
	}
	// Terminal reservation states have nothing to confirm.
	if reservation.Status != "PENDING" && reservation.Status != "CONFIRMED" {
		return apperrors.ErrConflict
	}
	if approve {
		reservation.PaymentStatus = PaymentApproved
		if reservation.Status == "PENDING" {
			// PENDING → CONFIRMED is a legal machine step; persist both.
			return s.updateStatus(reservation, "CONFIRMED")
		}
		return s.ReservationRepository.Update(reservation)
	}
	reservation.PaymentStatus = PaymentRejected
	if err := s.ReservationRepository.Update(reservation); err != nil {
		return err
	}
	_ = s.NotificationService.Create(reservation.ClientID, "proof_rejected",
		"Comprobante observado",
		"El centro observó tu comprobante de seña. Subí uno nuevo antes del vencimiento.",
		fmt.Sprintf(`{"reservation_id":%d}`, reservation.ID))
	return nil
}

// ExpireHolds implements ReservationService.
func (s *ReservationServiceImpl) ExpireHolds(now time.Time) (int, error) {
	stale, err := s.ReservationRepository.FindExpiredHolds(now)
	if err != nil {
		return 0, err
	}
	swept := 0
	for i := range stale {
		r := stale[i]
		reason := "hold expired"
		r.Status = "CANCELLED"
		r.CancelReason = &reason
		if err := s.ReservationRepository.Update(&r); err != nil {
			utils.Log.Errorf("[DEPOSIT] Failed to sweep expired hold %d: %v", r.ID, err)
			continue
		}
		swept++
	}
	return swept, nil
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

// reservationPersonnelRecipients returns the distinct professionals who have a
// service line on the reservation, in first-seen line order. A line whose
// PersonnelID is nil inherits the reservation's derived lead (the Phase 2a
// kernel rule); a reservation with no lines at all falls back to its lead so
// legacy rows keep notifying exactly who they did before.
//
// The lines arrive with the caller's single FindById preload, so this resolves
// every recipient without an extra query and never looks one up per recipient.
func reservationPersonnelRecipients(reservation *models.Reservation) []uint {
	seen := make(map[uint]struct{}, len(reservation.ReservationServices)+1)
	recipients := make([]uint, 0, len(reservation.ReservationServices)+1)
	add := func(id uint) {
		if id == 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		recipients = append(recipients, id)
	}

	for i := range reservation.ReservationServices {
		if line := reservation.ReservationServices[i]; line.PersonnelID != nil {
			add(*line.PersonnelID)
		} else {
			add(reservation.PersonnelID)
		}
	}
	if len(reservation.ReservationServices) == 0 {
		add(reservation.PersonnelID)
	}
	return recipients
}

// NotifyPersonnelReassigned tells every LOSING professional their reservation
// was reassigned to another professional. The losing parties are the
// professionals on the reservation's PREVIOUS lines (the caller passes the
// pre-update reservation); the new assignee is excluded so a professional who
// kept a line is never told their own turn moved away.
func (s *ReservationServiceImpl) NotifyPersonnelReassigned(reservation *models.Reservation, personnelID uint) error {
	dataStr := fmt.Sprintf(`{"reservation_id":%d,"personnel_id":%d}`, reservation.ID, personnelID)
	title := "Turno reasignado"
	message := fmt.Sprintf("El turno del %s a las %s fue reasignado a otro profesional",
		reservation.Date.Format("02/01"),
		reservation.StartTime,
	)
	var firstErr error
	for _, losing := range reservationPersonnelRecipients(reservation) {
		if losing == personnelID {
			continue
		}
		if err := s.NotificationService.CreateForPersonnel(losing, "personnel_changed", title, message, dataStr); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// NotifyPersonnelCancelled tells EVERY professional with a line on the
// reservation that it was cancelled by the client. Each recipient gets their
// own row (never one row addressed to many); a failure for one recipient never
// stops the others, and the first error is returned best-effort.
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
	var firstErr error
	for _, personnelID := range reservationPersonnelRecipients(reservation) {
		if err := s.NotificationService.CreateForPersonnel(personnelID, "reservation_cancelled", title, message, dataStr); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
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
	// A client with no upcoming reservation is a normal empty state: the
	// repository returns (nil, nil) and the controller answers 200 with a
	// null body. Genuine lookup failures now propagate as 500s instead of
	// being masked as 404.
	return s.ReservationRepository.FindUpcomingByClientID(clientID)
}

func (s *ReservationServiceImpl) FindByBeautyCenterID(beautyCenterID, actorCenterID uint, from, to *time.Time, clientID *uint) ([]models.Reservation, error) {
	if actorCenterID != beautyCenterID {
		return nil, apperrors.ErrForbidden
	}
	// If from/to are provided, use them as the inclusive date range; otherwise return all reservations for this center.
	if from != nil || to != nil {
		return s.ReservationRepository.FindByBeautyCenterId(beautyCenterID, from, to, clientID)
	}
	return s.ReservationRepository.FindByBeautyCenterId(beautyCenterID, nil, nil, clientID)
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
	// VerifyQR closes the token-economy loop, so it needs the services the
	// Phase 3 scheduler deliberately omits (see the struct contract). Return a
	// clear error instead of dereferencing a nil dependency.
	if s.TokenAccountService == nil || s.TokenAccountRepo == nil || s.LoyaltyService == nil {
		return nil, false, fmt.Errorf("reservation: VerifyQR requires TokenAccountService, TokenAccountRepo and LoyaltyService to be wired")
	}

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

	// ── Token consumption: config-driven charge, atomic with completion ──
	// feat-token-economy-config D3/D4: the charge comes from market_config
	// (percentage + floor) and status flip + guarded debit + ledger commit as
	// ONE transaction. A debit failure rolls back the completion — no more
	// completed turns that never paid.

	_, overdraftMilli, currency, err := s.TokenAccountService.GetBalance(authBeautyCenterID)
	if err != nil {
		return nil, false, err
	}
	economy := defaultEconomy
	if s.MarketConfigService != nil {
		economy = s.MarketConfigService.GetEconomyForCurrency(currency)
	}

	totalMilli := utils.ToMilli(reservation.TotalPrice)
	charge := ComputeChargeMilli(totalMilli, economy.CommissionPct, economy.MinChargeMilli)

	account, err := s.TokenAccountService.GetOrCreate(authBeautyCenterID, currency)
	if err != nil {
		return nil, false, err
	}

	var alreadyCompleted bool
	err = s.ReservationRepository.Transact(func(tx *gorm.DB) error {
		completed, txErr := s.ReservationRepository.CompleteVerificationTx(tx, reservation.ID)
		if txErr != nil {
			return txErr
		}
		if !completed {
			alreadyCompleted = true // concurrent scan won — free no-op success
			return nil
		}
		return s.TokenAccountRepo.DebitInTx(
			tx, account.ID, charge, overdraftMilli,
			"consumption",
			fmt.Sprintf("Reservation #%d completed — %.2f%% of %.2f", reservation.ID, economy.CommissionPct, reservation.TotalPrice),
			&reservation.ID,
		)
	})
	if err != nil {
		utils.Log.Errorf("[TOKEN] Charge failed for reservation %d (rolled back): %v", reservation.ID, err)
		return nil, false, err
	}
	if alreadyCompleted {
		utils.Log.Infof("[QR] Reservation ID %d already completed by another scan — no-op", reservation.ID)
		return reservation, true, nil
	}

	// Mirror the persisted state onto the returned struct (legacy behavior):
	// callers read these fields directly after a successful verification.
	reservation.Status = "COMPLETED"
	reservation.VerificationToken = nil
	reservation.TokenExpiresAt = nil

	// ── Trust reward (Phase 1): showing up rebuilds reputation. Best-effort
	// like the loyalty block below — a trust write must never fail check-in.
	// Idempotent re-scans (alreadyCompleted) do not double-reward.
	if _, trustErr := s.ClientRepository.AdjustTrustScore(reservation.ClientID, TrustAttendanceReward); trustErr != nil {
		utils.Log.Errorf("[TRUST] Failed to reward attendance for client %d: %v", reservation.ClientID, trustErr)
	}

	// ── Loyalty stamps (only if center has loyalty enabled) ──
	settings, settingsErr := s.LoyaltyService.GetCenterSettings(authBeautyCenterID)
	if settingsErr == nil && settings.LoyaltyEnabled {
		for _, svc := range reservation.ReservationServices {
			err = s.LoyaltyService.AddPunch(reservation.ID, reservation.ClientID, reservation.BeautyCenterID, svc.ServiceID)
			if err != nil {
				utils.Log.Errorf("[Loyalty Error] failed to add punch for service %d via QR for Client %d; %v", svc.ServiceID, reservation.ClientID, err)
			}
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

// ComputeChargeMilli resolves the token charge for a reservation in integer
// milli-YOTTU (feat-token-economy-config D3/D5): percentage of the total with
// a per-market floor, exact integer math (pct treated at 2-decimal precision).
func ComputeChargeMilli(totalMilli int64, commissionPct float64, minChargeMilli int64) int64 {
	bp := int64(math.Round(commissionPct * 100)) // 3.5% → 350 basis points
	pctCharge := totalMilli * bp / 10000
	if pctCharge < minChargeMilli {
		return minChargeMilli
	}
	return pctCharge
}
