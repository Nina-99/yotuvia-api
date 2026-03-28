package request

import "time"

type CreateReservationRequest struct {
	BeautyCenterID uint `json:"beauty_center_id" binding:"required"`
	ClientID       uint `json:"client_id" binding:"required"`
	PersonnelID    uint `json:"personnel_id" binding:"required"`
	ServiceID      uint `json:"service_id" binding:"required"`

	Date      time.Time `json:"date" binding:"required"`
	StartTime string    `json:"start_time" binding:"required"`
	EndTime   string    `json:"end_time" binding:"required"`
	Status    string    `json:"status" binding:"required"`
}
