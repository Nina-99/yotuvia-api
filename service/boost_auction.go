package service

import (
	"sort"
	"time"
)

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
		// A bid below the reserve cannot buy a slot, even when there are more
		// slots than bidders: the floor is a real price, not just a floor for
		// the charge.
		if candidate.BidCPMMilli < reserveCPMMilli {
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

// WithinPacing reports whether a campaign may spend more right now under linear
// budget pacing: over its window, a campaign may have spent at most its budget
// scaled by the fraction of time elapsed (plus a 1% opening buffer so a fresh
// campaign is never starved). This spreads a budget across the campaign instead
// of letting the highest bidder exhaust it in the first minutes.
//
// Returns false when the campaign has not started, is already exhausted, or has
// spent ahead of its schedule.
func WithinPacing(budgetMilli, spentMilli int64, startsAt, endsAt, now time.Time) bool {
	if budgetMilli <= 0 || spentMilli >= budgetMilli {
		return false
	}
	if !now.After(startsAt) && !now.Equal(startsAt) {
		return false
	}
	total := endsAt.Sub(startsAt)
	if total <= 0 {
		return true
	}
	fraction := float64(now.Sub(startsAt)) / float64(total)
	if fraction > 1 {
		fraction = 1
	}
	allowed := int64(float64(budgetMilli)*fraction) + budgetMilli/100 + 1
	return spentMilli < allowed
}

// MergePromotedIDs weaves promoted post ids into a ranked feed so that at most
// one promoted item appears every `density` positions, preserving the organic
// order otherwise. A promoted id that also occurs organically is MOVED to its
// promoted slot (never duplicated). When every organic item is exhausted any
// remaining promoted ids are appended, so a paid slot is never silently
// dropped. Returns the new order plus the set of ids that are promoted (for the
// caller to label).
func MergePromotedIDs(ranked, promoted []uint, density int) ([]uint, map[uint]bool) {
	if density <= 0 {
		density = 5
	}

	promotedSet := make(map[uint]bool, len(promoted))
	queue := make([]uint, 0, len(promoted))
	for _, id := range promoted {
		if promotedSet[id] {
			continue
		}
		promotedSet[id] = true
		queue = append(queue, id)
	}

	// Where each promoted post sat in the ORGANIC ranking before the merge. This
	// is the guarantee the advertiser buys: paying can only improve or keep the
	// position, never worsen it.
	organicRank := make(map[uint]int, len(promotedSet))
	for i, id := range ranked {
		if promotedSet[id] {
			organicRank[id] = i
		}
	}

	organic := make([]uint, 0, len(ranked))
	for _, id := range ranked {
		if promotedSet[id] {
			continue
		}
		organic = append(organic, id)
	}

	ordered := make([]uint, 0, len(ranked)+len(queue))
	organicIndex, promotedIndex := 0, 0
	for organicIndex < len(organic) || promotedIndex < len(queue) {
		// A promoted slot opens at every `density`-th position (5th, 10th, ...).
		if promotedIndex < len(queue) && (len(ordered)+1)%density == 0 {
			ordered = append(ordered, queue[promotedIndex])
			promotedIndex++
			continue
		}
		if organicIndex < len(organic) {
			ordered = append(ordered, organic[organicIndex])
			organicIndex++
			continue
		}
		ordered = append(ordered, queue[promotedIndex])
		promotedIndex++
	}

	// Rescue pass: a promoted post whose organic position was BETTER than the
	// slot it landed in is moved back up. Without this, a video ranked #3 that
	// pays for a boost is demoted to #5 — the customer paid to lose reach.
	for _, id := range queue {
		want, ok := organicRank[id]
		if !ok {
			continue
		}
		at := indexOfID(ordered, id)
		if at < 0 || want >= at {
			continue
		}
		ordered = append(ordered[:at], ordered[at+1:]...)
		// Clamp: the organic rank can point past the end after removals.
		if want > len(ordered) {
			want = len(ordered)
		}
		ordered = append(ordered, 0)
		copy(ordered[want+1:], ordered[want:])
		ordered[want] = id
	}

	return ordered, promotedSet
}

// indexOfID returns the position of id in ids, or -1.
func indexOfID(ids []uint, id uint) int {
	for i, candidate := range ids {
		if candidate == id {
			return i
		}
	}
	return -1
}
