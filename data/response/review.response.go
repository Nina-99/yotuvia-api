package response

import "time"

type ReviewResponse struct {
	ID             uint                 `json:"id"`
	Rating         int                  `json:"rating"`
	Comment        string               `json:"comment"`
	CreatedAt      time.Time            `json:"created_at"`
	BeautyCenterID uint                 `json:"beauty_center_id"`
	BeautyCenter   BeautyCenterResponse `json:"beauty_center"`
	ClientID       uint                 `json:"client_id"`
	Client         ClientResponse       `json:"client"`
}
