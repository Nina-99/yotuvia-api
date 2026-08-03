package tests

import (
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/stretchr/testify/assert"
)

// TestTokenDTO_TransactionMilliToDecimal — the wire format must present stored
// milli-YOTTUs as decimal YOTTUs with full precision (spec: "Full stored
// precision is preserved on the wire": 25555 milli -> 25.555).
func TestTokenDTO_TransactionMilliToDecimal(t *testing.T) {
	tx := models.TokenTransaction{
		ID:            1,
		Amount:        25555,
		BalanceBefore: 100000,
		BalanceAfter:  125555,
		Type:          "consumption",
		Description:   "test",
	}
	resp := response.NewTokenTransactionResponse(tx)

	assert.Equal(t, uint(1), resp.ID)
	assert.InDelta(t, 25.555, resp.Amount, 1e-9)
	assert.InDelta(t, 100.0, resp.BalanceBefore, 1e-9)
	assert.InDelta(t, 125.555, resp.BalanceAfter, 1e-9)
	assert.Equal(t, "consumption", resp.Type)
}

// TestTokenDTO_BalanceMilliToDecimal — 25500 milli must serialize as 25.50.
func TestTokenDTO_BalanceMilliToDecimal(t *testing.T) {
	resp := response.NewTokenBalanceResponse(25500, 5000, "USD")

	assert.InDelta(t, 25.50, resp.Balance, 1e-9)
	assert.InDelta(t, 5.0, resp.OverdraftLimit, 1e-9)
	assert.Equal(t, "USD", resp.Currency)
}

// TestTokenDTO_TopUpMilliToDecimal — tokens convert from milli while the
// bolivar-denominated AmountBs stays untouched (it is not part of the YOTTU
// ledger).
func TestTokenDTO_TopUpMilliToDecimal(t *testing.T) {
	topup := models.TokenTopUp{
		ID:             2,
		AmountBs:       100.5,
		TokensReceived: 25555,
		BonusTokens:    500,
		ReceiptURL:     "http://test.com/r",
		Status:         "approved",
	}
	resp := response.NewTokenTopUpResponse(topup)

	assert.InDelta(t, 25.555, resp.TokensReceived, 1e-9)
	assert.InDelta(t, 0.5, resp.BonusTokens, 1e-9)
	assert.InDelta(t, 100.5, resp.AmountBs, 1e-9, "AmountBs is bolivares, not YOTTU ledger")
	assert.Equal(t, "approved", resp.Status)
}

// TestTokenDTO_MissionRewardMilliToDecimal — mission rewards convert from milli.
func TestTokenDTO_MissionRewardMilliToDecimal(t *testing.T) {
	now := time.Now()
	progress := models.TokenMissionProgress{
		MissionID: 3,
		Mission: models.TokenMission{
			Name:         "First Completed Reservation",
			Description:  "Complete your first reservation",
			RewardTokens: 3000,
			Icon:         "🎯",
		},
		CurrentCount:  1,
		TargetCount:   2,
		CompletedAt:   &now,
		RewardClaimed: true,
	}
	resp := response.NewTokenMissionResponse(progress)

	assert.Equal(t, uint(3), resp.ID)
	assert.Equal(t, "First Completed Reservation", resp.Name)
	assert.InDelta(t, 3.0, resp.RewardTokens, 1e-9)
	assert.Equal(t, 1, resp.CurrentCount)
	assert.Equal(t, 2, resp.TargetCount)
	assert.True(t, resp.Completed)
	assert.True(t, resp.RewardClaimed)
}
