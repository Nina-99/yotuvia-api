package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type SavedVideoRepositoryImpl struct {
	Db *gorm.DB
}

func NewSavedVideoRepositoryImpl(Db *gorm.DB) SavedVideoRepository {
	return &SavedVideoRepositoryImpl{Db: Db}
}

func (r *SavedVideoRepositoryImpl) Create(savedVideo *models.SavedVideo) error {
	return r.Db.Create(savedVideo).Error
}

func (r *SavedVideoRepositoryImpl) Delete(savedVideo *models.SavedVideo) error {
	return r.Db.Delete(savedVideo).Error
}

func (r *SavedVideoRepositoryImpl) FindByPostAndClient(postID uint, clientID uint) (*models.SavedVideo, error) {
	var savedVideo models.SavedVideo
	err := r.Db.Where("feed_post_id = ? AND client_id = ?", postID, clientID).First(&savedVideo).Error
	if err != nil {
		return nil, err
	}
	return &savedVideo, nil
}

// FindByPostAndClientUnscoped finds a saved video even if soft-deleted.
func (r *SavedVideoRepositoryImpl) FindByPostAndClientUnscoped(postID uint, clientID uint) (*models.SavedVideo, error) {
	var savedVideo models.SavedVideo
	err := r.Db.Unscoped().Where("feed_post_id = ? AND client_id = ?", postID, clientID).First(&savedVideo).Error
	if err != nil {
		return nil, err
	}
	return &savedVideo, nil
}

// Toggle atomically saves or unsaves a video.
// Returns (true, nil) if now saved, (false, nil) if now unsaved.
func (r *SavedVideoRepositoryImpl) Toggle(postID uint, clientID uint) (bool, error) {
	existing, err := r.FindByPostAndClientUnscoped(postID, clientID)
	if err == nil && existing != nil {
		if existing.DeletedAt.Time.IsZero() {
			// Currently saved → soft-delete (unsave)
			if delErr := r.Db.Delete(existing).Error; delErr != nil {
				return false, delErr
			}
			return false, nil
		}
		// Soft-deleted → restore (re-save)
		if restoreErr := r.Db.Unscoped().Model(existing).Update("deleted_at", nil).Error; restoreErr != nil {
			return false, restoreErr
		}
		return true, nil
	}

	// No record at all → create new save
	saved := models.SavedVideo{
		FeedPostID: postID,
		ClientID:   clientID,
	}
	if createErr := r.Db.Create(&saved).Error; createErr != nil {
		return false, createErr
	}
	return true, nil
}

func (r *SavedVideoRepositoryImpl) FindByClientID(clientID uint) ([]models.SavedVideo, error) {
	var savedVideos []models.SavedVideo
	err := r.Db.Where("client_id = ?", clientID).
		Order("created_at DESC").
		Limit(ListQueryLimit).
		Find(&savedVideos).Error
	return savedVideos, err
}
