package service

import "github.com/Nina-99/yottu-api/data/request"

type VideoWatchEventService interface {
	/** Record a watch event for a client. */
	RecordWatch(clientID uint, req request.RecordWatchRequest) error
	/** Get personalized video IDs ranked by relevance. Returns feed_post IDs. */
	GetPersonalizedFeed(clientID uint, limit int) ([]uint, error)
}
