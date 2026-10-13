package request

import "time"

// CreateBoostCampaignRequest creates a CPM boost for one of the center's OWN
// feed posts. `Budget` and `BidCPM` are in tokens (the controller converts them
// to milli-tokens); `BidCPM` is the maximum paid per 1000 impressions.
type CreateBoostCampaignRequest struct {
	FeedPostID uint      `json:"feed_post_id" binding:"required"`
	Budget     float64   `json:"budget" binding:"required,gt=0"`
	BidCPM     float64   `json:"bid_cpm" binding:"required,gt=0"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at" binding:"required"`
}
