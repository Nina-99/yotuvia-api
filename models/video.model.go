package models

import "gorm.io/gorm"

type Video struct {
	gorm.Model
	BeautyCenterID uint         `gorm:"not null;index" json:"beauty_center_id"`
	BeautyCenter   BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"-"`
	Title          string       `gorm:"size:200" json:"title"`
	Description    string       `gorm:"size:500" json:"description"`
	FileName       string       `gorm:"size:255;not null" json:"file_name"`
	FilePath       string       `gorm:"size:500;not null" json:"file_path"`
	ThumbnailURL   string       `gorm:"size:500" json:"thumbnail_url"`
	Views          int          `gorm:"default:0" json:"views"`
	IsActive       bool         `gorm:"default:true" json:"is_active"`
}
