package request

type CreateLoyaltyProgrmaRequest struct {
	BeautyCenterID  uint    `json:"beauty_center_id" validate:"required"`
	ServiceID       *uint   `json:"service_id"`
	RewardServiceID *uint   `json:"reward_service_id"`
	RequiredSteps   int     `json:"required_steps" validate:"required,min=1,max=50"`
	RewardType      string  `json:"reward_type" validate:"required,oneof=free percentage_discount"`
	DiscountValue   float64 `json:"discount_value" validate:"min=0,max=100"`
}

type UpdateLoyaltyProgramRequest struct {
	ServiceID       *uint    `json:"service_id"`
	RewardServiceID *uint    `json:"reward_service_id"`
	RequiredSteps   *int     `json:"required_steps" validate:"omitempty,min=1,max=50"`
	RewardType      *string  `json:"reward_type" validate:"omitempty,oneof=free percentage_discount"`
	DiscountValue   *float64 `json:"discount_value" validate:"omitempty,min=0,max=100"`
}

type UpdateLoyaltySettingsRequest struct {
	LoyaltyEnabled      *bool `json:"loyalty_enabled"`
	LoyaltyPerPersonnel *bool `json:"loyalty_per_personnel"`
}
