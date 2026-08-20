package response

import "time"

type NotificationResponse struct {
	ID             uint       `json:"id"`
	ClientID       uint       `json:"client_id"`
	PersonnelID    *uint      `json:"personnel_id,omitempty"`
	BeautyCenterID *uint      `json:"beauty_center_id,omitempty"`
	Type           string     `json:"type"`
	Title          string     `json:"title"`
	Message        string     `json:"message"`
	Data           string     `json:"data,omitempty"`
	IsRead         bool       `json:"is_read"`
	CreatedAt      time.Time  `json:"created_at"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
}
