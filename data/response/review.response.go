package response

import "time"

// ReviewClientResponse is the trimmed client representation embedded in review
// responses. Reviews are publicly readable for any center, so only the display
// identity (name + avatar) is exposed. email, phone and username are
// deliberately absent: returning them let anyone enumerate review/center ids
// and harvest clients' PII. Contrast with ClientResponse, which keeps the full
// profile for the authenticated owner and admin client lists.
type ReviewClientResponse struct {
	ID        uint   `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	AvatarURL string `json:"avatar_url"`
}

type ReviewResponse struct {
	ID             uint                 `json:"id"`
	Rating         int                  `json:"rating"`
	Comment        string               `json:"comment"`
	CreatedAt      time.Time            `json:"created_at"`
	BeautyCenterID uint                 `json:"beauty_center_id"`
	BeautyCenter   BeautyCenterResponse `json:"beauty_center"`
	ClientID       uint                 `json:"client_id"`
	Client         ReviewClientResponse `json:"client"`
}
