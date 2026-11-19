package service

import (
	"math"
	"testing"
	"time"
)

func TestRecencyScore(t *testing.T) {
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)

	if got := RecencyScore(now, now); math.Abs(got-1.0) > 1e-9 {
		t.Fatalf("brand-new post: want 1.0, got %v", got)
	}
	halfLife := now.Add(-time.Duration(RecencyHalfLifeHours) * time.Hour)
	if got := RecencyScore(halfLife, now); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("one half-life: want 0.5, got %v", got)
	}
	if got := RecencyScore(now.Add(-30*24*time.Hour), now); got > 0.01 {
		t.Fatalf("30-day-old post should decay toward 0, got %v", got)
	}
	if got := RecencyScore(now.Add(time.Hour), now); math.Abs(got-1.0) > 1e-9 {
		t.Fatalf("future timestamp clamps to 1.0, got %v", got)
	}
}

func TestEngagementScore(t *testing.T) {
	// Quality is intent PER VIEW. The whole point: a post with 9 taps of 10 views
	// must beat one with 900 taps of 100000 views, even though the second has
	// vastly more absolute engagement.
	small := QualityScore(10, 9)
	hugeBoring := QualityScore(100_000, 900)
	if small <= hugeBoring {
		t.Fatalf("quality must reward the ratio, not the volume: small=%.3f huge=%.3f", small, hugeBoring)
	}

	// No views at all: nothing to infer, and the prior keeps it small (not zero,
	// not suspiciously high).
	if got := QualityScore(0, 0); got < 0 || got > QualityScale {
		t.Fatalf("empty post quality out of range: %v", got)
	}

	// The scale saturates: an absurd ratio cannot exceed QualityScale.
	if got := QualityScore(1, 1_000_000); got > QualityScale {
		t.Fatalf("quality must cap at %v, got %v", QualityScale, got)
	}

	// Popularity is bounded so reach alone can never dominate the ranking.
	if got := PopularityScore(10_000_000); got > PopularityCap {
		t.Fatalf("popularity must cap at %v, got %v", PopularityCap, got)
	}
	if PopularityScore(1000) <= PopularityScore(10) {
		t.Fatal("popularity must be monotone in reach")
	}
}

// TestSaturate_DiminishingReturns — the anti-collapse rule. The 2nd matching
// watch matters, the 20th almost does not, and the cap is never exceeded.
func TestSaturate_DiminishingReturns(t *testing.T) {
	cap := CategoryAffinityCap

	first := Saturate(3, cap)
	second := Saturate(6, cap)
	twentieth := Saturate(60, cap)

	if !(first < second && second < twentieth) {
		t.Fatalf("saturation must be monotone: %v %v %v", first, second, twentieth)
	}
	if twentieth > cap {
		t.Fatalf("saturation must never exceed the cap: %v > %v", twentieth, cap)
	}
	// Twenty matching watches may not be worth more than the sum of the other
	// signals, or the feed collapses into one category again.
	if twentieth > ProximityScale+QualityScale {
		t.Fatalf("saturated affinity %v still swamps proximity %v + quality %v",
			twentieth, ProximityScale, QualityScale)
	}

	// Signed saturation: a category the user keeps skipping is pushed down.
	if got := Saturate(-60, cap); got >= 0 || got < -cap {
		t.Fatalf("negative saturation must stay negative and bounded: %v", got)
	}
	if Saturate(-60, cap) >= Saturate(-6, cap) {
		t.Fatal("more skips must repel more")
	}
}

// TestInteractionWeight_SkipIsNegative — "I closed this" is evidence against.
func TestInteractionWeight_SkipIsNegative(t *testing.T) {
	if got := InteractionWeight(false, 1); got >= 0 {
		t.Fatalf("an abandoned view must weigh negatively, got %v", got)
	}
	if InteractionWeight(false, 5) >= 0 {
		t.Fatalf("a 5%% view is still a skip, got %v", InteractionWeight(false, 5))
	}
	if got := InteractionWeight(false, 30); got <= 0 {
		t.Fatalf("a 30%% view is a glance, not a skip: %v", got)
	}
	if got := InteractionWeight(true, 100); got != CompletedWeight {
		t.Fatalf("completed must weigh %v, got %v", CompletedWeight, got)
	}
}

// TestAffinityDecay_HalfLife
func TestAffinityDecay_HalfLife(t *testing.T) {
	if got := AffinityDecay(0); got != 1 {
		t.Fatalf("a signal from today weighs 1, got %v", got)
	}
	halfLife := time.Duration(AffinityHalfLifeDays) * 24 * time.Hour
	if got := AffinityDecay(halfLife); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("a signal at the half-life weighs 0.5, got %v", got)
	}
	// A six-month-old obsession must be worth far less than yesterday's.
	old := AffinityDecay(180 * 24 * time.Hour)
	recent := AffinityDecay(24 * time.Hour)
	if old >= recent {
		t.Fatalf("older signals must decay: old=%v recent=%v", old, recent)
	}
}

func TestExplorationScoreIsDeterministicAndBounded(t *testing.T) {
	for _, id := range []uint{1, 2, 99, 12345} {
		first := ExplorationScore(42, id)
		second := ExplorationScore(42, id)
		if first != second {
			t.Fatalf("same (seed,id) must be deterministic: %v != %v", first, second)
		}
		if first < 0 || first >= 1 {
			t.Fatalf("score must be in [0,1), got %v", first)
		}
	}
	if ExplorationScore(1, 7) == ExplorationScore(2, 7) && ExplorationScore(1, 7) == ExplorationScore(1, 8) {
		t.Fatal("seed and id must influence the score")
	}
}

func TestRankByScoreOrdersAndBreaksTiesByID(t *testing.T) {
	ids := RankByScore([]RankCandidate{
		{ID: 3, CenterID: 1, Score: 1.0},
		{ID: 1, CenterID: 2, Score: 2.0},
		{ID: 2, CenterID: 3, Score: 2.0},
	}, MaxSameCenterRun, MaxSameCategoryRun)
	// id 1 before id 2 (tie, ascending id), then id 3.
	want := []uint{1, 2, 3}
	if len(ids) != len(want) {
		t.Fatalf("length: want %d, got %d", len(want), len(ids))
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("position %d: want %d, got %d (%v)", i, want[i], ids[i], ids)
		}
	}
}

func TestRankByScoreCapsConsecutiveSameCenter(t *testing.T) {
	// Four posts from center 7 (top scores) + one from center 9.
	candidates := []RankCandidate{
		{ID: 1, CenterID: 7, Score: 10},
		{ID: 2, CenterID: 7, Score: 9},
		{ID: 3, CenterID: 7, Score: 8},
		{ID: 4, CenterID: 7, Score: 7},
		{ID: 5, CenterID: 9, Score: 1},
	}
	ids := RankByScore(candidates, MaxSameCenterRun, MaxSameCategoryRun)
	if len(ids) != 5 {
		t.Fatalf("must not truncate: want 5, got %d", len(ids))
	}
	// The 3rd slot must break the center-7 run with the center-9 post.
	byID := map[uint]RankCandidate{}
	for _, c := range candidates {
		byID[c.ID] = c
	}
	runCount := 0
	runCenter := uint(0)
	for _, id := range ids {
		c := byID[id]
		if runCount > 0 && c.CenterID == runCenter {
			runCount++
		} else {
			runCenter = c.CenterID
			runCount = 1
		}
		if runCount > MaxSameCenterRun {
			t.Fatalf("center %d ran %d times in a row (%v)", c.CenterID, runCount, ids)
		}
	}
}

func TestRankByScoreRelaxesWhenAllSameCenter(t *testing.T) {
	// Every post is from one center: the cap cannot hold, but nothing is lost.
	candidates := []RankCandidate{
		{ID: 1, CenterID: 5, Score: 3},
		{ID: 2, CenterID: 5, Score: 2},
		{ID: 3, CenterID: 5, Score: 1},
	}
	ids := RankByScore(candidates, MaxSameCenterRun, MaxSameCategoryRun)
	if len(ids) != 3 {
		t.Fatalf("relaxed pass must keep every post: got %v", ids)
	}
}
