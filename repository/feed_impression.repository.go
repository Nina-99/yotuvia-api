package repository

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
)

// FeedPositionMetric is one row of the CTR-by-position report.
type FeedPositionMetric struct {
	Position    int
	Impressions int64
	Engaged     int64
}

type FeedImpressionRepository interface {
	// RecordInBatches persists served impressions. Batch insert on purpose: one
	// statement per page, never one per row.
	RecordInBatches(rows []models.FeedImpression) error

	// PositionMetrics aggregates, per position, how many impressions were served
	// and how many produced a watch. The join is time-aware: an impression only
	// counts as engaged when the view arrived AFTER it was served.
	PositionMetrics(since time.Time) ([]FeedPositionMetric, error)
}
