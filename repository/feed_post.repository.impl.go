package repository

import (
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
	"BeautyCenter",
	"Service",
	"Service.Category",
	"Personnel",
}

func applyFeedPreloads(q *gorm.DB) *gorm.DB {
	for _, p := range feedPreloads {
		q = q.Preload(p)
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
		Order("is_promo DESC, created_at DESC").
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
		Order("is_promo DESC, created_at DESC").
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
