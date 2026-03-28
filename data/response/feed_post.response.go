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
}
