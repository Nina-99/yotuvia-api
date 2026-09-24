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
	err := r.Db.Where("client_id = ?", clientID).
		Order("created_at DESC").
		Limit(ListQueryLimit).
		Find(&events).Error
	return events, err
}

// FindByClientAndPost returns the newest watch event for a (client, post)
// pair, or gorm.ErrRecordNotFound when none exists. RecordWatch uses it to
// upsert instead of inserting a fresh row on every pause/visibility change.
func (r *VideoWatchEventRepositoryImpl) FindByClientAndPost(clientID, feedPostID uint) (*models.VideoWatchEvent, error) {
	var event models.VideoWatchEvent
	err := r.Db.Where("client_id = ? AND feed_post_id = ?", clientID, feedPostID).
		Order("id DESC").
		First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *VideoWatchEventRepositoryImpl) Update(event *models.VideoWatchEvent) error {
	return r.Db.Save(event).Error
}

func (r *VideoWatchEventRepositoryImpl) FindRecentByClientID(clientID uint, limit int) ([]models.VideoWatchEvent, error) {
	var events []models.VideoWatchEvent
	err := r.Db.Where("client_id = ?", clientID).Order("created_at DESC").Limit(limit).Find(&events).Error
	return events, err
}
