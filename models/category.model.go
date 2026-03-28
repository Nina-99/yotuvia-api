package models

import "gorm.io/gorm"

type Category struct {
	gorm.Model
	BeautyCenterID uint `gorm:"not null"`

	Name        string `gorm:"size:100;not null" json:"name"`
	Description string `json:"description"`

	Services []Service `gorm:"foreignKey:CategoryID" json:"services"`
}
