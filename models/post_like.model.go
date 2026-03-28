package models

import "gorm.io/gorm"

type PostLike struct {
	gorm.Model
	FeedPostID uint `gorm:"uniqueIndex:idx_post_client;not null"`
	ClientID   uint `gorm:"uniqueIndex:idx_post_client;not null"`
}
