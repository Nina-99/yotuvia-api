package request

import "time"

type CreateReservationRequest struct {
	BeautyCenterID uint `json:"beauty_center_id" validate:"required"`
	ClientID       uint `json:"client_id" validate:"required"`
	PersonnelID    uint `json:"personnel_id" validate:"required"`
	ServiceID      uint `json:"service_id" validate:"required"`

	Date      time.Time `json:"date" validate:"required"`
	StartTime string    `json:"start_time" validate:"required"`
	EndTime   string    `json:"end_time" validate:"required"`
	Status    string    `json:"status" validate:"required"`
}
