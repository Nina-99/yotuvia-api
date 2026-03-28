package models

import "gorm.io/gorm"

type FeedPost struct {
	gorm.Model
	BeautyCenterID uint `gorm:"index"`
	PersonnelID    uint `gorm:"index"`
	ServiceID      uint `gorm:"index"`

	VideoURL     string `gorm:"not null" json:"video_url"`
	ThumbnailURL string `gorm:"not null" json:"thumbnail_url"`
	Caption      string `gorm:"size:255" json:"caption"`
	Duration     int    `json:"duration"`
	ViewsCount   int    `gorm:"default:0" json:"views_count"`
	LikesCount   int    `gorm:"default:0" json:"likes_count"`
	IsActive     bool   `gorm:"default:true" json:"is_active"`

	Likes        []PostLike   `gorm:"foreignKey:FeedPostID" json:"-"`
	Service      Service      `gorm:"foreignKey:ServiceID" json:"service"`
	BeautyCenter BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"beauty_center"`
	Personnel    Personnel    `gorm:"foreignKey:PersonnelID" json:"personnel"`
}
