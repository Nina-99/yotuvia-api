package service

import "github.com/Nina-99/yotuvia-api/data/request"

type VideoWatchEventService interface {
	/** Record a watch event for a client. */
	RecordWatch(clientID uint, req request.RecordWatchRequest) error
	/** Get personalized video IDs ranked by relevance. Returns feed_post IDs. */
	// GetPersonalizedFeed ranks the client's candidates. `viewer` is optional:
	// with real coordinates nearby content outranks content from other cities;
	// without them the ranking is the affinity/recency one (never worse).
	// RecordImpressions persists the served page for evaluation. Bounded and
	// best-effort: a logging failure must never fail the feed.
	RecordImpressions(clientID uint, rows []ImpressionRow) error

	// PositionMetrics returns engagement per slot for the last `days` days: the
	// baseline every future ranking change is measured against.
	PositionMetrics(days int) ([]PositionMetric, error)

	// WatchedPostIDs reports which of the given posts this client already watched,
	// so the ad server can avoid re-serving a consumed video.
	WatchedPostIDs(clientID uint, postIDs []uint) (map[uint]bool, error)

	// GetPersonalizedFeed ranks the client's candidates and returns the window
	// [offset, offset+limit). The ranking must be STABLE across requests in one
	// session or pagination would duplicate and skip items — that is what the
	// session seed is for.
	//
	// Returns the page ids and the total number of ranked candidates.
	GetPersonalizedFeed(clientID uint, limit, offset int, viewer *GeoPoint, session string) (ids []uint, total int, err error)
}

// ImpressionRow is one served post.
type ImpressionRow struct {
	PostID    uint
	Position  int
	SessionID string
}

// PositionMetric is one row of the CTR-by-position report.
type PositionMetric struct {
	Position    int     `json:"position"`
	Impressions int64   `json:"impressions"`
	Engaged     int64   `json:"engaged"`
	Rate        float64 `json:"rate"`
}
