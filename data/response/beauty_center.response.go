package response

import "time"

type BeautyCenterResponse struct {
	Currency    string                  `json:"currency"`
	ID          uint                    `json:"id"`
	Name        string                  `json:"name"`
	Address     string                  `json:"address"`
	Email       string                  `json:"email"`
	Phone       string                  `json:"phone"`
	Url         string                  `json:"url"`
	Description string                  `json:"description"`
	IsActive    bool                    `json:"is_active"`
	Latitude    float64                 `json:"latitude"`
	Longitude   float64                 `json:"longitude"`
	AvgRating   float64                 `json:"avg_rating"`
	ReviewCount int64                   `json:"review_count"`
	Images      []BeautyCenterImageResp `json:"images"`
	Schedules   []WorkingHourResp       `json:"schedules"`
	SocialMedia *SocialMediaResp        `json:"social_media,omitempty"`
}

type BeautyCenterImageResp struct {
	ID             uint      `json:"id"`
	BeautyCenterID uint      `json:"beauty_center_id"`
	ImageUrl       string    `json:"image_url"`
	IsCover        bool      `json:"is_cover"`
	IsProfile      bool      `json:"is_profile"`
	Position       int       `json:"position"`
	CreatedAt      time.Time `json:"created_at"`
}

type WorkingHourResp struct {
	ID        uint   `json:"id"`
	DayOfWeek int    `json:"day_of_week"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type SocialMediaResp struct {
	Google    string `json:"google"`
	Facebook  string `json:"facebook"`
	Instagram string `json:"instagram"`
	Twitter   string `json:"twitter"`
	TikTok    string `json:"tiktok"`
}

type AvailableSlot struct {
	Time      string `json:"time"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type AvailableSlotsResponse struct {
	Date     string          `json:"date"`
	Slots    []AvailableSlot `json:"slots"`
	Currency string          `json:"currency"`
}
