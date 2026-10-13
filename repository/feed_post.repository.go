package repository

import "github.com/Nina-99/yotuvia-api/models"

type FeedPostRepository interface {
	Create(post *models.FeedPost) error
	Update(post *models.FeedPost) error
	Delete(id uint) error
	FindByID(id uint) (*models.FeedPost, error)
	// FindAllActive serves the PUBLIC feed: is_active = true only.
	FindAllActive(page, limit int) ([]models.FeedPost, int64, error)
	// FindActiveByPersonnel serves the public professional profile feed:
	// is_active = true for one professional, newest first, paginated.
	FindActiveByPersonnel(personnelID uint, page, limit int) ([]models.FeedPost, int64, error)
	// FindActiveByCenter returns active posts of one center (public profile).
	FindActiveByCenter(centerID uint) ([]models.FeedPost, error)
	// FindByIDs returns every post matching the given ids in ONE query,
	// including inactive rows (used to resolve affinity signals without an
	// N+1 of FindByID calls).
	FindByIDs(ids []uint) ([]models.FeedPost, error)
	// FindByPersonnelID / FindAllByCenter return ALL states for management
	// surfaces (staff own list; center moderation list incl. staff videos).
	FindByPersonnelID(personnelID uint) ([]models.FeedPost, error)
	FindAllByCenter(centerID uint) ([]models.FeedPost, error)
	// IncrementViews bumps the views counter for an active post.
	IncrementViews(id uint) error
	// IncrementBookTaps bumps the book-CTA tap counter for an active post.
	// Mirrors IncrementViews: public, best-effort, one increment per call.
	IncrementBookTaps(id uint) error
	// Exists reports whether a post with the given id exists, using a single
	// COUNT query. Booking attribution uses it to decide whether a client
	// tracking id resolves, without paying for the feed preloads.
	Exists(id uint) (bool, error)
}
