package response

import (
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
)

type TokenBalanceResponse struct {
	Balance        float64 `json:"balance"`
	Reserved       float64 `json:"reserved"`
	Available      float64 `json:"available"`
	OverdraftLimit float64 `json:"overdraft_limit"`
	TotalPurchased float64 `json:"total_purchased"`
	TotalConsumed  float64 `json:"total_consumed"`
	Currency       string  `json:"currency"`
	// FiatRate is the reference conversion for the center's currency:
	// 1 Ytv = FiatRate <Currency>. Display only; accounting never uses it.
	FiatRate float64 `json:"fiat_rate"`

	// TokenHealth is the owner-facing state (ok | low | deficit | suspended).
	// Same source the booking gate uses, so the banner never contradicts a
	// rejected booking.
	TokenHealth *TokenHealthDTO `json:"token_health,omitempty"`
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
	BeautyCenterID uint       `json:"beauty_center_id"`
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

// The wire format stays decimal Ytv; these constructors convert the stored
// int64 milli-Ytv to decimal on serialization so clients never see millis.

// NewTokenBalanceResponse builds a balance payload from milli values. Available
// is what the owner can still spend: balance minus the held portion plus the
// overdraft limit. FiatRate is 1 Ytv expressed in the center's currency.
func NewTokenBalanceResponse(balanceMilli, reservedMilli, overdraftMilli int64, currency string, fiatRate float64) TokenBalanceResponse {
	return TokenBalanceResponse{
		Balance:        utils.FromMilli(balanceMilli),
		Reserved:       utils.FromMilli(reservedMilli),
		Available:      utils.FromMilli(balanceMilli - reservedMilli + overdraftMilli),
		OverdraftLimit: utils.FromMilli(overdraftMilli),
		Currency:       currency,
		FiatRate:       fiatRate,
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
// part of the Ytv ledger, and stays untouched. BeautyCenterID is carried so
// the platform-wide queue can tell one centre's row from another's.
func NewTokenTopUpResponse(topup models.TokenTopUp) TokenTopUpResponse {
	return TokenTopUpResponse{
		ID:             topup.ID,
		BeautyCenterID: topup.BeautyCenterID,
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
