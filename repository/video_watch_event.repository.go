package repository

import "github.com/Nina-99/yotuvia-api/models"

type VideoWatchEventRepository interface {
	Create(event *models.VideoWatchEvent) error
	// Update persists changes to an existing event (used by the
	// idempotent RecordWatch upsert).
	Update(event *models.VideoWatchEvent) error
	// FindByClientAndPost returns the newest event for a (client, post) pair,
	// or gorm.ErrRecordNotFound when the pair has never been recorded.
	FindByClientAndPost(clientID, feedPostID uint) (*models.VideoWatchEvent, error)
	FindByClientID(clientID uint) ([]models.VideoWatchEvent, error)

	// WatchedPostIDs answers, for a bounded set of posts, which ones this client
	// has already watched. One indexed query; it exists so the ad server does not
	// re-serve a promoted video the viewer already consumed.
	WatchedPostIDs(clientID uint, postIDs []uint) (map[uint]bool, error)
	FindRecentByClientID(clientID uint, limit int) ([]models.VideoWatchEvent, error)
}
