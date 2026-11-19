package service

import (
	"testing"
	"time"
)

func candidate(id uint, bid, remaining int64) BoostCandidate {
	return BoostCandidate{CampaignID: id, PostID: id, CenterID: id, BidCPMMilli: bid, RemainingBudgetMilli: remaining}
}

func TestSelectBoostWinners_EmptyAndNoSlots(t *testing.T) {
	if got := SelectBoostWinners(nil, 3, 100); got != nil {
		t.Fatalf("no candidates: want nil, got %v", got)
	}
	if got := SelectBoostWinners([]BoostCandidate{candidate(1, 500, 1000)}, 0, 100); got != nil {
		t.Fatalf("zero slots: want nil, got %v", got)
	}
}

func TestSelectBoostWinners_ExcludesExhaustedCampaigns(t *testing.T) {
	winners := SelectBoostWinners([]BoostCandidate{
		candidate(1, 900, 0),   // exhausted, highest bid → must be skipped
		candidate(2, 400, 500), // eligible
	}, 1, 100)
	if len(winners) != 1 || winners[0].CampaignID != 2 {
		t.Fatalf("exhausted campaign must not win: got %v", winners)
	}
}

func TestSelectBoostWinners_UniformSecondPrice(t *testing.T) {
	// Bids 800, 600, 400, 200 with 2 slots → winners 1 and 2, both pay the
	// highest LOSING bid (400).
	winners := SelectBoostWinners([]BoostCandidate{
		candidate(1, 800, 10_000),
		candidate(2, 600, 10_000),
		candidate(3, 400, 10_000),
		candidate(4, 200, 10_000),
	}, 2, 100)

	if len(winners) != 2 {
		t.Fatalf("want 2 winners, got %d (%v)", len(winners), winners)
	}
	if winners[0].CampaignID != 1 || winners[1].CampaignID != 2 {
		t.Fatalf("wrong winners: %v", winners)
	}
	for _, w := range winners {
		if w.PriceCPMMilli != 400 {
			t.Fatalf("second price must be the highest losing bid (400), got %d", w.PriceCPMMilli)
		}
	}
}

func TestSelectBoostWinners_FewerCandidatesThanSlotsPaysReserve(t *testing.T) {
	winners := SelectBoostWinners([]BoostCandidate{
		candidate(1, 300, 1000),
		candidate(2, 250, 1000),
	}, 5, 150)
	if len(winners) != 2 {
		t.Fatalf("all candidates should win when slots exceed bidders: %v", winners)
	}
	for _, w := range winners {
		if w.PriceCPMMilli != 150 {
			t.Fatalf("price must floor at reserve (150), got %d", w.PriceCPMMilli)
		}
	}
}

func TestSelectBoostWinners_ReserveFloorsaLowLosingBid(t *testing.T) {
	// Losing bid (50) is below the reserve (100) → winners pay the reserve.
	winners := SelectBoostWinners([]BoostCandidate{
		candidate(1, 300, 1000),
		candidate(2, 50, 1000),
	}, 1, 100)
	if len(winners) != 1 || winners[0].PriceCPMMilli != 100 {
		t.Fatalf("price must floor at reserve: %v", winners)
	}
}

func TestSelectBoostWinners_TiesBreakByCampaignID(t *testing.T) {
	winners := SelectBoostWinners([]BoostCandidate{
		candidate(7, 500, 1000),
		candidate(3, 500, 1000),
	}, 1, 100)
	if len(winners) != 1 || winners[0].CampaignID != 3 {
		t.Fatalf("tie must break by ascending campaign id: %v", winners)
	}
}

func TestCostPerImpressionMilli_RoundsUp(t *testing.T) {
	if got := CostPerImpressionMilli(0); got != 0 {
		t.Fatalf("zero price → 0, got %d", got)
	}
	if got := CostPerImpressionMilli(1000); got != 1 {
		t.Fatalf("1 token/1000 → 1 milli, got %d", got)
	}
	if got := CostPerImpressionMilli(1500); got != 2 {
		t.Fatalf("1.5 milli/impression must round up to 2, got %d", got)
	}
}

func TestMergePromotedIDs_SpacesOnePerDensity(t *testing.T) {
	ordered, promoted := MergePromotedIDs([]uint{1, 2, 3, 4, 5, 6}, []uint{99}, 5)
	want := []uint{1, 2, 3, 4, 99, 5, 6}
	if len(ordered) != len(want) {
		t.Fatalf("length: want %d, got %d (%v)", len(want), len(ordered), ordered)
	}
	for i := range want {
		if ordered[i] != want[i] {
			t.Fatalf("position %d: want %d, got %d (%v)", i, want[i], ordered[i], ordered)
		}
	}
	if !promoted[99] {
		t.Fatal("99 must be marked promoted")
	}
}

func TestMergePromotedIDs_MovesOrganicDuplicateWithoutDuplicating(t *testing.T) {
	ordered, promoted := MergePromotedIDs([]uint{1, 2, 3}, []uint{2}, 5)
	if len(ordered) != 3 {
		t.Fatalf("length must stay 3, got %d (%v)", len(ordered), ordered)
	}
	seen := map[uint]int{}
	for _, id := range ordered {
		seen[id]++
	}
	if seen[2] != 1 {
		t.Fatalf("promoted id must appear exactly once: %v", ordered)
	}
	if !promoted[2] {
		t.Fatal("2 must be marked promoted")
	}
}

func TestMergePromotedIDs_RespectsDensityBetweenPromoted(t *testing.T) {
	ranked := []uint{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	ordered, _ := MergePromotedIDs(ranked, []uint{90, 91}, 5)

	positions := []int{}
	for i, id := range ordered {
		if id == 90 || id == 91 {
			positions = append(positions, i+1)
		}
	}
	if len(positions) != 2 {
		t.Fatalf("both promoted ids must appear: %v", ordered)
	}
	if positions[1]-positions[0] < 5 {
		t.Fatalf("promoted items must be >= density apart: %v", positions)
	}
}

func TestWithinPacing(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	const budget = int64(100_000)

	if !WithinPacing(budget, 0, start, end, start) {
		t.Fatal("a fresh campaign must be allowed (opening buffer)")
	}
	if !WithinPacing(budget, 40_000, start, end, start.Add(12*time.Hour)) {
		t.Fatal("40% spent at half-time is within pacing")
	}
	if WithinPacing(budget, 60_000, start, end, start.Add(12*time.Hour)) {
		t.Fatal("60% spent at half-time is ahead of schedule")
	}
	if WithinPacing(budget, budget, start, end, start.Add(23*time.Hour)) {
		t.Fatal("an exhausted budget is never allowed")
	}
	if WithinPacing(budget, 0, start, end, start.Add(-time.Hour)) {
		t.Fatal("a campaign that has not started is not allowed")
	}
}

// TestMergePromotedIDs_NeverDemotesAPaidPost — the guarantee the advertiser is
// buying: a post that already ranked organically BETTER than its ad slot keeps
// its organic position. Before this, a video ranked #3 that paid for a boost was
// moved down to #5 — the customer paid to lose reach.
func TestMergePromotedIDs_NeverDemotesAPaidPost(t *testing.T) {
	// 20 organic posts; the paid one (id 3) was organically THIRD.
	ranked := make([]uint, 0, 20)
	for i := uint(1); i <= 20; i++ {
		ranked = append(ranked, i)
	}

	ordered, promoted := MergePromotedIDs(ranked, []uint{3}, 5)

	if !promoted[3] {
		t.Fatal("3 must be marked promoted")
	}
	// 0-based index 2 == third position. It must stay there (or higher), never
	// slide to the slot at index 4.
	if got := indexOfID(ordered, 3); got > 2 {
		t.Fatalf("paid post was demoted: organic #3 ended at index %d (%v)", got, ordered)
	}
	if len(ordered) != len(ranked) {
		t.Fatalf("length must be preserved: want %d, got %d", len(ranked), len(ordered))
	}
	seen := map[uint]int{}
	for _, id := range ordered {
		seen[id]++
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("id %d appears %d times", id, n)
		}
	}
}

// TestMergePromotedIDs_StillSpacesWhenTheSlotIsBetter — the normal case is
// untouched: a post ranked below its slot is pulled UP to the slot.
func TestMergePromotedIDs_StillSpacesWhenTheSlotIsBetter(t *testing.T) {
	ranked := make([]uint, 0, 20)
	for i := uint(1); i <= 20; i++ {
		ranked = append(ranked, i)
	}

	ordered, _ := MergePromotedIDs(ranked, []uint{18}, 5)

	if got := indexOfID(ordered, 18); got != 4 {
		t.Fatalf("a promoted post ranked below its slot must land on slot 5 (index 4), got %d (%v)", got, ordered)
	}
}
