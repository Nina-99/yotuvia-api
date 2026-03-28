package models

import "gorm.io/gorm"

type Service struct {
	gorm.Model
	BeautyCenterID uint     `gorm:"not null;index"`
	CategoryID     uint     `gorm:"not null;index"`
	Category       Category `gorm:"foreignKey:CategoryID"`

	Name        string  `gorm:"size:100;not null" json:"name"`
	Description string  `gorm:"not null" json:"description"`
	Duration    int     `gorm:"not null" json:"duration"`
	Price       float64 `json:"price"`
	ImageURL    string  `json:"image_url"`
	IsActive    bool    `gorm:"default:true" json:"is_active"`

	Reservation []Reservation `gorm:"constraint: OnUpdate:CASCADE,OnDelete:SET NULL;"`
}
