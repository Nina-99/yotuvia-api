package models

import "gorm.io/gorm"

type Personnel struct {
	gorm.Model
	BeautyCenterID uint `gorm:"index"`

	FirstName string `gorm:"size:50" json:"first_name"`
	LastName  string `gorm:"size:50" json:"last_name"`
	Email     string `gorm:"size:70;unique_index" json:"email"`
	Phone     string `gorm:"size:20;not null" json:"phone"`
	Password  string `gorm:"size:255" json:"-"`
	IsActive  bool   `gorm:"default:false" json:"is_active"`

	PersonnelImage []PersonnelImage `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	BeautyCenter   BeautyCenter     `gorm:"foreignKey:BeautyCenterID"`
}
