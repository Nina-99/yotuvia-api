package service

import (
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
)

type LoyaltyService interface {
	CreateProgram(req request.CreateLoyaltyProgrmaRequest) (response.LoyaltyProgramResponse, error)
	AddPunch(reservationID, clientID, beautyCenterID, serviceID uint) error
	// GetClientsCards returns a client's punch cards. A `client` actor may only
	// read its own cards (actorClientID claim == clientID); beauty_center and
	// personnel legitimately inspect any client's cards.
	GetClientsCards(clientID, actorClientID uint, role string) ([]response.PunchCardResponse, error)
	// GetHistory returns the client's punch timeline (newest first). Same
	// ownership contract as GetClientsCards: client-owns-cards.
	GetHistory(clientID, actorClientID uint, role string) ([]response.PunchHistoryItemResponse, error)
	// GetPendingRewards returns completed-but-unredeemed cards for the client.
	// Same ownership contract as GetClientsCards.
	GetPendingRewards(clientID, actorClientID uint, role string) ([]response.PendingRewardResponse, error)
	// GetPublicPrograms returns active program summaries for a center.
	// Anonymous-safe: no auth check, minimal fields.
	GetPublicPrograms(beautyCenterID uint) ([]response.PublicProgramResponse, error)
	// HasPendingRedemption returns true if the client has a completed punch
	// card with a pending (unredeemed) reward for the given service.
	HasPendingRedemption(clientID, serviceID, beautyCenterID uint) bool
	// CalculateRedemption computes the discount amount without committing.
	// Returns (discount, cardID, nil). The caller must later call
	// FinalizeRedemption to persist the record.
	CalculateRedemption(clientID, serviceID, beautyCenterID uint) (discount float64, cardID uint, err error)
	// FinalizeRedemption records the redemption, resets the card, and links it
	// to the given reservation. This must be called after reservation creation.
	FinalizeRedemption(cardID, reservationID uint) error
	// FindRedemptionByReservation checks if a reservation already has a loyalty
	// redemption applied (idempotency guard).
	FindRedemptionByReservation(reservationID uint) bool

	// ── Center program management ──
	// Every method is scoped to the acting center's centerID claim (actorCenterID):
	// a ownership mismatch returns ErrForbidden. RequireRole proves the role, not
	// the ownership, so the check lives here (same contract as reservations).
	// FindProgramsByCenter returns all loyalty programs for a beauty center.
	FindProgramsByCenter(beautyCenterID, actorCenterID uint) ([]response.LoyaltyProgramResponse, error)
	// UpdateProgram updates an existing loyalty program owned by actorCenterID.
	UpdateProgram(id, actorCenterID uint, req request.UpdateLoyaltyProgramRequest) (response.LoyaltyProgramResponse, error)
	// DeleteProgram soft-deletes a loyalty program owned by actorCenterID.
	DeleteProgram(id, actorCenterID uint) error
	// ToggleProgram enables or disables a loyalty program owned by actorCenterID.
	ToggleProgram(id, actorCenterID uint, isActive bool) error
	// GetCenterSettings returns the loyalty settings for a beauty center. The
	// actor's centerID must match beautyCenterID.
	GetCenterSettings(beautyCenterID, actorCenterID uint) (response.LoyaltySettingsResponse, error)
	// UpdateCenterSettings updates the loyalty settings for a beauty center. The
	// actor's centerID must match beautyCenterID.
	UpdateCenterSettings(beautyCenterID, actorCenterID uint, req request.UpdateLoyaltySettingsRequest) error

	// GetMetrics returns per-program issued-vs-redeemed counts for a center.
	// actorCenterID must equal beautyCenterID (403 otherwise). Empty programs
	// return zero-valued rows, never null.
	GetMetrics(beautyCenterID, actorCenterID uint) ([]response.LoyaltyMetricsResponse, error)
}
