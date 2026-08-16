package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserSessionRepositoryImpl struct {
	db *gorm.DB
}

func NewUserSessionRepositoryImpl(db *gorm.DB) UserSessionRepository {
	return &UserSessionRepositoryImpl{db: db}
}

func (r *UserSessionRepositoryImpl) Create(ctx context.Context, session *models.UserSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *UserSessionRepositoryImpl) FindByHash(ctx context.Context, hash string) (*models.UserSession, error) {
	var session models.UserSession
	err := r.db.WithContext(ctx).Where("refresh_token_hash = ?", hash).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

func (r *UserSessionRepositoryImpl) RotateCAS(ctx context.Context, sessionID uint, presentedHash string) (bool, error) {
	return r.rotateCASWith(r.db.WithContext(ctx), sessionID, presentedHash)
}

func (r *UserSessionRepositoryImpl) rotateCASWith(tx *gorm.DB, sessionID uint, presentedHash string) (bool, error) {
	res := tx.Model(&models.UserSession{}).
		Where("id = ? AND refresh_token_hash = ? AND is_revoked = ?", sessionID, presentedHash, false).
		Update("is_revoked", true)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// Rotate implements D2's atomic rotation:
//   - SELECT ... FOR UPDATE the generation presenting oldHash (a real row lock
//     on Postgres; a harmless no-op on SQLite, where the CAS is the guard).
//   - revoked generation → the token was already rotated → revoke the whole
//     family → ErrSessionReuse (committed).
//   - expired generation → ErrSessionExpired (lifecycle, not theft).
//   - active → issue() builds the next generation, RotateCAS claims the old
//     one, and the new generation row is inserted — all in one transaction.
//   - a lost CAS (concurrent rotation) also revokes the family.
//
// Reuse outcomes (family revocation) MUST commit, so they are executed inside
// the transaction and surfaced as the returned error after it commits.
func (r *UserSessionRepositoryImpl) Rotate(ctx context.Context, presentedHash string,
	issue func(active *models.UserSession) (*models.UserSession, error)) (*models.UserSession, error) {

	var newSession *models.UserSession
	var outcome error

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session models.UserSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("refresh_token_hash = ?", presentedHash).First(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				outcome = ErrSessionNotFound
				return nil
			}
			return err
		}

		if session.IsRevoked {
			// An already-rotated generation is the reuse/theft signal (D2).
			if err := revokeFamilyWith(tx, session.FamilyID); err != nil {
				return err
			}
			outcome = ErrSessionReuse
			return nil
		}

		if time.Now().After(session.ExpiresAt) {
			outcome = ErrSessionExpired
			return nil
		}

		next, err := issue(&session)
		if err != nil {
			return err
		}

		claimed, err := r.rotateCASWith(tx, session.ID, presentedHash)
		if err != nil {
			return err
		}
		if !claimed {
			// Someone rotated this exact generation between our read and write.
			if err := revokeFamilyWith(tx, session.FamilyID); err != nil {
				return err
			}
			outcome = ErrSessionReuse
			return nil
		}

		if err := tx.Create(next).Error; err != nil {
			return err
		}
		newSession = next
		return nil
	})
	if err != nil {
		return nil, err
	}
	return newSession, outcome
}

func revokeFamilyWith(tx *gorm.DB, familyID string) error {
	return tx.Model(&models.UserSession{}).
		Where("family_id = ?", familyID).
		Update("is_revoked", true).Error
}

func (r *UserSessionRepositoryImpl) RevokeByHash(ctx context.Context, hash string) error {
	return r.db.WithContext(ctx).Model(&models.UserSession{}).
		Where("refresh_token_hash = ? AND is_revoked = ?", hash, false).
		Update("is_revoked", true).Error
}

func (r *UserSessionRepositoryImpl) RevokeFamily(ctx context.Context, familyID string) error {
	return revokeFamilyWith(r.db.WithContext(ctx), familyID)
}
