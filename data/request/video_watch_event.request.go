package request

type RecordWatchRequest struct {
	FeedPostID      uint    `json:"feed_post_id" binding:"required"`
	WatchDuration   int     `json:"watch_duration"`
	WatchPercentage float64 `json:"watch_percentage"`
	Completed       bool    `json:"completed"`
	// ImpressionToken is the signed token carried by a boosted post in the feed.
	// When present and the view qualifies, it bills that impression exactly once.
	ImpressionToken string `json:"impression_token"`
}
