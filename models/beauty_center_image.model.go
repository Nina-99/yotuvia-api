package models

import "gorm.io/gorm"

type BeautyCenterImage struct {
	gorm.Model
	BeautyCenterID uint `gorm:"index"`

	ImageUrl string `json:"image_url"`
	IsCover  bool   `json:"is_cover"`
	Position int    `json:"position"`
}
