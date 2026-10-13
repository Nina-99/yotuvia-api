package response

import "time"

type LoyaltyProgramResponse struct {
	ID               uint      `json:"id"`
	BeautyCenterID   uint      `json:"beauty_center_id,omitempty"`
	BeautyCenterName string    `json:"beauty_center_name,omitempty"`
	ServiceID        *uint     `json:"service_id"`
	ServiceName      string    `json:"service_name,omitempty"`
	RewardServiceID  *uint     `json:"reward_service_id"`
	RewardServiceName string   `json:"reward_service_name,omitempty"`
	RequiredSteps    int       `json:"required_steps"`
	RewardType       string    `json:"reward_type"`
	DiscountValue    float64   `json:"discount_value"`
	IsActive         bool      `json:"is_active"`
	DurationDays     *int      `json:"duration_days,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type PunchCardResponse struct {
	ID               uint                   `json:"id"`
	LoyaltyProgram   LoyaltyProgramResponse `json:"loyalty_program"`
	CurrentSteps     int                    `json:"current_steps"`
	Status           string                 `json:"status"`
	ExpiresAt        *time.Time             `json:"expires_at,omitempty"`
	BeautyCenterID   uint                   `json:"beauty_center_id,omitempty"`
	BeautyCenterName string                 `json:"beauty_center_name,omitempty"`
}

// PunchHistoryItemResponse is one row of the client loyalty history timeline.
type PunchHistoryItemResponse struct {
	ID          uint      `json:"id"`
	CardID      uint      `json:"card_id"`
	ProgramID   uint      `json:"program_id"`
	ServiceName string    `json:"service_name,omitempty"`
	Stamps      int       `json:"stamps"`
	Required    int       `json:"required"`
	CreatedAt   time.Time `json:"created_at"`
}

// PendingRewardResponse is a completed-but-unredeemed card awaiting redeem.
type PendingRewardResponse struct {
	CardID            uint    `json:"card_id"`
	ProgramID         uint    `json:"program_id"`
	ServiceName       string  `json:"service_name,omitempty"`
	RewardDescription string  `json:"reward_description"`
	RewardType        string  `json:"reward_type"`
	DiscountValue     float64 `json:"discount_value"`
}

// PublicProgramResponse is the minimal anonymous-safe program summary for a
// center (no internal IDs beyond program + names).
type PublicProgramResponse struct {
	ID               uint    `json:"id"`
	ServiceName      string  `json:"service_name,omitempty"`
	RequiredSteps    int     `json:"required_steps"`
	RewardType       string  `json:"reward_type"`
	RewardDescription string `json:"reward_description"`
	DiscountValue    float64 `json:"discount_value"`
	IsActive         bool    `json:"is_active"`
}

// RewardLineResponse rides the verify-qr envelope: the earn outcome of ONE
// scan (batched across the reservation's services).
type RewardLineResponse struct {
	CardID            uint   `json:"card_id"`
	ProgramID         uint   `json:"program_id"`
	Stamps            int    `json:"stamps"`
	Required          int    `json:"required"`
	Completed         bool   `json:"completed"`
	RewardDescription string `json:"reward_description"`
}

type LoyaltySettingsResponse struct {
	LoyaltyEnabled      bool `json:"loyalty_enabled"`
	LoyaltyPerPersonnel bool `json:"loyalty_per_personnel"`
}

// LoyaltyMetricsResponse is one program's issued-vs-redeemed summary for the
// owner metrics view. Counts are always present (zero-valued, never null).
type LoyaltyMetricsResponse struct {
	ProgramID        uint    `json:"program_id"`
	ServiceName      string  `json:"service_name,omitempty"`
	RequiredSteps    int     `json:"required_steps"`
	ActiveCards      int64   `json:"active_cards"`
	CompletedCards   int64   `json:"completed_cards"`
	Redemptions      int64   `json:"redemptions"`
	RedemptionRate   float64 `json:"redemption_rate"`
}
