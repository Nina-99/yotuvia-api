package repository

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type FeedImpressionRepositoryImpl struct {
	db *gorm.DB
}

func NewFeedImpressionRepositoryImpl(db *gorm.DB) FeedImpressionRepository {
	return &FeedImpressionRepositoryImpl{db: db}
}

func (r *FeedImpressionRepositoryImpl) RecordInBatches(rows []models.FeedImpression) error {
	if len(rows) == 0 {
		return nil
	}
	return r.db.CreateInBatches(rows, 100).Error
}

// PositionMetrics counts, per position, served impressions and how many of them
// led to a watch.
//
// The LEFT JOIN is time-aware (`w.created_at >= i.created_at`): a view from
// before the impression must not be credited to it. RecordWatch upserts one row
// per (client, post), so this join cannot fan out.
func (r *FeedImpressionRepositoryImpl) PositionMetrics(since time.Time) ([]FeedPositionMetric, error) {
	var rows []FeedPositionMetric
	err := r.db.Model(&models.FeedImpression{}).
		Select("feed_impressions.position AS position, "+
			"COUNT(*) AS impressions, "+
			"COUNT(w.id) AS engaged").
		Joins(`LEFT JOIN video_watch_events w
			ON w.feed_post_id = feed_impressions.post_id
			AND w.client_id = feed_impressions.client_id
			AND w.created_at >= feed_impressions.created_at
			AND w.deleted_at IS NULL`).
		Where("feed_impressions.created_at >= ?", since).
		Group("feed_impressions.position").
		Order("feed_impressions.position ASC").
		Scan(&rows).Error
	return rows, err
}
