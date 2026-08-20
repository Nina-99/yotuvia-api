package response

// TopServiceDTO is one rank in the top-services leaderboard: how many
// reservations (excluding CANCELLED) reference the service.
type TopServiceDTO struct {
	ServiceID        uint   `json:"service_id"`
	ServiceName      string `json:"service_name"`
	AppointmentCount int    `json:"appointment_count"`
}

// UpcomingReservationDTO is the lightweight reservation slice rendered in the
// admin dashboard "next bookings" list.
type UpcomingReservationDTO struct {
	ID         uint    `json:"id"`
	ClientName string  `json:"client_name"`
	Date       string  `json:"date"`
	StartTime  string  `json:"start_time"`
	EndTime    string  `json:"end_time"`
	Status     string  `json:"status"`
	TotalPrice float64 `json:"total_price"`
}

// SeriesDayDTO represents a single day's value in a time series.
type SeriesDayDTO struct {
	Date  string  `json:"date"`  // "2006-01-02"
	Value float64 `json:"value"`
}

// DashboardStatsResponse is the single server-computed stats payload returned
// by GET /dashboard/stats for an authenticated beauty_center actor, scoped to
// the caller's centerID claim.
type DashboardStatsResponse struct {
	AppointmentsToday    int                      `json:"appointments_today"`
	PendingAppointments  int                      `json:"pending_appointments"`
	TotalServices        int                      `json:"total_services"`
	TotalPersonnel       int                      `json:"total_personnel"`
	TotalReviews         int                      `json:"total_reviews"`
	AverageRating        float64                  `json:"average_rating"`
	RevenueToday         float64                  `json:"revenue_today"`
	RevenueWeek          float64                  `json:"revenue_week"`
	RevenueMonth         float64                  `json:"revenue_month"`
	AverageTicket        float64                  `json:"average_ticket"`
	TopServices          []TopServiceDTO          `json:"top_services"`
	UpcomingReservations []UpcomingReservationDTO `json:"upcoming_reservations"`
	RevenueSeries        []SeriesDayDTO           `json:"revenue_series"`
	AppointmentsSeries   []SeriesDayDTO           `json:"appointments_series"`
}
