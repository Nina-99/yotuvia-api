package service

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"gorm.io/gorm"
)

// boostEligibleScanLimit bounds how many campaigns the auction reads per page.
const boostEligibleScanLimit = 500

// Campaign statuses.
const (
	BoostStatusActive    = "active"
	BoostStatusPaused    = "paused"
	BoostStatusExhausted = "exhausted"
	BoostStatusEnded     = "ended"
)

// boostReserveCapPct is the fallback cap when no config service is wired.
const boostReserveCapPct = 0.50

// Campaign accounting models. "reserve" holds funds on the wallet (current);
// "legacy_debit" debited the budget at creation (pre-retention rows) and must
// never release or settle against a hold that never existed.
const (
	BoostAccountingReserve = "reserve"
	BoostAccountingLegacy  = "legacy_debit"
)

// usesReserve reports whether a campaign funds itself from a wallet hold.
func usesReserve(campaign *models.BoostCampaign) bool {
	return campaign.AccountingModel == BoostAccountingReserve
}

const (
	// impressionTokenTTL bounds how long after serving a view can still be
	// reported for billing.
	impressionTokenTTL = 30 * time.Minute
)

type BoostServiceImpl struct {
	repo             repository.BoostCampaignRepository
	feedRepo         repository.FeedPostRepository
	tokenSvc         TokenAccountService
	accountRepo      repository.TokenAccountRepository
	marketConfigSvc  MarketConfigService
	impressionSecret []byte
}

func NewBoostServiceImpl(
	repo repository.BoostCampaignRepository,
	feedRepo repository.FeedPostRepository,
	tokenSvc TokenAccountService,
	accountRepo repository.TokenAccountRepository,
	marketConfigSvc MarketConfigService,
) BoostService {
	// The HMAC secret must be shared across instances to verify tokens issued by
	// another replica. Without it (dev), a per-process random secret keeps the
	// feature working within this process without pretending to be durable.
	secret := []byte(os.Getenv("BOOST_IMPRESSION_SECRET"))
	if len(secret) == 0 {
		secret = make([]byte, 32)
		if _, err := crand.Read(secret); err != nil {
			secret = []byte("boost-impression-dev-secret")
		}
		utils.Log.Warn("[BOOST] BOOST_IMPRESSION_SECRET not set — using a per-process secret")
	}
	return &BoostServiceImpl{
		repo:             repo,
		feedRepo:         feedRepo,
		tokenSvc:         tokenSvc,
		accountRepo:      accountRepo,
		marketConfigSvc:  marketConfigSvc,
		impressionSecret: secret,
	}
}

// economyForCenter resolves the per-currency boost parameters from the center's
// wallet currency, falling back to canonical defaults when no config service or
// row is available.
func (s *BoostServiceImpl) economyForCenter(centerID uint) EconomyConfig {
	currency := defaultEconomy.Currency
	if account, err := s.accountRepo.FindByBeautyCenterID(centerID); err == nil && account != nil && account.Currency != "" {
		currency = account.Currency
	}
	if s.marketConfigSvc == nil {
		return defaultEconomy
	}
	return s.marketConfigSvc.GetEconomyForCurrency(currency)
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
		BeautyCenterID:  centerID,
		FeedPostID:      feedPostID,
		BudgetMilli:     budgetMilli,
		SpentMilli:      0,
		ReservedMilli:   budgetMilli,
		BidCPMMilli:     bidCPMMilli,
		Status:          BoostStatusActive,
		AccountingModel: BoostAccountingReserve,
		StartsAt:        startsAt,
		EndsAt:          endsAt,
	}

	// Prepaid hold: reserve the budget BEFORE creating the row. A failed hold
	// must leave no campaign behind. Reserving moves no money — the funds stay
	// in the wallet, held, until impressions are settled.
	capPct := boostReserveCapPct
	if s.marketConfigSvc != nil {
		capPct = s.economyForCenter(centerID).BoostReserveCapPct
	}
	if err := s.tokenSvc.Reserve(centerID, budgetMilli, capPct); err != nil {
		return nil, err
	}
	if err := s.repo.Create(campaign); err != nil {
		_ = s.tokenSvc.ReleaseReserved(centerID, budgetMilli)
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
	return s.releaseAndUpdate(campaign)
}

// releaseAndUpdate returns the campaign's remaining hold to available funds and
// persists the campaign in the same transaction. Legacy campaigns have no hold
// (ReservedMilli == 0), so they only persist their new status.
func (s *BoostServiceImpl) releaseAndUpdate(campaign *models.BoostCampaign) error {
	// Legacy campaigns never held funds, so they only persist their new status.
	if !usesReserve(campaign) {
		return s.repo.Update(campaign)
	}
	remaining := campaign.ReservedMilli - campaign.SpentMilli - campaign.ReleasedMilli
	if remaining <= 0 {
		return s.repo.Update(campaign)
	}

	account, err := s.accountRepo.FindByBeautyCenterID(campaign.BeautyCenterID)
	if err != nil {
		return err
	}
	if account == nil {
		return errors.New("boost: token account not found")
	}

	return s.accountRepo.Transaction(func(tx *gorm.DB) error {
		if err := s.accountRepo.ReleaseReservedInTx(tx, account.ID, remaining); err != nil {
			return err
		}
		campaign.ReleasedMilli += remaining
		return s.repo.UpdateInTx(tx, campaign)
	})
}

// SettleImpressions books delivered impressions against the campaign's hold at
// the auction price. The token debit and the campaign accounting commit in ONE
// transaction: the ledger row, the account hold and the campaign spend can never
// drift apart. Legacy campaigns (ReservedMilli == 0) keep the old campaign-only
// accounting because their budget was already debited at creation.
func (s *BoostServiceImpl) SettleImpressions(campaignID uint, impressions, priceCPMMilli int64) error {
	if impressions <= 0 {
		return nil
	}

	campaign, err := s.repo.FindByID(campaignID)
	if err != nil {
		return err
	}
	if campaign == nil {
		return errors.New("boost: campaign not found")
	}

	remaining := campaign.BudgetMilli - campaign.SpentMilli
	if remaining <= 0 {
		if campaign.Status == BoostStatusActive {
			campaign.Status = BoostStatusExhausted
			return s.repo.Update(campaign)
		}
		return nil
	}

	cost := impressions * CostPerImpressionMilli(priceCPMMilli)
	if cost > remaining {
		cost = remaining
	}
	if cost <= 0 {
		return nil
	}

	var account *models.TokenAccount
	if usesReserve(campaign) {
		account, err = s.accountRepo.FindByBeautyCenterID(campaign.BeautyCenterID)
		if err != nil {
			return err
		}
		if account == nil {
			return errors.New("boost: token account not found")
		}
	}

	return s.accountRepo.Transaction(func(tx *gorm.DB) error {
		return s.settleInTx(tx, campaign, account, impressions, cost)
	})
}

// settleInTx books one settlement inside a caller-managed transaction: it moves
// the reserved funds (skipped for legacy campaigns, which debited at creation),
// adds the spend and marks exhaustion, all atomically.
func (s *BoostServiceImpl) settleInTx(tx *gorm.DB, campaign *models.BoostCampaign, account *models.TokenAccount, impressions, cost int64) error {
	if usesReserve(campaign) {
		if account == nil {
			return errors.New("boost: token account not found")
		}
		ref := campaign.ID
		if err := s.accountRepo.SettleReservedInTx(tx, account.ID, cost, "boost_spend",
			fmt.Sprintf("Boost campaign #%d impressions", campaign.ID), &ref); err != nil {
			return err
		}
	}

	fits, err := s.repo.AddSpendInTx(tx, campaign.ID, impressions, cost)
	if err != nil {
		return err
	}
	if !fits {
		return apperrors.ErrConflict
	}
	return s.repo.MarkExhaustedInTx(tx, campaign.ID)
}

// ImpressionToken signs the billing token for one served promoted slot. The
// price travels inside the signature, so the client cannot choose its own cost.
func (s *BoostServiceImpl) ImpressionToken(campaignID uint, priceCPMMilli int64) (string, error) {
	nonce, err := newImpressionNonce()
	if err != nil {
		return "", err
	}
	return SignImpressionToken(s.impressionSecret, ImpressionPayload{
		CampaignID:    campaignID,
		PriceCPMMilli: priceCPMMilli,
		Nonce:         nonce,
		Exp:           time.Now().Add(impressionTokenTTL).Unix(),
	})
}

// SettleImpression bills ONE qualified view reported with a signed token. It is
// a no-op — never a charge — for unqualified views, duplicate nonces, exhausted
// or ended campaigns, and requests that arrive after the token expired.
func (s *BoostServiceImpl) SettleImpression(viewerID uint, token string, watchPercentage float64, completed bool) error {
	payload, err := VerifyImpressionToken(s.impressionSecret, token)
	if err != nil {
		return err
	}

	campaign, err := s.repo.FindByID(payload.CampaignID)
	if err != nil {
		return err
	}
	if campaign == nil {
		return nil // campaign deleted after serving: nothing to charge
	}

	qualificationPct := defaultEconomy.BoostQualificationPct
	if s.marketConfigSvc != nil {
		qualificationPct = s.economyForCenter(campaign.BeautyCenterID).BoostQualificationPct
	}
	if !completed && watchPercentage < qualificationPct {
		return nil
	}

	if campaign.Status == BoostStatusEnded || campaign.Status == BoostStatusExhausted {
		return nil
	}

	remaining := campaign.BudgetMilli - campaign.SpentMilli
	cost := CostPerImpressionMilli(payload.PriceCPMMilli)
	if cost <= 0 || cost > remaining {
		return nil
	}

	var account *models.TokenAccount
	if usesReserve(campaign) {
		account, err = s.accountRepo.FindByBeautyCenterID(campaign.BeautyCenterID)
		if err != nil {
			return err
		}
		if account == nil {
			return errors.New("boost: token account not found")
		}
	}

	return s.accountRepo.Transaction(func(tx *gorm.DB) error {
		inserted, err := s.repo.CreateImpressionInTx(tx, &models.BoostImpression{
			CampaignID:    campaign.ID,
			ViewerID:      viewerID,
			Nonce:         payload.Nonce,
			PriceCPMMilli: payload.PriceCPMMilli,
		})
		if err != nil {
			return err
		}
		if !inserted {
			return nil // replayed report: already billed
		}
		return s.settleInTx(tx, campaign, account, 1, cost)
	})
}

// SweepEnded releases the remaining hold of campaigns whose window closed and
// marks them ended. It is idempotent: those rows leave the active/paused set on
// the first pass.
func (s *BoostServiceImpl) SweepEnded(now time.Time) (int, error) {
	expired, err := s.repo.FindExpired(now)
	if err != nil {
		return 0, err
	}
	closed := 0
	for i := range expired {
		campaign := &expired[i]
		campaign.Status = BoostStatusEnded
		if err := s.releaseAndUpdate(campaign); err != nil {
			return closed, err
		}
		closed++
	}
	return closed, nil
}

func (s *BoostServiceImpl) ListByCenter(centerID uint) ([]models.BoostCampaign, error) {
	return s.repo.FindByCenter(centerID)
}

// SelectWinners reads the eligible campaigns and runs the second-price auction.
// RecordUnservedWins implements BoostService.
func (s *BoostServiceImpl) RecordUnservedWins(campaignIDs []uint) error {
	return s.repo.AddUnservedWins(campaignIDs)
}

func (s *BoostServiceImpl) SelectWinners(now time.Time, slots int, reserveCPMMilli int64, viewer *GeoPoint, viewerID uint) ([]BoostWinner, error) {
	if slots <= 0 {
		return nil, nil
	}
	// Release closed campaigns before auctioning so their hold returns to the
	// center. Best-effort: an auction must not fail because a sweep did.
	_, _ = s.SweepEnded(now)

	// Reserve price: the caller may pass an explicit floor; when it does not, the
	// configured per-currency floor applies (BOB launch default). The auction is
	// platform-wide, so a single currency's reserve governs it — mixing currency
	// bids in one auction is a known limitation to revisit with multi-currency
	// launch.
	if reserveCPMMilli <= 0 {
		reserveCPMMilli = defaultEconomy.BoostReserveCPMMilli
		if s.marketConfigSvc != nil {
			reserveCPMMilli = s.marketConfigSvc.GetEconomyForCurrency(defaultEconomy.Currency).BoostReserveCPMMilli
		}
	}
	campaigns, err := s.repo.FindEligible(now, boostEligibleScanLimit)
	if err != nil {
		return nil, err
	}

	radiusKm := boostMaxRadiusKM()

	// Frequency cap: one grouped query, only when a viewer is known.
	campaignIDs := make([]uint, 0, len(campaigns))
	for _, c := range campaigns {
		campaignIDs = append(campaignIDs, c.ID)
	}
	seenByViewer, err := s.repo.CountImpressionsByViewer(viewerID, campaignIDs)
	if err != nil {
		// A cap lookup failure must not stop the auction; it degrades to no cap.
		seenByViewer = nil
	}
	freqCap := boostFrequencyCap()

	candidates := make([]BoostCandidate, 0, len(campaigns))
	for _, campaign := range campaigns {
		// Geo filter: never serve an ad for a centre the viewer cannot reach.
		// A centre with no coordinates keeps its historical reach — punishing
		// missing data would silently zero out real campaigns.
		if viewer.Valid() && (campaign.CenterLatitude != 0 || campaign.CenterLongitude != 0) {
			km := HaversineKM(viewer.Latitude, viewer.Longitude, campaign.CenterLatitude, campaign.CenterLongitude)
			if km > radiusKm {
				continue
			}
		}
		// Frequency cap: this viewer already paid the advertiser enough attention.
		if viewerID != 0 && freqCap > 0 && seenByViewer[campaign.ID] >= freqCap {
			continue
		}

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

// boostMaxRadiusKM resolves BOOST_MAX_RADIUS_KM: how far from the viewer a
// promoted centre may be to still be auctioned. Defaults to 50 km (a city and
// its metropolitan ring). 0 disables the radius filter entirely.
func boostMaxRadiusKM() float64 {
	const defaultKm = 50.0
	raw := os.Getenv("BOOST_MAX_RADIUS_KM")
	if raw == "" {
		return defaultKm
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 {
		return defaultKm
	}
	if v == 0 {
		return math.Inf(1) // explicitly disabled
	}
	return v
}

// boostFrequencyCap resolves BOOST_FREQUENCY_CAP: how many billed views one
// viewer may produce for the same campaign before it stops being served to
// them. Defaults to 2 (see it, maybe rewatch once, then move on). 0 disables it.
func boostFrequencyCap() int64 {
	const defaultCap = 2
	raw := os.Getenv("BOOST_FREQUENCY_CAP")
	if raw == "" {
		return defaultCap
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return defaultCap
	}
	return n
}

// Performance assembles the owner-facing report for one campaign.
//
// Every number comes from what was actually BILLED (boost_impressions) or from
// the campaign row, never from a re-simulation: the owner must be able to trust
// that the report matches the money that left their wallet.
func (s *BoostServiceImpl) Performance(centerID, campaignID uint) (BoostPerformance, error) {
	campaign, err := s.repo.FindByID(campaignID)
	if err != nil {
		return BoostPerformance{}, err
	}
	if campaign == nil {
		return BoostPerformance{}, apperrors.ErrNotFound
	}
	// Owner-only: the route proves the ROLE, this proves the ROW.
	if campaign.BeautyCenterID != centerID {
		return BoostPerformance{}, apperrors.ErrForbidden
	}

	currency := "USD"
	if account, accErr := s.accountRepo.FindByBeautyCenterID(centerID); accErr == nil && account != nil && account.Currency != "" {
		currency = account.Currency
	}

	perf := BoostPerformance{
		CampaignID:   campaign.ID,
		Status:       campaign.Status,
		Currency:     currency,
		Budget:       utils.FromMilli(campaign.BudgetMilli),
		Spent:        utils.FromMilli(campaign.SpentMilli),
		Remaining:    utils.FromMilli(campaign.BudgetMilli - campaign.SpentMilli),
		Impressions:  campaign.Impressions,
		UnservedWins: campaign.UnservedWins,
		StartsAt:     campaign.StartsAt,
		EndsAt:       campaign.EndsAt,
	}

	// Effective CPM: what the owner actually paid per thousand delivered views.
	// It is the number a renewal decision hangs on, so it is derived from spend,
	// never from the bid (a second-price auction rarely charges the bid).
	if campaign.Impressions > 0 {
		perf.EffectiveCPM = utils.FromMilli(campaign.SpentMilli) * 1000 / float64(campaign.Impressions)
	}

	// Delivery rate: of every auction this campaign won, how many became a real
	// view. A low rate with a full budget is the signal that the creative or the
	// audience is the problem, not the bid.
	wins := campaign.Impressions + campaign.UnservedWins
	if wins > 0 {
		perf.DeliveryRate = float64(campaign.Impressions) / float64(wins)
	}

	rows, err := s.repo.DailyImpressions(campaign.ID)
	if err != nil {
		return BoostPerformance{}, err
	}
	perf.Daily = make([]BoostDailyPoint, 0, len(rows))
	for _, row := range rows {
		perf.Daily = append(perf.Daily, BoostDailyPoint{
			Day:         row.Day,
			Impressions: row.Impressions,
			Spend:       utils.FromMilli(row.SpendMilli),
		})
	}
	return perf, nil
}

// PlatformOverview aggregates the whole ad book.
//
// Same honesty rule as the per-campaign report: effective CPM comes from the
// money actually charged, never from the bids. A platform-wide CPM that drifted
// from reality would mislead every pricing decision.
func (s *BoostServiceImpl) PlatformOverview() (BoostOverview, error) {
	agg, err := s.repo.PlatformAggregates()
	if err != nil {
		return BoostOverview{}, err
	}

	overview := BoostOverview{
		Campaigns:       agg.Campaigns,
		ActiveCampaigns: agg.ActiveCampaigns,
		Impressions:     agg.Impressions,
		Spent:           utils.FromMilli(agg.SpentMilli),
		UnservedWins:    agg.UnservedWins,
	}

	if agg.Impressions > 0 {
		overview.EffectiveCPM = utils.FromMilli(agg.SpentMilli) * 1000 / float64(agg.Impressions)
	}
	if wins := agg.Impressions + agg.UnservedWins; wins > 0 {
		overview.DeliveryRate = float64(agg.Impressions) / float64(wins)
	}

	top, err := s.repo.TopBySpend(10)
	if err != nil {
		return BoostOverview{}, err
	}
	overview.TopCampaigns = make([]BoostTopCampaign, 0, len(top))
	for _, row := range top {
		overview.TopCampaigns = append(overview.TopCampaigns, BoostTopCampaign{
			CampaignID:   row.CampaignID,
			CenterID:     row.CenterID,
			CenterName:   row.CenterName,
			Spent:        utils.FromMilli(row.SpentMilli),
			Impressions:  row.Impressions,
			UnservedWins: row.UnservedWins,
		})
	}

	daily, err := s.repo.DailyAllImpressions()
	if err != nil {
		return BoostOverview{}, err
	}
	overview.Daily = make([]BoostDailyPoint, 0, len(daily))
	for _, row := range daily {
		overview.Daily = append(overview.Daily, BoostDailyPoint{
			Day:         row.Day,
			Impressions: row.Impressions,
			Spend:       utils.FromMilli(row.SpendMilli),
		})
	}
	return overview, nil
}
