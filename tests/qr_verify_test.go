package tests

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/go-playground/validator/v10"
)

// ──────────────────────────────────────────────
//  QR verification under the ATOMIC charge contract (feat-token-economy-
//  config): completion + debit + ledger share one transaction. Side effects
//  (punches/missions) stay best-effort post-commit; loyalty/mission are stubs
//  with call counters, token money is asserted through REAL account state.
// ──────────────────────────────────────────────

type stubLoyalty struct {
	punchCalls int
}

func (s *stubLoyalty) CreateProgram(request.CreateLoyaltyProgrmaRequest) (response.LoyaltyProgramResponse, error) {
	return response.LoyaltyProgramResponse{}, nil
}
func (s *stubLoyalty) AddPunch(uint, uint, uint, uint) error {
	s.punchCalls++
	return nil
}
func (s *stubLoyalty) GetClientsCards(uint, uint, string) ([]response.PunchCardResponse, error) {
	return nil, nil
}
func (s *stubLoyalty) GetHistory(uint, uint, string) ([]response.PunchHistoryItemResponse, error) {
	return nil, nil
}
func (s *stubLoyalty) GetPendingRewards(uint, uint, string) ([]response.PendingRewardResponse, error) {
	return nil, nil
}
func (s *stubLoyalty) GetPublicPrograms(uint) ([]response.PublicProgramResponse, error) {
	return nil, nil
}
func (s *stubLoyalty) HasPendingRedemption(uint, uint, uint) bool { return false }
func (s *stubLoyalty) CalculateRedemption(uint, uint, uint) (float64, uint, error) {
	return 0, 0, nil
}
func (s *stubLoyalty) FinalizeRedemption(uint, uint) error   { return nil }
func (s *stubLoyalty) FindRedemptionByReservation(uint) bool { return false }
func (s *stubLoyalty) FindProgramsByCenter(uint, uint) ([]response.LoyaltyProgramResponse, error) {
	return nil, nil
}
func (s *stubLoyalty) UpdateProgram(uint, uint, request.UpdateLoyaltyProgramRequest) (response.LoyaltyProgramResponse, error) {
	return response.LoyaltyProgramResponse{}, nil
}
func (s *stubLoyalty) DeleteProgram(uint, uint) error       { return nil }
func (s *stubLoyalty) ToggleProgram(uint, uint, bool) error { return nil }
func (s *stubLoyalty) GetCenterSettings(uint, uint) (response.LoyaltySettingsResponse, error) {
	// VerifyQR gates punching on the center's loyalty setting. The zero value
	// reports loyalty DISABLED, which would silently skip the punch block; this
	// stub stands in for an opted-in center so the stamp path is exercised.
	// Idempotent re-scans return before this call (no-op), so they still punch 0.
	return response.LoyaltySettingsResponse{LoyaltyEnabled: true}, nil
}
func (s *stubLoyalty) UpdateCenterSettings(uint, uint, request.UpdateLoyaltySettingsRequest) error {
	return nil
}
func (s *stubLoyalty) GetMetrics(uint, uint) ([]response.LoyaltyMetricsResponse, error) {
	return []response.LoyaltyMetricsResponse{}, nil
}

type stubMission struct {
	triggerCalls int
}

func (s *stubMission) TriggerCheck(uint, string) error {
	s.triggerCalls++
	return nil
}
func (s *stubMission) ClaimReward(uint, uint) error {
	return nil
}
func (s *stubMission) GetProgress(uint) ([]models.TokenMissionProgress, error) {
	return nil, nil
}
func (s *stubMission) GetAvailableMissions() ([]models.TokenMission, error) {
	return nil, nil
}

// qrTestService builds the reservation service over a real SQLite DB with a
// REAL token account stack. startingBalanceMilli seeds the center's account
// so callers control the insufficient-funds boundary.
func qrTestService(t *testing.T, startingBalanceMilli int64) (*service.ReservationServiceImpl, *models.TokenAccount, *stubLoyalty, *stubMission, *gorm.DB) {
	t.Helper()
	db := setupRouterAuthzDB(t)
	seedDashboardCenter(t, db, 1, "QR Center", "qr@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardService(t, db, 1, 1, "Svc", true)
	seedDashboardClient(t, db, 1, "Client", "One")

	account := &models.TokenAccount{BeautyCenterID: 1, Balance: startingBalanceMilli, Currency: "BOB"}
	if err := db.Create(account).Error; err != nil {
		t.Fatalf("failed to seed token account: %v", err)
	}

	loyalty := &stubLoyalty{}
	mission := &stubMission{}
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)

	// Two-phase wiring (same as reservation.router.go): mission svc needs
	// account svc for claims; account svc needs mission svc AND a real
	// top-up repo for computeOverdraft (GetBalance path).
	notifSvc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)
	missionSvc := mission // counting stub: VerifyQR must trigger exactly once
	accountSvc := service.NewTokenAccountServiceImpl(accountRepo, topUpRepo, nil, missionSvc, notifSvc)

	svc := &service.ReservationServiceImpl{
		ReservationRepository: repository.NewReservationRepositoryImpl(db),
		PersonnelRepository:   repository.NewPersonnelRepositoryImpl(db),
		ServiceRepository:     repository.NewServiceRepositoryImpl(db),
		ClientRepository:      repository.NewClientRepositoryImpl(db),
		TokenAccountRepo:      accountRepo,
		NotificationService:   notifSvc,
		LoyaltyService:        loyalty,
		TokenAccountService:   accountSvc,
		MissionService:        missionSvc,
		Validate:              validator.New(),
	}
	return svc, account, loyalty, mission, db
}

func consumptionLedgerCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	assert.NoError(t, db.Model(&models.TokenTransaction{}).
		Where("beauty_center_id = 1 AND type = 'consumption'").Count(&n).Error)
	return n
}

// seedQRReservation creates a reservation (one service, price 100) with the
// given status and optional verification token/expiry.
func seedQRReservation(t *testing.T, db *gorm.DB, status, token string, expiry *time.Time) *models.Reservation {
	t.Helper()
	res := &models.Reservation{
		BeautyCenterID: 1,
		ClientID:       1,
		PersonnelID:    1,
		Status:         status,
		TotalPrice:     100,
		ReservationServices: []models.ReservationService{{
			ServiceID:   1,
			ServiceName: "Corte",
			Price:       50,
			Duration:    30,
		}},
	}
	if token != "" {
		res.VerificationToken = &token
		res.TokenExpiresAt = expiry
	}
	if err := db.Create(res).Error; err != nil {
		t.Fatalf("failed to seed QR reservation: %v", err)
	}
	return res
}

// TestVerifyQR_CompletesExactlyOnce — first scan completes + charges
// atomically (3.5 YOTTU for a 100 Bs ticket), punches and triggers missions
// exactly once, persists UPPERCASE COMPLETED, clears the token; the re-scan
// finds no token (ErrNotFound) without a second charge.
func TestVerifyQR_CompletesExactlyOnce(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, _, loyalty, mission, db := qrTestService(t, 100_000)
	future := time.Now().Add(30 * time.Minute)
	res := seedQRReservation(t, db, "CONFIRMED", "TOK-ONCE", &future)

	completed, already, err := svc.VerifyQR("TOK-ONCE", 1)
	assert.NoError(t, err)
	assert.False(t, already, "this call completed the reservation -> already_completed=false")
	assert.Equal(t, res.ID, completed.ID)
	assert.Equal(t, "COMPLETED", completed.Status)

	// Atomic charge: 3.5% of 100 Bs = 3.5 YOTTU = 3500 milli.
	assert.Equal(t, int64(96_500), accountBalance(t, db))
	assert.Equal(t, int64(1), consumptionLedgerCount(t, db))
	assert.Equal(t, 1, loyalty.punchCalls)
	assert.Equal(t, 1, mission.triggerCalls)

	stored, err := svc.ReservationRepository.FindById(res.ID)
	assert.NoError(t, err)
	assert.Equal(t, "COMPLETED", stored.Status)
	assert.Nil(t, stored.VerificationToken)
	assert.Nil(t, stored.TokenExpiresAt)

	_, _, err = svc.VerifyQR("TOK-ONCE", 1)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
	assert.Equal(t, int64(96_500), accountBalance(t, db), "no second charge")
	assert.Equal(t, int64(1), consumptionLedgerCount(t, db))
}

// TestVerifyQR_CompletedIsNoOp — a COMPLETED reservation still carrying its
// token (concurrent in-flight scan scenario) is a success no-op.
func TestVerifyQR_CompletedIsNoOp(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, _, loyalty, mission, db := qrTestService(t, 100_000)
	future := time.Now().Add(30 * time.Minute)
	seedQRReservation(t, db, "COMPLETED", "TOK-DONE", &future)

	completed, already, err := svc.VerifyQR("TOK-DONE", 1)
	assert.NoError(t, err)
	assert.True(t, already, "reservation was already completed -> already_completed=true")
	assert.NotNil(t, completed)
	assert.Equal(t, "COMPLETED", completed.Status)
	assert.Equal(t, 0, loyalty.punchCalls)
	assert.Equal(t, 0, mission.triggerCalls)
}

// TestVerifyQR_LegacyLowercaseCompletedIsNoOp — legacy lowercase rows are
// COMPLETED at read (normalized) and skip all side effects.
func TestVerifyQR_LegacyLowercaseCompletedIsNoOp(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, _, loyalty, mission, db := qrTestService(t, 100_000)
	future := time.Now().Add(30 * time.Minute)
	seedQRReservation(t, db, "completed", "TOK-LEGACY", &future)

	_, already, err := svc.VerifyQR("TOK-LEGACY", 1)
	assert.NoError(t, err)
	assert.True(t, already, "legacy lowercase completed row is still already_completed")
	assert.Equal(t, 0, loyalty.punchCalls)
	assert.Equal(t, 0, mission.triggerCalls)
}

// TestVerifyQR_ErrorMapping — canonical sentinels per failure class. The
// insufficient case drives the NEW atomic guard by forcing a negative balance
// far below the overdraft allowance.
func TestVerifyQR_ErrorMapping(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(30 * time.Minute)

	tests := []struct {
		name       string
		token      string
		seedToken  string
		centerID   uint
		status     string
		expiry     *time.Time
		canConsume bool
		wantErr    error
	}{
		{"unknown token is ErrNotFound", "TOK-NOPE", "", 1, "PENDING", &future, true, apperrors.ErrNotFound},
		{"foreign center is ErrForbidden", "TOK-FOREIGN", "TOK-FOREIGN", 2, "CONFIRMED", &future, true, apperrors.ErrForbidden},
		{"expired token is ErrBadRequest", "TOK-EXPIRED", "TOK-EXPIRED", 1, "CONFIRMED", &past, true, apperrors.ErrBadRequest},
		{"cancelled is ErrConflict", "TOK-CANCELLED", "TOK-CANCELLED", 1, "CANCELLED", &future, true, apperrors.ErrConflict},
		{"insufficient balance is ErrInsufficientBalance", "TOK-BROKE", "TOK-BROKE", 1, "CONFIRMED", &future, false, apperrors.ErrInsufficientBalance},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, account, _, _, db := qrTestService(t, 100_000)
			if !tt.canConsume {
				// Force the atomic guard: balance far below -overdraft.
				assert.NoError(t, db.Model(account).Update("balance", -10_000).Error)
			}
			seedQRReservation(t, db, tt.status, tt.seedToken, tt.expiry)

			_, _, err := svc.VerifyQR(tt.token, tt.centerID)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// TestVerifyQR_LostRaceAlreadyCompleted — a NO-SHOW status passes the
// terminal check but is excluded by the conditional update's IN guard:
// 0 rows -> success no-op with already_completed=true, no debit.
func TestVerifyQR_LostRaceAlreadyCompleted(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, account, _, _, db := qrTestService(t, 100_000)
	future := time.Now().Add(30 * time.Minute)
	seedQRReservation(t, db, "NO-SHOW", "TOK-RACE", &future)

	_, already, err := svc.VerifyQR("TOK-RACE", 1)
	assert.NoError(t, err)
	assert.True(t, already, "0-row completion must report already_completed=true")
	assert.Equal(t, int64(100_000), account.Balance, "no charge when the exact-once guard refuses")
	assert.Equal(t, int64(0), consumptionLedgerCount(t, db))
}
