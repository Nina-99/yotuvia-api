package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/go-playground/validator/v10"
)

type VideoWatchEventServiceImpl struct {
	WatchEventRepo    repository.VideoWatchEventRepository
	FeedPostRepo      repository.FeedPostRepository
	PostLikeRepo      repository.PostLikeRepository
	SavedVideoRepo    repository.SavedVideoRepository
	Validate          *validator.Validate
}

func NewVideoWatchEventServiceImpl(
	watchRepo repository.VideoWatchEventRepository,
	feedRepo repository.FeedPostRepository,
	likeRepo repository.PostLikeRepository,
	savedRepo repository.SavedVideoRepository,
	validate *validator.Validate,
) VideoWatchEventService {
	return &VideoWatchEventServiceImpl{
		WatchEventRepo: watchRepo,
		FeedPostRepo:   feedRepo,
		PostLikeRepo:   likeRepo,
		SavedVideoRepo: savedRepo,
		Validate:       validate,
	}
}

func (s *VideoWatchEventServiceImpl) RecordWatch(clientID uint, req request.RecordWatchRequest) error {
	event := models.VideoWatchEvent{
		ClientID:        clientID,
		FeedPostID:      req.FeedPostID,
		WatchDuration:   req.WatchDuration,
		WatchPercentage: req.WatchPercentage,
		Completed:       req.Completed,
	}
	return s.WatchEventRepo.Create(&event)
}

/**
 * GetPersonalizedFeed computes a personalized ranking of videos for a client.
 *
 * Algorithm:
 * 1. Build user preference map from watch events, likes, and saves
 * 2. Score each candidate video against user preferences
 * 3. Sort by score descending
 * 4. Return top N IDs
 *
 * Preference weights:
 * - Watch completed: 3 points for that category/center/personnel
 * - Watched > 50%:  2 points
 * - Liked:           4 points
 * - Saved:           5 points
 */
func (s *VideoWatchEventServiceImpl) GetPersonalizedFeed(clientID uint, limit int, excludeIDs []uint) ([]uint, error) {
	// 1. Gather user signals
	watchEvents, _ := s.WatchEventRepo.FindByClientID(clientID)
	likes, _ := s.PostLikeRepo.FindByClientID(clientID)
	savedVideos, _ := s.SavedVideoRepo.FindByClientID(clientID)

	// Resolve every referenced post in ONE query. The previous code called
	// FindByID once per watch event, per like and per save (each running 5
	// preloads), then looked the same post up repeatedly. Fetch the union of
	// ids once and index it. FindByIDs keeps FindByID's semantics: it does NOT
	// filter is_active, so inactive posts still contribute to affinity exactly
	// as they did before.
	idSet := make(map[uint]struct{}, len(watchEvents)+len(likes)+len(savedVideos))
	for _, w := range watchEvents {
		idSet[w.FeedPostID] = struct{}{}
	}
	for _, l := range likes {
		idSet[l.FeedPostID] = struct{}{}
	}
	for _, sv := range savedVideos {
		idSet[sv.FeedPostID] = struct{}{}
	}
	ids := make([]uint, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	postByID := make(map[uint]models.FeedPost, len(ids))
	if posts, err := s.FeedPostRepo.FindByIDs(ids); err == nil {
		for _, p := range posts {
			postByID[p.ID] = p
		}
	}

	// 2. Build preference scores
	categoryScore := make(map[uint]float64)
	centerScore := make(map[uint]float64)
	personnelScore := make(map[uint]float64)

	// From watch events
	for _, w := range watchEvents {
		post, ok := postByID[w.FeedPostID]
		if !ok {
			continue
		}
		var weight float64
		if w.Completed {
			weight = 3.0
		} else if w.WatchPercentage >= 50 {
			weight = 2.0
		} else {
			weight = 1.0
		}
		if post.ServiceID != nil && post.Service.CategoryID != 0 {
			categoryScore[post.Service.CategoryID] += weight
		}
		centerScore[post.BeautyCenterID] += weight
		if post.PersonnelID != nil {
			personnelScore[*post.PersonnelID] += weight
		}
	}

	// From likes
	for _, like := range likes {
		post, ok := postByID[like.FeedPostID]
		if !ok {
			continue
		}
		if post.ServiceID != nil && post.Service.CategoryID != 0 {
			categoryScore[post.Service.CategoryID] += 4.0
		}
		centerScore[post.BeautyCenterID] += 4.0
		if post.PersonnelID != nil {
			personnelScore[*post.PersonnelID] += 4.0
		}
	}

	// From saved videos
	for _, sv := range savedVideos {
		post, ok := postByID[sv.FeedPostID]
		if !ok {
			continue
		}
		if post.ServiceID != nil && post.Service.CategoryID != 0 {
			categoryScore[post.Service.CategoryID] += 5.0
		}
		centerScore[post.BeautyCenterID] += 5.0
		if post.PersonnelID != nil {
			personnelScore[*post.PersonnelID] += 5.0
		}
	}

	// 3. Get all active posts
	allPosts, _, err := s.FeedPostRepo.FindAllActive(1, 1000)
	if err != nil {
		return nil, err
	}

	// 4. Exclude already-seen posts
	excludeSet := make(map[uint]bool, len(excludeIDs))
	for _, id := range excludeIDs {
		excludeSet[id] = true
	}

	// 5. Score each post
	type scored struct {
		ID    uint
		Score float64
	}
	var scoredPosts []scored

	for _, post := range allPosts {
		if excludeSet[post.ID] {
			continue
		}
		if !post.IsActive {
			continue
		}

		score := 0.0

		// Category affinity
		if post.ServiceID != nil && post.Service.CategoryID != 0 {
			score += categoryScore[post.Service.CategoryID]
		}
		// Center affinity
		score += centerScore[post.BeautyCenterID]
		// Personnel affinity
		if post.PersonnelID != nil {
			score += personnelScore[*post.PersonnelID]
		}

		// Boost promos slightly
		if post.IsPromo {
			score += 0.5
		}

		// Boost recent content
		// (Recency is handled by the DB ordering, but we add a small tiebreaker)

		scoredPosts = append(scoredPosts, scored{ID: post.ID, Score: score})
	}

	// 6. Sort by score descending (simple bubble sort for small N)
	for i := 0; i < len(scoredPosts); i++ {
		for j := i + 1; j < len(scoredPosts); j++ {
			if scoredPosts[j].Score > scoredPosts[i].Score {
				scoredPosts[i], scoredPosts[j] = scoredPosts[j], scoredPosts[i]
			}
		}
	}

	// 7. Return top N IDs
	result := make([]uint, 0, limit)
	for i := 0; i < len(scoredPosts) && i < limit; i++ {
		result = append(result, scoredPosts[i].ID)
	}

	return result, nil
}
