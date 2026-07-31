package repository

import (
	"errors"

	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type PersonnelServiceRepositoryImpl struct {
	db *gorm.DB
}

func NewPersonnelServiceRepositoryImpl(db *gorm.DB) PersonnelServiceRepository {
	return &PersonnelServiceRepositoryImpl{db: db}
}

func (r *PersonnelServiceRepositoryImpl) Create(ps *models.PersonnelService) error {
	return r.db.Create(ps).Error
}

func (r *PersonnelServiceRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.PersonnelService{}, id).Error
}

func (r *PersonnelServiceRepositoryImpl) FindByID(id uint) (*models.PersonnelService, error) {
	var ps models.PersonnelService
	err := r.db.Preload("Service").Preload("Service.Category").
		First(&ps, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("personnel service not found")
		}
		return nil, err
	}
	return &ps, nil
}

func (r *PersonnelServiceRepositoryImpl) FindByPersonnelID(personnelID uint) ([]models.PersonnelService, error) {
	var list []models.PersonnelService
	err := r.db.
		Preload("Service").Preload("Service.Category").
		Where("personnel_id = ?", personnelID).
		Find(&list).Error
	return list, err
}

func (r *PersonnelServiceRepositoryImpl) FindByServiceID(serviceID uint) ([]models.PersonnelService, error) {
	var list []models.PersonnelService
	err := r.db.
		Preload("Service").Preload("Service.Category").
		Preload("Personnel").
		Where("service_id = ?", serviceID).
		Find(&list).Error
	return list, err
}

func (r *PersonnelServiceRepositoryImpl) FindByPersonnelAndService(personnelID, serviceID uint) (*models.PersonnelService, error) {
	var ps models.PersonnelService
	err := r.db.Where("personnel_id = ? AND service_id = ?", personnelID, serviceID).
		First(&ps).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ps, nil
}
