package service

import (
	"errors"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
)

// boostEligibleScanLimit bounds how many campaigns the auction reads per page.
const boostEligibleScanLimit = 500

// Campaign statuses.
const (
	BoostStatusActive    = "active"
	BoostStatusPaused    = "paused"
	BoostStatusExhausted = "exhausted"
)

type BoostServiceImpl struct {
	repo     repository.BoostCampaignRepository
	feedRepo repository.FeedPostRepository
	tokenSvc TokenAccountService
}

func NewBoostServiceImpl(
	repo repository.BoostCampaignRepository,
	feedRepo repository.FeedPostRepository,
	tokenSvc TokenAccountService,
) BoostService {
	return &BoostServiceImpl{repo: repo, feedRepo: feedRepo, tokenSvc: tokenSvc}
}

// CreateCampaign validates the request, HOLDS the budget up front (so the
// platform never extends credit for ads) and only then creates the row. If the
// hold fails, the just-created row is deleted so a failed campaign never lingers.
func (s *BoostServiceImpl) CreateCampaign(
	centerID, feedPostID uint,
	budgetMilli, bidCPMMilli int64,
	startsAt, endsAt time.Time,
) (*models.BoostCampaign, error) {
	if budgetMilli <= 0 {
		return nil, errors.New("boost: budget must be positive")
	}
	if bidCPMMilli <= 0 {
		return nil, errors.New("boost: bid must be positive")
	}
	if !endsAt.After(startsAt) {
		return nil, errors.New("boost: end must be after start")
	}

	post, err := s.feedRepo.FindByID(feedPostID)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, errors.New("boost: feed post not found")
	}
	if post.BeautyCenterID != centerID {
		return nil, errors.New("boost: post does not belong to this center")
	}
	if !post.IsActive {
		return nil, errors.New("boost: cannot promote a suspended post")
	}

	campaign := &models.BoostCampaign{
		BeautyCenterID: centerID,
		FeedPostID:     feedPostID,
		BudgetMilli:    budgetMilli,
		SpentMilli:     0,
		BidCPMMilli:    bidCPMMilli,
		Status:         BoostStatusActive,
		StartsAt:       startsAt,
		EndsAt:         endsAt,
	}
	if err := s.repo.Create(campaign); err != nil {
		return nil, err
	}

	// Prepaid hold. The reference is the campaign id so the token ledger points
	// back to the exact campaign that spent the tokens.
	if err := s.tokenSvc.Consume(centerID, budgetMilli, campaign.ID, "boost campaign hold"); err != nil {
		_ = s.repo.Delete(campaign.ID)
		return nil, err
	}

	return campaign, nil
}

// PauseCampaign stops an owned campaign from winning further slots. Ownership is
// enforced so a center can only pause its OWN campaign.
func (s *BoostServiceImpl) PauseCampaign(centerID, campaignID uint) error {
	campaign, err := s.repo.FindByID(campaignID)
	if err != nil {
		return err
	}
	if campaign == nil {
		return errors.New("boost: campaign not found")
	}
	if campaign.BeautyCenterID != centerID {
		return errors.New("boost: campaign does not belong to this center")
	}
	if campaign.Status == BoostStatusPaused {
		return nil
	}
	campaign.Status = BoostStatusPaused
	return s.repo.Update(campaign)
}

// SettleImpressions books delivered impressions at the auction price, capped so
// spend never exceeds the prepaid budget. The budget was already debited at
// creation, so this only updates accounting; the campaign flips to `exhausted`
// when its budget is used up.
func (s *BoostServiceImpl) SettleImpressions(campaignID uint, impressions, priceCPMMilli int64) error {
	campaign, err := s.repo.FindByID(campaignID)
	if err != nil {
		return err
	}
	if campaign == nil {
		return errors.New("boost: campaign not found")
	}
	if impressions <= 0 {
		return nil
	}

	remaining := campaign.BudgetMilli - campaign.SpentMilli
	if remaining <= 0 {
		campaign.Status = BoostStatusExhausted
		return s.repo.Update(campaign)
	}

	cost := impressions * CostPerImpressionMilli(priceCPMMilli)
	if cost > remaining {
		cost = remaining
	}
	campaign.SpentMilli += cost
	campaign.Impressions += impressions
	if campaign.SpentMilli >= campaign.BudgetMilli {
		campaign.Status = BoostStatusExhausted
	}
	return s.repo.Update(campaign)
}

func (s *BoostServiceImpl) ListByCenter(centerID uint) ([]models.BoostCampaign, error) {
	return s.repo.FindByCenter(centerID)
}

// SelectWinners reads the eligible campaigns and runs the second-price auction.
func (s *BoostServiceImpl) SelectWinners(now time.Time, slots int, reserveCPMMilli int64) ([]BoostWinner, error) {
	if slots <= 0 {
		return nil, nil
	}
	campaigns, err := s.repo.FindEligible(now, boostEligibleScanLimit)
	if err != nil {
		return nil, err
	}
	candidates := make([]BoostCandidate, 0, len(campaigns))
	for _, campaign := range campaigns {
		// Budget pacing: a campaign may only spend what its schedule allows, so
		// the highest bidder cannot exhaust its budget in the first minutes.
		if !WithinPacing(campaign.BudgetMilli, campaign.SpentMilli, campaign.StartsAt, campaign.EndsAt, now) {
			continue
		}
		candidates = append(candidates, BoostCandidate{
			CampaignID:           campaign.ID,
			PostID:               campaign.FeedPostID,
			CenterID:             campaign.BeautyCenterID,
			BidCPMMilli:          campaign.BidCPMMilli,
			RemainingBudgetMilli: campaign.BudgetMilli - campaign.SpentMilli,
		})
	}
	return SelectBoostWinners(candidates, slots, reserveCPMMilli), nil
}
