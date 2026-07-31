package service

import (
	"mime/multipart"

	"github.com/Nina-99/yottu-api/models"
)

type VideoService interface {
	Upload(centerID uint, title, description string, file *multipart.FileHeader) (*models.Video, error)
	Delete(videoID, centerID uint) error
	GetFeed(page, limit int) ([]models.Video, int64, error)
	GetByCenter(centerID uint, page, limit int) ([]models.Video, int64, error)
	IncrementViews(videoID uint) error
}
