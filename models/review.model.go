package models

import "gorm.io/gorm"

type Review struct {
	gorm.Model
	BeautyCenterID uint   `gorm:"not null"`
	ClientID       uint   `gorm:"not null"`
	Rating         int    `gorm:"not null" json:"rating"`
	Comment        string `gorm:"type:text" json:"comment"`

	// Relationships
	Client       Client       `gorm:"foreignKey:ClientID"`
	BeautyCenter BeautyCenter `gorm:"foreignKey:BeautyCenterID"`
}
