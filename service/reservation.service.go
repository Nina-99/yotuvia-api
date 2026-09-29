package service

import (
	"time"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
)

type ReservationActor struct {
	// ClientID is the JWT subject: the client ID for the "client" role, the
	// personnel ID for the "personnel" role, the admin account ID otherwise.
	ClientID uint
	// CenterID is the JWT centerID claim (0 when the token carries none).
	CenterID uint
	// Role is the JWT role claim: "client" | "personnel" | "beauty_center".
	Role string
}

// Payment status values for the zero-cost deposit ("seña") flow (Phase 1).
const (
	PaymentNone         = "NONE"
	PaymentPendingProof = "PENDING_PROOF"
	PaymentApproved     = "APPROVED"
	PaymentRejected     = "REJECTED"
)

// ValidPaymentStatus reports whether s is a known deposit payment status.
func ValidPaymentStatus(s string) bool {
	switch s {
	case PaymentNone, PaymentPendingProof, PaymentApproved, PaymentRejected:
		return true
	default:
		return false
	}
}

// Trust engine tuning (Phase 1). Score range is [TrustMin, TrustMax];
// new clients start at TrustInitial.
const (
	TrustInitial          = 100
	TrustMin              = 0
	TrustMax              = 100
	TrustNoShowPenalty    = -20
	TrustAttendanceReward = 2
	// TrustLateCancelPenalty applies when a client cancels inside the
	// center's late window (Phase 3). Half a no-show: they warned, but the
	// slot is likely lost.
	TrustLateCancelPenalty = -10
	// DefaultDepositThreshold is the score below which a center should
	// require a deposit. Centers calibrate their own threshold via config.
	DefaultDepositThreshold = 70
)

type ReservationService interface {
	Create(req request.CreateReservationRequest) error
	// CreateAdminReservation books a walk-in on behalf of a center admin or
	// staff member (design D2/D4/D6). The actor's centerID claim is the source
	// of truth and is bound against the request body; staff membership is
	// proven via the Personnel row; the target client must exist and be
	// active. It returns the created reservation (preloaded for DTO mapping).
	CreateAdminReservation(req request.CreateAdminReservationRequest, actor ReservationActor) (*models.Reservation, error)
	// Update/Delete/FindById/GenerateVerificationToken take actor params
	// (clientID, centerID, role claims) and enforce ownership: a client may act
	// on its own reservation, a center on its own center's reservation.
	Update(reservationID uint, req request.UpdateReservationRequest, clientID, centerID uint, role string) error
	Delete(reservationID uint, clientID, centerID uint, role string) error
	UpdateStatus(reservationID uint, status string, centerID uint) error
	// UpdateStatusByPersonnel lets a staff member transition a reservation they
	// PARTICIPATE in — the derived lead or the professional of one of its
	// service lines — scoped to the acting center. Membership uses the same
	// predicate as the agenda (personnelMembershipSQL), so a professional can
	// operate on exactly what they can see. Same state machine as UpdateStatus.
	UpdateStatusByPersonnel(reservationID uint, status string, personnelID, centerID uint) error
	// PersonnelMetrics returns the server-side aggregated metrics for the
	// acting professional (personnelID from the JWT claim) over the inclusive
	// [from, to] range, zero-filled per day. Revenue is COMPLETED-only and
	// attributed per line; it mirrors the centre admin definitions so the two
	// surfaces agree. centerID resolves the currency.
	PersonnelMetrics(personnelID, centerID uint, from, to time.Time) (*response.PersonnelMetricsResponse, error)
	Cancel(reservationID uint, clientID uint, reason string) error
	// FindAll is scoped to the caller's centerID claim (beauty_center only).
	FindAll(page, limit int, centerID uint) ([]models.Reservation, int64, error)
	FindById(reservationID uint, clientID, centerID uint, role string) (*models.Reservation, error)
	// FindByClientID/FindUpcomingByClientID reject a path clientID that does not
	// match the actor's clientID claim.
	FindByClientID(clientID, actorClientID uint) ([]models.Reservation, error)
	FindUpcomingByClientID(clientID, actorClientID uint) (*models.Reservation, error)
	// FindByBeautyCenterID allows center staff only for their own centerID.
	// The optional from/to date range scopes the agenda to that inclusive range
	// (design D4); the optional date (parsed YYYY-MM-DD) scopes it to one day
	// (design D3); nil params keep the current unfiltered behavior.
	FindByBeautyCenterID(beautyCenterID, actorCenterID uint, from, to *time.Time, clientID *uint) ([]models.Reservation, error)
	// FindByPersonnelID returns a staff member's agenda. The personnelID is the
	// caller's own claim (never a path parameter); from/to are an optional
	// inclusive date range (nil = open-ended).
	FindByPersonnelID(personnelID uint, from, to *time.Time) ([]models.Reservation, error)
	GenerateVerificationToken(reservationID uint, clientID, centerID uint, role string) (string, error)
	// VerifyQR completes a reservation exactly once and reports whether the
	// reservation was ALREADY COMPLETED before this call: false when this call
	// performed the completion, true for an idempotent re-scan or a lost
	// exactly-once race.
	VerifyQR(token string, authBeautyCenterID uint) (*models.Reservation, bool, error)
	AssignPersonnel(reservationID uint, personnelID uint, centerID uint) error
	// RecordNoShow marks a past CONFIRMED reservation as NO_SHOW and applies
	// the trust penalty to the client. Only the owning center may call it
	// (staff button today, system ticker in Phase 3). Future slots and
	// non-CONFIRMED states are rejected — a no-show can only happen in the past.
	RecordNoShow(reservationID uint, centerID uint) error
	// RequiresDeposit reports whether a client must place a deposit, i.e. its
	// trust score is below the given threshold. Trusted clients skip the seña.
	RequiresDeposit(clientID uint, threshold int) (bool, error)
	// UploadProof attaches the bank receipt photo to a deposit hold. Only the
	// owning client, only while PENDING_PROOF (or REJECTED for re-upload).
	// An expired hold is cancelled first and reported as conflict.
	UploadProof(reservationID, clientID uint, imageURL string) error
	// ReviewProof approves or rejects a deposit receipt. Only the owning
	// center. Approving a PENDING reservation also CONFIRMs it; rejecting
	// returns it to REJECTED so the client can re-upload while the hold runs.
	ReviewProof(reservationID, centerID uint, approve bool) error
	// ExpireHolds cancels every PENDING_PROOF hold past its deadline and
	// returns how many were swept. No trust penalty: nothing was confirmed.
	// Called by the Phase 3 ticker; payment touchpoints also expire lazily.
	ExpireHolds(now time.Time) (int, error)
	// SendDueReminders sends the one-shot 24h/2h push+in-app reminders for
	// CONFIRMED reservations and returns how many reminders were sent.
	// Idempotent: each flag is set once (Phase 3 ticker calls this).
	SendDueReminders(now time.Time) (int, error)
	// SweepNoShows marks past CONFIRMED reservations (slot end + grace
	// elapsed, no check-in) as NO_SHOW with trust penalty. Returns the
	// count struck. Called by the Phase 3 ticker.
	SweepNoShows(now time.Time, graceMinutes int) (int, error)
}
