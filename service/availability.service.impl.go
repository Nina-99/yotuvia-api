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
	centerRepo    repository.BeautyCenterRepository
	serviceRepo   repository.ServiceRepository
	treatmentRepo repository.TreatmentRepository
}

func NewAvailabilityServiceImpl(
	centerRepo repository.BeautyCenterRepository,
	serviceRepo repository.ServiceRepository,
	treatmentRepo repository.TreatmentRepository,
) AvailabilityService {
	return &AvailabilityServiceImpl{centerRepo: centerRepo, serviceRepo: serviceRepo, treatmentRepo: treatmentRepo}
}

// Search loads candidate centers, their working hours and the whole range's
// reservations in a BOUNDED number of queries (one per concern, never one per
// center or per day), then runs the shared slot kernel for each
// (center, date) pair.
//
// Two sizing strategies exist:
//
//   - service_ids (legacy): the caller-supplied services are summed into one
//     duration used for every candidate center. Behavior is unchanged.
//   - treatment_ids: candidate centers are restricted to those offering EVERY
//     requested treatment, and each center's window is sized from ITS OWN
//     matching service duration. This is what makes a cross-center "find me a
//     haircut" search honest.
//
// PRECEDENCE: when both are supplied, treatment_ids wins and service_ids is
// ignored. The treatment path cannot honor a caller-supplied duration without
// breaking the per-center guarantee, so the two cannot be mixed.
func (s *AvailabilityServiceImpl) Search(req request.AvailabilitySearchRequest) (response.AvailabilitySearchResponse, error) {
	empty := response.AvailabilitySearchResponse{Results: []response.AvailabilitySlotResult{}}

	treatmentDriven := len(req.TreatmentIDs) > 0

	// ── Services → total duration (legacy path; same aggregation as booking) ──
	legacyDuration := 0
	if !treatmentDriven {
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
		legacyDuration = TotalServicesDuration(services)
		if legacyDuration <= 0 {
			return empty, apperrors.ErrBadRequest
		}
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

	// ── Duration plan: one duration per candidate center ──
	durationByCenter := make(map[uint]int, len(centers))
	if treatmentDriven {
		centers, durationByCenter, err = s.resolveTreatmentDurations(req.TreatmentIDs, centers)
		if err != nil {
			return empty, err
		}
		if len(centers) == 0 {
			return empty, nil // no center offers the requested treatment(s)
		}
	} else {
		for _, c := range centers {
			durationByCenter[c.ID] = legacyDuration
		}
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
		duration := durationByCenter[center.ID]
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

// resolveTreatmentDurations restricts centers to those offering EVERY requested
// treatment and computes the per-center window duration.
//
// DOCUMENTED RULES:
//
//   - MULTIPLE SERVICES FOR ONE TREATMENT IN A CENTER: the MINIMUM active
//     service duration wins. The search answers "can this center perform this
//     treatment?", so the shortest matching variant is the most inclusive; the
//     customer picks the exact service when they book.
//   - MULTIPLE TREATMENTS REQUESTED: durations are SUMMED across treatments
//     (each treatment contributing that center's minimum matching duration),
//     mirroring the booking flow, which sums the durations of all selected
//     services. A center must offer ALL requested treatments to be a candidate.
//
// It stays batched: one query to validate the treatments and one to load the
// matching services for every candidate center.
func (s *AvailabilityServiceImpl) resolveTreatmentDurations(
	treatmentIDs []uint,
	centers []models.BeautyCenter,
) ([]models.BeautyCenter, map[uint]int, error) {
	unique := dedupeIDs(treatmentIDs)

	treatments, err := s.treatmentRepo.FindByIDs(unique)
	if err != nil {
		return nil, nil, err
	}
	if len(treatments) != len(unique) {
		return nil, nil, apperrors.ErrBadRequest // unknown or inactive treatment
	}

	centerIDs := make([]uint, 0, len(centers))
	for _, c := range centers {
		centerIDs = append(centerIDs, c.ID)
	}
	services, err := s.serviceRepo.FindActiveByTreatmentIDs(unique, centerIDs)
	if err != nil {
		return nil, nil, err
	}

	// center → treatment → minimum duration
	minByCenterTreatment := map[uint]map[uint]int{}
	for _, svc := range services {
		if svc.TreatmentID == nil {
			continue
		}
		tid := *svc.TreatmentID
		if minByCenterTreatment[svc.BeautyCenterID] == nil {
			minByCenterTreatment[svc.BeautyCenterID] = map[uint]int{}
		}
		byTreatment := minByCenterTreatment[svc.BeautyCenterID]
		if current, ok := byTreatment[tid]; !ok || svc.Duration < current {
			byTreatment[tid] = svc.Duration
		}
	}

	filtered := make([]models.BeautyCenter, 0, len(centers))
	durations := make(map[uint]int, len(centers))
	for _, c := range centers {
		byTreatment := minByCenterTreatment[c.ID]
		if byTreatment == nil {
			continue
		}
		total := 0
		complete := true
		for _, tid := range unique {
			d, ok := byTreatment[tid]
			if !ok {
				complete = false
				break
			}
			total += d
		}
		if !complete || total <= 0 {
			continue
		}
		filtered = append(filtered, c)
		durations[c.ID] = total
	}
	return filtered, durations, nil
}

// dedupeIDs preserves first-seen order while removing duplicates. Duplicate
// treatment filters are treated as one (a repeated treatment must not double
// the summed duration).
func dedupeIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
