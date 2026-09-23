package request

type RecordWatchRequest struct {
	FeedPostID      uint    `json:"feed_post_id" binding:"required"`
	WatchDuration   int     `json:"watch_duration"`
	WatchPercentage float64 `json:"watch_percentage"`
	Completed       bool    `json:"completed"`
}
