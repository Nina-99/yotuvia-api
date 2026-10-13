package repository

import (
	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type PromotionRepositoryImpl struct {
	db *gorm.DB
}

func NewPromotionRepositoryImpl(db *gorm.DB) PromotionRepository {
	return &PromotionRepositoryImpl{db: db}
}

func (r *PromotionRepositoryImpl) Create(promotion *models.Promotion) error {
	return r.db.Create(promotion).Error
}

func (r *PromotionRepositoryImpl) FindByID(id uint) (*models.Promotion, error) {
	var promo models.Promotion
	err := r.db.Preload("Service").First(&promo, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &promo, nil
}

func (r *PromotionRepositoryImpl) FindByCenter(beautyCenterID uint, includeInactive bool) ([]models.Promotion, error) {
	var promos []models.Promotion
	query := r.db.Preload("Service").Where("beauty_center_id = ?", beautyCenterID)
	if !includeInactive {
		query = query.Where("is_active = ?", true)
	}
	err := query.Order("created_at DESC").Find(&promos).Error
	return promos, err
}

func (r *PromotionRepositoryImpl) FindActiveForService(beautyCenterID, serviceID uint, hour, dayOfWeek int) ([]models.Promotion, error) {
	var promos []models.Promotion
	err := r.db.Preload("Service").
		Where("beauty_center_id = ? AND is_active = true AND (service_id = ? OR service_id IS NULL)", beautyCenterID, serviceID).
		Where("(valid_from IS NULL OR valid_from <= NOW()) AND (valid_to IS NULL OR valid_to >= NOW())").
		Where("(max_redemptions = 0 OR current_redemptions < max_redemptions)").
		Find(&promos).Error
	return promos, err
}

func (r *PromotionRepositoryImpl) FindActiveForCenter(beautyCenterID uint, hour, dayOfWeek int) ([]models.Promotion, error) {
	var promos []models.Promotion
	err := r.db.Preload("Service").
		Where("beauty_center_id = ? AND is_active = true", beautyCenterID).
		Where("(valid_from IS NULL OR valid_from <= NOW()) AND (valid_to IS NULL OR valid_to >= NOW())").
		Where("(max_redemptions = 0 OR current_redemptions < max_redemptions)").
		Find(&promos).Error
	return promos, err
}

func (r *PromotionRepositoryImpl) Update(promotion *models.Promotion) error {
	return r.db.Save(promotion).Error
}

func (r *PromotionRepositoryImpl) Delete(promotion *models.Promotion) error {
	return r.db.Delete(promotion).Error
}

func (r *PromotionRepositoryImpl) CreateRedemption(redemption *models.PromotionRedemption) error {
	return r.db.Create(redemption).Error
}

func (r *PromotionRepositoryImpl) IncrementRedemptionCount(promotionID uint) error {
	return r.db.Model(&models.Promotion{}).Where("id = ?", promotionID).
		UpdateColumn("current_redemptions", gorm.Expr("current_redemptions + 1")).Error
}

func (r *PromotionRepositoryImpl) FindRedemptionByReservation(reservationID uint) (*models.PromotionRedemption, error) {
	var redemption models.PromotionRedemption
	err := r.db.Where("reservation_id = ?", reservationID).First(&redemption).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &redemption, nil
}
