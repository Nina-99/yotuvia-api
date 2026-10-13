package response

import (
	"time"

	"github.com/Nina-99/yotuvia-api/utils"
)

type BeautyCenterResponse struct {
	Currency              string                  `json:"currency"`
	ID                    uint                    `json:"id"`
	Name                  string                  `json:"name"`
	Address               string                  `json:"address"`
	Email                 string                  `json:"email"`
	Phone                 string                  `json:"phone"`
	Url                   string                  `json:"url"`
	Description           string                  `json:"description"`
	NameI18n              map[string]string       `json:"name_i18n,omitempty"`
	DescriptionI18n       map[string]string       `json:"description_i18n,omitempty"`
	IsActive              bool                    `json:"is_active"`
	Latitude              float64                 `json:"latitude"`
	Longitude             float64                 `json:"longitude"`
	AvgRating             float64                 `json:"avg_rating"`
	ReviewCount           int64                   `json:"review_count"`
	DepositRequired       bool                    `json:"deposit_required"`
	DepositAmount         float64                 `json:"deposit_amount"`
	DepositPercent        float64                 `json:"deposit_percent"`
	DepositTrustThreshold int                     `json:"deposit_trust_threshold"`
	DepositHoldMinutes    int                     `json:"deposit_hold_minutes"`
	DepositQRURL          string                  `json:"deposit_qr_url"`
	Images                []BeautyCenterImageResp `json:"images"`
	Schedules             []WorkingHourResp       `json:"schedules"`
	SocialMedia           *SocialMediaResp        `json:"social_media,omitempty"`
}

// Localize swaps Name/Description for the requested language using the stored
// per-locale maps, falling back to the base value. Raw maps are kept so owner
// edit forms can prefill every language.
func (r *BeautyCenterResponse) Localize(lang string) {
	r.Name = utils.LocalizedValue(r.Name, r.NameI18n, lang)
	r.Description = utils.LocalizedValue(r.Description, r.DescriptionI18n, lang)
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

// RankingResponse is one rank of the beauty-center leaderboard.
type RankingResponse struct {
	Position       int                     `json:"position"`
	CenterID       uint                    `json:"center_id"`
	CenterName     string                  `json:"center_name"`
	Address        string                  `json:"address"`
	AvgRating      float64                 `json:"avg_rating"`
	ReviewCount    int64                   `json:"review_count"`
	ReservationCnt int64                   `json:"reservation_count"`
	CompletionRate float64                 `json:"completion_rate"`
	VideoViews     int64                   `json:"video_views"`
	VideoLikes     int64                   `json:"video_likes"`
	FavoritesCount int64                   `json:"favorites_count"`
	Score          float64                 `json:"score"`
	Images         []BeautyCenterImageResp `json:"images"`
}
