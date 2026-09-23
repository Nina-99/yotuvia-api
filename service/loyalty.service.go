package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type LoyaltyService interface {
	CreateProgram(req request.CreateLoyaltyProgrmaRequest) (response.LoyaltyProgramResponse, error)
	AddPunch(reservationID, clientID, beautyCenterID, serviceID uint) error
	GetClientsCards(clientID uint) ([]response.PunchCardResponse, error)
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
	// FindProgramsByCenter returns all loyalty programs for a beauty center.
	FindProgramsByCenter(beautyCenterID uint) ([]response.LoyaltyProgramResponse, error)
	// UpdateProgram updates an existing loyalty program.
	UpdateProgram(id uint, req request.UpdateLoyaltyProgramRequest) (response.LoyaltyProgramResponse, error)
	// DeleteProgram soft-deletes a loyalty program.
	DeleteProgram(id uint) error
	// ToggleProgram enables or disables a loyalty program.
	ToggleProgram(id uint, isActive bool) error
	// GetCenterSettings returns the loyalty settings for a beauty center.
	GetCenterSettings(beautyCenterID uint) (response.LoyaltySettingsResponse, error)
	// UpdateCenterSettings updates the loyalty settings for a beauty center.
	UpdateCenterSettings(beautyCenterID uint, req request.UpdateLoyaltySettingsRequest) error
}
