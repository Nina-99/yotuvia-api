package models

import (
	"time"

	"gorm.io/gorm"
)

type Service struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	BeautyCenterID uint     `gorm:"not null;index" json:"beauty_center_id"`
	CategoryID     uint     `gorm:"not null;index" json:"category_id"`
	Category       Category `gorm:"foreignKey:CategoryID" json:"category,omitempty"`

	Name        string  `gorm:"size:100;not null" json:"name"`
	Description string  `gorm:"not null" json:"description"`
	Duration    int     `gorm:"not null" json:"duration"`
	Price       float64 `json:"price"`
	ImageURL    string  `json:"image_url"`
	IsActive    bool    `gorm:"default:true" json:"is_active"`
}
