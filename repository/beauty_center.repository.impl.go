package repository

import (
	"time"

	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

// ratingSelect is the shared SELECT fragment appended to every beauty-center
// listing/detail query: the base columns plus the avg_rating and review_count
// subqueries against reviews.
const ratingSelect = "beauty_centers.*, " +
	"COALESCE((SELECT CAST(AVG(rating) AS numeric(10,2)) FROM reviews WHERE beauty_center_id = beauty_centers.id), 0) as avg_rating, " +
	"COALESCE((SELECT COUNT(*) FROM reviews WHERE beauty_center_id = beauty_centers.id), 0) as review_count"

// withAverageRating is a GORM scope that selects the base beauty_centers columns
// plus the rating subqueries, replacing the duplicated inline expressions. A
// trailing SELECT expression with bind arguments (e.g. the distance calculation
// in FindNearby) is appended verbatim after the rating subqueries.
func withAverageRating(extraSelect string, args ...interface{}) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		selects := ratingSelect
		if extraSelect != "" {
			selects += ", " + extraSelect
		}
		return db.Select(selects, args...)
	}
}

type BeautyCenterRepositoryImpl struct {
	db *gorm.DB
}

func NewBeautyCenterRepositoryImpl(db *gorm.DB) BeautyCenterRepository {
	return &BeautyCenterRepositoryImpl{db: db}
}

// Create implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) Create(center *models.BeautyCenter) error {
	return r.db.Create(center).Error
}

// Update implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) Update(center *models.BeautyCenter) error {
	return r.db.Save(center).Error
}

// Delete implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.BeautyCenter{}, id).Error
}

// FindByID implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) FindByID(id uint) (*models.BeautyCenter, error) {
	var center models.BeautyCenter
	err := r.db.
		Scopes(withAverageRating("")).
		Preload("SocialMedia").
		Preload("WorkingHour").
		Preload("BeautyCenterImage").
		First(&center, id).Error
	if err != nil {
		return nil, err
	}
	return &center, nil
} // FindAll implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) FindAll(page, limit int) ([]models.BeautyCenter, int64, error) {
	var centers []models.BeautyCenter
	var total int64

	r.db.Model(&models.BeautyCenter{}).Count(&total)

	offset := (page - 1) * limit
	err := r.db.
		Scopes(withAverageRating("")).
		Preload("BeautyCenterImage").
		Offset(offset).Limit(limit).
		Find(&centers).Error
	return centers, total, err
}

func (r *BeautyCenterRepositoryImpl) FindAllActiveForClients() ([]models.BeautyCenter, error) {
	var centers []models.BeautyCenter
	err := r.db.Where("is_active = ?", true).Find(&centers).Error
	if err != nil {
		return nil, err
	}
	return centers, nil
}

// ReplaceSchedules implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) ReplaceSchedules(centerID uint, schedules []models.WorkingHour) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("beauty_center_id = ?", centerID).Delete(&models.WorkingHour{}).Error; err != nil {
			return err
		}
		if len(schedules) > 0 {
			if err := tx.Create(schedules).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ReplaceImages implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) ReplaceImages(centerID uint, images []models.BeautyCenterImage) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("beauty_center_id = ?", centerID).Delete(&models.BeautyCenterImage{}).Error; err != nil {
			return err
		}
		if len(images) > 0 {
			if err := tx.Create(images).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// FindByEmail implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) FindByEmail(email string) (*models.BeautyCenter, error) {
	var center models.BeautyCenter
	err := r.db.Where("email = ?", email).First(&center).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &center, nil
}

// FindWorkingHours implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) FindWorkingHours(centerID uint, dayOfWeek int) ([]models.WorkingHour, error) {
	var whs []models.WorkingHour
	err := r.db.Where("beauty_center_id = ? AND day_of_week = ?", centerID, dayOfWeek).Find(&whs).Error
	if err != nil {
		return nil, err
	}
	return whs, nil
}

// FindReservationsForDate implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) FindReservationsForDate(centerID uint, date time.Time, personnelID uint) ([]models.Reservation, error) {
	var reservations []models.Reservation
	q := r.db.Where("beauty_center_id = ? AND DATE(date) = DATE(?) AND status != ?", centerID, date, "CANCELLED")
	if personnelID > 0 {
		q = q.Where("personnel_id = ?", personnelID)
	}
	err := q.Find(&reservations).Error
	return reservations, err
}

// Search implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) Search(query string, page, limit int) ([]models.BeautyCenter, int64, error) {
	var centers []models.BeautyCenter
	var total int64

	offset := (page - 1) * limit
	searchPattern := "%" + query + "%"

	base := r.db.Model(&models.BeautyCenter{}).
		Where("name ILIKE ? OR address ILIKE ? OR description ILIKE ?", searchPattern, searchPattern, searchPattern)

	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := base.
		Scopes(withAverageRating("")).
		Preload("BeautyCenterImage").
		Offset(offset).Limit(limit).
		Find(&centers).Error

	return centers, total, err
}

// FindTopRated implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) FindTopRated(limit int) ([]models.BeautyCenter, error) {
	var centers []models.BeautyCenter

	err := r.db.
		Scopes(withAverageRating("")).
		Preload("BeautyCenterImage").
		Order("avg_rating DESC, review_count DESC").
		Limit(limit).
		Find(&centers).Error

	return centers, err
}

// FindNearby implements BeautyCenterRepository
func (r *BeautyCenterRepositoryImpl) FindNearby(latitude, longitude float64, limit int) ([]models.BeautyCenter, error) {
	var centers []models.BeautyCenter

	err := r.db.
		Scopes(withAverageRating(
			"(6371 * acos(cos(radians(?)) * cos(radians(latitude)) * cos(radians(longitude) - radians(?)) + sin(radians(?)) * sin(radians(latitude)))) AS distance",
			latitude, longitude, latitude)).
		Preload("BeautyCenterImage").
		Where("latitude != 0 AND longitude != 0").
		Order("distance ASC").
		Limit(limit).
		Find(&centers).Error

	return centers, err
}

func (r *BeautyCenterRepositoryImpl) FindByReferralCode(code string) (*models.BeautyCenter, error) {
	var center models.BeautyCenter
	err := r.db.Where("referral_code = ?", code).First(&center).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &center, nil
}

// HasSchedules reports whether at least one working hour exists (profile
// completeness, feat-token-missions D4).
func (r *BeautyCenterRepositoryImpl) HasSchedules(centerID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.WorkingHour{}).
		Where("beauty_center_id = ?", centerID).
		Count(&count).Error
	return count > 0, err
}

// HasImages reports whether at least one image row exists.
func (r *BeautyCenterRepositoryImpl) HasImages(centerID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.BeautyCenterImage{}).
		Where("beauty_center_id = ?", centerID).
		Count(&count).Error
	return count > 0, err
}

// VerifyCenter stamps verified_at on the first verification (NULL → set) and
// resolves the referrer from an active referral row in the same transaction.
// Already-verified centers return transitioned=false without touching data;
// a center without a referral row returns transitioned=true with nil referrer
// (verification itself is platform-ops driven — feat-token-missions D5).
func (r *BeautyCenterRepositoryImpl) VerifyCenter(centerID uint) (bool, *uint, error) {
	var referrerID *uint
	err := r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.BeautyCenter{}).
			Where("id = ? AND verified_at IS NULL", centerID).
			Update("verified_at", time.Now().UTC())
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // already verified — idempotent no-op
		}

		var referral models.Referral
		err := tx.Where("referred_id = ? AND deleted_at IS NULL", centerID).
			First(&referral).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil // verified without referral — nothing to attribute
			}
			return err
		}
		id := referral.ReferrerID
		referrerID = &id
		return nil
	})
	if err != nil {
		return false, nil, err
	}
	transitioned := referrerID != nil
	// A verified-without-referral center also "transitioned"; re-derive by
	// checking the column so callers can distinguish cleanly.
	if !transitioned {
		var center models.BeautyCenter
		if err := r.db.Select("verified_at").First(&center, centerID).Error; err == nil && center.VerifiedAt != nil {
			transitioned = true
		}
	}
	return transitioned, referrerID, nil
}
