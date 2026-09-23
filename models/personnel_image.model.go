package models

import "gorm.io/gorm"

type PersonnelImage struct {
	gorm.Model
	PersonnelID uint `gorm:"index"`

	ImageUrl  string `json:"image_url"`
	IsCover   bool   `json:"is_cover"`
	IsProfile bool   `json:"is_profile"`
	Position  int    `json:"position"`
}
