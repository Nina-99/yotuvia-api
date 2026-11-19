package service

import (
	"fmt"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
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

// Create adds a service to the CALLER'S centre.
//
// actorCenterID comes from the JWT `center_id` claim and overwrites whatever the
// body says: before this, `beauty_center_id` in the request decided the owner,
// so any authenticated account (a public `client` included) could add services
// to any centre. The category must belong to the same centre, otherwise a centre
// could hang its services off another centre's catalog.
func (s *ServiceServiceImpl) Create(actorCenterID uint, req request.CreateServiceRequest) (uint, error) {
	if actorCenterID == 0 {
		return 0, apperrors.ErrForbidden
	}
	req.BeautyCenterID = actorCenterID

	if err := s.Validate.Struct(req); err != nil {
		return 0, fmt.Errorf("%w: %v", apperrors.ErrValidation, err)
	}

	category, err := s.CategoryRepository.FindById(req.CategoryID)
	if err != nil || category == nil {
		return 0, apperrors.ErrNotFound
	}
	if category.BeautyCenterID != actorCenterID {
		return 0, apperrors.ErrForbidden
	}

	service := models.Service{
		BeautyCenterID:  actorCenterID,
		CategoryID:      req.CategoryID,
		Name:            req.Name,
		Description:     req.Description,
		NameI18n:        utils.NormalizeI18n(req.NameI18n),
		DescriptionI18n: utils.NormalizeI18n(req.DescriptionI18n),
		Duration:        req.Duration,
		Price:           req.Price,
		ImageURL:        req.ImageURL,
		IsActive:        req.IsActive,
		TreatmentID:     req.TreatmentID,
	}

	if err := s.ServiceRepository.Create(&service); err != nil {
		return 0, err
	}

	return service.ID, nil
}

// Update edits a service owned by the caller's centre.
//
// Ownership is checked against actorCenterID before anything is written, and the
// service can NEVER move to another centre: `beauty_center_id` in the body is
// ignored. That field used to be applied verbatim, which let a caller re-parent
// another centre's service into its own catalog.
func (s *ServiceServiceImpl) Update(actorCenterID uint, serviceID uint, req request.UpdateServiceRequest) error {
	if actorCenterID == 0 {
		return apperrors.ErrForbidden
	}
	if err := s.Validate.Struct(req); err != nil {
		return fmt.Errorf("%w: %v", apperrors.ErrValidation, err)
	}

	service, err := s.ServiceRepository.FindById(serviceID)
	if err != nil || service == nil {
		return apperrors.ErrNotFound
	}
	if service.BeautyCenterID != actorCenterID {
		return apperrors.ErrForbidden
	}

	if req.CategoryID != 0 {
		category, err := s.CategoryRepository.FindById(req.CategoryID)
		if err != nil || category == nil {
			return apperrors.ErrNotFound
		}
		if category.BeautyCenterID != actorCenterID {
			return apperrors.ErrForbidden
		}
		service.CategoryID = req.CategoryID
	}

	if req.Name != "" {
		service.Name = req.Name
	}
	if req.Description != "" {
		service.Description = req.Description
	}
	if req.NameI18n != nil {
		service.NameI18n = utils.NormalizeI18n(req.NameI18n)
	}
	if req.DescriptionI18n != nil {
		service.DescriptionI18n = utils.NormalizeI18n(req.DescriptionI18n)
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

// Delete removes a service owned by the caller's centre. A foreign id is
// ErrForbidden rather than ErrNotFound so a probe cannot distinguish "not yours"
// from "does not exist" by status code... except that both are rejected before
// any write either way; 403 is kept because the caller IS authenticated.
func (s *ServiceServiceImpl) Delete(actorCenterID uint, serviceID uint) error {
	if actorCenterID == 0 {
		return apperrors.ErrForbidden
	}
	service, err := s.ServiceRepository.FindById(serviceID)
	if err != nil || service == nil {
		return apperrors.ErrNotFound
	}
	if service.BeautyCenterID != actorCenterID {
		return apperrors.ErrForbidden
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
		return nil, apperrors.ErrNotFound
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
