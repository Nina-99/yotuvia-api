package request

import "time"

type CreatePromotionRequest struct {
	BeautyCenterID  uint     `json:"beauty_center_id" validate:"required"`
	Name            string   `json:"name" validate:"required,max=100"`
	Description     string   `json:"description"`
	Type            string   `json:"type" validate:"required,oneof=percentage fixed_amount buy_x_get_y"`
	DiscountValue   float64  `json:"discount_value" validate:"min=0"`
	BuyXCount       int      `json:"buy_x_count" validate:"min=1"`
	BuyYCount       int      `json:"buy_y_count" validate:"min=1"`
	ServiceID       *uint    `json:"service_id"`
	ValidFrom       *time.Time `json:"valid_from"`
	ValidTo         *time.Time `json:"valid_to"`
	ApplicableHours string   `json:"applicable_hours"`
	TargetClients   string   `json:"target_clients" validate:"omitempty,oneof=all new inactive"`
	MaxRedemptions  int      `json:"max_redemptions" validate:"min=0"`
}

type UpdatePromotionRequest struct {
	Name            *string    `json:"name" validate:"omitempty,max=100"`
	Description     *string    `json:"description"`
	Type            *string    `json:"type" validate:"omitempty,oneof=percentage fixed_amount buy_x_get_y"`
	DiscountValue   *float64   `json:"discount_value" validate:"omitempty,min=0"`
	BuyXCount       *int       `json:"buy_x_count" validate:"omitempty,min=1"`
	BuyYCount       *int       `json:"buy_y_count" validate:"omitempty,min=1"`
	ServiceID       *uint      `json:"service_id"`
	ValidFrom       *time.Time `json:"valid_from"`
	ValidTo         *time.Time `json:"valid_to"`
	ApplicableHours *string    `json:"applicable_hours"`
	TargetClients   *string    `json:"target_clients" validate:"omitempty,oneof=all new inactive"`
	MaxRedemptions  *int       `json:"max_redemptions" validate:"omitempty,min=0"`
	IsActive        *bool      `json:"is_active"`
}
