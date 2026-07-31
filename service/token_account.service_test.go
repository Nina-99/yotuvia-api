package service

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	utils.InitLogger(utils.LoggerConfig{Level: "fatal", Format: "console"})
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

func TestTokenAccountService_GetOrCreate(t *testing.T) {
	db := setupTokenTestDB(t)
	createTestCenter(db, 1)

	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	txRepo := repository.NewTokenTransactionRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	missionSvc := NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	svc := NewTokenAccountServiceImpl(accountRepo, topUpRepo, txRepo, centerRepo, missionSvc)

	// First call should create
	account, err := svc.GetOrCreate(1, "USD")
	assert.NoError(t, err)
	assert.NotNil(t, account)
	assert.Equal(t, 0.0, account.Balance)
	assert.Equal(t, "USD", account.Currency)

	// Second call should return existing
	account2, err := svc.GetOrCreate(1, "USD")
	assert.NoError(t, err)
	assert.Equal(t, account.ID, account2.ID)
}

func TestTokenAccountService_CanConsume(t *testing.T) {
	tests := []struct {
		name        string
		balance     float64
		overdraft   float64
		charge      float64
		canConsume  bool
		purchased   float64
		topups      int
		daysActive  int
	}{
		{
			name:       "positive balance covers charge",
			balance:    100,
			charge:     50,
			canConsume: true,
		},
		{
			name:       "exact balance matches charge",
			balance:    50,
			charge:     50,
			canConsume: true,
		},
		{
			name:       "overdraft covers deficit",
			balance:    2,
			charge:     5,
			canConsume: true, // 2 + 5 (new overdraft) >= 5
			daysActive: 1,
		},
		{
			name:       "premium overdraft covers bigger deficit",
			balance:    10,
			charge:     35,
			canConsume: true, // 10 + 30 (premium) >= 35
			purchased:  600,
			topups:     5,
		},
		{
			name:       "insufficient even with overdraft",
			balance:    1,
			charge:     50,
			canConsume: false,
		},
		{
			name:       "zero balance with new overdraft",
			balance:    0,
			charge:     5,
			canConsume: true, // 0 + 5 >= 5
			daysActive: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTokenTestDB(t)
			createTestCenter(db, 1)

			accountRepo := repository.NewTokenAccountRepositoryImpl(db)
			topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
			txRepo := repository.NewTokenTransactionRepositoryImpl(db)
			centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
			missionRepo := repository.NewTokenMissionRepositoryImpl(db)
			missionSvc := NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)

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
					TokensReceived: 100,
					Status:         "approved",
					ReceiptURL:     "http://test.com/receipt",
				})
			}

			svc := NewTokenAccountServiceImpl(accountRepo, topUpRepo, txRepo, centerRepo, missionSvc)

			canConsume, err := svc.CanConsume(1, tt.charge)
			assert.NoError(t, err)
			assert.Equal(t, tt.canConsume, canConsume)
		})
	}
}

func TestTokenAccountService_AddTokens(t *testing.T) {
	db := setupTokenTestDB(t)
	createTestCenter(db, 1)

	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	topUpRepo := repository.NewTokenTopUpRepositoryImpl(db)
	txRepo := repository.NewTokenTransactionRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	missionSvc := NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	svc := NewTokenAccountServiceImpl(accountRepo, topUpRepo, txRepo, centerRepo, missionSvc)

	// Create account
	svc.GetOrCreate(1, "USD")

	// Add 100 tokens
	err := svc.AddTokens(1, 100, "topup", 1, "Test top-up")
	assert.NoError(t, err)

	balance, _, _, err := svc.GetBalance(1)
	assert.NoError(t, err)
	assert.Equal(t, 100.0, balance)

	// Add 50 more
	err = svc.AddTokens(1, 50, "bonus", 1, "Test bonus")
	assert.NoError(t, err)

	balance, _, _, err = svc.GetBalance(1)
	assert.NoError(t, err)
	assert.Equal(t, 150.0, balance)

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
		name          string
		amount        float64
		wantTokens    float64
		wantBonus     float64
		wantTotal     float64
	}{
		{"50 Bs (no bonus)", 50, 50, 0, 50},
		{"100 Bs (+5%)", 100, 100, 5, 105},
		{"275 Bs (+9%)", 275, 275, 24.75, 299.75},
		{"500 Bs (+15%)", 500, 500, 75, 575},
		{"1000 Bs (+25%)", 1000, 1000, 250, 1250},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, bonus := utils.CalculateTier(tt.amount)
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
	txRepo := repository.NewTokenTransactionRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	missionSvc := NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	svc := NewTokenAccountServiceImpl(accountRepo, topUpRepo, txRepo, centerRepo, missionSvc)

	// Center 1: New (just created)
	svc.GetOrCreate(1, "USD")
	balance, overdraft, _, err := svc.GetBalance(1)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, balance)
	assert.Equal(t, float64(5), overdraft) // new tier = -5

	// Center 2: Active (30+ days old)
	oldAccount := &models.TokenAccount{
		BeautyCenterID: 2,
		Balance:        10,
		Currency:       "USD",
	}
	oldAccount.CreatedAt = time.Now().AddDate(0, 0, -45)
	accountRepo.Create(oldAccount)

	// Add 2 top-ups (not enough for premium)
	for i := 0; i < 2; i++ {
		topUpRepo.Create(&models.TokenTopUp{
			BeautyCenterID: 2,
			AmountBs:       100,
			TokensReceived: 100,
			Status:         "approved",
			ReceiptURL:     "http://test.com/receipt",
		})
	}

	balance, overdraft, _, err = svc.GetBalance(2)
	assert.NoError(t, err)
	assert.Equal(t, float64(15), overdraft) // active tier = -15

	// Center 3: Premium (500+ purchased)
	premAccount := &models.TokenAccount{
		BeautyCenterID: 3,
		Balance:        200,
		TotalPurchased: 600,
		Currency:       "USD",
	}
	premAccount.CreatedAt = time.Now().AddDate(0, 0, -5)
	accountRepo.Create(premAccount)

	// Add 3 top-ups
	for i := 0; i < 3; i++ {
		topUpRepo.Create(&models.TokenTopUp{
			BeautyCenterID: 3,
			AmountBs:       200,
			TokensReceived: 200,
			Status:         "approved",
			ReceiptURL:     "http://test.com/receipt",
		})
	}

	balance, overdraft, _, err = svc.GetBalance(3)
	assert.NoError(t, err)
	assert.Equal(t, float64(30), overdraft) // premium tier = -30
}
