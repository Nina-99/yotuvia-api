package repository

import (
	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type TokenTopUpRepositoryImpl struct {
	db *gorm.DB
}

func NewTokenTopUpRepositoryImpl(db *gorm.DB) TokenTopUpRepository {
	return &TokenTopUpRepositoryImpl{db: db}
}

func (r *TokenTopUpRepositoryImpl) Create(topup *models.TokenTopUp) error {
	return r.db.Create(topup).Error
}

func (r *TokenTopUpRepositoryImpl) Update(topup *models.TokenTopUp) error {
	return r.db.Save(topup).Error
}

func (r *TokenTopUpRepositoryImpl) FindByID(id uint) (*models.TokenTopUp, error) {
	var topup models.TokenTopUp
	err := r.db.First(&topup, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &topup, nil
}

func (r *TokenTopUpRepositoryImpl) FindByBeautyCenterID(beautyCenterID uint) ([]models.TokenTopUp, error) {
	var topups []models.TokenTopUp
	err := r.db.Where("beauty_center_id = ?", beautyCenterID).Order("created_at DESC").Find(&topups).Error
	return topups, err
}

func (r *TokenTopUpRepositoryImpl) FindPending() ([]models.TokenTopUp, error) {
	var topups []models.TokenTopUp
	err := r.db.Where("status = ?", "pending").Order("created_at ASC").Find(&topups).Error
	return topups, err
}

func (r *TokenTopUpRepositoryImpl) FindAll(page, limit int) ([]models.TokenTopUp, int64, error) {
	var topups []models.TokenTopUp
	var total int64
	r.db.Model(&models.TokenTopUp{}).Count(&total)
	offset := (page - 1) * limit
	err := r.db.Offset(offset).Limit(limit).Order("created_at DESC").Find(&topups).Error
	return topups, total, err
}

func (r *TokenTopUpRepositoryImpl) FindByCenterIDWithStatus(beautyCenterID uint, page, limit int, status string) ([]models.TokenTopUp, int64, error) {
	var topups []models.TokenTopUp
	var total int64

	query := r.db.Model(&models.TokenTopUp{}).Where("beauty_center_id = ?", beautyCenterID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	offset := (page - 1) * limit
	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&topups).Error
	return topups, total, err
}

// FindAllWithFilters is the platform-wide queue read. Both filters are honoured
// by the query itself (never faked in the handler): a nil pointer means "every
// centre", an empty status means "every status".
func (r *TokenTopUpRepositoryImpl) FindAllWithFilters(page, limit int, beautyCenterID *uint, status string) ([]models.TokenTopUp, int64, error) {
	var topups []models.TokenTopUp
	var total int64

	query := r.db.Model(&models.TokenTopUp{})
	if beautyCenterID != nil {
		query = query.Where("beauty_center_id = ?", *beautyCenterID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * limit
	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&topups).Error
	return topups, total, err
}
