package response

import "time"

type FeedPostResponse struct {
	ID             uint                 `json:"id"`
	BeautyCenterID uint                 `json:"beauty_center_id"`
	BeautyCenter   BeautyCenterResponse `json:"beauty_center,omitempty"`
	PersonnelID    uint                 `json:"personnel_id"`
	ServiceID      uint                 `json:"service_id"`
	VideoURL       string               `json:"video_url"`
	ThumbnailURL   string               `json:"thumbnail_url"`
	Caption        string               `json:"caption"`
	Duration       int                  `json:"duration"`
	ViewsCount     int                  `json:"views_count"`
	LikesCount     int                  `json:"likes_count"`
	CreatedAt      time.Time            `json:"created_at"`

	// feat-video-feed-unification
	IsPromo        bool   `json:"is_promo"`
	ModerationNote string `json:"moderation_note,omitempty"`
	IsActive       bool   `json:"is_active"`
	ServiceName    string `json:"service_name,omitempty"`
	CategoryID     uint   `json:"category_id,omitempty"`
	CategoryName   string `json:"category_name,omitempty"`
	PersonnelName  string `json:"personnel_name,omitempty"`
}
