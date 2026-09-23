package response

import "time"

type PromotionResponse struct {
	ID                uint       `json:"id"`
	BeautyCenterID    uint       `json:"beauty_center_id"`
	Name              string     `json:"name"`
	Description       string     `json:"description"`
	Type              string     `json:"type"`
	DiscountValue     float64    `json:"discount_value"`
	BuyXCount         int        `json:"buy_x_count"`
	BuyYCount         int        `json:"buy_y_count"`
	ServiceID         *uint      `json:"service_id,omitempty"`
	ServiceName       string     `json:"service_name,omitempty"`
	ValidFrom         *time.Time `json:"valid_from,omitempty"`
	ValidTo           *time.Time `json:"valid_to,omitempty"`
	ApplicableHours   string     `json:"applicable_hours,omitempty"`
	TargetClients     string     `json:"target_clients"`
	MaxRedemptions    int        `json:"max_redemptions"`
	CurrentRedemptions int       `json:"current_redemptions"`
	IsActive          bool       `json:"is_active"`
	CreatedAt         time.Time  `json:"created_at"`
}
