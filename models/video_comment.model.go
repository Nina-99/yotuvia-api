package models

import (
	"time"

	"gorm.io/gorm"
)

type VideoComment struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	FeedPostID uint           `gorm:"index;not null" json:"feed_post_id"`
	ClientID   *uint          `gorm:"index" json:"client_id,omitempty"`
	ParentID   *uint          `gorm:"index" json:"parent_id,omitempty"`
	Body       string         `gorm:"size:1000;not null" json:"body"`
	IsOfficial bool           `gorm:"default:false" json:"is_official"`
	IsHidden   bool           `gorm:"default:false" json:"is_hidden"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	Replies []VideoComment `gorm:"foreignKey:ParentID" json:"replies,omitempty"`
}

func (VideoComment) TableName() string {
	return "video_comments"
}