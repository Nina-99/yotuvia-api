package response

import "time"

type FeedPostResponse struct {
	ID             uint                 `json:"id"`
	BeautyCenterID uint                 `json:"beauty_center_id"`
	BeautyCenter   BeautyCenterResponse `json:"beauty_center,omitempty"`
	PersonnelID    *uint                `json:"personnel_id,omitempty"`
	ServiceID      *uint                `json:"service_id,omitempty"`
	VideoURL       string               `json:"video_url"`
	ThumbnailURL   string               `json:"thumbnail_url"`
	Caption        string               `json:"caption"`
	Duration       int                  `json:"duration"`
	ViewsCount     int                  `json:"views_count"`
	LikesCount     int                  `json:"likes_count"`
	// BookTaps is the CTA tap counter (additive; mirrors views_count).
	BookTaps  int       `json:"book_taps"`
	CreatedAt time.Time `json:"created_at"`

	// feat-video-feed-unification
	IsPromo        bool   `json:"is_promo"`
	ModerationNote string `json:"moderation_note,omitempty"`
	IsActive       bool   `json:"is_active"`
	ServiceName    string `json:"service_name,omitempty"`
	CategoryID     uint   `json:"category_id,omitempty"`
	CategoryName   string `json:"category_name,omitempty"`
	PersonnelName  string `json:"personnel_name,omitempty"`

	// Offer surfacing (shop the look): the tagged service's price and duration,
	// so the book CTA is concrete. Both are POINTERS with omitempty so a free
	// service (price 0) is distinguishable from "no service tagged" (nil, which
	// omits the key entirely). Populated from the already-preloaded post.Service;
	// nil whenever that association is not loaded — never a per-row query.
	ServicePrice    *float64 `json:"service_price,omitempty"`
	ServiceDuration *int     `json:"service_duration,omitempty"`

	// User interaction state (populated when clientID is available)
	IsLiked       bool `json:"is_liked"`
	IsSaved       bool `json:"is_saved"`
	CommentsCount int  `json:"comments_count"`

	// IsBoosted marks a post served through the CPM auction (a paid promoted
	// slot). Additive: existing clients ignore it, and the feed must always
	// label a true value as "Promocionado" wherever it renders.
	IsBoosted bool `json:"is_boosted"`
}
