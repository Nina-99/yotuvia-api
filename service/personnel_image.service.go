package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"mime/multipart"
)

type PersonnelImageService interface {
	Create(req request.CreatePersonnelImageRequest) (response.PersonnelImageResponse, error)
	GetByID(id uint) (response.PersonnelImageResponse, error)
	GetByPersonnelID(personnelID uint) ([]response.PersonnelImageResponse, error)
	Delete(id uint) error
	Upload(personnelID uint, file *multipart.FileHeader, isCover bool, isProfile bool) (response.PersonnelImageResponse, error)
}
