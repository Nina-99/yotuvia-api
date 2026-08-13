package response

// VerifyQRResponse is the success payload of GET /reservation/verify-qr. It
// carries the reservation DTO plus an already_completed flag so a re-scan of an
// already-completed reservation renders an idempotent state instead of a
// second charge (design D3).
type VerifyQRResponse struct {
	Reservation      ReservationResponse `json:"reservation"`
	AlreadyCompleted bool                `json:"already_completed"`
}
