package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type SocialMediaService interface {
	Create(req request.CreateSocialMediaRequest) error
	Update(id uint, req request.UpdateSocialMediaRequest) error
	Delete(id uint) error
	FindByID(id uint) (*response.SocialMediaResponse, error)
	FindByBeautyCenterID(beautyCenterID uint) (*response.SocialMediaResponse, error)
}
