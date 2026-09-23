package repository

import "github.com/Nina-99/yottu-api/models"

type LoyaltyRepository interface {
	CreateProgram(program *models.LoyaltyProgram) error
	FindProgramByService(beautyCenterID, serviceID uint) (*models.LoyaltyProgram, error)
	FindActiveCard(clientID, programID uint) (*models.CustomerPunchCard, error)
	CreatePunchCard(card *models.CustomerPunchCard) error
	UpdatePunchCard(card *models.CustomerPunchCard) error
	CreatePunchHistory(history *models.PunchHistory) error
	FindCardsByClient(clientID uint) ([]models.CustomerPunchCard, error)
	// FindCompletedCard returns the first "completed" card for a client on a
	// given program, or nil if none exists. This is the card eligible for
	// reward redemption.
	FindCompletedCard(clientID, programID uint) (*models.CustomerPunchCard, error)
	// FindCompletedCardByCardID returns a completed card by its ID.
	FindCompletedCardByCardID(cardID uint) (*models.CustomerPunchCard, error)
	// CreateRedemption records that a completed card's reward was applied to a
	// reservation.
	CreateRedemption(redemption *models.LoyaltyRedemption) error
	// FindRedemptionByCard checks if a specific card has already been redeemed.
	FindRedemptionByCard(cardID uint) (*models.LoyaltyRedemption, error)
	// FindRedemptionByReservation checks if a reservation already has a loyalty
	// redemption applied.
	FindRedemptionByReservation(reservationID uint) (*models.LoyaltyRedemption, error)

	// ── Center program management ──
	// FindProgramsByCenter returns all loyalty programs for a beauty center.
	FindProgramsByCenter(beautyCenterID uint) ([]models.LoyaltyProgram, error)
	// FindProgramByID returns a single program by ID.
	FindProgramByID(id uint) (*models.LoyaltyProgram, error)
	// UpdateProgram updates a loyalty program.
	UpdateProgram(program *models.LoyaltyProgram) error
	// DeleteProgram soft-deletes a loyalty program.
	DeleteProgram(program *models.LoyaltyProgram) error
	// UpdateProgramsActiveByCenter activates or deactivates all programs for a center.
	UpdateProgramsActiveByCenter(beautyCenterID uint, isActive bool) error
	// FindActiveCardForPersonnel finds an active card for a specific personnel.
	FindActiveCardForPersonnel(clientID, programID, personnelID uint) (*models.CustomerPunchCard, error)
}
