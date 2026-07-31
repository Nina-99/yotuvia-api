package service

import (
	"mime/multipart"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type BeautyCenterImageService interface {
	Create(req request.BeautyCenterImageRequest) (response.BeautyCenterImageResp, error)
	Upload(beautyCenterID uint, file *multipart.FileHeader, isCover bool, isProfile bool) (response.BeautyCenterImageResp, error)
	Update(id uint, req request.UpdateBeautyCenterImageRequest) (response.BeautyCenterImageResp, error)
	Reorder(centerID uint, req request.ReorderImagesRequest) error
	GetByID(id uint) (response.BeautyCenterImageResp, error)
	GetByBeautyCenterID(beautyCenterID uint) ([]response.BeautyCenterImageResp, error)
	Delete(id uint) error
}
