package request

type CreateReservationRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	ClientID       uint   `json:"client_id"`
	PersonnelID    uint   `json:"personnel_id"`
	ServiceIDs     []uint `json:"service_ids" validate:"required,min=1"`

	Date      string `json:"date" validate:"required"`
	StartTime string `json:"start_time" validate:"required"`
}

// CreateAdminReservationRequest is the walk-in booking DTO (design D3): the
// client is explicit and required (unlike the client flow, where it is
// resolved from the JWT claim), while personnel stays optional. Date follows
// YYYY-MM-DD and start_time HH:MM; the format itself is enforced by the
// creation core, not the validator.
type CreateAdminReservationRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	ClientID       uint   `json:"client_id" validate:"required"`
	PersonnelID    uint   `json:"personnel_id"`
	ServiceIDs     []uint `json:"service_ids" validate:"required,min=1"`
	Date           string `json:"date" validate:"required"`
	StartTime      string `json:"start_time" validate:"required"`
}

type CancelReservationRequest struct {
	Reason string `json:"reason"`
}
