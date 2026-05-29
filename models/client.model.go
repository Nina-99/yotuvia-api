package models

import "gorm.io/gorm"

type Client struct {
	gorm.Model
	FirstName string `gorm:"size:50;not null" json:"first_name"`
	LastName  string `gorm:"size:50;not null" json:"last_name"`
	Username  string `gorm:"size:50;not null;unique_index" json:"username"`
	Phone     string `gorm:"size:20;not null" json:"phone"`
	Email     string `gorm:"size:70;not null;unique_index" json:"email"`
	Password  string `gorm:"size:255" json:"password"`
	AvatarURL string `json:"avatar_url"`
	IsActive  bool   `gorm:"default:true" json:"is_active"`

	Reservations []Reservation `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}
