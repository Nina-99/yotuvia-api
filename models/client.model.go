package models

import "gorm.io/gorm"

type Client struct {
	gorm.Model
	FirstName string `gorm:"size:50;not null" json:"first_name"`
	LastName  string `gorm:"size:50;not null" json:"last_name"`
	Username  string `gorm:"size:50;not null;unique_index" json:"username"`
	Phone     string `gorm:"size:20;not null" json:"phone"`
	Email     string `gorm:"size:70;not null;unique_index" json:"email"`
	// Never serialised: the hash must not reach the wire. Endpoints that
	// return the raw model (e.g. the client's reservation list) preload Client,
	// so a JSON tag here leaked the bcrypt hash in the response body.
	Password  string `gorm:"size:255" json:"-"`
	AvatarURL string `json:"avatar_url"`
	IsActive  bool   `gorm:"default:true" json:"is_active"`
	// TrustScore is the client's reliability reputation (0-100, starts 100).
	// -20 per no-show, +2 per attendance. Centers exempt high-score clients
	// from deposit requirements. Clamped at the repository boundary.
	TrustScore int `gorm:"default:100" json:"trust_score"`

	Reservations []Reservation `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}
