package request

type CreateReservationRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	ClientID       uint   `json:"client_id"`
	PersonnelID    uint   `json:"personnel_id"`
	ServiceIDs     []uint `json:"service_ids" validate:"required,min=1"`

	Date      string `json:"date" validate:"required"`
	StartTime string `json:"start_time" validate:"required"`
}

type CancelReservationRequest struct {
	Reason string `json:"reason"`
}
