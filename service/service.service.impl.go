package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
)

type ServiceServiceImpl struct {
	ServiceRepository  repository.ServiceRepository
	CategoryRepository repository.CategoryRepository
	Validate           *validator.Validate
}

func NewServiceServiceImpl(
	serviceRepository repository.ServiceRepository,
	categoryRepository repository.CategoryRepository,
	validate *validator.Validate,
) ServiceService {
	return &ServiceServiceImpl{
		ServiceRepository:  serviceRepository,
		CategoryRepository: categoryRepository,
		Validate:           validate,
	}
}

func (s *ServiceServiceImpl) Create(req request.CreateServiceRequest) (uint, error) {
	if err := s.Validate.Struct(req); err != nil {
		return 0, err
	}

	// Validate category existence
	category, err := s.CategoryRepository.FindById(req.CategoryID)
	if err != nil || category == nil {
		return 0, errors.New("category not found")
	}

	service := models.Service{
		BeautyCenterID: req.BeautyCenterID,
		CategoryID:     req.CategoryID,
		Name:           req.Name,
		Description:    req.Description,
		Duration:       req.Duration,
		Price:          req.Price,
		ImageURL:       req.ImageURL,
		IsActive:       req.IsActive,
		TreatmentID:    req.TreatmentID,
	}

	if err := s.ServiceRepository.Create(&service); err != nil {
		return 0, err
	}

	return service.ID, nil
}

func (s *ServiceServiceImpl) Update(serviceID uint, req request.UpdateServiceRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}

	service, err := s.ServiceRepository.FindById(serviceID)
	if err != nil || service == nil {
		return errors.New("service not found")
	}

	if req.CategoryID != 0 {
		category, err := s.CategoryRepository.FindById(req.CategoryID)
		if err != nil || category == nil {
			return errors.New("category not found")
		}
		service.CategoryID = req.CategoryID
	}

	if req.BeautyCenterID != 0 {
		service.BeautyCenterID = req.BeautyCenterID
	}
	if req.Name != "" {
		service.Name = req.Name
	}
	if req.Description != "" {
		service.Description = req.Description
	}
	if req.Duration != 0 {
		service.Duration = req.Duration
	}
	if req.Price != 0 {
		service.Price = req.Price
	}
	if req.ImageURL != "" {
		service.ImageURL = req.ImageURL
	}
	if req.IsActive != nil {
		service.IsActive = *req.IsActive
	}
	if req.TreatmentID != nil {
		if *req.TreatmentID == 0 {
			// 0 means "unlink": clear the catalog link. The repository uses
			// Save, which writes every column, so a nil pointer persists NULL.
			service.TreatmentID = nil
		} else {
			service.TreatmentID = req.TreatmentID
		}
	}

	return s.ServiceRepository.Update(service)
}

func (s *ServiceServiceImpl) Delete(serviceID uint) error {
	service, err := s.ServiceRepository.FindById(serviceID)
	if err != nil || service == nil {
		return errors.New("service not found")
	}

	return s.ServiceRepository.Delete(service)
}

func (s *ServiceServiceImpl) FindAll(beautyCenterID uint, categoryID uint, isActive *bool, page, limit int) ([]models.Service, int64, error) {
	return s.ServiceRepository.FindAll(beautyCenterID, categoryID, isActive, page, limit)
}

func (s *ServiceServiceImpl) FindById(serviceID uint) (*models.Service, error) {
	return s.ServiceRepository.FindById(serviceID)
}

func (s *ServiceServiceImpl) GetServiceTemplates(categoryID uint, lang string) ([]ServiceTemplate, error) {
	category, err := s.CategoryRepository.FindById(categoryID)
	if err != nil || category == nil {
		return nil, errors.New("category not found")
	}

	suggestions := utils.GetServiceTemplates(category.Name, lang)
	if suggestions == nil {
		return nil, nil
	}

	result := make([]ServiceTemplate, len(suggestions))
	for i, t := range suggestions {
		result[i] = ServiceTemplate{
			Name:        t.Name,
			Description: t.Description,
		}
	}
	return result, nil
}
