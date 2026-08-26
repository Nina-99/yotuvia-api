package repository

import "github.com/Nina-99/yottu-api/models"

type FeedPostRepository interface {
	Create(post *models.FeedPost) error
	Update(post *models.FeedPost) error
	Delete(id uint) error
	FindByID(id uint) (*models.FeedPost, error)
	// FindAllActive serves the PUBLIC feed: is_active = true only.
	FindAllActive(page, limit int) ([]models.FeedPost, int64, error)
	// FindActiveByCenter returns active posts of one center (public profile).
	FindActiveByCenter(centerID uint) ([]models.FeedPost, error)
	// FindByPersonnelID / FindAllByCenter return ALL states for management
	// surfaces (staff own list; center moderation list incl. staff videos).
	FindByPersonnelID(personnelID uint) ([]models.FeedPost, error)
	FindAllByCenter(centerID uint) ([]models.FeedPost, error)
	// IncrementViews bumps the views counter for an active post.
	IncrementViews(id uint) error
}
