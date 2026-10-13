package service

import (
	"errors"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/go-playground/validator/v10"
)

type PersonnelServiceServiceImpl struct {
	Repo               repository.PersonnelServiceRepository
	PersonnelRepo      repository.PersonnelRepository
	ServiceRepo        repository.ServiceRepository
	Validate           *validator.Validate
}

func NewPersonnelServiceServiceImpl(
	repo repository.PersonnelServiceRepository,
	personnelRepo repository.PersonnelRepository,
	serviceRepo repository.ServiceRepository,
	validate *validator.Validate,
) PersonnelServiceService {
	return &PersonnelServiceServiceImpl{
		Repo:          repo,
		PersonnelRepo: personnelRepo,
		ServiceRepo:   serviceRepo,
		Validate:      validate,
	}
}

func (s *PersonnelServiceServiceImpl) Create(req request.CreatePersonnelServiceRequest, actorCenterID uint) (response.PersonnelServiceResponse, error) {
	if err := s.Validate.Struct(req); err != nil {
		return response.PersonnelServiceResponse{}, apperrors.ErrValidation
	}

	// Validate personnel exists and belongs to the acting centre. A foreign
	// row is a permission failure (403), not a 404: the row exists but is not
	// the actor's to manage.
	personnel, err := s.PersonnelRepo.FindByID(req.PersonnelID)
	if err != nil {
		return response.PersonnelServiceResponse{}, apperrors.ErrNotFound
	}
	if personnel.BeautyCenterID != actorCenterID {
		return response.PersonnelServiceResponse{}, apperrors.ErrForbidden
	}

	// Validate service exists and belongs to the acting centre.
	svc, err := s.ServiceRepo.FindById(req.ServiceID)
	if err != nil {
		return response.PersonnelServiceResponse{}, apperrors.ErrNotFound
	}
	if svc.BeautyCenterID != actorCenterID {
		return response.PersonnelServiceResponse{}, apperrors.ErrForbidden
	}

	// Check duplicate
	existing, _ := s.Repo.FindByPersonnelAndService(req.PersonnelID, req.ServiceID)
	if existing != nil {
		return response.PersonnelServiceResponse{}, apperrors.ErrConflict
	}

	ps := models.PersonnelService{
		PersonnelID: req.PersonnelID,
		ServiceID:   req.ServiceID,
		Price:       req.Price,
		Duration:    req.Duration,
	}

	if err := s.Repo.Create(&ps); err != nil {
		return response.PersonnelServiceResponse{}, err
	}

	return s.mapToResponse(ps, *svc), nil
}

func (s *PersonnelServiceServiceImpl) Delete(id uint, actorCenterID uint) error {
	ps, err := s.Repo.FindByID(id)
	if err != nil {
		return apperrors.ErrNotFound
	}

	// Ownership: the assignment's personnel AND service must belong to the
	// acting centre. Checking the personnel alone would leave a row whose
	// service came from another centre deletable; both are verified.
	personnel, err := s.PersonnelRepo.FindByID(ps.PersonnelID)
	if err != nil {
		return apperrors.ErrNotFound
	}
	svc, err := s.ServiceRepo.FindById(ps.ServiceID)
	if err != nil {
		return apperrors.ErrNotFound
	}
	if personnel.BeautyCenterID != actorCenterID || svc.BeautyCenterID != actorCenterID {
		return apperrors.ErrForbidden
	}

	return s.Repo.Delete(id)
}

func (s *PersonnelServiceServiceImpl) FindByPersonnelID(personnelID uint) ([]response.PersonnelServiceResponse, error) {
	// Validate personnel exists
	_, err := s.PersonnelRepo.FindByID(personnelID)
	if err != nil {
		return nil, errors.New("personnel not found")
	}

	list, err := s.Repo.FindByPersonnelID(personnelID)
	if err != nil {
		return nil, err
	}

	var res []response.PersonnelServiceResponse
	for _, ps := range list {
		res = append(res, s.mapToResponse(ps, ps.Service))
	}
	return res, nil
}

func (s *PersonnelServiceServiceImpl) FindByServiceID(serviceID uint) ([]response.PersonnelServiceResponse, error) {
	// Validate service exists
	_, err := s.ServiceRepo.FindById(serviceID)
	if err != nil {
		return nil, errors.New("service not found")
	}

	list, err := s.Repo.FindByServiceID(serviceID)
	if err != nil {
		return nil, err
	}

	var res []response.PersonnelServiceResponse
	for _, ps := range list {
		res = append(res, s.mapToResponse(ps, ps.Service))
	}
	return res, nil
}

func (s *PersonnelServiceServiceImpl) mapToResponse(ps models.PersonnelService, svc models.Service) response.PersonnelServiceResponse {
	categoryName := ""
	if svc.Category.Name != "" {
		categoryName = svc.Category.Name
	}

	return response.PersonnelServiceResponse{
		ID:           ps.ID,
		PersonnelID:  ps.PersonnelID,
		ServiceID:    ps.ServiceID,
		Price:        ps.Price,
		Duration:     ps.Duration,
		ServiceName:  svc.Name,
		CategoryName: categoryName,
		CreatedAt:    ps.CreatedAt,
	}
}
