package models

import "gorm.io/gorm"

type PersonnelImage struct {
	gorm.Model
	PersonnelID uint `gorm:"index"`

	ImageUrl string `json:"image_url"`
	IsCover  bool   `json:"is_cover"`
	Position int    `json:"position"`
}
