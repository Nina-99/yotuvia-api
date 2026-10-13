package service

import "testing"

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
