package repository

import (
	"errors"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TikTokAccountRepositoryImpl struct {
	Db *gorm.DB
}

func NewTikTokAccountRepositoryImpl(Db *gorm.DB) TikTokAccountRepository {
	return &TikTokAccountRepositoryImpl{Db: Db}
}

// Upsert replaces the row for the actor key, reusing the existing primary key
// so a reconnect never leaves a stale duplicate behind.
func (r *TikTokAccountRepositoryImpl) Upsert(account *models.TikTokAccount) error {
	var existing models.TikTokAccount
	err := r.Db.Where("beauty_center_id = ? AND personnel_id = ?",
		account.BeautyCenterID, account.PersonnelID).First(&existing).Error
	switch {
	case err == nil:
		account.ID = existing.ID
		account.CreatedAt = existing.CreatedAt
		return r.Db.Model(&existing).Select("*").Omit("id", "created_at", "deleted_at").Updates(account).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		return r.Db.Create(account).Error
	default:
		return err
	}
}

// FindByActor returns (nil, nil) when the actor has no connected account, so
// callers do not need to know about gorm sentinel errors.
func (r *TikTokAccountRepositoryImpl) FindByActor(centerID, personnelID uint) (*models.TikTokAccount, error) {
	var account models.TikTokAccount
	err := r.Db.Where("beauty_center_id = ? AND personnel_id = ?", centerID, personnelID).
		First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *TikTokAccountRepositoryImpl) DeleteByActor(centerID, personnelID uint) error {
	return r.Db.Where("beauty_center_id = ? AND personnel_id = ?", centerID, personnelID).
		Delete(&models.TikTokAccount{}).Error
}

func (r *TikTokAccountRepositoryImpl) CreateState(state *models.TikTokConnectionState) error {
	return r.Db.Create(state).Error
}

// ConsumeState deletes the row inside the same transaction that reads it, so a
// replayed callback can never reuse a state.
func (r *TikTokAccountRepositoryImpl) ConsumeState(value string) (*models.TikTokConnectionState, error) {
	var state models.TikTokConnectionState
	err := r.Db.Transaction(func(tx *gorm.DB) error {
		if findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("state = ?", value).First(&state).Error; findErr != nil {
			return findErr
		}
		return tx.Delete(&state).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}
