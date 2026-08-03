package tests

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ── Side-effect stubs ──────────────────────────────────────────────
// These count calls so the idempotency tests can prove VerifyQR charges,
// punches, and triggers missions exactly once.

type stubTokenAccount struct {
	canConsume    bool
	consumeCalls  int
	consumeAmount int64 // milli
}

func (s *stubTokenAccount) GetOrCreate(uint, string) (*models.TokenAccount, error) {
	return nil, nil
}
func (s *stubTokenAccount) GetBalance(uint) (int64, int64, string, error) {
	return 0, 0, "", nil
}
func (s *stubTokenAccount) CanConsume(uint, int64) (bool, error) {
	return s.canConsume, nil
}
func (s *stubTokenAccount) Consume(_ uint, amountMilli int64, _ uint, _ string) error {
	s.consumeCalls++
	s.consumeAmount = amountMilli
	return nil
}
func (s *stubTokenAccount) ProcessTopUp(uint, uint, string) error {
	return nil
}
func (s *stubTokenAccount) AddTokens(uint, int64, string, uint, string) error {
	return nil
}

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
func (s *stubLoyalty) GetClientsCards(uint) ([]response.PunchCardResponse, error) {
	return nil, nil
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

// qrTestService builds the reservation service with a real SQLite repository
// and stub side-effect services so QR verification can be asserted exactly.
func qrTestService(t *testing.T, canConsume bool) (*service.ReservationServiceImpl, *stubTokenAccount, *stubLoyalty, *stubMission, *gorm.DB) {
	t.Helper()
	db := setupRouterAuthzDB(t)
	account := &stubTokenAccount{canConsume: canConsume}
	loyalty := &stubLoyalty{}
	mission := &stubMission{}
	svc := &service.ReservationServiceImpl{
		ReservationRepository: repository.NewReservationRepositoryImpl(db),
		PersonnelRepository:   repository.NewPersonnelRepositoryImpl(db),
		ServiceRepository:     repository.NewServiceRepositoryImpl(db),
		NotificationService:   service.NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), repository.NewPushDeviceRepositoryImpl(db)),
		LoyaltyService:        loyalty,
		TokenAccountService:   account,
		MissionService:        mission,
		Validate:              validator.New(),
	}
	return svc, account, loyalty, mission, db
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

// TestVerifyQR_CompletesExactlyOnce — the first verification charges, punches,
// and triggers missions exactly once, persists UPPERCASE COMPLETED, clears the
// token, and a later re-verification produces no extra side effects.
func TestVerifyQR_CompletesExactlyOnce(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, account, loyalty, mission, db := qrTestService(t, true)
	future := time.Now().Add(30 * time.Minute)
	res := seedQRReservation(t, db, "CONFIRMED", "TOK-ONCE", &future)

	err := svc.VerifyQR("TOK-ONCE", 1)
	assert.NoError(t, err)
	assert.Equal(t, 1, account.consumeCalls)
	assert.Equal(t, 1, loyalty.punchCalls)
	assert.Equal(t, 1, mission.triggerCalls)

	// Canonical uppercase persistence; token and expiry cleared.
	stored, err := svc.ReservationRepository.FindById(res.ID)
	assert.NoError(t, err)
	assert.Equal(t, "COMPLETED", stored.Status)
	assert.Nil(t, stored.VerificationToken)
	assert.Nil(t, stored.TokenExpiresAt)

	// Re-verification after completion: token is gone → nothing runs again.
	err = svc.VerifyQR("TOK-ONCE", 1)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
	assert.Equal(t, 1, account.consumeCalls)
	assert.Equal(t, 1, loyalty.punchCalls)
	assert.Equal(t, 1, mission.triggerCalls)
}

// TestVerifyQR_CompletedIsNoOp — a retry that finds a COMPLETED reservation
// still carrying its token (concurrent in-flight scan) must not charge, punch,
// or trigger missions; it is a success no-op.
func TestVerifyQR_CompletedIsNoOp(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, account, loyalty, mission, db := qrTestService(t, true)
	future := time.Now().Add(30 * time.Minute)
	seedQRReservation(t, db, "COMPLETED", "TOK-DONE", &future)

	err := svc.VerifyQR("TOK-DONE", 1)
	assert.NoError(t, err)
	assert.Equal(t, 0, account.consumeCalls)
	assert.Equal(t, 0, loyalty.punchCalls)
	assert.Equal(t, 0, mission.triggerCalls)
}

// TestVerifyQR_LegacyLowercaseCompletedIsNoOp — a legacy row written with a
// lowercase status must be treated as COMPLETED (normalized at read) and skip
// all completion side effects.
func TestVerifyQR_LegacyLowercaseCompletedIsNoOp(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, account, loyalty, mission, db := qrTestService(t, true)
	future := time.Now().Add(30 * time.Minute)
	seedQRReservation(t, db, "completed", "TOK-LEGACY", &future)

	err := svc.VerifyQR("TOK-LEGACY", 1)
	assert.NoError(t, err)
	assert.Equal(t, 0, account.consumeCalls)
	assert.Equal(t, 0, loyalty.punchCalls)
	assert.Equal(t, 0, mission.triggerCalls)
}

// TestVerifyQR_ErrorMapping — every VerifyQR failure must surface as its
// canonical sentinel so the controller maps it to the right HTTP status.
func TestVerifyQR_ErrorMapping(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(30 * time.Minute)

	tests := []struct {
		name       string
		token      string // token passed to VerifyQR
		seedToken  string // token stored on the reservation row
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
			svc, _, _, _, db := qrTestService(t, tt.canConsume)
			seedQRReservation(t, db, tt.status, tt.seedToken, tt.expiry)

			err := svc.VerifyQR(tt.token, tt.centerID)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// TestGenerateQR_TerminalStatesConflict — QR generation for terminal states
// (including legacy lowercase rows) must be rejected with 409, and no token
// may be issued.
func TestGenerateQR_TerminalStatesConflict(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	states := []struct {
		name   string
		status string
	}{
		{"COMPLETED reservation", "COMPLETED"},
		{"CANCELLED reservation", "CANCELLED"},
		{"legacy lowercase completed", "completed"},
	}

	for _, st := range states {
		t.Run(st.name, func(t *testing.T) {
			r, db := authzTestRouter(t)
			seedQRReservation(t, db, st.status, "", nil)

			owner := roleToken(t, 1, 1, "client")
			w := do(r, http.MethodPost, "/api/reservation/generate-qr/1", owner, "")

			assert.Equal(t, http.StatusConflict, w.Code)
			assert.Equal(t, "conflict", errBody(t, w.Body.String()))
			assert.NotContains(t, w.Body.String(), `"token":`)
		})
	}
}

// TestGenerateQR_PendingIssuesToken — a PENDING reservation still issues a
// six-character token.
func TestGenerateQR_PendingIssuesToken(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	r, db := authzTestRouter(t)
	seedQRReservation(t, db, "PENDING", "", nil)

	owner := roleToken(t, 1, 1, "client")
	w := do(r, http.MethodPost, "/api/reservation/generate-qr/1", owner, "")

	assert.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Token string `json:"token"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(t, body.Token, 6)
}
