package models

import "gorm.io/gorm"

type SocialMedia struct {
	gorm.Model
	BeautyCenterID uint `gorm:"unique_index"`

	Google    string `gorm:"unique_index" json:"google"`
	Facebook  string `gorm:"unique_index" json:"facebook"`
	Instagram string `gorm:"unique_index" json:"instagram"`
	Twitter   string `gorm:"unique_index" json:"twitter"`
	TikTok    string `gorm:"unique_index" json:"tiktok"`
}
