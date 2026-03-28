package response

import "time"

type SocialMediaResponse struct {
	ID             uint      `json:"id"`
	BeautyCenterID uint      `json:"beauty_center_id"`
	Google         string    `json:"google"`
	Facebook       string    `json:"facebook"`
	Instagram      string    `json:"instagram"`
	Twitter        string    `json:"twitter"`
	TikTok         string    `json:"tiktok"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
