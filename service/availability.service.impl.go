package service

import (
	"sort"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
)

// maxSearchRangeDays is the widest inclusive window the search accepts.
const maxSearchRangeDays = 14

// searchDefaultLimit / searchMaxLimit clamp the returned slot count.
const (
	searchDefaultLimit = 20
	searchMaxLimit     = 50
)

type AvailabilityServiceImpl struct {
	centerRepo  repository.BeautyCenterRepository
	serviceRepo repository.ServiceRepository
}

func NewAvailabilityServiceImpl(centerRepo repository.BeautyCenterRepository, serviceRepo repository.ServiceRepository) AvailabilityService {
	return &AvailabilityServiceImpl{centerRepo: centerRepo, serviceRepo: serviceRepo}
}

// Search loads candidate centers, their working hours and the whole range's
// reservations in a BOUNDED number of queries (one per concern, never one per
// center or per day), then runs the shared slot kernel for each
// (center, date) pair.
func (s *AvailabilityServiceImpl) Search(req request.AvailabilitySearchRequest) (response.AvailabilitySearchResponse, error) {
	empty := response.AvailabilitySearchResponse{Results: []response.AvailabilitySlotResult{}}

	// ── Services → total duration (same aggregation as the booking flow) ──
	if len(req.ServiceIDs) == 0 {
		return empty, apperrors.ErrBadRequest
	}
	services, err := s.serviceRepo.FindByIDs(req.ServiceIDs)
	if err != nil {
		return empty, err
	}
	if len(services) != len(req.ServiceIDs) {
		return empty, apperrors.ErrBadRequest // unknown service
	}
	duration := TotalServicesDuration(services)
	if duration <= 0 {
		return empty, apperrors.ErrBadRequest
	}

	// ── Date range validation ──
	from, err := time.Parse("2006-01-02", req.From)
	if err != nil {
		return empty, apperrors.ErrBadRequest
	}
	to, err := time.Parse("2006-01-02", req.To)
	if err != nil {
		return empty, apperrors.ErrBadRequest
	}
	if to.Before(from) {
		return empty, apperrors.ErrBadRequest // reversed range
	}
	if int(to.Sub(from).Hours()/24) > maxSearchRangeDays {
		return empty, apperrors.ErrBadRequest // window too wide
	}

	// ── Period filter ──
	if !ValidAvailabilityPeriod(req.Period) {
		return empty, apperrors.ErrBadRequest
	}

	// ── Limit clamp ──
	limit := req.Limit
	if limit <= 0 {
		limit = searchDefaultLimit
	}
	if limit > searchMaxLimit {
		limit = searchMaxLimit
	}

	// ── Candidate centers (one query) ──
	centers, err := s.centerRepo.FindCentersForAvailability(req.CenterID)
	if err != nil {
		return empty, err
	}
	if len(centers) == 0 {
		return empty, nil
	}

	centerIDs := make([]uint, 0, len(centers))
	for _, c := range centers {
		centerIDs = append(centerIDs, c.ID)
	}

	// ── Weekdays present in the range, so working hours load in one query ──
	weekdays := map[int]bool{}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		weekdays[int(d.Weekday())] = true
	}
	dayList := make([]int, 0, len(weekdays))
	for d := range weekdays {
		dayList = append(dayList, d)
	}
	hours, err := s.centerRepo.FindWorkingHoursForCenters(centerIDs, dayList)
	if err != nil {
		return empty, err
	}
	// center → weekday → blocks
	hoursByCenter := map[uint]map[int][]models.WorkingHour{}
	for _, wh := range hours {
		if hoursByCenter[wh.BeautyCenterID] == nil {
			hoursByCenter[wh.BeautyCenterID] = map[int][]models.WorkingHour{}
		}
		hoursByCenter[wh.BeautyCenterID][wh.DayOfWeek] = append(hoursByCenter[wh.BeautyCenterID][wh.DayOfWeek], wh)
	}

	// ── Reservations over the whole range (one query) ──
	reservations, err := s.centerRepo.FindReservationsForRange(centerIDs, from, to, req.PersonnelID)
	if err != nil {
		return empty, err
	}
	// center → date (YYYY-MM-DD) → reservations
	resByCenter := map[uint]map[string][]models.Reservation{}
	for _, r := range reservations {
		key := r.Date.Format("2006-01-02")
		if resByCenter[r.BeautyCenterID] == nil {
			resByCenter[r.BeautyCenterID] = map[string][]models.Reservation{}
		}
		resByCenter[r.BeautyCenterID][key] = append(resByCenter[r.BeautyCenterID][key], r)
	}

	now := time.Now()
	all := make([]response.AvailabilitySlotResult, 0)
	for _, center := range centers {
		centerHours := hoursByCenter[center.ID]
		centerRes := resByCenter[center.ID]
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			blocks := centerHours[int(d.Weekday())]
			if len(blocks) == 0 {
				continue
			}
			dateStr := d.Format("2006-01-02")
			slots := ComputeAvailabilitySlots(d, duration, blocks, centerRes[dateStr], now)
			for _, slot := range slots {
				if !slot.Available {
					continue
				}
				if req.Period != "" && AvailabilityPeriodForMinutes(ParseScheduleMinutes(slot.Time)) != req.Period {
					continue
				}
				var personPtr *uint
				if req.PersonnelID > 0 {
					p := req.PersonnelID
					personPtr = &p
				}
				all = append(all, response.AvailabilitySlotResult{
					CenterID:    center.ID,
					CenterName:  center.Name,
					Date:        dateStr,
					StartTime:   slot.Time,
					PersonnelID: personPtr,
					Duration:    duration,
				})
			}
		}
	}

	// Order by date, then start_time, then center. HH:MM and YYYY-MM-DD are
	// fixed-width so lexicographic comparison is chronological.
	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		if a.StartTime != b.StartTime {
			return a.StartTime < b.StartTime
		}
		return a.CenterID < b.CenterID
	})

	total := len(all)
	if len(all) > limit {
		all = all[:limit]
	}
	return response.AvailabilitySearchResponse{Results: all, Total: total}, nil
}
