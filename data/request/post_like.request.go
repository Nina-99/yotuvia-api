package request

type CreatePostLikeRequest struct {
	FeedPostID uint `json:"feed_post_id" binding:"required"`
}
