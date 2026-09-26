package models

import "gorm.io/gorm"

type FeedPost struct {
	gorm.Model
	BeautyCenterID uint  `gorm:"index"`
	PersonnelID    *uint `gorm:"index" json:"personnel_id,omitempty"`
	ServiceID      *uint `gorm:"index" json:"service_id,omitempty"`

	VideoURL     string `gorm:"not null" json:"video_url"`
	ThumbnailURL string `gorm:"not null" json:"thumbnail_url"`
	Caption      string `gorm:"size:255" json:"caption"`
	Duration     int    `json:"duration"`
	ViewsCount   int    `gorm:"default:0" json:"views_count"`
	LikesCount   int    `gorm:"default:0" json:"likes_count"`
	// BookTaps counts CTA taps on this feed post (video → booking funnel).
	// It is the middle signal between a view and an attributed booking.
	BookTaps int `gorm:"default:0" json:"book_taps"`
	IsActive     bool   `gorm:"default:true" json:"is_active"`
	IsPromo        bool   `gorm:"default:false" json:"is_promo"`
	ModerationNote string `gorm:"size:255" json:"moderation_note,omitempty"`

	Likes        []PostLike   `gorm:"foreignKey:FeedPostID" json:"-"`
	Service      Service      `gorm:"foreignKey:ServiceID" json:"service"`
	BeautyCenter BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"beauty_center"`
	Personnel    Personnel    `gorm:"foreignKey:PersonnelID" json:"personnel"`
}
