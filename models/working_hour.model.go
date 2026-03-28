package models

import "gorm.io/gorm"

type WorkingHour struct {
	gorm.Model
	BeautyCenterID uint `gorm:"index;not null"`

	DayOfWeek int    `gorm:"not null" json:"day_of_week"`
	StartTime string `gorm:"size:5;not null" json:"start_time"`
	EndTime   string `gorm:"size:5;not null" json:"end_time"`
}
