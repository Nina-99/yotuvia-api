package tests

import (
	"errors"
	"testing"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  feat-token-missions — trigger wiring (Unit 2)
// ──────────────────────────────────────────────

// failingMissionService always errors — proves primary flows are immune to
// mission infrastructure failures (best-effort contract, D6).
type failingMissionService struct{}

func (f *failingMissionService) TriggerCheck(beautyCenterID uint, triggerType string) error {
	return errors.New("mission infra down")
}
func (f *failingMissionService) ClaimReward(beautyCenterID, missionID uint) error {
	return errors.New("mission infra down")
}
func (f *failingMissionService) GetProgress(beautyCenterID uint) ([]models.TokenMissionProgress, error) {
	return nil, errors.New("mission infra down")
}
func (f *failingMissionService) GetAvailableMissions() ([]models.TokenMission, error) {
	return nil, errors.New("mission infra down")
}

func newTriggerSUT(t *testing.T) (*gorm.DB, service.TokenMissionService) {
	t.Helper()
	db := setupRouterAuthzDB(t)
	seedDashboardCenter(t, db, 1, "Trigger Center", "trigger@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Svc", true)
	seedDashboardClient(t, db, 1, "Client", "One")

	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	missionSvc := service.NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	return db, missionSvc
}

func missionProgress(t *testing.T, db *gorm.DB, centerID, missionID uint) models.TokenMissionProgress {
	t.Helper()
	var p models.TokenMissionProgress
	err := db.Where("beauty_center_id = ? AND mission_id = ?", centerID, missionID).
		First(&p).Error
	assert.NoError(t, err)
	return p
}

// TestTrigger_ReviewReceived covers T5: creating reviews advances the
// reputation mission, and mission failures never block the business write.
func TestTrigger_ReviewReceived(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	db, missionSvc := newTriggerSUT(t)

	// Client 1 must have a COMPLETED reservation at center 1 to review it.
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: utcMidnight(-1), startTime: "09:00", endTime: "10:00",
		status: "COMPLETED", totalPrice: 100, clientID: 1, personnelID: 1, serviceIDs: []uint{1},
	})

	mission := &models.TokenMission{
		Name: "Receive 5 Reviews", Description: "", TriggerType: "review_received",
		TriggerCount: 5, RewardTokens: 3000, Icon: "💬", IsActive: true,
	}
	assert.NoError(t, db.Create(mission).Error)

	notifSvc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)
	validate := validator.New()

	// Failure isolation first: broken mission infra does not block reviews.
	brokenReviewSvc := service.NewReviewServiceImpl(
		repository.NewReviewRepositoryImpl(db), notifSvc, &failingMissionService{}, validate)
	req := request.CreateReviewRequest{BeautyCenterID: 1, Rating: 5, Comment: "great"}
	assert.NoError(t, brokenReviewSvc.Create(1, req))

	// Healthy wiring: second client reviews the same center.
	seedDashboardClient(t, db, 2, "Second", "Client")
	seedReservationWithServices(t, db, 1, seedReservationPlan{
		date: utcMidnight(-1), startTime: "11:00", endTime: "12:00",
		status: "COMPLETED", totalPrice: 90, clientID: 2, personnelID: 1, serviceIDs: []uint{1},
	})
	reviewSvc := service.NewReviewServiceImpl(
		repository.NewReviewRepositoryImpl(db), notifSvc, missionSvc, validate)
	assert.NoError(t, reviewSvc.Create(2, request.CreateReviewRequest{BeautyCenterID: 1, Rating: 4, Comment: "ok"}))

	p := missionProgress(t, db, 1, mission.ID)
	assert.Equal(t, 1, p.CurrentCount,
		"broken-infra review did NOT advance (isolated); healthy wiring did")
	assert.Nil(t, p.CompletedAt)
}

// TestTrigger_ProfileCompleted covers T6: the mission fires only on the
// incomplete→complete transition of the D4 checklist.
func TestTrigger_ProfileCompleted(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	db, missionSvc := newTriggerSUT(t)

	// Center 1 is INCOMPLETE: seedDashboardCenter sets name/email/phone only.
	// Give it schedules and images so basics are the missing piece.
	assert.NoError(t, db.Create(&models.WorkingHour{
		BeautyCenterID: 1, DayOfWeek: 1, StartTime: "09:00", EndTime: "17:00",
	}).Error)
	assert.NoError(t, db.Create(&models.BeautyCenterImage{
		BeautyCenterID: 1, ImageUrl: "https://cdn.test/1.jpg", Position: 0,
	}).Error)

	mission := &models.TokenMission{
		Name: "Profile Star", Description: "", TriggerType: "profile_completed",
		TriggerCount: 1, RewardTokens: 5000, Icon: "⭐", IsActive: true,
	}
	assert.NoError(t, db.Create(mission).Error)

	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	svc := service.NewBeautyCenterServiceImpl(centerRepo, missionSvc, validator.New())

	// Completing update: description + address were missing.
	req := request.UpdateBeautyCenterRequest{
		Description: "A full-service salon",
		Address:     "Av. Siempre Viva 742",
	}
	_, err := svc.Update(1, req)
	assert.NoError(t, err)

	p := missionProgress(t, db, 1, mission.ID)
	assert.Equal(t, 1, p.CurrentCount)
	assert.NotNil(t, p.CompletedAt)

	// Repeated updates never re-fire.
	req.Name = "Trigger Center Renamed"
	_, err = svc.Update(1, req)
	assert.NoError(t, err)
	p = missionProgress(t, db, 1, mission.ID)
	assert.Equal(t, 1, p.CurrentCount, "completed mission frozen")
}

// TestTrigger_StampActivated covers T7 plus the drive-by persistence fix:
// the program row must exist and the mission must advance.
func TestTrigger_StampActivated(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	db, missionSvc := newTriggerSUT(t)

	mission := &models.TokenMission{
		Name: "Stamp Promoter", Description: "", TriggerType: "stamp_activated",
		TriggerCount: 1, RewardTokens: 15000, Icon: "🎟️", IsActive: true,
	}
	assert.NoError(t, db.Create(mission).Error)

	svc := service.NewLoyaltyService(
		repository.NewLoyaltyRepositoryImpl(db),
		repository.NewBeautyCenterRepositoryImpl(db),
		missionSvc, validator.New())

	serviceID := uint(1)
	resp, err := svc.CreateProgram(request.CreateLoyaltyProgrmaRequest{
		BeautyCenterID: 1, ServiceID: &serviceID, RequiredSteps: 5,
		RewardType: "percentage_discount", DiscountValue: 10,
	})
	assert.NoError(t, err)
	assert.NotZero(t, resp.ID, "program persisted (drive-by fix)")

	var count int64
	assert.NoError(t, db.Model(&models.LoyaltyProgram{}).Where("beauty_center_id = 1").Count(&count).Error)
	assert.Equal(t, int64(1), count, "program row exists in DB")

	p := missionProgress(t, db, 1, mission.ID)
	assert.Equal(t, 1, p.CurrentCount)
	assert.NotNil(t, p.CompletedAt)
}

// TestTrigger_ReferralVerification covers T8: the referrer's mission fires
// when the referred center becomes VERIFIED, exactly once.
func TestTrigger_ReferralVerification(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	db, missionSvc := newTriggerSUT(t)

	seedDashboardCenter(t, db, 2, "Referred Salon", "referred@test.com", true)
	seedDashboardCenter(t, db, 3, "Lone Salon", "lone@test.com", true)

	mission := &models.TokenMission{
		Name: "Refer a Colleague", Description: "", TriggerType: "referral",
		TriggerCount: 1, RewardTokens: 30000, Icon: "🤝", IsActive: true,
	}
	assert.NoError(t, db.Create(mission).Error)

	// Referral: center 1 referred center 2.
	referral := &models.Referral{ReferrerID: 1, ReferredID: 2, Status: "pending"}
	assert.NoError(t, db.Create(referral).Error)

	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	svc := service.NewBeautyCenterServiceImpl(centerRepo, missionSvc, validator.New())

	// Verify the referred center → referrer credited.
	assert.NoError(t, svc.MarkVerified(2))
	p := missionProgress(t, db, 1, mission.ID)
	assert.Equal(t, 1, p.CurrentCount)
	assert.NotNil(t, p.CompletedAt)

	// Idempotent: verifying again changes nothing.
	assert.NoError(t, svc.MarkVerified(2))
	p = missionProgress(t, db, 1, mission.ID)
	assert.Equal(t, 1, p.CurrentCount)

	// Verified WITHOUT referral: no crash, no phantom progress elsewhere.
	assert.NoError(t, svc.MarkVerified(3))

	// Referrer itself gets verified: no self-attribution noise.
	assert.NoError(t, svc.MarkVerified(1))

	var total int64
	assert.NoError(t, db.Model(&models.TokenMissionProgress{}).
		Where("mission_id = ?", mission.ID).Count(&total).Error)
	assert.Equal(t, int64(1), total, "only the referrer holds progress on this mission")
}
