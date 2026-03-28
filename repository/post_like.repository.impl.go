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

func (p *PostLikeRepositoryImpl) FindByPostID(postID uint) ([]models.PostLike, error) {
	var postLikes []models.PostLike
	err := p.Db.Where("feed_post_id = ?", postID).Find(&postLikes).Error
	return postLikes, err
}
