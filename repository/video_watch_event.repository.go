package repository

import "github.com/Nina-99/yottu-api/models"

type VideoWatchEventRepository interface {
	Create(event *models.VideoWatchEvent) error
	FindByClientID(clientID uint) ([]models.VideoWatchEvent, error)
	FindRecentByClientID(clientID uint, limit int) ([]models.VideoWatchEvent, error)
}
