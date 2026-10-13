package repository

import (
	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PersonnelFollowRepositoryImpl struct {
	Db *gorm.DB
}

func NewPersonnelFollowRepositoryImpl(db *gorm.DB) PersonnelFollowRepository {
	return &PersonnelFollowRepositoryImpl{Db: db}
}

// Follow implements PersonnelFollowRepository. OnConflict DoNothing turns a
// duplicate follow into a silent no-op, so the DB unique index (not a
// read-then-write race) is what guarantees a single row per pair.
func (r *PersonnelFollowRepositoryImpl) Follow(follow *models.PersonnelFollow) error {
	return r.Db.Clauses(clause.OnConflict{DoNothing: true}).Create(follow).Error
}

// Unfollow implements PersonnelFollowRepository. Deleting a non-existent edge
// affects zero rows and still returns nil — idempotent by construction.
func (r *PersonnelFollowRepositoryImpl) Unfollow(clientID, personnelID uint) error {
	return r.Db.Where("client_id = ? AND personnel_id = ?", clientID, personnelID).
		Delete(&models.PersonnelFollow{}).Error
}

func (r *PersonnelFollowRepositoryImpl) IsFollowing(clientID, personnelID uint) (bool, error) {
	var count int64
	err := r.Db.Model(&models.PersonnelFollow{}).
		Where("client_id = ? AND personnel_id = ?", clientID, personnelID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PersonnelFollowRepositoryImpl) CountByPersonnelID(personnelID uint) (int64, error) {
	var count int64
	err := r.Db.Model(&models.PersonnelFollow{}).
		Where("personnel_id = ?", personnelID).
		Count(&count).Error
	return count, err
}

// FindFollowerIDsByPersonnelID loads every follower client id for one
// professional in a single SELECT (Pluck), so the caller never runs a query
// per follower. limit caps the fan-out.
func (r *PersonnelFollowRepositoryImpl) FindFollowerIDsByPersonnelID(personnelID uint, limit int) ([]uint, error) {
	ids := make([]uint, 0)
	err := r.Db.Model(&models.PersonnelFollow{}).
		Where("personnel_id = ?", personnelID).
		Order("id ASC").
		Limit(limit).
		Pluck("client_id", &ids).Error
	return ids, err
}
