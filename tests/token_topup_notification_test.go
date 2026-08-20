package tests

import (
	"fmt"
	"net/http"
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

// TestTokenTopupNotification_HTTPAuthzMatrix — the api-authz topup matrix over
// the real mounted router (TokenRoutes): missing token 401, client/personnel
// 403 with the topup unchanged, own center 200 + approved + emitted row,
// cross-center 403 with the topup still pending.
func TestTokenTopupNotification_HTTPAuthzMatrix(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := authzTestRouter(t)
	// Token account for the approve credit path (AddTokens).
	assert.NoError(t, db.Create(&models.TokenAccount{BeautyCenterID: 1, Balance: 0, Currency: "USD"}).Error)
	own := seedPendingTopUp(t, db, 1)
	other := seedPendingTopUp(t, db, 2)

	t.Run("missing token rejected", func(t *testing.T) {
		w := do(r, http.MethodPut, fmt.Sprintf("/api/admin/tokens/topups/%d", own.ID), "", `{"status":"approved"}`)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("client token denied and topup unchanged", func(t *testing.T) {
		w := do(r, http.MethodPut, fmt.Sprintf("/api/admin/tokens/topups/%d", own.ID), roleToken(t, 1, 0, "client"), `{"status":"approved"}`)
		assert.Equal(t, http.StatusForbidden, w.Code)
		var stored models.TokenTopUp
		assert.NoError(t, db.First(&stored, own.ID).Error)
		assert.Equal(t, "pending", stored.Status, "role denial must not touch the topup")
	})

	t.Run("personnel token denied", func(t *testing.T) {
		w := do(r, http.MethodPut, fmt.Sprintf("/api/admin/tokens/topups/%d", own.ID), roleToken(t, 7, 1, "personnel"), `{"status":"approved"}`)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("own center processes its topup", func(t *testing.T) {
		w := do(r, http.MethodPut, fmt.Sprintf("/api/admin/tokens/topups/%d", own.ID), roleToken(t, 1, 1, "beauty_center"), `{"status":"approved"}`)
		assert.Equal(t, http.StatusOK, w.Code)
		var stored models.TokenTopUp
		assert.NoError(t, db.First(&stored, own.ID).Error)
		assert.Equal(t, "approved", stored.Status)
		// The HTTP flow also emits the outcome row (D4).
		var n models.Notification
		assert.NoError(t, db.Where("type = ? AND beauty_center_id = ?", "topup_approved", 1).First(&n).Error)
	})

	t.Run("reprocess keeps the legacy 400 path", func(t *testing.T) {
		// Already processed (no longer pending): the controller must map the
		// plain error to 400 — only ErrForbidden maps to 403 (design D5).
		w := do(r, http.MethodPut, fmt.Sprintf("/api/admin/tokens/topups/%d", own.ID), roleToken(t, 1, 1, "beauty_center"), `{"status":"approved"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "top-up already processed", errBody(t, w.Body.String()))
		var count int64
		assert.NoError(t, db.Model(&models.Notification{}).Count(&count).Error)
		assert.Equal(t, int64(1), count, "no second notification on reprocess")
	})

	t.Run("cross-center processing blocked", func(t *testing.T) {
		w := do(r, http.MethodPut, fmt.Sprintf("/api/admin/tokens/topups/%d", other.ID), roleToken(t, 1, 1, "beauty_center"), `{"status":"approved"}`)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
		var stored models.TokenTopUp
		assert.NoError(t, db.First(&stored, other.ID).Error)
		assert.Equal(t, "pending", stored.Status, "cross-center denial leaves the topup pending")
		var count int64
		assert.NoError(t, db.Model(&models.Notification{}).Where("type = ? AND beauty_center_id = ?", "topup_approved", 2).Count(&count).Error)
		assert.Equal(t, int64(0), count, "no outcome row for the cross-center attempt")
	})
}
