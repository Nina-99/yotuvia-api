package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type BeautyCenterImageService interface {
	Create(req request.BeautyCenterImageRequest) (response.BeautyCenterImageResp, error)
	GetByID(id uint) (response.BeautyCenterImageResp, error)
	GetByBeautyCenterID(beautyCenterID uint) ([]response.BeautyCenterImageResp, error)
	Delete(id uint) error
}
