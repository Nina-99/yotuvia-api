package models

import "gorm.io/gorm"

// VideoWatchEvent records when a client watches a video.
// Used to compute personalized feed rankings.
type VideoWatchEvent struct {
	gorm.Model
	ClientID        uint    `gorm:"index:idx_watch_client_post;not null"`
	FeedPostID      uint    `gorm:"index:idx_watch_client_post;not null"`
	WatchDuration   int     `json:"watch_duration"`    // seconds watched
	WatchPercentage float64 `json:"watch_percentage"`   // 0-100
	Completed       bool    `json:"completed" gorm:"default:false"`
}
