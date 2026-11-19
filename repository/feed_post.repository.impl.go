package repository

import (
	"math"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type FeedPostRepositoryImpl struct {
	db *gorm.DB
}

func NewFeedPostRepositoryImpl(db *gorm.DB) FeedPostRepository {
	return &FeedPostRepositoryImpl{db: db}
}

// Create implements FeedPostRepository
func (r *FeedPostRepositoryImpl) Create(post *models.FeedPost) error {
	return r.db.Create(post).Error
}

// Update implements FeedPostRepository
func (r *FeedPostRepositoryImpl) Update(post *models.FeedPost) error {
	return r.db.Save(post).Error
}

// Delete implements FeedPostRepository
func (r *FeedPostRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.FeedPost{}, id).Error
}

// FindByID implements FeedPostRepository
func (r *FeedPostRepositoryImpl) FindByID(id uint) (*models.FeedPost, error) {
	var post models.FeedPost
	err := applyFeedPreloads(r.db).First(&post, id).Error
	if err != nil {
		return nil, err
	}
	return &post, nil
}

// feedPreloads lists every association the feed surfaces require, including
// the category chain for filter inheritance. PostLike rows are deliberately
// NOT preloaded: the like count is computed live with a grouped COUNT, and
// nothing reads post.Likes.
var feedPreloads = []string{
	"Service",
	"Service.Category",
	"Personnel",
}

// feedProfileImagePreloads narrows the creator's image associations to the
// profile image only (is_profile), so the card can render an avatar without
// dragging every gallery image: still one bounded query per association, never
// a per-row lookup.
var feedProfileImagePreloads = []string{
	"BeautyCenter.BeautyCenterImage",
	"Personnel.PersonnelImage",
}

func applyFeedPreloads(q *gorm.DB) *gorm.DB {
	// The feed's centre must carry avg_rating / review_count. Those are
	// read-only PROJECTIONS on the model (computed by the shared rating
	// aggregate scope), so a plain Preload("BeautyCenter") leaves them at zero —
	// which is exactly why every slide showed a blank rating. Its schedule rides
	// along for the "open now" badge.
	q = q.
		Preload("BeautyCenter", func(db *gorm.DB) *gorm.DB {
			return withAverageRating("")(db)
		}).
		Preload("BeautyCenter.WorkingHour")
	for _, p := range feedPreloads {
		q = q.Preload(p)
	}
	for _, p := range feedProfileImagePreloads {
		q = q.Preload(p, "is_profile = ?", true)
	}
	return q
}

// FindByIDs implements FeedPostRepository — resolves a set of post ids in a
// single round trip. It applies the same preloads as FindByID and, like
// FindByID, does NOT filter is_active so affinity signals keep referencing the
// exact rows the previous per-id lookups returned.
func (r *FeedPostRepositoryImpl) FindByIDs(ids []uint) ([]models.FeedPost, error) {
	posts := make([]models.FeedPost, 0, len(ids))
	if len(ids) == 0 {
		return posts, nil
	}
	err := applyFeedPreloads(r.db).Where("id IN ?", ids).Find(&posts).Error
	return posts, err
}

// FindAllActive implements FeedPostRepository — public feed filters suspended.
func (r *FeedPostRepositoryImpl) FindAllActive(page, limit int) ([]models.FeedPost, int64, error) {
	var posts []models.FeedPost
	var total int64

	q := r.db.Model(&models.FeedPost{}).Where("is_active = ?", true)
	q.Count(&total)

	offset := (page - 1) * limit
	err := applyFeedPreloads(q).
		// Newest first. `is_promo` is a LABEL, not a ranking signal: ordering by
		// it gave any centre free top-of-feed placement, which undercut the paid
		// boost that is the only placement the platform sells. Promotion is
		// injected by the auction (MergePromotedIDs) and nowhere else.
		Order("created_at DESC").
		Offset(offset).Limit(limit).Find(&posts).Error
	return posts, total, err
}

// FindActiveByPersonnel implements FeedPostRepository — the public
// professional-profile feed: one professional's ACTIVE posts, newest first,
// with the same preloads as the main feed. A page mirror of FindAllActive.
func (r *FeedPostRepositoryImpl) FindActiveByPersonnel(personnelID uint, page, limit int) ([]models.FeedPost, int64, error) {
	var posts []models.FeedPost
	var total int64

	q := r.db.Model(&models.FeedPost{}).
		Where("personnel_id = ? AND is_active = ?", personnelID, true)
	q.Count(&total)

	offset := (page - 1) * limit
	err := applyFeedPreloads(q).
		Order("created_at DESC").
		Offset(offset).Limit(limit).Find(&posts).Error
	return posts, total, err
}

// FindActiveByCenter implements FeedPostRepository — public profile variant.
func (r *FeedPostRepositoryImpl) FindActiveByCenter(centerID uint) ([]models.FeedPost, error) {
	var posts []models.FeedPost
	err := applyFeedPreloads(r.db).
		Where("beauty_center_id = ? AND is_active = ?", centerID, true).
		// Newest first. `is_promo` is a LABEL, not a ranking signal: ordering by
		// it gave any centre free top-of-feed placement, which undercut the paid
		// boost that is the only placement the platform sells. Promotion is
		// injected by the auction (MergePromotedIDs) and nowhere else.
		Order("created_at DESC").
		Limit(ListQueryLimit).
		Find(&posts).Error
	return posts, err
}

// FindByPersonnelID implements FeedPostRepository — management list: all states.
func (r *FeedPostRepositoryImpl) FindByPersonnelID(personnelID uint) ([]models.FeedPost, error) {
	var posts []models.FeedPost
	err := r.db.Preload("BeautyCenter").
		Preload("Service.Category").
		Where("personnel_id = ?", personnelID).
		Order("created_at desc").
		Limit(ListQueryLimit).
		Find(&posts).Error
	return posts, err
}

// FindAllByCenter implements FeedPostRepository — center moderation list:
// every post of the center (own + staff) in all states.
func (r *FeedPostRepositoryImpl) FindAllByCenter(centerID uint) ([]models.FeedPost, error) {
	var posts []models.FeedPost
	err := applyFeedPreloads(r.db).
		Where("beauty_center_id = ?", centerID).
		Order("created_at desc").
		Limit(ListQueryLimit).
		Find(&posts).Error
	return posts, err
}

// IncrementViews bumps views for an active post (public, best-effort caller).
func (r *FeedPostRepositoryImpl) IncrementViews(id uint) error {
	return r.db.Model(&models.FeedPost{}).
		Where("id = ? AND is_active = ?", id, true).
		UpdateColumn("views_count", gorm.Expr("views_count + 1")).Error
}

// IncrementBookTaps bumps book-CTA taps for an active post, mirroring
// IncrementViews exactly (public, best-effort, one increment per call).
func (r *FeedPostRepositoryImpl) IncrementBookTaps(id uint) error {
	return r.db.Model(&models.FeedPost{}).
		Where("id = ? AND is_active = ?", id, true).
		UpdateColumn("book_taps", gorm.Expr("book_taps + 1")).Error
}

// Exists reports whether a post id resolves, in one COUNT query. It is the
// existence probe behind booking attribution and deliberately avoids the feed
// preloads (no association is read).
func (r *FeedPostRepositoryImpl) Exists(id uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.FeedPost{}).Where("id = ?", id).Count(&count).Error
	return count > 0, err
}

// FindActiveForFeed bounds the ranking candidate set.
//
// Two problems solved at once:
//  1. VOLUME: the ranker used to load the entire newest-1000 catalog with every
//     association just to score it and throw most of it away.
//  2. RELEVANCE: loading the whole country only to filter it in Go wasted the
//     `idx_bc_location` index that already exists.
//
// Centres with (0,0) — never given coordinates — are INCLUDED: excluding them
// would silently delete them from discovery, which is worse than a post scoring
// zero on proximity.
func (r *FeedPostRepositoryImpl) FindActiveForFeed(lat, lng, radiusKm float64, limit int) ([]models.FeedPost, error) {
	if limit <= 0 {
		limit = 300
	}

	q := applyFeedPreloads(r.db.Model(&models.FeedPost{})).
		Where("feed_posts.is_active = ?", true)

	if lat != 0 || lng != 0 {
		const kmPerDegree = 111.32
		dLat := radiusKm / kmPerDegree
		cosLat := math.Cos(lat * math.Pi / 180)
		if cosLat < 0.01 {
			cosLat = 0.01 // near the poles the longitude span explodes
		}
		dLng := radiusKm / (kmPerDegree * cosLat)

		q = q.
			Joins("JOIN beauty_centers bc ON bc.id = feed_posts.beauty_center_id AND bc.deleted_at IS NULL").
			Where(`(
				(bc.latitude BETWEEN ? AND ? AND bc.longitude BETWEEN ? AND ?)
				OR (bc.latitude = 0 AND bc.longitude = 0)
			)`, lat-dLat, lat+dLat, lng-dLng, lng+dLng)
	}

	var posts []models.FeedPost
	err := q.Order("feed_posts.created_at DESC, feed_posts.id DESC").Limit(limit).Find(&posts).Error
	return posts, err
}
