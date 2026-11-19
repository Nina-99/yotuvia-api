package service

// Frequency cap and delivery-miss accounting for the ad auction.

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"gorm.io/gorm"
)

// newAuctionSvc builds the real service over a seeded centre + post.
func newAuctionSvc(t *testing.T) (BoostService, *gorm.DB, uint) {
	t.Helper()
	db := setupBoostTestDB(t)
	createTestCenter(db, 1)
	tokenSvc := fundCenter(t, db, 1, 100)
	post := createFeedPost(t, db, 1)
	svc := NewBoostServiceImpl(
		repository.NewBoostCampaignRepositoryImpl(db),
		repository.NewFeedPostRepositoryImpl(db),
		tokenSvc,
		repository.NewTokenAccountRepositoryImpl(db),
		nil,
	)
	return svc, db, post
}

// TestBoostAuction_FrequencyCapStopsServingOneViewer — the cap is measured in
// BILLED views: the advertiser pays per qualified view, so that is the number
// that must stop growing for one person.
func TestBoostAuction_FrequencyCapStopsServingOneViewer(t *testing.T) {
	t.Setenv("BOOST_FREQUENCY_CAP", "2")

	svc, db, post := newAuctionSvc(t)
	now := time.Now()
	campaign, err := svc.CreateCampaign(1, post, utils.ToMilli(50), 800, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}

	answered := func(viewerID uint) bool {
		winners, err := svc.SelectWinners(now, 1, 1, nil, viewerID)
		if err != nil {
			t.Fatalf("SelectWinners: %v", err)
		}
		for _, w := range winners {
			if w.CampaignID == campaign.ID {
				return true
			}
		}
		return false
	}

	// Two billed views by viewer 5: still under/at the cap boundary.
	for i := 0; i < 2; i++ {
		if err := db.Create(&models.BoostImpression{
			CampaignID: campaign.ID, ViewerID: 5, Nonce: "n" + string(rune('a'+i)), PriceCPMMilli: 300,
		}).Error; err != nil {
			t.Fatalf("seed impression: %v", err)
		}
	}

	if answered(5) {
		t.Fatal("a viewer who already produced 2 billed views must not be served the campaign again")
	}
	if !answered(6) {
		t.Fatal("a different viewer must still be served")
	}

	// Disabling the cap restores the old behaviour.
	t.Setenv("BOOST_FREQUENCY_CAP", "0")
	if !answered(5) {
		t.Fatal("cap 0 must disable the frequency limit")
	}
}

// TestBoostService_RecordsUnservedWins — a campaign that wins but cannot be
// rendered must leave a trace the advertiser can read.
func TestBoostService_RecordsUnservedWins(t *testing.T) {
	svc, db, post := newAuctionSvc(t)
	now := time.Now()
	campaign, err := svc.CreateCampaign(1, post, utils.ToMilli(50), 800, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}

	if err := svc.RecordUnservedWins([]uint{campaign.ID, campaign.ID}); err != nil {
		t.Fatalf("RecordUnservedWins: %v", err)
	}

	var stored models.BoostCampaign
	if err := db.First(&stored, campaign.ID).Error; err != nil {
		t.Fatalf("reload campaign: %v", err)
	}
	// UpdateColumn with `+ 1` runs once for the set, regardless of duplicates.
	if stored.UnservedWins != 1 {
		t.Fatalf("unserved wins must be booked once per call, got %d", stored.UnservedWins)
	}

	// An empty set is a no-op rather than a query.
	if err := svc.RecordUnservedWins(nil); err != nil {
		t.Fatalf("empty RecordUnservedWins must be a no-op: %v", err)
	}
}

// TestBoostAuction_FrequencyLookupFailureDoesNotBreakTheAuction — a cap lookup
// error degrades to "no cap", never to "no ads".
func TestBoostAuction_FrequencyLookupFailureDoesNotBreakTheAuction(t *testing.T) {
	svc, db, post := newAuctionSvc(t)
	now := time.Now()
	campaign, err := svc.CreateCampaign(1, post, utils.ToMilli(50), 800, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}

	// Drop the impressions table: the counter query fails.
	if err := db.Migrator().DropTable(&models.BoostImpression{}); err != nil {
		t.Fatalf("drop table: %v", err)
	}

	winners, err := svc.SelectWinners(now, 1, 1, nil, 5)
	if err != nil {
		t.Fatalf("the auction must not fail when the cap lookup does: %v", err)
	}
	served := false
	for _, w := range winners {
		if w.CampaignID == campaign.ID {
			served = true
		}
	}
	if !served {
		t.Fatal("a cap lookup failure must not suppress the ad")
	}
}
