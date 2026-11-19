package service

import (
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type VideoWatchEventServiceImpl struct {
	WatchEventRepo repository.VideoWatchEventRepository
	FeedPostRepo   repository.FeedPostRepository
	PostLikeRepo   repository.PostLikeRepository
	SavedVideoRepo repository.SavedVideoRepository
	ImpressionRepo repository.FeedImpressionRepository
	Validate       *validator.Validate
}

func NewVideoWatchEventServiceImpl(
	watchRepo repository.VideoWatchEventRepository,
	feedRepo repository.FeedPostRepository,
	likeRepo repository.PostLikeRepository,
	savedRepo repository.SavedVideoRepository,
	impressionRepo repository.FeedImpressionRepository,
	validate *validator.Validate,
) VideoWatchEventService {
	return &VideoWatchEventServiceImpl{
		WatchEventRepo: watchRepo,
		FeedPostRepo:   feedRepo,
		PostLikeRepo:   likeRepo,
		SavedVideoRepo: savedRepo,
		ImpressionRepo: impressionRepo,
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
// WatchedPostIDs implements VideoWatchEventService.
func (s *VideoWatchEventServiceImpl) WatchedPostIDs(clientID uint, postIDs []uint) (map[uint]bool, error) {
	return s.WatchEventRepo.WatchedPostIDs(clientID, postIDs)
}

func (s *VideoWatchEventServiceImpl) GetPersonalizedFeed(clientID uint, limit, offset int, viewer *GeoPoint, session string) ([]uint, int, error) {
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

	// Signals are DECAYED by age: yesterday's obsession outranks last season's.
	// Weights come from InteractionWeight, which can be NEGATIVE (a skip).
	signalAge := func(createdAt time.Time) time.Duration {
		return time.Since(createdAt)
	}
	addSignal := func(post models.FeedPost, baseWeight float64, age time.Duration) {
		weight := baseWeight * AffinityDecay(age)
		if post.ServiceID != nil && post.Service.CategoryID != 0 {
			categoryScore[post.Service.CategoryID] += weight
		}
		centerScore[post.BeautyCenterID] += weight
		if post.PersonnelID != nil {
			personnelScore[*post.PersonnelID] += weight
		}
	}

	// From watch events.
	for _, w := range watchEvents {
		post, ok := postByID[w.FeedPostID]
		if !ok {
			continue
		}
		addSignal(post, InteractionWeight(w.Completed, w.WatchPercentage), signalAge(w.CreatedAt))
	}

	// From likes.
	for _, like := range likes {
		post, ok := postByID[like.FeedPostID]
		if !ok {
			continue
		}
		addSignal(post, LikeWeight, signalAge(like.CreatedAt))
	}

	// From saved videos — the strongest explicit signal a client can give.
	for _, sv := range savedVideos {
		post, ok := postByID[sv.FeedPostID]
		if !ok {
			continue
		}
		addSignal(post, SaveWeight, signalAge(sv.CreatedAt))
	}

	// 3. Candidate pool: geographically bounded when we know where the viewer is,
	// capped either way. Loading the whole catalog to score it was the most
	// expensive thing this endpoint did.
	allPosts, err := s.feedCandidates(viewer)
	if err != nil {
		return nil, 0, err
	}

	// 4. Exclude already-watched posts.
	//
	// The exclusion must be COMPLETE, so it is asked of the database for exactly
	// the candidate set. Deriving it from the signal list (capped at the last 200
	// events) meant a client who watched more than 200 videos started seeing the
	// oldest ones again — the opposite of "content you have not already seen".
	candidateIDs := make([]uint, 0, len(allPosts))
	for _, p := range allPosts {
		candidateIDs = append(candidateIDs, p.ID)
	}
	watched, watchedErr := s.WatchEventRepo.WatchedPostIDs(clientID, candidateIDs)
	if watchedErr != nil {
		// Degrade to "nothing was watched": repeats beat an empty feed.
		watched = map[uint]bool{}
	}

	// 5. Score each candidate: affinity + engagement + recency + a bounded
	// exploration jitter, then rank with a same-center diversity cap.
	now := time.Now()
	// STABLE seed: the same client in the same session must get the same order on
	// every request, or page 2 duplicates page 1 and a refresh reshuffles under
	// the user's thumb.
	seed := rankingSeed(clientID, session)

	buildCandidates := func(applyExclusion bool) []RankCandidate {
		out := make([]RankCandidate, 0, len(allPosts))
		for _, post := range allPosts {
			if !post.IsActive {
				continue
			}
			if applyExclusion && watched[post.ID] {
				continue
			}

			score := 0.0

			// Affinity, SATURATED per dimension: unbounded sums let twenty
			// matching watches swamp everything else and collapse the feed into
			// one category.
			if post.ServiceID != nil && post.Service.CategoryID != 0 {
				score += Saturate(categoryScore[post.Service.CategoryID], CategoryAffinityCap)
			}
			score += Saturate(centerScore[post.BeautyCenterID], CenterAffinityCap)
			if post.PersonnelID != nil {
				score += Saturate(personnelScore[*post.PersonnelID], PersonnelAffinityCap)
			}

			// Quality (booking intent per view) plus a bounded reach prior,
			// then recency decay.
			score += QualityScore(post.ViewsCount, post.BookTaps)
			score += PopularityScore(post.ViewsCount)
			score += RecencyScore(post.CreatedAt, now)

			// NOTE: `is_promo` adds NOTHING to the score. It is a label ("this
			// post advertises a promotion"), not a placement: the only thing that
			// changes a position is the paid auction. Giving the flag even a small
			// nudge made a free checkbox worth more than the product we sell.

			// Proximity: content the viewer could actually visit outranks content
			// from another city. Unknown distance (no viewer position, or a centre
			// without coordinates) contributes 0 — never a penalty.
			if viewer.Valid() {
				center := post.BeautyCenter
				if center.Latitude != 0 || center.Longitude != 0 {
					km := HaversineKM(viewer.Latitude, viewer.Longitude, center.Latitude, center.Longitude)
					score += ProximityScore(&km)
				}
			}

			// Exploration: a bounded, seed-deterministic jitter that reshuffles
			// near-ties so the feed never looks frozen in its DB order.
			score += ExplorationScale * ExplorationScore(seed, post.ID)

			categoryID := uint(0)
			if post.ServiceID != nil {
				categoryID = post.Service.CategoryID
			}
			out = append(out, RankCandidate{
				ID:         post.ID,
				CenterID:   post.BeautyCenterID,
				CategoryID: categoryID,
				Score:      score,
			})
		}
		return out
	}

	candidates := buildCandidates(true)
	// Never ship an empty feed just because the client has watched everything:
	// fall back to the unexcluded ranking so the endpoint still returns content.
	if len(candidates) == 0 {
		candidates = buildCandidates(false)
	}

	orderedIDs := RankByScore(candidates, MaxSameCenterRun, MaxSameCategoryRun)

	// 6. Slice the requested window out of the SAME stable ranking. Pagination is
	// a cursor over one list, not a jump to a different feed.
	total := len(orderedIDs)
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []uint{}, total, nil
	}
	if limit <= 0 || offset+limit > total {
		limit = total - offset
	}
	return orderedIDs[offset : offset+limit], total, nil
}

// rankingSeed derives a deterministic seed for one client session.
//
// With a session id, the order is reproducible for as long as the session lasts
// (pagination and re-opens stay coherent, and an A/B test can attribute results).
// Without one, it falls back to a 10-minute bucket: stable enough for a scroll,
// changing enough that a client who never sends a session still sees variety.
func rankingSeed(clientID uint, session string) int64 {
	if session != "" {
		h := fnv.New64a()
		_, _ = fmt.Fprintf(h, "%d:%s", clientID, session)
		return int64(h.Sum64())
	}
	bucket := time.Now().Unix() / 600
	return int64(clientID)*1_000_003 + bucket
}

// feedCandidates is the ranking's candidate pool.
//
// Constants: a 120 km box covers a city plus its metropolitan ring — beyond that
// the proximity score is already ~0, so shipping those rows would only cost
// bandwidth. 300 candidates is more than a feed page can ever show while keeping
// the per-request work bounded.
func (s *VideoWatchEventServiceImpl) feedCandidates(viewer *GeoPoint) ([]models.FeedPost, error) {
	const (
		candidateRadiusKm = 120.0
		candidateLimit    = 300
	)
	if viewer.Valid() {
		return s.FeedPostRepo.FindActiveForFeed(viewer.Latitude, viewer.Longitude, candidateRadiusKm, candidateLimit)
	}
	posts, _, err := s.FeedPostRepo.FindAllActive(1, candidateLimit)
	return posts, err
}

// RecordImpressions implements VideoWatchEventService.
func (s *VideoWatchEventServiceImpl) RecordImpressions(clientID uint, rows []ImpressionRow) error {
	if s.ImpressionRepo == nil || len(rows) == 0 {
		return nil
	}
	records := make([]models.FeedImpression, 0, len(rows))
	for _, row := range rows {
		records = append(records, models.FeedImpression{
			PostID:    row.PostID,
			ClientID:  clientID,
			Position:  row.Position,
			SessionID: row.SessionID,
		})
	}
	return s.ImpressionRepo.RecordInBatches(records)
}

// PositionMetrics implements VideoWatchEventService.
func (s *VideoWatchEventServiceImpl) PositionMetrics(days int) ([]PositionMetric, error) {
	if s.ImpressionRepo == nil {
		return []PositionMetric{}, nil
	}
	if days <= 0 {
		days = 7
	}
	rows, err := s.ImpressionRepo.PositionMetrics(time.Now().AddDate(0, 0, -days))
	if err != nil {
		return nil, err
	}
	out := make([]PositionMetric, 0, len(rows))
	for _, row := range rows {
		metric := PositionMetric{
			Position:    row.Position,
			Impressions: row.Impressions,
			Engaged:     row.Engaged,
		}
		if row.Impressions > 0 {
			metric.Rate = float64(row.Engaged) / float64(row.Impressions)
		}
		out = append(out, metric)
	}
	return out, nil
}
