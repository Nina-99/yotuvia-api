package response

import "time"

type InvitationResponse struct {
	ID             uint       `json:"id,omitempty"`
	Token          string     `json:"token"`
	Email          string     `json:"email"`
	BeautyCenterID uint       `json:"beauty_center_id"`
	CenterName     string     `json:"center_name,omitempty"`
	Status         string     `json:"status,omitempty"`
	ExpiresAt      time.Time  `json:"expires_at"`
	UsedAt         *time.Time `json:"used_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at,omitempty"`
}
