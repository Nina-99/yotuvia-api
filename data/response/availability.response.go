package response

// AvailabilitySlotResult is one bookable start time found by the cross-center
// search. PersonnelID is echoed only when the caller supplied personnel_id,
// hence the pointer + omitempty.
//
// ServiceIDs is populated ONLY on the treatment-driven path (treatment_ids
// supplied). It carries the ids of the CENTER'S OWN matching ACTIVE services
// whose durations produced Duration, so the booking flow can go straight from a
// result to a reservation without re-picking a service. On the legacy path
// (service_ids supplied, no treatment_ids) the field is omitted: the caller
// already knows the ids and there is no center filter, so the requested ids may
// not even belong to the result center.
//
// MULTIPLE MATCHING SERVICES: the window duration rule is MIN per treatment, so
// ServiceIDs reports exactly the service whose duration was used (deterministic:
// the lowest id on a duration tie). Reporting every variant would misstate the
// duration and break the booking sum, so only the sizing service is surfaced.
// MULTIPLE TREATMENTS: the union across treatments is reported in request order;
// summing their durations equals the searched window, mirroring the booking flow.
type AvailabilitySlotResult struct {
	CenterID    uint   `json:"center_id"`
	CenterName  string `json:"center_name"`
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`
	PersonnelID *uint  `json:"personnel_id,omitempty"`
	Duration    int    `json:"duration"`
	ServiceIDs  []uint `json:"service_ids,omitempty"`
}

// AvailabilitySearchResponse is the GET /availability/search envelope. Total is
// the full number of matching slots before Limit; Results is the first Limit of
// them, ordered by date, then start_time, then center.
type AvailabilitySearchResponse struct {
	Results []AvailabilitySlotResult `json:"results"`
	Total   int                      `json:"total"`
}
