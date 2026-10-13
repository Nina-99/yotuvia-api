package repository

import "github.com/Nina-99/yotuvia-api/models"

// TikTokAccountRepository persists connected TikTok accounts and the
// short-lived OAuth CSRF states.
type TikTokAccountRepository interface {
	// Upsert creates or replaces the account for (centerID, personnelID).
	Upsert(account *models.TikTokAccount) error
	FindByActor(centerID, personnelID uint) (*models.TikTokAccount, error)
	DeleteByActor(centerID, personnelID uint) error

	CreateState(state *models.TikTokConnectionState) error
	// ConsumeState atomically fetches and deletes a state row, returning nil
	// when it is unknown.
	ConsumeState(value string) (*models.TikTokConnectionState, error)
}
