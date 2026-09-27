package response

import "time"

type ReservationServiceResponse struct {
	ServiceID   uint    `json:"service_id"`
	ServiceName string  `json:"service_name"`
	Price       float64 `json:"price"`
	Duration    int     `json:"duration"`
	// PersonnelID is the line's professional, exposed additively. It is
	// omitted when unset (legacy rows before the startup backfill ran), so
	// existing consumers are unaffected. No personnel NAME is exposed here to
	// avoid a per-line preload and the N+1 it would cause.
	PersonnelID *uint `json:"personnel_id,omitempty"`
}

type PersonnelLite struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type ClientLite struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

type ReservationResponse struct {
	ID               uint                         `json:"id"`
	BeautyCenterID   uint                         `json:"beauty_center_id"`
	BeautyCenterName string                       `json:"beauty_center_name"`
	ClientID         uint                         `json:"client_id"`
	ClientName       string                       `json:"client_name"`
	Currency         string                       `json:"currency"`
	PersonnelID      uint                         `json:"personnel_id"`
	Date             time.Time                    `json:"date"`
	StartTime        string                       `json:"start_time"`
	EndTime          string                       `json:"end_time"`
	TotalPrice       float64                      `json:"total_price"`
	Status           string                       `json:"status"`
	PaymentStatus    string                       `json:"payment_status"`
	DepositAmount    float64                      `json:"deposit_amount"`
	DepositQRURL     string                       `json:"deposit_qr_url"`
	ProofImageURL    *string                      `json:"proof_image_url,omitempty"`
	HoldExpiresAt    *time.Time                   `json:"hold_expires_at,omitempty"`
	CancelReason     *string                      `json:"cancel_reason,omitempty"`
	// SourcePostID is the feed post credited with this booking, when known.
	// Additive and omitted when nil, so existing consumers are unaffected.
	SourcePostID *uint `json:"source_post_id,omitempty"`
	Services         []ReservationServiceResponse `json:"services,omitempty"`
	Client           *ClientLite                  `json:"client,omitempty"`
	Personnel        *PersonnelLite               `json:"personnel,omitempty"`
}
