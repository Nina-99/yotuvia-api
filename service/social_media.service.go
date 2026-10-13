package service

import (
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
)

type SocialMediaService interface {
	Create(req request.CreateSocialMediaRequest) error
	// Update/Delete take the caller's centerID claim and reject a row that does
	// not belong to it (apperrors.ErrForbidden), closing the social-link IDOR.
	Update(id, actorCenterID uint, req request.UpdateSocialMediaRequest) error
	Delete(id, actorCenterID uint) error
	FindByID(id uint) (*response.SocialMediaResponse, error)
	FindByBeautyCenterID(beautyCenterID uint) (*response.SocialMediaResponse, error)
}
