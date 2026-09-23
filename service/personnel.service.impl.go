package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type PersonnelServiceImpl struct {
	repo     repository.PersonnelRepository
	Validate *validator.Validate
}

func NewPersonnelServiceImpl(repo repository.PersonnelRepository, validate *validator.Validate) PersonnelService {
	return &PersonnelServiceImpl{repo: repo, Validate: validate}
}

// Update implements PersonnelService
func (s *PersonnelServiceImpl) Update(id uint, req request.UpdatePersonnelRequest) (response.PersonnelResponse, error) {
	if err := s.Validate.Struct(req); err != nil {
		return response.PersonnelResponse{}, apperrors.ErrValidation
	}
	personnel, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.PersonnelResponse{}, apperrors.ErrNotFound
		}
		return response.PersonnelResponse{}, err
	}

	// Update fields if provided
	if req.FirstName != "" {
		personnel.FirstName = req.FirstName
	}
	if req.LastName != "" {
		personnel.LastName = req.LastName
	}
	if req.Email != "" && req.Email != personnel.Email {
		// Check uniqueness
		existing, _ := s.repo.FindByEmail(req.Email)
		if existing != nil {
			return response.PersonnelResponse{}, apperrors.ErrConflict
		}
		personnel.Email = req.Email
	}
	if req.Phone != "" {
		personnel.Phone = req.Phone
	}
	if req.Password != "" {
		hashed, err := utils.HashPassword(req.Password)
		if err != nil {
			return response.PersonnelResponse{}, err
		}
		personnel.Password = hashed
	}
	personnel.IsActive = req.IsActive

	// Sync images
	if len(req.Images) > 0 {
		s.applyImages(personnel, req.Images)
	}

	if err := s.repo.Update(personnel); err != nil {
		return response.PersonnelResponse{}, err
	}

	return s.mapToResponse(*personnel), nil
}

// applyImages replaces the personnel's image set from a request DTO. Shared by
// the admin Update and self-service UpdateMe paths.
func (s *PersonnelServiceImpl) applyImages(personnel *models.Personnel, images []request.PersonnelImageRequest) {
	var newImages []models.PersonnelImage
	for _, imgReq := range images {
		newImages = append(newImages, models.PersonnelImage{
			PersonnelID: personnel.ID,
			ImageUrl:    imgReq.ImageUrl,
			IsCover:     imgReq.IsCover,
			IsProfile:   imgReq.IsProfile,
			Position:    imgReq.Position,
		})
	}
	personnel.PersonnelImage = newImages
}

// UpdateMe implements PersonnelService. It reuses Update's bcrypt and images
// sync internals but only touches fields the self-service DTO exposes — email,
// is_active, and beauty_center_id are never read or written here, so a staff
// member's login identity and center affiliation cannot change through
// PATCH /personnel/me.
func (s *PersonnelServiceImpl) UpdateMe(id uint, req request.UpdateMePersonnelRequest) (response.PersonnelResponse, error) {
	if err := s.Validate.Struct(req); err != nil {
		return response.PersonnelResponse{}, apperrors.ErrValidation
	}
	personnel, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.PersonnelResponse{}, apperrors.ErrNotFound
		}
		return response.PersonnelResponse{}, err
	}

	if req.FirstName != "" {
		personnel.FirstName = req.FirstName
	}
	if req.LastName != "" {
		personnel.LastName = req.LastName
	}
	if req.Phone != "" {
		personnel.Phone = req.Phone
	}
	if req.Password != "" {
		hashed, err := utils.HashPassword(req.Password)
		if err != nil {
			return response.PersonnelResponse{}, err
		}
		personnel.Password = hashed
	}

	if len(req.Images) > 0 {
		s.applyImages(personnel, req.Images)
	}

	if err := s.repo.Update(personnel); err != nil {
		return response.PersonnelResponse{}, err
	}

	return s.mapToResponse(*personnel), nil
}

// Delete implements PersonnelService
func (s *PersonnelServiceImpl) Delete(id uint) error {
	if _, err := s.repo.FindByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrNotFound
		}
		return err
	}
	return s.repo.Delete(id)
}

// FindByID implements PersonnelService
func (s *PersonnelServiceImpl) FindByID(id uint) (response.PersonnelResponse, error) {
	personnel, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.PersonnelResponse{}, apperrors.ErrNotFound
		}
		return response.PersonnelResponse{}, err
	}
	return s.mapToResponse(*personnel), nil
}

// FindAll implements PersonnelService
func (s *PersonnelServiceImpl) FindAll(beautyCenterID uint, page, limit int) ([]response.PersonnelResponse, int64, error) {
	list, total, err := s.repo.FindAll(beautyCenterID, page, limit)
	if err != nil {
		return nil, total, err
	}

	var res []response.PersonnelResponse
	for _, p := range list {
		res = append(res, s.mapToResponse(p))
	}
	return res, total, nil
}

func (s *PersonnelServiceImpl) mapToResponse(p models.Personnel) response.PersonnelResponse {
	var images []response.PersonnelImageResponse
	for _, img := range p.PersonnelImage {
		images = append(images, response.PersonnelImageResponse{
			ID:        img.ID,
			ImageUrl:  img.ImageUrl,
			IsCover:   img.IsCover,
			IsProfile: img.IsProfile,
			Position:  img.Position,
			CreatedAt: img.CreatedAt,
		})
	}

	return response.PersonnelResponse{
		ID:             p.ID,
		BeautyCenterID: p.BeautyCenterID,
		FirstName:      p.FirstName,
		LastName:       p.LastName,
		Email:          p.Email,
		Phone:          p.Phone,
		IsActive:       p.IsActive,
		Images:         images,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}
