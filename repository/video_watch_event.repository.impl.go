package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type VideoWatchEventRepositoryImpl struct {
	Db *gorm.DB
}

func NewVideoWatchEventRepositoryImpl(Db *gorm.DB) VideoWatchEventRepository {
	return &VideoWatchEventRepositoryImpl{Db: Db}
}

func (r *VideoWatchEventRepositoryImpl) Create(event *models.VideoWatchEvent) error {
	return r.Db.Create(event).Error
}

func (r *VideoWatchEventRepositoryImpl) FindByClientID(clientID uint) ([]models.VideoWatchEvent, error) {
	var events []models.VideoWatchEvent
	err := r.Db.Where("client_id = ?", clientID).Order("created_at DESC").Find(&events).Error
	return events, err
}

func (r *VideoWatchEventRepositoryImpl) FindRecentByClientID(clientID uint, limit int) ([]models.VideoWatchEvent, error) {
	var events []models.VideoWatchEvent
	err := r.Db.Where("client_id = ?", clientID).Order("created_at DESC").Limit(limit).Find(&events).Error
	return events, err
}
