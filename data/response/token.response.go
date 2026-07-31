package response

import "time"

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
