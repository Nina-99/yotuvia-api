package models

import "gorm.io/gorm"

type BeautyCenter struct {
	gorm.Model
	Name        string `gorm:"size:50;not null" json:"name"`
	Address     string `gorm:"size:50;not null" json:"address"`
	Email       string `gorm:"size:70;not null;uniqueIndex" json:"email"`
	Phone       string `gorm:"size:20;not null" json:"phone"`
	Url         string `gorm:"unique_index" json:"url"`
	Description string `gorm:"size:255" json:"description"`
	Password    string `gorm:"size:255" json:"-"`
	IsActive    bool   `gorm:"default:true" json:"is_active"`

	BeautyCenterImage []BeautyCenterImage `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Membership        []Membership        `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Personnel         []Personnel         `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Reservation       []Reservation       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	SocialMedia       []SocialMedia       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	WorkingHour       []WorkingHour       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`

	DeletedAt gorm.DeletedAt `gorm:"index"`
}
