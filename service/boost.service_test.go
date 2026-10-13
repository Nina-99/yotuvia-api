package service

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupBoostTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.BeautyCenter{},
		&models.TokenAccount{},
		&models.TokenTopUp{},
		&models.TokenTransaction{},
		&models.TokenMission{},
		&models.TokenMissionProgress{},
		&models.FeedPost{},
		// The feed repository now preloads the creator profile images for the
		// card avatar, so any DB that backs it must carry these tables.
		&models.BeautyCenterImage{},
		&models.PersonnelImage{},
		&models.BoostCampaign{},
	); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

// fundCenter creates the token account and credits `tokens` so a hold can debit.
func fundCenter(t *testing.T, db *gorm.DB, centerID uint, tokens float64) TokenAccountService {
	t.Helper()
	svc := newTokenTestService(db)
	if _, err := svc.GetOrCreate(centerID, "ARS"); err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	if err := svc.AddTokens(centerID, utils.ToMilli(tokens), "topup", 0, "seed"); err != nil {
		t.Fatalf("AddTokens: %v", err)
	}
	return svc
}

func createFeedPost(t *testing.T, db *gorm.DB, centerID uint) uint {
	t.Helper()
	post := models.FeedPost{
		BeautyCenterID: centerID,
		VideoURL:       "https://cdn.example.com/v.mp4",
		ThumbnailURL:   "https://cdn.example.com/t.jpg",
		IsActive:       true,
	}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("create feed post: %v", err)
	}
	return post.ID
}

func balanceMilli(t *testing.T, svc TokenAccountService, centerID uint) int64 {
	t.Helper()
	balance, _, _, err := svc.GetBalance(centerID)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	return balance
}

func TestBoostService_CreateHoldsBudgetUpFront(t *testing.T) {
	db := setupBoostTestDB(t)
	createTestCenter(db, 1)
	tokenSvc := fundCenter(t, db, 1, 100)
	postID := createFeedPost(t, db, 1)

	svc := NewBoostServiceImpl(repository.NewBoostCampaignRepositoryImpl(db), repository.NewFeedPostRepositoryImpl(db), tokenSvc)

	before := balanceMilli(t, tokenSvc, 1)
	budget := utils.ToMilli(20)
	now := time.Now()
	campaign, err := svc.CreateCampaign(1, postID, budget, 1000, now, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	if campaign.Status != BoostStatusActive {
		t.Fatalf("new campaign must be active, got %q", campaign.Status)
	}
	if after := balanceMilli(t, tokenSvc, 1); after != before-budget {
		t.Fatalf("budget must be held up front: before %d, after %d, budget %d", before, after, budget)
	}
}

func TestBoostService_CreateRejectsForeignPost(t *testing.T) {
	db := setupBoostTestDB(t)
	createTestCenter(db, 1)
	createTestCenter(db, 2)
	tokenSvc := fundCenter(t, db, 1, 100)
	foreignPost := createFeedPost(t, db, 2) // belongs to center 2

	svc := NewBoostServiceImpl(repository.NewBoostCampaignRepositoryImpl(db), repository.NewFeedPostRepositoryImpl(db), tokenSvc)

	now := time.Now()
	if _, err := svc.CreateCampaign(1, foreignPost, utils.ToMilli(10), 1000, now, now.Add(time.Hour)); err == nil {
		t.Fatal("promoting another center's post must be rejected")
	}
}

func TestBoostService_SettlementExhaustsBudgetAndDropsFromAuction(t *testing.T) {
	db := setupBoostTestDB(t)
	createTestCenter(db, 1)
	tokenSvc := fundCenter(t, db, 1, 100)
	postID := createFeedPost(t, db, 1)

	svc := NewBoostServiceImpl(repository.NewBoostCampaignRepositoryImpl(db), repository.NewFeedPostRepositoryImpl(db), tokenSvc)

	now := time.Now()
	// budget 10 tokens; bid 1000 milli/CPM → 1 milli per impression.
	campaign, err := svc.CreateCampaign(1, postID, utils.ToMilli(10), 1000, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}

	if err := svc.SettleImpressions(campaign.ID, 10_000, 1000); err != nil {
		t.Fatalf("SettleImpressions: %v", err)
	}
	stored, _ := repository.NewBoostCampaignRepositoryImpl(db).FindByID(campaign.ID)
	if stored.Status != BoostStatusExhausted {
		t.Fatalf("campaign must be exhausted, got %q (spent %d)", stored.Status, stored.SpentMilli)
	}
	if stored.SpentMilli != utils.ToMilli(10) {
		t.Fatalf("spend must equal budget: got %d", stored.SpentMilli)
	}

	winners, err := svc.SelectWinners(now, 3, 100)
	if err != nil {
		t.Fatalf("SelectWinners: %v", err)
	}
	if len(winners) != 0 {
		t.Fatalf("exhausted campaign must not win a slot: %v", winners)
	}
}

func TestBoostService_SecondPriceAuctionAndPause(t *testing.T) {
	db := setupBoostTestDB(t)
	createTestCenter(db, 1)
	createTestCenter(db, 2)
	tokenSvc := fundCenter(t, db, 1, 100)
	fundCenter(t, db, 2, 100)
	postA := createFeedPost(t, db, 1)
	postB := createFeedPost(t, db, 2)

	svc := NewBoostServiceImpl(repository.NewBoostCampaignRepositoryImpl(db), repository.NewFeedPostRepositoryImpl(db), tokenSvc)

	now := time.Now()
	high, err := svc.CreateCampaign(1, postA, utils.ToMilli(50), 800, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("campaign A: %v", err)
	}
	if _, err := svc.CreateCampaign(2, postB, utils.ToMilli(50), 300, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("campaign B: %v", err)
	}

	// One slot, two bidders → the high bidder wins and pays the highest LOSING
	// bid (300), not its own 800.
	winners, err := svc.SelectWinners(now, 1, 100)
	if err != nil {
		t.Fatalf("SelectWinners: %v", err)
	}
	if len(winners) != 1 || winners[0].CampaignID != high.ID {
		t.Fatalf("highest bidder must win: %v", winners)
	}
	if winners[0].PriceCPMMilli != 300 {
		t.Fatalf("second price must be 300, got %d", winners[0].PriceCPMMilli)
	}

	// Pausing the winner removes it; the only remaining campaign wins at price 0
	// floored to the reserve.
	if err := svc.PauseCampaign(1, high.ID); err != nil {
		t.Fatalf("PauseCampaign: %v", err)
	}
	winners, err = svc.SelectWinners(now, 1, 100)
	if err != nil {
		t.Fatalf("SelectWinners after pause: %v", err)
	}
	if len(winners) != 1 || winners[0].PriceCPMMilli != 100 {
		t.Fatalf("paused campaign must drop out and reserve must floor: %v", winners)
	}
}
