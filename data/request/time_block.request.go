package request

// CreateTimeBlockRequest is the body of POST /time-blocks.
//
// `personnel_id` is OPTIONAL: omitted means the WHOLE centre is blocked
// (owner-only); set means one professional. A staff actor may never set it to
// another professional — the service forces it to the caller.
type CreateTimeBlockRequest struct {
	Date      string `json:"date" binding:"required"`
	StartTime string `json:"start_time" binding:"required"`
	EndTime   string `json:"end_time" binding:"required"`
	// PersonnelID nil = centre-wide. Pointer so "absent" is distinguishable
	// from a zero id.
	PersonnelID *uint  `json:"personnel_id"`
	Reason      string `json:"reason"`
}
