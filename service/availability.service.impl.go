package service

import (
	"sort"
	"time"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
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
	// personnelRepo backs the chain plan's "known professionals" validation.
	// OPTIONAL: nil disables the existence check (only relevant for a chain
	// request, which is never the sweeps wiring).
	personnelRepo repository.PersonnelRepository
}

func NewAvailabilityServiceImpl(
	centerRepo repository.BeautyCenterRepository,
	serviceRepo repository.ServiceRepository,
	treatmentRepo repository.TreatmentRepository,
	personnelRepo repository.PersonnelRepository,
) AvailabilityService {
	return &AvailabilityServiceImpl{
		centerRepo:    centerRepo,
		serviceRepo:   serviceRepo,
		treatmentRepo: treatmentRepo,
		personnelRepo: personnelRepo,
	}
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
	var legacyServices []models.Service
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
		legacyServices = services
		legacyDuration = TotalServicesDuration(services)
		if legacyDuration <= 0 {
			return empty, apperrors.ErrBadRequest
		}
	}

	// ── Multi-professional chain plan (OPTIONAL) ──
	// A chain is only meaningful over the caller-supplied service list, so it
	// cannot be combined with the treatment-driven sizing path. The plan is
	// validated with the SAME predicate the booking write path uses, and every
	// professional must resolve, otherwise the whole request is 400.
	chainDriven := len(req.Assignments) > 0
	var chain []ChainStep
	if chainDriven {
		if treatmentDriven || len(req.ServiceIDs) == 0 {
			return empty, apperrors.ErrBadRequest
		}
		plan, perr := BuildAssignmentPlan(req.ServiceIDs, req.Assignments)
		if perr != nil {
			return empty, apperrors.ErrBadRequest
		}
		if verr := s.validateChainPersonnel(plan); verr != nil {
			return empty, verr
		}
		durationByID := make(map[uint]int, len(legacyServices))
		for _, svc := range legacyServices {
			durationByID[svc.ID] = svc.Duration
		}
		chain = make([]ChainStep, 0, len(req.ServiceIDs))
		for _, sid := range req.ServiceIDs {
			chain = append(chain, ChainStep{PersonnelID: plan[sid], Duration: durationByID[sid]})
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
	// serviceIDsByCenter is filled only on the treatment path and reuses the
	// services already loaded to size the window — no extra query is issued.
	durationByCenter := make(map[uint]int, len(centers))
	serviceIDsByCenter := map[uint][]uint{}
	if treatmentDriven {
		centers, durationByCenter, serviceIDsByCenter, err = s.resolveTreatmentDurations(req.TreatmentIDs, centers)
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
	var reservations []models.Reservation
	if chainDriven {
		// Only reservations that involve a chain professional, with lines
		// preloaded — ONE query (+ preload), never one per professional.
		reservations, err = s.centerRepo.FindReservationsForRangeForPersonnel(centerIDs, from, to, ChainPersonnel(chain))
		if err != nil {
			return empty, err
		}
	} else {
		reservations, err = s.centerRepo.FindReservationsForRange(centerIDs, from, to, req.PersonnelID)
		if err != nil {
			return empty, err
		}
		// Professional-scoped searches must block only that professional's own
		// segments, not the envelope of every shared reservation. Center-wide
		// searches (personnelID == 0) are returned untouched.
		reservations = PersonnelBusyReservations(reservations, req.PersonnelID)
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

			// appendSlot keeps the period filter and the result shape in ONE
			// place so the legacy and chain branches cannot drift.
			appendSlot := func(slot response.AvailableSlot) {
				if !slot.Available {
					return
				}
				if req.Period != "" && AvailabilityPeriodForMinutes(ParseScheduleMinutes(slot.Time)) != req.Period {
					return
				}
				var personPtr *uint
				if req.PersonnelID > 0 {
					p := req.PersonnelID
					personPtr = &p
				}
				// Only the treatment path names the center's own services; the
				// legacy path leaves the slice nil so omitempty drops the field.
				var resultServiceIDs []uint
				if treatmentDriven {
					resultServiceIDs = serviceIDsByCenter[center.ID]
				}
				all = append(all, response.AvailabilitySlotResult{
					CenterID:    center.ID,
					CenterName:  center.Name,
					Date:        dateStr,
					StartTime:   slot.Time,
					PersonnelID: personPtr,
					Duration:    duration,
					ServiceIDs:  resultServiceIDs,
				})
			}

			if chainDriven {
				// The chain must fit each professional's OWN segment, so the
				// base grid is generated with NO busy reservations (only the
				// shared kernel's window/closing/past rules apply) and the chain
				// predicate then filters it. Using the center-wide busy union
				// here would over-block: a shared reservation can occupy pro 1
				// at [t,t+d1) and pro 2 at [t+d1,t+d2) without the chain
				// conflicting at all.
				busy := ChainBusyByPersonnel(centerRes[dateStr], chain)
				for _, slot := range ComputeAvailabilitySlots(d, duration, blocks, nil, now) {
					if !slot.Available {
						continue
					}
					if !ChainFits(ParseScheduleMinutes(slot.Time), chain, busy) {
						continue
					}
					appendSlot(slot)
				}
				continue
			}

			slots := ComputeAvailabilitySlots(d, duration, blocks, centerRes[dateStr], now)
			for _, slot := range slots {
				appendSlot(slot)
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

// validateChainPersonnel enforces the chain's "known professionals" rule in ONE
// batched query: every distinct professional in the plan must resolve to a live
// personnel row, otherwise the request is invalid INPUT (400). A nil repository
// disables the check (the chain path is never reachable from the sweeps wiring
// that leaves it nil).
func (s *AvailabilityServiceImpl) validateChainPersonnel(plan map[uint]uint) error {
	if s.personnelRepo == nil {
		return nil
	}
	ids := make([]uint, 0, len(plan))
	seen := make(map[uint]struct{}, len(plan))
	for _, pid := range plan {
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		ids = append(ids, pid)
	}
	existing, err := s.personnelRepo.FindExistingIDs(ids)
	if err != nil {
		return err
	}
	for _, pid := range ids {
		if !existing[pid] {
			return apperrors.ErrBadRequest
		}
	}
	return nil
}

// resolveTreatmentDurations restricts centers to those offering EVERY requested
// treatment and computes the per-center window duration.
//
// DOCUMENTED RULES:
//
//   - MULTIPLE SERVICES FOR ONE TREATMENT IN A CENTER: the MINIMUM active
//     service duration wins (lowest id on a tie, for determinism). The search
//     answers "can this center perform this treatment?", so the shortest
//     matching variant is the most inclusive; the customer picks the exact
//     service when they book.
//   - MULTIPLE TREATMENTS REQUESTED: durations are SUMMED across treatments
//     (each treatment contributing that center's minimum matching duration),
//     mirroring the booking flow, which sums the durations of all selected
//     services. A center must offer ALL requested treatments to be a candidate.
//
// It stays batched: one query to validate the treatments and one to load the
// matching services for every candidate center. The returned serviceIDs map is
// derived from those same already-loaded rows, so surfacing which services
// produced the window adds ZERO queries.
//
// The returned serviceIDs[c] is, per center, the id of the service whose
// duration was used for each requested treatment, in request order. Because the
// duration rule is MIN, this is the SINGLE sizing service per treatment — the
// booking flow can sum exactly these durations and reproduce the window.
func (s *AvailabilityServiceImpl) resolveTreatmentDurations(
	treatmentIDs []uint,
	centers []models.BeautyCenter,
) ([]models.BeautyCenter, map[uint]int, map[uint][]uint, error) {
	unique := dedupeIDs(treatmentIDs)

	treatments, err := s.treatmentRepo.FindByIDs(unique)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(treatments) != len(unique) {
		return nil, nil, nil, apperrors.ErrBadRequest // unknown or inactive treatment
	}

	centerIDs := make([]uint, 0, len(centers))
	for _, c := range centers {
		centerIDs = append(centerIDs, c.ID)
	}
	services, err := s.serviceRepo.FindActiveByTreatmentIDs(unique, centerIDs)
	if err != nil {
		return nil, nil, nil, err
	}

	// center → treatment → the active service whose MIN duration sizes the window.
	minByCenterTreatment := map[uint]map[uint]sizingService{}
	for _, svc := range services {
		if svc.TreatmentID == nil {
			continue
		}
		tid := *svc.TreatmentID
		if minByCenterTreatment[svc.BeautyCenterID] == nil {
			minByCenterTreatment[svc.BeautyCenterID] = map[uint]sizingService{}
		}
		byTreatment := minByCenterTreatment[svc.BeautyCenterID]
		current, ok := byTreatment[tid]
		// Strictly shorter wins; equal duration keeps the lowest id so the
		// reported service is deterministic.
		if !ok || svc.Duration < current.duration ||
			(svc.Duration == current.duration && svc.ID < current.id) {
			byTreatment[tid] = sizingService{id: svc.ID, duration: svc.Duration}
		}
	}

	filtered := make([]models.BeautyCenter, 0, len(centers))
	durations := make(map[uint]int, len(centers))
	serviceIDs := make(map[uint][]uint, len(centers))
	for _, c := range centers {
		byTreatment := minByCenterTreatment[c.ID]
		if byTreatment == nil {
			continue
		}
		total := 0
		complete := true
		ids := make([]uint, 0, len(unique))
		for _, tid := range unique {
			chosen, ok := byTreatment[tid]
			if !ok {
				complete = false
				break
			}
			total += chosen.duration
			ids = append(ids, chosen.id)
		}
		if !complete || total <= 0 {
			continue
		}
		filtered = append(filtered, c)
		durations[c.ID] = total
		serviceIDs[c.ID] = ids
	}
	return filtered, durations, serviceIDs, nil
}

// sizingService is the service actually used to size a center's window for one
// treatment: its id (surfaced in the search result) and its duration.
type sizingService struct {
	id       uint
	duration int
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
