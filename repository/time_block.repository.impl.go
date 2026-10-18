package repository

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type TimeBlockRepositoryImpl struct {
	db *gorm.DB
}

func NewTimeBlockRepositoryImpl(db *gorm.DB) TimeBlockRepository {
	return &TimeBlockRepositoryImpl{db: db}
}

func (r *TimeBlockRepositoryImpl) Create(block *models.TimeBlock) error {
	return r.db.Create(block).Error
}

func (r *TimeBlockRepositoryImpl) FindByID(id uint) (*models.TimeBlock, error) {
	var block models.TimeBlock
	if err := r.db.First(&block, id).Error; err != nil {
		return nil, err
	}
	return &block, nil
}

func (r *TimeBlockRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.TimeBlock{}, id).Error
}

func (r *TimeBlockRepositoryImpl) ListByCentersRange(centerIDs []uint, from, to time.Time) ([]models.TimeBlock, error) {
	if len(centerIDs) == 0 {
		return []models.TimeBlock{}, nil
	}
	var blocks []models.TimeBlock
	err := r.db.
		Where("beauty_center_id IN ?", centerIDs).
		Where("date >= ? AND date <= ?", dateOnly(from), dateOnly(to)).
		Find(&blocks).Error
	return blocks, err
}

// dateOnly truncates to midnight UTC so a stored day compares equal to the
// day keys the availability kernel uses.
func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
