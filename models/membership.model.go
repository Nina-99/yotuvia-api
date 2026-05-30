package models

import "gorm.io/gorm"

type Membership struct {
	gorm.Model
	Name         string  `gorm:"size:50;not null;default:tutty" json:"name"`
	Price        float64 `gorm:"type:decimal(10,2);not null" json:"price"`
	DurationDays int     `gorm:"not null" json:"duration_days"`
	Description  string  `gorm:"size:255" json:"description"`
	IsActive     bool    `gorm:"default:true" json:"is_active"`
}
