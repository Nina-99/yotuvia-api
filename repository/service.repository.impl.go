package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type ServiceRepositoryImpl struct {
	db *gorm.DB
}

func NewServiceRepositoryImpl(db *gorm.DB) ServiceRepository {
	return &ServiceRepositoryImpl{db: db}
}

func (r *ServiceRepositoryImpl) Create(service *models.Service) error {
	return r.db.Create(service).Error
}

func (r *ServiceRepositoryImpl) Update(service *models.Service) error {
	return r.db.Save(service).Error
}

func (r *ServiceRepositoryImpl) Delete(service *models.Service) error {
	return r.db.Delete(service).Error
}

func (r *ServiceRepositoryImpl) FindById(serviceID uint) (*models.Service, error) {
	var service models.Service
	err := r.db.Preload("Category").First(&service, serviceID).Error
	if err != nil {
		return nil, err
	}
	return &service, nil
}

func (r *ServiceRepositoryImpl) FindAll(page, limit int) ([]models.Service, int64, error) {
	var services []models.Service
	var total int64

	r.db.Model(&models.Service{}).Count(&total)

	offset := (page - 1) * limit
	err := r.db.Preload("Category").Offset(offset).Limit(limit).Find(&services).Error
	if err != nil {
		return nil, total, err
	}
	return services, total, nil
}
