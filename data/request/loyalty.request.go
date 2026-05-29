package request

type CreateLoyaltyProgrmaRequest struct {
	BeautyCenterID uint    `json:"beauty_center_id" validate:"required"`
	ServiceID      uint    `json:"service_id" validate:"required"`
	RequiredSteps  int     `json:"required_steps" validate:"required,min=1,max=50"`
	RewardType     string  `json:"reward_type" validate:"required,oneof=free percentage_discount"`
	DiscountValue  float64 `json:"discount_value" validate:"required,min=0,max=100"`
}
