package service

import (
	"math"
	"sort"
	"time"
)

// Feed ranking knobs. Kept as named constants so the ranking can be tuned
// without touching the call sites (see GetPersonalizedFeed).
const (
	// RecencyHalfLifeHours is the age at which a post's recency score halves.
	RecencyHalfLifeHours = 72.0

	// Engagement weights: a booking-intent tap means far more than a view.
	EngagementViewWeight = 0.5
	EngagementBookWeight = 3.0

	// ExplorationScale bounds the per-post random jitter added to a score. It is
	// small enough never to promote junk above a strongly relevant post, but big
	// enough to reshuffle near-ties and keep the feed from looking frozen.
	ExplorationScale = 0.75

	// PromoRankBoost nudges promoted posts up, mirroring the old +0.5.
	PromoRankBoost = 0.5

	// MaxSameCenterRun caps how many consecutive posts one center may occupy, so
	// a single prolific center cannot monopolise the top of the feed.
	MaxSameCenterRun = 2
)

// RecencyScore is a smooth 0..1 decay: 1.0 for a brand-new post, 0.5 at one
// half-life and → 0 for old posts. Future timestamps clamp to 1.0.
func RecencyScore(createdAt, now time.Time) float64 {
	ageHours := now.Sub(createdAt).Hours()
	if ageHours < 0 {
		ageHours = 0
	}
	return math.Pow(0.5, ageHours/RecencyHalfLifeHours)
}

// EngagementScore turns the public counters into one bounded "how much traction
// did this get" signal. log1p keeps a viral outlier from swamping the ranking.
func EngagementScore(views, bookTaps int) float64 {
	return math.Log1p(float64(views))*EngagementViewWeight +
		math.Log1p(float64(bookTaps))*EngagementBookWeight
}

// ExplorationScore is a deterministic pseudo-random value in [0,1) derived from
// a session seed and the post id. Deterministic per (seed, id) means a single
// request is reproducible, while a new seed (e.g. per session) reshuffles the
// feed — variety without `ORDER BY RANDOM()` breaking pagination.
func ExplorationScore(seed int64, postID uint) float64 {
	h := uint64(seed)*2654435761 + uint64(postID)*40503
	h ^= h >> 15
	h *= 2246822519
	h ^= h >> 13
	return float64(h%100000) / 100000.0
}

// RankCandidate is one post competing for a position in a ranked feed.
type RankCandidate struct {
	ID       uint
	CenterID uint
	Score    float64
}

// RankByScore returns post ids ordered by descending score, then applies a
// diversity pass that caps consecutive posts from the same center at
// `maxSameCenterRun`. Ties break by ascending id so the order is stable.
//
// The diversity pass is greedy: at each step it takes the highest-scoring post
// whose center is not already at the run limit, and only relaxes that rule when
// every remaining post would violate it (so a feed never truncates).
func RankByScore(candidates []RankCandidate, maxSameCenterRun int) []uint {
	ordered := make([]RankCandidate, len(candidates))
	copy(ordered, candidates)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Score != ordered[j].Score {
			return ordered[i].Score > ordered[j].Score
		}
		return ordered[i].ID < ordered[j].ID
	})

	out := make([]uint, 0, len(ordered))
	used := make([]bool, len(ordered))
	runCenter := uint(0)
	runCount := 0

	for len(out) < len(ordered) {
		picked := -1
		for i := range ordered {
			if used[i] {
				continue
			}
			sameRun := runCount > 0 && ordered[i].CenterID == runCenter
			if sameRun && runCount >= maxSameCenterRun {
				continue
			}
			picked = i
			break
		}
		if picked == -1 {
			// Every remaining post would exceed the run cap: relax and take the
			// next best so the feed is never shorter than its input.
			for i := range ordered {
				if !used[i] {
					picked = i
					break
				}
			}
		}
		if picked == -1 {
			break
		}
		used[picked] = true
		if runCount > 0 && ordered[picked].CenterID == runCenter {
			runCount++
		} else {
			runCenter = ordered[picked].CenterID
			runCount = 1
		}
		out = append(out, ordered[picked].ID)
	}

	return out
}
