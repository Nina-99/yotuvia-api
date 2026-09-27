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
	// CategoryName is the line's service category label, flattened onto the
	// line the way the mobile client reads it: `services[].category_name`. It is
	// omitted when the category is unavailable (no preloaded chain, or a service
	// whose category row is missing). No new query is introduced — the value is
	// projected from the already-preloaded Service.Category chain wherever that
	// chain is loaded; paths that do not preload it simply leave the key out.
	CategoryName string `json:"category_name,omitempty"`
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
	ID               uint   `json:"id"`
	BeautyCenterID   uint   `json:"beauty_center_id"`
	BeautyCenterName string `json:"beauty_center_name"`
	ClientID         uint   `json:"client_id"`
	ClientName       string `json:"client_name"`
	Currency         string `json:"currency"`
	// PersonnelID is the reservation's DERIVED LEAD: the professional of the
	// first (creation-order) service line, kept for backward compatibility.
	// With multi-professional bookings it may not be every viewer's own
	// professional — the authoritative per-line attribution is
	// Services[].personnel_id. A caller that needs "is this mine?" must check
	// the lines, not this field.
	PersonnelID   uint       `json:"personnel_id"`
	Date          time.Time  `json:"date"`
	StartTime     string     `json:"start_time"`
	EndTime       string     `json:"end_time"`
	TotalPrice    float64    `json:"total_price"`
	Status        string     `json:"status"`
	PaymentStatus string     `json:"payment_status"`
	DepositAmount float64    `json:"deposit_amount"`
	DepositQRURL  string     `json:"deposit_qr_url"`
	ProofImageURL *string    `json:"proof_image_url,omitempty"`
	HoldExpiresAt *time.Time `json:"hold_expires_at,omitempty"`
	CancelReason  *string    `json:"cancel_reason,omitempty"`
	// SourcePostID is the feed post credited with this booking, when known.
	// Additive and omitted when nil, so existing consumers are unaffected.
	SourcePostID *uint                        `json:"source_post_id,omitempty"`
	Services     []ReservationServiceResponse `json:"services,omitempty"`
	Client       *ClientLite                  `json:"client,omitempty"`
	Personnel    *PersonnelLite               `json:"personnel,omitempty"`
}
