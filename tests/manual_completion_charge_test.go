package tests

import (
	"testing"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Nina-99/yotuvia-api/utils"
)

// TestUpdateStatus_ManualCompletionChargesCommission — closing a booking
// WITHOUT scanning the QR must still pay the commission and advance the
// reservation mission. Before this fix the manual path flipped the status for
// free.
func TestUpdateStatus_ManualCompletionChargesCommission(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, _, _, mission, db := qrTestService(t, 100_000)
	res := seedQRReservation(t, db, "CONFIRMED", "", nil)

	require.NoError(t, svc.UpdateStatus(res.ID, "COMPLETED", 1))

	// 3.5% of 100 = 3.5 Ytv = 3500 milli.
	assert.Equal(t, int64(96_500), accountBalance(t, db))
	assert.Equal(t, int64(1), consumptionLedgerCount(t, db))
	assert.Equal(t, 1, mission.triggerCalls, "manual completion advances the reservation ladder")

	stored, err := svc.ReservationRepository.FindById(res.ID)
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", stored.Status)
}

// TestUpdateStatus_ManualCompletionFailsWithoutFunds — when the commission does
// not fit in balance + overdraft, the status flip rolls back: a reservation can
// never become COMPLETED unpaid.
func TestUpdateStatus_ManualCompletionFailsWithoutFunds(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, _, _, mission, db := qrTestService(t, 0)
	res := seedQRReservation(t, db, "CONFIRMED", "", nil)
	// New-center overdraft is 5 Ytv; a 1,000,000 ticket charges 35,000 Ytv.
	require.NoError(t, db.Model(res).Update("total_price", 1_000_000).Error)

	err := svc.UpdateStatus(res.ID, "COMPLETED", 1)
	assert.ErrorIs(t, err, apperrors.ErrInsufficientBalance)

	stored, err := svc.ReservationRepository.FindById(res.ID)
	require.NoError(t, err)
	assert.Equal(t, "CONFIRMED", stored.Status, "the status flip must roll back with the failed charge")
	assert.Equal(t, int64(0), accountBalance(t, db))
	assert.Equal(t, int64(0), consumptionLedgerCount(t, db))
	assert.Equal(t, 0, mission.triggerCalls)
}

// TestUpdateStatus_ManualCompletionIsIdempotent — a booking already completed by
// a concurrent scan does not charge twice.
func TestUpdateStatus_ManualCompletionIsIdempotent(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, _, _, _, db := qrTestService(t, 100_000)
	res := seedQRReservation(t, db, "CONFIRMED", "", nil)

	require.NoError(t, svc.UpdateStatus(res.ID, "COMPLETED", 1))
	// A second transition is rejected by the state machine (COMPLETED is
	// terminal) and must not move money.
	err := svc.UpdateStatus(res.ID, "COMPLETED", 1)
	assert.ErrorIs(t, err, apperrors.ErrConflict)
	assert.Equal(t, int64(96_500), accountBalance(t, db))
	assert.Equal(t, int64(1), consumptionLedgerCount(t, db))

	var stored models.Reservation
	require.NoError(t, db.First(&stored, res.ID).Error)
	assert.Equal(t, "COMPLETED", stored.Status)
}
