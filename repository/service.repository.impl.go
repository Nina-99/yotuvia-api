package repository

import (
	"errors"

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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("service not found")
		}
		return nil, err
	}
	return &service, nil
}

func (r *ServiceRepositoryImpl) FindByIDs(ids []uint) ([]models.Service, error) {
	var services []models.Service
	err := r.db.Where("id IN ?", ids).Find(&services).Error
	return services, err
}

func (r *ServiceRepositoryImpl) FindAll(beautyCenterID uint, categoryID uint, isActive *bool, page, limit int) ([]models.Service, int64, error) {
	var services []models.Service
	var total int64

	// Build conditions once, apply to both count and fetch queries
	type condition struct {
		query string
		args  []interface{}
	}
	var conditions []condition

	if beautyCenterID != 0 {
		conditions = append(conditions, condition{
			query: "beauty_center_id = ?",
			args:  []interface{}{beautyCenterID},
		})
	}
	if categoryID != 0 {
		conditions = append(conditions, condition{
			query: "category_id = ?",
			args:  []interface{}{categoryID},
		})
	}
	if isActive != nil {
		conditions = append(conditions, condition{
			query: "is_active = ?",
			args:  []interface{}{*isActive},
		})
	}

	// Count query
	countQuery := r.db.Model(&models.Service{})
	for _, c := range conditions {
		countQuery = countQuery.Where(c.query, c.args...)
	}
	countQuery.Count(&total)

	if total == 0 {
		return []models.Service{}, 0, nil
	}

	// Fetch query
	fetchQuery := r.db.Preload("Category")
	for _, c := range conditions {
		fetchQuery = fetchQuery.Where(c.query, c.args...)
	}

	offset := (page - 1) * limit
	err := fetchQuery.Offset(offset).Limit(limit).Find(&services).Error
	return services, total, err
}
