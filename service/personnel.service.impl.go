package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
)

type PersonnelServiceImpl struct {
	repo     repository.PersonnelRepository
	Validate *validator.Validate
}

func NewPersonnelServiceImpl(repo repository.PersonnelRepository, validate *validator.Validate) PersonnelService {
	return &PersonnelServiceImpl{repo: repo, Validate: validate}
}

// Create implements PersonnelService
func (s *PersonnelServiceImpl) Create(req request.CreatePersonnelRequest) (response.PersonnelResponse, error) {
	if err := s.Validate.Struct(req); err != nil {
		return response.PersonnelResponse{}, err
	}
	// Check if email already exists
	existing, err := s.repo.FindByEmail(req.Email)
	if err != nil {
		return response.PersonnelResponse{}, err
	}
	if existing != nil {
		return response.PersonnelResponse{}, errors.New("email already in use")
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return response.PersonnelResponse{}, err
	}

	// Map images
	var images []models.PersonnelImage
	for _, imgReq := range req.Images {
		images = append(images, models.PersonnelImage{
			ImageUrl: imgReq.ImageUrl,
			IsCover:  imgReq.IsCover,
			Position: imgReq.Position,
		})
	}

	// Map to model
	personnel := models.Personnel{
		BeautyCenterID: req.BeautyCenterID,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Email:          req.Email,
		Phone:          req.Phone,
		Password:       hashedPassword,
		IsActive:       req.IsActive,
		PersonnelImage: images,
	}

	// Persist
	if err := s.repo.Create(&personnel); err != nil {
		return response.PersonnelResponse{}, err
	}

	return s.mapToResponse(personnel), nil
}

// Update implements PersonnelService
func (s *PersonnelServiceImpl) Update(id uint, req request.UpdatePersonnelRequest) (response.PersonnelResponse, error) {
	if err := s.Validate.Struct(req); err != nil {
		return response.PersonnelResponse{}, err
	}
	personnel, err := s.repo.FindByID(id)
	if err != nil {
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
			return response.PersonnelResponse{}, errors.New("email already in use")
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
		var newImages []models.PersonnelImage
		for _, imgReq := range req.Images {
			newImages = append(newImages, models.PersonnelImage{
				PersonnelID: personnel.ID,
				ImageUrl:    imgReq.ImageUrl,
				IsCover:     imgReq.IsCover,
				Position:    imgReq.Position,
			})
		}
		personnel.PersonnelImage = newImages
	}

	if err := s.repo.Update(personnel); err != nil {
		return response.PersonnelResponse{}, err
	}

	return s.mapToResponse(*personnel), nil
}

// Delete implements PersonnelService
func (s *PersonnelServiceImpl) Delete(id uint) error {
	return s.repo.Delete(id)
}

// FindByID implements PersonnelService
func (s *PersonnelServiceImpl) FindByID(id uint) (response.PersonnelResponse, error) {
	personnel, err := s.repo.FindByID(id)
	if err != nil {
		return response.PersonnelResponse{}, err
	}
	return s.mapToResponse(*personnel), nil
}

// FindAll implements PersonnelService
func (s *PersonnelServiceImpl) FindAll(beautyCenterID uint) ([]response.PersonnelResponse, error) {
	list, err := s.repo.FindAll(beautyCenterID)
	if err != nil {
		return nil, err
	}

	var res []response.PersonnelResponse
	for _, p := range list {
		res = append(res, s.mapToResponse(p))
	}
	return res, nil
}

func (s *PersonnelServiceImpl) mapToResponse(p models.Personnel) response.PersonnelResponse {
	var images []response.PersonnelImageResponse
	for _, img := range p.PersonnelImage {
		images = append(images, response.PersonnelImageResponse{
			ID:        img.ID,
			ImageUrl:  img.ImageUrl,
			IsCover:   img.IsCover,
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
