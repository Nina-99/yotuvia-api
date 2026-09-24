package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
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

// RecordWatch records a watch event for a client. It is idempotent per
// (client_id, feed_post_id): the mobile client reports on every pause and
// visibility change, so inserting a fresh row per call grew the table without
// bound and amplified the affinity scoring in GetPersonalizedFeed.
//
// The merge keeps both scored fields monotonic: WatchPercentage and
// WatchDuration take the maximum seen, and Completed latches to true and is
// never downgraded. That guarantees re-sending a shorter/partial event cannot
// lower a client's affinity for the post.
//
// Implementation note (migration safety): this is a code-only upsert with NO
// new unique index. A unique index on (client_id, feed_post_id) would make
// AutoMigrate fail on an existing database that already holds duplicate rows.
// The read-then-write is race-tolerant rather than perfectly atomic: under a
// concurrent double-insert the pair may briefly hold two rows, but the merge
// still converges (each row stays monotonic) and no schema change is required.
func (s *VideoWatchEventServiceImpl) RecordWatch(clientID uint, req request.RecordWatchRequest) error {
	existing, err := s.WatchEventRepo.FindByClientAndPost(clientID, req.FeedPostID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if existing != nil {
		changed := false
		if req.WatchPercentage > existing.WatchPercentage {
			existing.WatchPercentage = req.WatchPercentage
			changed = true
		}
		if req.WatchDuration > existing.WatchDuration {
			existing.WatchDuration = req.WatchDuration
			changed = true
		}
		if req.Completed && !existing.Completed {
			existing.Completed = true
			changed = true
		}
		if !changed {
			return nil
		}
		return s.WatchEventRepo.Update(existing)
	}

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
 *
 * Exclusion: the client's own watch events are excluded from the ranked
 * result, so the feed surfaces content the client has not already seen. The
 * previous signature carried an `excludeIDs` parameter whose sole caller always
 * passed nil (dead contract); exclusion is now derived internally from the
 * watch events this function already fetches. If EVERY active post has been
 * watched, we fall back to the unexcluded ranking rather than returning an
 * empty feed.
 */
func (s *VideoWatchEventServiceImpl) GetPersonalizedFeed(clientID uint, limit int) ([]uint, error) {
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

	// 4. Exclude already-watched posts. The client's watch events are the
	// exclusion signal: the personalized feed should surface content the client
	// has not already seen. (The old `excludeIDs` parameter was dead — the only
	// caller passed nil — so exclusion is derived here from the same events.)
	watched := make(map[uint]bool, len(watchEvents))
	for _, w := range watchEvents {
		watched[w.FeedPostID] = true
	}

	// 5. Score each post
	type scored struct {
		ID    uint
		Score float64
	}

	buildScored := func(applyExclusion bool) []scored {
		var out []scored
		for _, post := range allPosts {
			if !post.IsActive {
				continue
			}
			if applyExclusion && watched[post.ID] {
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

			out = append(out, scored{ID: post.ID, Score: score})
		}
		return out
	}

	scoredPosts := buildScored(true)
	// Never ship an empty feed just because the client has watched everything:
	// fall back to the unexcluded ranking so the endpoint still returns content.
	if len(scoredPosts) == 0 {
		scoredPosts = buildScored(false)
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
