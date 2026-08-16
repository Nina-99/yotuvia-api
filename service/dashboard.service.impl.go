package service

import (
	"errors"
	"strings"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"gorm.io/gorm"
)

// topListLimit is the fixed truncation for the top-services leaderboard and
// the upcoming reservations list (spec: "top 3 services", "next 3
// reservations").
const topListLimit = 3

// DashboardServiceImpl implements DashboardService by composing aggregate
// repository queries. All time boundaries are computed here in UTC (design
// decision D2) so the repository stays deterministic and easy to test.
type DashboardServiceImpl struct {
	repo repository.DashboardRepository
}

func NewDashboardServiceImpl(repo repository.DashboardRepository) DashboardService {
	return &DashboardServiceImpl{repo: repo}
}

func (s *DashboardServiceImpl) GetStats(centerID uint) (*response.DashboardStatsResponse, error) {
	// Business rule (D3): an inactive or deleted center is invisible to the
	// dashboard — 404, not an empty stats page.
	if _, err := s.repo.GetActiveCenter(centerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	now := time.Now().UTC()
	todayStart := startOfDayUTC(now)
	tomorrowStart := todayStart.AddDate(0, 0, 1)
	weekStart := startOfWeekUTC(now)
	weekEnd := weekStart.AddDate(0, 0, 7)
	monthStart := startOfMonthUTC(now)
	monthEnd := monthStart.AddDate(0, 1, 0)

	var (
		appointmentsToday int64
		pending           int64
		totalServices     int64
		totalPersonnel    int64
		totalReviews      int64
		avgRating         float64
		revenueToday      float64
		revenueWeek       float64
		revenueMonth      float64
		revenueTotal      float64
		completedCount    int64
		topServices       []repository.TopServiceRow
		upcoming          []models.Reservation
		err               error
	)

	if appointmentsToday, err = s.repo.CountAppointmentsToday(centerID, todayStart, tomorrowStart); err != nil {
		return nil, err
	}
	if pending, err = s.repo.CountPending(centerID); err != nil {
		return nil, err
	}
	if totalServices, err = s.repo.CountActiveServices(centerID); err != nil {
		return nil, err
	}
	if totalPersonnel, err = s.repo.CountActivePersonnel(centerID); err != nil {
		return nil, err
	}
	if totalReviews, err = s.repo.CountReviews(centerID); err != nil {
		return nil, err
	}
	if avgRating, err = s.repo.AverageRating(centerID); err != nil {
		return nil, err
	}
	if revenueToday, err = s.repo.RevenueBetween(centerID, todayStart, tomorrowStart); err != nil {
		return nil, err
	}
	if revenueWeek, err = s.repo.RevenueBetween(centerID, weekStart, weekEnd); err != nil {
		return nil, err
	}
	if revenueMonth, err = s.repo.RevenueBetween(centerID, monthStart, monthEnd); err != nil {
		return nil, err
	}
	if revenueTotal, completedCount, err = s.repo.CompletedAggregates(centerID); err != nil {
		return nil, err
	}
	if topServices, err = s.repo.TopServices(centerID, topListLimit); err != nil {
		return nil, err
	}
	if upcoming, err = s.repo.UpcomingReservations(centerID, todayStart, topListLimit); err != nil {
		return nil, err
	}

	// Average ticket: total revenue over all COMPLETED reservations; 0.0 when
	// there are none (spec: "0.0 if none").
	averageTicket := 0.0
	if completedCount > 0 {
		averageTicket = revenueTotal / float64(completedCount)
	}

	// Empty slices — never null — satisfy the wire contract.
	resp := &response.DashboardStatsResponse{
		AppointmentsToday:    int(appointmentsToday),
		PendingAppointments:  int(pending),
		TotalServices:        int(totalServices),
		TotalPersonnel:       int(totalPersonnel),
		TotalReviews:         int(totalReviews),
		AverageRating:        avgRating,
		RevenueToday:         revenueToday,
		RevenueWeek:          revenueWeek,
		RevenueMonth:         revenueMonth,
		AverageTicket:        averageTicket,
		TopServices:          []response.TopServiceDTO{},
		UpcomingReservations: []response.UpcomingReservationDTO{},
	}

	for _, row := range topServices {
		resp.TopServices = append(resp.TopServices, response.TopServiceDTO{
			ServiceID:        row.ServiceID,
			ServiceName:      row.ServiceName,
			AppointmentCount: int(row.AppointmentCount),
		})
	}
	for i := range upcoming {
		resp.UpcomingReservations = append(resp.UpcomingReservations, mapUpcomingReservation(&upcoming[i]))
	}

	return resp, nil
}

// mapUpcomingReservation flattens a Reservation (with preloaded Client) into
// the public DTO; the date is normalized to the wire format YYYY-MM-DD in UTC.
func mapUpcomingReservation(res *models.Reservation) response.UpcomingReservationDTO {
	return response.UpcomingReservationDTO{
		ID:         res.ID,
		ClientName: strings.TrimSpace(res.Client.FirstName + " " + res.Client.LastName),
		Date:       res.Date.UTC().Format("2006-01-02"),
		StartTime:  res.StartTime,
		EndTime:    res.EndTime,
		Status:     res.Status,
		TotalPrice: res.TotalPrice,
	}
}

// startOfDayUTC returns midnight UTC for the day containing t.
func startOfDayUTC(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// startOfWeekUTC returns Monday 00:00 UTC for the week containing t.
func startOfWeekUTC(t time.Time) time.Time {
	t = startOfDayUTC(t)
	offset := (int(t.Weekday()) + 6) % 7 // Monday = 0
	return t.AddDate(0, 0, -offset)
}

// startOfMonthUTC returns the first day of the month, 00:00 UTC.
func startOfMonthUTC(t time.Time) time.Time {
	y, m, _ := t.UTC().Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}