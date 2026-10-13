package tests

import (
	"errors"
	"testing"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  feat-token-missions — engine tests (Unit 1)
// ──────────────────────────────────────────────

type missionSUT struct {
	db          *gorm.DB
	missionSvc  service.TokenMissionService
	accountRepo repository.TokenAccountRepository
}

// newMissionSUT wires the real mission + account stacks over a fresh DB and
// seeds a center with a client, personnel, service and token account.
func newMissionSUT(t *testing.T) *missionSUT {
	t.Helper()
	db := setupRouterAuthzDB(t)

	seedDashboardCenter(t, db, 1, "Mission Center", "missions@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Svc", true)
	seedDashboardClient(t, db, 1, "Client", "One")

	account := &models.TokenAccount{BeautyCenterID: 1, Balance: 0, Currency: "BOB"}
	if err := db.Create(account).Error; err != nil {
		t.Fatalf("failed to seed token account: %v", err)
	}

	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	accountSvc := service.NewTokenAccountServiceImpl(accountRepo, nil, nil, nil, nil)
	svc := service.NewTokenMissionServiceImpl(missionRepo, accountSvc, accountRepo)
	return &missionSUT{db: db, missionSvc: svc, accountRepo: accountRepo}
}

// seedMission inserts an active mission directly.
func (s *missionSUT) seedMission(t *testing.T, name, trigger string, target int, reward int64, order int) *models.TokenMission {
	t.Helper()
	m := &models.TokenMission{
		Name: name, Description: "desc", TriggerType: trigger,
		TriggerCount: target, RewardTokens: reward, Icon: "🎯",
		IsActive: true, SortOrder: order,
	}
	if err := s.db.Create(m).Error; err != nil {
		t.Fatalf("failed to seed mission %s: %v", name, err)
	}
	return m
}

// TestMission_MultiTypeProgression is THE D1 regression: two active missions
// sharing trigger_type reservation_completed must co-progress.
func TestMission_MultiTypeProgression(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	sut := newMissionSUT(t)

	first := sut.seedMission(t, "First Completed Reservation", "reservation_completed", 1, 10000, 1)
	ladder := sut.seedMission(t, "10 Completed Reservations", "reservation_completed", 10, 6000, 6)

	// First completion: mission-1 completes; mission-2 sits at 1/10.
	assert.NoError(t, sut.missionSvc.TriggerCheck(1, "reservation_completed"))

	p1, err := sut.missionSvc.GetProgress(1)
	assert.NoError(t, err)
	progressByID := map[uint]models.TokenMissionProgress{}
	for _, p := range p1 {
		progressByID[p.MissionID] = p
	}

	if got, ok := progressByID[first.ID]; ok {
		assert.Equal(t, 1, got.CurrentCount)
		assert.NotNil(t, got.CompletedAt, "target reached → completion stamped")
	} else {
		t.Fatal("missing progress for first mission")
	}
	if got, ok := progressByID[ladder.ID]; ok {
		assert.Equal(t, 1, got.CurrentCount)
		assert.Nil(t, got.CompletedAt)
	} else {
		t.Fatal("missing progress for ladder mission")
	}

	// Second completion: only the ladder advances.
	assert.NoError(t, sut.missionSvc.TriggerCheck(1, "reservation_completed"))
	p2, _ := sut.missionSvc.GetProgress(1)
	for _, p := range p2 {
		if p.MissionID == ladder.ID {
			assert.Equal(t, 2, p.CurrentCount)
		}
		if p.MissionID == first.ID {
			assert.Equal(t, 1, p.CurrentCount, "completed mission frozen")
		}
	}
}

// TestMission_FrozenAfterComplete drives a mission to completion and proves
// extra triggers neither overshoot nor re-stamp completion.
func TestMission_FrozenAfterComplete(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	sut := newMissionSUT(t)

	mission := sut.seedMission(t, "Three Stamps", "reservation_completed", 3, 3000, 9)

	for i := 0; i < 5; i++ {
		assert.NoError(t, sut.missionSvc.TriggerCheck(1, "reservation_completed"))
	}

	rows := []models.TokenMissionProgress{}
	assert.NoError(t, sut.db.Where("mission_id = ?", mission.ID).Find(&rows).Error)
	if assert.Len(t, rows, 1) {
		assert.Equal(t, 3, rows[0].CurrentCount, "LEAST caps at target")
		assert.NotNil(t, rows[0].CompletedAt)
	}
}

// TestMission_Claim_DoubleClaimConflict covers the atomic claim: single credit,
// conflict on retry, and conflict when claiming an incomplete mission.
func TestMission_Claim_DoubleClaimConflict(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	sut := newMissionSUT(t)

	mission := sut.seedMission(t, "One Shot", "profile_completed", 1, 5000, 4)
	assert.NoError(t, sut.missionSvc.TriggerCheck(1, "profile_completed"))

	// Incomplete-mission guard (different center without progress).
	err := sut.missionSvc.ClaimReward(2, mission.ID)
	assert.True(t, errors.Is(err, apperrors.ErrConflict), "no progress row → guard rejects")

	// Happy path claim.
	assert.NoError(t, sut.missionSvc.ClaimReward(1, mission.ID))

	var account models.TokenAccount
	assert.NoError(t, sut.db.Where("beauty_center_id = 1").First(&account).Error)
	assert.Equal(t, int64(5000), account.Balance, "reward credited exactly once")

	var ledgers []models.TokenTransaction
	assert.NoError(t, sut.db.Where("beauty_center_id = 1 AND type = 'mission'").Find(&ledgers).Error)
	assert.Len(t, ledgers, 1)

	// Double claim → conflict, no second credit.
	err = sut.missionSvc.ClaimReward(1, mission.ID)
	assert.True(t, errors.Is(err, apperrors.ErrConflict))
	assert.NoError(t, sut.db.Where("beauty_center_id = 1").First(&account).Error)
	assert.Equal(t, int64(5000), account.Balance, "balance unchanged after rejected claim")
}
