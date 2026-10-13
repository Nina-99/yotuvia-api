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
	if got := EngagementScore(0, 0); got != 0 {
		t.Fatalf("no engagement: want 0, got %v", got)
	}
	base := EngagementScore(10, 1)
	if EngagementScore(100, 1) <= base {
		t.Fatal("more views must score higher")
	}
	if EngagementScore(10, 5) <= base {
		t.Fatal("more book-taps must score higher")
	}
	// A book-tap outweighs a view burst (booking intent > attention).
	if EngagementScore(0, 10) <= EngagementScore(1000, 0) {
		t.Fatal("book-taps should outweigh views")
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
	}, MaxSameCenterRun)
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
	ids := RankByScore(candidates, MaxSameCenterRun)
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
	ids := RankByScore(candidates, MaxSameCenterRun)
	if len(ids) != 3 {
		t.Fatalf("relaxed pass must keep every post: got %v", ids)
	}
}
