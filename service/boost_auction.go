package service

import "sort"

// BoostCandidate is one active campaign competing for a promoted slot in a feed
// page. Bids and budgets are milli-tokens per 1000 impressions / total.
type BoostCandidate struct {
	CampaignID           uint
	PostID               uint
	CenterID             uint
	BidCPMMilli          int64
	RemainingBudgetMilli int64
}

// BoostWinner is a campaign that won a slot, with the SECOND-PRICE CPM it is
// billed at (never above its own bid).
type BoostWinner struct {
	CampaignID    uint
	PostID        uint
	CenterID      uint
	PriceCPMMilli int64
}

// SelectBoostWinners runs a uniform second-price auction for the promoted slots
// of one feed page.
//
//   - only campaigns with remaining budget participate;
//   - the `slots` highest bidders win;
//   - every winner pays the highest LOSING bid (floored by reserveCPMMilli),
//     the classic second price — a center never pays more than the minimum
//     needed to hold its slot, which is what keeps bidding honest.
//
// Ties break by ascending campaign id so the outcome is deterministic and
// testable. A nil/empty result means "no promoted slots this page".
func SelectBoostWinners(candidates []BoostCandidate, slots int, reserveCPMMilli int64) []BoostWinner {
	if slots <= 0 {
		return nil
	}

	eligible := make([]BoostCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.RemainingBudgetMilli <= 0 {
			continue
		}
		eligible = append(eligible, candidate)
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].BidCPMMilli != eligible[j].BidCPMMilli {
			return eligible[i].BidCPMMilli > eligible[j].BidCPMMilli
		}
		return eligible[i].CampaignID < eligible[j].CampaignID
	})
	if len(eligible) == 0 {
		return nil
	}

	// Uniform second price: the highest losing bid, floored by the reserve.
	price := reserveCPMMilli
	if len(eligible) > slots {
		price = eligible[slots].BidCPMMilli
		eligible = eligible[:slots]
	}
	if price < reserveCPMMilli {
		price = reserveCPMMilli
	}

	winners := make([]BoostWinner, 0, len(eligible))
	for _, candidate := range eligible {
		winners = append(winners, BoostWinner{
			CampaignID:    candidate.CampaignID,
			PostID:        candidate.PostID,
			CenterID:      candidate.CenterID,
			PriceCPMMilli: price,
		})
	}
	return winners
}

// CostPerImpressionMilli converts a CPM price (milli-tokens per 1000
// impressions) into the milli-token cost of ONE impression, rounding UP so the
// platform never under-bills.
func CostPerImpressionMilli(priceCPMMilli int64) int64 {
	if priceCPMMilli <= 0 {
		return 0
	}
	return (priceCPMMilli + 999) / 1000
}
