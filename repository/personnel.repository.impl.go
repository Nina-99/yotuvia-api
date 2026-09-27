package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type PersonnelRepositoryImpl struct {
	db *gorm.DB
}

func NewPersonnelRepositoryImpl(db *gorm.DB) PersonnelRepository {
	return &PersonnelRepositoryImpl{db: db}
}

// Create implements PersonnelRepository
func (r *PersonnelRepositoryImpl) Create(personnel *models.Personnel) error {
	return r.db.Create(personnel).Error
}

// Update implements PersonnelRepository
func (r *PersonnelRepositoryImpl) Update(personnel *models.Personnel) error {
	return r.db.Save(personnel).Error
}

// Delete implements PersonnelRepository
func (r *PersonnelRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.Personnel{}, id).Error
}

// FindByID implements PersonnelRepository
func (r *PersonnelRepositoryImpl) FindByID(id uint) (*models.Personnel, error) {
	var personnel models.Personnel
	err := r.db.Preload("PersonnelImage").Preload("BeautyCenter").First(&personnel, id).Error
	if err != nil {
		return nil, err
	}
	return &personnel, nil
}

// FindAll implements PersonnelRepository. The optional serviceIDs filter is the
// read side of the personnel↔service capability rule and must agree with the
// booking write validation (service.PersonnelCoversServices):
//
//	no capability rows         → unrestricted, always kept
//	rows covering ALL services → kept
//	rows missing any service   → dropped
//
// It is enforced in SQL so COUNT and pagination stay correct. The rule is
// expressed as two correlated subqueries: a NOT EXISTS for "has no rows" and a
// COUNT(DISTINCT …) for "covers every requested service".
func (r *PersonnelRepositoryImpl) FindAll(beautyCenterID uint, serviceIDs []uint, page, limit int) ([]models.Personnel, int64, error) {
	base := func() *gorm.DB {
		q := r.db.Model(&models.Personnel{})
		if beautyCenterID != 0 {
			q = q.Where("personnels.beauty_center_id = ?", beautyCenterID)
		}
		if len(serviceIDs) > 0 {
			hasNoRows := r.db.Model(&models.PersonnelService{}).
				Select("1").
				Where("personnel_services.personnel_id = personnels.id")
			covered := r.db.Model(&models.PersonnelService{}).
				Select("COUNT(DISTINCT personnel_services.service_id)").
				Where("personnel_services.personnel_id = personnels.id").
				Where("personnel_services.service_id IN ?", serviceIDs)
			q = q.Where("NOT EXISTS (?) OR (?) = ?", hasNoRows, covered, len(serviceIDs))
		}
		return q
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var personnel []models.Personnel
	offset := (page - 1) * limit
	err := base().
		Preload("PersonnelImage").Preload("BeautyCenter").
		Order("personnels.id").
		Offset(offset).Limit(limit).
		Find(&personnel).Error
	return personnel, total, err
}

// FindByEmail implements PersonnelRepository
func (r *PersonnelRepositoryImpl) FindByEmail(email string) (*models.Personnel, error) {
	var personnel models.Personnel
	err := r.db.Where("email = ?", email).First(&personnel).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &personnel, nil
}
