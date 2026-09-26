package response

// AvailabilitySlotResult is one bookable start time found by the cross-center
// search. PersonnelID is echoed only when the caller supplied personnel_id,
// hence the pointer + omitempty.
type AvailabilitySlotResult struct {
	CenterID    uint   `json:"center_id"`
	CenterName  string `json:"center_name"`
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`
	PersonnelID *uint  `json:"personnel_id,omitempty"`
	Duration    int    `json:"duration"`
}

// AvailabilitySearchResponse is the GET /availability/search envelope. Total is
// the full number of matching slots before Limit; Results is the first Limit of
// them, ordered by date, then start_time, then center.
type AvailabilitySearchResponse struct {
	Results []AvailabilitySlotResult `json:"results"`
	Total   int                      `json:"total"`
}
