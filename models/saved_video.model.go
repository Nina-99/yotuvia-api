package models

import "gorm.io/gorm"

// SavedVideo represents a video saved/bookmarked by a client.
type SavedVideo struct {
	gorm.Model
	FeedPostID uint `gorm:"uniqueIndex:idx_saved_post_client;not null"`
	ClientID   uint `gorm:"uniqueIndex:idx_saved_post_client;not null"`
}
