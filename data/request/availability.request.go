package request

// AvailabilitySearchRequest is the parsed query contract of
// GET /availability/search. The controller parses the raw query strings into
// these typed fields; the service owns the semantic validation (dates, range
// width, period, service/treatment existence) and returns apperrors.ErrBadRequest
// on any violation.
type AvailabilitySearchRequest struct {
	// ServiceIDs are the booking services whose durations are summed exactly
	// like the booking flow sums them. Legacy path: it sizes the window but does
	// NOT filter centers.
	ServiceIDs []uint
	// TreatmentIDs are GLOBAL treatment ids. When present they take precedence
	// over ServiceIDs: they restrict candidate centers to those offering every
	// requested treatment and size the window from EACH center's own matching
	// service duration.
	TreatmentIDs []uint
	// From and To are inclusive YYYY-MM-DD bounds.
	From string
	To   string
	// CenterID narrows the search to one active center when > 0.
	CenterID uint
	// PersonnelID is passed through to the shared slot kernel when > 0.
	PersonnelID uint
	// Assignments is the OPTIONAL multi-professional chain, parsed from the
	// `assignments=serviceId:personnelId,...` query param. ORDER DEFINES THE
	// CHAIN: for a candidate start t, each professional k must be free for
	// [t + Σ previous durations, + own duration).
	//
	//   - ABSENT: unchanged behaviour and contract.
	//   - PRESENT: only slots where the WHOLE CHAIN fits are returned. The plan
	//     must cover ServiceIDs exactly once and every professional must exist;
	//     otherwise 400.
	//
	// It reuses the same ServiceAssignment shape (and the same plan validation)
	// as the client booking body, so the search and the booking cannot disagree
	// on what a valid plan is.
	Assignments []ServiceAssignment
	// Period filters candidate starts to morning | afternoon | evening.
	Period string
	// Limit caps the returned slots (default 20, max 50).
	Limit int
}
