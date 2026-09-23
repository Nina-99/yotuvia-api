package service

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"gorm.io/gorm"
)

// topListLimit is the fixed truncation for the top-services leaderboard and
// the upcoming reservations list (spec: "top 3 services", "next 3
// reservations").
const topListLimit = 3

// personnelRevenueLimit is the fixed truncation for the staff revenue
// leaderboard (spec: limit 10).
const personnelRevenueLimit = 10

// topClientsLimit / atRiskLimit are the fixed truncations for the Phase-3
// client leaderboards (spec: 10 spenders, 20 at-risk).
const (
	topClientsLimit = 10
	atRiskLimit     = 20
)

// parseHHMMParseMinutes converts "HH:MM" into minutes since midnight.
// Returns ok=false for malformed values so bad rows never skew aggregates.
func slotMinutes(startTime, endTime string) (int, bool) {
	start, ok1 := hourMinuteToMinutes(startTime)
	end, ok2 := hourMinuteToMinutes(endTime)
	if !ok1 || !ok2 || end < start {
		return 0, false
	}
	return end - start, true
}

func hourMinuteToMinutes(value string) (int, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, false
	}
	hour, errH := strconv.Atoi(parts[0])
	minute, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, false
	}
	return hour*60 + minute, true
}

// deltaPct computes the percentage change between two windows per spec:
// round2((current - previous) / previous × 100). When previous is 0 there is
// no baseline — nil (JSON null) if current is also 0, otherwise a full +100.
func deltaPct(current, previous float64) *float64 {
	if previous == 0 {
		if current == 0 {
			return nil
		}
		v := 100.0
		return &v
	}
	v := round2((current - previous) / previous * 100)
	return &v
}

// round2 rounds to two decimal places (wire contract for pct fields).
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// emptyHourHeatmap returns a zero-initialized 7×24 matrix: index 0 = Sunday
// (mirrors working_hours.day_of_week), inner index = start hour 0–23.
func emptyHourHeatmap() [][]int {
	m := make([][]int, 7)
	for i := range m {
		m[i] = make([]int, 24)
	}
	return m
}

// heatmapHour parses the leading hour out of an "HH:MM" reservation slot,
// rejecting malformed or out-of-range values defensively.
func heatmapHour(startTime string) (int, bool) {
	if len(startTime) < 2 {
		return 0, false
	}
	hour, err := strconv.Atoi(startTime[:2])
	if err != nil || hour < 0 || hour > 23 {
		return 0, false
	}
	return hour, true
}

// parseDateish parses the driver-dependent textual rendering of a date
// aggregate (CAST(... AS TEXT)): SQLite emits "2006-01-02 00:00:00", PG may
// emit "2006-01-02 00:00:00+00" or RFC3339. Date-only is accepted too.
func parseDateish(value string) (time.Time, bool) {
	layouts := []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// DashboardServiceImpl implements DashboardService by composing aggregate
// repository queries and the token account service for balance info.
// All time boundaries are computed here in UTC (design decision D2) so
// the repository stays deterministic and easy to test.
type DashboardServiceImpl struct {
	repo            repository.DashboardRepository
	tokenAccountSvc TokenAccountService
}

func NewDashboardServiceImpl(repo repository.DashboardRepository, tokenAccountSvc TokenAccountService) DashboardService {
	return &DashboardServiceImpl{repo: repo, tokenAccountSvc: tokenAccountSvc}
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

	// ── Token balance (feat-token-economy) ──
	var (
		balanceMilli   int64
		overdraftMilli int64
	)
	if s.tokenAccountSvc != nil {
		var tokenErr error
		balanceMilli, overdraftMilli, _, tokenErr = s.tokenAccountSvc.GetBalance(centerID)
		if tokenErr != nil {
			balanceMilli = 0
			overdraftMilli = 0
		}
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
		Balance:              utils.FromMilli(balanceMilli),
		OverdraftLimit:       utils.FromMilli(overdraftMilli),
		AvailableBalance:     utils.FromMilli(balanceMilli + overdraftMilli),
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

// GetStatsWithRange returns the full stats payload plus time series for the
// given date range [from, to). The service computes UTC boundaries, calls
// repository by-day queries, and zero-fills missing days so every date in the
// range is present in the series arrays (never null). churnDays drives the
// at-risk-clients window (controller-validated 1–365; defensive default 30).
func (s *DashboardServiceImpl) GetStatsWithRange(centerID uint, from, to time.Time, churnDays int) (*response.DashboardStatsResponse, error) {
	if churnDays <= 0 || churnDays > 365 {
		churnDays = 30
	}
	// Business rule: inactive/deleted center → 404
	if _, err := s.repo.GetActiveCenter(centerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	// Compute UTC boundaries: half-open [fromUTC, toUTC)
	fromUTC := startOfDayUTC(from)
	toUTC := startOfDayUTC(to.AddDate(0, 0, 1))

	// Get base stats (reusing GetStats logic but without the date-range specific calls)
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
	if upcoming, err = s.repo.UpcomingReservations(centerID, todayStart, topListLimit); err != nil {
		return nil, err
	}

	// ── Token balance (feat-token-economy) ──
	var (
		balanceMilli   int64
		overdraftMilli int64
	)
	if s.tokenAccountSvc != nil {
		var tokenErr error
		balanceMilli, overdraftMilli, _, tokenErr = s.tokenAccountSvc.GetBalance(centerID)
		if tokenErr != nil {
			balanceMilli = 0
			overdraftMilli = 0
		}
	}

	// Average ticket
	averageTicket := 0.0
	if completedCount > 0 {
		averageTicket = revenueTotal / float64(completedCount)
	}

	// Get time series data
	revenueRows, err := s.repo.RevenueByDay(centerID, fromUTC, toUTC)
	if err != nil {
		return nil, err
	}
	appointmentRows, err := s.repo.AppointmentsByDay(centerID, fromUTC, toUTC)
	if err != nil {
		return nil, err
	}

	// Convert repo rows to maps for O(1) lookup during zero-fill
	revenueMap := make(map[string]float64)
	for _, row := range revenueRows {
		revenueMap[row.Date.Format("2006-01-02")] = row.Value
	}
	appointmentMap := make(map[string]int64)
	for _, row := range appointmentRows {
		appointmentMap[row.Date.Format("2006-01-02")] = row.Value
	}

	// ── Phase 3 metrics (feat-admin-analytics) ──────────────────────────

	// Ticket trend inputs: daily COMPLETED counts (denominator).
	completedCountRows, err := s.repo.CompletedCountByDay(centerID, fromUTC, toUTC)
	if err != nil {
		return nil, err
	}
	completedMap := make(map[string]int64)
	for _, row := range completedCountRows {
		completedMap[row.Date.Format("2006-01-02")] = row.Value
	}

	// Rating trend: raw review points bucketed by UTC day in Go (D4).
	reviewPoints, err := s.repo.ReviewPoints(centerID, fromUTC, toUTC)
	if err != nil {
		return nil, err
	}
	ratingSum := make(map[string]float64)
	ratingN := make(map[string]int)
	for _, point := range reviewPoints {
		key := startOfDayUTC(point.CreatedAt).Format("2006-01-02")
		ratingSum[key] += float64(point.Rating)
		ratingN[key]++
	}

	// Client analytics: new vs recurring + top spenders.
	recurrence, err := s.repo.ClientRecurrence(centerID, fromUTC, toUTC)
	if err != nil {
		return nil, err
	}
	topClientRows, err := s.repo.TopClientsBySpend(centerID, fromUTC, toUTC, topClientsLimit)
	if err != nil {
		return nil, err
	}

	// At-risk clients: last COMPLETED visit strictly before today-churnDays.
	cutoff := todayStart.AddDate(0, 0, -churnDays)
	atRiskRows, err := s.repo.AtRiskClients(centerID, cutoff, atRiskLimit)
	if err != nil {
		return nil, err
	}
	atRiskDTOs := make([]response.AtRiskClientDTO, 0, len(atRiskRows))
	for _, row := range atRiskRows {
		lastVisit, ok := parseDateish(row.LastVisit)
		if !ok {
			continue
		}
		daysSince := int(todayStart.Sub(startOfDayUTC(lastVisit)).Hours() / 24)
		atRiskDTOs = append(atRiskDTOs, response.AtRiskClientDTO{
			ClientID:      row.ClientID,
			ClientName:    row.ClientName,
			LastVisitDate: startOfDayUTC(lastVisit).Format("2006-01-02"),
			DaysSince:     daysSince,
		})
	}

	// Occupancy per personnel. The schema has no per-person schedules, so the
	// center-level working hours are the shared availability denominator —
	// valid for RELATIVE comparison across staff (D7 revision). Staff without
	// bookings in range don't appear; overlaps may push occupancy above 100.
	occupancyDTOs, err := s.buildOccupancy(centerID, fromUTC, toUTC)
	if err != nil {
		return nil, err
	}

	// Zero-fill: iterate [fromUTC, toUTC) step 1 day
	var revenueSeries []response.SeriesDayDTO
	var appointmentsSeries []response.SeriesDayDTO
	var ticketSeries []response.SeriesDayDTO
	var ratingSeries []response.SeriesDayDTO

	for d := fromUTC; d.Before(toUTC); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		revVal := revenueMap[dateStr]
		appVal := float64(appointmentMap[dateStr])

		revenueSeries = append(revenueSeries, response.SeriesDayDTO{
			Date:  dateStr,
			Value: revVal,
		})
		appointmentsSeries = append(appointmentsSeries, response.SeriesDayDTO{
			Date:  dateStr,
			Value: appVal,
		})

		// Ticket trend (Phase 3): revenue ÷ COMPLETED count; 0 when none.
		ticketVal := 0.0
		if n := completedMap[dateStr]; n > 0 {
			ticketVal = revVal / float64(n)
		}
		ticketSeries = append(ticketSeries, response.SeriesDayDTO{
			Date:  dateStr,
			Value: round2(ticketVal),
		})

		// Rating trend (Phase 3): mean review rating; zero-filled like series.
		ratingVal := 0.0
		if n := ratingN[dateStr]; n > 0 {
			ratingVal = ratingSum[dateStr] / float64(n)
		}
		ratingSeries = append(ratingSeries, response.SeriesDayDTO{
			Date:  dateStr,
			Value: round2(ratingVal),
		})
	}

	// ── Phase 2 metrics (feat-admin-analytics) ──────────────────────────

	// Delta % vs the immediately preceding window of equal length (D6).
	// UTC windows have no DST, so day count from the duration is exact.
	days := int(toUTC.Sub(fromUTC).Hours() / 24)
	prevFrom := fromUTC.AddDate(0, 0, -days)

	curRevenue, err := s.repo.RevenueBetween(centerID, fromUTC, toUTC)
	if err != nil {
		return nil, err
	}
	prevRevenue, err := s.repo.RevenueBetween(centerID, prevFrom, fromUTC)
	if err != nil {
		return nil, err
	}
	curCounts, err := s.repo.CountStatusBetween(centerID, fromUTC, toUTC)
	if err != nil {
		return nil, err
	}
	prevCounts, err := s.repo.CountStatusBetween(centerID, prevFrom, fromUTC)
	if err != nil {
		return nil, err
	}

	// Cancellation rate over the requested window (spec: rate = CANCELLED /
	// total × 100; 0 when total is 0).
	cancellationRate := 0.0
	if curCounts.Total > 0 {
		cancellationRate = round2(float64(curCounts.Cancelled) / float64(curCounts.Total) * 100)
	}

	// Heatmap rows → 7×24 matrix built in Go (D4, driver portability).
	heatmapRows, err := s.repo.HeatmapRows(centerID, fromUTC, toUTC)
	if err != nil {
		return nil, err
	}
	hourHeatmap := emptyHourHeatmap()
	for _, row := range heatmapRows {
		hour, ok := heatmapHour(row.StartTime)
		if !ok {
			continue
		}
		hourHeatmap[int(row.Date.UTC().Weekday())][hour]++
	}

	// Range-scoped leaderboards.
	topServicesByRevenue, err := s.repo.TopServicesByRevenue(centerID, fromUTC, toUTC, topListLimit)
	if err != nil {
		return nil, err
	}
	personnelRevenueRows, err := s.repo.PersonnelRevenue(centerID, fromUTC, toUTC, personnelRevenueLimit)
	if err != nil {
		return nil, err
	}

	// Build response with series
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
		RevenueSeries:        revenueSeries,
		AppointmentsSeries:   appointmentsSeries,

		RevenueDeltaPct:      deltaPct(curRevenue, prevRevenue),
		AppointmentsDeltaPct: deltaPct(float64(curCounts.Total-curCounts.Cancelled), float64(prevCounts.Total-prevCounts.Cancelled)),
		CancelledCount:       int(curCounts.Cancelled),
		TotalAppointments:    int(curCounts.Total),
		CancellationRatePct:  cancellationRate,
		HourHeatmap:          hourHeatmap,
		PersonnelRevenue:     []response.PersonnelRevenueDTO{},

		NewClients:         int(recurrence.NewClients),
		RecurringClients:   int(recurrence.RecurringClients),
		TopClients:         []response.ClientSpendDTO{},
		AtRiskClients:      atRiskDTOs,
		PersonnelOccupancy: occupancyDTOs,
		TicketSeries:       ticketSeries,
		RatingSeries:       ratingSeries,

		Balance:          utils.FromMilli(balanceMilli),
		OverdraftLimit:   utils.FromMilli(overdraftMilli),
		AvailableBalance: utils.FromMilli(balanceMilli + overdraftMilli),
	}

	for _, row := range topServicesByRevenue {
		resp.TopServices = append(resp.TopServices, response.TopServiceDTO{
			ServiceID:        row.ServiceID,
			ServiceName:      row.ServiceName,
			AppointmentCount: int(row.AppointmentCount),
			Revenue:          row.Revenue,
		})
	}
	for _, row := range personnelRevenueRows {
		resp.PersonnelRevenue = append(resp.PersonnelRevenue, response.PersonnelRevenueDTO{
			PersonnelID:      row.PersonnelID,
			PersonnelName:    row.PersonnelName,
			Revenue:          row.Revenue,
			AppointmentCount: int(row.AppointmentCount),
		})
	}
	for _, row := range topClientRows {
		resp.TopClients = append(resp.TopClients, response.ClientSpendDTO{
			ClientID:   row.ClientID,
			ClientName: row.ClientName,
			TotalSpent: row.TotalSpent,
			VisitCount: int(row.VisitCount),
		})
	}
	for i := range upcoming {
		resp.UpcomingReservations = append(resp.UpcomingReservations, mapUpcomingReservation(&upcoming[i]))
	}

	return resp, nil
}

// buildOccupancy computes per-staff booked minutes (parsed from HH:MM slots)
// over the center-level schedule availability in [fromUTC, toUTC). Staff with
// no non-CANCELLED bookings in range are omitted; staff whose availability is
// zero would divide by zero and are likewise skipped.
func (s *DashboardServiceImpl) buildOccupancy(centerID uint, fromUTC, toUTC time.Time) ([]response.PersonnelOccupancyDTO, error) {
	slots, err := s.repo.BookedSlots(centerID, fromUTC, toUTC)
	if err != nil {
		return nil, err
	}
	schedules, err := s.repo.WorkingHoursRows(centerID)
	if err != nil {
		return nil, err
	}

	// Shared denominator: center availability across the full range.
	availableByDow := make(map[time.Weekday]int)
	for _, wh := range schedules {
		minutes, ok := slotMinutes(wh.StartTime, wh.EndTime)
		if !ok {
			continue
		}
		availableByDow[time.Weekday(wh.DayOfWeek)] += minutes
	}
	availableMinutes := 0
	for d := fromUTC; d.Before(toUTC); d = d.AddDate(0, 0, 1) {
		availableMinutes += availableByDow[d.UTC().Weekday()]
	}
	if availableMinutes <= 0 {
		return []response.PersonnelOccupancyDTO{}, nil
	}

	bookedByName := make(map[uint]int)
	nameByID := make(map[uint]string)
	var order []uint
	for _, slot := range slots {
		minutes, ok := slotMinutes(slot.StartTime, slot.EndTime)
		if !ok {
			continue
		}
		if _, seen := bookedByName[slot.PersonnelID]; !seen {
			order = append(order, slot.PersonnelID)
		}
		bookedByName[slot.PersonnelID] += minutes
	}
	if len(order) == 0 {
		return []response.PersonnelOccupancyDTO{}, nil
	}

	// Resolve display names via the personnel revenue leaderboard rows when
	// available is overkill — reuse a dedicated lightweight query instead:
	names, err := s.repo.PersonnelNames(centerID, order)
	if err != nil {
		return nil, err
	}
	for id, name := range names {
		nameByID[id] = name
	}

	result := make([]response.PersonnelOccupancyDTO, 0, len(order))
	for _, id := range order {
		booked := bookedByName[id]
		pct := round2(float64(booked) / float64(availableMinutes) * 100)
		result = append(result, response.PersonnelOccupancyDTO{
			PersonnelID:      id,
			PersonnelName:    nameByID[id],
			BookedMinutes:    booked,
			AvailableMinutes: availableMinutes,
			OccupancyPct:     pct,
		})
	}
	return result, nil
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
