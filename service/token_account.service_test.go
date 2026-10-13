package service

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	utils.InitLogger(utils.LoggerConfig{Level: "fatal", Format: "console"})
	// Set tier config path for tests
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filename)) // service/ -> yotuvia-api/
	os.Setenv("TIERS_CONFIG_PATH", filepath.Join(projectRoot, "configs", "tiers.json"))
	os.Exit(m.Run())
}

func setupTokenTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	err = db.AutoMigrate(
		&models.TokenAccount{},
		&models.TokenTopUp{},
		&models.TokenTransaction{},
		&models.TokenMission{},
		&models.TokenMissionProgress{},
	)
	if err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func createTestCenter(db *gorm.DB, id uint) {
	db.Create(&models.BeautyCenter{
		ID:       id,
		Name:     "Test Center",
		Email:    "test" + fmt.Sprint(id) + "@test.com",
		Phone:    "123456789",
		Password: "hashed",
		IsActive: true,
	})
}

// newTokenTestService wires the account service against the in-memory DB.
func newTokenTestService(db *gorm.DB) TokenAccountService {
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	missionSvc := NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	notifSvc := NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), repository.NewPushDeviceRepositoryImpl(db))
	return NewTokenAccountServiceImpl(accountRepo, topUpRepo, centerRepo, missionSvc, notifSvc)
}

func TestTokenAccountService_GetOrCreate(t *testing.T) {
	db := setupTokenTestDB(t)
	createTestCenter(db, 1)

	svc := newTokenTestService(db)

	// First call should create
	account, err := svc.GetOrCreate(1, "USD")
	assert.NoError(t, err)
	assert.NotNil(t, account)
	assert.Equal(t, int64(0), account.Balance)
	assert.Equal(t, "USD", account.Currency)

	// Second call should return existing
	account2, err := svc.GetOrCreate(1, "USD")
	assert.NoError(t, err)
	assert.Equal(t, account.ID, account2.ID)
}

func TestTokenAccountService_CanConsume(t *testing.T) {
	tests := []struct {
		name       string
		balance    int64 // milli
		overdraft  int64 // milli
		charge     int64 // milli
		canConsume bool
		purchased  int64 // milli
		topups     int
		daysActive int
	}{
		{
			name:       "positive balance covers charge",
			balance:    100_000,
			charge:     50_000,
			canConsume: true,
		},
		{
			name:       "exact balance matches charge",
			balance:    50_000,
			charge:     50_000,
			canConsume: true,
		},
		{
			name:       "overdraft covers deficit",
			balance:    2_000,
			charge:     5_000,
			canConsume: true, // 2000 + 5000 (new overdraft) >= 5000
			daysActive: 1,
		},
		{
			name:       "premium overdraft covers bigger deficit",
			balance:    10_000,
			charge:     35_000,
			canConsume: true, // 10000 + 30000 (premium) >= 35000
			purchased:  600_000,
			topups:     5,
		},
		{
			name:       "insufficient even with overdraft",
			balance:    1_000,
			charge:     50_000,
			canConsume: false,
		},
		{
			name:       "zero balance with new overdraft",
			balance:    0,
			charge:     5_000,
			canConsume: true, // 0 + 5000 >= 5000
			daysActive: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTokenTestDB(t)
			createTestCenter(db, 1)

			accountRepo := repository.NewTokenAccountRepositoryImpl(db)
			topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
			centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
			missionRepo := repository.NewTokenMissionRepositoryImpl(db)
			missionSvc := NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
			notifSvc := NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), repository.NewPushDeviceRepositoryImpl(db))
			svc := NewTokenAccountServiceImpl(accountRepo, topUpRepo, centerRepo, missionSvc, notifSvc)

			// Create account with specific created_at for overdraft tier
			createdAt := time.Now().AddDate(0, 0, -tt.daysActive)
			account := &models.TokenAccount{
				BeautyCenterID: 1,
				Balance:        tt.balance,
				TotalPurchased: tt.purchased,
				Currency:       "USD",
			}
			account.CreatedAt = createdAt
			accountRepo.Create(account)

			// Add topups if needed
			for i := 0; i < tt.topups; i++ {
				topUpRepo.Create(&models.TokenTopUp{
					BeautyCenterID: 1,
					AmountBs:       100,
					TokensReceived: 100_000,
					Status:         "approved",
					ReceiptURL:     "http://test.com/receipt",
				})
			}

			canConsume, err := svc.CanConsume(1, tt.charge)
			assert.NoError(t, err)
			assert.Equal(t, tt.canConsume, canConsume)
		})
	}
}

func TestTokenAccountService_Consume(t *testing.T) {
	// Consume succeeds when balance is 0 but overdraft covers the charge
	// (new-center tier = -5 YOTTU = 5000 milli).
	t.Run("consume with zero balance and overdraft", func(t *testing.T) {
		db := setupTokenTestDB(t)
		createTestCenter(db, 1)
		svc := newTokenTestService(db)
		svc.GetOrCreate(1, "USD")

		// balance is 0, overdraft is 5000. Consume 3000 should succeed.
		err := svc.Consume(1, 3_000, 0, "test consume")
		assert.NoError(t, err)

		balance, _, _, err := svc.GetBalance(1)
		assert.NoError(t, err)
		assert.Equal(t, int64(-3_000), balance, "balance should be -3 YOTTUs after consuming with overdraft")
	})

	// Consume fails when balance + overdraft < charge.
	t.Run("consume exceeds overdraft", func(t *testing.T) {
		db := setupTokenTestDB(t)
		createTestCenter(db, 1)
		svc := newTokenTestService(db)
		svc.GetOrCreate(1, "USD")

		// balance is 0, overdraft is 5000. Consume 6000 should fail.
		err := svc.Consume(1, 6_000, 0, "test consume overdraft exceeded")
		assert.ErrorIs(t, err, apperrors.ErrInsufficientBalance)
	})

	// Consume succeeds when positive balance covers the charge.
	t.Run("consume with positive balance", func(t *testing.T) {
		db := setupTokenTestDB(t)
		createTestCenter(db, 1)
		svc := newTokenTestService(db)
		svc.GetOrCreate(1, "USD")

		err := svc.AddTokens(1, 100_000, "topup", 1, "top-up")
		assert.NoError(t, err)

		err = svc.Consume(1, 50_000, 0, "test consume positive")
		assert.NoError(t, err)

		balance, _, _, err := svc.GetBalance(1)
		assert.NoError(t, err)
		assert.Equal(t, int64(50_000), balance)
	})
}

func TestTokenAccountService_AddTokens(t *testing.T) {
	db := setupTokenTestDB(t)
	createTestCenter(db, 1)

	svc := newTokenTestService(db)

	// Create account
	svc.GetOrCreate(1, "USD")

	// Add 100 tokens (100_000 milli)
	err := svc.AddTokens(1, 100_000, "topup", 1, "Test top-up")
	assert.NoError(t, err)

	balance, _, _, err := svc.GetBalance(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(100_000), balance)

	// Add 50 more (50_000 milli)
	err = svc.AddTokens(1, 50_000, "bonus", 1, "Test bonus")
	assert.NoError(t, err)

	balance, _, _, err = svc.GetBalance(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(150_000), balance)

	// Check transaction log
	txRepo2 := repository.NewTokenTransactionRepositoryImpl(db)
	txs, total, err := txRepo2.FindByBeautyCenterID(1, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Equal(t, "topup", txs[1].Type)
	assert.Equal(t, "bonus", txs[0].Type)
}

func TestCalculateTier(t *testing.T) {
	tests := []struct {
		name       string
		amount     float64
		currency   string
		wantTokens float64
		wantBonus  float64
		wantTotal  float64
	}{
		{"50 VES (no bonus)", 50, "VES", 50, 0, 50},
		{"100 VES (+5%)", 100, "VES", 100, 5, 105},
		{"275 VES (+9%)", 275, "VES", 275, 24.75, 299.75},
		{"500 VES (+15%)", 500, "VES", 500, 75, 575},
		{"1000 VES (+25%)", 1000, "VES", 1000, 250, 1250},
		{"10 USD (+5%)", 10, "USD", 10, 0.5, 10.5},
		{"50 USD (+15%)", 50, "USD", 50, 7.5, 57.5},
		{"100 USD (+25%)", 100, "USD", 100, 25, 125},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, bonus := utils.CalculateTier(tt.amount, tt.currency)
			assert.InDelta(t, tt.wantTokens, tokens, 0.01)
			assert.InDelta(t, tt.wantBonus, bonus, 0.01)
			assert.InDelta(t, tt.wantTotal, tokens+bonus, 0.01)
		})
	}
}

// TestTokenAccountService_OverdraftTiers tests the overdraft computation indirectly
func TestTokenAccountService_OverdraftTiers(t *testing.T) {
	db := setupTokenTestDB(t)
	createTestCenter(db, 1)
	createTestCenter(db, 2)
	createTestCenter(db, 3)

	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	missionSvc := NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	notifSvc := NewNotificationServiceImpl(repository.NewNotificationRepositoryImpl(db), repository.NewPushDeviceRepositoryImpl(db))
	svc := NewTokenAccountServiceImpl(accountRepo, topUpRepo, centerRepo, missionSvc, notifSvc)

	// Center 1: New (just created)
	svc.GetOrCreate(1, "USD")
	balance, overdraft, _, err := svc.GetBalance(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), balance)
	assert.Equal(t, int64(5000), overdraft) // new tier = -5 YOTTUs

	// Center 2: Active (30+ days old)
	oldAccount := &models.TokenAccount{
		BeautyCenterID: 2,
		Balance:        10_000,
		Currency:       "USD",
	}
	oldAccount.CreatedAt = time.Now().AddDate(0, 0, -45)
	accountRepo.Create(oldAccount)

	// Add 2 top-ups (not enough for premium)
	for i := 0; i < 2; i++ {
		topUpRepo.Create(&models.TokenTopUp{
			BeautyCenterID: 2,
			AmountBs:       100,
			TokensReceived: 100_000,
			Status:         "approved",
			ReceiptURL:     "http://test.com/receipt",
		})
	}

	balance, overdraft, _, err = svc.GetBalance(2)
	assert.NoError(t, err)
	assert.Equal(t, int64(15_000), overdraft) // active tier = -15 YOTTUs

	// Center 3: Premium (500+ purchased = 500_000+ milli)
	premAccount := &models.TokenAccount{
		BeautyCenterID: 3,
		Balance:        200_000,
		TotalPurchased: 600_000,
		Currency:       "USD",
	}
	premAccount.CreatedAt = time.Now().AddDate(0, 0, -5)
	accountRepo.Create(premAccount)

	// Add 3 top-ups
	for i := 0; i < 3; i++ {
		topUpRepo.Create(&models.TokenTopUp{
			BeautyCenterID: 3,
			AmountBs:       200,
			TokensReceived: 200_000,
			Status:         "approved",
			ReceiptURL:     "http://test.com/receipt",
		})
	}

	balance, overdraft, _, err = svc.GetBalance(3)
	assert.NoError(t, err)
	assert.Equal(t, int64(30_000), overdraft) // premium tier = -30 YOTTUs
}
