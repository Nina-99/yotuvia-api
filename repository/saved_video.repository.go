package repository

import "github.com/Nina-99/yottu-api/models"

type SavedVideoRepository interface {
	Create(savedVideo *models.SavedVideo) error
	Delete(savedVideo *models.SavedVideo) error
	FindByPostAndClient(postID uint, clientID uint) (*models.SavedVideo, error)
	FindByClientID(clientID uint) ([]models.SavedVideo, error)
	// Toggle atomically saves or unsaves a video, returning the new saved state.
	Toggle(postID uint, clientID uint) (bool, error)
	// FindByPostAndClientUnscoped finds a saved video even if soft-deleted.
	FindByPostAndClientUnscoped(postID uint, clientID uint) (*models.SavedVideo, error)
}
