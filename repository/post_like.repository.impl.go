package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type PostLikeRepositoryImpl struct {
	Db *gorm.DB
}

func NewPostLikeRepositoryImpl(Db *gorm.DB) PostLikeRepository {
	return &PostLikeRepositoryImpl{Db: Db}
}

func (p *PostLikeRepositoryImpl) Create(postLike *models.PostLike) error {
	return p.Db.Create(postLike).Error
}

func (p *PostLikeRepositoryImpl) Delete(postLike *models.PostLike) error {
	return p.Db.Delete(postLike).Error
}

func (p *PostLikeRepositoryImpl) FindByPostAndClient(postID uint, clientID uint) (*models.PostLike, error) {
	var postLike models.PostLike
	err := p.Db.Where("feed_post_id = ? AND client_id = ?", postID, clientID).First(&postLike).Error
	if err != nil {
		return nil, err
	}
	return &postLike, nil
}

// FindByPostAndClientUnscoped finds a like even if soft-deleted.
func (p *PostLikeRepositoryImpl) FindByPostAndClientUnscoped(postID uint, clientID uint) (*models.PostLike, error) {
	var postLike models.PostLike
	err := p.Db.Unscoped().Where("feed_post_id = ? AND client_id = ?", postID, clientID).First(&postLike).Error
	if err != nil {
		return nil, err
	}
	return &postLike, nil
}

// Toggle atomically creates or restores a like.
// Returns (true, nil) if now liked, (false, nil) if now unliked.
func (p *PostLikeRepositoryImpl) Toggle(postID uint, clientID uint) (bool, error) {
	// Find even soft-deleted records
	existing, err := p.FindByPostAndClientUnscoped(postID, clientID)
	if err == nil && existing != nil {
		// Record exists — if active, soft-delete (unlike); if soft-deleted, restore (re-like)
		if existing.DeletedAt.Time.IsZero() {
			// Currently active → soft-delete (unlike)
			if delErr := p.Db.Delete(existing).Error; delErr != nil {
				return false, delErr
			}
			return false, nil
		}
		// Soft-deleted → restore (re-like)
		if restoreErr := p.Db.Unscoped().Model(existing).Update("deleted_at", nil).Error; restoreErr != nil {
			return false, restoreErr
		}
		return true, nil
	}

	// No record at all → create new like
	postLike := models.PostLike{
		FeedPostID: postID,
		ClientID:   clientID,
	}
	if createErr := p.Db.Create(&postLike).Error; createErr != nil {
		return false, createErr
	}
	return true, nil
}

// FindByPostID backs the PUBLIC GET /post-likes/post/:id list. Ordering is
// added explicitly (the previous query had none) so the cap keeps the newest
// likes deterministically.
func (p *PostLikeRepositoryImpl) FindByPostID(postID uint) ([]models.PostLike, error) {
	var postLikes []models.PostLike
	err := p.Db.Where("feed_post_id = ?", postID).
		Order("created_at DESC").
		Limit(ListQueryLimit).
		Find(&postLikes).Error
	return postLikes, err
}

func (p *PostLikeRepositoryImpl) FindByClientID(clientID uint) ([]models.PostLike, error) {
	var postLikes []models.PostLike
	err := p.Db.Where("client_id = ?", clientID).
		Order("created_at DESC").
		Limit(ListQueryLimit).
		Find(&postLikes).Error
	return postLikes, err
}

// CountByFeedPostIDs runs one grouped COUNT for a page of posts. GORM's default
// scope excludes soft-deleted rows, so the result is the number of LIVE likes.
func (p *PostLikeRepositoryImpl) CountByFeedPostIDs(ids []uint) (map[uint]int64, error) {
	counts := make(map[uint]int64, len(ids))
	if len(ids) == 0 {
		return counts, nil
	}

	var rows []struct {
		FeedPostID uint
		Count      int64
	}
	err := p.Db.Model(&models.PostLike{}).
		Select("feed_post_id, COUNT(*) AS count").
		Where("feed_post_id IN ?", ids).
		Group("feed_post_id").
		Scan(&rows).Error
	if err != nil {
		return counts, err
	}
	for _, row := range rows {
		counts[row.FeedPostID] = row.Count
	}
	return counts, nil
}
