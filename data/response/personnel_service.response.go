package response

import "time"

type PersonnelServiceResponse struct {
	ID           uint      `json:"id"`
	PersonnelID  uint      `json:"personnel_id"`
	ServiceID    uint      `json:"service_id"`
	Price        *float64  `json:"price,omitempty"`
	Duration     *int      `json:"duration,omitempty"`
	ServiceName  string    `json:"service_name"`
	CategoryName string    `json:"category_name"`
	CreatedAt    time.Time `json:"created_at"`
}
