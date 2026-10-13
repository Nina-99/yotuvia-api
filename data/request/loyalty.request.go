package request

type CreateLoyaltyProgrmaRequest struct {
	BeautyCenterID  uint    `json:"beauty_center_id" validate:"required"`
	ServiceID       *uint   `json:"service_id"`
	RewardServiceID *uint   `json:"reward_service_id"`
	RequiredSteps   int     `json:"required_steps" validate:"required,min=1,max=50"`
	RewardType      string  `json:"reward_type" validate:"required,oneof=free percentage_discount"`
	DiscountValue   float64 `json:"discount_value" validate:"min=0,max=100"`
	// DurationDays is the nullable card-lifetime policy in days. Nil = no
	// expiry (default, back-compat). Slice 1 persists only, no enforcement.
	DurationDays *int `json:"duration_days" validate:"omitempty,min=1,max=3650"`
}

type UpdateLoyaltyProgramRequest struct {
	ServiceID       *uint    `json:"service_id"`
	RewardServiceID *uint    `json:"reward_service_id"`
	RequiredSteps   *int     `json:"required_steps" validate:"omitempty,min=1,max=50"`
	RewardType      *string  `json:"reward_type" validate:"omitempty,oneof=free percentage_discount"`
	DiscountValue   *float64 `json:"discount_value" validate:"omitempty,min=0,max=100"`
	// DurationDays nil = leave unchanged. Use explicit null-clear via
	// ClearDuration (separate flag) — plain JSON null decodes as nil which is
	// ambiguous, so clearing is handled in slice 2 owner UI.
	DurationDays *int `json:"duration_days" validate:"omitempty,min=1,max=3650"`
}

type UpdateLoyaltySettingsRequest struct {
	LoyaltyEnabled      *bool `json:"loyalty_enabled"`
	LoyaltyPerPersonnel *bool `json:"loyalty_per_personnel"`
}
