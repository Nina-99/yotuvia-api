package service

import (
	"mime/multipart"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
)

type BeautyCenterImageService interface {
	Create(req request.BeautyCenterImageRequest) (response.BeautyCenterImageResp, error)
	Upload(beautyCenterID uint, file *multipart.FileHeader, isCover bool, isProfile bool) (response.BeautyCenterImageResp, error)
	// Update/Delete take the caller's centerID claim and reject a row that does
	// not belong to it (apperrors.ErrForbidden), closing the gallery IDOR.
	Update(id, actorCenterID uint, req request.UpdateBeautyCenterImageRequest) (response.BeautyCenterImageResp, error)
	Reorder(centerID uint, req request.ReorderImagesRequest) error
	GetByID(id uint) (response.BeautyCenterImageResp, error)
	GetByBeautyCenterID(beautyCenterID uint) ([]response.BeautyCenterImageResp, error)
	Delete(id, actorCenterID uint) error
}
