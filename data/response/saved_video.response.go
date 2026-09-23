package response

import "time"

type SavedVideoResponse struct {
	ID         uint      `json:"id"`
	FeedPostID uint      `json:"feed_post_id"`
	ClientID   uint      `json:"client_id"`
	CreatedAt  time.Time `json:"created_at"`
}
