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

type BeautyCenterServiceImpl struct {
	repo     repository.BeautyCenterRepository
	Validate *validator.Validate
}

func NewBeautyCenterServiceImpl(repo repository.BeautyCenterRepository, validate *validator.Validate) BeautyCenterService {
	return &BeautyCenterServiceImpl{
		repo:     repo,
		Validate: validate,
	}
}

// Update implements BeautyCenterService
func (s *BeautyCenterServiceImpl) Update(id uint, req request.UpdateBeautyCenterRequest) (response.BeautyCenterResponse, error) {
	if err := s.Validate.Struct(req); err != nil {
		return response.BeautyCenterResponse{}, err
	}
	// Find existing
	center, err := s.repo.FindByID(id)
	if err != nil {
		return response.BeautyCenterResponse{}, errors.New("beauty center not found")
	}

	// Update basic fields
	if req.Name != "" {
		center.Name = req.Name
	}
	if req.Address != "" {
		center.Address = req.Address
	}
	if req.Phone != "" {
		center.Phone = req.Phone
	}
	if req.Url != "" {
		center.Url = req.Url
	}
	if req.Description != "" {
		center.Description = req.Description
	}
	if req.IsActive != nil {
		center.IsActive = *req.IsActive
	}

	if req.Email != "" && req.Email != center.Email {
		existingEmail, err := s.repo.FindByEmail(req.Email)
		if err != nil {
			utils.Log.Errorf("Update failed: error checking email %s: %v", req.Email, err)
			return response.BeautyCenterResponse{}, err
		}
		if existingEmail != nil {
			utils.Log.Warnf("Update failed: email %s already exists", req.Email)
			return response.BeautyCenterResponse{}, errors.New("email already exists")
		}
		center.Email = req.Email
	}
	if req.Password != "" {
		hashed, err := utils.HashPassword(req.Password)
		if err != nil {
			utils.Log.Errorf("Update failed: failed to hash password: %v", err)
			return response.BeautyCenterResponse{}, err
		}

		center.Password = hashed
	}

	// Update images (Replacement strategy)
	if len(req.Images) > 0 {
		var newImages []models.BeautyCenterImage
		for _, imgReq := range req.Images {
			newImages = append(newImages, models.BeautyCenterImage{
				BeautyCenterID: center.ID,
				ImageUrl:       imgReq.ImageUrl,
				IsCover:        imgReq.IsCover,
				Position:       imgReq.Position,
			})
		}
		center.BeautyCenterImage = newImages
	}

	// Update schedules (Replacement strategy)
	if len(req.Schedules) > 0 {
		var newSchedules []models.WorkingHour
		for _, schedReq := range req.Schedules {
			newSchedules = append(newSchedules, models.WorkingHour{
				BeautyCenterID: center.ID,
				DayOfWeek:      schedReq.DayOfWeek,
				StartTime:      schedReq.StartTime,
				EndTime:        schedReq.EndTime,
			})
		}
		center.WorkingHour = newSchedules
	}

	// Save changes
	if err := s.repo.Update(center); err != nil {
		return response.BeautyCenterResponse{}, err
	}

	return s.mapToResponse(*center), nil
}

// Delete implements BeautyCenterService
func (s *BeautyCenterServiceImpl) Delete(id uint) error {
	return s.repo.Delete(id)
}

// FindByID implements BeautyCenterService
func (s *BeautyCenterServiceImpl) FindByID(id uint) (response.BeautyCenterResponse, error) {
	center, err := s.repo.FindByID(id)
	if err != nil {
		return response.BeautyCenterResponse{}, err
	}
	return s.mapToResponse(*center), nil
}

// FindAll implements BeautyCenterService
func (s *BeautyCenterServiceImpl) FindAll() ([]response.BeautyCenterResponse, error) {
	centers, err := s.repo.FindAll()
	if err != nil {
		return nil, err
	}

	var resps []response.BeautyCenterResponse
	for _, c := range centers {
		resps = append(resps, s.mapToResponse(c))
	}
	return resps, nil
}

// Internal mapper
func (s *BeautyCenterServiceImpl) mapToResponse(center models.BeautyCenter) response.BeautyCenterResponse {
	resp := response.BeautyCenterResponse{
		ID:          center.ID,
		Name:        center.Name,
		Address:     center.Address,
		Email:       center.Email,
		Phone:       center.Phone,
		Url:         center.Url,
		Description: center.Description,
		IsActive:    center.IsActive,
	}

	for _, img := range center.BeautyCenterImage {
		resp.Images = append(resp.Images, response.BeautyCenterImageResp{
			ID:       img.ID,
			ImageUrl: img.ImageUrl,
			IsCover:  img.IsCover,
			Position: img.Position,
		})
	}

	for _, sched := range center.WorkingHour {
		resp.Schedules = append(resp.Schedules, response.WorkingHourResp{
			ID:        sched.ID,
			DayOfWeek: sched.DayOfWeek,
			StartTime: sched.StartTime,
			EndTime:   sched.EndTime,
		})
	}

	return resp
}
