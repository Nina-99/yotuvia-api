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
	// Period filters candidate starts to morning | afternoon | evening.
	Period string
	// Limit caps the returned slots (default 20, max 50).
	Limit int
}
