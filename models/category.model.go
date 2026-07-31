package models

import (
	"time"

	"gorm.io/gorm"
)

type Category struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	BeautyCenterID uint `gorm:"not null" json:"beauty_center_id"`

	Name        string `gorm:"size:100;not null" json:"name"`
	Description string `json:"description"`
	Icon        string `gorm:"size:10" json:"icon"`

	Services []Service `gorm:"foreignKey:CategoryID" json:"services"`
}
