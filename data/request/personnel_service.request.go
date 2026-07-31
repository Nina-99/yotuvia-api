package request

type CreatePersonnelServiceRequest struct {
	PersonnelID uint     `json:"personnel_id" validate:"required"`
	ServiceID   uint     `json:"service_id" validate:"required"`
	Price       *float64 `json:"price,omitempty"`
	Duration    *int     `json:"duration,omitempty"`
}
