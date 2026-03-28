package models

import "gorm.io/gorm"

type Favorite struct {
	gorm.Model
	ClientID       uint         `gorm:"not null;index" json:"client_id"`
	BeautyCenterID uint         `gorm:"not null;index" json:"beauty_center_id"`
	BeautyCenter   BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"beauty_center"`
}
