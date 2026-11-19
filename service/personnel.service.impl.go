package service

import (
	"context"
	"errors"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type PersonnelServiceImpl struct {
	repo        repository.PersonnelRepository
	serviceRepo repository.ServiceRepository
	sessionRepo repository.UserSessionRepository
	Validate    *validator.Validate
}

func NewPersonnelServiceImpl(repo repository.PersonnelRepository, serviceRepo repository.ServiceRepository, sessionRepo repository.UserSessionRepository, validate *validator.Validate) PersonnelService {
	return &PersonnelServiceImpl{repo: repo, serviceRepo: serviceRepo, sessionRepo: sessionRepo, Validate: validate}
}

// revokeSessions kills every live session of a staff member. Called when the
// staff is deactivated or deleted so an inactive account cannot keep using a
// token minted while it was active.
func (s *PersonnelServiceImpl) revokeSessions(personnelID uint) {
	if s.sessionRepo == nil {
		return
	}
	if err := s.sessionRepo.RevokeAllForUser(context.Background(), personnelID, "personnel"); err != nil {
		utils.Log.Errorf("failed to revoke sessions for personnel %d: %v", personnelID, err)
	}
}

// Update implements PersonnelService
// Update edits a staff member that belongs to the caller's centre.
//
// The ownership check is the whole point: `beauty_center` is a self-registrable
// role, so without it any centre (or attacker who just created one) could edit
// a competitor's staff by enumerating ids. The password is NOT part of this
// request any more — an admin setting a staff member's password is an account
// takeover primitive, and password changes belong to the self-service path
// (UpdateMe) or to a dedicated, re-authenticated flow.
func (s *PersonnelServiceImpl) Update(id uint, actorCenterID uint, req request.UpdatePersonnelRequest) (response.PersonnelResponse, error) {
	if actorCenterID == 0 {
		return response.PersonnelResponse{}, apperrors.ErrForbidden
	}
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
	if personnel.BeautyCenterID != actorCenterID {
		return response.PersonnelResponse{}, apperrors.ErrForbidden
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
	// No password handling here on purpose: UpdatePersonnelRequest no longer
	// carries one. See the doc comment above.
	wasActive := personnel.IsActive
	personnel.IsActive = req.IsActive

	// Sync images
	if len(req.Images) > 0 {
		s.applyImages(personnel, req.Images)
	}

	if err := s.repo.Update(personnel); err != nil {
		return response.PersonnelResponse{}, err
	}
	if wasActive && !personnel.IsActive {
		s.revokeSessions(personnel.ID)
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
// Delete removes a staff member of the caller's centre.
func (s *PersonnelServiceImpl) Delete(id uint, actorCenterID uint) error {
	if actorCenterID == 0 {
		return apperrors.ErrForbidden
	}
	personnel, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrNotFound
		}
		return err
	}
	if personnel.BeautyCenterID != actorCenterID {
		return apperrors.ErrForbidden
	}
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	s.revokeSessions(id)
	return nil
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

// FindAll implements PersonnelService. The optional serviceIDs filter powers the
// booking picker; the capability rule itself lives in the repository query,
// which mirrors service.PersonnelCoversServices. Requested ids are deduped so a
// repeated id cannot inflate the "covers ALL" comparison, and any id that does
// not resolve to a real service is invalid input (400).
func (s *PersonnelServiceImpl) FindAll(beautyCenterID uint, serviceIDs []uint, page, limit int) ([]response.PersonnelResponse, int64, error) {
	unique := dedupeIDs(serviceIDs)
	if len(unique) > 0 && s.serviceRepo != nil {
		services, err := s.serviceRepo.FindByIDs(unique)
		if err != nil {
			return nil, 0, err
		}
		if len(services) != len(unique) {
			return nil, 0, apperrors.ErrBadRequest // unknown service id
		}
	}

	list, total, err := s.repo.FindAll(beautyCenterID, unique, page, limit)
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
