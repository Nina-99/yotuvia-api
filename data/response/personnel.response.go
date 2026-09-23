package response

import "time"

type PersonnelImageResponse struct {
	ID          uint      `json:"id"`
	PersonnelID uint      `json:"personnel_id"`
	ImageUrl    string    `json:"image_url"`
	IsCover     bool      `json:"is_cover"`
	IsProfile   bool      `json:"is_profile"`
	Position    int       `json:"position"`
	CreatedAt   time.Time `json:"created_at"`
}

type PersonnelResponse struct {
	ID             uint                     `json:"id"`
	BeautyCenterID uint                     `json:"beauty_center_id"`
	FirstName      string                   `json:"first_name"`
	LastName       string                   `json:"last_name"`
	Email          string                   `json:"email"`
	Phone          string                   `json:"phone"`
	IsActive       bool                     `json:"is_active"`
	Images         []PersonnelImageResponse `json:"images"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
}
