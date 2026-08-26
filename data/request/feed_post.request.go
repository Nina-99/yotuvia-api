package request

type CreateFeedPostRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	PersonnelID    uint   `json:"personnel_id"`
	ServiceID      uint   `json:"service_id"`
	VideoURL       string `json:"video_url" validate:"required,url"`
	ThumbnailURL   string `json:"thumbnail_url" validate:"required,url"`
	Caption        string `json:"caption" validate:"max=255"`
	Duration       int    `json:"duration"`
	IsPromo        bool   `json:"is_promo"`
}

type UpdateFeedPostRequest struct {
	Caption      string `json:"caption" validate:"max=255"`
	IsPromo      bool   `json:"is_promo"`
	VideoURL     string `json:"video_url" validate:"omitempty,url"`
	ThumbnailURL string `json:"thumbnail_url" validate:"omitempty,url"`
}
