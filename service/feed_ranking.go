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

	// Quality: booking intent per VIEW, not per post. Views are incremented when
	// a slide is shown, so an absolute view count measures exposure, not interest
	// — using it as quality made the first post to be shown win forever.
	QualityScale = 4.0
	// QualityReferenceRatio is the tap-per-view ratio that saturates the scale:
	// 10% of viewers tapping "book" is excellent, 1% is weak.
	QualityReferenceRatio = 0.10
	// QualityPrior* smooth tiny samples and encode the BASELINE: a post with no
	// data at all is assumed to tap at 1% (a tenth of the reference), so unknown
	// quality scores low instead of looking excellent. A generous prior (1 tap of
	// 20) made an unviewed post score half the maximum.
	QualityPriorTaps  = 2.0
	QualityPriorViews = 200.0

	// Popularity: a small prior so proven reach still matters, bounded so it can
	// never outrank relevance on its own.
	PopularityWeight = 0.25
	PopularityCap    = 1.5

	// Affinity caps. Affinity used to accumulate without a ceiling: twenty
	// completed watches scored 60 points against ~1 for recency, so the feed
	// collapsed into the user's dominant category. These saturate instead, and
	// together they top out around the weight of proximity + quality.
	CategoryAffinityCap  = 3.0
	CenterAffinityCap    = 2.5
	PersonnelAffinityCap = 2.0

	// AffinityHalfLifeDays decays each interaction's weight: a six-month-old
	// obsession must not weigh the same as yesterday's.
	AffinityHalfLifeDays = 30.0

	// SkipWeight is the weight of a video abandoned almost immediately. It is
	// NEGATIVE: "I closed this" is evidence against, and coding it as +1 (as the
	// old code did) taught the ranker to show more of it.
	SkipWeight      = -2.0
	GlanceWeight    = 0.5
	HalfWatchWeight = 2.0
	CompletedWeight = 3.0
	LikeWeight      = 4.0
	SaveWeight      = 5.0
	// SkipThresholdPct: below this percentage the view counts as abandoned.
	SkipThresholdPct = 10.0

	// ExplorationScale bounds the per-post random jitter added to a score. It is
	// small enough never to promote junk above a strongly relevant post, but big
	// enough to reshuffle near-ties and keep the feed from looking frozen.
	ExplorationScale = 0.75

	// ProximityScale is the maximum points a post gains for being in the
	// viewer's city. It is deliberately comparable to ONE interaction (category
	// affinity adds 3-5): local content should win ties and break into the top
	// without burying strong personal-bests from elsewhere.
	ProximityScale = 6.0

	// ProximityHalfLifeKm is the distance at which the proximity bonus halves:
	// 6 points in the same neighbourhood, 3 at 25 km, 1.5 at 50 km, ~0 at 100+.
	ProximityHalfLifeKm = 25.0

	// MaxSameCenterRun caps how many consecutive posts one center may occupy, so
	// a single prolific center cannot monopolise the top of the feed.
	MaxSameCenterRun = 2

	// MaxSameCategoryRun caps consecutive posts from the same category. Center
	// diversity alone let one TOPIC monopolise the feed: three different salons
	// posting haircuts still produced a wall of haircuts.
	MaxSameCategoryRun = 3
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

// QualityScore is booking intent per view, smoothed and bounded to [0, QualityScale].
//
// Why a ratio: `views` counts slides SHOWN, so a post displayed a thousand times
// has a thousand views no matter how boring it is. Ranking on the raw count
// rewards exposure with more exposure. The ratio answers the only question that
// matters: of the people who saw it, how many wanted to book?
func QualityScore(views, bookTaps int) float64 {
	ratio := (float64(bookTaps) + QualityPriorTaps) / (float64(views) + QualityPriorViews)
	scaled := ratio / QualityReferenceRatio
	if scaled > 1 {
		scaled = 1
	}
	return QualityScale * scaled
}

// PopularityScore is a bounded reach prior: proven content keeps a small edge,
// capped so it can never carry a post on its own.
func PopularityScore(views int) float64 {
	score := math.Log1p(float64(views)) * PopularityWeight
	if score > PopularityCap {
		return PopularityCap
	}
	return score
}

// Saturate compresses an unbounded sum into [0, cap] with diminishing returns:
// 0 -> 0, cap -> cap/2, and it never exceeds cap. Used for affinity, where the
// twentieth matching watch must not be worth as much as the second.
func Saturate(sum, limit float64) float64 {
	if limit <= 0 {
		return 0
	}
	magnitude := math.Abs(sum)
	compressed := limit * magnitude / (magnitude + limit)
	if sum < 0 {
		return -compressed
	}
	return compressed
}

// AffinityDecay is the per-interaction weight multiplier: 1.0 today, 0.5 after
// AffinityHalfLifeDays, and toward 0 for old history.
func AffinityDecay(age time.Duration) float64 {
	days := age.Hours() / 24
	if days < 0 {
		days = 0
	}
	return math.Pow(0.5, days/AffinityHalfLifeDays)
}

// InteractionWeight maps a watch event to its affinity weight. It can be
// NEGATIVE: abandoning a video within SkipThresholdPct is evidence against its
// category, not for it.
func InteractionWeight(completed bool, watchPercentage float64) float64 {
	switch {
	case completed:
		return CompletedWeight
	case watchPercentage >= 50:
		return HalfWatchWeight
	case watchPercentage >= SkipThresholdPct:
		return GlanceWeight
	default:
		return SkipWeight
	}
}

// ProximityScore is a smooth 0..ProximityScale decay with distance. It answers
// the only question a feed can honestly answer about location: "is this near
// enough that the person could actually go?". It is 0 for an unknown distance
// (no viewer position, or a centre that never set coordinates), so unset data
// never receives a bonus and never gets punished either.
func ProximityScore(distanceKm *float64) float64 {
	if distanceKm == nil || *distanceKm < 0 {
		return 0
	}
	return ProximityScale * math.Pow(0.5, *distanceKm/ProximityHalfLifeKm)
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
	ID         uint
	CenterID   uint
	CategoryID uint // 0 when the post has no service/category
	Score      float64
}

// RankByScore returns post ids ordered by descending score, then applies a
// diversity pass that caps consecutive posts from the same center at
// `maxSameCenterRun`. Ties break by ascending id so the order is stable.
//
// The diversity pass is greedy: at each step it takes the highest-scoring post
// whose center is not already at the run limit, and only relaxes that rule when
// every remaining post would violate it (so a feed never truncates).
func RankByScore(candidates []RankCandidate, maxSameCenterRun int, maxSameCategoryRun int) []uint {
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
	runCategory := uint(0)
	runCategoryCount := 0

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
			// Category cap. Only meaningful for candidates that HAVE a category:
			// two uncategorised posts are not "the same category" in any useful
			// sense, and capping them would needlessly reorder the feed.
			sameCategory := ordered[i].CategoryID != 0 &&
				runCategoryCount > 0 && ordered[i].CategoryID == runCategory
			if sameCategory && runCategoryCount >= maxSameCategoryRun {
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
		if ordered[picked].CategoryID != 0 {
			if runCategoryCount > 0 && ordered[picked].CategoryID == runCategory {
				runCategoryCount++
			} else {
				runCategory = ordered[picked].CategoryID
				runCategoryCount = 1
			}
		} else {
			// An uncategorised post breaks the category run.
			runCategory = 0
			runCategoryCount = 0
		}
		out = append(out, ordered[picked].ID)
	}

	return out
}
