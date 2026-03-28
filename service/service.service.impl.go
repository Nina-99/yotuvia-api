package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
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

func (s *ServiceServiceImpl) Create(req request.CreateServiceRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}

	// Validate category existence
	category, err := s.CategoryRepository.FindById(req.CategoryID)
	if err != nil || category == nil {
		return errors.New("category not found")
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
	}

	return s.ServiceRepository.Create(&service)
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

	return s.ServiceRepository.Update(service)
}

func (s *ServiceServiceImpl) Delete(serviceID uint) error {
	service, err := s.ServiceRepository.FindById(serviceID)
	if err != nil || service == nil {
		return errors.New("service not found")
	}

	return s.ServiceRepository.Delete(service)
}

func (s *ServiceServiceImpl) FindAll() ([]models.Service, error) {
	return s.ServiceRepository.FindAll()
}

func (s *ServiceServiceImpl) FindById(serviceID uint) (*models.Service, error) {
	return s.ServiceRepository.FindById(serviceID)
}
