package response

import (
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/utils"
)

type TokenBalanceResponse struct {
	Balance        float64 `json:"balance"`
	OverdraftLimit float64 `json:"overdraft_limit"`
	TotalPurchased float64 `json:"total_purchased"`
	TotalConsumed  float64 `json:"total_consumed"`
	Currency       string  `json:"currency"`
}

type TokenTransactionResponse struct {
	ID            uint      `json:"id"`
	Amount        float64   `json:"amount"`
	Type          string    `json:"type"`
	BalanceBefore float64   `json:"balance_before"`
	BalanceAfter  float64   `json:"balance_after"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
}

type TokenHistoryResponse struct {
	Transactions []TokenTransactionResponse `json:"transactions"`
	Total        int64                      `json:"total"`
	Page         int                        `json:"page"`
	Limit        int                        `json:"limit"`
}

type TokenTopUpResponse struct {
	ID             uint       `json:"id"`
	AmountBs       float64    `json:"amount_bs"`
	TokensReceived float64    `json:"tokens_received"`
	BonusTokens    float64    `json:"bonus_tokens"`
	ReceiptURL     string     `json:"receipt_url"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ValidatedAt    *time.Time `json:"validated_at,omitempty"`
}

type TokenMissionResponse struct {
	ID            uint    `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	RewardTokens  float64 `json:"reward_tokens"`
	Icon          string  `json:"icon"`
	CurrentCount  int     `json:"current_count"`
	TargetCount   int     `json:"target_count"`
	Completed     bool    `json:"completed"`
	RewardClaimed bool    `json:"reward_claimed"`
}

type ReferralResponse struct {
	ID         uint      `json:"id"`
	ReferredID uint      `json:"referred_id"`
	Status     string    `json:"status"`
	Reward     float64   `json:"reward"`
	CreatedAt  time.Time `json:"created_at"`
}

// The wire format stays decimal YOTTUs; these constructors convert the stored
// int64 milli-YOTTUs to decimal on serialization so clients never see millis.

// NewTokenBalanceResponse builds a balance payload from milli values.
func NewTokenBalanceResponse(balanceMilli, overdraftMilli int64, currency string) TokenBalanceResponse {
	return TokenBalanceResponse{
		Balance:        utils.FromMilli(balanceMilli),
		OverdraftLimit: utils.FromMilli(overdraftMilli),
		Currency:       currency,
	}
}

// NewTokenTransactionResponse converts a stored ledger row (milli) to decimal.
func NewTokenTransactionResponse(tx models.TokenTransaction) TokenTransactionResponse {
	return TokenTransactionResponse{
		ID:            tx.ID,
		Amount:        utils.FromMilli(tx.Amount),
		Type:          tx.Type,
		BalanceBefore: utils.FromMilli(tx.BalanceBefore),
		BalanceAfter:  utils.FromMilli(tx.BalanceAfter),
		Description:   tx.Description,
		CreatedAt:     tx.CreatedAt,
	}
}

// NewTokenTopUpResponse converts tokens to decimal; AmountBs is bolivares, not
// part of the YOTTU ledger, and stays untouched.
func NewTokenTopUpResponse(topup models.TokenTopUp) TokenTopUpResponse {
	return TokenTopUpResponse{
		ID:             topup.ID,
		AmountBs:       topup.AmountBs,
		TokensReceived: utils.FromMilli(topup.TokensReceived),
		BonusTokens:    utils.FromMilli(topup.BonusTokens),
		ReceiptURL:     topup.ReceiptURL,
		Status:         topup.Status,
		CreatedAt:      topup.CreatedAt,
		ValidatedAt:    topup.ValidatedAt,
	}
}

// NewTokenMissionResponse converts the mission reward (milli) to decimal.
func NewTokenMissionResponse(progress models.TokenMissionProgress) TokenMissionResponse {
	return TokenMissionResponse{
		ID:            progress.MissionID,
		Name:          progress.Mission.Name,
		Description:   progress.Mission.Description,
		RewardTokens:  utils.FromMilli(progress.Mission.RewardTokens),
		Icon:          progress.Mission.Icon,
		CurrentCount:  progress.CurrentCount,
		TargetCount:   progress.TargetCount,
		Completed:     progress.CompletedAt != nil,
		RewardClaimed: progress.RewardClaimed,
	}
}
