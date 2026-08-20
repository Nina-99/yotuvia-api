package tests

import (
	"testing"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// newTopupNotifyTestService wires the real repos (SQLite in memory) so the
// topup outcome triggers can be asserted on the DB (design D4/D5).
func newTopupNotifyTestService(db *gorm.DB) service.TokenAccountService {
	utils.Log = zap.NewNop().Sugar() // the tests package has no TestMain
	accountRepo := repository.NewTokenAccountRepositoryImpl(db)
	missionRepo := repository.NewTokenMissionRepositoryImpl(db)
	missionSvc := service.NewTokenMissionServiceImpl(missionRepo, nil, accountRepo)
	return service.NewTokenAccountServiceImpl(
		accountRepo,
		repository.NewTokenTopUpRepositoryImpl(db),
		repository.NewBeautyCenterRepositoryImpl(db),
		missionSvc,
		service.NewNotificationServiceImpl(
			repository.NewNotificationRepositoryImpl(db),
			repository.NewPushDeviceRepositoryImpl(db),
		),
	)
}

// seedPendingTopUp inserts a pending topup row for the given center.
func seedPendingTopUp(t *testing.T, db *gorm.DB, centerID uint) *models.TokenTopUp {
	t.Helper()
	topup := &models.TokenTopUp{
		BeautyCenterID: centerID,
		AmountBs:       100,
		TokensReceived: 10000,
		BonusTokens:    2500,
		ReceiptURL:     "http://test.com/receipt",
		Status:         "pending",
	}
	if err := db.Create(topup).Error; err != nil {
		t.Fatalf("failed to seed pending topup: %v", err)
	}
	return topup
}

// TestTokenTopupNotification_ApprovalEmitsApproved — approving a pending topup
// credits the tokens AND persists a center-scoped topup_approved row carrying
// the topup id (spec: "Approval notifies the center").
func TestTokenTopupNotification_ApprovalEmitsApproved(t *testing.T) {
	db := setupRouterAuthzDB(t)
	assert.NoError(t, db.Create(&models.TokenAccount{BeautyCenterID: 1, Balance: 0, Currency: "USD"}).Error)
	topup := seedPendingTopUp(t, db, 1)

	svc := newTopupNotifyTestService(db)
	assert.NoError(t, svc.ProcessTopUp(topup.ID, 1, "approved"))

	var stored models.TokenTopUp
	assert.NoError(t, db.First(&stored, topup.ID).Error)
	assert.Equal(t, "approved", stored.Status)

	var n models.Notification
	assert.NoError(t, db.Where("type = ? AND beauty_center_id = ?", "topup_approved", 1).First(&n).Error)
	assert.Equal(t, uint(0), n.ClientID, "D1 sentinel: center rows use ClientID=0")
	assert.Equal(t, uint(1), *n.BeautyCenterID)
	assert.Nil(t, n.PersonnelID)
	assert.Contains(t, n.Data, `"topup_id":`, "Data carries the topup id")
}

// TestTokenTopupNotification_RejectionEmitsRejected — rejecting a pending topup
// persists a topup_rejected row for the topup's center (spec: "Rejection
// notifies the center").
func TestTokenTopupNotification_RejectionEmitsRejected(t *testing.T) {
	db := setupRouterAuthzDB(t)
	topup := seedPendingTopUp(t, db, 1)

	svc := newTopupNotifyTestService(db)
	assert.NoError(t, svc.ProcessTopUp(topup.ID, 1, "rejected"))

	var stored models.TokenTopUp
	assert.NoError(t, db.First(&stored, topup.ID).Error)
	assert.Equal(t, "rejected", stored.Status)

	var n models.Notification
	assert.NoError(t, db.Where("type = ? AND beauty_center_id = ?", "topup_rejected", 1).First(&n).Error)
	assert.Equal(t, uint(1), *n.BeautyCenterID)
	assert.Contains(t, n.Data, `"topup_id":`, "Data carries the topup id")
}

// TestTokenTopupNotification_ReprocessEmitsNothing — processing an already
// processed topup returns the legacy "already processed" error and MUST NOT
// emit a second notification (spec: "Re-processing emits nothing"; the pending
// guard prevents double-send).
func TestTokenTopupNotification_ReprocessEmitsNothing(t *testing.T) {
	db := setupRouterAuthzDB(t)
	assert.NoError(t, db.Create(&models.TokenAccount{BeautyCenterID: 1, Balance: 0, Currency: "USD"}).Error)
	topup := seedPendingTopUp(t, db, 1)

	svc := newTopupNotifyTestService(db)
	assert.NoError(t, svc.ProcessTopUp(topup.ID, 1, "approved"))

	err := svc.ProcessTopUp(topup.ID, 1, "rejected")
	assert.EqualError(t, err, "top-up already processed", "legacy 400 path preserved")

	var count int64
	assert.NoError(t, db.Model(&models.Notification{}).Where("type IN ?", []string{"topup_approved", "topup_rejected"}).Count(&count).Error)
	assert.Equal(t, int64(1), count, "exactly one outcome row after reprocess attempt")
}

// TestTokenTopupNotification_CrossCenterForbidden — an admin that does not own
// the topup's center gets ErrForbidden, the topup stays pending, and no
// notification row is emitted (spec api-authz: "Cross-center processing
// blocked" + "topup unchanged").
func TestTokenTopupNotification_CrossCenterForbidden(t *testing.T) {
	db := setupRouterAuthzDB(t)
	topup := seedPendingTopUp(t, db, 2)

	svc := newTopupNotifyTestService(db)
	err := svc.ProcessTopUp(topup.ID, 1, "approved")
	assert.ErrorIs(t, err, apperrors.ErrForbidden)

	var stored models.TokenTopUp
	assert.NoError(t, db.First(&stored, topup.ID).Error)
	assert.Equal(t, "pending", stored.Status, "topup unchanged on forbidden processing")

	var count int64
	assert.NoError(t, db.Model(&models.Notification{}).Where("type IN ?", []string{"topup_approved", "topup_rejected"}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "no outcome row for a forbidden processing")
}
