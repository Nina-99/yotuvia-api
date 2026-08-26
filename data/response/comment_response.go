package response

import "time"

type CommentResponse struct {
	ID         uint               `json:"id"`
	Body       string             `json:"body"`
	IsOfficial bool               `json:"is_official"`
	IsHidden   bool               `json:"is_hidden"`
	CreatedAt  time.Time          `json:"created_at"`
	Author     AuthorInfo         `json:"author"`
	Replies    []CommentResponse  `json:"replies,omitempty"`
}

type AuthorInfo struct {
	ID       uint   `json:"id"`
	CenterID uint   `json:"center_id,omitempty"`
	Name     string `json:"name"`
	Avatar   string `json:"avatar_url,omitempty"`
	IsStaff  bool   `json:"is_staff"`
	IsCenter bool   `json:"is_center"`
}
