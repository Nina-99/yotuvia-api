package response

import "time"

type LoyaltyProgramResponse struct {
	ID               uint      `json:"id"`
	ServiceID        *uint     `json:"service_id"`
	ServiceName      string    `json:"service_name,omitempty"`
	RewardServiceID  *uint     `json:"reward_service_id"`
	RewardServiceName string   `json:"reward_service_name,omitempty"`
	RequiredSteps    int       `json:"required_steps"`
	RewardType       string    `json:"reward_type"`
	DiscountValue    float64   `json:"discount_value"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
}

type PunchCardResponse struct {
	ID             uint                   `json:"id"`
	LoyaltyProgram LoyaltyProgramResponse `json:"loyalty_program"`
	CurrentSteps   int                    `json:"current_steps"`
	Status         string                 `json:"status"`
}

type LoyaltySettingsResponse struct {
	LoyaltyEnabled      bool `json:"loyalty_enabled"`
	LoyaltyPerPersonnel bool `json:"loyalty_per_personnel"`
}
