package request

type CreateReservationRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	ClientID       uint   `json:"client_id"`
	PersonnelID    uint   `json:"personnel_id"`
	ServiceIDs     []uint `json:"service_ids" validate:"required,min=1"`

	Date      string `json:"date" validate:"required"`
	StartTime string `json:"start_time" validate:"required"`

	// SourcePostID is an OPTIONAL attribution hint from the client app: the
	// feed post whose book CTA produced this booking. It is a tracking field,
	// so it must NEVER fail a real booking — absent, zero, or a value that does
	// not resolve to an existing post is ignored and the booking still lands
	// with a nil attribution.
	SourcePostID *uint `json:"source_post_id"`
}

// CreateAdminReservationRequest is the walk-in booking DTO (design D3): the
// client is explicit and required (unlike the client flow, where it is
// resolved from the JWT claim), while personnel stays optional. Date follows
// YYYY-MM-DD and start_time HH:MM; the format itself is enforced by the
// creation core, not the validator.
type CreateAdminReservationRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	ClientID       uint   `json:"client_id" validate:"required"`
	PersonnelID    uint   `json:"personnel_id"`
	ServiceIDs     []uint `json:"service_ids" validate:"required,min=1"`
	Date           string `json:"date" validate:"required"`
	StartTime      string `json:"start_time" validate:"required"`
}

type CancelReservationRequest struct {
	Reason string `json:"reason"`
}

// JoinWaitlistRequest takes a place in line for a concrete slot. Date is
// YYYY-MM-DD, times HH:MM. The claim rebooks exactly these parameters.
type JoinWaitlistRequest struct {
	BeautyCenterID uint   `json:"beauty_center_id" validate:"required"`
	Date           string `json:"date" validate:"required"`
	StartTime      string `json:"start_time" validate:"required"`
	PersonnelID    uint   `json:"personnel_id" validate:"required"`
	ServiceIDs     []uint `json:"service_ids" validate:"required,min=1"`
}

// UploadProofRequest carries the pre-uploaded bank receipt image URL for the
// deposit hold. The file itself travels through the existing /upload surface;
// the reservation only stores the URL.
type UploadProofRequest struct {
	ImageURL string `json:"image_url" validate:"required"`
}

// ReviewProofRequest approves (confirm) or rejects (re-upload) a receipt.
type ReviewProofRequest struct {
	Approve bool `json:"approve"`
}
