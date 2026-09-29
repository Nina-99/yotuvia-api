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

// PersonnelMetricsRange is the inclusive date window echoed back by the staff
// metrics endpoint ("2006-01-02").
type PersonnelMetricsRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// PersonnelMetricsTotals are the range totals for the acting professional.
// appointments excludes CANCELLED; revenue counts COMPLETED only, attributed
// per line. currency is the centre's ISO code.
type PersonnelMetricsTotals struct {
	Appointments int     `json:"appointments"`
	Completed    int     `json:"completed"`
	Cancelled    int     `json:"cancelled"`
	NoShow       int     `json:"no_show"`
	Revenue      float64 `json:"revenue"`
	Currency     string  `json:"currency"`
}

// PersonnelTopService is one rank of the professional-scoped top-services
// leaderboard. Revenue is SUM(price) over their COMPLETED lines in range;
// count is the number of distinct COMPLETED bookings carrying the service.
type PersonnelTopService struct {
	ServiceID   uint    `json:"service_id"`
	ServiceName string  `json:"service_name"`
	Revenue     float64 `json:"revenue"`
	Count       int     `json:"count"`
}

// PersonnelMetricsPrevious is the same-range-length window that immediately
// precedes the requested range, used by the client to compute per-metric deltas
// ("+12% vs the previous period"). Only the totals travel: the series would be
// duplicated payload for no client use. Currency is the same centre's code.
type PersonnelMetricsPrevious struct {
	Range  PersonnelMetricsRange  `json:"range"`
	Totals PersonnelMetricsTotals `json:"totals"`
}

// PersonnelOccupancyLabel is the declared basis of the availability
// denominator. The schema has no per-professional schedule, so the CENTRE's
// working hours are the shared denominator — an APPROXIMATION for part-timers.
// The literal is exposed so the client can label the figure honestly instead of
// presenting a centre-based denominator as a personal contract.
const PersonnelOccupancyLabel = "center_hours"

// PersonnelOccupancy is the acting professional's utilisation over the
// requested range. AvailableMinutes is the centre's scheduled minutes for the
// range (approximation: no per-professional schedule exists). BookedMinutes is
// the professional's OWN booked time, derived from their segments via the
// shared kernel. RatePct is round2(booked/available*100) and is 0 (never NaN)
// when there are no working hours in range. Overlapping out-of-schedule rows
// can push the rate above 100, exactly as the admin dashboard occupancy does.
type PersonnelOccupancy struct {
	AvailableMinutes int     `json:"available_minutes"`
	BookedMinutes    int     `json:"booked_minutes"`
	RatePct          float64 `json:"rate_pct"`
	Basis            string  `json:"basis"`
}

// PersonnelMetricsResponse is the frozen wire shape of GET
// /api/reservation/me/metrics. All series cover the requested inclusive range
// with zero-filled gaps; arrays are never null. Previous and Occupancy are
// additive blocks; the original fields keep their names and meaning.
type PersonnelMetricsResponse struct {
	Range              PersonnelMetricsRange    `json:"range"`
	Totals             PersonnelMetricsTotals   `json:"totals"`
	RevenueSeries      []SeriesDayDTO           `json:"revenue_series"`
	AppointmentsSeries []SeriesDayDTO           `json:"appointments_series"`
	TopServices        []PersonnelTopService    `json:"top_services"`
	Previous           PersonnelMetricsPrevious `json:"previous"`
	Occupancy          PersonnelOccupancy       `json:"occupancy"`
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
