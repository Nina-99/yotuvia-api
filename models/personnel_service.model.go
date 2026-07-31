package models

import "gorm.io/gorm"

type PersonnelService struct {
	gorm.Model
	PersonnelID uint     `gorm:"not null;uniqueIndex:idx_personnel_service"`
	ServiceID   uint     `gorm:"not null;uniqueIndex:idx_personnel_service"`
	Price       *float64 `json:"price,omitempty"`
	Duration    *int     `json:"duration,omitempty"`

	Personnel Personnel `gorm:"foreignKey:PersonnelID;constraint:OnDelete:CASCADE"`
	Service   Service   `gorm:"foreignKey:ServiceID;constraint:OnDelete:CASCADE"`
}
