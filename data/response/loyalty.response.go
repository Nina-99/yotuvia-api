package response

import "time"

type LoyaltyProgramResponse struct {
	ID            uint      `json:"id"`
	ServiceID     uint      `json:"service_id"`
	RequiredSteps int       `json:"required_steps"`
	RewardType    string    `json:"reward_type"`
	DiscountValue float64   `json:"discount_value"`
	IsActive      bool      `json:"is_activate"`
	CreatedAt     time.Time `json:"created_at"`
}

type PunchCardResponse struct {
	ID             uint                   `json:"id"`
	LoyaltyProgram LoyaltyProgramResponse `json:"loyalty_program"`
	CurrentSteps   int                    `json:"current_steps"`
	Status         string                 `json:"status"`
}
