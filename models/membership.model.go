package models

import "gorm.io/gorm"

type Membership struct {
	gorm.Model
	BeautyCenterID uint         `gorm:"index" json:"beauty_center_id"`
	BeautyCenter   BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"beauty_center"`

	Name        string `gorm:"size:50;not null;default:tutty" json:"name"`
	Description string `gorm:"size:255" json:"description"`
	IsActive    bool   `gorm:"default:true" json:"is_active"`
}
