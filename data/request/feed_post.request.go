package request

type CreateFeedPostRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" binding:"required"`
	PersonnelID    uint   `json:"personnel_id"`
	ServiceID      uint   `json:"service_id"`
	VideoURL       string `json:"video_url" binding:"required,url"`
	ThumbnailURL   string `json:"thumbnail_url" binding:"required,url"`
	Caption        string `json:"caption" binding:"max=255"`
	Duration       int    `json:"duration"`
}

type UpdateFeedPostRequest struct {
	Caption      string `json:"caption" binding:"max=255"`
	VideoURL     string `json:"video_url" binding:"omitempty,url"`
	ThumbnailURL string `json:"thumbnail_url" binding:"omitempty,url"`
}
